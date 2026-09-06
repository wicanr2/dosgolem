# 098 — FD2 sopen 初始化本地 handle

日期：2026-09-06
證據等級：函式、原始位元組與後續 writer 為**已證實**

固定雜湊 `FD2.EXE` 的 `sopen` 在 IDA Pro 9.4 線性位址 `0x3CD6A`
執行 `C7 45 F8 FF FF FF FF`，即 `mov dword ptr [ebp-8],-1`；隨後
`0x3CD71` 設定 `AH=3Dh`、`0x3CD73` 執行 DOS open。成功路徑會在
`0x3CD82` 將零擴展後的 AX 寫回同一 `[ebp-8]`，證明此值是失敗預設的
本地 handle/result。

輸入 MD5 `b97caf2239a27a896069d03549d96e1e`；SHA-256
`222b7d067ad4450eb9c5f6e6bce1797d54bb050417ba39ced6067f8039f28c4f`。
IDA JSON 證據 SHA-256：
`8985f2455215080e78534dafff7de00d14493301fd383c3e925a221baa7712c9`。
