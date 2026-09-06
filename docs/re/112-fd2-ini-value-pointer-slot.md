# 112 — FD2 INI 解析後的 value pointer 暫存

日期：2026-09-06  
證據等級：函式、字串掃描、pointer 計算與 stack slot writer 為**已證實**；
「value pointer」名稱為**強推論**

輸入為固定原版 `FD2.EXE`（大小 `357074`；MD5
`b97caf2239a27a896069d03549d96e1e`；SHA-256
`222b7d067ad4450eb9c5f6e6bce1797d54bb050417ba39ced6067f8039f28c4f`）。
工具為 IDA Pro 9.4，映像 `ida-pro-9.4-idapython:locked-v1`；位址均為
IDA 線性位址。

原始函式 `sub_3F306` 範圍 `0x3F306..0x3F5E7`，caller 位於 `0x37C81`。
在行尾裁切後，它由行緩衝起點加 ESI，將所得 pointer 寫入 stack slot：

```text
0x3F3E7  8D 84 24 18 01 00 00  lea eax,[esp+118h]
0x3F3EE  01 F0                 add eax,esi
0x3F3F0  89 84 24 68 01 00 00  mov [esp+168h],eax
0x3F3F7  EB 18                 jmp 0x3F411
```

後續程式由同一函式比較 `DRIVER`、`IO_ADDR`、`IRQ`、`DMA_8_bit` 與
`DMA_16_bit` key，並把解析值寫入輸出結構。依此 consumer 將 stack slot
描述為 value pointer 是強推論；其 writer、來源 pointer 與位址則為已證實。

本證據授權 `89 /r` 的 `mod=2`、SIB `0x24`、disp32 stack dword store。
