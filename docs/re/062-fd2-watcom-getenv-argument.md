# 062 — FD2 Watcom `getenv` 參數壓棧

日期：2026-09-06
證據等級：函式邊界、caller、原始指令與 consumer 為**已證實**
輸入：`FD2.EXE`，大小 `357074`，MD5 `b97caf2239a27a896069d03549d96e1e`，
SHA-256 `222b7d067ad4450eb9c5f6e6bce1797d54bb050417ba39ced6067f8039f28c4f`
工具：IDA Pro 9.4.0.260610，映像 `ida-pro-9.4-idapython:locked-v1`
位址空間：IDA 載入 LE 映像後的 32 位線性位址

IDA 保留原始名稱 `getenv`，函式邊界 `0x3F13B..0x3F190`。`0x3F151`
的原始位元組與指令是 `FF 75 14`／`push dword ptr [ebp+14h]`。函式先保存
四個暫存器再令 EBP 指向保存區，因此 `[ebp+14h]` 是 `getenv` 的第一個參數；
下一個 consumer 是 `0x3F154` 對原始 `strlen`
的直接呼叫。AIL 啟動函式 `0x379EE..0x37B88` 在 `0x37A0C` 與
`0x37A29` 呼叫 `getenv`。

此證據只證實 DOS/Watcom C 執行期需要 `FF /6` 的 `mod=1` 基址加有符號
8 位移記憶體形狀；它不是 FD2 遊戲規則，也不證明環境變數的玩家可見語意。

IDA 報告 SHA-256：
`18dc2da2bb14a4cba9ff129e83d0d0fcc3c5c9ac468cbbda5a0a42c6532a5519`。
