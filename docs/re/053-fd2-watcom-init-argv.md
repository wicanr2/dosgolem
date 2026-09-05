# 053 — FD2 的 Watcom `__Init_Argv`

日期：2026-09-06  
證據等級：函式身分、邊界、輸入與輸出全域為**已證實**  
輸入：`FD2.EXE`，大小 `357074`，MD5 `b97caf2239a27a896069d03549d96e1e`，
SHA-256 `222b7d067ad4450eb9c5f6e6bce1797d54bb050417ba39ced6067f8039f28c4f`  
工具：IDA Pro 9.4.0.260610，映像 `ida-pro-9.4-idapython:locked-v1`  
位址空間：IDA 載入 LE 映像後的 32 位元線性位址

IDA 的 Watcom runtime 簽章把 `0x46114..0x461C7` 識別為 `__Init_Argv`。
它讀取 command line 指標 `dword_52808` 與 program name 指標 `dword_5280C`，
配置字串與指標表，最後寫入：

- `dword_527F8`：計算所得 argc；
- `dword_527FC`：配置區內的 argv 表；
- `0x5462C`：公開 argc；
- `0x54628`：公開 argv。

固定啟動實驗在函式入口得到 `dword_52808=0x546B0`、其首 byte 為零，且
`dword_5280C=0x546B1`，內容為 `FD2.EXE\0`。因此本次正常路徑的結果是 argc=1、
argv[0]=`0x546B1`、argv[1]=NULL。非空 command line 的完整 Watcom 引號／跳脫
規則尚未驗證，不在本輪冒充已支援。

IDA 函式報告 SHA-256：
`a1532b8b0d3d9d450d8f0ef26bd09f52a7807fb335d1e10d8ea269facbbe0a6d`。
