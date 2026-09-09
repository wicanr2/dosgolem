# 079 — 386 暫存器 SUB

狀態：**CONFORMED**
日期：2026-09-06
前置：[`RE 056`](../re/056-fd2-watcom-cmain.md)

- 支援無前綴 `29 /r` 且 `mod=3` 的 32 位元 `SUB r/m32,r32`。
- 沿用完整 32 位元 subtraction 旗標；來源暫存器不變。
- 記憶體 ModRM 與所有前綴維持失敗即關閉。
- 固定 FD2 在 `0x45D66` 執行 `sub esp,eax`，保留 Watcom `__CMain` 的 stack
  配置路徑。

驗收收據（2026-09-06）：`TestRegisterSUB32` 通過；固定雜湊 FD2 經過
`0x45D66` 後自然進入 `main`。
