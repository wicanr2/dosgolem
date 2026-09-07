# 057 — FD2 `main` 入口

日期：2026-09-06
證據等級：函式身分、邊界、caller 與入口指令為**已證實**
輸入：`FD2.EXE`，大小 `357074`，MD5 `b97caf2239a27a896069d03549d96e1e`，
SHA-256 `222b7d067ad4450eb9c5f6e6bce1797d54bb050417ba39ced6067f8039f28c4f`
工具：IDA Pro 9.4.0.260610，映像 `ida-pro-9.4-idapython:locked-v1`
位址空間：IDA 載入 LE 映像後的 32 位線性位址

IDA 保留原始名稱 `main`，邊界 `0x25BF4..0x25EBB`；直接 caller 是
Watcom `__CMain` 的 `0x45D8C`。入口從 `0x25BF4` 的 `push 1Ch`
（raw bytes `68 1C 00 00 00`）開始，並於 `0x25BF9` 呼叫保留原始名稱
`sub_36CD7`。

dosgolem 固定雜湊實驗已由 LE entry 自然執行到 `0x25BF4`；在尚未
支援 opcode `68` 時，第 1094 步以該原始指令失敗即關閉。本證據只授權
32 位 immediate PUSH，不為 `sub_36CD7` 命名或猜測其遊戲語意。

IDA 函式報告 SHA-256：
`53aa004404498e7c192df21b8ea91f3753dd9986e27f60c6e192de2a97673dfb`。
