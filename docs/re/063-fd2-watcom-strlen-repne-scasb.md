# 063 — FD2 Watcom `strlen` 的 REPNE SCASB

日期：2026-09-06
證據等級：函式邊界、caller、指令序列與 consumer 為**已證實**
輸入：`FD2.EXE`，大小 `357074`，MD5 `b97caf2239a27a896069d03549d96e1e`，
SHA-256 `222b7d067ad4450eb9c5f6e6bce1797d54bb050417ba39ced6067f8039f28c4f`
工具：IDA Pro 9.4.0.260610，映像 `ida-pro-9.4-idapython:locked-v1`
位址空間：IDA 載入 LE 映像後的 32 位線性位址

IDA 保留原始名稱 `strlen`，函式邊界 `0x37805..0x37821`。函式在
`0x37809` 將字串參數送入 EDI，令 ES=DS、ECX=`0xFFFFFFFF`、AL=0，
再於 `0x37816` 執行 `repne scasb`。`0x37818` 的 `not ecx` 與
`0x3781A` 的 `dec ecx` 消費剩餘計數，形成不含 NUL 的長度。

目前路徑由 Watcom `getenv` 在 `0x3F154` 直接呼叫此函式；其他直接 caller
仍保留於 IDA 報告。本項是標準 Watcom C 執行期字串掃描，不是 FD2 遊戲規則。

IDA 報告 SHA-256：
`f25b5243e1268891deb1a726918d1b43a39aac724b0a92916f0b98b6ca17043d`。
