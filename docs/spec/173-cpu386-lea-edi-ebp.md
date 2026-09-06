# 173 — CPU386 載入 EDI+EBP effective address

狀態：**CONFORMED**  
日期：2026-09-06  
前置：[`RE 115`](../re/115-fd2-ini-number-parser-stack-state.md)

- 擴充無前綴 `8D 04 2F`：`lea eax,[edi+ebp]`。
- 結果依 32 位 unsigned 加法自然回繞，只寫 EAX；不得存取記憶體、修改 EDI、
  EBP 或旗標。
- operand-size、segment、repeat prefix 與其他 `mod=0` SIB 形狀維持失敗即關閉
  （fail-closed）。

驗收：CPU 測試覆蓋一般加法、回繞、狀態不變及未列 SIB 拒絕；固定雜湊 FD2
自然執行 `0x3F27F 8D 04 2F`，確認 EAX 等於 EDI+EBP 後抵達 `0x3F282`。

## 驗收收據

- `go test ./internal/cpu386 ./internal/machine -run
  'TestLEAEAXFromEDIPlusEBP|TestFD2AddressesCurrentININumberByte' -count=1`
  通過。
- 有界自然執行 13,219 步後，下一個失敗即關閉位置為 `0x3F2D1`
  （錯誤回報時 EIP 已指向 `0x3F2D3`），opcode `3B`、ModRM `5C`；它不屬於
  本規格，未被本切片順帶放寬。
