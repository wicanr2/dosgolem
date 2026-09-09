# 104 — 386 暫存器與符號擴展 imm8 比較

狀態：**CONFORMED**
日期：2026-09-06
前置：[`RE 066`](../re/066-fd2-ail-default-table-loop-bound.md)

- 擴充無 prefix 的 opcode `83 /7`、`mod=3`（`CMP r32,sign-extended imm8`）。
- 立即數必須先以有符號 8 位擴展成 32 位，再執行不寫回的 32 位減法旗標。
- 目的暫存器保持不變；更新 CF、PF、AF、ZF、SF、OF。
- 16 位 operand、prefix 與其他未列 `83` 形狀維持失敗即關閉（fail-closed）。
- 固定 FD2 在 `0x3FA4D` 使用 `83 F8 10` 比較 EAX 與 16；
  `0x3FA50` 的 `JL` 是直接 consumer。

驗收：單元測試覆蓋小於與負立即數相等；固定雜湊 FD2 必須由 LE entry
自然執行至 `0x3FA50`，確認 EAX 不變且目前第一輪比較為 signed less。

驗收收據（2026-09-06）：`TestGroup83CompareRegister` 與固定原版
`TestFD2ComparesAILTableIndexWhenProvided` 通過；後者由 LE entry 自然執行
至 `0x3FA50`，確認 EAX 不變且第一輪比較設定 SF=1、ZF=0。同一原版唯讀
掛載下 `go test ./... -count=1` 全數通過。
