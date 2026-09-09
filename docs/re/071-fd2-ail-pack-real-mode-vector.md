# 071 — FD2 AIL 打包實模式中斷向量

日期：2026-09-06
證據等級：指令、資料來源與 consumer 為**已證實**
輸入：`FD2.EXE`，大小 `357074`，MD5 `b97caf2239a27a896069d03549d96e1e`，
SHA-256 `222b7d067ad4450eb9c5f6e6bce1797d54bb050417ba39ced6067f8039f28c4f`
工具：IDA Pro 9.4.0.260610，映像 `ida-pro-9.4-idapython:locked-v1`
位址空間：IDA 載入 LE 映像後的 32 位線性位址

`sub_3E92C` 在 `0x3E9B2` 以 DPMI `INT 31h/AX=0200h` 取得 CX:DX
形式的實模式向量。IDA 指令與資料流顯示：

- `0x3E9B4 shl ecx, 10h`：將 segment 移至高 16 位元；
- `0x3E9B7 mov cx, dx`：把 offset 放入低 16 位元；
- `0x3E9D3 mov dword_52BDA, ecx`：保存打包後的 `segment:offset`。

因此 `0x3E9B7` 所需 CPU 能力只是操作數寬度前綴下的暫存器對暫存器
`MOV r16, r/m16`（此處 ModRM=`CAh`，來源與目的均為暫存器）。本證據不支持
一般化任何 16 位元記憶體 ModRM 形狀。
