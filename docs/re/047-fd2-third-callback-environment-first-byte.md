# 047 — FD2 第三次回呼讀取 environment 首 byte

日期：2026-09-06  
證據等級：**已證實**（固定雜湊原始位元組、selector writer 與直接 consumer）  
輸入：`FD2.EXE`，大小 `357074`，MD5 `b97caf2239a27a896069d03549d96e1e`，
SHA-256 `222b7d067ad4450eb9c5f6e6bce1797d54bb050417ba39ced6067f8039f28c4f`  
工具：dosgolem `feat/fd2-parity`，基線提交 `0fd4dc6`  
位址空間：dosgolem 載入 LE 映像後的 32 位元線性位址

FS selector 與零 offset 準備完成後，原始指令為：

```text
0x4CC24  8E C0        mov es,eax
0x4CC26  8B 45 FC     mov eax,[ebp-4]
0x4CC29  26 80 38 00  cmp byte ptr es:[eax],0
0x4CC2D  74 12        jz 0x4CC41
```

前置狀態 EAX=`0x30`、`[EBP-4]=0`，因此 ES 載入同一具邊界 environment
selector，EAX 恢復為 offset 0，consumer 讀取 `ES:0`。dosgolem 的合法最小
environment 首 byte 為 0，故 ZF=1 並跳至 `0x4CC41`。

這個直接 consumer 證明 selector `0x30` 除了 DS／FS 也允許載入 ES；它只擴充
既有 host selector gate，不證明一般 descriptor base、limit 或 writable。
