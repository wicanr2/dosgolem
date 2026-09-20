# 194 — INT 33h AX=0014h：交換事件處理常式

狀態：**READY**（2026-09-20；證據見下）

## 問題

巫術7（wizardry7）的 VGA.DRV 在 driver init 的尾端以 `INT 33h AX=0014h`、
`CX=000Bh`（move＋左鍵下＋右鍵下）、`ES:DX=處理常式` 註冊滑鼠事件
（driver 檔 offset 0x1257 起：`B8 14 00 … B9 0B 00 BA D4 1E CD 33`）。
dosgolem 的 int33 沒有這個功能號，呼叫落進 Unimplemented
（probe 摘要出現 `int 33h AH=00 AL=14 ×1`），handler 沒被記下，
事件不投遞。遊戲的 UI 點擊只走事件閂鎖
（wizardry7 `docs/re/007`：驅動把 `INT 33h AX=0003` 的按鈕位元丟掉，
只認 handler 寫進 `cs:1ECD` 的閂鎖），結果是選單畫面對點擊無反應。

## 證據

- 註冊呼叫實測發生：載入 title-stable.state 跑到 ~#883M，int33 功能號
  統計出現 `AX=0014×1`，同時 driver init 的其餘呼叫（0000/0004/0007/0008）
  各 ×1，順序與 driver 檔內的指令序一致。
- 直接 poke 閂鎖（`cs:1ECD/1ECF/1ED1`）後選單畫面前進
  （wizardry7 `docs/re/007`）——閂鎖之後的整條鏈是通的。
- DOSBox-X 的參考實作：`src/ints/mouse.cpp:1970`
  （MS MOUSE v3.0+ - EXCHANGE INTERRUPT SUBROUTINES）。

## 規格

1. `case 0x0014`：把 `m.Handler` 設為 `ES:DX`、mask 設為 `CX`、`Set=true`
   ——與 AX=000Ch 相同的儲存動作。
2. **交換語意**：回傳舊值——`CX=舊 mask`、`DX=舊 offset`、`ES=舊 segment`；
   先前沒有登記時回 `0000:0000`＋mask 0（與 DOSBox-X 的初始零值一致）。
3. 事件投遞走既有 `fireMouseEventMickeys`（`docs/spec/013-mouse-event-callback`
   的佇列回呼），本規格不動它。

## 狀態持久化（2026-09-20 補）

`Mouse.Handler` 原本不在 `dos.SaveState`／`LoadState` 的抄寫清單裡：
存檔端用欄位字面值只挑六個欄位、讀檔端也只還原同六個。
症狀：從快照展開的機器「滑鼠有裝、點擊沒反應」，畫面完全正常
（巫術7 主選單實測）。修復：兩端補 `Handler`，並加
`TestStateKeepsMouseEventHandler`。

## 驗收

- contract test：第一次 `AX=14` 註冊後 `MouseEvent(EvLeftDown)` 回 true、
  handler 收到 `AX=mask, BX=buttons, CX=X*scale, DX=Y`（既有回呼欄位）；
  第二次 `AX=14` 註冊換新 handler 時，`CX/DX/ES` 回傳**前一次**的值。
- 巫術7端到端：從 title-stable.state 跑過 #883M（註冊點）之後，
  `-clicks` 點選單列中心，畫面前進離開 43,530 相位（wizardry7 的 #13）。
