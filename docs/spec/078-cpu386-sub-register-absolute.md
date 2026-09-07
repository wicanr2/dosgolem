# 078 — 386 暫存器減去絕對 dword

狀態：**CONFORMED**
日期：2026-09-06
前置：[`RE 055`](../re/055-fd2-stack-distance-helper.md)

- 擴充無前綴 `2B /r`、`mod=0 r/m=5`，從 DS 絕對位址讀取 dword，再由目的
  暫存器減去該值並更新完整 subtraction 旗標。
- 讀取失敗不得修改目的暫存器；前綴與其他新增形狀維持失敗即關閉。
- 固定 FD2 在 `0x463BE` 以 `2B 05 14280500` 計算 stack 基準差。

驗收收據（2026-09-06）：`TestSubtractRegisterAbsolute` 通過；固定雜湊
FD2 從 LE entry 自然經過 `0x463BE` 並進入 `main`。
