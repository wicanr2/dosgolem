# 028 — 386 protected-mode POP DS

狀態：**CONFORMED**  
日期：2026-09-06  
前置：[`017`](017-dos4gw-selector-load-validation.md)、[`RE 020`](../re/020-fd2-restore-flat-ds.md)

- 無 operand-size override 的 `1F` 從 SS:ESP 讀取 32-bit stack cell，以低 16 位作
  DS selector；只有 descriptor 或 host selector gate 已登錄的 selector 才可載入。
- 讀取或 selector 驗證失敗時，不得修改 DS 或 ESP。
- 成功時 DS 更新為 selector，ESP 增加 4；本切片不支援 `66 1F`。
- 固定雜湊 FD2 應由 `0x3CB3B` 執行至 `0x3CB3C`，DS=`0x160`、ESP=`0x556AC`。

驗收包含合法 selector、未登錄 selector 的失敗即關閉，以及固定雜湊 FD2 整合測試。

2026-09-06：上述單元測試與固定雜湊 FD2 整合測試通過，抵達 `0x3CB3C`。
