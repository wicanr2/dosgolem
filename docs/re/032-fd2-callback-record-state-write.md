# 032 — FD2 callback record 執行後狀態回寫

日期：2026-09-06  
證據等級：**已證實**（固定雜湊 raw bytes、callback return、memory diff）

沿用 `031` 的 FD2.EXE 身分與 LE 線性位址：

```text
0x45DD3  C6 03 02   mov byte ptr [ebx],2
0x45DD6  EB C6      jmp 0x45D9E
```

x87 callback 返回後 EBX=`0x539C2`。執行 `C6 03 02` 使選中 record 由
`00 01 CC CB 03 00` 變成 `02 01 CC CB 03 00`，只有 offset 0 改變；order 與
callback pointer 均保持。結合掃描時先排除 status=2，可證實 2 表示該 dispatcher
不應再次選取的已處理狀態；更高層生命週期名稱仍不延伸。
