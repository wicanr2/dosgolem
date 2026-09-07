# 058 — 386 暫存器 TEST

狀態：**CONFORMED**  
日期：2026-09-06  
前置：[`RE 050`](../re/050-fd2-third-callback-allocation-consumer.md)

- 支援無前綴 `85 /r` 且 `mod=3` 的 32 位元 `TEST r/m32,r32`。
- 計算兩個暫存器的位元 AND，只更新邏輯旗標，不修改任一運算元。
- 記憶體 ModRM、operand-size、segment override 與 REP 形狀維持失敗即關閉。
- 單元測試驗證零／非零旗標及暫存器不變；固定 FD2 必須由 LE 入口自然越過
  `0x4CC58`，走到第二次 `_nmalloc` 的返回點 `0x4CC70`。

2026-09-06：暫存器不變、零／非零旗標與拒絕記憶體形狀的單元測試通過；
固定雜湊整合路徑已越過 `0x4CC58` 與 `0x4CC73`。
