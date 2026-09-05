# 014 — FD2 PSP／flat data selector 交換與零長度分支

日期：2026-09-06
證據等級：**已證實**（固定雜湊 bytes、writer／consumer、dosgolem 停止點）

沿用 `013` 的 FD2.EXE 身分與 LE 線性位址。已知 `EBX=0x28`（原 PSP selector）、
目前 `DS=0x160`（flat data selector），後續固定 bytes 為：

```text
0x3CAF2  8C DA                 mov edx,ds
0x3CAF4  8E DB                 mov ds,bx
0x3CAF6  8E C2                 mov es,dx
0x3CAF8  0F 84 03 00 00 00     jz 0x3CB01
0x3CAFE  41                    inc ecx
0x3CAFF  F3 A4                 rep movsb
0x3CB01  2A C0                 sub al,al
```

`0x28` 已由 PSP offset `0x2C` 與 `0x80` 的實際 consumer 證實為可讀 host
selector，因此允許載入 DS 與 ES；這不建立未知 base／limit。`0x160` 已是完整
flat descriptor。無參數路徑在 `sub ecx,ecx` 後維持 ZF=1，故 `JZ` 直接跳至
`0x3CB01`，不執行 `INC ECX`／`REP MOVSB`。
