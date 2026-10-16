# 439 — EXEC 為子行程建立環境副本（附程式路徑）

狀態：**DRAFT**（第 2 版，依第 1 輪審查改寫）
日期：2026-10-16
前置：[`009-exec`](009-exec.md)、[`009-memory-allocator`](009-memory-allocator.md)、[`438-probe-guard-physical-pc`](438-probe-guard-physical-pc.md)

DOSBox-X 引用一律指 `DOSBox-X-MCP-Debugger/dosbox-src/src/` 下的檔案。

---

## 1. 為什麼要做

《Strike Commander》由 `SCCD.EXE` 以 `int 21h AX=4B00h` 依序執行 `menu.exe`、`opttest.exe`、`mkterr.exe`、`strike.exe`（`strike_commander` 的 `docs/re/002`）。
dosgolem 在不開音效的設定下跑到 MENU 選單並點選後（冷開機，第 3403993729 步起），`OPTTEST.EXE` 只執行約 340 萬道就以結束碼 3 結束，主控台出現
「Abnormal program termination」。這段期間 OPTTEST 唯一開過的檔是 `C:\SCCD.EXE`：讀 20 bytes 的檔頭，跳到位移 9232（即 `SCCD.EXE` 的檔尾）讀 16 bytes 得 0。
這是執行期以 argv[0] 開自己的執行檔找附加資料（overlay）的典型行為（強推論）；它開到的是父程式。

原因（已證實，程式碼）：`internal/dos/exec.go` 的 `spawn` 在參數區塊的環境段為 0 時，把子行程 PSP:2Ch 直接設成父行程的環境段（約 211–215 行），子行程與父行程
共用同一塊環境，環境尾端的程式路徑仍是父程式的 `C:\SCCD.EXE`。

DOSBox-X 在 `AL=00h`／`01h` 的 EXEC 都替子行程配一塊新環境（`dos/dos_execute.cpp` 的 `MakeEnv`，182–238 行；呼叫處 400 行）：

1. 來源：參數區塊的環境段；為 0 時用父行程 PSP:2Ch。來源段為 0（沒有環境）時只寫一個 0 byte（`envsize=1`）。
2. 來源有環境時從偏移 0 起逐 byte 檢查該處的 word 是否為 0；到 `MAXENV-83=32685` 仍未找到回錯誤 10（`DOSERR_ENVIRONMENT_INVALID`）；`envsize` 為該位置加 2。
3. 配置 `para(envsize+83)` 節（至少 1 節），在程式塊之前配置。
4. 複製 `envsize` bytes，接著寫 word `0001h`，再寫 `DOS_Canonicalize` 的完整路徑與結尾 0。
5. 子行程 PSP:2Ch 指向新塊。程式塊配置失敗時釋放環境塊（441 行）。

`AL=03h`（載入 overlay）不建環境。

## 2. 規則

### 2.1 選項

- `internal/dos` 的 `DOS` 新增欄位 `ExecEnvCopy bool`（放在 `StrictPSPGrowth` 旁），預設 false，行為與現行完全相同；所有新程式碼都在 `if d.ExecEnvCopy` 之內。
- `cmd/probe` 加 `flag.Bool` 旗標 `-exec-env`（放在 `-program-path`、`-empty-env` 附近，說明文字引用本規格），在 `d := dos.New(m, *root)` 之後賦值。可與 `-load-state`
  任意搭配，不要求與狀態檔一致。`-pm-profile` 不自動開啟本選項。
- 預設關閉的理由：這是多個專案共用的 DOS 層，開啟後子程式的 PSP 位置會往上移，既有收據可能改變；要不要改為預設開啟，另由使用者決定。

### 2.2 `ExecEnvCopy` 開啟時的 EXEC（`AL=00h`）

dosgolem 的 `exec` 只支援 `AL=00h` 與 `03h`（其餘回 AX=1）；`AL=01h` 不在範圍，`AL=03h` 不建環境。

1. 正規化路徑：`d.canonical(name)`（`internal/dos/dosinfo.go`，`AH=60h` 的 `trueName` 同一函式）；超過 127 字元回錯誤 3。這個字串只寫進環境，與服務層找檔用的
   `resolve` 無關。
2. 來源與長度：照 §1 第 1、2 項（含錯誤 10）。
3. 環境塊大小：`para(envsize + max(83, 2+len(path)+1))` 節，至少 1 節。路徑長時比 DOSBox-X 多配（DOSBox-X 只配 83，路徑超過 80 字元時寫穿到下一塊），刻意差異；
   `C:\OPTTEST.EXE` 兩者相同。
