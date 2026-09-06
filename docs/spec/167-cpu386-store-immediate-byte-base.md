# 167 — CPU386 以基址間接寫入 immediate byte

狀態：**CONFORMED**  
日期：2026-09-06  
前置：[`RE 113`](../re/113-fd2-ini-key-value-split.md)

- 擴充無前綴 opcode `C6 /0`，接受 `mod=0`、非 ESP／EBP 的
  `MOV byte ptr [r32],imm8`；資料 segment 使用 DS。
- immediate 完整取自下一 byte；目的 descriptor 不可寫或超界時不得修改
  記憶體。MOV 不修改基址、其他暫存器或旗標。
- ESP SIB、EBP 絕對 disp32、非 `/0`、operand-size、segment／repeat prefix
  與其他形狀維持失敗即關閉（fail-closed）。

驗收：CPU 測試覆蓋非 EBX 基址、旗標不變及唯讀失敗，並保留既有 EBX 路徑；
固定雜湊 FD2 自然執行 `0x3F449`，確認 `[EAX]` 寫成 NUL 並抵達 `0x3F44C`。

驗收收據（2026-09-06）：`TestMoveImmediateByteToDSRegisterMemory` 覆蓋既有
EBX、新增 EAX 基址、旗標不變與唯讀失敗；`TestFD2SplitsINIKeyAndValue`
由固定原版 LE entry 自然執行 `0x3F449`，確認 `[EAX]` 寫成 NUL 並抵達
`0x3F44C`。後續一次性有界探針已刪除，下一阻塞移至 `0x3F44C` 的
`80 3B`。
