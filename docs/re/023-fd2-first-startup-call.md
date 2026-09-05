# 023 — FD2 第一個啟動期 near CALL

日期：2026-09-06  
證據等級：**已證實**（固定雜湊 raw bytes、線性位址計算）

沿用 `022` 的 FD2.EXE 身分與 LE 線性位址：

```text
0x3CB85  E8 10920000   call rel32 +0x9210
0x3CB8A                 return address
0x45D9A                 calculated target
```

執行前 SS:ESP=`0x160:0x556B0`。32-bit near CALL 應把 `0x3CB8A` 寫入
SS:`0x556AC`，再令 EIP=`0x45D9A`。本切片只證實控制轉移；callee 功能語意維持未知。
