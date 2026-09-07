# 013 — FD2 載入 ES selector

狀態：**CONFORMED**
日期：2026-09-06
前置：[`011`](011-fd2-es-environment-cell.md)、[`RE 005`](../re/005-fd2-load-es-selector.md)

## 契約

cpu386 支援 opcode `8E /r` 的窄 register-direct 形式：

- ModR/M `mod=3`；
- `reg` 欄位只接受 ES、SS、DS、FS、GS，拒絕 CS 與保留編碼；
- 從指定 16-bit 通用暫存器低字載入 segment register；
- EIP 前進兩 bytes，EFLAGS 不變；
- memory operand、descriptor 驗證與 segment cache 不在本切片，遇到時失敗即關閉。

固定雜湊 FD2 的驗收錨點是 `0x3CAB8: 8E C3`：執行前 `BX=0x0160`，執行後
`ES=0x0160` 且 `EIP=0x3CABA`。不得因此宣稱下一筆 ES-relative memory consumer 已完成。

## 驗收

- 單元測試涵蓋 `MOV ES,BX`、旗標不變、CS destination 拒絕及 memory operand 拒絕；
- 固定雜湊 FD2 從 LE entry 執行越過 `0x3CAB8`，並在下一個未支援邊界停止；
- 完成後將本規格升為 `CONFORMED`，保存新的停止點而不猜補後續語意。

2026-09-06 固定雜湊整合測試已從 LE entry 執行至 `0x3CABA`，核對
`ES=0x0160`；下一筆 ES-relative memory consumer 仍失敗即關閉。
