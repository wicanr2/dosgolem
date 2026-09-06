# 136 — 386 MOV byte base+disp8 register

狀態：**CONFORMED**
日期：2026-09-06
前置：[`RE 096`](../re/096-fd2-open-saves-normalized-mode.md)

- 支援無 prefix 的 `88 /r`、`mod=1`、非 SIB：
  `MOV byte ptr [base+sign-extended disp8],r8`。
- EBP base 使用 SS；其他 base 使用 DS。只寫入一個 byte，不修改旗標、base
  或來源暫存器。
- operand16、segment override、repeat、SIB 與其他新形狀維持失敗即關閉。

驗收：合成測試覆蓋 DS／SS、低／高 byte 與負位移；固定雜湊 FD2 必須
自然執行 `0x36EE7` 至 `0x36EEA`，確認 `[SS:EBP-4]` 等於原 AL。

驗收收據（2026-09-06）：`TestStoreRegister8ToBaseDisp8` 驗證 SS、負位移、
來源及旗標不變；`TestFD2SavesNormalizedOpenMode` 從固定原版 LE entry
自然執行至 `0x36EEA`，確認 `[SS:EBP-4]` 等於原 AL。
