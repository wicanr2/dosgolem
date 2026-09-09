# 041 — 386 DS 絕對位址位元組立即值比較

狀態：**CONFORMED**  
日期：2026-09-06  
前置：[`040`](040-cpu386-record-status-write.md)、[`RE 033`](../re/033-fd2-second-callback-gate.md)

- `80 /7` 新增 ModRM `3D`：讀取 disp32 指定的 DS byte，與 immediate byte 比較，
  更新 subtraction flags 但不寫回。
- 記憶體讀取必須通過 DS 描述子；其他新定址模式不由本切片授權。
- 固定雜湊 FD2 由第一回呼已標記狀態自然重掃，進入第二回呼 `0x460D5`，
  再抵達 `0x460DF`；EAX=`0x460D5`、EBX=`0x539C8`、ESP=`0x55694`、ZF=1。

驗收包含獨立絕對位址位元組 `CMP`、不寫回，以及第二回呼的獨立整合收據。

2026-09-06：上述單元測試與獨立第二回呼固定雜湊整合測試通過，抵達 `0x460DF`。
