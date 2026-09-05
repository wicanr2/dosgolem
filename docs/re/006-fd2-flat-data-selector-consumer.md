# 006 — FD2 flat data selector 的第一個 consumer

日期：2026-09-06
證據等級：**已證實**（bytes／控制流）＋**平台規格推導**（selector base）

## 固定輸入

沿用 [`005`](005-fd2-load-es-selector.md) 的 `FD2.EXE` 大小與雜湊，位址均為
DOS/4GW 載入後的 LE 線性位址。

```text
0x3CAB8  8E C3                    mov es,bx
0x3CABA  26 8C 1D D8 C9 03 00     mov es:[0x3C9D8],ds
```

固定啟動狀態在第一筆指令前為 `BX=DS=SS=0x0160`。所以第二筆是 selector
`0x0160` 的第一個直接 segment-memory consumer，寫入值也是 `0x0160`。

## DOS/4GW 契約

Open Watcom 的 DOS/4GW 文件列出 DPMI `0006h`／`0007h`／`0008h` 分別取得 base、
設定 base、設定 limit，並說明 flat model 的 DS 與 CS 具有相同 base：
<https://openwatcom.org/ftp/manuals/current/pguide.pdf>。

FD2 的 LE object 1 relocation 範圍是 `0x10000..0x4EBD8`，operand
`0x3C9D8` 落在該範圍。綜合 flat-model 規格與實際 operand，本切片採
`0x0160 -> base 0, limit 0xFFFFFFFF, writable`。這是硬體／extender 規格近似，
不是 DOSBox 逐週期證據。

## 邊界

只有 DOS/4GW 啟動服務明確註冊的 selector 可以轉換；未知 selector、超出 limit、
寫入唯讀 descriptor 或未支援 addressing form 一律失敗。`0x0028:0x2C` 環境 cell
仍是既有固定 oracle，不據此建立一般 descriptor。
