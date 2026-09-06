# 117 — 386 立即值寫入 base+disp32

狀態：**CONFORMED**
日期：2026-09-06
前置：[`RE 077`](../re/077-fd2-ail-indexed-table-clear.md)

- 支援無 prefix 的 `C7 /0`、`mod=2`、非 SIB 的
  `MOV dword ptr [base+disp32],imm32`。
- 位移以有號 32 位元加入 base；EBP 使用 SS，其餘使用 DS。
- descriptor 不可寫或範圍不足時，記憶體保持不變並失敗。
- 非 `/0`、operand16、segment override、SIB 與其他形狀維持失敗即關閉。

驗收：合成測試覆蓋負位移、立即值及唯讀拒絕；固定雜湊 FD2 自然抵達
`0x3F05F`，測試先把目標欄位設為非零，再執行原始指令至 `0x3F069`，確認
`0x52B10` 被清零。此受控前置只證明單一指令，不提升為一般玩家路徑證據。

驗收收據（2026-09-06）：`TestStoreImmediateDwordAtBaseDisp32` 覆蓋負位移、
立即值及唯讀拒絕；`TestFD2ClearsAILIndexedTableEntry` 由固定原版自然抵達
`0x3F05F`，將目標預設為非零後執行原始指令至 `0x3F069`，確認清零。
