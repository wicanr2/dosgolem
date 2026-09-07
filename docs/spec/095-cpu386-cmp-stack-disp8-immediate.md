# 095 — 386 stack disp8 與 sign-extended imm8 比較

狀態：**CONFORMED**
日期：2026-09-06
前置：[`RE 059`](../re/059-fd2-ail-dpmi-lock-region.md)、
[`spec 094`](094-watcom-int386-dpmi-lock.md)

- 擴充無前綴 `83 /7`，只接受 `mod=1`、SIB
  `scale=0/index=none/base=ESP` 的 `CMP dword ptr [SS:ESP+disp8],imm8`。
- immediate 以有符號擴張為 32 位，只更新 subtraction 旗標，不修改記憶體。
- 讀取失敗、其他 SIB 與前綴形狀失敗即關閉。
- 固定 FD2 在 `0x362E0` 比較 `int386` 輸出 `REGS.CFLAG` 與 0。

驗收：單元測試覆蓋相等、不等與越界；固定雜湊 FD2 必須由 LE entry
自然消費 DPMI hook 的 CFLAG=0。

驗收收據（2026-09-06）：`TestGroup83CompareStackDisp8Dword` 與固定雜湊
DPMI CFLAG 消費路徑通過。
