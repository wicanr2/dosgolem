# 104 — FD2 有界行讀取的長度 gate

日期：2026-09-06  
證據等級：函式邊界、參數資料流、分支條件與 loop consumer 為**已證實**；
「有界行讀取 helper」分類為**強推論**

輸入為固定原版 `FD2.EXE`（大小 `357074`；MD5
`b97caf2239a27a896069d03549d96e1e`；SHA-256
`222b7d067ad4450eb9c5f6e6bce1797d54bb050417ba39ced6067f8039f28c4f`）。
工具為 IDA Pro 9.4，映像 `ida-pro-9.4-idapython:locked-v1`；位址均為
IDA 線性位址。

原始函式 `sub_46C4C` 範圍 `0x46C4C..0x46CB6`，直接 caller 位於
`0x3F481`。`ESI=[ebp+arg_4]`，隨後：

```text
0x46C6B  4E       dec esi
0x46C6C  85 F6    test esi,esi
0x46C6E  7E 1B    jle 0x46C8B
0x46C70  57       push edi
0x46C71  ...      call fgetc
```

未跳轉路徑反覆呼叫 `fgetc`、將低 byte 寫入遞增的目的指標、遇 `0Ah`
換行離開；`0x46CA5` 寫入 NUL。故 `JLE` 是 `length-1 <= 0` 時略過讀取
迴圈的直接 gate。將整個函式分類成有界行讀取 helper 是由這組 consumer
形成的強推論，不將未證實的 C library 名稱寫成原始事實。

本證據只授權 `0x46C6E` 的 short JLE；`fgetc` 與後續檔案讀取另行閉合。
