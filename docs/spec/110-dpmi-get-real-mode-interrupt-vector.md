# 110 — DPMI 0200h 取得實模式中斷向量

狀態：**CONFORMED**
日期：2026-09-06
前置：[`RE 070`](../re/070-fd2-ail-get-real-mode-vector.md)

- `INT 31h` 只新增 AX=`0200h`；BL 指定 0..255 中斷號。
- 從 dosgolem 的 256 項決定性實模式向量表回傳 CX:DX=segment:offset，
  清除 CF；不修改 DOS `Calls()` 序列計數。
- 無硬體 host 的未設定向量回傳 `0000:0000`，標記為平台近似。
- 其他 DPMI 功能維持失敗即關閉（fail-closed）。
- 固定 FD2 在 `0x3E9B2` 以 BL=8 取得 timer 中斷向量；`0x3E9B4`
  是第一個 consumer。

驗收：單元測試覆蓋非零向量、CF、上半暫存器保留、Calls 不變及未知功能拒絕；
固定雜湊 FD2 必須由 LE entry 自然執行至 `0x3E9B4`，確認預設向量為零且 CF=0。

驗收收據（2026-09-06）：固定原版測試
`TestFD2GetsTimerRealModeVectorWhenProvided` 已由 LE entry 自然執行至
`0x3E9B4`；完整 `go test ./... -count=1` 通過。下一個未支援邊界位於
`0x3E9B7`，不屬於本規格的 DPMI 服務語意。
