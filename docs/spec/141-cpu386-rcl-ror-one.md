# 141 — CPU386 單步 32-bit 暫存器 RCL／ROR

狀態：**CONFORMED**  
日期：2026-09-06  
前置：[`RE 100`](../re/100-fd2-sopen-carry-to-signed-result.md)

## 範圍

- 支援 opcode `D1`、register-direct ModRM 的 `/2 RCL r32,1` 與
  `/1 ROR r32,1`。
- `RCL` 將舊 CF 放入 bit 0，將舊 bit 31 放入 CF；count 1 的 OF 為
  結果 bit 31 XOR 新 CF。
- `ROR` 將舊 bit 0 放入 bit 31 與 CF；count 1 的 OF 為結果 bit 31
  XOR 結果 bit 30。
- 兩者不修改 SF、ZF、AF、PF。

## 失敗即關閉

- operand-size prefix、segment／repeat prefix、記憶體 ModRM，以及 `/0`、
  `/3` 至 `/7` 均拒絕，不擴張成未經需求驗證的完整 shift group。

## 驗收

- CPU 單元測試分別驗證 CF=0 與 CF=1 的兩指令串接結果、CF／OF，以及未受
  影響的狀態旗標。
- 測試拒絕未授權的記憶體與群組形狀。
- 固定原版由 LE entry 自然執行，越過 `0x3CD75`／`0x3CD77`，並由下一個
  真實阻塞點證明不再停在 opcode `D1`。

本規格不宣告 DOS read／seek／close 或一般 CPU rotate 群組已完成。

驗收收據（2026-09-06）：`TestRegisterRCLAndROR32ByOne` 通過成功／錯誤
carry 串接、CF／OF 與未受影響旗標，並拒絕未授權形狀；
`TestFD2ConsumesDOSOpenCarry` 由固定原版 LE entry 自然越過
`0x3CD75`／`0x3CD77` 至 `0x3CD79`。後續有界探針的下一阻塞是
`0x3CD7F` 的 `0F B7 C0`，不再是 opcode `D1`。
