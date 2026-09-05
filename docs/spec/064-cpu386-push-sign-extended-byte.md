# 064 — 386 符號延伸 byte PUSH

狀態：**CONFORMED**  
日期：2026-09-06  
前置：[`RE 050`](../re/050-fd2-third-callback-allocation-consumer.md)

- 支援無前綴 `6A ib`，把立即 byte 符號延伸為 32 位元後壓入 SS:ESP。
- 堆疊不足或寫入失敗時不得提交 ESP；所有前綴維持失敗即關閉。
- 固定 FD2 在 `0x4CCBC` 以 `6A 00` 傳入 `memset` 的零值參數，並由 LE 入口
  自然完成第三回呼返回。

2026-09-06：負 byte 符號延伸與固定 `memset` 呼叫路徑測試通過。
