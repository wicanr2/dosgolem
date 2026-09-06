# 160 — CPU386 以基址間接寫入暫存器 byte

狀態：**CONFORMED**  
日期：2026-09-06  
前置：[`RE 110`](../re/110-fd2-bounded-line-reader-byte-store.md)

- 擴充無前綴 opcode `88 /r`，接受 `mod=0`、非 ESP／EBP 的
  `MOV byte ptr [r32],r8`；資料 segment 使用 DS。
- byte 來源依 ModRM reg 欄位採 x86 AL／CL／DL／BL／AH／CH／DH／BH 映射，
  目的位址由 r/m 基址暫存器提供。
- descriptor 不可寫或超界時不得修改記憶體。MOV 不修改旗標。
- EBP 絕對 disp32、ESP SIB、operand-size、segment／repeat prefix 與其他形狀
  維持失敗即關閉（fail-closed）。

驗收：CPU 測試覆蓋 AL、high-byte 暫存器、旗標不變與唯讀失敗；固定雜湊
FD2 自然執行 `0x46C84`，確認目前 `MDI.INI` 字元寫入 `[EBX]` 並抵達
`0x46C86`。

驗收收據（2026-09-06）：`TestStoreRegister8ToBase` 覆蓋 high-byte 來源、
旗標不變與唯讀失敗；`TestFD2StoresLineReaderByte` 由固定原版 LE entry
自然執行 `0x46C84`，確認字元寫入目前目的 cursor 並抵達 `0x46C86`。
後續一次性有界探針已刪除，迴圈開始讀取下一字元；下一阻塞移至 `fgetc`
的 `0x3D9FA`（`8A 00`）。
