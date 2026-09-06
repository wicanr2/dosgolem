# 158 — CPU386 基址間接 dword 遞增

狀態：**CONFORMED**  
日期：2026-09-06  
前置：[`RE 109`](../re/109-fd2-filbuf-consumes-first-byte.md)

- 擴充無前綴 opcode `FF /0`，只接受 `mod=0`、非 ESP／EBP 的
  `INC dword ptr [r32]`；資料 segment 使用 DS。
- 讀取、加一、寫回，並依 32 位 INC 規則更新 OF、SF、ZF、AF、PF；CF 必須
  保留原值。
- descriptor 不可讀／寫或超界時不得部分寫入。SIB、disp32、operand-size、
  segment override 及其他形狀維持失敗即關閉（fail-closed）。

驗收：CPU 測試覆蓋一般結果、溢位、CF 保留與唯讀失敗；固定雜湊 FD2 自然
執行 `0x3DA58`，確認 `[EBX]` 增加一並抵達 `0x3DA5A`。

驗收收據（2026-09-06）：`TestIncrementBaseDword` 驗證有號溢位旗標、CF
保留與唯讀失敗；`TestFD2AdvancesFilbufPointer` 由固定原版 LE entry 自然
執行 `0x3DA58`，確認 `[EBX]` 增加一並抵達 `0x3DA5A`。後續一次性有界
探針已刪除，下一阻塞是 `0x3DA5A` 的 `8B 03`。
