# 131 — 386 JG short

狀態：**CONFORMED**
日期：2026-09-06
前置：[`RE 091`](../re/091-fd2-tolower-upper-bound.md)

- 支援無 prefix 的 `7F cb`：當 `ZF=0` 且 `SF=OF` 時，將 sign-extended
  rel8 加到下一指令 EIP；否則順序執行。
- 不修改通用暫存器或旗標。
- 所有 prefix 維持失敗即關閉。

驗收：合成測試覆蓋跳轉、不跳轉與負位移；固定雜湊 FD2 必須自然執行
`0x3D7EF` 的 `jg +3`，並依前一個 `cmp eax,'Z'` 的旗標抵達正確 EIP。

驗收收據（2026-09-06）：`TestJumpGreaterShort` 覆蓋跳轉、不跳轉與負位移；
`TestFD2BranchesPastTolowerConversion` 從固定原版 LE entry 自然執行
`0x3D7EF`，依當時旗標抵達 `0x3D7F4` 且不修改 EAX／旗標。
