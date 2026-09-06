# 171 — CPU386 以 ESP 為基址儲存 dword

狀態：**CONFORMED**  
日期：2026-09-06  
前置：[`RE 115`](../re/115-fd2-ini-number-parser-stack-state.md)

- 擴充無前綴 opcode `89 /r`，僅接受 `mod=0`、`r/m=4`、SIB `24` 的
  `mov [esp],r32`。
- effective address 為目前 ESP，segment 固定使用 SS；成功時寫入來源完整
  dword，不修改來源、ESP 或旗標。
- 寫入越界或 descriptor 不可寫時失敗，且不得部分寫入。
- operand-size、segment、repeat prefix 與其他尚未列入的 SIB 形狀維持失敗即
  關閉（fail-closed）。

驗收：CPU 測試覆蓋成功寫入、狀態不變及唯讀拒絕；固定雜湊 FD2 自然執行
`0x3F270 89 14 24`，確認 EDX 的零值寫至 `[SS:ESP]` 後抵達 `0x3F273`。

## 驗收收據

- `go test ./internal/cpu386 ./internal/machine -run
  'TestStoreRegisterToStackBase|TestFD2InitializesININumberParserState' -count=1`
  與完整 `go test ./... -count=1` 均通過。
- 有界自然執行 13,182 步後，下一個失敗即關閉位置為 `0x3F273`
  （錯誤回報時 EIP 已指向 `0x3F275`），opcode `C7`、ModRM `44`；它不屬於
  本規格，未被本切片順帶放寬。