4. 配置（在程式塊之前）：
   - arena 非 nil：`pickBlock(envParags)`、`carveBlock(i, envParags)`，再呼叫 `placeChild` 配程式塊。
   - arena 為 nil：不呼叫 `initArena`（會改變既有行為）。環境 MCB 在 `freeSeg`，資料在 `freeSeg+1`，程式塊的 MCB 在 `freeSeg+1+envParags`；`placeChild` 改為接受起點偏移，
     容量檢查改為 `envParags+1+need > avail`。
   - 兩條路徑都以 `WriteMCB` 寫環境塊的客體 MCB；size 欄用實際配到的段數（`carveBlock` 在剩餘不足時整塊吃掉，讀 `arena[i].size`）。
   - 位置先後依配置策略（`AH=58h`）決定；預設 first fit 下環境塊在程式塊之前。
5. 擁有者：PSP 確定後（`psp := mcb+1`），把環境塊的 arena 項目 `owner` 與客體 MCB 的擁有者改成子 PSP。子行程非駐留結束時，`terminate` 既有的
   `releaseOwnedBy(d.curPSP)` 一併釋放；駐留（`AH=31h`、`int 27h`）不呼叫它，環境塊保留。arena 為 nil 時靠結束時 `d.freeSeg = f.freeSeg` 回收。
   不新增 `procFrame` 或 `procState` 欄位，不改狀態檔格式。程式塊本身的 arena 擁有者維持現行（父行程），不在本規格範圍。
6. 失敗與回收，任何失敗都不留下半配置的塊：
   - 正規化或環境掃描失敗：回錯誤 3 或 10，未配置任何塊。
   - 環境塊配置失敗：回錯誤 8。
   - 環境塊已配、程式塊配置失敗（`placeChild`）：釋放環境塊（`freeArenaSeg`），回錯誤 8。
   - `LoadProgramAt` 失敗：釋放環境塊與程式塊（現行程式此處已洩漏程式塊，一併修正）。
   - 環境內容在以上都成功之後才寫入。
7. `ExecRecord` 增加 `EnvSeg`（只在記錄中，不進 `procState`）。

`-empty-env` 產生的環境（`machine/loader.go`）為 `00 01 00 <path> 00`，偏移 0 的 word 不是雙 0；照 §1 第 2 項的掃描會越過路徑繼續找。`-empty-env` 與 `-exec-env`
並用的行為以測試記錄實測結果，不保證與真 DOS 一致。

## 3. 驗收

1. 選項關閉時：`go test ./internal/... ./cmd/probe ./session ./oracle` 與 `go test ./apps/...` 的結果與改動前逐行相同（實跑比對）。
2. 選項開啟的單元測試：
   - 父程式以環境段 0 執行子程式：子 PSP:2Ch 指向新塊；內容為父環境字串、雙 0、`01 00`、`C:\CHILD.EXE`、0；新塊 MCB 的擁有者為子 PSP、size 為實際段數；
   - 參數區塊給非 0 環境段時從該段掃描與複製；父 PSP:2Ch 為 0 時只寫一個 0 byte；
   - arena 為 nil 與非 nil 各一組；預設策略下子 PSP 在環境塊之後；
   - 洞的情境（比照 `exec_arena_test.go`、`exec_psptop_test.go`）：環境塊與程式塊落在不同洞時，結束後兩者都釋放；結束後 `largestFree` 回到 EXEC 前的值；
   - `AH=4Ch` 與 `int 20h` 兩條結束路徑都釋放；`AH=31h` 與 `int 27h` 駐留後環境塊仍屬子 PSP；
   - 失敗：路徑超過 127 回錯誤 3、環境掃描超過上限回錯誤 10、環境塊配不到回 8、程式塊配不到回 8 且環境塊已釋放、`LoadProgramAt` 失敗時兩塊都釋放；
   - `AL=03h` 不建環境；`-empty-env` 並用的實測行為記錄於測試；
   - `-exec-env` 旗標預設 false、開啟後欄位為 true（比照 `cmd/probe/pmflag_test.go`）。
3. 觸發案例（需原版，另附 probe 收據）：`SCCD.EXE -dos-files 30 -ext-mem-kb 8192 -no-ems -pm-profile -exec-env`（`workplace/root-nosound/`，冷開機，
   第 3400000000 步點 (160,45)）：印出 OPTTEST 環境塊尾端的路徑字串；OPTTEST 不再開 `C:\SCCD.EXE`；照實記錄 OPTTEST 的行為、後續 `EXEC` 紀錄、最後停點與原因。
4. `go build ./...`、`go vet ./internal/... ./cmd/probe` 全綠。
