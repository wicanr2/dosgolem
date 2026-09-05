# 035 — 386 DS disp8 dword load 與 register OR

狀態：**CONFORMED**  
日期：2026-09-06  
前置：[`RE 027`](../re/027-fd2-callee-record-consumer.md)

- `8B /r` 新增 DS:`[r32+disp8]` 的 32-bit load；SIB、其他 addressing 與 segment
  override 未由本切片授權。
- `0B /r` 本切片只支援 register-direct 32-bit OR，寫回目的 register，更新邏輯
  flags；`66 0B` 與 memory form 維持失敗即關閉。
- 固定雜湊 FD2 從 `0x45DC4` 抵達 `0x45DCF`，EBX=`0x539C2`、
  EAX=`0x3CBCC`、CF=0、ZF=0。

驗收包含 descriptor-backed disp8 dword load、非零 OR flags 與固定原檔 pointer gate。

2026-09-06：上述單元測試與固定雜湊 FD2 consumer 測試通過，抵達 `0x45DCF`。
