# 440 — 讀檔後重建 arena 區塊擁有者與子行程程式塊

狀態：**READY**（第 2 版，依審查修正擁有者重建規則）
日期：2026-10-16
前置：[`009-memory-allocator`](009-memory-allocator.md)、[`439-exec-child-environment`](439-exec-child-environment.md)

---

## 1. 為什麼要做

`cmd/probe -load-state` 從 MENU.EXE 執行中的存檔接著跑，MENU 以 `AH=4Ch` 結束後，SCCD 以 `AX=4B00h` 執行 `opttest.exe` 失敗（「Error loading executable」）；
冷開機不存檔則成功（`strike_commander` WORKLOG 2026-10-16）。

原因（強推論，程式碼閱讀）：`internal/dos/state.go` 的 `blockState` 只存 `Seg`、`Size`、`Free`，`procState` 不存 `arenaSeg`。讀檔後 arena 每塊的 `owner`
與每個行程框架的 `arenaSeg` 都是 0。子行程非駐留結束時 `terminate` 以 `releaseOwnedBy(curPSP)`（`exec.go` 約 523 行）與 `freeArenaSeg(f.arenaSeg)`（約 544–549 行）
回收記憶體，兩者都失效，子行程的塊一直標為已用，下一次 `spawn` 的 `placeChild` 找不到自由塊而回錯誤 8。

直接在 `blockState`、`procState` 加欄位會改變 `DOSState` 的 gob 位元組；`internal/state/baseline_anchor_test.go` 註解寫明這個常數不得再動。所以本規格不改狀態檔格式，
改在讀檔時從已存的資料重建這兩項：客體記憶體（`Mem`）裡的 MCB 鏈有擁有者欄位，`spawn` 以 `WriteMCB(mcb, false, psp, …)` 寫入子 PSP，`AH=48h` 等路徑經 `syncMCB`
寫入各塊擁有者（擁有者為 0 的已用塊寫成 `PSPSeg`）。

## 2. 規則

`DOS.LoadState` 在還原 arena 與 `procStack` 之後、回傳之前：

1. **擁有者**：對每個 `free=false` 的 arena 塊，讀客體 MCB（段 `b.seg`）偏移 1 的 word 當擁有者，直接取值（不做任何轉換；`carveBlock` 的擁有者來自 `blockOwner()`，
   頂層程式配置的塊為 `PSPSeg`，客體與 Go 端一致）。讀到 0（客體改壞 MCB）時補成 `PSPSeg`。`free=true` 的塊擁有者為 0。讀取走實體層（`ReadPhys16`）。
2. **子行程程式塊**：`procStack[i]` 是啟動第 i 個子行程時存下的父行程框架（`spawn` 先存 `f.psp = curPSP` 再壓疊，之後才把 `curPSP` 設為子 PSP）；該子行程的 PSP 為
   `procStack[i+1].psp`（i 不是最後一個時），否則為 `curPSP`。若 arena 中有 `seg == 子 PSP − 1` 且 `free=false` 的塊：
   - `procStack[i].arenaSeg` 設為該 `seg`（等於 `spawn` 的 `mcb`；439 的環境塊另有 MCB，不影響）；
   - 該塊的擁有者設為 `procStack[i].psp`（`placeChild` 配置時 `blockOwner()` 的值，即父 PSP），不套用第 1 項的客體值。原因：`spawn` 在客體 MCB 寫的是子 PSP，
     子行程第一次 `AH=48h/49h/4Ah` 時 `syncMCB` 才以 Go 端的父 PSP 覆寫；直接取客體值會讓續跑後的客體位元組與不存讀的執行不同。
   否則 `arenaSeg` 為 0（舊 `freeSeg` 路徑）。子行程自己以 `AH=49h` 釋放程式塊的極端情形，原值非 0、重建為 0，行為略有不同，可接受。
3. **佇列推出的底層行程**：`len(procStack)==0` 且 arena 中有 `seg == curPSP − 1` 且 `free=false` 的塊時，`queuedSeg` 設為該 `seg`（頂層初始程式的 `PSPSeg−1` 不在
   arena 內，不會誤判）。該塊擁有者照第 1 項取客體值；`spawnQueued` 配置時的舊 `curPSP` 無法從存檔推得，未 sync 過時重建值為新 PSP，與不存讀的執行可能不同（已知限制）。
