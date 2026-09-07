# 089 — 386 stack SIB 立即 dword 寫入

狀態：**CONFORMED**
日期：2026-09-06
前置：[`RE 059`](../re/059-fd2-ail-dpmi-lock-region.md)

- 擴充無前綴 `C7 /0`，只接受 `mod=0`、SIB
  `scale=0/index=none/base=ESP` 的 `MOV dword ptr [SS:ESP],imm32`。
- SIB index 欄位 4 代表沒有 index，不得當成 ESP index 相加兩次。
- immediate 完整取得後，以 SS descriptor 檢查與寫入；失敗時不修改
  記憶體。其他新 SIB 形狀與前綴失敗即關閉。
- 固定 FD2 在 `0x362A0` 以 `C7 04 24 00060000` 建立 DPMI
  `AX=0600h` 的 Watcom `int386` 輸入結構。

驗收：單元測試覆蓋 SS:ESP 寫入、DS/SS 基址區分與越界失敗；固定雜湊
FD2 必須由 LE entry 自然經過 `0x362A0`。

驗收收據（2026-09-06）：`TestMoveImmediateDwordToStackSIB` 與固定雜湊
`TestFD2CompletesFirstDPMILockWhenProvided` 通過。
