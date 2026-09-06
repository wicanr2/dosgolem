# 174 — CPU386 比較暫存器與 ESP+disp8 dword

狀態：**CONFORMED**  
日期：2026-09-06  
前置：[`RE 116`](../re/116-fd2-ini-radix-digit-loop-bound.md)

- 擴充無前綴 opcode `3B /r`，僅接受 `mod=1`、`r/m=4`、SIB `24` 的
  `cmp r32,dword ptr [esp+disp8]`。
- displacement 以有號 8 位延伸，資料使用 SS；以 `r32-memory` 更新
  CF、PF、AF、ZF、SF、OF，不寫回暫存器或記憶體。
- 讀取越界時失敗且保持通用暫存器與記憶體不變。
- operand-size、segment、repeat prefix 與其他 SIB 形狀維持失敗即關閉
  （fail-closed）。

驗收：CPU 測試覆蓋相等、unsigned borrow、signed overflow、負 displacement、
來源不變及越界；固定雜湊 FD2 自然執行 `0x3F2D1 3B 5C 24 1C`，確認比較
EBX 與 radix 上界後抵達 `0x3F2D5`。

## 驗收收據

- `go test ./internal/cpu386 ./internal/machine -run
  'TestCompareRegisterWithStackDisp8Dword|TestFD2BoundsINIRadixDigitScan'
  -count=1` 通過。
- 有界自然執行 13,221 步後，下一個失敗即關閉位置為 `0x3F2A5`
  （錯誤回報時 EIP 已指向 `0x3F2A8`），opcode `0F B6`、ModRM `B3`；它不屬於
  本規格，未被本切片順帶放寬。
