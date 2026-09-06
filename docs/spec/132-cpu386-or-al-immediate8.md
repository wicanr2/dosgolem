# 132 — 386 OR AL immediate8

狀態：**CONFORMED**
日期：2026-09-06
前置：[`RE 092`](../re/092-fd2-open-flags-base-mode.md)

- 支援無 prefix 的 `0C ib`：`OR AL,imm8`。
- 只寫回 EAX 低 byte，保留 EAX 高 24 位，並依 byte 結果更新邏輯旗標。
- 其他暫存器與記憶體不變。
- 所有 prefix 維持失敗即關閉。

驗收：合成測試確認低 byte、高位與旗標；固定雜湊 FD2 必須自然執行
`0x36E45` 至 `0x36E47`，確認 AL 變為原值 OR `3`。

驗收收據（2026-09-06）：`TestOrALImmediate8` 確認低 byte、高位與旗標；
`TestFD2BuildsBaseOpenFlags` 從固定原版 LE entry 自然執行至 `0x36E47`，
確認 EAX 變為原值 OR `3`。
