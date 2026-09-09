# 066 — FD2 AIL 預設表格迴圈上限

日期：2026-09-06
證據等級：函式邊界、原始指令、資料寫入與分支 consumer 為**已證實**；
`dword_541B4` 的完整欄位語意為**未知**
輸入：`FD2.EXE`，大小 `357074`，MD5 `b97caf2239a27a896069d03549d96e1e`，
SHA-256 `222b7d067ad4450eb9c5f6e6bce1797d54bb050417ba39ced6067f8039f28c4f`
工具：IDA Pro 9.4.0.260610，映像 `ida-pro-9.4-idapython:locked-v1`
位址空間：IDA 載入 LE 映像後的 32 位線性位址

IDA 保留 `sub_3F959`（`0x3F959..0x3FA5F`），由 AIL 啟動函式
`0x379EE..0x37B88` 在 `0x37A1A` 與 `0x37B05` 直接呼叫。函式先設定
18 個已列 preference 項目；`0x3FA3F` 清零 EAX，`0x3FA45` 以
`dword_541B4[eax*4]` 寫零，`0x3FA4C` 增加 EAX。

`0x3FA4D` 的原始位元組／指令為 `83 F8 10`／`cmp eax,10h`；緊接的
`0x3FA50` 使用 `jl 0x3FA43` 消費 signed 比較旗標，形成 16 項清零迴圈。
這足以證實比較與迴圈界線，不替表格內容猜測玩家可見語意。

一次性 IDA 報告 SHA-256：
`6f52b37712799b26d5085e2d2e437cb39bc01db1a09a02c294f1aff012f9c5bc`；
一次性 `.i64` SHA-256：
`a0ff96af637c05916384a16ef3d0a317a6a4324ba26ced21b22d8243db35360d`。
