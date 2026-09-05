# 017 — DOS/4GW selector 載入驗證與 memory-source MOV

狀態：**CONFORMED**
日期：2026-09-06
前置：[`013`](013-fd2-load-es-selector.md)、[`RE 009`](../re/009-fd2-restore-es-selector.md)

## 契約

- CPU 將「selector 可載入」與「descriptor 可做一般位址轉換」分開。
- 已登錄完整 descriptor 的 selector 可載入；DPMI host 亦可用窄 callback 認可
  opaque selector，但這不授予未知 offset 的記憶體存取。
- `8E /r` 的 register-direct 與 `mod=0,r/m=5` memory-source 形式，在修改 segment
  register 前都必須通過可載入驗證；CS destination 與其他 addressing 仍拒絕。
- memory-source 讀取 `disp32` 的 little-endian word，EFLAGS 不變。

## FD2 驗收

啟動 host 只額外認可已有實機 cell 證據的 opaque selector `0x0028`。固定雜湊 FD2
從 `0x3CACF` 執行後必須得到 `ES=0x0028`、`EIP=0x3CAD5`，且不得替 `0x0028`
建立未證實的 base／limit。

2026-09-06 cpu386 單元測試與固定雜湊 FD2 整合測試通過；執行已抵達
`0x3CAD5`，`ES=0x0028`，且 descriptor table 中沒有偽造 `0x0028` base／limit。
