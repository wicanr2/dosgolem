# 061 — FD2 DPMI 定長區域鎖定包裝器

日期：2026-09-06
證據等級：函式邊界、caller、參數轉換與 consumer 為**已證實**
輸入：`FD2.EXE`，大小 `357074`，MD5 `b97caf2239a27a896069d03549d96e1e`，
SHA-256 `222b7d067ad4450eb9c5f6e6bce1797d54bb050417ba39ced6067f8039f28c4f`
工具：IDA Pro 9.4.0.260610，映像 `ida-pro-9.4-idapython:locked-v1`
位址空間：IDA 載入 LE 映像後的 32 位線性位址

IDA 保留原始名稱 `sub_36316`，邊界 `0x36316..0x3632D`。它讀取
`[esp+4]` 的開始位址與 `[esp+8]` 的大小，以 `add eax,[esp+8]` 得到終點，
再依序將開始、終點送入 `sub_36284`。`sub_36284` 以兩端點差加 1
建立 DPMI `0600h` 長度。

AIL 啟動 `sub_3783C` 連續八次以各全域 dword 位址與大小 4 呼叫
`sub_36316`（`0x37863..0x378CC`），是第一個程式區鎖定後的資料區鎖定鏈。
這是 DPMI／AIL runtime 行為，不是遊戲規則。

IDA 報告 SHA-256：
`2cc393cb055a6895a8f6e457ee1cc2541b500a80feea77d6e00c89c37dc62503`。
