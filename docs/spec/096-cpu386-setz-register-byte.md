# 096 — 386 依 ZF 設定 byte 暫存器

狀態：**CONFORMED**
日期：2026-09-06
前置：[`RE 059`](../re/059-fd2-ail-dpmi-lock-region.md)、
[`spec 095`](095-cpu386-cmp-stack-disp8-immediate.md)

- 支援無前綴 `0F 94 /r`、`mod=3` 的 `SETZ r8`。
- `ZF=1` 寫入 1，否則寫入 0；不修改旗標與目的 byte 以外的暫存器位元。
- 記憶體 ModRM 與前綴失敗即關閉。
- 固定 FD2 在 `0x362E5` 將 DPMI CFLAG 是否為 0 轉成 AL 成功值。

驗收：單元測試覆蓋 ZF 兩種狀態與 high-byte 目的；固定雜湊 FD2 由 LE entry
自然經過 `0x362E5`。

驗收收據（2026-09-06）：`TestSETZRegisterByte` 與固定雜湊 DPMI 成功返回路徑通過。
