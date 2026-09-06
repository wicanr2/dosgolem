# 133 — 386 OR register8 immediate8

狀態：**CONFORMED**
日期：2026-09-06
前置：[`RE 093`](../re/093-fd2-open-flags-binary-mode.md)

- 支援無 prefix 的 `80 /1`、`mod=3`：`OR r8,imm8`。
- r8 依 ModRM 選擇低／高 byte，只寫回該 byte，保留所屬暫存器其他位元，
  並依 byte 結果更新邏輯旗標。
- 其他暫存器與記憶體不變。
- operand16、segment override、repeat 與其他新 `80` 形狀維持失敗即關閉。

驗收：合成測試覆蓋低 byte 與高 byte；固定雜湊 FD2 必須自然執行
`0x36E6F` 至 `0x36E72`，確認 DL 變為原值 OR `0x40`。

驗收收據（2026-09-06）：`TestOrRegister8Immediate8` 覆蓋 DL 與 AH；
`TestFD2BuildsBinaryOpenFlags` 從固定原版 LE entry 自然執行至 `0x36E72`，
確認 DL 變為原值 OR `0x40`。
