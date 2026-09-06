# RE 121 — INI IO_ADDR 解析結果寫入設定欄位

日期：2026-09-06
證據等級：**已證實**（原始指令、函式、呼叫者與直接資料流）

## 輸入與工具

- `FD2.EXE`：大小 `357074`；MD5
  `b97caf2239a27a896069d03549d96e1e`；SHA-256
  `222b7d067ad4450eb9c5f6e6bce1797d54bb050417ba39ced6067f8039f28c4f`。
- IDA Pro 9.4，映像 `ida-pro-9.4-idapython:locked-v1`；一次性資料庫與 JSON
  只寫入 `/tmp/fd2-ida-3f4ee`。
- 位址均為 IDA 線性位址。原始函式是 `sub_3F306`，邊界
  `0x3F306..0x3F5E7`；直接呼叫者是 `0x3817C`。

## 原始指令與資料流

```text
0x3F4D0  68 D6 11 05 00           push offset "IO_ADDR"
0x3F4D5  53                       push ebx
0x3F4D6  E8 24 77 00 00           call strnicmp
0x3F4DE  85 C0                    test eax,eax
0x3F4E0  75 19                    jnz  0x3F4FB
0x3F4E2  50                       push eax
0x3F4E3  6A 10                    push 0x10
0x3F4E5  57                       push edi
0x3F4E6  E8 79 FD FF FF           call sub_3F264
0x3F4EB  83 C4 0C                 add  esp,0x0C
0x3F4EE  66 89 84 24 00 01 00 00  mov  word ptr [esp+0x100],ax
```

`sub_3F264` 是 RE 115–120 已閉合的 radix 數值解析器；此處固定以 radix 16
解析 `IO_ADDR` 的值。operand-size override `66` 使 opcode `89` 寫入 word。
ModRM `84` 指定 disp32 SIB 記憶體目的，SIB `24` 是 scale 0、無 index、base
ESP，所以目的為 `SS:[ESP+0x100]`，來源 reg=0 是 AX。

相鄰 `IRQ` 分支在 `0x3F51B` 使用相同形狀將 AX 寫到 `SS:[ESP+0x102]`，支持
這是 16 位設定欄位寫入族，而不是 32 位 store。本文不把 stack-local 名稱或
後續硬體用途提升成超過直接資料流的結論。

本證據只授權無 segment／repeat prefix 的 `66 89 84 24 disp32`、來源 AX、
SS:ESP base。其他來源暫存器、SIB、base 或 32 位 store 維持未知。
