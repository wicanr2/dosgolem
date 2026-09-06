# 114 — 386 從 16 位元暫存器載入 segment selector

狀態：**CONFORMED**
日期：2026-09-06
前置：[`RE 074`](../re/074-fd2-ail-load-code-selector.md)

- 支援 `66 8E /r`，限無 segment override 且 `mod=3` 的暫存器來源。
- 從來源通用暫存器低 16 位元取得 selector，依既有 `SegmentLoadOK`
  驗證後寫入目的 segment register。
- 無效 segment 編碼、被拒 selector、記憶體 ModRM 與其他未列前綴維持
  失敗即關閉。

驗收：合成測試覆蓋允許及拒絕 selector；固定雜湊 FD2 必須由 LE entry
自然執行 `0x3E9E9 mov ds,bx`，再由 `AH=25h` 消費 DS:EDX。

驗收收據（2026-09-06）：`TestLoadSegmentRegister16` 覆蓋允許與拒絕；
`TestFD2ReplacesTimerDOSVector` 由固定原版自然執行至 `0x3E9EF`，並確認
`AH=25h` 已消費 `CS:0x3E73E`。
