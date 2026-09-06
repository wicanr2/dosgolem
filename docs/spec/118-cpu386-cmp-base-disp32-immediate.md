# 118 — 386 base+disp32 與 imm8 比較

狀態：**CONFORMED**
日期：2026-09-06
前置：[`RE 078`](../re/078-fd2-ail-active-table-scan.md)

- 支援無 prefix 的 `83 /7`、`mod=2`、非 SIB 的
  `CMP dword ptr [base+disp32],sign-extended imm8`。
- 位移以有號 32 位元加入 base；EBP 使用 SS，其餘使用 DS。
- 依 32 位減法更新旗標，不修改記憶體與通用暫存器。
- operand16、segment override、SIB、非 `/7` 與其他形狀維持失敗即關閉。

驗收：合成測試覆蓋負位移、負立即值、旗標及越界拒絕；固定雜湊 FD2
必須由 LE entry 自然執行 `0x3E8DD` 至 `0x3E8E4`，確認首項零值令 ZF=1。

驗收收據（2026-09-06）：`TestCompareDwordAtBaseDisp32` 覆蓋負位移、
負立即值與越界拒絕；`TestFD2ScansAILActiveTableEntry` 由固定原版 LE entry
自然執行至 `0x3E8E4`，確認首項為零且 ZF=1。
