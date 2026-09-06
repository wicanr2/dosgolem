# 095 — FD2 C runtime 重讀開檔模式首字元

日期：2026-09-06
證據等級：函式、原始位元組與 consumer 為**已證實**

固定原版 `FD2.EXE`（大小 `357074`；MD5
`b97caf2239a27a896069d03549d96e1e`；SHA-256
`222b7d067ad4450eb9c5f6e6bce1797d54bb050417ba39ced6067f8039f28c4f`）在
IDA Pro 9.4 的 LE 線性位址 `0x36ED8` 載入模式字串參數至 EAX；
`0x36EDB` 原始位元組 `0F B6 00` 是 `movzx eax,byte ptr [eax]`，隨即推入
並呼叫 `tolower`。這是 `sub_36EBC` 對同一模式首字元的第二個 consumer。

原始指令來自 [`RE 089`](089-fd2-open-clears-file-mode-bits.md) 的 IDA
函式證據；SHA-256：
`f89cea59a92457981ecb3a99f773bd77af621ee8a3bc8f5353b9dccbc8351c28`。
