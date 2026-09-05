# 030 — 386 REP STOSB 與 accumulator immediate ALU

狀態：**CONFORMED**  
日期：2026-09-06  
前置：[`029`](029-cpu386-startup-buffer-clear.md)、[`RE 022`](../re/022-fd2-startup-buffer-tail.md)

- `F3 AA` 重複 STOSB ECX 次，每次依 DF 更新 EDI 並遞減 ECX；初值 0 不得寫入。
- `05 id` 以 32-bit ADD 更新 EAX 與完整算術旗標；`66 05` 尚未支援。
- `24 ib` 對 AL 執行 AND、只更新 AL，並依 8-bit 結果更新邏輯旗標。
- 所有 STOSB 寫入仍經 ES descriptor 的範圍與 writable gate。
- 固定雜湊 FD2 由 `0x3CB6C` 抵達 CALL 前 `0x3CB85`，EAX=`0x546B0`、
  ESI=`0x546B1`、EDI=`0x546B0`、ECX=0。

驗收包含非零 REP STOSB、零 count 固定路徑、立即值 ALU flags 與固定原檔整合。

2026-09-06：上述單元測試與固定雜湊 FD2 整合測試通過，抵達 `0x3CB85`。
