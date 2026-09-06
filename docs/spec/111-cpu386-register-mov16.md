# 111 — 386 暫存器對暫存器 16 位元 MOV

狀態：**CONFORMED**
日期：2026-09-06
前置：[`RE 071`](../re/071-fd2-ail-pack-real-mode-vector.md)

- 支援 32 位元預設程式碼中的 `66 8B /r`，限 `mod=3` 暫存器來源。
- 將來源暫存器低 16 位元寫入目的暫存器低 16 位元，保留目的高 16 位元。
- 不修改旗標。
- segment override、記憶體 ModRM 與其他未列形狀維持失敗即關閉。

驗收：合成測試以非零高半部驗證低 16 位元複製、高半部保留及旗標不變；
固定雜湊 FD2 必須由 LE entry 自然執行 `0x3E9B7 mov cx,dx` 至 `0x3E9BA`。

驗收收據（2026-09-06）：`TestRegisterMOV16` 與
`TestFD2PacksTimerRealModeVector` 通過；固定原版由 LE entry 自然到達
`0x3E9BA`，完整回歸命令見能力矩陣當前收據。
