# 110 — FD2 有界行讀取的目的緩衝寫入

日期：2026-09-06  
證據等級：函式邊界、來源字元、目的 pointer、loop 與 NUL consumer 為
**已證實**；「行讀取」名稱為**強推論**

輸入為固定原版 `FD2.EXE`（大小 `357074`；MD5
`b97caf2239a27a896069d03549d96e1e`；SHA-256
`222b7d067ad4450eb9c5f6e6bce1797d54bb050417ba39ced6067f8039f28c4f`）。
工具為 IDA Pro 9.4，映像 `ida-pro-9.4-idapython:locked-v1`；位址均為
IDA 線性位址。

原始函式 `sub_46C4C` 範圍 `0x46C4C..0x46CB6`，caller 位於 `0x3F481`。
EBX 由第一參數載入，`fgetc` 回傳值先存入 local，再由下列序列消費：

```text
0x46C81  8A 45 FC       mov al,[ebp-4]
0x46C84  88 03          mov [ebx],al
0x46C86  43             inc ebx
0x46C87  3C 0A          cmp al,0Ah
0x46C89  75 E0          jnz 0x46C6B
...
0x46CA5  C6 03 00       mov byte ptr [ebx],0
```

故 `0x46C84` 把已讀取字元寫入目前目的 pointer，EBX 隨後遞增；換行或長度
gate 後的 `0x46CA5` 寫入 NUL。這證實 EBX 是本 helper 的目的 buffer cursor，
並授權 `88 /r` 的非 SIB、無位移基址間接 byte store。
