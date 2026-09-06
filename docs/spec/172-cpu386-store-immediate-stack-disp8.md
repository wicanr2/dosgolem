# 172 — CPU386 將 immediate dword 寫入 ESP+disp8

狀態：**CONFORMED**  
日期：2026-09-06  
前置：[`RE 115`](../re/115-fd2-ini-number-parser-stack-state.md)

- 擴充無前綴 opcode `C7 /0`，僅接受 `mod=1`、`r/m=4`、SIB `24` 的
  `mov dword ptr [esp+disp8],imm32`。
- displacement 以有號 8 位延伸，effective address 使用目前 ESP，segment 固定
  使用 SS；成功時不修改 ESP、通用暫存器或旗標。
- 寫入越界或 descriptor 不可寫時失敗，且不得部分寫入。
- operand-size、segment、repeat prefix、非 `/0` 及其他 SIB 形狀維持失敗即關閉
  （fail-closed）。

驗收：CPU 測試覆蓋正／負 displacement、狀態不變及唯讀拒絕；固定雜湊 FD2
自然執行 `0x3F273 C7 44 24 04 01 00 00 00`，確認 `[SS:ESP+4]` 寫入 1 後
抵達 `0x3F27B`。

## 驗收收據

- `go test ./internal/cpu386 ./internal/machine -run
  'TestMoveImmediateDwordToStackSIBDisp8|TestFD2InitializesININumberParserSign'
  -count=1` 通過。
- 有界自然執行 13,208 步後，下一個失敗即關閉位置為 `0x3F27F`
  （錯誤回報時 EIP 已指向 `0x3F281`），opcode `8D`、ModRM `04`；它不屬於
  本規格，未被本切片順帶放寬。
