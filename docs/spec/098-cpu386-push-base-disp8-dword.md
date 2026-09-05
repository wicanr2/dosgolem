# 098 — 386 基址加 disp8 dword 壓棧

狀態：**CONFORMED**
日期：2026-09-06
前置：[`RE 062`](../re/062-fd2-watcom-getenv-argument.md)

- 擴充無前綴 opcode `FF /6`，接受 `mod=1`、非 SIB 的
  `PUSH dword ptr [r32+disp8]`。
- 位移必須作有符號 8 位擴展；基址為 EBP 時使用 SS，其餘基址使用 DS。
- 先完整讀取來源，再檢查 ESP 下溢並寫入 SS:ESP。來源讀取或堆疊寫入失敗時，
  ESP 與堆疊內容不得修改。
- SIB、disp32、16 位 operand、segment override 與其他 `FF` 形狀維持
  失敗即關閉（fail-closed）。
- 固定 FD2 在 `0x3F151` 以 `FF 75 08` 將 `getenv` 第一個參數壓棧，
  交給 `0x3F154` 的 `strlen`。

驗收：單元測試覆蓋 EBP/SS 成功路徑及來源越界不改 ESP；固定雜湊 FD2
必須由 LE entry 自然執行到 `0x3F154`，並確認新堆疊頂值等於 `[EBP+8]`。

驗收收據（2026-09-06）：`TestPushBaseDisp8Dword` 與
`TestFD2PassesGetenvArgumentWhenProvided` 通過；後者由 LE entry 自然進入
Watcom `getenv` 並停在 `0x3F154`，確認壓入值等於 `[EBP+8]`。同一容器內
`go test ./... -count=1` 全數通過。
