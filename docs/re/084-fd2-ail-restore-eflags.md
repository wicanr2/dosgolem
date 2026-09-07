# 084 — FD2 AIL 恢復 PIT 設定前 EFLAGS

日期：2026-09-06
證據等級：指令、stack 配對與 consumer 為**已證實**
輸入：`FD2.EXE`，大小 `357074`，MD5 `b97caf2239a27a896069d03549d96e1e`，
SHA-256 `222b7d067ad4450eb9c5f6e6bce1797d54bb050417ba39ced6067f8039f28c4f`
工具：IDA Pro 9.4.0.260610，映像 `ida-pro-9.4-idapython:locked-v1`
位址空間：IDA 載入 LE 映像後的 32 位線性位址

`sub_3E864` 在 `0x3E86A` 執行 `pushf`，完成 CLI、PIT port 輸出及原 IF
檢查後，於 `0x3E88D` 恢復暫存 EBP，緊接 `0x3E88E popf`。後續
`0x3E88F..0x3E893` 只恢復通用暫存器、LEAVE 並返回，因此 `POPFD` 的
直接用途是恢復函式進入時保存的 EFLAGS。

本切片只要求 32 位元 stack 的 `POPFD`（opcode `9Dh`）；不模擬 privilege
level 對 IF／IOPL 的遮罩，因目前 LE 執行環境沒有 ring 切換模型。
