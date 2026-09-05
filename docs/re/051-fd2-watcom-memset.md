# 051 — FD2 的 Watcom `memset`

日期：2026-09-06  
證據等級：函式身分、邊界、ABI 與第三回呼 caller 為**已證實**  
輸入：`FD2.EXE`，大小 `357074`，MD5 `b97caf2239a27a896069d03549d96e1e`，
SHA-256 `222b7d067ad4450eb9c5f6e6bce1797d54bb050417ba39ced6067f8039f28c4f`  
工具：IDA Pro 9.4.0.260610，映像 `ida-pro-9.4-idapython:locked-v1`  
位址空間：IDA 載入 LE 映像後的 32 位元線性位址

IDA 的 Watcom runtime 簽章把 `0x375C0..0x375E2` 識別為 `memset`。函式從
`[esp+4]`、`[esp+8]`、`[esp+12]` 讀取目的地、填充值與長度，呼叫內部
`__STOSB`，最後把目的地重新載入 EAX 並 `retn`。`sub_4CBFD` 於 `0x4CCC6`
直接呼叫它，參數依序為 environment tail、零值與字串數。

IDA 匯出列出多個 FD2 caller；本輪只以第三回呼 caller 作正常路徑驗收，不把其他
caller 的遊戲語意一併升格。函式報告 SHA-256：
`1af65751300682919fde19b006097ee070899e565a5bc7ad181d2f00add51ef4`。
