# 076 — FD2 AIL 索引表格寫入

日期：2026-09-06
證據等級：函式、參數資料流、指令與目的位址為**已證實**
輸入：`FD2.EXE`，大小 `357074`，MD5 `b97caf2239a27a896069d03549d96e1e`，
SHA-256 `222b7d067ad4450eb9c5f6e6bce1797d54bb050417ba39ced6067f8039f28c4f`
工具：IDA Pro 9.4.0.260610，映像 `ida-pro-9.4-idapython:locked-v1`
位址空間：IDA 載入 LE 映像後的 32 位線性位址

`sub_3F048`（`0x3F048..0x3F078`）由 `sub_3864A` 在 `0x386B6` 呼叫。
它在 `0x3F053` 將第一個參數載入 EBX、`0x3F056` 將第二個參數載入 EAX，
並於 `0x3F059` 執行 `mov dword_52B14[ebx],eax`。固定原版自然路徑在此
EBX=`0x3C`、EAX=`0xD68D`，因此目的線性位址為 `0x52B50`。

下一指令另將 `dword_52AD4[ebx]` 清零，但屬不同 opcode 形狀，不在本切片
猜補。本切片只要求 `MOV r/m32,r32` 的 base+disp32、無 SIB 形式。

一次性 IDA 報告 SHA-256：
`cda27de7779992cd4af2f172e04fb3223434e266eb8627da5df8231210816eca`。
