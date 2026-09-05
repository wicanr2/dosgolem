# 109 — 386 從絕對位址載入 segment word

狀態：**CONFORMED**
日期：2026-09-06
前置：[`RE 068`](../re/068-fd2-ail-data-selector-save.md)

- 擴充 `66 8E /r`，只接受無其他 prefix、`mod=0 r/m=5` 的
  `MOV Sreg,word ptr [disp32]`。
- 透過 DS descriptor 讀取 16 位 selector，並在完整讀取後使用既有
  selector 驗證契約載入目的 segment。
- 讀取失敗或 selector 不可載入時，目的 segment 保持不變。
- register、其他記憶體尋址、segment override 與其他 operand-size 形狀維持
  失敗即關閉（fail-closed）。
- 固定 FD2 在 `0x3E93C` 以 `66 8E 05 EE 2B 05 00` 從
  `word_52BEE` 載入 ES。

驗收：單元測試覆蓋成功與非法 selector 拒絕；固定雜湊 FD2 必須由 LE entry
自然執行至 `0x3E943`，並確認 ES 等於 `word_52BEE`。

驗收收據（2026-09-06）：`TestLoadSegmentAbsoluteWord` 與固定原版
`TestFD2LoadsAILDataSelectorWhenProvided` 通過；後者由 LE entry 自然執行
至 `0x3E943`，確認 ES 等於 `word_52BEE`。同一原版唯讀掛載下
`go test ./... -count=1` 全數通過。
