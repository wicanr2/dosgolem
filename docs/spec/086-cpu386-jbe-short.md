# 086 — 386 無符號小於等於短跳躍

狀態：**CONFORMED**
日期：2026-09-06
前置：[`RE 058`](../re/058-fd2-watcom-stack-probe.md)

- 支援無前綴 opcode `76` 的 `JBE rel8`。
- 當 `CF=1` 或 `ZF=1` 時，將有符號 rel8 加到已取得位移後的 EIP；
  否則不跳躍。不修改旗標。
- operand-size、segment 與 repeat 前綴失敗即關閉。
- 固定 FD2 在 `0x36CF8` 以 `76 01` 選擇正常 stack probe 路徑。

驗收：單元測試分別覆蓋 CF、ZF 與不跳躍；固定雜湊 FD2 由 LE entry
自然經過 `0x36CF8`。

驗收收據（2026-09-06）：`TestJBEShort` 與固定雜湊 stack probe 正常分支通過。
