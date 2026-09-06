# 086 — FD2 MDI.INI 設定 buffer 位址

日期：2026-09-06
證據等級：函式、原始位元組、位移與 consumer 為**已證實**
輸入：`FD2.EXE`，大小 `357074`，MD5 `b97caf2239a27a896069d03549d96e1e`，
SHA-256 `222b7d067ad4450eb9c5f6e6bce1797d54bb050417ba39ced6067f8039f28c4f`
工具：IDA Pro 9.4.0.260610，映像 `ida-pro-9.4-idapython:locked-v1`
位址空間：IDA 載入 LE 映像後的 32 位線性位址

`sub_3F306` 配置並清零第一個本地區域後，在 `0x3F323`、`0x3F325`
推入第二次 `memset` 的長度 `0x18` 與 value 0。`0x3F327` 原始位元組為
`8D 84 24 08 01 00 00`，即 `lea eax,[esp+108h]`；`0x3F32E` 將結果推入，
`0x3F32F` 呼叫 `memset`。

本切片只需無前綴 `LEA r32,[esp+disp32]` 的 `mod=2`、SIB=`24h` 形狀。
LEA 不存取 stack 記憶體，只計算有效位址。

一次性 IDA 函式報告 SHA-256：
`93de05197299ce36c144d70999ab648169eed9cf2014b6ced68f4afca53be42e`。
