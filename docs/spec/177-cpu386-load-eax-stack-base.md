# 177 — CPU386 從 SS:ESP 載入 EAX

狀態：**CONFORMED**
日期：2026-09-06
前置：[`RE 119`](../re/119-fd2-ini-accumulator-stack-load.md)

- 擴充無前綴 `8B 04 24`：`mov eax,dword ptr [esp]`。
- 來源固定透過 SS descriptor 讀取 ESP 指向的 little-endian dword。
- 成功時覆寫完整 EAX；ESP、來源記憶體及旗標保持不變。
- 讀取越界時失敗，且不得修改 EAX、ESP、記憶體或旗標。
- operand-size、segment、repeat prefix、其他 ModRM `04` SIB 與其他尚未列入的
  `8B` SIB 形狀維持失敗即關閉（fail-closed）。

## 驗收條件

- CPU 測試覆蓋 little-endian 載入、狀態不變、descriptor 邊界及錯誤 SIB 拒絕。
- 固定雜湊 FD2 自然執行 `0x3F2C1`，確認 EAX 等於 SS:[ESP] 並抵達
  `0x3F2C4`。
- 有界自然執行記錄下一個失敗即關閉位址，不順帶放寬後續指令。

## 驗收收據

- `go test -p=1 -v ./internal/cpu386 ./internal/machine -run
  'TestLoadEAXFromStackBase|TestFD2LoadsINIAccumulatorFromStack' -count=1` 通過；
  固定雜湊 FD2 測試未略過。
- CPU 測試覆蓋 little-endian 載入、狀態不變、descriptor 越界及錯誤 SIB
  拒絕。
- 固定 FD2 自然執行通過 `0x3F2C1`；有界追蹤在第 13,273 步得到下一個
  失敗即關閉位置 `0x3F2C4`（錯誤回報時 EIP 為 `0x3F2C6`），opcode
  `0F AF`、ModRM `44`、SIB `24`、disp8 `1C`。該乘法指令不屬於本規格，
  未被順帶放寬。
