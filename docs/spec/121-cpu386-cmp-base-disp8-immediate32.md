# 121 — 386 base+disp8 與 imm32 比較

狀態：**CONFORMED**
日期：2026-09-06
前置：[`RE 081`](../re/081-fd2-ail-rate-threshold.md)

- 支援無 prefix 的 `81 /7`、`mod=1`、非 SIB 的
  `CMP dword ptr [base+disp8],imm32`。
- 位移以有號 8 位元加入 base；EBP 使用 SS，其餘使用 DS。
- 依 32 位減法更新旗標，不修改記憶體與通用暫存器。
- operand16、segment override、SIB、非 `/7` 與其他形狀維持失敗即關閉。

驗收：合成測試覆蓋負位移、相等旗標與越界拒絕；固定雜湊 FD2 必須由
LE entry 自然執行 `0x3E89F` 至 `0x3E8A6`，確認 stack 參數等於
`0xD68D` 且 ZF=1。

驗收收據（2026-09-06）：`TestCompareDwordAtBaseDisp8Immediate32` 覆蓋
負位移、相等旗標與越界拒絕；`TestFD2ComparesAILRateThreshold` 由固定
原版 LE entry 自然執行至 `0x3E8A6`，確認參數 `0xD68D` 且 ZF=1。
