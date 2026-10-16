# 441 — 子行程結束時把 nil 路徑落子的程式塊併回 arena

狀態：**READY**（第 2 版，依審查改為選項開啟、修正 TSR 護欄、補驗收）
日期：2026-10-16
前置：[`009-memory-allocator`](009-memory-allocator.md)、[`439-exec-child-environment`](439-exec-child-environment.md)、[`440-dos-loadstate-arena-ownership`](440-dos-loadstate-arena-ownership.md)

---

## 1. 為什麼要做

《Strike Commander》的 `SCCD.EXE` 依序 EXEC `menu.exe`、`opttest.exe`、`strike.exe`。dosgolem 上 `strike.exe` 的 EXEC 失敗，SCCD 印
「Error loading executable: strike.exe」（`strike_commander` 的 `workplace/logs/probe451a.log`，第 19016561177 步 `AX=4B00h`）。

記憶體流向（已證實，`probe443.log` 的 `AH=4Ah` 紀錄與 EXEC 紀錄；該 log 在 `-exec-env` 下取得）：

| 步數 | 事件 | 結果 |
|---|---|---|
| 105499 | SCCD（PSP `0100`）`AH=4Ah BX=0128h` | `freeSeg=0228` |
| — | SCCD EXEC `menu.exe`（arena 為 nil） | `ExecEnvCopy` 的 nil 路徑：環境 MCB 在 `0228`，程式塊 MCB 在 `0230`，PSP `0231`，`procFrame.arenaSeg=0`（`exec.go` 175–198 行） |
| 106710 | MENU `AH=4Ah BX=32DAh` | `setPSPBlock`（arena nil）：`freeSeg=350B` |
| 之後 | MENU 第一次 `AH=48h` | `initArena` 以 `350B` 建 arena |
| MENU 結束 | `terminate` 巢狀非 TSR 路 | `releaseOwnedBy(0231)` 只放 arena 內的塊；`arenaSeg=0` 不呼叫 `freeArenaSeg`；`freeSeg=0228`。arena 仍從 `350B` 起 |

`[0228, 350B)`（約 77 KB）從此不在 arena 內。OPTTEST 因此放在 `3514`；OPTTEST 結束後 arena 只剩一塊 `350B`、大小 `6AF4h` 的自由塊。`strike.exe`
的映像段數為 `721Ah`（`MZImageParags`，467360 bytes），`need` 再加 PSP 的 `10h` 與 1，另有環境塊，配置失敗。併回後最大自由塊為 `7DD7h` 節，足夠。

真 DOS 的 `AH=4Ch` 釋放 owner 等於子 PSP 的所有 MCB（含程式塊）並與相鄰自由塊合併（DOSBox-X-MCP-Debugger `dosbox-src/src/dos/dos_execute.cpp` 150 行，
`DOS_Terminate` 非 TSR 時呼叫 `dos_memory.cpp` 129 行的 `DOS_FreeProcessMemory`），孫行程的常駐塊因 owner 不同而保留。dosgolem 的 arena 路徑已照做
（`freeArenaSeg`）；nil 路徑落子、子行程執行中才建 arena 的組合漏了。

## 2. 規則

### 2.1 選項

- `internal/dos` 的 `DOS` 新增欄位 `ExecReclaim bool`（放在 `ExecEnvCopy` 旁），預設 false，行為與現行完全相同；新程式碼都在 `if d.ExecReclaim` 之內。
- `cmd/probe` 加 `flag.Bool` 旗標 `-exec-reclaim`（放在 `-exec-env` 旁，說明文字引用本規格），在 `d := dos.New(m, *root)` 之後賦值，可與 `-load-state` 任意搭配。
- 預設關閉的理由：併回後 nil 路徑子行程結束後的 `AH=48h` first fit 會落在更低段，`apps/` 下有 EXEC 的程式（`dm`、`eob3`、`phantasie`、`fd2` 等）配置位置會變，
  既有收據可能改變。要不要預設開啟另由使用者決定。

### 2.2 併回（`ExecReclaim` 開啟時）

`terminate` 的巢狀非 TSR 分支（`len(procStack) > 0 && !tsr`），在 `releaseOwnedBy`、`freeArenaSeg` 之後，原本的 `d.freeSeg = f.freeSeg` 改為：

1. 條件：`f.arenaSeg == 0` 且 `d.arena != nil` 且 `len(d.arena) > 0` 且 `f.freeSeg < d.arena[0].seg` 且 `d.nilTSRTop <= f.freeSeg`。
2. 成立時呼叫新函式 `reclaimBelowArena(f.freeSeg)`：
   - `gap := arena[0].seg − f.freeSeg`；`gap >= 2` 時在 arena 最前插入自由塊 `{seg: f.freeSeg, size: gap−1, free: true}` 並 `coalesce`（`arena[0]` 是已用塊時不合併）；
     `gap == 1` 時不插塊（MCB 放不下，承襲 `setPSPBlock` 的規則，留 1 段洞）。
   - `freeSeg = f.freeSeg`，接著 `syncMCB()`，讓客體 MCB 鏈與 arena 一致（`f.freeSeg` 處原本的環境或程式 MCB 改寫成自由 MCB）。
   - 不呼叫 `setPSPBlock`，避免它日後為 `AH=4Ah` 修改時影響此處。
3. 不成立時維持現行 `d.freeSeg = f.freeSeg`。
4. `syncMCB` 把頂層程式的 `PSPSeg−1` 寫成涵蓋 `[PSPSeg, freeSeg)`，巢狀時仍在執行的父行程 nil 路徑塊也算在內，這是既有做法，不在本規格範圍。

