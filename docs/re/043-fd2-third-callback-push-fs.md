# 043 — FD2 第三次回呼保存 FS

日期：2026-09-06  
證據等級：**已證實**（固定雜湊原始位元組與自然執行入口）  
輸入：`FD2.EXE`，大小 `357074`，MD5 `b97caf2239a27a896069d03549d96e1e`，
SHA-256 `222b7d067ad4450eb9c5f6e6bce1797d54bb050417ba39ced6067f8039f28c4f`  
工具：dosgolem `feat/fd2-parity`，基線提交 `50fe653`  
位址空間：dosgolem 載入 LE 映像後的 32 位元線性位址  
平台前提：Intel 80386 `PUSH FS`／`POP FS` 公開指令契約

第三次選中的回呼開頭為：

```text
0x4CBFD  53     push ebx
0x4CBFE  56     push esi
0x4CBFF  57     push edi
0x4CC00  06     push es
0x4CC01  0F A0  push fs
0x4CC03  55     push ebp
```

固定路徑已自然抵達 `0x4CBFD`；既有四個 push 後，首個缺口是 `PUSH FS`。
這是標準 CPU 堆疊行為，不賦予回呼任何遊戲高層名稱。對稱的 `POP FS` 只作為
標準配對形式實作，最終是否由此回呼消費仍須以後續原始指令確認。
