# 111 — FD2 INI 行尾分類掃描

日期：2026-09-06  
證據等級：函式、stack 緩衝位址、索引迴圈、分類表 consumer 與清零 writer
為**已證實**；「空白分類」為**強推論**

輸入為固定原版 `FD2.EXE`（大小 `357074`；MD5
`b97caf2239a27a896069d03549d96e1e`；SHA-256
`222b7d067ad4450eb9c5f6e6bce1797d54bb050417ba39ced6067f8039f28c4f`）。
工具為 IDA Pro 9.4，映像 `ida-pro-9.4-idapython:locked-v1`；位址均為
IDA 線性位址。

原始函式 `sub_3F306` 範圍 `0x3F306..0x3F5E7`，caller 位於 `0x37C81`。
它先以 `sub_46C4C` 讀入 stack 行緩衝，隨後執行：

```text
0x3F35B  8A 84 34 18 01 00 00  mov al,[esp+esi+118h]
0x3F362  FE C0                 inc al
0x3F364  25 FF 00 00 00        and eax,0FFh
0x3F369  F6 80 40 18 05 00 02  test byte_51840[eax],2
0x3F370  74 0E                 jz 0x3F380
0x3F372  30 F6                 xor dh,dh
0x3F374  88 B4 34 18 01 00 00  mov [esp+esi+118h],dh
0x3F37B  4E                    dec esi
0x3F37C  85 F6                 test esi,esi
0x3F37E  7D DB                 jge 0x3F35B
```

`[esp+esi+118h]` 是已讀入的 stack 行緩衝元素；表格 bit 2 成立時該元素被
寫成 NUL，ESI 遞減繼續。依行字串脈絡將其描述為行尾空白裁切是強推論，
但索引讀取、分類表與清零 consumer 均為直接指令證據。

本證據授權 `8A /r` 的 `mod=2` SIB＋disp32 byte 載入；對稱的 `88 /r`
寫入需在實際抵達後另行驗收。
