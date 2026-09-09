# 127 — 386 MOV stack+disp32 dword read

狀態：**CONFORMED**
日期：2026-09-06
前置：[`RE 087`](../re/087-fd2-mdi-ini-path-load.md)

- 支援無 prefix 的 `8B /r`、`mod=2`、SIB=`24h`：
  `MOV r32,[esp+sign-extended disp32]`。
- effective address 以讀取指令當下的 ESP 計算，使用 SS，讀取一個 dword；
  目的暫存器可由 ModRM 的 reg 欄位選擇。
- 指令不修改來源記憶體、ESP 或旗標。
- operand16、segment override、其他 SIB 與其他新 ModRM 形狀維持失敗即關閉。

驗收：合成測試覆蓋正／負位移、SS 讀取與狀態不變；固定雜湊 FD2 必須從
LE entry 自然執行 `0x3F33C` 至 `0x3F343`，確認 EDX 等於指令前
`[SS:ESP+0x184]`，且 ESP 與旗標不變。

本規格不授權實作 host 檔案映射或 `fopen`。

驗收收據（2026-09-06）：`TestLoadRegisterFromStackDisp32` 覆蓋正／負位移、
SS 讀取、ESP 與旗標不變；`TestFD2LoadsMDIINIPathArgument` 從固定原版 LE
entry 自然執行至 `0x3F343`，確認 EDX 等於指令前 `[SS:ESP+0x184]`。
