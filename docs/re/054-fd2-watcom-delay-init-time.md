# 054 — FD2 的 Watcom `__delay_init` 時間來源

日期：2026-09-06  
證據等級：函式身分、DOS service 與消費端為**已證實**；dosgolem 時間步進為
**硬體規格近似**  
輸入：`FD2.EXE`，大小 `357074`，MD5 `b97caf2239a27a896069d03549d96e1e`，
SHA-256 `222b7d067ad4450eb9c5f6e6bce1797d54bb050417ba39ced6067f8039f28c4f`  
工具：IDA Pro 9.4.0.260610，映像 `ida-pro-9.4-idapython:locked-v1`  
位址空間：IDA 載入 LE 映像後的 32 位元線性位址

IDA 將 `0x3DC9F..0x3DCCD` 識別為 Watcom `__delay_init`。它三度或更多次呼叫
`INT 21h/AH=2Ch`：先等待 DH 的秒值改變，再計算下一次秒值改變前的迴圈次數，
最後寫入 `dword_541B0`。這是 DOS wall-clock 校準，不是 FD2 遊戲規則。

依專案硬體時序停止線，dosgolem 不追求主機速度或 DOS busy-loop 的逐週期一致；
以每次查詢遞增一秒的決定性序列避免無界等待，標為硬體規格近似，不宣稱原版
wall-clock parity。函式報告 SHA-256：
`6cfb948f96366b6aa2d0a3b7680942208f3719a8f3bb3cba3f7c8368ae27e48a`。
