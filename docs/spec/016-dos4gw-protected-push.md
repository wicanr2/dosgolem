# 016 — DOS/4GW protected-mode 32-bit PUSH register

狀態：**CONFORMED**
日期：2026-09-06
前置：[`014`](014-dos4gw-flat-descriptors.md)、[`RE 008`](../re/008-fd2-protected-stack-entry.md)

cpu386 支援 opcode `50h..57h` 的預設 32-bit register push：先計算 `ESP-4`，再透過
目前 SS selector 的 descriptor 以 little-endian dword 寫入，成功後更新 ESP。
未知 selector、唯讀 descriptor、limit／Bus 越界或 ESP underflow 皆失敗即關閉，
且失敗時 ESP 不變。operand-size override 的 16-bit push 暫不支援。

驗收涵蓋合成 descriptor、拒絕案例及固定雜湊 FD2 的
`0x3CACE -> 0x3CACF`；後者核對 `ESP=0x556AC` 與 stack dword 為 0。

2026-09-06 全套測試與固定雜湊 FD2 整合測試通過；下一個未納入指令為
`0x3CACF: 8E 05 10 28 05 00`。
