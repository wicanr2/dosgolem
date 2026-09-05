# 090 — 386 暫存器寫入 stack disp8

狀態：**CONFORMED**
日期：2026-09-06
前置：[`RE 059`](../re/059-fd2-ail-dpmi-lock-region.md)

- 擴充無前綴 opcode `89`，只接受 `mod=1`、SIB
  `scale=0/index=none/base=ESP` 的 `MOV [SS:ESP+disp8],r32`。
- 使用有符號 disp8 與 SS descriptor；寫入失敗不修改記憶體或來源暫存器。
- 其他 SIB 與前綴形狀失即關閉，現有 `89` 形狀保持不變。
- 固定 FD2 在 `0x362AC`、`0x362BE` 與 `0x362C8` 建立 DPMI
  BX:CX 與 SI:DI 輸入欄位。

驗收：單元測試覆蓋正、負 disp8 與越界失敗；固定雜湊 FD2 必須由
LE entry 自然經過上述寫入。

驗收收據（2026-09-06）：`TestStoreRegisterToStackDisp8` 與固定雜湊 DPMI 路徑通過。
