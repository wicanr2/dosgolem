# 024 — DOS/4GW environment selector 載入

狀態：**CONFORMED**
日期：2026-09-06
前置：[`023`](023-cpu386-buffer-stack-finalize.md)、[`RE 016`](../re/016-fd2-environment-selector-load.md)

- segment word read 先詢問 host hook，未命中再依完整 descriptor 做 base／limit／Bus
  驗證；兩者都拒絕才失敗。
- `8E /r` 的 `mod=0,r/m=5` memory-source 形式允許 ES override，從轉換後位址讀取
  selector，再通過目的 segment 的 load validation；其他 override／addressing 拒絕。
- FD2 host 認可 environment selector `0x0030` 載入 DS，但不因此授予一般記憶體存取。
- 固定雜湊 FD2 執行至 `0x3CB12`，核對 `DS=0x30`、`ES=0x160`、`EBP=0`。

2026-09-06 cpu386、DOS host 與固定雜湊 FD2 整合測試通過；執行抵達第一筆
environment block 內容讀取 `0x3CB12`。
