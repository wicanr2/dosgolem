# RE 122 — INI 數值解析器反轉負號乘數

日期：2026-09-06
證據等級：**已證實**（原始指令、函式、呼叫者與直接 consumer）

## 輸入與工具

- `FD2.EXE`：大小 `357074`；MD5
  `b97caf2239a27a896069d03549d96e1e`；SHA-256
  `222b7d067ad4450eb9c5f6e6bce1797d54bb050417ba39ced6067f8039f28c4f`。
- IDA Pro 9.4，映像 `ida-pro-9.4-idapython:locked-v1`；一次性資料庫與 JSON
  只寫入 `/tmp/fd2-ida-3f289`。
- 位址均為 IDA 線性位址。原始函式是 `sub_3F264`，邊界
  `0x3F264..0x3F306`；直接呼叫者是 `0x3F4E6`、`0x3F513`、`0x3F540`、
  `0x3F571`。

## 原始指令與資料流

```text
0x3F273  C7 44 24 04 01 00 00 00  mov  dword ptr [esp+4],1
0x3F27F  8D 04 2F                 lea  eax,[edi+ebp]
0x3F282  8A 10                    mov  dl,[eax]
0x3F284  80 FA 2D                 cmp  dl,2Dh
0x3F287  75 06                    jnz  0x3F28F
0x3F289  F7 5C 24 04              neg  dword ptr [esp+4]
0x3F28D  EB 4E                    jmp  0x3F2DD
```

RE 115 已證實 `0x3F273` 將第二個本地 dword 初始化為 1。此路徑將目前輸入
字元與 ASCII `'-'` 比較；相等時，`F7 /3` 透過 ModRM `5C`、SIB `24` 與
disp8 `04` 對 `SS:[ESP+4]` 執行 32 位 NEG，使該值成為 `-1`。函式尾端將
解析累加值乘上此欄位，因此它是符號乘數，而非隨機狀態。

本證據只授權無 prefix 的 `F7 5C 24 disp8`、`/3 NEG`、SS:ESP base。其他
F7 記憶體形狀、SIB、運算群組或 segment override 維持未知。
