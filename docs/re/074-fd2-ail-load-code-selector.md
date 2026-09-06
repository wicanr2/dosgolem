# 074 — FD2 AIL 載入 handler code selector

日期：2026-09-06
證據等級：指令、來源與 consumer 為**已證實**
輸入：`FD2.EXE`，大小 `357074`，MD5 `b97caf2239a27a896069d03549d96e1e`，
SHA-256 `222b7d067ad4450eb9c5f6e6bce1797d54bb050417ba39ced6067f8039f28c4f`
工具：IDA Pro 9.4.0.260610，映像 `ida-pro-9.4-idapython:locked-v1`
位址空間：IDA 載入 LE 映像後的 32 位線性位址

`sub_3E92C` 在 `0x3E9D9` 準備 interrupt 8、在 `0x3E9DE` 將
`sub_3E73E` 放入 EDX，並於 `0x3E9E3 mov bx,cs` 取得 code selector。
`0x3E9E9 mov ds,bx`（操作數寬度前綴下的 `8E DB`）把該 selector 載入
DS，直接由 `0x3E9EC INT 21h/AH=25h` 消費為 DS:EDX handler 指標。

因此 CPU 只需接受 `66 8E /r` 的 `mod=3` 暫存器來源；segment 載入有效性
仍由既有 `SegmentLoadOK` 契約裁決。本證據不支持其他記憶體 ModRM。
