# 101 — FD2 `__IOMode` 測試 FILE 狀態旗標

日期：2026-09-06  
證據等級：函式、指令、表格 stride 與分支 consumer 為**已證實**；旗標的人類語意為
**強推論**

輸入為固定原版 `FD2.EXE`（大小 `357074`；MD5
`b97caf2239a27a896069d03549d96e1e`；SHA-256
`222b7d067ad4450eb9c5f6e6bce1797d54bb050417ba39ced6067f8039f28c4f`）。
工具為 IDA Pro 9.4，映像 `ida-pro-9.4-idapython:locked-v1`；位址均為
IDA 線性位址。

原始函式名 `__IOMode`，範圍 `0x46352..0x463A4`；直接 caller 位於
`0x3CC5D`、`0x3CE89`、`0x3CF0F`、`0x3D0A1`、`0x46A0D`。固定原版在
成功保存 `MDI.INI` handle 後自然抵達：

```text
0x4636B  89 F3             mov ebx,esi
0x4636D  A1 E4 37 05 00    mov eax,[0x537E4]
0x46372  C1 E3 02          shl ebx,2
0x46375  F6 44 03 01 40    test byte ptr [ebx+eax+1],40h
0x4637A  75 1C             jnz 0x46398
0x4637C  56                push esi
0x4637D  80 4C 03 01 40    or byte ptr [ebx+eax+1],40h
0x46382  E8 88 97 FF FF    call isatty
```

`esi` 是 handle index，乘以 4 後索引 `0x537E4` 所指表格；`+1` byte 的
bit `0x40` 控制是否略過後續 `isatty`。因此「這是一個每筆四 bytes 的 FILE
狀態欄位與一次性分類 gate」為已證實；依原始函式名與 `isatty` consumer 將
它描述為 I/O 模式已初始化旗標則只列強推論，不將工具名稱升格成 ABI 事實。

本切片只授權 `F6 /0` 在此處使用的 SIB `base+index+disp8` byte TEST；後續
`80 /1` OR 與 `isatty` 尚未由本切片實作。
