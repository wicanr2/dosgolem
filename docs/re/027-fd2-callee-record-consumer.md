# 027 — FD2 啟動 record 結構與 pointer 消費端

日期：2026-09-06  
證據等級：**已證實**（固定雜湊 raw bytes、table bytes、load／branch consumer）

沿用 `026` 的 FD2.EXE 身分與 LE 線性位址。八筆六 byte records 的固定 bytes 為：

```text
00 20 D0 6C 03 00   00 20 9F DC 03 00
00 20 20 5E 04 00   00 01 CC CB 03 00
00 02 D5 60 04 00   00 20 14 61 04 00
00 20 F8 68 04 00   00 20 FD CB 04 00
```

直接 consumer：

```text
0x45DC4  3B DF       cmp ebx,edi
0x45DC6  74 10       jz 0x45DD8
0x45DC8  8B 43 02    mov eax,[ebx+2]
0x45DCB  0B C0       or eax,eax
0x45DCD  74 04       jz 0x45DD3
0x45DCF  1E          push ds
```

配合 `026` 的比較與後續 `FF D0` 間接 CALL，record shape 可證實為：offset 0 狀態、
offset 1 排序值、offset 2 32-bit callback pointer。固定掃描選中 `0x539C2`，載入
EAX=`0x3CBCC`，非零 gate 不採分支並抵達 `0x45DCF`。

此結構屬啟動 callback 表是強推論；是否為特定編譯器的 initializer／terminator
分類仍未由工具鏈 signature 證實，不把該推論寫成函式名稱。
