# 155 — CPU386 OR byte base＋disp8

狀態：**CONFORMED**  
日期：2026-09-06  
前置：[`RE 106`](../re/106-fd2-ioalloc-buffer-owned-flag.md)

- 擴充 opcode `80 /1`，支援非 SIB 的 `OR byte ptr [base+disp8],imm8`。
- disp8 有號延伸；EBP base 使用 SS，其餘 base 使用 DS。讀取 byte、OR、寫回，
  並以既有 `setLogicFlags8` 更新 CF／PF／AF／ZF／SF／OF。
- operand-size、segment／repeat prefix、ESP SIB 與其他 group 維持失敗即關閉。
- descriptor 不可讀／寫或超界時不得部分寫入。

驗收：CPU 測試驗證有號 disp8、結果、旗標與唯讀失敗；固定原版自然執行
`__ioalloc` 的 `0x3D97D`，確認 FILE record `[ebx+0x0C]` 設上 bit `0x08`
並抵達 `0x3D981`。

本規格不宣告 `__filbuf` 的 DOS read 已完成。

驗收收據（2026-09-06）：`TestByteORAtBaseDisp8` 驗證有號 disp8、OR
結果、旗標與唯讀失敗；`TestFD2MarksAllocatedIOBuffer` 由固定原版 LE entry
自然執行 `0x3D97D`，確認 `[ebx+0x0C]` 設上 bit `0x08` 並抵達
`0x3D981`。後續有界探針返回 `__filbuf`，下一阻塞移至 `0x3DAE1` 的
`FF 33`。
