# 051 — 386 PUSH／POP FS

狀態：**CONFORMED**  
日期：2026-09-06  
前置：[`RE 043`](../re/043-fd2-third-callback-push-fs.md)

- 支援無前綴 `0F A0`（`PUSH FS`）：ESP 減 4，以 SS 描述子寫入零擴充的 FS
  selector；下溢、界限或寫入失敗必須失敗即關閉。
- 支援無前綴 `0F A1`（`POP FS`）：以 SS 描述子讀取 32 位元堆疊項目，低 16
  位元須通過 segment load gate 後載入 FS，ESP 加 4。
- operand-size override、其他前綴與其他 extended opcode 不由本切片授權。
- 固定 FD2 從 LE 入口自然抵達 `0x4CC03`；驗證 ESP=`0x55684`，堆疊項目低
  16 位元等於進入前 FS selector。

驗收包含獨立 push/pop round-trip 單元測試、固定雜湊整合測試與探針收據。

2026-09-06：單元測試、固定雜湊整合測試、探針與全套 Go 回歸通過；探針於
467 步抵達 `0x4CC03`，ESP=`0x55684`，保存的 selector 與 FS 相符。
