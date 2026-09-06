# 113 — FD2 INI key／value 字串分界

日期：2026-09-06  
證據等級：函式、pointer 來源、NUL writer、後續 key 比較與 value copy
consumer 為**已證實**

輸入為固定原版 `FD2.EXE`（大小 `357074`；MD5
`b97caf2239a27a896069d03549d96e1e`；SHA-256
`222b7d067ad4450eb9c5f6e6bce1797d54bb050417ba39ced6067f8039f28c4f`）。
工具為 IDA Pro 9.4，映像 `ida-pro-9.4-idapython:locked-v1`；位址均為
IDA 線性位址。

原始函式 `sub_3F306` 範圍 `0x3F306..0x3F5E7`。前一切片將解析後 pointer
保存在 `SS:[ESP+168h]`，此處重新載入並寫 NUL：

```text
0x3F442  8B 84 24 68 01 00 00  mov eax,[esp+168h]
0x3F449  C6 00 00              mov byte ptr [eax],0
0x3F44C  80 3B 3B              cmp byte ptr [ebx],';'
0x3F451  6A 07                 push 7
0x3F453  68 C8 11 05 00        push offset "DRIVER"
0x3F458  53                    push ebx
0x3F459  ...                   call strnicmp
```

`EAX` 指向掃描所得分界，NUL 將前段終止；EBX 隨後作註解字元與 `DRIVER`
等 key 比較，而另一個 stack buffer 後續接收 value。key／value 分界語意因
writer 與兩側 consumer 均已閉合，列為已證實。

本證據授權 `C6 /0` 的非 SIB、無位移基址間接 immediate byte store。
