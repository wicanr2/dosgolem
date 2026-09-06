# 080 — FD2 AIL 對應索引表讀取

日期：2026-09-06
證據等級：指令、索引關係與執行期值為**已證實**
輸入：`FD2.EXE`，大小 `357074`，MD5 `b97caf2239a27a896069d03549d96e1e`，
SHA-256 `222b7d067ad4450eb9c5f6e6bce1797d54bb050417ba39ced6067f8039f28c4f`
工具：IDA Pro 9.4.0.260610，映像 `ida-pro-9.4-idapython:locked-v1`
位址空間：IDA 載入 LE 映像後的 32 位線性位址

`sub_3E8C7` 在 `0x3E8DD` 確認 `dword_52A94[EDI]` 非零後，於
`0x3E8E6` 執行 `mov eax,dword_52B14[edi]`。固定原版初始化路徑使
EDI=`0x3C` 的啟用欄位非零，對應線性位址 `0x52B50` 先前已由
`sub_3F048` 寫入 `0xD68D`；因此本讀取的已證實結果為 EAX=`0xD68D`。

本切片只要求 `MOV r32,r/m32` 的 base+disp32、無 SIB 形式；不推測
表項所代表的高階音訊裝置或 channel 類型。
