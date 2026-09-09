# 050 — FD2 第三回呼的兩段配置消費端

日期：2026-09-06  
證據等級：函式邊界、指令、分支與 `_nmalloc` 呼叫關係為**已證實**  
輸入：`FD2.EXE`，大小 `357074`，MD5 `b97caf2239a27a896069d03549d96e1e`，
SHA-256 `222b7d067ad4450eb9c5f6e6bce1797d54bb050417ba39ced6067f8039f28c4f`  
工具：IDA Pro 9.4.0.260610，映像 `ida-pro-9.4-idapython:locked-v1`  
位址空間：IDA 載入 LE 映像後的 32 位元線性位址

IDA 函式 `sub_4CBFD` 的邊界是 `0x4CBFD..0x4CCE1`。第一次 `_nmalloc`
於 `0x4CC4C` 呼叫，返回後依序執行 `mov edi,eax`、清除參數、保存 EAX，並在
`0x4CC58` 以原始 bytes `85 C0`（`test eax,eax`）檢查失敗。成功路徑計算第二段
大小，再於 `0x4CC6B` 呼叫同一 `_nmalloc`；`0x4CC73` 也用 `test eax,eax`
檢查結果。這證明 `TEST` 是配置回傳值的實際消費端，不是任意啟動樣板。

受控函式報告 SHA-256：
`8824bccb81f4204b0f1873f6050a0429275bcf1ccc86b2499b5a0afafaf54e3b`。
