# 019 — DOS/4GW PSP command-tail byte read

狀態：**CONFORMED**
日期：2026-09-06
前置：[`018`](018-cpu386-command-tail-prelude.md)、[`RE 011`](../re/011-fd2-psp-command-tail-length.md)

- DPMI host 可提供 selector-backed `SegmentRead8`，拒絕未知 selector／offset。
- `8A /r` 只新增 segment override、`mod=1`、無 SIB register base 加 signed disp8；
  讀取 byte 到指定 r8，旗標不變，其他形狀失敗即關閉。
- FD2 無參數啟動時，PSP selector `0x0028` 的 offset `0x80` 回傳 0；其他 PSP
  byte 未經規格登錄仍拒絕。
- 固定雜湊 FD2 從 `0x3CAE2` 執行至 `0x3CAE6`，核對 `ECX=0`。

2026-09-06 全套 dosgolem 測試與固定雜湊整合測試通過；PSP command-tail length
已由 dosgolem host 提供，後續 string scan 指令仍待下一切片。
