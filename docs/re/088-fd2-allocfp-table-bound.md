# 088 — FD2 C runtime FILE table 上界比較

日期：2026-09-06
證據等級：函式、原始位元組、比較值與分支 consumer 為**已證實**
輸入：`FD2.EXE`，大小 `357074`，MD5 `b97caf2239a27a896069d03549d96e1e`，
SHA-256 `222b7d067ad4450eb9c5f6e6bce1797d54bb050417ba39ced6067f8039f28c4f`
工具：IDA Pro 9.4，映像 `ida-pro-9.4-idapython:locked-v1`
位址空間：IDA 載入 LE 映像後的 32 位線性位址

固定原版從 `MDI.INI` 的 `fopen` 呼叫自然進入 `__allocfp`
（`0x3D802..0x3D897`）。`0x3D84F` 原始位元組為
`81 FB 48 2A 05 00`，即 `cmp ebx,0x52A48`；緊接的 `0x3D855`
為 `72 DA`（`jb 0x3D831`），以無號小於條件決定是否繼續掃描 FILE table。
函式的直接 caller 位於 `0x36FA6`。

本切片只證明 `CMP r32,imm32` 的 register 形狀及其旗標 consumer；`__allocfp`
是 IDA 還原的原始 C runtime 名稱，不等同於 dosgolem 已完成檔案服務。

一次性 IDA JSON 證據 SHA-256：
`bf84c566bcf552f98c7a324c35411ba2361292aa8641ad14cf0ac080df7bad5a`。
