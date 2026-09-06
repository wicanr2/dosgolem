# 090 — FD2 C runtime 讀取開檔模式首字元

日期：2026-09-06
證據等級：函式、原始位元組、資料來源與 consumer 為**已證實**
輸入：`FD2.EXE`，大小 `357074`，MD5 `b97caf2239a27a896069d03549d96e1e`，
SHA-256 `222b7d067ad4450eb9c5f6e6bce1797d54bb050417ba39ced6067f8039f28c4f`
工具：IDA Pro 9.4，映像 `ida-pro-9.4-idapython:locked-v1`
位址空間：IDA 載入 LE 映像後的 32 位線性位址

固定原版從 `sub_36EBC` 自然呼叫 `__open_flags`
（`0x36E0D..0x36EBC`）。函式先在 `0x36E12` 將第一個參數載入 ESI；
`0x36E15` 原始位元組為 `0F B6 06`，即 `movzx eax,byte ptr [esi]`。
`0x36E18` 隨即推入 EAX，`0x36E1B` 呼叫 `tolower`，其後比較 `'r'`、`'w'`
等開檔模式字元，證明此 byte 是模式字串首字元。

直接 caller 位於 `0x36E08` 與 `0x36ECD`。名稱來自 IDA 的 C runtime
導覽符號；本切片不授權 host 檔案映射。

一次性 IDA JSON 證據 SHA-256：
`272b0691cee5c532009e4cbe405f567fd4231c7a40d8c67eaf63470115179ed4`。
