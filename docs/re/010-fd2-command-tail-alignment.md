# 010 — FD2 command-tail 前置算術

日期：2026-09-06
證據等級：**已證實**（固定雜湊 bytes、dosgolem 執行停止點）

沿用 `009` 的 FD2.EXE 身分與 LE 線性位址。恢復 `ES=0x0028` 後，dosgolem
自行執行 `mov edx,0x546B0`，並停於：

```text
0x3CADA  83 C2 0F       add edx,0x0F
0x3CADD  80 E2 F0       and dl,0xF0
0x3CAE0  2B C9          sub ecx,ecx
0x3CAE2  26 8A 4F FF    mov cl,es:[edi-1]
```

輸入為 `EDX=0x546B0`、`ECX=0x30`、`EDI=0x81`。前三筆使
`EDX=0x546B0`（先加 15 再將低 byte 對齊至 16-byte 邊界）及 `ECX=0`。
第四筆讀取 `ES:[0x80]`；它與 DOS PSP command-tail length 位置一致，但尚未在
本切片實作或升格為已證實的 FD2 consumer 語意。
