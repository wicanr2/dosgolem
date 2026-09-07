# 093 — FD2 C runtime 建立 binary 開檔旗標

日期：2026-09-06
證據等級：函式、原始位元組與後續 consumer 為**已證實**
輸入：`FD2.EXE`，大小 `357074`，MD5 `b97caf2239a27a896069d03549d96e1e`，
SHA-256 `222b7d067ad4450eb9c5f6e6bce1797d54bb050417ba39ced6067f8039f28c4f`
工具：IDA Pro 9.4，映像 `ida-pro-9.4-idapython:locked-v1`
位址空間：IDA 載入 LE 映像後的 32 位線性位址

`__open_flags` 在處理模式字串的 binary 分支時，`0x36E6D` 將 EBX 複製到
EDX；`0x36E6F` 原始位元組 `80 CA 40` 是 `or dl,0x40`。後續依序比較
`[esi+1]`／`[esi+2]` 的 `'b'` 與 `'+'`，並在 `0x36E80` 再以
`or dl,3` 合併更新模式，因此 DL 是此分支的旗標累積值。

caller 位於 `0x36E08` 與 `0x36ECD`。本切片只授權 `OR r8,imm8` 的 CPU
語意，不把 C runtime bit 值直接映射成 host API 旗標。

一次性 IDA JSON 證據 SHA-256：
`746e6ef8a25c0035947e9181fa4bfdf4c34baf343fcbc122167c1d92f7bddb8e`。
