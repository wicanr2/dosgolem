# 126 — 386 LEA stack+disp32

狀態：**CONFORMED**
日期：2026-09-06
前置：[`RE 086`](../re/086-fd2-mdi-ini-settings-buffer.md)

- 支援無 prefix 的 `8D /r`、`mod=2`、SIB=`24h`：
  `LEA r32,[esp+sign-extended disp32]`。
- 只計算 32 位元模數有效位址，不讀取記憶體、不修改旗標。
- operand16、segment、repeat、其他 SIB 與其他 ModRM 維持失敗即關閉。

驗收：合成測試覆蓋正／負位移與狀態不變；固定雜湊 FD2 必須自然執行
`0x3F327` 至 `0x3F32E`，確認 EAX 等於指令前 ESP+`0x108`。

驗收收據（2026-09-06）：`TestLEAStackDisp32` 覆蓋正／負位移及狀態不變；
`TestFD2AddressesMDIINISettingsBuffer` 由固定原版 LE entry 自然執行至
`0x3F32E`，確認 EAX=指令前 ESP+`0x108`，且 ESP 不變。
