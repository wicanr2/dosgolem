# RE 120 — INI 數值累加值乘以 radix

日期：2026-09-06
證據等級：**已證實**（原始指令、函式邊界與直接 consumer）

## 輸入與工具

- `FD2.EXE`：大小 `357074`；MD5
  `b97caf2239a27a896069d03549d96e1e`；SHA-256
  `222b7d067ad4450eb9c5f6e6bce1797d54bb050417ba39ced6067f8039f28c4f`。
- IDA Pro 9.4，映像 `ida-pro-9.4-idapython:locked-v1`；沿用 RE 118 同一次
  雜湊驗證匯出。
- 位址均為 IDA 線性位址。原始函式為 `sub_3F264`，邊界
  `0x3F264..0x3F306`。

## 原始指令與資料流

```text
0x3F2C1  8B 04 24           mov  eax,dword ptr [esp]
0x3F2C4  0F AF 44 24 1C     imul eax,dword ptr [esp+0x1C]
0x3F2C9  01 D8              add  eax,ebx
0x3F2CB  89 04 24           mov  dword ptr [esp],eax
```

ModRM `44` 是 disp8 SIB 記憶體來源，SIB `24` 是 scale 0、無 index、base ESP；
因此來源為 `SS:[ESP+0x1C]`。RE 116 已證實該 stack 位置是 caller 傳入的 radix。
`IMUL` 的完整 32 位低半結果覆寫 EAX，後續加上 digit index EBX，再寫回
SS:[ESP] 的累加值。這是數值解析器的 `accumulator*radix+digit` 路徑。

本證據只授權無前綴 `0F AF 44 24 1C`。它不授權其他 `IMUL` 目的暫存器、
SIB、位移、operand-size 或 segment override。
