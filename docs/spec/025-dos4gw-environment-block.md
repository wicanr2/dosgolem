# 025 — DOS/4GW environment block 最小 backing

狀態：**CONFORMED**
日期：2026-09-06
前置：[`024`](024-dos4gw-environment-selector-load.md)、[`RE 017`](../re/017-fd2-environment-block-first-read.md)

- FD2 DOS host 為 selector `0x0030` 提供不可寫、具邊界的 byte backing。
- 預設 block 為雙 NUL、word `1`、`FD2.EXE` 加 NUL；這是合法最小啟動環境，
  不宣稱重現使用者 PATH／COMSPEC。
- segment 32-bit read 由四個受檢查 byte read 組合，不得越過 backing。
- `8B /r` 新增預設 DS、`mod=0,r/m=6`（`[ESI]`）的 dword read；其他 memory
  addressing 仍拒絕。
- 固定雜湊 FD2 執行至 `0x3CB14`，核對首 dword `0x00010000`。

2026-09-06 cpu386、DOS host 與固定雜湊 FD2 整合測試通過；最小 environment
backing 的邊界與首 dword 均已驗證。
