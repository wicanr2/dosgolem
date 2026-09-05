# 016 — FD2 載入 environment selector

日期：2026-09-06
證據等級：**已證實**（固定雜湊 bytes、writer／consumer）

沿用 `015` 的 FD2.EXE 身分與 LE 線性位址。`0x3CAC7` 已把從 PSP
`ES:[0x2C]` 取得的 environment selector `0x0030` 寫入 `[0x52838]`；consumer 為：

```text
0x3CB09  26 8E 1D 38 28 05 00    mov ds,word es:[0x52838]
0x3CB10  2B ED                   sub ebp,ebp
0x3CB12  8B 06                   mov eax,[esi]
```

在 `0x3CB09`，ES 是具完整 descriptor 的 flat selector `0x0160`，所以來源 word
解析為 `0x0030`。PSP `0x2C` 的 DOS 契約與前述 writer／consumer 證明它可載入 DS；
但一般 environment block base／內容尚未在本切片提供。執行應停在第一筆內容讀取。