### 2.3 TSR 護欄

- 新增 `DOS` 欄位 `nilTSRTop uint16`（不進狀態檔）。
- 巢狀 TSR 分支（`len(procStack) > 0 && tsr`）在 `f.arenaSeg == 0` 時（常駐行程的程式塊在 arena 之外），於還原 `curPSP` 之前
  設 `nilTSRTop = max(nilTSRTop, curPSP + keep)`。不論當下 arena 是否為 nil：孫行程以 nil 路徑落子、自己建了 arena 再常駐的情形也要記。
- 疊底 TSR 不記：疊底常駐後 `freeSeg >= curPSP+keep`，之後的 `procFrame.freeSeg` 都不低於它，護欄用不到。
- 只升不降，是保守上界：被擋下只代表漏回收（與現行相同），不會誤併。`Install` 與 `DOS.LoadState` 歸零。
- 只在 `ExecReclaim` 開啟時記錄。

### 2.4 可觀測性

`DOS` 新增診斷 slice `Reclaims []ReclaimRecord`（不進狀態檔），每次 §2.2 第 1 項前四個條件成立時記一筆：步數、`f.freeSeg`、原 `arena[0].seg`、是否被護欄擋下。
`cmd/probe` 在配置器狀態後列出。

### 2.5 範圍外

疊底行程（`len(procStack)==0`）的同類洞（`spawnQueued` 在 arena nil 時 `queuedSeg=0`，之後建了 arena，結束時只設 `freeSeg = curPSP−1`）不改；
`initArena` 在 `base >= MemTop` 建出空 arena 時不回收；`SaveState` 格式不改（錨點不動）。

## 3. 驗收

1. 選項關閉時：`go test ./internal/... ./cmd/probe ./session ./oracle` 與 `go test ./apps/...` 結果與改動前相同；錨點常數不變。
2. `internal/dos` 新測試（選項開啟，沿用 `exec_arena_test.go` 的工具）：
   - 重現：父行程不碰記憶體，EXEC 子程式（nil 路徑）；子程式 `AH=4Ah` 縮自己、再 `AH=48h` 拿一塊後 `AH=4Ch`；結束後 `largestFree` 等於 `MemTop − f.freeSeg − 1`；
     再 EXEC 一支需要超過「舊 arena 最大塊」的子程式成功（CF=0），PSP 為 `f.freeSeg+1`。對照組選項關閉時第二次 EXEC 回錯誤 8。
   - 子行程只建 arena、`AH=48h` 拿到的塊自己 `AH=49h` 還掉後結束（`releaseOwnedBy` 回 0、不會先 sync）：客體 MCB 鏈仍正確（抓「拿掉 syncMCB」突變）。
   - 客體 MCB 鏈：從 `PSPSeg−1` 走到 `Z`，每塊位置、大小、自由與否等於 arena；巢狀情境也驗。
   - 結束路徑：`AH=4Ch`、`int 20h`、`AH=00h` 各一組。
   - `ExecEnvCopy` 開啟一組：`f.freeSeg` 處原本的環境 MCB 被改寫成自由 MCB。
   - 巢狀深度 2：R → C（nil）→ G（nil），G 建 arena 後結束，再 C 結束，兩次都驗 MCB 鏈與 `largestFree`。
   - TSR 護欄：C（nil）→ G（nil），G `AH=4Ah`、`AH=48h` 建 arena 後 `AH=31h` 常駐；C 結束時常駐區不被併回，`Reclaims` 記一筆被擋下。另一組 G 在 arena nil 時常駐。
   - `arena[0]` 是已用塊時，插入的自由塊不與它合併。
   - `gap == 1`：不插塊，`freeSeg` 設為 `f.freeSeg`，arena 不變。
   - 子行程建 arena 後 `AH=4Ah` 放大自己再結束。
   - 讀檔：在 nil 路徑子行程執行中、arena 已建時 `SaveState`，新機器 `LoadState`（440 重建後 `arenaSeg=0`）；子行程結束後照樣併回；同一組驗讀檔後 `nilTSRTop=0`。
   - 條件不成立（子程式不碰記憶體，arena 仍 nil）時行為與選項關閉相同。
   - `-exec-reclaim` 旗標預設 false、開啟後欄位為 true（比照 `cmd/probe/pmflag_test.go`）。
3. 突變自檢：拿掉併回、拿掉 `syncMCB`、拿掉 TSR 護欄、護欄改回「只在 arena 為 nil 時記」、拿掉 `f.freeSeg < arena[0].seg` 條件，各至少一個測試失敗。
4. `go build ./...`、`go vet ./internal/... ./cmd/probe`、`gofmt -l` 無輸出；SingleStepTests 不得 skip。
5. 觸發案例（需原版，另附 probe 收據）：冷開機（`-exec-env -exec-reclaim`、`root-nosound`）走完 MENU→OPTTEST→訓練任務，記錄 OPTTEST 的 PSP、`Reclaims`、
   `strike.exe` 的 EXEC 結果與後續停點。

## 4. 已知限制

- `nilTSRTop` 不在狀態檔：讀檔前曾有 nil 路徑巢狀 TSR、讀檔後其父行程結束時，常駐區可能被併回。改狀態檔要動 `DOSState` 錨點，不在本規格範圍。
- 讀檔時子行程仍在執行者可以併回；讀檔前已經結束、已遺失的段不會修復。
