# 170 — CPU386 暫存器 byte TEST

狀態：**CONFORMED**  
日期：2026-09-06  
前置：[`RE 114`](../re/114-fd2-strnicmp-ascii-fold.md)

- 實作無前綴 opcode `84 /r`、`mod=3` 的 `TEST r8,r8`。
- 以現有 8 位暫存器映射讀取 AL／CL／DL／BL／AH／CH／DH／BH，對兩個來源
  做 AND，只更新 PF、ZF、SF，並清除 CF、OF、AF；不得寫回任一來源。
- operand-size、segment、repeat prefix 與所有記憶體 ModRM 形狀維持失敗即關閉
  （fail-closed）。

驗收：CPU 測試覆蓋低／高 8 位、零／非零、來源不變及記憶體形狀拒絕；固定
雜湊 FD2 自然執行 `0x46C40 84 ED`，確認 `test ch,ch` 的旗標及 CH 不變，並
抵達 `0x46C42`。

## 驗收收據

- `go test ./internal/cpu386 ./internal/machine -run
  'TestRegisterTEST8|TestFD2TestsFoldedINIByteForNUL' -count=1` 通過。
- 有界自然執行 13,181 步後，下一個失敗即關閉位置為 `0x3F270`
  （錯誤回報時 EIP 已指向 `0x3F273`），opcode `89` 的 SIB `24`；它不屬於
  本規格，未被本切片順帶放寬。
