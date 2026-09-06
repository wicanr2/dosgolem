# 094 — FD2 C runtime 套用開檔模式旗標

日期：2026-09-06
證據等級：函式、原始位元組、writer 與 consumer 為**已證實**
輸入：`FD2.EXE`，大小 `357074`，MD5 `b97caf2239a27a896069d03549d96e1e`，
SHA-256 `222b7d067ad4450eb9c5f6e6bce1797d54bb050417ba39ced6067f8039f28c4f`
工具：IDA Pro 9.4，映像 `ida-pro-9.4-idapython:locked-v1`
位址空間：IDA 載入 LE 映像後的 32 位線性位址

[`RE 089`](089-fd2-open-clears-file-mode-bits.md) 的同一份 IDA 函式證據顯示：
`sub_36EBC` 在 `0x36EC9` 清除 `[ebx+0x0C]` 低兩位，呼叫
`__open_flags` 後，`0x36ED2` 原始位元組 `09 43 0C` 是
`or dword ptr [ebx+0x0C],eax`。因此 EAX 的解析結果會合併回 FILE record。

直接 caller 位於 `0x36FBE` 與 `0x37068`。本切片只授權 CPU 的
`OR dword ptr [base+disp8],r32`，不宣稱該 record 可直接映射 host 結構。

沿用 IDA JSON 證據 SHA-256：
`f89cea59a92457981ecb3a99f773bd77af621ee8a3bc8f5353b9dccbc8351c28`。
