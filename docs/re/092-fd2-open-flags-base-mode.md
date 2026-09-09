# 092 — FD2 C runtime 建立基本開檔模式旗標

日期：2026-09-06
證據等級：函式、原始位元組與後續 consumer 為**已證實**
輸入：`FD2.EXE`，大小 `357074`，MD5 `b97caf2239a27a896069d03549d96e1e`，
SHA-256 `222b7d067ad4450eb9c5f6e6bce1797d54bb050417ba39ced6067f8039f28c4f`
工具：IDA Pro 9.4，映像 `ida-pro-9.4-idapython:locked-v1`
位址空間：IDA 載入 LE 映像後的 32 位線性位址

`__open_flags` 在確認模式首字元是 `r`、`w` 或 `a` 後，`0x36E43`
將 EBX 複製到 EAX；`0x36E45` 原始位元組 `0C 03` 是 `or al,3`。
其後立即檢查模式字串的 `'+'`、`'b'`、`'t'` 修飾字元，並繼續以 OR
加入旗標。因此 `0x36E45` 是基本模式位元的建立步驟。

caller 位於 `0x36E08` 與 `0x36ECD`。本切片只授權 CPU 的 `OR AL,imm8`
語意，不授權猜測各旗標位元的 host 檔案映射。

一次性 IDA JSON 證據 SHA-256：
`16a5c35fb3a8f80939273141d53d32e0981f6e2c48f8ba9382adddea509f9226`。
