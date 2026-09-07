# 105 — 386 短距離 JL

狀態：**CONFORMED**
日期：2026-09-06
前置：[`RE 066`](../re/066-fd2-ail-default-table-loop-bound.md)

- 支援無 prefix 的 opcode `7C cb`（`JL rel8`）。
- 將 rel8 作有符號擴展；當 SF 與 OF 不相等時，從指令結尾更新 EIP。
- 不成立時停在下一指令；兩條路徑都不得修改旗標或其他暫存器。
- operand-size、segment override 與 repeat prefix 維持失敗即關閉（fail-closed）。
- 固定 FD2 在 `0x3FA50` 以 `7C F1` 回跳 `0x3FA43`，直接消費
  `0x3FA4D` 對 EAX 與 16 的 signed 比較。

驗收：單元測試覆蓋 SF/OF 四種組合；固定雜湊 FD2 必須由 LE entry 自然執行
第一輪 `0x3FA50`，並在 EAX 小於 16 時回到 `0x3FA43`。

驗收收據（2026-09-06）：`TestJLShort` 與固定原版
`TestFD2BranchesThroughAILTableLoopWhenProvided` 通過；後者由 LE entry
自然執行第一輪 `0x3FA50`，並在 EAX 小於 16 時回到 `0x3FA43`。
同一原版唯讀掛載下 `go test ./... -count=1` 全數通過。
