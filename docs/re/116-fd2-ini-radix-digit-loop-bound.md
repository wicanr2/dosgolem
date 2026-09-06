# RE 116 — INI 數值解析器的 radix digit 迴圈上界

日期：2026-09-06  
證據等級：**已證實**（函式、指令、資料流與分支 consumer）

## 輸入與工具

- `FD2.EXE`：大小 `357074`；MD5
  `b97caf2239a27a896069d03549d96e1e`；SHA-256
  `222b7d067ad4450eb9c5f6e6bce1797d54bb050417ba39ced6067f8039f28c4f`。
- IDA Pro 9.4.0.260610，映像 `ida-pro-9.4-idapython:locked-v1`。
- 本文位址均為 IDA 線性位址；原始函式名與邊界為
  `sub_3F264`、`0x3F264..0x3F306`。

## 原始資料流

- `0x3F2A1 31 DB` 將 EBX 清零，作為 digit-table index。
- `0x3F2A5 0F B6 B3 B4 11 05 00` 以 EBX 讀取 `byte_511B4` 的候選 digit；
  `0x3F2AC..0x3F2BF` 將目前輸入字元轉成大寫後與候選比較。
- 不相等時 `0x3F2D0 43` 遞增 EBX。
- `0x3F2D1 3B 5C 24 1C` 為 `cmp ebx,[esp+1Ch]`；該 stack 位置是 caller
  傳入的 radix／允許 digit 數上界。
- `0x3F2D5 7C CE` 以 signed less-than 跳回 `0x3F2A5`，是比較旗標的直接
  consumer；`0x3F2D7` 再做相同比較以判斷是否命中合法 digit。
- 固定雜湊 FD2 自然執行已抵達 `0x3F2D1`。

因此此處需要 `cmp r32,[SS:ESP+disp8]` 的 SIB `24` stack 形狀；比較不得寫回
EBX 或 stack。這不授權其他 `3B` SIB、segment override 或 displacement 形狀。
