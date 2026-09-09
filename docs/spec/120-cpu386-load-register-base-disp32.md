# 120 — 386 從 base+disp32 載入暫存器

狀態：**CONFORMED**
日期：2026-09-06
前置：[`RE 080`](../re/080-fd2-ail-indexed-table-read.md)

- 支援無 prefix 的 `8B /r`、`mod=2`、非 SIB 的
  `MOV r32,dword ptr [base+disp32]`。
- 位移以有號 32 位元加入 base；EBP 使用 SS，其餘使用 DS。
- 讀取失敗時目的暫存器保持不變並失敗。
- operand16、segment override、SIB 與其他形狀維持失敗即關閉。

驗收：合成測試覆蓋負位移與越界拒絕；固定雜湊 FD2 必須由 LE entry
自然執行 `0x3E8E6` 至 `0x3E8EC`，確認 EDI=`0x3C`、EAX=`0xD68D`。

驗收收據（2026-09-06）：`TestLoadRegisterFromBaseDisp32` 覆蓋負位移及
越界拒絕；`TestFD2ReadsAILIndexedTableEntry` 由固定原版 LE entry 自然
執行至 `0x3E8EC`，確認 EDI=`0x3C`、EAX=`0xD68D`。
