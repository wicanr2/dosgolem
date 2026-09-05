# 108 — 386 絕對位址 segment word 寫入

狀態：**CONFORMED**
日期：2026-09-06
前置：[`RE 068`](../re/068-fd2-ail-data-selector-save.md)

- 擴充 `66 8C /r`，只接受無其他 prefix、`mod=0 r/m=5` 的
  `MOV word ptr [disp32],Sreg`。
- 依 ModRM segment 編碼取得 16 位 selector，透過 DS descriptor 寫入兩 byte。
- descriptor 不可寫或範圍不足時，記憶體保持不變。
- register、其他記憶體尋址、segment override 與其他 operand-size 形狀維持
  失敗即關閉（fail-closed）。
- 固定 FD2 在 `0x3E935` 以 `66 8C 1D EE 2B 05 00` 保存 DS 至
  `word_52BEE`，下一指令是直接 consumer。

驗收：單元測試覆蓋成功與越界拒絕；固定雜湊 FD2 必須由 LE entry 自然執行
至 `0x3E93C`，並確認 `word_52BEE` 等於 DS。

驗收收據（2026-09-06）：`TestStoreSegmentAbsoluteWord` 與固定原版
`TestFD2StoresAILDataSelectorWhenProvided` 通過；後者由 LE entry 自然執行
至 `0x3E93C`，確認 `word_52BEE` 等於 DS。同一原版唯讀掛載下
`go test ./... -count=1` 全數通過。
