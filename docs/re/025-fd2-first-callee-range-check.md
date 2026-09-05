# 025 — FD2 第一個 callee 表格範圍比較

日期：2026-09-06  
證據等級：**已證實**（固定雜湊 raw bytes、dosgolem register／flags receipt）

沿用 `024` 的 FD2.EXE 身分與 LE 線性位址：

```text
0x45DAC  3B F7   cmp esi,edi
0x45DAE  73 14   jae 0x45DC4
0x45DB0  80 3E 02   cmp byte ptr [esi],2
```

固定狀態 ESI=`0x539B0`、EDI=`0x539E0`，unsigned compare 產生 CF=1、ZF=0，故
`JAE` 不採用並抵達 `0x45DB0`。表格 stride 與 record 高層語意尚未在此切片命名。
