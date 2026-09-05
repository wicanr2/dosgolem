# 026 — FD2 第一個 callee 六 byte record 掃描

日期：2026-09-06  
證據等級：**已證實**（固定雜湊 raw bytes、dosgolem loop receipt）

沿用 `025` 的 FD2.EXE 身分與 LE 線性位址：

```text
0x45DB0  80 3E 02      cmp byte ptr [esi],2
0x45DB3  74 0A         jz 0x45DBF
0x45DB5  38 46 01      cmp [esi+1],al
0x45DB8  77 05         ja 0x45DBF
0x45DBA  8B DE         mov ebx,esi
0x45DBC  8A 46 01      mov al,[esi+1]
0x45DBF  83 C6 06      add esi,6
0x45DC2  EB E8         jmp 0x45DAC
0x45DC4                 range-exit
```

表格範圍為 `0x539B0..0x539DF`，固定 stride 6，共八筆。固定載入映像讓迴圈自然
掃描至 ESI=`0x539E0`，再由 `CMP ESI,EDI` 的 equal 結果採用 JAE，抵達
`0x45DC4`。實際資料使迴圈更新候選，最後 EBX=`0x539C2`、AL=`0x01`；欄位高層
用途尚未證實，因此不把該 record 或比較方向附加成語意名稱。
