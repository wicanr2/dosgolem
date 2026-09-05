# 082 — 386 immediate dword 壓棧

狀態：**CONFORMED**
日期：2026-09-06
前置：[`RE 057`](../re/057-fd2-main-entry.md)

- 支援無前綴 opcode `68` 的 `PUSH imm32`，指令中的 32 位值原樣壓入
  SS:ESP。
- 只有在即時值完整取得、ESP 無下溢且堆疊 dword 可寫時才更新
  ESP；任一步失敗不得修改 ESP 或堆疊。
- operand-size override、segment override 與 repeat 前綴維持失敗即關閉。
- 固定 FD2 的 `main` 在 `0x25BF4` 以 `68 1C000000` 壓入 `0x1C`。

驗收：單元測試覆蓋成功、不完整 immediate 與無效堆疊不改 ESP；
固定雜湊 FD2 必須由 LE entry 自然經過 `0x25BF4`。

驗收收據（2026-09-06）：`TestPushImmediateDword` 通過；
`TestFD2CompletesWatcomStackProbeWhenProvided` 從 LE entry 自然經過該指令。
