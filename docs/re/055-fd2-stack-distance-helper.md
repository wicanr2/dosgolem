# 055 — FD2 啟動路徑的 stack distance helper

日期：2026-09-06
證據等級：函式邊界、指令與 caller 為**已證實**；「stack distance」用途為
**強推論**
輸入：`FD2.EXE`，大小 `357074`，MD5 `b97caf2239a27a896069d03549d96e1e`，
SHA-256 `222b7d067ad4450eb9c5f6e6bce1797d54bb050417ba39ced6067f8039f28c4f`
工具：IDA Pro 9.4.0.260610，映像 `ida-pro-9.4-idapython:locked-v1`
位址空間：IDA 載入 LE 映像後的 32 位元線性位址

IDA 保留原始名稱 `sub_463BC`，邊界 `0x463BC..0x463C5`。函式只執行
`mov eax,esp`、`sub eax,dword_52814`、`retn`；直接 caller 位於 `0x3D14D`
與啟動分派 `0x45D5B`。因此回傳值是目前 ESP 與全域基準的差，至於 caller 如何
命名或使用此距離仍不升格為已證實語意。

IDA 函式報告 SHA-256：
`cd84fbf2febf5a673a23d1f0892146cb53d77994981f23ef571cfeca623b42e3`。
