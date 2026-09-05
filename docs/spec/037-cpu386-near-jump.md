# 037 — 386 near relative JMP

狀態：**CONFORMED**  
日期：2026-09-06  
前置：[`RE 029`](../re/029-fd2-first-callback-thunk.md)

- `E9 cd` 在 32-bit operand size 讀 signed rel32，以 immediate 後 EIP 為基準更新 EIP。
- 不讀寫 stack、register 或 flags；`66 E9` 尚未支援。
- 固定雜湊 FD2 從 thunk `0x3CBCC` 抵達 `0x45E36`，ESP 保持 `0x55698`。

驗收包含獨立正位移 near JMP 與固定 callback thunk。

2026-09-06：上述單元測試與固定雜湊 callback thunk 測試通過，抵達 `0x45E36`。
