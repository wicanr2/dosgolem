# 013 — FD2 command-tail 指標整理

日期：2026-09-06
證據等級：**已證實**（固定雜湊 bytes、前一執行停止點）

沿用 `012` 的 FD2.EXE 身分與 LE 線性位址：

```text
0x3CAEB  8D 77 FF    lea esi,[edi-1]
0x3CAEE  8B FA       mov edi,edx
0x3CAF0  8C C3       mov ebx,es
```

無參數路徑輸入為 `EDI=0x81`、`EDX=0x546B0`、`ES=0x28`；執行後應為
`ESI=0x80`、`EDI=0x546B0`、`EBX=0x28`。LEA 只計算 effective address，
不讀記憶體、不改旗標。這些 register 值的後續用途尚待 consumer 證據。
