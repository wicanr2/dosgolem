# 068 — 386 絕對 byte AND／OR

狀態：**CONFORMED**  
日期：2026-09-06  
前置：[`RE 052`](../re/052-fd2-post-environment-runtime-list.md)

- 支援無前綴 `80 /4` 與 `80 /1`、`mod=0 r/m=5` 的 DS 絕對 byte 目的地。
- 先完整讀取位址、立即值與目的 byte，成功後才寫回並更新邏輯旗標。
- 其他新增的 memory 形狀、前綴及群組維持失敗即關閉。
- 固定 FD2 須由 LE 入口自然越過 `0x468F9` 與 `0x46905`，不可直接注入
  `byte_52881`。

2026-09-06：AND／OR 單元測試與固定原版 runtime 清單整合路徑通過。
