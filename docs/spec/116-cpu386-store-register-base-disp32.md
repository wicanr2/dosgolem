# 116 — 386 暫存器寫入 base+disp32

狀態：**CONFORMED**
日期：2026-09-06
前置：[`RE 076`](../re/076-fd2-ail-indexed-table-store.md)

- 支援無 prefix 的 `89 /r`、`mod=2`、非 SIB 的
  `MOV dword ptr [base+disp32],r32`。
- 位移以有號 32 位元加入 base；EBP 使用 SS，其餘使用 DS。
- descriptor 不可寫或範圍不足時，記憶體保持不變並失敗。
- operand16、segment override、SIB 與其他未列形狀維持失敗即關閉。

驗收：合成測試覆蓋成功、負位移與唯讀拒絕；固定雜湊 FD2 必須由 LE entry
自然執行 `0x3F059` 至 `0x3F05F`，確認 `0x52B50` 寫入 `0xD68D`。

驗收收據（2026-09-06）：`TestStoreDwordAtBaseDisp32` 覆蓋負位移寫入與
唯讀拒絕；`TestFD2StoresAILIndexedTableEntry` 由固定原版 LE entry 自然
執行至 `0x3F05F`，確認 `0x52B50` 寫入 `0xD68D`。
