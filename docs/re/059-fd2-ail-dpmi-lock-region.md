# 059 — AIL 啟動的 DPMI 線性記憶體鎖定

日期：2026-09-06
證據等級：函式邊界、caller、指令與參數佈局為**已證實**；
`INT 31h/AX=0600h` 語意來自 DPMI 0.9 公開契約
輸入：`FD2.EXE`，大小 `357074`，MD5 `b97caf2239a27a896069d03549d96e1e`，
SHA-256 `222b7d067ad4450eb9c5f6e6bce1797d54bb050417ba39ced6067f8039f28c4f`
工具：IDA Pro 9.4.0.260610，映像 `ida-pro-9.4-idapython:locked-v1`
位址空間：IDA 載入 LE 映像後的 32 位線性位址

`main` 在 `0x25C01` 呼叫保留原始名稱 `sub_379EE`
（`0x379EE..0x37B88`）。該函式含 `AIL_DEBUG`、`AIL_SYS_DEBUG`、
`"Audio Interface Library application usage"`、版本 `3.02` 與
`"AIL_startup()"`，因此它屬於 Miles Audio Interface Library 啟動路徑。

`sub_379EE` 於 `0x379F4` 呼叫 `sub_3783C`；後者以
`sub_3783C` 與 `sub_3C6D3` 的線性範圍呼叫 `sub_36284`
（`0x36284..0x362F1`）。`sub_36284` 把 `0x600` 寫入輸入暫存器
結構，將開始位址拆為 BX:CX、長度拆為 SI:DI，再以中斷號 `0x31`
呼叫 Watcom `int386`。呼叫後檢查輸出結構的 carry 結果，成功回傳 1。

DPMI 0.9 `INT 31h/AX=0600h` 契約將 BX:CX 視為開始線性位址、SI:DI 視為
鎖定長度，成功清除 carry；不支援虛擬記憶體的 host 可以無副作用地
回報成功。來源：
<https://www.delorie.com/djgpp/doc/dpmi/api/310600.html>。

因此 dosgolem 可在無虛擬記憶體的決定性 host 模型中回報成功，但必須
先驗證 AX、BX:CX、SI:DI 與範圍不溢位；其他 DPMI 功能維持失敗即關閉。

IDA 報告 SHA-256：
`90183628879a3dc4c252605cbec3c504853716edcaee88a62d6bc2c74894be21`。
