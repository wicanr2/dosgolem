# 046 — FD2 第三次回呼建立 FS 掃描狀態

日期：2026-09-06  
證據等級：**已證實**（固定雜湊原始位元組與已證實 LFS 前置狀態）  
輸入：`FD2.EXE`，大小 `357074`，MD5 `b97caf2239a27a896069d03549d96e1e`，
SHA-256 `222b7d067ad4450eb9c5f6e6bce1797d54bb050417ba39ced6067f8039f28c4f`  
工具：dosgolem `feat/fd2-parity`，基線提交 `8d89730`  
位址空間：dosgolem 載入 LE 映像後的 32 位元線性位址

`LFS` 得到 EAX=0、FS=`0x30` 後，原始指令為：

```text
0x4CC1D  89 45 FC  mov [ebp-4],eax
0x4CC20  8C E0     mov eax,fs
0x4CC22  31 F6     xor esi,esi
0x4CC24  8E C0     mov es,eax
```

第一條保存遠指標 offset，第二條取出 FS selector，第三條建立零 offset。結合
下一條 `mov es,eax`，可證實這是 segment selector 與 offset 的準備資料流；是否
以及如何掃描 environment 內容，須由後續 consumer 另行閉合。
