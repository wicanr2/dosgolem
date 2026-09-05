# 085 — 386 暫存器與絕對 dword 比較

狀態：**CONFORMED**
日期：2026-09-06
前置：[`RE 058`](../re/058-fd2-watcom-stack-probe.md)

- 擴充無前綴 `3B /r`、`mod=0 r/m=5` 的 `CMP r32,[DS:disp32]`。
- 從 DS descriptor 讀取 dword，以暫存器為左運算元更新完整 32 位
  subtraction 旗標，不修改任一運算元。
- 讀取失敗、其他記憶體尋址與前綴失敗即關閉；現有 register/register
  形狀保持不變。
- 固定 FD2 在 `0x36CF2` 以 `3B 05 14280500` 將 EAX stack distance 與
  `dword_52814` 比較。

驗收：單元測試覆蓋相等與越界不修改運算元；固定雜湊 FD2 必須
由 LE entry 自然經過 `0x36CF2`。

驗收收據（2026-09-06）：`TestCompareRegisterAbsoluteDword` 與固定雜湊 stack probe
實驗通過。
