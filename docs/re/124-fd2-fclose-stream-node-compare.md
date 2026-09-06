# RE 124 — fclose 比較串流節點中的 FILE 指標

日期：2026-09-06
證據等級：**已證實**（原始符號、指令、呼叫者與直接資料流）

## 輸入與工具

- `FD2.EXE`：大小 `357074`；MD5
  `b97caf2239a27a896069d03549d96e1e`；SHA-256
  `222b7d067ad4450eb9c5f6e6bce1797d54bb050417ba39ced6067f8039f28c4f`。
- IDA Pro 9.4，映像 `ida-pro-9.4-idapython:locked-v1`；一次性資料庫與 JSON
  只寫入 `/tmp/fd2-ida-3725a`。
- 位址均為 IDA 線性位址。原始函式符號是 `fclose`，邊界
  `0x37244..0x37270`。IDA 列出 24 個直接呼叫者；固定 FD2 設定解析路徑的
  caller 是 `0x3F587`。

## 原始指令與資料流

```text
0x37247  8B 55 08        mov  edx,[ebp+8]
0x3724A  A1 AC 41 05 00  mov  eax,dword_541AC
0x3724F  85 C0           test eax,eax
0x37251  75 07           jnz  0x3725A
0x37253  B8 FF FF FF FF  mov  eax,-1
0x37258  EB 14           jmp  0x3726E
0x3725A  3B 50 04        cmp  edx,[eax+4]
0x3725D  74 04           jz   0x37263
0x3725F  8B 00           mov  eax,[eax]
0x37261  EB EC           jmp  0x3724F
0x37263  6A 01           push 1
0x37265  52              push edx
0x37266  E8 05 00 00 00  call __shutdown_stream
```

函式參數由 `[EBP+8]` 載入 EDX。EAX 從 `dword_541AC` 取得鏈結串列首節點；
不相等路徑以 `[EAX]` 取得下一節點，相等路徑則把 EDX 傳給原始符號
`__shutdown_stream`。因此 `[EAX+4]` 是供 fclose 配對的 FILE 指標，而
`0x3725A` 是目前節點的配對比較。

ModRM `50` 是 mod=01、reg=EDX、r/m=EAX，disp8 是 `04`，右運算元使用 DS。
本文只授權無 prefix 的 `3B 50 disp8`；其他來源暫存器、base、SIB 或 segment
override 維持未知。
