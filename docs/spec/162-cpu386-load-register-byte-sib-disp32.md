# 162 — CPU386 從 SIB＋disp32 載入 byte

狀態：**CONFORMED**  
日期：2026-09-06  
前置：[`RE 111`](../re/111-fd2-ini-trailing-space-scan.md)

- 擴充無前綴 opcode `8A /r`，接受 `mod=2,r/m=4` 的
  `MOV r8,byte ptr [base+index*scale+disp32]`。
- SIB index 不得為 ESP；disp32 以有號 32 位加入。base 為 ESP／EBP 時使用
  SS，其餘使用 DS。所有 32 位加法依 x86 位址寬度回繞。
- 目的 byte 採完整 AL／CL／DL／BL／AH／CH／DH／BH 映射；來源讀取失敗時
  目的暫存器不變，MOV 不改旗標。
- operand-size、segment／repeat prefix、無 index 與其他 addressing shape
  維持失敗即關閉（fail-closed）。

驗收：CPU 測試覆蓋 ESP base、一般 base、scale、有號負 disp32、high-byte
目的、旗標不變與越界失敗；固定雜湊 FD2 自然執行 `0x3F35B`，確認 AL 等於
`SS:[ESP+ESI+0x118]` 並抵達 `0x3F362`。

驗收收據（2026-09-06）：`TestSIBDisp32ByteRead` 覆蓋一般與 ESP base、
scale、有號負 disp32、AH 目的、旗標不變與越界失敗；
`TestFD2ScansINITrailingCharacter` 由固定原版 LE entry 自然執行
`0x3F35B`，確認 AL 等於 `SS:[ESP+ESI+0x118]` 並抵達 `0x3F362`。
後續一次性有界探針已刪除，下一阻塞移至 `0x3F362` 的 `FE C0`。
