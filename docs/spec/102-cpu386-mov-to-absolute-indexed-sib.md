# 102 — 386 絕對位移加索引 SIB dword 寫入

狀態：**CONFORMED**
日期：2026-09-06
前置：[`RE 064`](../re/064-fd2-ail-preference-table-access.md)

- 擴充無 prefix 的 opcode `89`，只接受 `mod=0`、`r/m=4` 且 SIB
  `base=5`（無基址）、`index!=ESP` 的 `MOV [disp32+r32*scale],r32`。
- scale 依 SIB 的 0..3 解讀為左移位數；位址以 32 位算術計算並使用 DS。
- 寫入必須通過 descriptor 範圍與 writable 檢查；失敗時記憶體保持不變。
- SIB 無索引、含基址、prefix、16 位 operand 與其他未列形狀維持
  失敗即關閉（fail-closed）。
- 固定 FD2 在 `0x3F5F6` 以 `89 1C 85 0C 43 05 00` 將 EBX 寫入
  `0x5430C + EAX*4`。

驗收：單元測試覆蓋 scale=2 的成功與越界拒絕；固定雜湊 FD2 必須由 LE entry
自然執行至 `0x3F5FD`，並確認表格項目等於步進前的 EBX。

驗收收據（2026-09-06）：`TestMoveDwordToAbsoluteIndexedSIB` 與固定原版
`TestFD2WritesAILPreferenceWhenProvided` 通過；後者自然執行至 `0x3F5FD`，
確認 `0x5430C + EAX*4` 項目等於步進前的 EBX。同一原版唯讀掛載下
`go test ./... -count=1` 全數通過。
