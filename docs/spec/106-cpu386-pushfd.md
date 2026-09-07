# 106 — 386 PUSHFD

狀態：**CONFORMED**
日期：2026-09-06
前置：[`RE 067`](../re/067-fd2-ail-interrupt-setup-entry.md)

- 支援無 prefix 的 opcode `9C`（32 位 `PUSHFD`）。
- 將目前 dosgolem 已建模的 EFLAGS dword 寫入 SS:ESP-4，成功後才更新 ESP。
- ESP 下溢、descriptor 不可寫或範圍不足時，ESP、旗標與記憶體保持不變。
- operand-size、segment override 與 repeat prefix 維持失敗即關閉（fail-closed）。
- 固定 FD2 在 `0x3E930` 保存 AIL 中斷設定前的旗標，對稱 consumer 是
  `0x3EA14` 的 `POPF`；本規格不提前實作後者。

驗收：單元測試覆蓋成功與不可寫失敗；固定雜湊 FD2 必須由 LE entry 自然
執行至 `0x3E931`，確認堆疊頂等於步進前 EFLAGS 且 ESP 減四。

驗收收據（2026-09-06）：`TestPushFlags32` 與固定原版
`TestFD2SavesFlagsBeforeAILInterruptSetupWhenProvided` 通過；後者自然執行至
`0x3E931`，確認堆疊頂等於原 EFLAGS 且 ESP 減四。