4. `SaveState` 不得有任何改動；讀檔前的狀態檔不需任何轉換；新舊 dosgolem 寫出的狀態檔位元組不變（`DOSState` 錨點不動）。
5. 擁有者讀回值不屬於任何現存 PSP 時照用（不驗證）；客體程式自行改寫 MCB 的情形不在範圍。子行程程式塊的客體擁有者在 `syncMCB` 後由子 PSP 變為父 PSP，屬既有的
   客體可見不一致，不在本規格範圍。

## 3. 驗收

1. 錨點測試（`internal/state/baseline_anchor_test.go`）全部常數不變；`go test ./apps/...` 與改動前相同；另加 `save → load → save` 的 `DOSState` 位元組相同。
2. `internal/dos` 新測試（不需原版檔，沿用 `exec_arena_test.go` 的 `newTest`、`writeChild`、`execChild`、`call`）：
   - 父行程先 `AH=48h` 建立 arena；子行程 `AH=48h` 拿一塊後停住；在子行程執行中 `SaveState`，於同一台機器以新的 `DOS` `LoadState`；讓子行程 `AH=4Ch` 結束；再 EXEC 另一支
     子程式成功（CF=0），`largestFree` 回到子行程啟動前的值。對照組不存讀檔，結果相同。
   - 往返：存讀後每個 `free=false` 的 arena 塊的 `owner`、每個框架的 `arenaSeg`、`queuedSeg` 與存檔前相同（自由塊略過；arena 路徑與 nil 路徑各一組；兩層巢狀子行程
     一組；子行程存檔前未 sync 與已 sync 各一組；開 `ExecEnvCopy` 時環境塊擁有者一組）。
   - 頂層程式配置的塊擁有者讀回仍為 `PSPSeg`。
   - 測試順序仿真實流程：`m.SaveState`、新機器 `m2.LoadState`、新 `DOS` `Install` 後 `LoadState`（`internal/dos` 不 import `internal/state`）。
   - 斷言不變式：`procStack[i+1].psp`（或 `curPSP`）等於第 i 個子行程的 PSP。
3. 突變自檢：不重建擁有者、不重建 `arenaSeg`、子行程程式塊擁有者改取客體值、不重建 `queuedSeg`，各至少一個測試失敗。
4. 觸發案例（需原版，另附 probe 收據）：從冷開機在 MENU 執行中存檔（`-exec-env`），`-load-state` 接著跑並點選 PLAY STRIKE COMMANDER，SCCD 能執行 OPTTEST.EXE。
5. `go build ./...`、`go vet ./internal/... ./cmd/probe`、`go test ./internal/... ./cmd/probe ./session ./oracle` 全綠。

## 4. 實作收據（2026-10-16）

commit：`ae96a8f`。

- 第 1、2、3、5 項：容器內 `go test -count=1 ./internal/... ./cmd/probe ./session ./oracle` 13 個套件 ok；SingleStepTests 316 PASS、0 SKIP；`gofmt`、`go vet` 無輸出；
  `go test -v ./apps/...` 改動前後結果相同（`apps/phantasie/cmd/phantasie-play` 前後都因無顯示器失敗）；錨點常數不變；`SaveState` 無改動。
- 新測試 `internal/dos/loadstate_arena_test.go`：存讀與不存讀兩組結果相同（arena、第二支子程式 PSP、常規記憶體 0–A0000h）；往返 6 組；每次存讀驗 save→load→save 位元組相同。
- 突變自檢：不重建擁有者、不重建 `arenaSeg`、子行程程式塊擁有者改取客體值、不重建 `queuedSeg`，各至少一個測試失敗。
- 實作選擇：`queuedSeg` 不命中第 3 項時清為 0；測試照 §3 第 2 項第 4 點用新機器。
- 已知限制：佇列推出的底層行程在存檔時正執行子行程，讀檔後 `queuedSeg` 為 0，該程式塊只在客體 MCB 擁有者等於其 PSP 時經 `releaseOwnedBy` 回收。
- 第 4 項觸發案例另附收據。
