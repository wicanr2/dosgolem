# 091 — 386 EAX 與 immediate dword

狀態：**CONFORMED**
日期：2026-09-06
前置：[`RE 059`](../re/059-fd2-ail-dpmi-lock-region.md)

- 支援無前綴 opcode `25` 的 `AND EAX,imm32`。
- 寫回 EAX，依 32 位邏輯結果設定 SF/ZF/PF，清除 CF/OF/AF。
- immediate 不完整與所有前綴失敗即關閉。
- 固定 FD2 在 `0x362B0` 以 `25 FFFF0000` 取出線性位址低 16 位。

驗收：單元測試覆蓋結果與旗標；固定雜湊 FD2 必須由 LE entry 自然經過
`0x362B0`。

驗收收據（2026-09-06）：`TestAndEAXImmediate32` 與固定雜湊 DPMI 路徑通過。
