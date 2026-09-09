# 115 — 386 絕對位址 dword INC

狀態：**CONFORMED**
日期：2026-09-06
前置：[`RE 075`](../re/075-fd2-ail-handler-gate-increment.md)

- 支援無 prefix 的 `FF /0`、`mod=0 r/m=5`（`INC dword ptr [disp32]`）。
- 來源與目的使用 DS descriptor；必須成功讀取且可寫才更新記憶體。
- 依 32 位加一更新 OF、SF、ZF、AF、PF，保留原 CF。
- 讀取或寫入失敗時，記憶體與旗標保持不變。
- 其他 `FF /0` 尋址、16 位 operand 與 prefix 維持失敗即關閉。

驗收：單元測試覆蓋加至零、CF 保留與唯讀拒絕；固定雜湊 FD2 必須由
LE entry 自然執行 `0x3E727` 至 `0x3E72D`，確認 `dword_52BEA` 由零變一。

驗收收據（2026-09-06）：`TestIncrementAbsoluteDword` 覆蓋加至零、CF
保留及唯讀拒絕；`TestFD2EntersAILHandlerGate` 由固定原版 LE entry
自然執行至 `0x3E72D`，確認 `dword_52BEA` 由零變一。
