# 175 — CPU386 從 EBX+disp32 零擴展 byte

狀態：**CONFORMED**  
日期：2026-09-06  
前置：[`RE 116`](../re/116-fd2-ini-radix-digit-loop-bound.md)

- 擴充無前綴 `0F B6 B3 disp32`：`movzx esi,byte ptr [ebx+disp32]`。
- displacement 以有號 32 位加入 EBX，資料使用 DS；成功時只寫 ESI 的完整
  32 位零擴展結果，不修改 EBX、記憶體或旗標。
- 讀取越界時失敗，並保持 ESI、EBX 與記憶體不變。
- operand-size、segment、repeat prefix 與其他尚未列入的 `0F B6` disp32
  形狀維持失敗即關閉（fail-closed）。

驗收：CPU 測試覆蓋正／負 displacement、零擴展、狀態不變及越界；固定雜湊
FD2 自然執行 `0x3F2A5 0F B6 B3 B4 11 05 00`，確認由 digit table 載入 ESI
後抵達 `0x3F2AC`。

## 驗收收據

- `go test ./internal/cpu386 ./internal/machine -run
  'TestMoveZXByteFromEBXDisp32ToESI|TestFD2LoadsINIRadixDigitCandidate'
  -count=1` 通過。
- 有界自然執行 13,222 步後，下一個失敗即關閉位置為 `0x3F2AC`
  （錯誤回報時 EIP 已指向 `0x3F2AE`），opcode `8A`、ModRM `04`；它不屬於
  本規格，未被本切片順帶放寬。
