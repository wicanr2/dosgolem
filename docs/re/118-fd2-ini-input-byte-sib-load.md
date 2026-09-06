# RE 118 — INI 數值解析器讀取目前輸入字元

日期：2026-09-06
證據等級：**已證實**（原始指令、函式邊界、呼叫者與直接 consumer）

## 輸入與工具

- `FD2.EXE`：大小 `357074`；MD5
  `b97caf2239a27a896069d03549d96e1e`；SHA-256
  `222b7d067ad4450eb9c5f6e6bce1797d54bb050417ba39ced6067f8039f28c4f`。
- IDA Pro 9.4，映像 `ida-pro-9.4-idapython:locked-v1`；一次性資料庫與 JSON
  只寫入 `/tmp/fd2-ida-3f2ac`。
- 本文位址均為 IDA 線性位址。原始函式名與邊界是
  `sub_3F264`、`0x3F264..0x3F306`；直接呼叫者為
  `0x3F4E6`、`0x3F513`、`0x3F540`、`0x3F571`。

## 原始指令與資料流

```text
0x3F2A5  0F B6 B3 B4 11 05 00  movzx esi,byte ptr [ebx+0x511B4]
0x3F2AC  8A 04 2F                 mov   al,byte ptr [edi+ebp]
0x3F2AF  25 FF 00 00 00           and   eax,0xFF
0x3F2B4  50                       push  eax
0x3F2B5  E8 30 79 00 00           call  toupper
0x3F2BA  83 C4 04                 add   esp,4
0x3F2BD  39 F0                    cmp   eax,esi
0x3F2BF  75 0F                    jnz   0x3F2D0
```

`8A 04 2F` 的 ModRM `04` 要求 SIB；SIB `2F` 是 scale 0、index EBP、base EDI，
所以有效位址是 `EDI+EBP`，預設資料區段是 DS。目的 `reg=0` 是 AL。後續立即把
EAX 限制為 unsigned byte、呼叫 `toupper`，再與 ESI 中的 digit-table 候選比較；
這條 consumer 鏈證實它讀的是目前輸入字元，而不是指標或多位元欄位。

本證據只授權無前綴的固定 `8A 04 2F`。它不證明其他 `8A` SIB 組合、scale、
segment override 或一般化記憶體運算元都已可安全執行。
