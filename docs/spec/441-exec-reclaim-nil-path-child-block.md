# 441 — 子行程結束時把 nil 路徑落子的程式塊併回 arena

狀態：**DRAFT**（第 1 版）
日期：2026-10-16
前置：[`009-memory-allocator`](009-memory-allocator.md)、[`439-exec-child-environment`](439-exec-child-environment.md)、[`440-dos-loadstate-arena-ownership`](440-dos-loadstate-arena-ownership.md)

---

## 1. 為什麼要做

《Strike Commander》的 `SCCD.EXE` 依序 EXEC `menu.exe`、`opttest.exe`、`strike.exe`。dosgolem 上 `strike.exe` 的 EXEC 失敗，SCCD 印
「Error loading executable: strike.exe」（`strike_commander` 的 `workplace/logs/probe451a.log`，第 19016561177 步 `AX=4B00h`）。

記憶體流向（已證實，`probe443.log` 第 20–22 行與 EXEC 紀錄）：

| 步數 | 事件 | 結果 |
|---|---|---|
| 105499 | SCCD（PSP `0100`）`AH=4Ah BX=0128h` | `freeSeg=0228` |
| — | SCCD EXEC `menu.exe`（arena 為 nil） | 走 `placeChild` 的 nil 路徑：程式塊從 `freeSeg` 起，PSP `0231`，`procFrame.arenaSeg=0` |
| 106710 | MENU `AH=4Ah BX=32DAh` | `setPSPBlock`（arena nil）：`freeSeg=350B` |
| 之後 | MENU 第一次 `AH=48h` | `initArena` 以 `350B` 建 arena |
| MENU 結束 | `terminate` 巢狀非 TSR 路 | `releaseOwnedBy(0231)` 只放 arena 內的塊；`arenaSeg=0` 不呼叫 `freeArenaSeg`；`freeSeg=0228`。arena 仍從 `350B` 起 |

`[0228, 350B)`（`12E3h` 節，約 77 KB）從此不在 arena 內，任何配置都拿不到。OPTTEST 因此放在 `3514`；OPTTEST 結束後 arena 只剩一塊
`350B`、大小 `6AF4h` 的自由塊，`strike.exe` 映像需要 `721Ah` 節（MZ 檔頭：映像 467360 bytes、`e_minalloc=0`）加 PSP 與環境，配置失敗。
補回這段後最大自由塊為 `7DD7h` 節，足夠。

真 DOS 的 `AH=4Ch` 釋放子行程擁有的所有 MCB（含程式塊）並與相鄰自由塊合併（DOSBox-X-MCP-Debugger `dosbox-src/src/dos/dos_execute.cpp` 150 行，`DOS_Terminate` 非 TSR 時呼叫
`dos_memory.cpp` 129 行的 `DOS_FreeProcessMemory`）。dosgolem 的 arena 路徑已照做（`freeArenaSeg`）；nil 路徑落子、子行程執行中才建 arena 的組合漏了。

## 2. 規則

`terminate` 的巢狀非 TSR 分支（`len(procStack) > 0 && !tsr`），在 `releaseOwnedBy`、`freeArenaSeg` 之後，原本的 `d.freeSeg = f.freeSeg` 改為：

1. 條件：`f.arenaSeg == 0`（子行程走 nil 路徑落子）且 `d.arena != nil` 且 `len(d.arena) > 0` 且 `f.freeSeg < d.arena[0].seg`，且 `d.nilTSRTop <= f.freeSeg`（第 3 項）。
2. 成立時呼叫 `d.setPSPBlock(f.freeSeg)`：`[f.freeSeg, arena[0].seg)` 以一個自由塊插到 arena 最前並 `coalesce`（差距不足 2 節時照 `setPSPBlock` 現行規則不插），
   `freeSeg` 設為 `f.freeSeg`；接著 `d.syncMCB()` 讓客體 MCB 鏈與 arena 一致。不成立時維持現行 `d.freeSeg = f.freeSeg`。
3. TSR 護欄：新增 `DOS` 欄位 `nilTSRTop uint16`（不進狀態檔）。任何 TSR 結束（巢狀與疊底兩條路）發生在 `d.arena == nil` 時，`nilTSRTop = max(nilTSRTop, curPSP+keep)`。
   理由：nil 路徑下的孫行程 TSR 只推高 `freeSeg`，常駐區落在 `[f.freeSeg, arena[0].seg)` 內，不能併回。`nilTSRTop` 不在狀態檔，讀檔後為 0（已知限制，§4）。
4. 範圍外：疊底行程（`len(procStack)==0`）與 TSR 分支不改；arena 建立時機、`placeChild`、`setPSPBlock` 本身不改；`SaveState` 格式不改（錨點不動）。

## 3. 驗收

1. `internal/dos` 新測試（沿用 `exec_arena_test.go` 的工具）：
   - 重現：父行程不碰記憶體，EXEC 子程式（nil 路徑）；子程式 `AH=4Ah` 縮自己、再 `AH=48h` 拿一塊（建 arena）後 `AH=4Ch`；結束後 `largestFree` 等於
     `MemTop − f.freeSeg − 1`（整段併回）；再 EXEC 一支需要超過「舊 arena 最大塊」的子程式成功（CF=0），PSP 等於 `f.freeSeg+1`（或開 `ExecEnvCopy` 時在環境塊之後）。
   - 客體 MCB 鏈：從 `PSPSeg−1` 走到 `Z`，每塊位置與大小等於 arena。
   - TSR 護欄：子程式 EXEC 孫程式（nil 路徑）、孫程式 `AH=31h` 常駐，子程式之後 `AH=48h` 建 arena 再結束：常駐區不被併回（`largestFree` 不含它）。
   - `int 20h` 結束路徑同樣併回。
   - 開 `ExecEnvCopy`（nil 路徑環境塊在 `freeSeg`）一組。
   - 條件不成立（子程式從頭到尾不碰記憶體，arena 仍 nil）時行為與改動前相同。
2. 突變自檢：拿掉併回、拿掉 `syncMCB`、拿掉 TSR 護欄，各至少一個測試失敗。
3. `go test ./internal/... ./cmd/probe ./session ./oracle` 全綠（SingleStepTests 不得 skip）；`go test ./apps/...` 與改動前比對，差異逐項說明；錨點常數不變。
4. 觸發案例（需原版，另附 probe 收據）：從 `s1900.state`（OPTTEST 機庫畫面）不能驗（MENU 結束已發生）；改用冷開機（`-exec-env`、`root-nosound`）走完
   MENU→OPTTEST→訓練任務，記錄 OPTTEST 的 PSP 與 `strike.exe` EXEC 結果、STRIKE.EXE 的後續停點。

## 4. 已知限制

- `nilTSRTop` 不在狀態檔：讀檔前曾有 nil 路徑 TSR、讀檔後其父行程結束時，常駐區可能被併回。改狀態檔要動 `DOSState` 錨點，不在本規格範圍。
- 讀檔前已發生的遺失（例如從 MENU 結束後的存檔接著跑）不會修復；觸發案例要冷開機。
