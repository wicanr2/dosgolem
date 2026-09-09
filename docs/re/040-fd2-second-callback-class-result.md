# 040 — FD2 第二個啟動回呼保存分類結果

日期：2026-09-06  
證據等級：**已證實**（固定雜湊原始位元組與自然執行狀態）  
輸入：`FD2.EXE`，大小 `357074`，MD5 `b97caf2239a27a896069d03549d96e1e`，
SHA-256 `222b7d067ad4450eb9c5f6e6bce1797d54bb050417ba39ced6067f8039f28c4f`  
工具：dosgolem `feat/fd2-parity`，基線提交 `e7a30da`  
位址空間：dosgolem 載入 LE 映像後的 32 位元線性位址

x87 helper 返回 AX=`0x0103` 後，第二回呼執行：

```text
0x460FB  88 C3              mov bl,al
0x460FD  80 3D 30280500 00  cmp byte ptr [0x52830],0
0x46104  75 0C              jnz 0x46112
0x46106  88 1D F4270500     mov byte ptr [0x527F4],bl
0x4610C  88 1D F5270500     mov byte ptr [0x527F5],bl
0x46112  5B                 pop ebx
0x46113  C3                 ret
```

固定載入狀態 `[0x52830]=0`，因此不跳轉；AL 的分類值 3 經 BL 同時寫入
`[0x527F4]` 與 `[0x527F5]`。三個全域位址的高層名稱仍為**未知**；本證據只
確認 writer 與值，不把相鄰 x87 流程推論升格為欄位名稱。
