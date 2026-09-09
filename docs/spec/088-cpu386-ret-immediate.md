# 088 — 386 返回並清理 immediate stack bytes

狀態：**CONFORMED**
日期：2026-09-06
前置：[`RE 058`](../re/058-fd2-watcom-stack-probe.md)

- 支援無前綴 opcode `C2` 的 `RET imm16`。
- 先讀取 SS:ESP 的 32 位回傳位址，成功後設定 EIP，並將 ESP 增加
  `4+imm16`。堆疊讀取失敗或 ESP 加法溢位時，EIP 與 ESP 均不修改。
- operand-size、segment 與 repeat 前綴失敗即關閉；現有 `C3` 契約不變。
- 固定 FD2 在 `0x36CE4` 以 `C2 0400` 返回並清除 stack probe 參數。

驗收：單元測試覆蓋成功返回與溢位不改狀態；固定雜湊 FD2 由 LE entry
自然完成 `sub_36CD7` 並回到 `main+0xA`。

驗收收據（2026-09-06）：`TestReturnImmediate32` 通過；固定雜湊由 LE entry
執行 1107 步後停在 `0x25BFE`，ESP 回復 `0x55698`。
