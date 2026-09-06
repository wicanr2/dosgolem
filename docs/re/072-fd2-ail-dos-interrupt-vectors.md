# 072 — FD2 AIL DOS 中斷向量保存與安裝

日期：2026-09-06
證據等級：指令、輸入、輸出與 consumer 為**已證實**；虛擬向量初值為
**平台近似**
輸入：`FD2.EXE`，大小 `357074`，MD5 `b97caf2239a27a896069d03549d96e1e`，
SHA-256 `222b7d067ad4450eb9c5f6e6bce1797d54bb050417ba39ced6067f8039f28c4f`
工具：IDA Pro 9.4.0.260610，映像 `ida-pro-9.4-idapython:locked-v1`
位址空間：IDA 載入 LE 映像後的 32 位線性位址

`sub_3E92C` 在 `0x3E9BA..0x3E9D3` 以 EAX=8、AH=`35h` 呼叫
`INT 21h`，再將回傳的 ES:EBX 保存到 `word_52BD8:dword_52BD4`。接著
`0x3E9D9..0x3E9EC` 設 EAX=8、EDX=`sub_3E73E`、BX=CS、DS=BX，並以
AH=`25h` 呼叫 `INT 21h` 安裝新的 interrupt 8 handler。

dosgolem 的既有 8086 DOS 層已以 DOS 公開契約實作 `AH=35h`（AL 指定
中斷號，回傳 ES:BX）與 `AH=25h`（AL 指定中斷號，從 DS:DX 設定）。
32 位元 LE 路徑依相同契約使用 ES:EBX 與 DS:EDX，但保存於獨立的
protected-mode DOS 向量表；不可與 DPMI `0200h` 的實模式向量表混用。

無 DOS host 時未設定向量初值為零，屬平台近似；本切片不模擬 BIOS、PIT、
實體 timer 或逐週期中斷。
