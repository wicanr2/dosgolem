# 125 — 386 暫存器減 imm32

狀態：**CONFORMED**
日期：2026-09-06
前置：[`RE 085`](../re/085-fd2-mdi-ini-stack-frame.md)

- 支援無 prefix 的 `81 /5`、`mod=3`：`SUB r32,imm32`。
- 以 32 位元模數算術寫回目的暫存器，依既有 `sub32` 更新旗標。
- operand16、segment、repeat、記憶體 ModRM 與其他 group 維持失敗即關閉。

驗收：合成測試覆蓋一般減法、借位與旗標；固定雜湊 FD2 必須由 LE entry
自然執行 `0x43EF2 sub esp,118h` 至 `0x43EF8`，確認 ESP 精確減少 `0x118`。

驗收收據（2026-09-06）：`TestSubtractRegisterImmediate32` 覆蓋一般減法、
借位與旗標；`TestFD2AllocatesMDIINIStackFrame` 由固定原版 LE entry 自然
執行至 `0x43EF8`，確認 ESP 精確減少 `0x118`。
