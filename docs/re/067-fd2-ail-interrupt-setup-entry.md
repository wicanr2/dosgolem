# 067 — FD2 AIL 中斷設定入口

日期：2026-09-06
證據等級：函式邊界、caller、旗標保存／恢復與中斷呼叫為**已證實**；
硬體與 DOS 服務細節依平台規格，不作遊戲語意深挖
輸入：`FD2.EXE`，大小 `357074`，MD5 `b97caf2239a27a896069d03549d96e1e`，
SHA-256 `222b7d067ad4450eb9c5f6e6bce1797d54bb050417ba39ced6067f8039f28c4f`
工具：IDA Pro 9.4.0.260610，映像 `ida-pro-9.4-idapython:locked-v1`
位址空間：IDA 載入 LE 映像後的 32 位線性位址

AIL 初始化 `sub_3F959` 在 `0x3FA5A` 尾跳至 `sub_3E92C`
（`0x3E92C..0x3EA1A`）。後者保存 EBX／ESI／EDI／ES，並在 `0x3E930`
執行 `pushf`、`0x3E931` 執行 `cli`。函式尾端 `0x3EA14` 以 `popf`
恢復旗標，再恢復 ES 與通用暫存器後返回。

中段包含 DPMI `INT 31h`、DOS `INT 21h/AH=35h` 讀取中斷向量與
`AH=25h` 設定中斷向量。這些是 AIL／DOS 平台初始化，不是 FD2 遊戲規則；
dosgolem 只依實際 caller 與公開平台契約逐項實作，不追逐 BIOS 或硬體週期。

一次性 IDA 報告 SHA-256：
`250464280ab0d0f81ca78e31de612109e4ed681609614fd60812efef69f9cd5a`；
一次性 `.i64` SHA-256：
`02d869384178814af1acb5795a40431bed100719200e5b066e0b3bae943eeda6`。
