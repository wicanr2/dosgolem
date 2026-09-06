# 102 — FD2 `isatty` 查詢 DOS 裝置資訊

日期：2026-09-06  
證據等級：函式、handle 資料流、DOS 呼叫與回傳 consumer 為**已證實**

輸入為固定原版 `FD2.EXE`（大小 `357074`；MD5
`b97caf2239a27a896069d03549d96e1e`；SHA-256
`222b7d067ad4450eb9c5f6e6bce1797d54bb050417ba39ced6067f8039f28c4f`）。
工具為 IDA Pro 9.4，映像 `ida-pro-9.4-idapython:locked-v1`；位址均為
IDA 線性位址。

原始函式名 `isatty`，範圍 `0x3FB0F..0x3FB2F`，直接 caller 位於
`0x37A5A`、`0x3CD98`、`0x3CE98`、`0x3D2FC`、`0x46382`。指令如下：

```text
0x3FB13  8B 45 0C       mov eax,[ebp+arg_0]
0x3FB16  66 89 C3       mov bx,ax
0x3FB19  B0 00          mov al,0
0x3FB1B  B4 44          mov ah,44h
0x3FB1D  CD 21          int 21h
0x3FB1F  D1 D2          rcl edx,1
0x3FB21  D1 CA          ror edx,1
0x3FB23  F6 C2 80       test dl,80h
0x3FB26  0F 95 C0       setnz al
0x3FB29  0F B6 C0       movzx eax,al
```

`0x3FB16` 將函式參數的低 16 bits 搬到 BX，直接供 `AH=44h, AL=0` 的
DOS IOCTL get-device-information 使用。呼叫後相同的 RCL／ROR 慣用序列把
CF 放進 EDX bit 31，而 `test dl,80h`、`setnz`、`movzx` 將 DX bit 7
正規化成布林回傳值。依 DOS 公開介面，bit 7 表示裝置而非磁碟檔案；本 RE
只證實 FD2 runtime 選用的 handle、子功能與 consumer，不重新考古 DOS ABI。

本切片先授權 `0x3FB16` 使用的 16-bit register-direct MOV；DOS IOCTL
服務另立規格，未實作時維持失敗即關閉。
