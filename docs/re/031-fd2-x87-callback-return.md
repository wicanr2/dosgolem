# 031 — FD2 x87 callback control 與返回

日期：2026-09-06  
證據等級：**已證實**（固定雜湊 raw bytes、control／stack／return receipt）

沿用 `030` 的 FD2.EXE 身分與 LE 線性位址：

```text
0x45E40  58                    pop eax
0x45E41  80 FC 03              cmp ah,3
0x45E44  74 01                 jz 0x45E47
0x45E47  0B ED                 or ebp,ebp
0x45E49  74 05                 jz 0x45E50
0x45E50  9B                    wait
0x45E51  DB E3                 fninit
0x45E53  9B                    wait
0x45E54  D9 2D 1C390500        fldcw [0x5391C]
0x45E5A  D9 EE                 fldz
0x45E5C  D9 EE                 fldz
0x45E5E  D9 EE                 fldz
0x45E60  D9 EE                 fldz
0x45E62  5A 5B 59 07 C3        pop edx; pop ebx; pop ecx; pop es; ret
```

原始 control word `[0x5391C]`=`0x127F`。FNSTCW 結果使 AH=3，且 EBP=0，兩個
條件分支都略過替代 CALL；callback 最終載入 `0x127F`、推入四個 0，恢復暫存器
與 ES，RET 至 dispatcher `0x45DD3`。此為 x87 runtime 初始化，非遊戲規則。
