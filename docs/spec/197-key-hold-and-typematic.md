# 197 — 按住按鍵：定時掃描碼事件與 typematic 重複

狀態：**READY**
日期：2026-09-17
前置：[`014-hardware-keyboard.md`](014-hardware-keyboard.md)（IRQ1、埠 60h）、[`185-keyboard-trace-and-keypad-names.md`](185-keyboard-trace-and-keypad-names.md)

---

## 1. 問題

`PushKey`／`-press` 只能送「按下＋放開」一對事件，而且進 FIFO 佇列，由 `KeyEvery` 節流：

- **按不住**：按下與放開之間只隔 `KeyEvery`（82,500 道指令），沒辦法表達「按住 2 秒」。
- **時間不準**：排在佇列後面的鍵要等前面的送完，排得密時實際送出時間一路往後拖（案例：150 次按鍵戰鬥結束時佇列還剩 107 個事件）。

會用「按鍵狀態表」判斷的遊戲（IRQ1 處理常式在按下時設旗標、放開時清掉，主迴圈看旗標），
在戰鬥、移動這類操作上需要真的按住。案例：`psychic_war_cht` 第一場戰鬥（其 `docs/re/008`）。

## 2. 真機行為

- 按住時鍵盤送出按下碼；超過 **typematic 延遲**後，以 **typematic 速率**重複送按下碼；放開時送一次放開碼（bit 7）。
- BIOS 開機預設：延遲 **500 ms**、速率 **10.9 次／秒**（`INT 16h AH=03h` 可改，本規格不接）。

## 3. 規格

### 3.1 定時事件

- `Machine.ScheduleKey(step uint64, scan uint8, brk bool)`：在「第 `step` 道指令或之後第一個能送的時機」送出。
- 定時事件依 `step` 排序（同一步依加入順序），**不受 `KeyEvery` 節流**，也不排在 FIFO 佇列後面。
- 能送的條件與 FIFO 佇列相同：IF 開、8259 沒遮蔽 IRQ1、`INT 09h` 向量不是 stub；不能送時留到下一道指令再試，不丟棄。
- 同一道指令只送一個事件（一次 IRQ1）；FIFO 佇列與定時事件都有到期的，**定時事件優先**。
- 埠 `64h` 狀態：FIFO 佇列非空，或有已到期的定時事件時，回「輸出緩衝區有資料」。

### 3.2 按住

`Machine.HoldKey(scan uint8, from, duration uint64, typematic bool)`：

- 第 `from` 步送按下碼。
- `typematic` 為真時，從 `from ＋ 延遲` 起每隔 `週期` 再送一次按下碼，直到放開前。
  延遲與週期以 `StepsPerSecond()` 換算：500 ms、`1 ÷ 10.9` 秒。
- 第 `from ＋ duration` 步送放開碼。

### 3.3 probe

`-hold "<鍵>@<起點>+<長度>[,…]"`：

- 鍵名同 `-press`（名稱或兩位十六進位掃描碼）。
- 起點、長度可寫指令數，或加 `ms` 後綴以 `StepsPerSecond()` 換算。
- `-hold-typematic`（預設 true）。

## 4. CPU 速度的注意事項

`StepsPerSecond()` 是 dosgolem 的時間模型（約每秒 11.58 M 道指令），不是原機。
遊戲若用「空迴圈次數」計時（例：`mov cx,N` ＋ `loop $`），這類延遲在 dosgolem 上會比原機短，
與計時器中斷（以指令數換算）的相對速度也會不同。`ms` 換算只保證「在 dosgolem 自己的時間軸上」一致；
要讓人玩起來速度正確，屬於前端節拍（牆上時間對齊）的範圍，不在本規格。

## 5. 驗收

1. 定時事件在指定步數送出，不受 `KeyEvery` 影響（兩個相隔 10 道指令的事件都能送出，間隔 ≥ 10）。
2. IF 關閉期間到期的事件不丟：開 IF 後送出。
3. `HoldKey` 不開 typematic：只有一個按下與一個放開，放開步數 ＝ from ＋ duration（容許因 IF 延後）。
4. `HoldKey` 開 typematic、按住 2 秒：按下碼次數 ＝ 1 ＋ ⌊(2000 − 500) ÷ (1000 ÷ 10.9)⌋ ＋ 1（±1）。
5. 同時有 FIFO 與定時事件到期時，先送定時事件。
6. 反向對照：拿掉「定時事件不受節流」，第 1 項要失敗。
