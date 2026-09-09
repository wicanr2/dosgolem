# 040 — 386 immediate byte write through DS register

狀態：**CONFORMED**  
日期：2026-09-06  
前置：[`RE 032`](../re/032-fd2-callback-record-state-write.md)

- `C6 /0` 本切片只支援 ModRM `03`，即 DS:`[EBX]` immediate byte write。
- 寫入必須通過 DS descriptor range／writable gate；失敗不得修改 memory、EBX 或 flags。
- 其他 ModRM、prefix 與 addressing 維持失敗即關閉。
- 固定雜湊 FD2 從 `0x45DD3` 抵達 `0x45DD6`，record `0x539C2` 只有 status
  從 0 改為 2，其餘五 bytes 不變。

驗收包含 descriptor-backed byte write、register 不變及固定 record memory diff。

2026-09-06：上述單元測試與固定雜湊 record memory diff 通過，抵達 `0x45DD6`。
