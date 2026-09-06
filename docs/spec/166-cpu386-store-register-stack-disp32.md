# 166 — CPU386 寫入 stack＋disp32 dword

狀態：**CONFORMED**  
日期：2026-09-06  
前置：[`RE 112`](../re/112-fd2-ini-value-pointer-slot.md)

- 擴充無前綴 opcode `89 /r`，只接受 `mod=2,r/m=4,SIB=0x24` 的
  `MOV dword ptr [ESP+disp32],r32`。
- disp32 以有號 32 位加入 ESP，目的 segment 固定使用 SS；來源由 ModRM reg
  欄位選擇。
- descriptor 不可寫或超界時不得修改記憶體。MOV 不修改來源、ESP 或旗標。
- operand-size、segment／repeat prefix、其他 SIB 與其他 addressing shape
  維持失敗即關閉（fail-closed）。

驗收：CPU 測試覆蓋負 disp32、任意來源、旗標不變與唯讀失敗；固定雜湊 FD2
自然執行 `0x3F3F0`，確認 EAX 寫入 `SS:[ESP+0x168]` 並抵達 `0x3F3F7`。

驗收收據（2026-09-06）：`TestStoreRegisterToStackDisp32` 覆蓋負 disp32、
EDX／EAX 來源、ESP 與旗標不變及唯讀失敗；`TestFD2StoresINIValuePointer`
由固定原版 LE entry 自然執行 `0x3F3F0`，確認 EAX 寫入
`SS:[ESP+0x168]` 並抵達 `0x3F3F7`。後續一次性有界探針已刪除，原版再
前進約 2500 steps；下一阻塞移至 `0x3F449` 的 `C6 00`。
