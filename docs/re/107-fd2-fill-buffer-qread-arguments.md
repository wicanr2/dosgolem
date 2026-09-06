# 107 — FD2 `__fill_buffer` 呼叫 `__qread` 的參數

日期：2026-09-06  
證據等級：函式邊界、呼叫關係、三個參數來源與原始指令為**已證實**；
FILE 欄位的用途名稱為**強推論**

輸入為固定原版 `FD2.EXE`（大小 `357074`；MD5
`b97caf2239a27a896069d03549d96e1e`；SHA-256
`222b7d067ad4450eb9c5f6e6bce1797d54bb050417ba39ced6067f8039f28c4f`）。
工具為 IDA Pro 9.4，映像 `ida-pro-9.4-idapython:locked-v1`；位址均為
IDA 線性位址。

IDA 原始函式 `__fill_buffer` 範圍為 `0x3DA65..0x3DB10`。固定原版由
`fgetc` 進入此函式，配置緩衝後在 `0x3DAE1` 準備 `__qread` 呼叫：

```text
0x3DADD  8B 43 14       mov  eax,[ebx+14h]
0x3DAE0  50             push eax
0x3DAE1  FF 33          push dword ptr [ebx]
0x3DAE3  FF 73 10       push dword ptr [ebx+10h]
0x3DAE6  E8 A5 FE FF FF call __qread
0x3DAEB  83 C4 0C       add  esp,0Ch
0x3DAEE  89 43 04       mov  [ebx+4],eax
```

依右至左參數順序，`[ebx+10h]`、`[ebx]`、`[ebx+14h]` 分別送入
`__qread`，回傳值寫到 `[ebx+4]`。結合前一切片已證實的 buffer consumer，
這三欄可導覽性地描述為 DOS handle、目前 buffer pointer、要求讀取長度；欄位
名稱仍只列強推論，不取代原始 offset。

本證據只授權 `FF /6` 的非 SIB、無位移基址間接 dword 壓棧形狀。真正
`__qread` 內的 DOS read 服務必須另立 writer／consumer 證據與規格。
