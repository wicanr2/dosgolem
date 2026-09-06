# 087 — FD2 MDI.INI 路徑參數載入

日期：2026-09-06
證據等級：函式、原始位元組、stack 位移與 consumer 為**已證實**
輸入：`FD2.EXE`，大小 `357074`，MD5 `b97caf2239a27a896069d03549d96e1e`，
SHA-256 `222b7d067ad4450eb9c5f6e6bce1797d54bb050417ba39ced6067f8039f28c4f`
工具：IDA Pro 9.4，映像 `ida-pro-9.4-idapython:locked-v1`
位址空間：IDA 載入 LE 映像後的 32 位線性位址

`sub_3F306` 在 `0x3F337` 推入字串 `"rt"` 後，`0x3F33C` 的原始位元組為
`8B 94 24 84 01 00 00`，即 `mov edx,[esp+184h]`。`0x3F343` 隨即推入
`edx`，`0x3F344` 呼叫 `fopen`；因此這次 stack dword read 的 consumer 是
`fopen` 的路徑參數。函式 caller 為 `sub_38113` 的 `0x3817C`。

此切片只證明無前綴 `MOV r32,[esp+disp32]` 的指令形狀與這個 consumer；
不證明 host 檔案映射、`fopen` ABI 或 `MDI.INI` 內容已經實作。

一次性 IDA JSON 證據 SHA-256：
`01b3b897d45411fb38732f2d0a4d5f652243065527421c623cbfd473cdf77019`。
