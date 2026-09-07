# 064 — FD2 AIL preference 表格存取

日期：2026-09-06
證據等級：函式邊界、caller、原始指令、writer 與 consumer 為**已證實**；
表格的完整索引語意仍為**未知**
輸入：`FD2.EXE`，大小 `357074`，MD5 `b97caf2239a27a896069d03549d96e1e`，
SHA-256 `222b7d067ad4450eb9c5f6e6bce1797d54bb050417ba39ced6067f8039f28c4f`
工具：IDA Pro 9.4.0.260610，映像 `ida-pro-9.4-idapython:locked-v1`
位址空間：IDA 載入 LE 映像後的 32 位線性位址

IDA 保留 `sub_3F5E7`（`0x3F5E7..0x3F600`）。`0x3F5E7` 讀取第一參數
作索引；`0x3F5EB` 的原始位元組／指令為
`8B 14 85 0C 43 05 00`／`mov edx,[eax*4+5430Ch]`。`0x3F5F6` 把第二參數
寫回同一表格項目，`0x3F5FD` 再令 EAX=EDX，故函式回傳被替換的舊值。

AIL 函式 `sub_37C20`（`0x37C20..0x37D1A`）在 `0x37C83..0x37C92`
依序壓入新值與索引、呼叫此函式，並把回傳舊值保存於 ESI；上層
`0x3F959..0x3FA5F` 有 18 個直接呼叫。這支持它是 AIL preference
設定邊界，但目前不替個別索引猜名稱，也不把它視為 FD2 遊戲規則。

一次性 IDA 報告 SHA-256：
`7e29ffa92736e1fbc12bc4a790ccc890747045249c324593835e316b0dccd5f8`；
一次性 `.i64` SHA-256：
`2c04d78c7d4f8b6bd564dbd4baf143d76ee33474205345f0c5573e0c4f820b43`。
