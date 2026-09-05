# 019 — FD2 environment 雙 NUL 掃描與路徑搬移

日期：2026-09-06  
證據等級：**已證實**（固定雜湊 bytes、dosgolem 執行狀態）

沿用 `018` 的 FD2.EXE 身分、雜湊與 LE 線性位址。prefix 判別採不相等分支後：

```text
0x3CB27  80 3E 00   cmp byte ptr [esi],0
0x3CB2A  AC         lodsb
0x3CB2B  75 FA      jnz 0x3CB27
0x3CB2D  80 3E 00   cmp byte ptr [esi],0
0x3CB30  75 E0      jnz 0x3CB12
0x3CB32  AC         lodsb
0x3CB33  46         inc esi
0x3CB34  46         inc esi
0x3CB35  80 3E 00   cmp byte ptr [esi],0
0x3CB38  A4         movsb
0x3CB39  75 FA      jnz 0x3CB35
0x3CB3B  1F         pop ds
```

dosgolem 提供的最小合法 environment 以雙 NUL 開始，後接 count=`1` 與程式路徑。
第一個迴圈逐 byte 略過 environment 字串直到雙 NUL；第二次 `LODSB` 消耗第二個
NUL，再略過 count word。`MOVSB` 迴圈將程式路徑 `FD2.EXE` 與終止 NUL 搬至 ES
目的緩衝區。

這份位址表以固定雜湊映像載入後的 raw byte assertion 再次核對；它也更正先前工作
摘要把 `0x3CB2A` 誤抄成 `JNZ`、把 `POP DS` 誤列為 `0x3CB3A` 的錯位。這證實控制流
與指令形狀，不證實原機器 environment 文字內容逐位元相同。後者維持 DOS 平台近似；
單元測試另以非零來源驗證 CMP 不寫回及 MOVSB 確實搬移資料。
