# 056 — FD2 的 Watcom `__CMain`

日期：2026-09-06
證據等級：函式身分、邊界、stack 配置與 `main` 呼叫為**已證實**
輸入：`FD2.EXE`，大小 `357074`，MD5 `b97caf2239a27a896069d03549d96e1e`，
SHA-256 `222b7d067ad4450eb9c5f6e6bce1797d54bb050417ba39ced6067f8039f28c4f`
工具：IDA Pro 9.4.0.260610，映像 `ida-pro-9.4-idapython:locked-v1`
位址空間：IDA 載入 LE 映像後的 32 位元線性位址

IDA 的 Watcom runtime 簽章把 `0x45D4B..0x45D9A` 識別為 `__CMain`，直接 caller
是 LE entry 內的 `0x3CB8C`。它使用 `dword_5281C` 與 `sub_463BC` 的 stack 差值
決定是否於 `0x45D66` 執行 `sub esp,eax`，設定 `dword_52820`，呼叫
`sub_4977D`，最後把 `argv`、`argc` 壓棧並於 `0x45D8C` 呼叫 `main`。

因此 `0x45D66` 是到達 FD2 main 前的真實 stack 配置，不可略過。IDA 函式報告
SHA-256：`8953f09cd741654dae11ee9ab67e1de55f65a196ba54415cdfff1a7f39c2080c`。
