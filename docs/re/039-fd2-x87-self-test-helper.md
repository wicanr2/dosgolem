# 039 — FD2 x87 自我測試 helper

日期：2026-09-06  
證據等級：原始指令與控制流為**已證實**；「DOS/4GW x87 自我測試」為**強推論**  
輸入：`FD2.EXE`，大小 `357074`，MD5 `b97caf2239a27a896069d03549d96e1e`，
SHA-256 `222b7d067ad4450eb9c5f6e6bce1797d54bb050417ba39ced6067f8039f28c4f`  
工具：dosgolem `feat/fd2-parity`，基線提交 `c7baf06`  
位址空間：dosgolem 載入 LE 映像後的 32 位元線性位址  
平台前提：Intel x87／80386 公開指令契約；不由 FD2.EXE 重新發明 CPU 語意

`0x460C6` 跳入的目的路徑如下：

```text
0x4CBD0  66 50        push ax
0x4CBD2  9B           wait
0x4CBD3  DB E3        fninit
0x4CBD5  D9 E8        fld1
0x4CBD7  D9 EE        fldz
0x4CBD9  DE F9        fdivp st(1),st(0)
0x4CBDB  D9 C0        fld st(0)
0x4CBDD  D9 E0        fchs
0x4CBDF  DE D9        fcompp
0x4CBE1  9B           wait
0x4CBE2  DF E0        fnstsw ax
0x4CBE4  B0 02        mov al,2
0x4CBE6  9E           sahf
0x4CBE7  0F 84 02000000  jz 0x4CBEF
0x4CBED  B0 03        mov al,3
0x4CBEF  9B           wait
0x4CBF0  DB E3        fninit
0x4CBF2  9B           wait
0x4CBF3  D9 2C 24     fldcw word ptr [esp]
0x4CBF6  66 87 04 24  xchg ax,word ptr [esp]
0x4CBFA  66 58        pop ax
0x4CBFC  C3           ret
```

以已證實前置 EAX=`0x127F` 執行時，helper 將原 AX 當作控制字保存於堆疊，
以 `1/0`、正負值比較及 x87 status word 經 `SAHF` 形成分支，最後重新初始化
x87、載回 `0x127F`，把分類結果留在 AX 並返回 `0x460FB`。

固定輸入的實際 dosgolem 執行結果為 AX=`0x0103`、ESP=`0x55694`、
FPUControl=`0x127F`、FPUStatus=0、FPUDepth=0；第二回呼的呼叫框架已正常解除。

這個 helper 沒有 DOS 中斷、I/O port 或遊戲資料存取。它應依公開 CPU 指令契約
實作，不應被描述成 FD2 特有玩法，也不構成「完整 x87 模擬」證據。
