# 069 — 386 base＋disp8 dword CMP

狀態：**CONFORMED**  
日期：2026-09-06  
前置：[`RE 052`](../re/052-fd2-post-environment-runtime-list.md)

- 支援無前綴 `83 /7`、`mod=1`、非 SIB base 暫存器的 32 位元記憶體比較。
- disp8 與 imm8 均符號延伸；只更新 subtraction 旗標，不修改記憶體或 base。
- SIB、其他群組及所有新前綴形狀維持失敗即關閉。
- 固定 FD2 在 `0x4692B` 以 `83 7B 0C 00` 檢查 stride `0x1A` 紀錄欄位。

2026-09-06：零值欄位比較與固定紀錄掃描迴圈測試通過。
