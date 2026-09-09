# 082 — FD2 AIL 設定 PIT channel 0

日期：2026-09-06
證據等級：函式、port、寫入順序與資料來源為**已證實**；硬體行為為
**硬體規格近似**
輸入：`FD2.EXE`，大小 `357074`，MD5 `b97caf2239a27a896069d03549d96e1e`，
SHA-256 `222b7d067ad4450eb9c5f6e6bce1797d54bb050417ba39ced6067f8039f28c4f`
工具：IDA Pro 9.4.0.260610，映像 `ida-pro-9.4-idapython:locked-v1`
位址空間：IDA 載入 LE 映像後的 32 位線性位址

`sub_3E864`（`0x3E864..0x3E894`）保存旗標並 CLI，於 `0x3E86C`
設定 AL=`36h`，在 `0x3E86E` 寫入 port `43h`。接著載入第一個參數、保存
到 `dword_52BDE`，並依序在 `0x3E87A`、`0x3E880` 把 divisor 的低 byte、
高 byte 寫入 port `40h`，最後恢復旗標。

依 8253/8254 公開契約，port `43h` 的 `36h` 選擇 channel 0、低 byte／高
byte 存取與 mode 3，port `40h` 接收 divisor。參考：
<https://wiki.osdev.org/Programmable_Interval_Timer>。

dosgolem 只需決定性記錄 port/value 序列，供原版對拍與後續 PIT 行為模型
消費；不追逐逐週期 wall-clock。這是硬體規格近似，不宣稱原硬體時序 parity。

固定原版目前路徑由 `sub_3E894` 傳入零，因此實際序列為
`43h:36h、40h:00h、40h:00h`，`dword_52BDE=0`。依 8253/8254 契約，
載入 counter 的零值代表 65536；此解釋屬硬體規格近似。

一次性 IDA 報告 SHA-256：
`89d1be8d9e6db3c5cdeb6dcf769ea9a2a322e92e16df7199ed0352d7c9f1bafa`。
