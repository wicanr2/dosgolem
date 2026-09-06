# 149 — CPU386 register byte 零擴展

狀態：**CONFORMED**  
日期：2026-09-06  
前置：[`RE 102`](../re/102-fd2-isatty-ioctl-device-query.md)

- 擴充既有 opcode `0F B6`，支援 register-direct `MOVZX r32,r8`。
- 來源依 x86 byte register mapping 取低／高 byte，目的寫入零擴展的 32-bit
  值；來源與目的重疊時先讀來源再覆寫目的。
- 不修改旗標；prefix 與其他記憶體形狀維持既有失敗即關閉契約。

驗收：CPU 測試覆蓋低／高 byte、不同及相同來源／目的與旗標不變；固定原版
自然執行 `0x3FB29` 的 `movzx eax,al`，抵達 `0x3FB2C` 且 EAX=0。

本規格不宣告 `isatty` 以外的 DOS 檔案服務已完成。

驗收收據（2026-09-06）：`TestMOVZXRegisterByte` 覆蓋低／高 byte、不同及
相同來源／目的與旗標不變；`TestFD2ReturnsOpenedFileIsNotTTY` 由固定原版
LE entry 自然執行 `0x3FB29`，抵達 `0x3FB2C` 且 EAX=0。後續有界探針已
完整返回 `isatty`，下一阻塞移至 `__IOMode` 的 `0x4639D`：
`8B 04 B0`。
