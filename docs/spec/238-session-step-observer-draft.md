# 238 — Session 逐步觀測器（唯讀視圖）

狀態：**READY**（2026-09-26 獨立審查後修訂）
日期：2026-09-26
前置：[`235-sealed-owner-private-boot`](235-sealed-owner-private-boot-draft.md)、
[`236-bootroot-composition-preflight`](236-bootroot-composition-preflight-draft.md)、
`machine.RunUntilObserved`（`internal/machine/probe.go`，DRAFT 接縫）

---

## 1. 為什麼要做

`session.Owner.Advance` 用裸的 `Machine.RunUntil`，沒有任何觀測點，sealed session
跑不出覆繪。所有已 CONFORMED 的 Buck 覆繪收據都出自 receipt runner 的模型：
每一步執行前讀 CS:IP、SS:SP 與堆疊參數，判斷 dispatcher 進入、guarded return、
清除呼叫；另以 `Machine.ObserveVideoWrites` 在 A000 寫入前做失效。這個模型不需要
Oracle 的 `OnCall` 動態 hook。

本規格把同一個模型放進 sealed Owner：Owner 在私有開機時安裝觀測器，觀測器只拿得到
唯讀視圖，拿不到 machine、DOS 或任何可變資源。

## 2. 契約

1. **型別**（`session` 套件）：

   ```go
   type StepObserver interface {
       BeforeStep(v StepView) error                  // 每次 Step 嘗試前
       VideoWrite(w machine.VideoWrite)              // A000 寫入前（值型）
       Frame(indexed []byte, palette [256][3]uint8)  // 每次垂直回掃，皆為複本
   }
   ```

   `StepView` 是 session 套件的具名型別，欄位全不匯出，只提供唯讀方法：
   `Steps()`、`CS()`、`IP()`、`DS()`、`ES()`、`SS()`、`AX()`、`BX()`、`CX()`、
   `DX()`、`SP()`、`BP()`、`SI()`、`DI()`、`Flags()`、`Palette()`、`Read8(addr)`、
   `Read16(addr)`。沒有寫入方法，也不回傳內部指標。
   `Read8`／`Read16` 走 `machine.Peek8`：不載入平面模式 VGA latch、不觸發讀取監看；
   平面模式的 A0000 視窗讀回 0。
   上一步的 CS:IP 與 opcode（劇情頁 glyph return 驗證要用）由觀測器在前一次
   `BeforeStep` 自行記錄，StepView 不提供。

2. **安裝**：`Config.Observer` 在 `New` 時提供，Owner 私有持有；觀測器本身不得持有
   machine、DOS、panel 或任何 bridge；`BootOriginal`
   成功、第一個 instruction 之前，觀測器已生效。之後不能替換或移除。
   `Config.Observer` 為 nil 時，行為與現行完全相同（仍用 `RunUntil`）。

3. **Advance**：有觀測器時改呼叫 `RunUntilObserved(nil, budget, observe)`；
   Owner 不安裝斷點，也不給 predicate。`observe` 先檢查 `DOS.Exited`：已結束就
   停止回合，由 Advance 歸為 `ProgramStopped`（與 runner 的 `!d.Exited` 一致）；
   否則以當次回合的 `StepView` 呼叫 `BeforeStep`。回合期間以 `ObserveVideoWrites`
   與 `SetOnFrame` 轉發寫入與畫格，回合結束（含錯誤路徑）一律解除轉發。
   沒有觀測器的既有路徑仍不逐步檢查 Exited，這是既有限制，本規格不改。

4. **視圖時效**：每個 `StepView` 綁定回合序號與 step 數；回合結束後或 Step 已前進，
   舊視圖的讀取回傳零值並記錄一次觀測器誤用，下一次 `BeforeStep` 回報
   `ObserverFault`。

5. **錯誤**：`BeforeStep` 回傳 error 或 panic（在 callback 邊界 recover），
   當次 Step 不執行，Owner 鎖存 first fault，收據原因為 `ObserverFault`，
   之後 `Deliver`／`Advance` 零新步，`Close` 一次。
   `VideoWrite` 與 `Frame` 在 Step 內同步呼叫，無法中途中止該 Step：session 在轉發
   函式內 recover，把 panic 鎖存，當前 Step 照常完成；下一次 `BeforeStep` 前先查
   鎖存值並停止。觀測器錯誤與原版程式錯誤分開傳回，Advance 分別歸為
   `ObserverFault` 與 `OriginalFault`。
6. **執行緒**：Owner 與觀測器只在單一前端 goroutine 使用（沿用規格 019）；
   `StepView` 的時效檢查不是跨 goroutine 保護。

7. **決定性**：觀測器只讀；同 state、同輸入、有無觀測器，machine 的 steps、記憶體、
   indexed、palette、DOS 狀態必須相同。

## 3. 不做什麼

- 不處理畫面 `Snapshot`、覆繪層合成或倍率切換（屬後續 B2／B3）。
- 不接 Oracle、不提供 `OnCall`、中斷點或 stub。
- 不改 `Config.Observer` 為 nil 時的任何行為。

## 4. 驗收

1. 合成測試：觀測器看到的 CS:IP 序列等於逐步執行的序列；寫入轉發只在回合內；
   舊視圖讀取回零並導致下一步 `ObserverFault`；error 與 panic 都零步鎖存、
   Close 一次。
2. 原版：同一冷開機序列，有觀測器（只計數）與無觀測器各跑 N 回合，
   steps、記憶體、indexed、palette 雜湊相同；觀測器計到的 step 數等於實際步數。
