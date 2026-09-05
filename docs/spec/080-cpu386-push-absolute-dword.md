# 080 — 386 絕對位址 dword 壓棧

狀態：**CONFORMED**
日期：2026-09-06
前置：[`RE 056`](../re/056-fd2-watcom-cmain.md)

- 支援無前綴 `FF /6`、`mod=0 r/m=5` 的 32 位 `PUSH dword ptr [disp32]`。
- 來源值由 DS 所指平坦 descriptor 的絕對位移讀取；先成功讀取，
  再檢查與寫入 SS:ESP 堆疊。
- 來源讀取、ESP 下溢或堆疊寫入失敗時，ESP 與堆疊內容不得修改。
- 其他 `FF /6` 尋址形狀、前綴與現有 `FF /2` 以外形狀維持
  失敗即關閉（fail-closed）。
- 固定 FD2 在 `0x45D80` 與 `0x45D86` 依序壓入 `argv`、`argc`，並由
  `0x45D8C` 呼叫 `main`。

驗收：單元測試覆蓋成功壓棧與越界失敗不改 ESP；固定雜湊 FD2
必須從 LE entry 自然執行到 `main` 入口，不可直接注入 EIP。

驗收收據（2026-09-06）：`TestPushAbsoluteDword` 與全套 Go 回歸通過；
`TestFD2ReachesMainWhenProvided` 以固定雜湊 FD2 由 LE entry 執行 1094 步後
停在 `0x25BF4` main 入口，堆疊回傳位址為 `0x45D91`、`argc=1`，
堆疊 `argv` 與 Watcom 公開全域一致。
