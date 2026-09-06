# 069 — FD2 AIL 次級 DPMI 鎖定區段

日期：2026-09-06
證據等級：函式邊界、caller、參數與 consumer 為**已證實**
輸入：`FD2.EXE`，大小 `357074`，MD5 `b97caf2239a27a896069d03549d96e1e`，
SHA-256 `222b7d067ad4450eb9c5f6e6bce1797d54bb050417ba39ced6067f8039f28c4f`
工具：IDA Pro 9.4.0.260610，映像 `ida-pro-9.4-idapython:locked-v1`
位址空間：IDA 載入 LE 映像後的 32 位線性位址

AIL 中斷設定 `sub_3E92C` 在 `0x3E943` 呼叫 `sub_3F0F6`
（`0x3F0F6..0x3F11B`）。後者先將 `dword_52A54` 至 `dword_53604`
的兩端點送入已證實的 DPMI 鎖定包裝器 `sub_36284`，再將
`sub_3E724` 至 `sub_3F0F6` 的兩端點送入同一包裝器。

兩次呼叫後都由 caller 清理八 byte 參數並返回，沒有新回傳值 consumer。
因此這是既有 `INT 31h/AX=0600h` 線性區域鎖定契約的另一組 caller，
不是新的 DPMI 功能，也不是 FD2 遊戲規則；dosgolem 應重用現有
[`spec 094`](../spec/094-watcom-int386-dpmi-lock.md)，不得另加跳過函式的 hook。

一次性 IDA 報告 SHA-256：
`760da20384d3fa53fa054f03fd208413804809610204cf72f6b53c939096485f`；
一次性 `.i64` SHA-256：
`d4981d64e645c7d3d2b499f892ba95ef1156f72a79a067ad9270223dfd140445`。
