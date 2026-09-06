# 085 — FD2 MDI.INI 載入函式 stack frame

日期：2026-09-06
證據等級：函式、caller、stack 大小與後續 consumer 為**已證實**
輸入：`FD2.EXE`，大小 `357074`，MD5 `b97caf2239a27a896069d03549d96e1e`，
SHA-256 `222b7d067ad4450eb9c5f6e6bce1797d54bb050417ba39ced6067f8039f28c4f`
工具：IDA Pro 9.4.0.260610，映像 `ida-pro-9.4-idapython:locked-v1`
位址空間：IDA 載入 LE 映像後的 32 位線性位址

`sub_43EF0`（`0x43EF0..0x43F43`）由 `sub_3A722` 在 `0x3A77C` 呼叫。
它保存 ESI／EDI 後，於 `0x43EF2` 執行 `sub esp,118h` 配置本地 buffer；
`0x43EF8..0x43F02` 將 `"MDI.INI"` 與 buffer 指標傳給 `sub_38113`。
函式尾端 `0x43F3A add esp,118h` 對稱釋放。

因此此切片只需無前綴 `81 /5`、`mod=3` 的 `SUB r32,imm32`。此函式已
位於 AIL 初始化之後，後續將進入檔案服務；本切片不猜 `sub_38113` 的完整
ABI，待實際 caller／consumer 與 DOS 呼叫閉合後另立規格。

一次性 IDA 報告 SHA-256：
`006f5952afa1571c77f3a5c16105487193877ba77dd475d2b7355477a291b78e`。
