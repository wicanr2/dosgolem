# 089 — FD2 C runtime 開檔清除 FILE 模式位元

日期：2026-09-06
證據等級：函式、原始位元組、資料位置與 consumer 為**已證實**
輸入：`FD2.EXE`，大小 `357074`，MD5 `b97caf2239a27a896069d03549d96e1e`，
SHA-256 `222b7d067ad4450eb9c5f6e6bce1797d54bb050417ba39ced6067f8039f28c4f`
工具：IDA Pro 9.4，映像 `ida-pro-9.4-idapython:locked-v1`
位址空間：IDA 載入 LE 映像後的 32 位線性位址

固定原版的 `fopen` 路徑在 `__allocfp` 後自然抵達 `sub_36EBC`
（`0x36EBC..0x36FA1`）。`0x36EC3` 由 `[ebp+0x18]` 載入 EBX；
`0x36EC9` 原始位元組為 `80 63 0C FC`，即
`and byte ptr [ebx+0x0C],0xFC`。下一條 `0x36ECD` 呼叫
`__open_flags`，其後 `0x36ED2` 將 EAX OR 回同一 `[ebx+0x0C]`，證明
此 AND 是在建立新開檔旗標前清除低兩位。

直接 caller 位於 `0x36FBE` 與 `0x37068`。函式與欄位語意採 IDA 原始
C runtime 名稱作導覽；本切片不宣稱 dosgolem 已完成 host 檔案映射。

一次性 IDA JSON 證據 SHA-256：
`f89cea59a92457981ecb3a99f773bd77af621ee8a3bc8f5353b9dccbc8351c28`。
