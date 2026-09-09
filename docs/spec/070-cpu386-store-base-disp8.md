# 070 — 386 base＋disp8 dword 寫入

狀態：**CONFORMED**  
日期：2026-09-06  
前置：[`RE 052`](../re/052-fd2-post-environment-runtime-list.md)

- 擴充無前綴 `89 /r`、`mod=1`、非 SIB base 的 32 位元寫入。
- EBP base 使用 SS，其餘 base 使用 DS；disp8 符號延伸。
- 先完整解碼並成功寫入後才視為完成；SIB 與其他前綴維持失敗即關閉。
- 固定 FD2 在 `0x46915` 以 `89 58 04` 把既有紀錄指標寫到新配置節點偏移 4。

2026-09-06：一般 base 寫入與固定配置節點串接測試通過。
