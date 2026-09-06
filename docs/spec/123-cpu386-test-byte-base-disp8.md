# 123 — 386 base+disp8 byte TEST

狀態：**CONFORMED**
日期：2026-09-06
前置：[`RE 083`](../re/083-fd2-ail-restore-interrupt-flag.md)

- 支援無 prefix 的 `F6 /0`、`mod=1`、非 SIB 的
  `TEST byte ptr [base+disp8],imm8`。
- 位移以有號 8 位元加入 base；EBP 使用 SS，其餘使用 DS。
- 依 byte AND 結果更新 SF、ZF、PF，清除 CF、OF；AF 依 x86 契約不作
  保證，不修改記憶體或通用暫存器。
- operand16、segment override、SIB、非 `/0` 與其他形狀維持失敗即關閉。

驗收：合成測試覆蓋零／非零結果、負位移與越界拒絕；固定雜湊 FD2 必須由
LE entry 自然執行 `0x3E885` 至 `0x3E889`，確認保存的 IF 未設定且 ZF=1。

驗收收據（2026-09-06）：`TestByteTESTAtBaseDisp8` 覆蓋零／非零、負位移
與越界拒絕；`TestFD2ChecksSavedInterruptFlag` 由固定原版 LE entry 自然
執行至 `0x3E889`，確認保存的 IF 未設定且 ZF=1。
