# 078 — FD2 AIL 表格啟用欄位掃描

日期：2026-09-06
證據等級：函式、迴圈、指令與分支 consumer 為**已證實**
輸入：`FD2.EXE`，大小 `357074`，MD5 `b97caf2239a27a896069d03549d96e1e`，
SHA-256 `222b7d067ad4450eb9c5f6e6bce1797d54bb050417ba39ced6067f8039f28c4f`
工具：IDA Pro 9.4.0.260610，映像 `ida-pro-9.4-idapython:locked-v1`
位址空間：IDA 載入 LE 映像後的 32 位線性位址

`sub_3E8C7`（`0x3E8C7..0x3E92C`）由 `sub_3F048` 在 `0x3F069`
呼叫。它以 EDI 從 0 每次加 4，直到 `0x40`，共掃描 16 個 dword 表項。
`0x3E8DD` 執行 `cmp dword_52A94[edi],0`，`0x3E8E4` 在相等時跳過該項；
非零時才由 `0x3E8E6` 讀取對應 `dword_52B14[edi]`，並更新目前最小值。

可證實 `dword_52A94[]` 是這段掃描的啟用／存在判斷欄位，但不據此推測
更高階 AIL channel 類型。本切片只需要無前綴 `83 /7`、base+disp32、
無 SIB 的 dword 與 sign-extended imm8 比較。

一次性 IDA 報告 SHA-256：
`715c16ffa5c47632c843c4e3e794e7efaa2579558a6a913eb418d4af6fd6191d`。
