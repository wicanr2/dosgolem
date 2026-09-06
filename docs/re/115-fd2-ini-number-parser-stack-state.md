# RE 115 — INI 數值解析器的 stack 初始狀態

日期：2026-09-06  
證據等級：**已證實**（函式邊界、指令、caller 與後續 consumer）

## 輸入與工具

- `FD2.EXE`：大小 `357074`；MD5
  `b97caf2239a27a896069d03549d96e1e`；SHA-256
  `222b7d067ad4450eb9c5f6e6bce1797d54bb050417ba39ced6067f8039f28c4f`。
- IDA Pro 9.4.0.260610，映像 `ida-pro-9.4-idapython:locked-v1`。
- 本文位址均為 IDA 線性位址；原始函式名與邊界為
  `sub_3F264`、`0x3F264..0x3F306`。

## 原始控制流

- caller 位址為 `0x3F4E6`、`0x3F513`、`0x3F540`、`0x3F571`。
- `0x3F267 83 EC 08` 配置 8 bytes 區域，`0x3F26A` 讀入字串參數。
- `0x3F26E 31 D2` 將 EDX 清零。
- `0x3F270 89 14 24` 為 `mov [esp],edx`：ModRM `14`、SIB `24`，使用 SS
  segment、ESP base、無 index、無 displacement，初始化第一個本地 dword 為零。
- `0x3F273 C7 44 24 04 01 00 00 00` 將第二個本地 dword 初始化為 1；
  `0x3F27F..0x3F2DD` 隨後消費輸入字元、正負號與字元分類表，證明這兩個
  stack slot 屬於數值解析狀態，而不是一般絕對索引資料。
- 固定雜湊 FD2 自然執行已抵達 `0x3F270`。

本切片只要求 CPU 支援上述精確的 `[SS:ESP]` dword store；不把 SIB `24`
誤解為 absolute indexed addressing，也不一般化其他 SIB 組合。
