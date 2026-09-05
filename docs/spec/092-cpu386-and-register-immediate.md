# 092 — 386 暫存器與 immediate dword

狀態：**CONFORMED**
日期：2026-09-06
前置：[`RE 059`](../re/059-fd2-ail-dpmi-lock-region.md)

- 支援無前綴 `81 /4`、`mod=3` 的 `AND r32,imm32`。
- 寫回目的暫存器，依 32 位邏輯結果設定 SF/ZF/PF，清除 CF/OF/AF。
- 其他 81 group、記憶體形狀與前綴失敗即關閉。
- 固定 FD2 在 `0x362C2` 以 `81 E2 FFFF0000` 取出線性長度低 16 位。

驗收：單元測試覆蓋任意目的暫存器與旗標；固定雜湊 FD2 必須由 LE entry
自然經過 `0x362C2`。

驗收收據（2026-09-06）：`TestAndRegisterImmediate32` 與固定雜湊 DPMI 路徑通過。
