# 044 — FD2 第三次回呼堆疊區與全域閘門

日期：2026-09-06  
證據等級：**已證實**（固定雜湊原始位元組與自然執行前置狀態）  
輸入：`FD2.EXE`，大小 `357074`，MD5 `b97caf2239a27a896069d03549d96e1e`，
SHA-256 `222b7d067ad4450eb9c5f6e6bce1797d54bb050417ba39ced6067f8039f28c4f`  
工具：dosgolem `feat/fd2-parity`，基線提交 `785088b`  
位址空間：dosgolem 載入 LE 映像後的 32 位元線性位址

第三回呼保存 FS 後執行：

```text
0x4CC03  55                    push ebp
0x4CC04  89 E5                 mov ebp,esp
0x4CC06  83 EC 04              sub esp,4
0x4CC09  83 3D FC370500 00     cmp dword ptr [0x537FC],0
0x4CC10  0F 85 C3000000        jne 0x4CCD9
```

本切片確認四位元組區域的配置方式，以及 `0x537FC` 是此分支的直接讀取端；
該全域的 writer 與高層用途尚未證實，因此名稱維持**未知**。
