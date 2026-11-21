# 449 — 執行中改指令時脈

狀態：**READY**（前端契約；來源見 §2）
日期：2026-10-22
前置：[`242-machine-clock-scale-draft`](242-machine-clock-scale-draft.md)（開機前的時脈比例）、[`445-real-mode-sb-single-dma`](445-real-mode-sb-single-dma.md)

---

## 1. 規則

`Machine.SetInstructionClockPercent(p)`，p 為 1–3200，可在任兩道指令之間呼叫：

| 項目 | 行為 |
|---|---|
| 名目速度 | `InstructionsPerSecond` 變成 p% |
| IRQ0、VGA 回掃、鍵盤間隔 | 依 p 重算週期；到下一次事件的剩餘指令數依新舊週期等比換算（整數四捨五入），不重設相位 |
| SB 單次 DMA | 進行中的傳輸：每個取樣的信用門檻與已累積信用依 p／舊 p 換算，取樣率在真實時間上不變 |
| 拒絕 | 週期時鐘、呼叫端釘住的 IRQ0（`IRQ0Pinned`）、自訂的 IRQ0 基準、p 超出範圍 |
| 不做 | 不多跑指令、不加快音訊晶片；前端依名目速度推進，並可依主機負載自行降低 |

前端以 p＝100×倍率實作 CPU 倍率：遊戲內的計時器、回掃與語音在真實時間上維持原速，只有每秒可執行的指令變多。

## 2. 來源

§1 前四列與測試 `TestLiveClockScalesDevicePeriodsAndPreservesPhase`、`TestLiveClockRejectsOtherTimingContracts`
移植自 mm3 專案的 `tools/dosgolem-live-clock.patch`（同一位作者的 dosgolem 補丁，原用於魔法門三的 CPU 1×–32× 工具列）。
SB 一列與 `TestLiveClockKeepsSBSampleRate` 為本規格新增。

## 3. 驗收

1. `internal/machine/live_clock_test.go` 三項測試。
2. `go test ./...` 全綠（Xvfb）。
