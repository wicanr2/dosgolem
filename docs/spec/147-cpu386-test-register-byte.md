# 147 — CPU386 register byte TEST immediate

狀態：**CONFORMED**  
日期：2026-09-06  
前置：[`RE 102`](../re/102-fd2-isatty-ioctl-device-query.md)

## 範圍

- 擴充 opcode `F6 /0`，支援 register-direct `TEST r8,imm8`。
- 以既有高／低 byte register mapping 讀取來源，計算 `value & imm`，並以
  `setLogicFlags8` 更新 CF／PF／AF／ZF／SF／OF；不修改來源暫存器。

## 失敗即關閉

- 不擴張其他 `F6` register group 或 prefix。

## 驗收

- CPU 單元測試驗證低 byte、高 byte、零／非零結果、來源不變與其他 group
  拒絕。
- 固定原版由 LE entry 自然執行 `0x3FB23` 的 `test dl,80h`，抵達
  `0x3FB26`；目前 regular file 的 DX bit 7=0，因此 ZF 必須為 1。

本規格不宣告後續 `SETNZ` 已完成。

驗收收據（2026-09-06）：`TestByteTESTRegister` 覆蓋 DL、AH、零／非零、
來源不變與未授權 group；`TestFD2TestsOpenedFileDeviceBit` 由固定原版
LE entry 自然執行 `0x3FB23`，抵達 `0x3FB26`，確認 regular file 的
DX bit 7=0 且 ZF=1。後續有界探針的下一阻塞為 `0x3FB26` 的 `0F 95 C0`。
