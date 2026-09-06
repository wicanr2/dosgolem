# 091 — FD2 C runtime tolower 大寫上界分支

日期：2026-09-06
證據等級：函式、原始位元組、比較來源與分支目標為**已證實**
輸入：`FD2.EXE`，大小 `357074`，MD5 `b97caf2239a27a896069d03549d96e1e`，
SHA-256 `222b7d067ad4450eb9c5f6e6bce1797d54bb050417ba39ced6067f8039f28c4f`
工具：IDA Pro 9.4，映像 `ida-pro-9.4-idapython:locked-v1`
位址空間：IDA 載入 LE 映像後的 32 位線性位址

`tolower`（`0x3D7E1..0x3D7F6`）在 `0x3D7EB` 比較 EAX 與 `'Z'`；
`0x3D7EF` 原始位元組 `7F 03` 是 `jg 0x3D7F4`。未跳轉時
`0x3D7F1` 將 EAX 加 `0x20`，因此此 signed-greater 分支排除大於 `'Z'`
的輸入。caller 位於 `0x36E1B` 與 `0x36EDF`。

一次性 IDA JSON 證據 SHA-256：
`bc7e3cd354e6c449528e9dfd7753ea8de0da79599b93e58ceb86dd68ff38a90b`。
