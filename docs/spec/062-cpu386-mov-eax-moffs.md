# 062 — 386 絕對位址載入 EAX

狀態：**CONFORMED**  
日期：2026-09-06  
前置：[`RE 050`](../re/050-fd2-third-callback-allocation-consumer.md)

- 支援無前綴 `A1 moffs32`，從 DS 平坦描述子的 32 位元絕對位址讀入 EAX。
- 讀取失敗不得提交 EAX；operand-size、segment override 與 REP 維持失敗即關閉。
- 固定 FD2 以 `0x4CCA7` 的 `A1 FC370500` 讀取 environment 指標表，並由
  LE 入口自然完成第三回呼返回。

2026-09-06：正常讀取、越界不提交及固定雜湊整合測試通過。
