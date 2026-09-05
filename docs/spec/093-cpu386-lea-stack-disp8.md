# 093 — 386 stack disp8 有效位址

狀態：**CONFORMED**
日期：2026-09-06
前置：[`RE 059`](../re/059-fd2-ail-dpmi-lock-region.md)

- 擴充無前綴 opcode `8D`，只接受 `mod=1`、SIB
  `scale=0/index=none/base=ESP` 的 `LEA r32,[ESP+disp8]`。
- 使用有符號 disp8；LEA 不存取記憶體、不檢查 descriptor 且不改旗標。
- 其他 SIB 與前綴形狀失敗即關閉，現有 `8D` 形狀保持不變。
- 固定 FD2 在 `0x362CC` 與 `0x362D1` 取得 Watcom `int386` 的輸出、
  輸入 register 結構位址。

驗收：單元測試覆蓋正與負 disp8；固定雜湊 FD2 必須由 LE entry 自然經過
兩個 LEA。

驗收收據（2026-09-06）：`TestLEAStackDisp8` 與固定雜湊 DPMI 路徑通過。
