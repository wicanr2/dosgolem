# 138 — 386 MOV immediate dword base+disp8

狀態：**CONFORMED**
日期：2026-09-06
前置：[`RE 098`](../re/098-fd2-sopen-initializes-handle.md)

- 支援無 prefix 的 `C7 /0`、`mod=1`、非 SIB：
  `MOV dword ptr [base+sign-extended disp8],imm32`。
- EBP base 使用 SS；其他 base 使用 DS。寫入完整 dword，不修改旗標或 base。
- prefix、SIB、非 `/0` 與其他新形狀維持失敗即關閉。

驗收：合成測試覆蓋 DS／SS 與負位移；固定原版必須自然執行
`0x3CD6A` 至 `0x3CD71`，確認 `[SS:EBP-8]` 等於 `0xFFFFFFFF`。

驗收收據（2026-09-06）：`TestStoreImmediateDwordBaseDisp8` 驗證 SS、
負位移與旗標不變；`TestFD2InitializesDOSOpenHandle` 從固定原版 LE entry
自然執行至 `0x3CD71`，確認 `[SS:EBP-8]` 等於 `0xFFFFFFFF`。
