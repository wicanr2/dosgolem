# 100 — 386 暫存器 NOT

狀態：**CONFORMED**
日期：2026-09-06
前置：[`RE 063`](../re/063-fd2-watcom-strlen-repne-scasb.md)

- 支援無 prefix 的 `F7 /2`、`mod=3`（`NOT r32`）。
- 對指定 32 位暫存器逐位反相，不修改任何旗標。
- 記憶體 operand、16 位 operand、prefix 與其他未列 `F7` 群組維持
  失敗即關閉（fail-closed）；既有 `F7 /3`（`NEG r32`）行為不變。
- 固定 FD2 在 Watcom `strlen` 的 `0x37818` 使用 `F7 D1`（`not ecx`），
  consumer 是 `0x3781A` 的 `dec ecx`。

驗收：單元測試覆蓋結果與旗標不變；固定雜湊 FD2 必須由 LE entry 自然執行
至 `0x3781A`，並在目前固定環境輸入得到 ECX=`0x0000000A`。

驗收收據（2026-09-06）：`TestNotRegister32` 與固定原版
`TestFD2ComplementsStrlenCountWhenProvided` 通過；後者由 LE entry 自然執行
至 `0x3781A`，確認 ECX=`0x0000000A`。同一原版唯讀掛載下
`go test ./... -count=1` 全數通過。
