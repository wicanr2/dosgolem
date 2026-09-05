# 094 — Watcom `int386` DPMI 線性區域鎖定

狀態：**CONFORMED**
日期：2026-09-06
前置：[`RE 059`](../re/059-fd2-ail-dpmi-lock-region.md)、
[`RE 060`](../re/060-fd2-watcom-int386-wrapper.md)

- 在固定 FD2 的 Watcom `int386` 入口 `0x36D98` 建立 runtime hook。
- 只接受 cdecl 參數 `intno=0x31`，且輸入 `REGS.eax&0xFFFF=0x0600`。
- `REGS` 佈局為 EAX、EBX、ECX、EDX、ESI、EDI、CFLAG 七個 dword；
  輸入與輸出範圍必須完整可讀寫。
- 開始位址為 `(EBX&0xFFFF)<<16 | (ECX&0xFFFF)`，長度為
  `(ESI&0xFFFF)<<16 | (EDI&0xFFFF)`。長度必須非零，範圍不得溢位且必須落在
  dosgolem 已配置記憶體。
- dosgolem 目前沒有虛擬記憶體分頁；依 DPMI 0.9 契約，對合法區域可以
  無副作用回報成功。輸出複製輸入寄存器，將 CFLAG 設為 0，EAX 回傳
  輸出 EAX。
- 其他 `int386`、DPMI 功能、越界與無效堆疊一律失敗即關閉。

驗收：單元測試覆蓋成功、未列中斷、未列 AX 與區域越界；固定雜湊
FD2 必須由 LE entry 自然完成 `sub_36284`。

驗收收據（2026-09-06）：`TestWatcomInt386DPMILockLinearRegion`、
`TestWatcomInt386DPMIRejectsUnknownInterrupt`、
`TestWatcomInt386DPMIRejectsUnknownFunctionAndRange` 與固定雜湊
`TestFD2CompletesFirstDPMILockWhenProvided` 通過。
