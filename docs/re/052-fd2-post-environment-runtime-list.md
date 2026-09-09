# 052 — FD2 environment 後的 Watcom runtime 清單初始化

日期：2026-09-06  
證據等級：函式邊界、原始指令與 `_nmalloc` caller 為**已證實**；清單用途為
**強推論**  
輸入：`FD2.EXE`，大小 `357074`，MD5 `b97caf2239a27a896069d03549d96e1e`，
SHA-256 `222b7d067ad4450eb9c5f6e6bce1797d54bb050417ba39ced6067f8039f28c4f`  
工具：IDA Pro 9.4.0.260610，映像 `ida-pro-9.4-idapython:locked-v1`  
位址空間：IDA 載入 LE 映像後的 32 位元線性位址

第三回呼完成後，啟動分派抵達 IDA 原始名稱 `sub_468F8`，邊界
`0x468F8..0x4693D`。它在 `0x468F9` 對 `byte_52881` 執行 AND `0xF8`，並在
`0x46905` OR `4`。之後由 `unk_52840` 起檢查 stride `0x1A` 的紀錄；每個符合
條件者於 `0x46910` 呼叫已證實 `_nmalloc(8)`，再串接兩個 dword。

這證明兩個 byte 運算是後續配置路徑的直接閘門；清單的產品語意尚未證實，
不建立猜測名稱。IDA 函式報告 SHA-256：
`a4321b7ea99ff48fe5859f19c2f0981412c360fe8713514019fcbfd1b8899b26`。
