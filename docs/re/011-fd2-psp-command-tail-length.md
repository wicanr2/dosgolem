# 011 — FD2 讀取 PSP command-tail length

日期：2026-09-06
證據等級：**已證實**（bytes／資料流）＋**DOS 規格推導**

固定雜湊 FD2 在 `0x3CAE2` 執行 `26 8A 4F FF`，即以 `EDI=0x81` 讀取
`ES:[EDI-1]` 到 `CL`。此時 `ES=0x0028` 是啟動時保存並恢復的 PSP selector，
故 offset `0x80` 對應 command-tail length。Microsoft MS-DOS Programmer's Reference
亦定義 PSP `0x80` 為長度、`0x81` 起為內容；本次無參數啟動的長度為 0。

本切片不實作一般 ModR/M addressing，也不推論 command-tail 後續 parser 已完成。
