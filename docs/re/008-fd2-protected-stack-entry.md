# 008 — FD2 protected-mode stack 第一筆 push

日期：2026-09-06
證據等級：**已證實**（固定雜湊 bytes／前一停止點）＋**平台規格推導**

沿用 `007` 的 FD2.EXE 身分與 LE 線性位址。環境 word 寫入後的下一筆指令是：

```text
0x3CACE  56                       push esi
0x3CACF  8E 05 10 28 05 00        mov es,word [0x52810]
```

此時 `ESI=0`、`ESP=0x556B0`、`SS=0x0160`。32-bit `PUSH ESI` 先令
`ESP=0x556AC`，再透過 SS descriptor 寫入 dword 0。`0x3CACF` 的 memory-source
segment load 是下一個獨立切片，本證據不授權它。
