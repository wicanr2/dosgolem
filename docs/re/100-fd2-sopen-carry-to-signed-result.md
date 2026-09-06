# 100 — FD2 `sopen` 將 DOS CF 轉為有號結果

日期：2026-09-06  
證據等級：指令、控制流與 consumer 為**已證實**

輸入為固定原版 `FD2.EXE`（大小 `357074`；MD5
`b97caf2239a27a896069d03549d96e1e`；SHA-256
`222b7d067ad4450eb9c5f6e6bce1797d54bb050417ba39ced6067f8039f28c4f`）。
工具為 IDA Pro 9.4，映像 `ida-pro-9.4-idapython:locked-v1`；下列位址均為
IDA 線性位址。原始函式名為 `sopen`，範圍 `0x3CD43..0x3CF03`，直接
caller 位於 `0x36F12`、`0x36F52`、`0x3CD39`。

`AH=3Dh` 的 `int 21h` 回傳後，原始指令如下：

```text
0x3CD73  CD 21       int 21h
0x3CD75  D1 D0       rcl eax,1
0x3CD77  D1 C8       ror eax,1
0x3CD79  89 C7       mov edi,eax
0x3CD7B  85 C0       test eax,eax
0x3CD7D  7C 06       jl 0x3CD85
0x3CD7F  0F B7 C0    movzx eax,ax
0x3CD82  89 45 F8    mov [ebp-8],eax
```

對 32-bit operand、count 1 而言，這一對旋轉保留原 EAX 的低 31 bits，
並將 `int 21h` 的輸入 CF 放入結果 bit 31。後續 `test`／`jl` 因此將
CF=1 的 DOS 錯誤分流至失敗路徑；CF=0 時則零擴展 AX，將 DOS handle
保存到 `[ebp-8]`。這也證實旋轉指令是開檔結果的直接 consumer，不是與
玩家行為無關的任意 runtime 指令。

本證據只授權 opcode `D1` 在這條固定路徑實際使用的 register-direct
`RCL r32,1` 與 `ROR r32,1`。其他群組、記憶體 operand、16-bit operand
及可變 count 仍為未實作範圍。
