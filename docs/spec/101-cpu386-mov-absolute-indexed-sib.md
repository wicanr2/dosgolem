# 101 — 386 絕對位移加索引 SIB dword 讀取

狀態：**CONFORMED**
日期：2026-09-06
前置：[`RE 064`](../re/064-fd2-ail-preference-table-access.md)

- 擴充無 prefix 的 opcode `8B`，只接受 `mod=0`、`r/m=4` 且 SIB
  `base=5`（無基址）、`index!=ESP` 的 `MOV r32,[disp32+r32*scale]`。
- scale 依 SIB 的 0..3 解讀為左移位數；位址以 32 位算術計算並使用 DS。
- 完整讀取 dword 後才修改目的暫存器；讀取失敗時目的暫存器保持不變。
- SIB 無索引、含基址、prefix、16 位 operand 與其他未列形狀維持
  失敗即關閉（fail-closed）。
- 固定 FD2 在 `0x3F5EB` 以 `8B 14 85 0C 43 05 00` 讀取
  `0x5430C + EAX*4`，目的暫存器為 EDX。

驗收：單元測試覆蓋 scale=2 的成功與越界不改目的暫存器；固定雜湊 FD2
必須由 LE entry 自然執行至 `0x3F5F2`，並確認 EDX 等於步進前的表格項目。

驗收收據（2026-09-06）：`TestMoveDwordFromAbsoluteIndexedSIB` 與固定原版
`TestFD2ReadsAILPreferenceWhenProvided` 通過；後者自然執行至 `0x3F5F2`，
確認 EDX 等於步進前的 `0x5430C + EAX*4` 項目。
