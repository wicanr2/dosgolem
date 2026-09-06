# 077 — FD2 AIL 索引表格清零

日期：2026-09-06
證據等級：函式、參數資料流、指令與目的位址為**已證實**
輸入：`FD2.EXE`，大小 `357074`，MD5 `b97caf2239a27a896069d03549d96e1e`，
SHA-256 `222b7d067ad4450eb9c5f6e6bce1797d54bb050417ba39ced6067f8039f28c4f`
工具：IDA Pro 9.4.0.260610，映像 `ida-pro-9.4-idapython:locked-v1`
位址空間：IDA 載入 LE 映像後的 32 位線性位址

`sub_3F048` 在 `0x3F053` 將第一個參數載入 EBX；於 `0x3F05F` 執行
`mov dword_52AD4[ebx],0`。固定原版路徑在此 EBX=`0x3C`，所以目的線性
位址為 `0x52B10`。`0x3F069` 隨即呼叫 `sub_3E8C7`，是此清零結果的
第一個控制流 consumer。

本切片只要求無前綴 `C7 /0`、base+disp32、無 SIB 的立即 dword 寫入；
不從表格位置推測更高階 AIL channel 語意。
