# 083 — FD2 AIL 恢復中斷旗標前置檢查

日期：2026-09-06
證據等級：指令、stack 來源、mask 與控制流為**已證實**
輸入：`FD2.EXE`，大小 `357074`，MD5 `b97caf2239a27a896069d03549d96e1e`，
SHA-256 `222b7d067ad4450eb9c5f6e6bce1797d54bb050417ba39ced6067f8039f28c4f`
工具：IDA Pro 9.4.0.260610，映像 `ida-pro-9.4-idapython:locked-v1`
位址空間：IDA 載入 LE 映像後的 32 位線性位址

`sub_3E864` 在 `0x3E86A` 先 `pushf`、`0x3E86B` 執行 CLI。完成 PIT
輸出後，`0x3E882 push ebp`、`0x3E883 mov ebp,esp`，接著於 `0x3E885`
執行 `test byte ptr [ebp+5],2`。這個 byte 是先前保存 EFLAGS 的第二個
byte，mask `02h` 對應 IF；`0x3E88A jz` 在原 IF 未設定時跳過 STI，最後
`0x3E88E popf` 完整恢復原旗標。

本切片只需無前綴 `F6 /0`、base+disp8 的 byte 與 imm8 TEST；不改寫
stack byte 或通用暫存器。
