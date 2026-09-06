# 070 — FD2 AIL 取得實模式中斷向量

日期：2026-09-06
證據等級：呼叫點、輸入暫存器與 consumer 為**已證實**；無硬體 host 的
預設零向量為**平台近似**
輸入：`FD2.EXE`，大小 `357074`，MD5 `b97caf2239a27a896069d03549d96e1e`，
SHA-256 `222b7d067ad4450eb9c5f6e6bce1797d54bb050417ba39ced6067f8039f28c4f`
工具：IDA Pro 9.4.0.260610，映像 `ida-pro-9.4-idapython:locked-v1`
位址空間：IDA 載入 LE 映像後的 32 位線性位址

AIL 中斷設定 `sub_3E92C` 在 `0x3E9A5..0x3E9B2` 設定 EAX=8、EBX=8、
AX=`0200h`，再直接執行 `INT 31h`。`0x3E9B4..0x3E9B7` 將回傳的
CX:DX 組合成一個 dword；稍後 `0x3E9D3` 寫入 `dword_52BDA`。

DPMI 0.9 `0200h` 契約指定 BL 為中斷號，永遠成功並清除 CF，回傳 CX:DX
為實模式 handler 的 segment:offset。來源：
<https://www.delorie.com/djgpp/doc/dpmi/api/310200.html>。

dosgolem 沒有主機 BIOS 向量可轉交，因此以 256 項決定性虛擬向量表建模；
未設定項目回傳 `0000:0000`。這是平台近似，不宣稱原硬體向量內容一致，
也不需要反組譯 BIOS／PIT。
