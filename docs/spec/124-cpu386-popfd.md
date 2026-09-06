# 124 — 386 POPFD

狀態：**CONFORMED**
日期：2026-09-06
前置：[`RE 084`](../re/084-fd2-ail-restore-eflags.md)

- 支援無 prefix 的 `POPFD`（opcode `9Dh`）。
- 從 SS:ESP 讀取 32 位元值至 EFLAGS，ESP 增加 4。
- stack 讀取失敗或 ESP 加法溢位時，ESP 與 EFLAGS 保持不變並失敗。
- operand16、segment、repeat 等未列 prefix 維持失敗即關閉。
- 目前平坦單 ring 模型不實作 privilege-dependent IF／IOPL 遮罩。

驗收：合成測試覆蓋成功與越界原子拒絕；固定雜湊 FD2 必須確認
`0x3E86A PUSHFD` 保存的旗標，在 `0x3E88E POPFD` 後完整恢復並抵達
`0x3E88F`。

驗收收據（2026-09-06）：`TestPopFlags32` 覆蓋成功與越界原子拒絕；
`TestFD2RestoresPITCallerFlags` 由固定原版 LE entry 比較 `0x3E86A` 前與
`0x3E88E` 後旗標，確認完整相同並抵達 `0x3E88F`。
