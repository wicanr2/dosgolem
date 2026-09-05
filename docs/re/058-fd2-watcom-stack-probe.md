# 058 — Watcom 堆疊空間探針

日期：2026-09-06
證據等級：指令、控制流、大量 caller 與失敗字串為**已證實**；
「Watcom 堆疊空間探針」分類為**強推論**
輸入：`FD2.EXE`，大小 `357074`，MD5 `b97caf2239a27a896069d03549d96e1e`，
SHA-256 `222b7d067ad4450eb9c5f6e6bce1797d54bb050417ba39ced6067f8039f28c4f`
工具：IDA Pro 9.4.0.260610，映像 `ida-pro-9.4-idapython:locked-v1`
位址空間：IDA 載入 LE 映像後的 32 位線性位址

IDA 保留原始名稱 `sub_36CD7`（`0x36CD7..0x36CE7`）、`sub_36CEA`
（`0x36CEA..0x36D07`）與 `sub_36D07`（`0x36D07..0x36D16`）。前者以
`xchg eax,[esp+4]` 取得 caller 壓入的 frame 大小，後由 `sub_36CEA`
與 ESP、`dword_52814` 及 `word_52794` 比較；失敗尾端把
`"Stack Overflow!\r\n"` 與狀態 1 送入 `__exit_with_msg_`。

`sub_36CD7` 有數百個跨越遊戲各子系統的直接 caller，且每個 caller 在
呼叫前壓入 frame 大小；因此將它分類為編譯器 runtime 的堆疊空間
探針，不列為 FD2 遊戲邏輯。重製對拍仍必須正確執行其正常路徑，
不可把呼叫直接略過。

IDA 報告 SHA-256：`sub_36CD7`
`9049d4ec1e7da73921f77bbbd808ebd9c700b4d4ad4ea8eda37d34b92136f001`；
`sub_36CEA` `b70f0f1f80367a2c39ea67149e5818ffc9fe0e255e17d6fbeba61ec1dc17fcf1`；
`sub_36D07` `4662f85b4a0f6c0a9d5bc4aa06113ef8a5ea607cabb5bfe9017f241730abf4fb`。
