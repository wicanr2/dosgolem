# 179 — CPU386 將 AX 寫入 SS:ESP+disp32

狀態：**CONFORMED**
日期：2026-09-06
前置：[`RE 121`](../re/121-fd2-ini-io-address-word-store.md)

- 擴充 operand-size override `66 89 84 24 disp32`：
  `mov word ptr [esp+disp32],ax`。
- disp32 有號加入 ESP，目的固定透過 SS 描述子（descriptor）寫入 little-endian
  word；只讀取 EAX 低 16 位。
- 成功時 EAX、ESP、其他暫存器及旗標保持不變。
- 寫入越界或唯讀描述子時失敗，且不得部分寫入或修改 CPU 狀態。
- segment／repeat prefix、其他來源暫存器、ModRM、SIB 與位址計算（addressing）形狀
  維持失敗即關閉（fail-closed）。

## 驗收條件

- CPU 測試覆蓋正／負 disp32、little-endian word、狀態不變、唯讀與邊界拒絕、
  錯誤 SIB 拒絕。
- 固定雜湊 FD2 自然執行 `0x3F4EE`，確認 `SS:[ESP+0x100]` 等於 AX 並抵達
  `0x3F4F6`。
- 有界自然執行記錄下一個失敗即關閉位址，不順帶放寬後續指令。

## 驗證收據

- 固定雜湊 FD2 自然執行在 15,000 步內抵達 `0x3F4EE`；寫入後抵達
  `0x3F4F6`，`SS:[ESP+0x100]` 等於 AX，暫存器與旗標保持不變。
- CPU 聚焦測試涵蓋正／負 disp32、低 16 位、小端序、唯讀／越界拒絕與錯誤
  SIB 拒絕，並確認失敗時不部分寫入。
- 強制重建後的有界自然探針抵達下一個失敗即關閉位址 `0x3F289`：
  opcode `F7`、ModRM `5C`。本規格未順帶解釋或放寬該指令。
