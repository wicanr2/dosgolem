# 169 — CPU386 暫存器 byte ADD immediate

狀態：**CONFORMED**  
日期：2026-09-06  
前置：[`RE 114`](../re/114-fd2-strnicmp-ascii-fold.md)

- 擴充無前綴 opcode `80 /0`、`mod=3` 的 `ADD r8,imm8`。
- 以現有 8 位暫存器映射讀寫 AL／CL／DL／BL／AH／CH／DH／BH，並依 8 位
  加法更新 CF、PF、AF、ZF、SF、OF。
- operand-size、segment、repeat prefix，以及 `80 /0` 的記憶體形狀維持
  失敗即關閉（fail-closed）。

驗收：CPU 測試覆蓋一般加法、進位、帶符號溢位及高 8 位暫存器；固定雜湊
FD2 自然執行 `0x46C22`，確認 CL 由 ASCII 大寫加 `20h` 且抵達 `0x46C25`。

## 驗收收據

- `go test ./internal/cpu386 ./internal/machine -run
  'TestByteADDRegisterImmediate|TestFD2FoldsINIKeyASCIIUppercase' -count=1`
  通過。
- 有界自然執行 9,588 步後，下一個失敗即關閉位置為 `0x46C40`
  （錯誤回報時 EIP 已指向 `0x46C41`），opcode `84`；它不屬於本規格，未被
  本切片順帶放寬。
