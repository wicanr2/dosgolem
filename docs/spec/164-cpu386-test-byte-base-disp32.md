# 164 — CPU386 base＋disp32 byte TEST

狀態：**CONFORMED**  
日期：2026-09-06  
前置：[`RE 111`](../re/111-fd2-ini-trailing-space-scan.md)

- 擴充無前綴 opcode `F6 /0`，接受 `mod=2`、非 SIB 的
  `TEST byte ptr [base+disp32],imm8`。
- disp32 以有號 32 位加入 base；EBP 使用 SS，其餘使用 DS。
- 依 byte AND 結果更新 SF、ZF、PF，清除 CF、OF；AF 依 x86 契約不保證。
  不修改來源記憶體或通用暫存器。
- operand-size、segment／repeat prefix、SIB、非 `/0` 與其他形狀維持
  失敗即關閉（fail-closed）。

驗收：CPU 測試覆蓋零／非零、負位移、EBP／SS、暫存器不變與越界失敗；
固定雜湊 FD2 自然執行 `0x3F369`，確認以 `byte_51840[EAX] & 2` 設定旗標並
抵達 `0x3F370`。

驗收收據（2026-09-06）：`TestByteTESTAtBaseDisp32` 覆蓋零／非零、負位移、
EBP／SS、暫存器與記憶體不變及越界失敗；`TestFD2TestsINICharacterClass`
由固定原版 LE entry 自然執行 `0x3F369`，確認依 `byte_51840[EAX] & 2`
設定 ZF、EAX 不變並抵達 `0x3F370`。後續一次性有界探針已刪除，下一阻塞
移至 `0x3F374` 的 `88 B4`。
