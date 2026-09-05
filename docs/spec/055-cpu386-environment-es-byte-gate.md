# 055 — 386 environment ES byte 閘門

狀態：**CONFORMED**  
日期：2026-09-06  
前置：[`RE 047`](../re/047-fd2-third-callback-environment-first-byte.md)、[`spec 025`](025-dos4gw-environment-block.md)

- FD2 host 對既有 environment selector `0x0030` 增加 ES 載入目的地；backing、
  邊界及不可寫契約不變，不建立一般 flat descriptor。
- `8B /r` 的 `mod=1, r/m=EBP` 32 位元讀取使用預設 SS 描述子。
- `ES: 80 /7` 支援精確 `mod=0, r/m=EAX` byte immediate compare，經 ES 的
  segment backing 讀取且不寫回；其他新前綴／定址形式維持失敗即關閉。
- 固定 FD2 由 LE 入口自然抵達 `0x4CC41`，驗證 ES=`0x30`、EAX=0、ZF=1。

驗收包含 SS 預設 segment、ES byte compare 與 host gate 單元測試、固定雜湊
整合測試及探針收據。

2026-09-06：三層單元測試、固定雜湊整合測試、探針及全套 Go 回歸通過；
探針於 480 步抵達 `0x4CC41`，ES=`0x30`、EAX=0、ZF=1。
