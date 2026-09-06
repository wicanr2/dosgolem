# 163 — CPU386 暫存器 byte 遞增

狀態：**CONFORMED**  
日期：2026-09-06  
前置：[`RE 111`](../re/111-fd2-ini-trailing-space-scan.md)

- 實作無前綴 opcode `FE /0`、`mod=3` 的 `INC r8`。
- r8 採 AL／CL／DL／BL／AH／CH／DH／BH 映射；只修改選定 byte。
- 依 8 位 INC 更新 OF、SF、ZF、AF、PF，CF 保留原值。
- memory operand、其他 FE group、operand-size、segment／repeat prefix 維持
  失敗即關閉（fail-closed）。

驗收：CPU 測試覆蓋一般結果、`0x7F→0x80` 溢位、`0xFF→0`、high-byte
暫存器與 CF 保留；固定雜湊 FD2 自然執行 `0x3F362` 的 `FE C0`，確認 AL
增加一並抵達 `0x3F364`。

驗收收據（2026-09-06）：`TestIncrementRegisterByte` 覆蓋 AL、AH、
`0x7F→0x80`、`0xFF→0`、完整算術旗標與 CF 保留，並拒絕 memory operand；
`TestFD2IncrementsINITrailingCharacter` 由固定原版 LE entry 自然執行
`0x3F362`，確認 AL 增加一並抵達 `0x3F364`。後續一次性有界探針已刪除，
下一阻塞移至 `0x3F369` 的 `F6 80`。
