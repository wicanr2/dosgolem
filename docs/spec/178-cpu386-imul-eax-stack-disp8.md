# 178 — CPU386 以 SS:ESP+disp8 乘入 EAX

狀態：**CONFORMED**
日期：2026-09-06
前置：[`RE 120`](../re/120-fd2-ini-accumulator-radix-multiply.md)

- 擴充無前綴 `0F AF 44 24 disp8`，且只接受目的 reg=EAX、SIB `24`。
- disp8 有號擴展後加入 ESP，來源透過 SS 描述子（descriptor）讀取 dword；
  兩個運算元皆視為 signed 32 位，EAX 寫入乘積低 32 位。
- 完整 signed 64 位乘積無法由低 32 位符號擴展還原時，同時設定 CF 與 OF；
  否則同時清除。x86 未定義的其他算術旗標維持輸入值，確保重播決定性。
- 讀取越界時失敗，且不得修改 EAX、ESP、記憶體或旗標。
- operand-size、segment、repeat prefix、其他目的暫存器、SIB 與 addressing
  形狀維持失敗即關閉（fail-closed）。

## 驗收條件

- CPU 測試覆蓋正數、負數、溢位、負位移、狀態不變、越界與錯誤 SIB 拒絕。
- 固定雜湊 FD2 自然執行 `0x3F2C4`，確認 EAX 等於原值乘以
  `SS:[ESP+0x1C]` 的低 32 位並抵達 `0x3F2C9`。
- 有界自然執行記錄下一個失敗即關閉位址，不順帶放寬後續指令。

## 驗收收據

- `go test -p=1 -v ./internal/cpu386 ./internal/machine -run
  'TestIMULEAXFromStackDisp8|TestFD2MultipliesINIAccumulatorByRadix' -count=1`
  通過；固定雜湊 FD2 測試未略過。
- CPU 測試覆蓋正數、負數、溢位、負位移、來源與 ESP 不變、算術旗標契約、
  descriptor 越界及錯誤 SIB 拒絕。
- 以 `-a` 強制重編的固定 FD2 有界探針通過 `0x3F2C4`；下一個失敗即關閉
  位置為 `0x3F4EE`（錯誤回報時 EIP `0x3F4F1`），opcode `89`、operand-size
  override、ModRM `84`。該指令不屬於本規格，未被順帶放寬。
