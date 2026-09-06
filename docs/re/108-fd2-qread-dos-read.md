# 108 — FD2 `__qread` 的 DOS 檔案讀取

日期：2026-09-06  
證據等級：函式邊界、參數搬移、DOS 服務號、回傳值與錯誤 consumer 為
**已證實**

輸入為固定原版 `FD2.EXE`（大小 `357074`；MD5
`b97caf2239a27a896069d03549d96e1e`；SHA-256
`222b7d067ad4450eb9c5f6e6bce1797d54bb050417ba39ced6067f8039f28c4f`）。
工具為 IDA Pro 9.4，映像 `ida-pro-9.4-idapython:locked-v1`；位址均為
IDA 線性位址。

IDA 原始函式 `__qread` 範圍為 `0x3D990..0x3D9C1`，直接 caller 包含
`__fill_buffer` 的 `0x3DAE6`。其核心序列：

```text
0x3D994  8B 45 0C       mov eax,[ebp+0Ch]
0x3D997  8B 55 10       mov edx,[ebp+10h]
0x3D99A  8B 4D 14       mov ecx,[ebp+14h]
0x3D99D  66 89 C3       mov bx,ax
0x3D9A0  B4 3F          mov ah,3Fh
0x3D9A2  CD 21          int 21h
0x3D9A4  D1 D0          rcl eax,1
0x3D9A6  D1 C8          ror eax,1
0x3D9A8  89 C2          mov edx,eax
0x3D9AA  85 C0          test eax,eax
0x3D9AC  7D 0E          jge 0x3D9BC
0x3D9AE  0F B7 C0       movzx eax,ax
0x3D9B2  ...            call _set_errno
```

因此原始 runtime 把第一參數低 16 位送入 BX、第二參數送入 EDX、第三參數
送入 ECX，再呼叫 DOS `INT 21h/AH=3Fh`。DOS CF 經 `RCL/ROR` 轉成有號結果；
錯誤時 AX 是錯誤碼並交給 `_set_errno`，成功時 EAX 是實際讀取 byte 數。

固定原版自然執行至該中斷時，BX=`5`、CX=`0x1000`、EDX=`0x63518`；handle 5
是前一切片由唯讀 provider 開啟的 `MDI.INI`。本證據授權有界唯讀 DOS read；
不授權寫檔、裝置 handle、非 DS selector 或未登錄 handle 的寬鬆 fallback。
