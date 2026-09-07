# 015 — FD2 command-tail buffer 收尾

日期：2026-09-06
證據等級：**已證實**（固定雜湊 bytes、前置資料流）

沿用 `014` 的 FD2.EXE 身分與 LE 線性位址。零長度分支落在：

```text
0x3CB01  2A C0    sub al,al
0x3CB03  AA       stosb
0x3CB04  AA       stosb
0x3CB05  5E       pop esi
0x3CB06  4F       dec edi
0x3CB07  57       push edi
0x3CB08  52       push edx
0x3CB09  26 8E 1D 38 28 05 00
```

輸入 `ES=0x160`、`EDI=0x546B0`、stack top=0、`EDX=0x160`。前七筆將兩個
zero bytes 寫至 flat data buffer，恢復 `ESI=0`，令 `EDI=0x546B1`，再依序 push
EDI 與 EDX。最後一筆開始載入 environment selector，另立切片。
