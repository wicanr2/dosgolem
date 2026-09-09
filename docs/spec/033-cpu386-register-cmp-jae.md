# 033 — 386 register CMP 與 short JAE

狀態：**CONFORMED**  
日期：2026-09-06  
前置：[`RE 025`](../re/025-fd2-first-callee-range-check.md)

- `3B /r` 本切片只支援 register-direct 32-bit CMP；以 subtraction 規則更新旗標，
  不修改兩個 register。memory 與 `66 3B` 維持失敗即關閉。
- `73 cb` 在 CF=0 時採 signed rel8，否則落下；operand-size override 拒絕。
- 固定雜湊 FD2 從 `0x45DAC` 抵達 `0x45DB0`，ESI／EDI 不變，CF=1、ZF=0。

驗收包含小於與相等兩組 compare／branch，以及固定原檔 callee range gate。

2026-09-06：上述單元測試與固定雜湊 FD2 整合測試通過，抵達 `0x45DB0`。
