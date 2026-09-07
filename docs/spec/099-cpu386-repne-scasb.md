# 099 — 386 REPNE SCASB

狀態：**CONFORMED**
日期：2026-09-06
前置：[`RE 063`](../re/063-fd2-watcom-strlen-repne-scasb.md)

- 支援無 operand-size 與 segment override 的 `F2 AE`（`REPNE SCASB`）。
- ECX 為零時不得讀取記憶體或移動 EDI。
- 每次以 AL 比較 ES:EDI 的 byte，依 DF 更新 EDI、遞減 ECX；ZF=1 或
  ECX=0 時停止，保留最後一次比較的旗標。
- `F2` 與其他 repeat prefix 重複或衝突時拒絕；`F2` 配合非 `AE`
  opcode 維持失敗即關閉（fail-closed）。
- 固定 FD2 在 Watcom `strlen` 的 `0x37816` 使用此形狀掃描 NUL。

驗收：單元測試覆蓋遇 NUL 停止、計數與 EDI，以及非 SCASB 拒絕；固定雜湊
FD2 必須由 LE entry 自然執行至 `0x37818`，且 ZF=1、ECX 已遞減。

驗收收據（2026-09-06）：`TestREPNESCASB` 與固定原版
`TestFD2ScansFirstEnvironmentNameWhenProvided` 通過；後者由 LE entry 自然
執行至 `0x37818`，確認 ZF=1 且 ECX 已由 `0xFFFFFFFF` 遞減。
