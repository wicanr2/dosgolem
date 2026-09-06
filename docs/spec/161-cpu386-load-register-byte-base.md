# 161 — CPU386 從基址間接載入 byte

狀態：**CONFORMED**  
日期：2026-09-06  
前置：[`RE 105`](../re/105-fd2-fgetc-buffer-count.md)

- 擴充無前綴 opcode `8A /r`，接受 `mod=0`、非 ESP／EBP 的
  `MOV r8,byte ptr [r32]`；資料 segment 使用 DS。
- byte 目的依 ModRM reg 欄位採 AL／CL／DL／BL／AH／CH／DH／BH 映射，來源
  位址由 r/m 基址暫存器提供；只替換目的 byte，其餘暫存器位元保持不變。
- 來源讀取失敗時目的暫存器保持不變。MOV 不修改旗標。
- EBP 絕對 disp32、ESP SIB、operand-size、segment／repeat prefix 與其他形狀
  維持失敗即關閉（fail-closed）。

驗收：CPU 測試覆蓋 high-byte 目的、旗標不變與越界失敗；固定雜湊 FD2 自然
執行 `0x3D9FA`，確認 AL 等於目前 buffer byte 並抵達 `0x3D9FC`。

驗收收據（2026-09-06）：`TestDSBaseByteRead` 覆蓋 AH 目的、旗標不變與
越界失敗；`TestFD2LoadsNextBufferedCharacter` 由固定原版 LE entry 自然
執行 `0x3D9FA`，確認 AL 等於目前 buffer byte、其餘 EAX 位元不變，並抵達
`0x3D9FC`。後續一次性有界探針已刪除，原版完成目前行讀取並回到 parser；
下一阻塞移至 `0x3F35B` 的 `8A 84`。
