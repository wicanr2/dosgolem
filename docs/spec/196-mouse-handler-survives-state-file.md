# 196 — 狀態檔要留住滑鼠事件常式與座標範圍

狀態：**READY**
日期：2026-09-30
前置：[`009-mouse-event-handler.md`](009-mouse-event-handler.md)（`AX=000Ch` 事件常式）、
[`004-dos-bios-services.md`](004-dos-bios-services.md)（`int 33h`）

---

## 1. 問題

`internal/dos/state.go` 的 `SaveState` 只留滑鼠的座標、按鍵與 `AX=5／6` 統計，
**沒有留 `Handler`（`AX=000Ch` 登記的事件常式）與 `MinX／MaxX／MinY／MaxY`
（`AX=7／8` 設的範圍）**，也沒有留 `PressAt／ReleaseAt`。

讀檔之後遊戲不會再登記一次事件常式（它只在開機時登記），於是：

- 注入的移動與按鍵不再呼叫事件常式。
- 靠事件常式畫游標、收點擊的程式看起來「滑鼠完全沒接上」：游標停在原地，
  點什麼都沒反應，而且不會有任何錯誤。

`oracle` 的記憶體內狀態（`oracle/state.go`）複製整個 `Mouse`，不受影響；
只有寫到檔案的 `-save-state`／`-load-state` 會壞。

## 2. 量到的證據（已證實）

《魔眼殺機二》中文版 `START.EXE`：

- 從頭跑到讀檔進 LEVEL4（第 399,999,000 步）時點擊正常。
- 在該步 `-save-state`，再 `-load-state` 後於第 401,000,000 步點左轉箭頭
  （320×200 的 `27,136`）與法師的法術書：到第 412,000,000 步畫面與游標位置都不變。

## 3. 修法

- `SaveState` 另存 `Handler`、`MinX／MaxX／MinY／MaxY`、`PressAt／ReleaseAt`。
- `LoadState` 還原這些欄位。
- 觀測紀錄（`Polls`、`Sets`、`Calls`、`PressReads`、`Events`、`PressQ`）照舊不存。
- gob 對新增欄位相容：舊狀態檔讀進來這些欄位是零值（與修正前行為相同），
  不必升版號。

## 4. 驗收

- 單元：登記事件常式並設範圍後存檔，讀進另一台機器，`Handler`、範圍、
  `PressAt／ReleaseAt` 與存檔前一致。
- 實跑：EOB2 重新存讀檔後，點左轉箭頭畫面會轉向。
