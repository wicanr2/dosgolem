# 137 — 386 OR register8 base+disp8

狀態：**CONFORMED**
日期：2026-09-06
前置：[`RE 097`](../re/097-fd2-sopen-builds-dos-mode.md)

- 支援無 prefix 的 `0A /r`、`mod=1`、非 SIB：
  `OR r8,byte ptr [base+sign-extended disp8]`。
- EBP base 使用 SS；其他 base 使用 DS。讀取來源 byte，OR 後只寫回目的
  r8，更新 byte 邏輯旗標；base 與記憶體不變。
- 所有 prefix、SIB 與其他 ModRM 形狀維持失敗即關閉。

驗收：合成測試覆蓋 DS／SS、低／高 byte 與負位移；固定雜湊 FD2 必須
自然執行 `0x3CD67` 至 `0x3CD6A`，確認 AL 合併 `[SS:EBP+0x1C]`。

驗收收據（2026-09-06）：`TestOrRegister8BaseDisp8` 驗證 SS、負位移、
來源不變與 byte 旗標；`TestFD2BuildsDOSOpenMode` 從固定原版 LE entry
自然執行至 `0x3CD6A`，確認 AL 合併 `[SS:EBP+0x1C]`。
