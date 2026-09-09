# 119 — 386 短距離無號低於跳躍

狀態：**CONFORMED**
日期：2026-09-06
前置：[`RE 079`](../re/079-fd2-ail-table-loop-bound.md)

- 支援無 prefix 的 `JB rel8`（opcode `72h`）。
- CF=1 時將 sign-extended rel8 加到下一指令 EIP；CF=0 時順序執行。
- 不修改任何旗標或通用暫存器。
- operand-size、segment、repeat 等未列 prefix 維持失敗即關閉。

驗收：合成測試覆蓋向後已跳、未跳與狀態不變；固定雜湊 FD2 必須由
LE entry 自然完成 16 個 AIL 表項掃描，EDI=`0x40` 並抵達 `0x3E8FA`。

驗收收據（2026-09-06）：`TestShortJumpBelow` 覆蓋向後已跳、未跳與
旗標不變；`TestFD2CompletesAILActiveTableScan` 由固定原版 LE entry 自然
完成 16 項掃描，EDI=`0x40`、目前最小值 ECX=`0xD68D`，抵達 `0x3E8FA`。
