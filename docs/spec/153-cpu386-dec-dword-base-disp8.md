# 153 — CPU386 DEC dword base＋disp8

狀態：**CONFORMED**  
日期：2026-09-06  
前置：[`RE 105`](../re/105-fd2-fgetc-buffer-count.md)

- 擴充 opcode `FF /1`，支援非 SIB 的 `DEC dword ptr [base+disp8]`。
- disp8 有號延伸；EBP base 使用 SS，其餘 base 使用 DS。讀取 dword、減一、
  完整寫回，並依既有 `sub32` 更新 PF／AF／ZF／SF／OF，但必須保留原 CF。
- operand-size、segment／repeat prefix、ESP SIB 與其他 group 維持失敗即關閉。
- descriptor 不可讀／寫或超界時不得部分寫入。

驗收：CPU 測試驗證結果、ZF／OF、CF 保留、有號 disp8、唯讀失敗；固定原版
自然執行 `fgetc` 的 `0x3D9E4`，確認 `[ebx+4]` 減一並抵達 `0x3D9E7`。

本規格不宣告 `__filbuf` 或 DOS read 已完成。

驗收收據（2026-09-06）：`TestDecrementDwordAtBaseDisp8` 驗證有號 disp8、
ZF／OF、CF 保留及唯讀失敗；`TestFD2DecrementsFgetcBufferCount` 由固定原版
LE entry 自然執行 `0x3D9E4`，確認 `[ebx+4]` 減一並抵達 `0x3D9E7`。
後續有界探針的下一阻塞為 `0x3D9EB` 的 opcode `7D`。
