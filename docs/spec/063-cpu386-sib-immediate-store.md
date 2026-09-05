# 063 — 386 SIB 位址立即值寫入

狀態：**CONFORMED**  
日期：2026-09-06  
前置：[`RE 050`](../re/050-fd2-third-callback-allocation-consumer.md)

固定原版 `0x4CCAC` 的原始 bytes 是 `C7 04 01 00 00 00 00`，即
`mov dword ptr [ecx+eax],0`。dosgolem 支援無前綴 `C7 /0`、`mod=0`、SIB
scale=1 且無 displacement 的 32 位元立即值寫入；base 與 index 由 SIB 指定。
保留 ESP-as-index、無 base、其他 ModRM／前綴形狀為失敗即關閉。寫入失敗不得
部分提交記憶體。

驗收涵蓋一般 base＋index 寫入、拒絕保留 index 及固定 FD2 environment 終止空
指標，最後自然返回第三回呼。

2026-09-06：base＋index 寫入、保留 index 拒絕及固定 environment 終止指標
整合測試通過。
