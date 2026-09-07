# 128 — 386 CMP register immediate32

狀態：**CONFORMED**
日期：2026-09-06
前置：[`RE 088`](../re/088-fd2-allocfp-table-bound.md)

- 支援無 prefix 的 `81 /7`、`mod=3`：`CMP r32,imm32`。
- 以 32 位減法更新比較旗標，但不寫回暫存器。
- register 由 ModRM 的 r/m 欄位選擇；immediate 採完整 32 位值。
- operand16、segment override、repeat 與其他新 `81` 形狀維持失敗即關閉。

驗收：合成測試覆蓋小於、等於及大於；固定雜湊 FD2 必須從 LE entry
自然執行 `0x3D84F` 至 `0x3D855`，確認 EBX 不變且旗標等於
`EBX-0x52A48` 的比較結果。

本規格不授權新的 C runtime FILE table 或 host 檔案映射行為。

驗收收據（2026-09-06）：`TestCompareRegisterImmediate32` 覆蓋小於、等於與
大於且確認不寫回；`TestFD2ComparesAllocFPTableBound` 從固定原版 LE entry
自然執行至 `0x3D855`，確認 EBX 不變與 CF／ZF 比較結果。
