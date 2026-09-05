# 068 — FD2 AIL 資料 selector 保存

日期：2026-09-06
證據等級：原始位元組、指令、writer 與直接 consumer 為**已證實**
輸入：`FD2.EXE`，大小 `357074`，MD5 `b97caf2239a27a896069d03549d96e1e`，
SHA-256 `222b7d067ad4450eb9c5f6e6bce1797d54bb050417ba39ced6067f8039f28c4f`
工具：IDA Pro 9.4.0.260610，映像 `ida-pro-9.4-idapython:locked-v1`
位址空間：IDA 載入 LE 映像後的 32 位線性位址

在已登錄的 AIL 中斷設定 `sub_3E92C` 中，`0x3E932..0x3E934` 先以
`push ds`／`pop es` 令 ES=DS，再執行 `cld`。`0x3E935` 的原始位元組／指令為
`66 8C 1D EE 2B 05 00`／`mov word ptr [52BEEh],ds`；`0x3E93C` 隨即以
`66 8E 05 EE 2B 05 00` 將同一欄位載回 ES，是直接 consumer。

這證實 `word_52BEE` 保存 AIL 初始化使用的資料 selector；它是 DOS extender
平台狀態，不是 FD2 遊戲資料。位元組匯出涵蓋 `0x3E930:0x14`。

一次性 IDA 位元組報告 SHA-256：
`677e586277ff94e4de16ffbf5b6659897bc3615aae15f823fbd2f3950a5d5c04`；
一次性 `.i64` SHA-256：
`65f362abe407e8508752b0f422d8da1f46381a594f52f7a24043223b2b68b145`。
