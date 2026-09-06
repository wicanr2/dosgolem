# 122 — 386 立即 port byte 輸出與 LE 觀測

狀態：**CONFORMED**
日期：2026-09-06
前置：[`RE 082`](../re/082-fd2-ail-pit-channel0-programming.md)

- CPU 支援無 prefix 的 `OUT imm8,AL`（opcode `E6h`）。
- port 以零擴展 imm8，value 取 EAX 低 8 位元；指令不修改暫存器或旗標。
- 輸出必須由已安裝的 `PortOut` consumer 接受；沒有 consumer 或拒絕時
  失敗即關閉。
- `LEMachine` 保存每個 port 最後值及具順序編號的完整 byte 寫入紀錄。
- 不在此層模擬 PIT wall-clock、IRQ 頻率或逐週期行為。

驗收：CPU 合成測試覆蓋 port/value、狀態不變與 consumer 缺失拒絕；LE
測試覆蓋紀錄順序；固定雜湊 FD2 必須由 LE entry 自然執行 `0x3E86E` 至
`0x3E882`，確認寫入序列為 `43h:36h、40h:00h、40h:00h`，且保存的
divisor 為零。

驗收收據（2026-09-06）：`TestOutputALToImmediatePort` 覆蓋 port/value、
狀態不變及 consumer 缺失拒絕；`TestFD2ProgramsPITControl` 與
`TestFD2ProgramsPITDivisor` 由固定原版 LE entry 自然執行至 `0x3E882`，
確認三筆序列及零 divisor。
