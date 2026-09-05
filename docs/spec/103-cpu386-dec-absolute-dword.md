# 103 — 386 絕對位址 dword DEC

狀態：**CONFORMED**
日期：2026-09-06
前置：[`RE 065`](../re/065-fd2-ail-wrapper-depth-tail.md)

- 支援無 prefix 的 `FF /1`、`mod=0 r/m=5`（`DEC dword ptr [disp32]`）。
- 來源與目的使用 DS descriptor；必須先成功讀取並確認可寫，才更新記憶體。
- 依 32 位減一更新 OF、SF、ZF、AF、PF，必須保留原 CF。
- 讀取或寫入失敗時，記憶體與旗標保持不變。
- 其他 `FF /1` 尋址、16 位 operand 與 prefix 維持失敗即關閉（fail-closed）。
- 固定 FD2 在 `0x38E1A` 使用 `FF 0D 78 41 05 00` 遞減
  `dword_54178`，consumer 是 `0x38E20` 後的共用返回尾端。

驗收：單元測試覆蓋減至零、CF 保留與唯讀拒絕；固定雜湊 FD2 必須由 LE entry
自然執行至 `0x38E20`，並確認 `dword_54178` 恰好減一。

驗收收據（2026-09-06）：`TestDecrementAbsoluteDword` 與固定原版
`TestFD2LeavesAILWrapperWhenProvided` 通過；後者由 LE entry 自然執行至
`0x38E20`，確認 `dword_54178` 恰好減一。同一原版唯讀掛載下
`go test ./... -count=1` 全數通過。
