# 020 — 386 CLD 與 REPE SCASB

狀態：**CONFORMED**
日期：2026-09-06
前置：[`019`](019-dos4gw-psp-command-tail.md)、[`RE 012`](../re/012-fd2-command-tail-space-scan.md)

- `FC` 清除 DF，不改其他旗標。
- `F3` 只作為 `AE` 的 REPE prefix；重複 prefix 或搭配其他 opcode 失敗即關閉。
- `SCASB` 比較 `AL-ES:[EDI]`，依 DF 更新 EDI，使用既有 byte subtraction flags。
- REPE 以 ECX 為上限；ECX=0 時不得讀記憶體、修改 EDI 或 arithmetic flags；每次
  比較後遞減 ECX，ZF=0 或 ECX=0 時結束。
- segment byte read 必須通過 host hook 或 descriptor；未知存取失敗。

驗收包含向前／向後、零計數與拒絕前綴；固定雜湊 FD2 從 `0x3CAE6` 執行至
`0x3CAEB`，核對 `AL=0x20`、`ECX=0`、`EDI=0x81`、DF=0。

2026-09-06 單元測試以非空內容驗證實際重複與停止條件，固定雜湊 FD2 整合測試
驗證零計數路徑；兩者均通過。
