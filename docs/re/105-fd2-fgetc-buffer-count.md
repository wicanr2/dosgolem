# 105 — FD2 `fgetc` 緩衝計數欄位

日期：2026-09-06  
證據等級：函式、欄位 offset、讀寫端與 refill consumer 為**已證實**

輸入為固定原版 `FD2.EXE`（大小 `357074`；MD5
`b97caf2239a27a896069d03549d96e1e`；SHA-256
`222b7d067ad4450eb9c5f6e6bce1797d54bb050417ba39ced6067f8039f28c4f`）。
工具為 IDA Pro 9.4，映像 `ida-pro-9.4-idapython:locked-v1`；位址均為
IDA 線性位址。

原始函式 `fgetc` 範圍 `0x3D9C1..0x3DA3A`，由 `sub_46C4C` 的
`0x46C71` 直接呼叫。核心資料流：

```text
0x3D9E4  FF 4B 04       dec dword ptr [ebx+4]
0x3D9E7  83 7B 04 00    cmp dword ptr [ebx+4],0
0x3D9EB  7D 0B          jge 0x3D9F8
0x3D9ED  53             push ebx
0x3D9EE  ...            call __filbuf
0x3D9F8  8B 03          mov eax,[ebx]
0x3D9FA  8A 00          mov al,[eax]
0x3D9FF  FF 03          inc dword ptr [ebx]
```

`[ebx+4]` 在取 byte 前遞減，非負時直接由 `[ebx]` 指向的 buffer 讀取；
負值時呼叫 `__filbuf` 補充緩衝。相同序列在 `0x3DA0C` 處理 CR 後的下一個
byte。因此 offset `+4` 是剩餘緩衝 byte 計數，具直接 writer 與 consumer。

本證據授權 `FF /1 base+disp8`、`8A /r` 無位移基址間接 byte 載入，以及
`FF /0` 無位移基址間接 dword 遞增的必要 CPU shape；`__filbuf` 與 DOS
read 由後續獨立證據與規格閉合。
