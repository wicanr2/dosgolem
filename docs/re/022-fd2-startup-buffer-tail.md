# 022 — FD2 啟動緩衝區尾數與對齊

日期：2026-09-06  
證據等級：**已證實**（固定雜湊 raw bytes、dosgolem 執行狀態）

沿用 `021` 的 FD2.EXE 身分與 LE 線性位址：

```text
0x3CB6C  F3 AA                 rep stosb
0x3CB6E  B8 B0460500           mov eax,0x546B0
0x3CB73  05 0F000000           add eax,0x0F
0x3CB78  24 F0                 and al,0xF0
0x3CB7A  A3 08280500           mov [0x52808],eax
0x3CB7F  89 35 0C280500        mov [0x5280C],esi
0x3CB85  E8 10920000           call ...
```

前段留下的 ECX byte 餘數在固定路徑為 0，因此 `REP STOSB` 不寫入；接著以
`(0x546B0+0x0F) & ...F0` 取得 `0x546B0`，寫入全域，並保存 ESI=`0x546B1`。
執行於第一個 CALL 前停止；callee 語意尚未在本切片判讀。
