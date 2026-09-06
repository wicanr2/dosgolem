# 109 — FD2 `__filbuf` 消費補充後的第一個 byte

日期：2026-09-06  
證據等級：函式邊界、欄位讀寫與回傳 consumer 為**已證實**

輸入為固定原版 `FD2.EXE`（大小 `357074`；MD5
`b97caf2239a27a896069d03549d96e1e`；SHA-256
`222b7d067ad4450eb9c5f6e6bce1797d54bb050417ba39ced6067f8039f28c4f`）。
工具為 IDA Pro 9.4，映像 `ida-pro-9.4-idapython:locked-v1`；位址均為
IDA 線性位址。

IDA 原始函式 `__filbuf` 範圍為 `0x3DA3A..0x3DA65`。它呼叫
`__fill_buffer`，成功時執行：

```text
0x3DA55  FF 4B 04       dec dword ptr [ebx+4]
0x3DA58  FF 03          inc dword ptr [ebx]
0x3DA5A  8B 03          mov eax,[ebx]
0x3DA5C  8A 40 FF       mov al,[eax-1]
0x3DA5F  0F B6 C0       movzx eax,al
```

因此 `[ebx]` 是目前 buffer pointer：原始程式先把它增加一，再由新位置前一
byte 讀出剛補充的第一個字元並回傳；`[ebx+4]` 同時減一。本證據授權
`FF /0` 的非 SIB、無位移基址間接 dword 遞增，以及緊接的 `8B /r`
無位移基址間接 dword 載入；不外推其他 addressing shape。
