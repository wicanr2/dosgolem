# 134 — 386 OR base+disp8 dword register

狀態：**CONFORMED**
日期：2026-09-06
前置：[`RE 094`](../re/094-fd2-open-applies-mode-flags.md)

- 支援無 prefix 的 `09 /r`、`mod=1`、非 SIB：
  `OR dword ptr [base+sign-extended disp8],r32`。
- EBP base 使用 SS；其他 base 使用 DS。讀取、OR、寫回同一 dword，並更新
  32 位邏輯旗標；來源暫存器與 base 不變。
- 所有 prefix、SIB 與其他 ModRM 形狀維持失敗即關閉。

驗收：合成測試覆蓋 DS／SS 與負位移；固定雜湊 FD2 必須自然執行
`0x36ED2` 至 `0x36ED5`，確認 `[DS:EBX+0x0C]` 合併 EAX。

驗收收據（2026-09-06）：`TestOrBaseDisp8RegisterDword` 覆蓋 DS／SS、
負位移、寫回與旗標；`TestFD2AppliesOpenModeFlags` 從固定原版 LE entry
自然執行至 `0x36ED5`，確認 FILE record dword 合併 EAX。
