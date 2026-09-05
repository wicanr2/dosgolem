# 084 — 386 暫存器 NEG

狀態：**CONFORMED**
日期：2026-09-06
前置：[`RE 058`](../re/058-fd2-watcom-stack-probe.md)

- 支援無前綴 `F7 /3`、`mod=3` 的 `NEG r32`。
- 結果等同 `0-value`，沿用完整 32 位 subtraction 旗標；其他 F7 group、
  記憶體形狀與前綴失敗即關閉。
- 固定 FD2 在 `0x36CF0` 以 `F7 D8` 執行 `neg eax`，將 stack distance
  轉成正值再與界限比較。

驗收：單元測試覆蓋一般值、零、`0x80000000` 與未列 F7 形狀；固定雜湊
FD2 必須由 LE entry 自然經過 `0x36CF0`。

驗收收據（2026-09-06）：`TestNegRegister32` 與固定雜湊 stack probe 實驗通過。
