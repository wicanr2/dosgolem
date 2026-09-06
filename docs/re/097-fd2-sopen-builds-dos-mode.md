# 097 — FD2 sopen 組裝 DOS 開檔模式

日期：2026-09-06
證據等級：函式、原始位元組與 DOS consumer 為**已證實**
輸入：`FD2.EXE`，大小 `357074`，MD5 `b97caf2239a27a896069d03549d96e1e`，
SHA-256 `222b7d067ad4450eb9c5f6e6bce1797d54bb050417ba39ced6067f8039f28c4f`
工具：IDA Pro 9.4，映像 `ida-pro-9.4-idapython:locked-v1`
位址空間：IDA 載入 LE 映像後的 32 位線性位址

`sopen`（`0x3CD43..0x3CF03`）在 `0x3CD5A` 將開檔旗標遮罩為
`0x83`，`0x3CD62` 載入其低 byte 至 AL。`0x3CD67` 原始位元組
`0A 45 1C` 是 `or al,[ebp+0x1C]`；接著 `0x3CD70` 設定 `AH=3Dh`，
`0x3CD72` 執行 `int 21h`。因此 OR 的結果是 DOS `AH=3Dh` open 的 AL
模式參數。direct callers 位於 `0x36F12`、`0x36F52` 與 `0x3CD39`。

本切片只授權 CPU 的 `OR r8,[base+disp8]`；DOS open 的檔名、分享模式與
host 映射仍由下一個 DOS 服務規格決定。

一次性 IDA JSON 證據 SHA-256：
`8985f2455215080e78534dafff7de00d14493301fd383c3e925a221baa7712c9`。
