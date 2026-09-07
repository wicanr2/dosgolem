# 014 — DOS/4GW flat descriptor 最小模型

狀態：**CONFORMED**
日期：2026-09-06
前置：[`013`](013-fd2-load-es-selector.md)、[`RE 006`](../re/006-fd2-flat-data-selector-consumer.md)

## 目的

建立可由其他 DOS/4GW 程式沿用的 protected-mode descriptor 轉換，不以 FD2 位址
特判 segment-memory。每筆 descriptor 保存 base、inclusive limit 與 writable。

## 契約

- DOS／DPMI host 以 selector 註冊 descriptor；CPU 不自動把未知 selector 視為 flat。
- `linear = base + offset` 必須檢查 32-bit overflow、完整存取範圍與 Bus 邊界。
- 寫入必須檢查 writable；違反任一條件即回傳未處理，指令失敗即關閉。
- 既有固定 segment oracle hook 可先處理特殊 cell；拒絕後才查一般 descriptor。
- `8C /r` 新增 `ES:` override、`mod=0,r/m=5` 的 16-bit segment-register 寫入形式；
  其他 segment override／addressing form仍拒絕。
- FD2 啟動服務只註冊已具平台與 operand 證據的 `0x0160` flat writable descriptor。

## 驗收

- 單元測試涵蓋 base 轉換、limit、唯讀、未知 selector 與 `ES:[disp32]` 寫入；
- 固定雜湊 FD2 從 entry 執行越過 `0x3CABA`，核對 `[0x3C9D8]=0x0160`；
- 保存下一個未支援 EIP，不以測試補寫未知 opcode。

2026-09-06 固定雜湊 FD2 已執行至 `0x3CAC1` 並核對
`[0x3C9D8]=0x0160`。下一個停止點為 `0x3CAC7: 66 89 0D ...`，不屬
descriptor 轉換，故本切片已符合規格。
