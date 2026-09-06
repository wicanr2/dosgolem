# 142 — CPU386 暫存器 word 零擴展

狀態：**CONFORMED**  
日期：2026-09-06  
前置：[`RE 100`](../re/100-fd2-sopen-carry-to-signed-result.md)

## 範圍

- 擴充既有 opcode `0F B7`，支援 register-direct `MOVZX r32,r16`。
- 來源取指定 32-bit 暫存器的低 16 bits，目的暫存器寫入其零擴展值。
- 指令不修改任何狀態旗標；來源與目的相同時亦須先讀低 16 bits 再覆寫。

## 失敗即關閉

- 不新增 operand-size、segment 或 repeat prefix。
- 既有 absolute DS word 形狀維持原契約；其他記憶體 ModRM 仍拒絕。

## 驗收

- CPU 單元測試驗證不同來源／目的、相同來源／目的及旗標不變。
- 固定原版由 LE entry 自然執行 `0x3CD7F` 的 `0F B7 C0`，於
  `0x3CD82` 將零擴展後的 handle 寫入 `[ebp-8]`，並抵達 `0x3CD85`。

本規格不宣告其他 `MOVZX` addressing shape 或 DOS read／seek／close 已完成。

驗收收據（2026-09-06）：`TestMOVZXRegisterWord` 驗證不同及相同來源／目的
暫存器與旗標不變；`TestFD2StoresOpenedHandle` 由固定原版 LE entry 自然執行
`0x3CD7F`，在 `0x3CD82` 將已登錄 handle 保存至 `[ebp-8]` 並抵達
`0x3CD85`。後續有界探針的下一阻塞移至 `0x46375` 的 `F6 44 ...`。
