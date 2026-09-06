# RE 123 — 有界行讀取在 EOF 比較目的游標與起點

日期：2026-09-06
證據等級：**已證實**（原始指令、函式、呼叫者與直接資料流）

## 輸入與工具

- `FD2.EXE`：大小 `357074`；MD5
  `b97caf2239a27a896069d03549d96e1e`；SHA-256
  `222b7d067ad4450eb9c5f6e6bce1797d54bb050417ba39ced6067f8039f28c4f`。
- IDA Pro 9.4，映像 `ida-pro-9.4-idapython:locked-v1`；一次性資料庫與 JSON
  只寫入 `/tmp/fd2-ida-46c91`。
- 位址均為 IDA 線性位址。原始函式是 `sub_46C4C`，邊界
  `0x46C4C..0x46CB6`；直接呼叫者是 `0x3F481`。

## 原始指令與資料流

```text
0x46C79  89 45 FC        mov  [ebp-4],eax
0x46C7C  83 F8 FF        cmp  eax,-1
0x46C7F  74 0A           jz   0x46C8B
0x46C81  8A 45 FC        mov  al,[ebp-4]
0x46C84  88 03           mov  [ebx],al
0x46C86  43              inc  ebx
0x46C8B  83 7D FC FF     cmp  dword ptr [ebp-4],-1
0x46C8F  75 14           jnz  0x46CA5
0x46C91  3B 5D 14        cmp  ebx,[ebp+0x14]
0x46C94  74 06           jz   0x46C9C
```

RE 104 與 RE 110 已證實 EBX 是逐字遞增的目的 buffer cursor；函式入口由
第一參數建立其起始值，而 Watcom stack frame 使 `[EBP+0x14]` 回讀該參數。
因此 EOF 路徑的 `0x46C91` 比較目前游標與起始位址；相等表示本次沒有寫入
任何字元。這項結論不依賴函式名稱猜測。

ModRM `5D` 是 mod=01、reg=EBX、r/m=EBP，disp8 為 `0x14`；目的運算元使用
SS。本文只授權無 prefix 的 `3B 5D disp8`，其他 base、來源暫存器、SIB 或
segment override 維持未知。
