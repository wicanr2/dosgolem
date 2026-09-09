# 030 — FD2 x87 startup callback 入口

日期：2026-09-06  
證據等級：**已證實**（固定雜湊 raw bytes、stack／x87 control receipt）

沿用 `029` 的 FD2.EXE 身分與 LE 線性位址。真正 callback 開頭：

```text
0x45E36  06         push es
0x45E37  51         push ecx
0x45E38  53         push ebx
0x45E39  52         push edx
0x45E3A  DB E3      fninit
0x45E3C  50         push eax
0x45E3D  D9 3C 24   fnstcw word ptr [esp]
0x45E40  58         pop eax
```

`FNINIT` 建立 x87 control word `0x037F`；`FNSTCW` 只覆寫已 push EAX stack cell 的
低 16 位，因此 SS:`0x55684` 成為 `0x0003037F`。這證實 callback 是 x87 初始化
路徑；不是 FD2 遊戲規則。後續固定資料 control word `0x127F` 的載入另開切片。
