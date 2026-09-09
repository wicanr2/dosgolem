# 032 — 386 protected-mode PUSH ES

狀態：**CONFORMED**  
日期：2026-09-06  
前置：[`016`](016-dos4gw-protected-push.md)、[`RE 024`](../re/024-fd2-first-callee-prologue.md)

- 無 operand-size override 的 `06` 將 ES selector 零擴展成 32-bit stack cell。
- 先檢查 ESP underflow，再經 SS descriptor range／writable gate 寫入 ESP-4；成功後
  才提交 ESP。ES 本身與旗標不得改變。
- `66 06` 尚未支援。
- 固定雜湊 FD2 從 `0x45D9A` 抵達 `0x45DAC`，ESP=`0x5569C`，
  SS:`0x5569C`=`0x00000160`。

驗收包含獨立 PUSH ES stack cell 與固定原檔 callee prologue。

2026-09-06：上述單元測試與固定雜湊 FD2 整合測試通過，抵達 `0x45DAC`。
