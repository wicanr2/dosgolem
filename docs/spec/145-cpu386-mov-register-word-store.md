# 145 — CPU386 16-bit register-direct MOV store

狀態：**CONFORMED**  
日期：2026-09-06  
前置：[`RE 102`](../re/102-fd2-isatty-ioctl-device-query.md)

## 範圍

- 擴充 operand-size `66h` 加 opcode `89h`，支援 register-direct
  `MOV r16,r16`。
- 來源取 ModRM reg 指定暫存器低 16 bits；只覆寫 ModRM r/m 目的暫存器
  低 16 bits，保留目的暫存器高 16 bits。
- 指令不修改旗標。

## 失敗即關閉

- 不擴張其他 16-bit `89h` 記憶體形狀或 segment／repeat prefix。

## 驗收

- CPU 單元測試驗證不同來源／目的、保留目的高 word、來源不變及旗標不變。
- 固定原版由 LE entry 自然執行 `0x3FB16` 的 `mov bx,ax`，抵達
  `0x3FB19` 且 BX 等於已登錄的 DOS handle。

本規格不宣告 `AH=44h` DOS IOCTL 已完成。

驗收收據（2026-09-06）：`TestStoreRegisterMOV16` 驗證保留目的高 word、
來源與旗標不變；`TestFD2MovesHandleIntoBXForIOCTL` 由固定原版 LE entry
自然執行 `0x3FB16`，抵達 `0x3FB19` 且 BX 等於已登錄 handle。後續有界
探針在 `0x3FB1D` 以 `AX=4400h`、`BX=5` 遇到尚未處理的 `int 21h`。
