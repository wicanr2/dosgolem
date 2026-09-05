# 022 — DOS/4GW PSP／data selector 交換與 near JZ

狀態：**CONFORMED**
日期：2026-09-06
前置：[`021`](021-cpu386-lea-disp8.md)、[`RE 014`](../re/014-fd2-psp-data-selector-swap.md)

- FD2 host 認可 PSP selector `0x0028` 載入 DS 或 ES；其他未登錄 destination
  仍拒絕，且不替 PSP selector建立一般 descriptor。
- cpu386 的 `0F 84 cd` 支援 32-bit signed near displacement；ZF=1 時跳轉，否則
  落下。與既有 `0F 85` 共用解碼，operand-size override及其他 extended opcode拒絕。
- 固定雜湊無參數 FD2 必須從 `0x3CAF2` 執行至 `0x3CB01`，核對
  `DS=0x28`、`ES=0x160`、`EDX=0x160`，並證明未執行被跳過的 copy path。

2026-09-06 cpu386、DOS host 與固定雜湊 FD2 整合測試通過；執行抵達
`0x3CB01`，零長度路徑未進入 `INC ECX`／`REP MOVSB`。
