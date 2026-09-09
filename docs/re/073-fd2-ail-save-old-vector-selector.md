# 073 — FD2 AIL 保存舊中斷向量 selector

日期：2026-09-06
證據等級：指令、來源與 consumer 為**已證實**
輸入：`FD2.EXE`，大小 `357074`，MD5 `b97caf2239a27a896069d03549d96e1e`，
SHA-256 `222b7d067ad4450eb9c5f6e6bce1797d54bb050417ba39ced6067f8039f28c4f`
工具：IDA Pro 9.4.0.260610，映像 `ida-pro-9.4-idapython:locked-v1`
位址空間：IDA 載入 LE 映像後的 32 位線性位址

`sub_3E92C` 在 `0x3E9BF` 以 DOS `INT 21h/AH=35h` 取得 interrupt 8
的 ES:EBX 向量。`0x3E9C1 mov dx,es`（原始形狀 `66 8C C2`）把 selector
複製至 DX，`0x3E9CC` 再將 DX 寫入 `word_52BD8`；EBX offset 則在
`0x3E9C6` 寫入 `dword_52BD4`。

因此 CPU 只需支援操作數寬度前綴下 `MOV r/m16,Sreg` 的 `mod=3`
暫存器目的形狀；本證據不支持擴張其他記憶體 ModRM。
