# 096 — FD2 C runtime 保存正規化模式字元

日期：2026-09-06
證據等級：函式、原始位元組、writer 與 consumer 為**已證實**
輸入：`FD2.EXE`，大小 `357074`，MD5 `b97caf2239a27a896069d03549d96e1e`，
SHA-256 `222b7d067ad4450eb9c5f6e6bce1797d54bb050417ba39ced6067f8039f28c4f`
工具：IDA Pro 9.4，映像 `ida-pro-9.4-idapython:locked-v1`
位址空間：IDA 載入 LE 映像後的 32 位線性位址

`sub_36EBC` 呼叫 `tolower` 正規化模式首字元後，`0x36EE7` 原始位元組
`88 45 FC` 是 `mov [ebp-4],al`。緊接的 `0x36EEA` 比較 AL 與 `'r'`；
此 stack local 也保留該正規化字元供函式後段使用。caller 位於
`0x36FBE` 與 `0x37068`。

一次性 IDA JSON 證據 SHA-256：
`7a9c5546b9cb158487f29296543bde7c6cf9a8a738ffc3ed1917826ab9c00db6`。
