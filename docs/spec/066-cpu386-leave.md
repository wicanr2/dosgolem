# 066 — 386 LEAVE

狀態：**CONFORMED**  
日期：2026-09-06  
前置：[`RE 050`](../re/050-fd2-third-callback-allocation-consumer.md)

- 支援無前綴 `C9`：先以 EBP 作新 ESP，再從 SS:[ESP] 載入舊 EBP，最後 ESP
  增加 4。
- 堆疊讀取失敗時不得修改 EBP 或 ESP；所有前綴維持失敗即關閉。
- 固定 FD2 在 `0x4CCD9` 以此拆除第三回呼堆疊框架並自然返回 `0x45DD3`。

2026-09-06：EBP／ESP 還原與固定回呼拆框測試通過。
