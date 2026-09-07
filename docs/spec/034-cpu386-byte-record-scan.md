# 034 — 386 byte memory CMP／MOV 與 short JA

狀態：**CONFORMED**  
日期：2026-09-06  
前置：[`RE 026`](../re/026-fd2-first-callee-record-scan.md)

- `38 /r` 本切片只支援 DS:`[r32+disp8]` 與 byte register 的 CMP；不寫回 memory。
- `8A /r` 的既有 `[r32+disp8]` byte load 在沒有 override 時使用 DS；ES override
  既有行為保持。SIB 與其他 addressing 仍失敗即關閉。
- `77 cb` 僅在 CF=0 且 ZF=0 時採 signed rel8；operand-size override 拒絕。
- 固定雜湊 FD2 從 `0x45DB0` 自然掃描八筆 stride-6 records，抵達 `0x45DC4`；
  ESI=`0x539E0`、EBX=`0x539C2`、AL=`0x01`、CF=0、ZF=1。

驗收包含非零 byte compare、taken JA、DS byte load 與固定原檔完整 loop。

2026-09-06：上述單元測試與固定雜湊 FD2 八筆 record 迴圈通過，抵達 `0x45DC4`。
