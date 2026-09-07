# 031 — 386 protected-mode near CALL

狀態：**CONFORMED**  
日期：2026-09-06  
前置：[`016`](016-dos4gw-protected-push.md)、[`RE 023`](../re/023-fd2-first-startup-call.md)

- `E8 cd` 在 32-bit operand size 讀取 signed rel32。
- 先以 SS descriptor 將下一個 EIP 作 32-bit return address 寫至 ESP-4；通過範圍與
  writable gate 後才提交 ESP 與 EIP。
- ESP underflow、stack descriptor 拒絕或寫入失敗時，控制狀態不得前進到 target。
- `66 E8` 尚未支援。
- 固定雜湊 FD2 從 `0x3CB85` 抵達 `0x45D9A`，ESP=`0x556AC`，stack cell
  `0x556AC`=`0x3CB8A`。

驗收包含獨立正位移 CALL、return address 寫入，以及固定原檔控制轉移。

2026-09-06：上述單元測試與固定雜湊 FD2 整合測試通過，抵達 `0x45D9A`。
