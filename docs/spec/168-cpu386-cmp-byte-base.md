# 168 — CPU386 基址間接 byte CMP

狀態：**CONFORMED**  
日期：2026-09-06  
前置：[`RE 113`](../re/113-fd2-ini-key-value-split.md)

- 擴充無前綴 opcode `80 /7`，接受 `mod=0`、非 ESP／EBP 的
  `CMP byte ptr [r32],imm8`；資料 segment 使用 DS。
- 依 8 位減法結果更新 CF、PF、AF、ZF、SF、OF，但不寫回來源記憶體，也不
  修改基址與其他暫存器。
- 來源讀取失敗時維持記憶體與通用暫存器不變並失敗。
- ESP SIB、EBP 絕對 disp32、非 `/7`、operand-size、segment／repeat prefix
  與其他形狀維持失敗即關閉（fail-closed）。

驗收：CPU 測試覆蓋相等、unsigned borrow、有號溢位、非 ESI 基址與越界；
固定雜湊 FD2 自然執行 `0x3F44C`，確認比較 `[EBX]` 與 `';'` 後抵達
`0x3F44F`，且來源與 EBX 不變。

## 驗收收據

- `go test ./internal/cpu386 ./internal/machine -run
  'TestByteCMPAtBase|TestFD2ChecksINICommentMarker' -count=1` 通過。
- 有界自然執行 9,580 步後，下一個失敗即關閉位置為 `0x46C22`
  （錯誤回報時 EIP 已指向 `0x46C24`），opcode `80`、ModRM `C1`；它不屬於
  本規格允許的記憶體形狀，未被本切片順帶放寬。
