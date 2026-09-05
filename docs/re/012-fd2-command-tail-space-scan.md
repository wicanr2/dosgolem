# 012 — FD2 command-tail 前導空白掃描

日期：2026-09-06
證據等級：**已證實**（固定雜湊 bytes／資料流）＋**x86 規格推導**

沿用 `011` 的 FD2.EXE 身分與 LE 線性位址。讀取 PSP command-tail length 後：

```text
0x3CAE6  FC             cld
0x3CAE7  B0 20          mov al,0x20
0x3CAE9  F3 AE          repe scasb
0x3CAEB  8D 77 FF       lea esi,[edi-1]
```

`F3 AE` 是 `REPE SCASB`，不是 `REPNE SCASB`。在 DF=0 時，它從 `ES:EDI`
向高位址掃描值等於空白的 bytes，最多 `ECX` 次；遇到非空白或計數歸零即停。
固定無參數啟動的 `ECX=0`，所以不讀記憶體且 EDI／flags 不變。

這組 bytes 與 PSP command-tail parser 一致，但本文件只把指令與資料流標為已證實；
高階 parser 名稱仍屬強推論。
