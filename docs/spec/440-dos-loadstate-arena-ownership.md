# 440 — 讀檔後重建 arena 區塊擁有者與子行程程式塊

狀態：**DRAFT**（第 1 版）
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

1. **擁有者**：對每個 `free=false` 的 arena 塊，讀客體 MCB（段 `b.seg`）偏移 1 的 word 當擁有者；值等於 `PSPSeg` 時設為 0（`syncMCB` 的反向）。`free=true` 的塊擁有者為 0。
   讀取走實體層（`ReadPhys16`，MCB 在 1 MB 以下；與載入器寫 MCB 的方式一致）。
2. **子行程程式塊**：`procStack[i]` 是啟動第 i 個子行程時存下的父行程框架；該子行程的 PSP 為 `procStack[i+1].psp`（i 不是最後一個時），否則為 `curPSP`。
   若 arena 中有 `seg == 子 PSP − 1` 且 `free=false` 的塊，`procStack[i].arenaSeg` 設為該 `seg`；否則為 0（舊 `freeSeg` 路徑，與現行相同）。
3. 讀檔前的狀態檔不需任何轉換；新舊 dosgolem 寫出的狀態檔位元組不變（`DOSState` 錨點不動）。
4. 擁有者讀回值不屬於任何現存 PSP 時照用（不驗證）；客體程式自行改寫 MCB 的情形不在範圍。

## 3. 驗收

1. 錨點測試（`internal/state/baseline_anchor_test.go`）全部常數不變；`go test ./apps/...` 與改動前相同。
2. `internal/dos` 新測試（不需原版檔，沿用 `exec_arena_test.go` 的 `newTest`、`writeChild`、`execChild`、`call`）：
   - 父行程先 `AH=48h` 建立 arena；子行程 `AH=48h` 拿一塊後停住；在子行程執行中 `SaveState`，於同一台機器以新的 `DOS` `LoadState`；讓子行程 `AH=4Ch` 結束；再 EXEC 另一支
     子程式成功（CF=0），`largestFree` 回到子行程啟動前的值。對照組不存讀檔，結果相同。
   - 往返：存讀後每個 arena 塊的 `owner`、每個框架的 `arenaSeg` 與存檔前相同（arena 路徑與 nil 路徑各一組；兩層巢狀子行程一組；開 `ExecEnvCopy` 時環境塊擁有者一組）。
   - 擁有者為 0 的已用塊（父行程頂層配置）讀回仍為 0。
3. 突變自檢：不重建擁有者、不重建 `arenaSeg`、`PSPSeg` 不轉回 0，各至少一個測試失敗。
4. 觸發案例（需原版，另附 probe 收據）：從冷開機在 MENU 執行中存檔（`-exec-env`），`-load-state` 接著跑並點選 PLAY STRIKE COMMANDER，SCCD 能執行 OPTTEST.EXE。
5. `go build ./...`、`go vet ./internal/... ./cmd/probe`、`go test ./internal/... ./cmd/probe ./session ./oracle` 全綠。
