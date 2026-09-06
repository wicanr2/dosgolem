# 079 — FD2 AIL 表格掃描迴圈界線

日期：2026-09-06
證據等級：指令、旗標來源、分支目標與迴圈次數為**已證實**
輸入：`FD2.EXE`，大小 `357074`，MD5 `b97caf2239a27a896069d03549d96e1e`，
SHA-256 `222b7d067ad4450eb9c5f6e6bce1797d54bb050417ba39ced6067f8039f28c4f`
工具：IDA Pro 9.4.0.260610，映像 `ida-pro-9.4-idapython:locked-v1`
位址空間：IDA 載入 LE 映像後的 32 位線性位址

`sub_3E8C7` 在每次表項處理後由 `0x3E8F2 add edi,4` 前進，並在
`0x3E8F5 cmp edi,40h` 建立無號比較旗標。`0x3E8F8 jb 0x3E8DD`
僅在 CF=1 時回到表項比較，因此 EDI 依序為 0、4、…、`0x3C`，共處理
16 項；EDI=`0x40` 時不跳並抵達 `0x3E8FA`。

本切片只需要無前綴短距離 `JB rel8`（opcode `72h`），不涉及 DOS/PIT
或更高階 AIL 語意。
