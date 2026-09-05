# 021 — FD2 DOS/4GW 啟動緩衝區清除

日期：2026-09-06  
證據等級：**已證實**（固定雜湊 raw bytes、dosgolem 執行狀態）

沿用 `020` 的 FD2.EXE 身分與 LE 線性位址。恢復 flat DS 後的固定 bytes：

```text
0x3CB3C  5E                    pop esi
0x3CB3D  8B DC                 mov ebx,esp
0x3CB3F  66 89 2D 30280500     mov word ptr [0x52830],bp
0x3CB46  89 3D 14280500        mov [0x52814],edi
0x3CB4C  89 1D 00280500        mov [0x52800],ebx
0x3CB52  B9 B0460500           mov ecx,0x546B0
0x3CB57  BF EC390500           mov edi,0x539EC
0x3CB5C  2B CF                 sub ecx,edi
0x3CB5E  8A D1                 mov dl,cl
0x3CB60  C1 E9 02              shr ecx,2
0x3CB63  2B C0                 sub eax,eax
0x3CB65  F3 AB                 rep stosd
0x3CB67  8A CA                 mov cl,dl
0x3CB69  80 E1 03              and cl,3
```

這段保存啟動暫存器後，以 `0x539EC..0x546AF` 計算 dword 數量，令 EAX=0，透過
ES flat descriptor 清零，最後保留不足 dword 的 byte 餘數。固定路徑抵達
`0x3CB6C` 時 ECX=0、EDI=`0x546B0`、EAX=0。

上述是固定執行檔的啟動期緩衝區行為；區域更高層用途尚未證實，不附加遊戲語意。
