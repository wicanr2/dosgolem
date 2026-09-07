# 113 — 386 segment selector 寫入 16 位元暫存器

狀態：**CONFORMED**
日期：2026-09-06
前置：[`RE 073`](../re/073-fd2-ail-save-old-vector-selector.md)

- 支援 `66 8C /r`，限無 segment override 且 `mod=3` 的暫存器目的。
- 將指定 segment selector 寫入目的通用暫存器低 16 位元，保留高 16 位元。
- 不修改旗標。
- 無效 segment 編碼、記憶體 ModRM 與其他未列前綴維持失敗即關閉。

驗收：合成測試覆蓋非零高半部、selector、旗標與來源 segment 不變；固定
雜湊 FD2 必須由 LE entry 自然執行 `0x3E9C1 mov dx,es` 至 `0x3E9C4`。

驗收收據（2026-09-06）：`TestStoreSegmentRegister16` 通過；固定原版在
`TestFD2ReplacesTimerDOSVector` 自然執行此指令並將舊 selector 保存至
`word_52BD8`。
