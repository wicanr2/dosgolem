# 165 — CPU386 以 SIB＋disp32 寫入暫存器 byte

狀態：**CONFORMED**  
日期：2026-09-06  
前置：[`RE 111`](../re/111-fd2-ini-trailing-space-scan.md)

- 擴充無前綴 opcode `88 /r`，接受 `mod=2,r/m=4` 的
  `MOV byte ptr [base+index*scale+disp32],r8`。
- SIB index 不得為 ESP；disp32 以有號 32 位加入。base 為 ESP／EBP 時使用
  SS，其餘使用 DS；位址算術依 32 位 x86 回繞。
- byte 來源採 AL／CL／DL／BL／AH／CH／DH／BH 映射。目的不可寫或超界時
  記憶體保持不變；MOV 不修改旗標。
- operand-size、segment／repeat prefix、無 index 與其他形狀維持失敗即關閉
  （fail-closed）。

驗收：CPU 測試覆蓋 ESP base、一般 base、scale、負 disp32、high-byte 來源、
旗標不變與唯讀失敗；固定雜湊 FD2 自然執行 `0x3F374`，確認 DH 的零值寫入
`SS:[ESP+ESI+0x118]` 並抵達 `0x3F37B`。

驗收收據（2026-09-06）：`TestStoreRegister8ToSIBDisp32` 覆蓋一般與 ESP
base、scale、負 disp32、AH／DH 來源、旗標不變及唯讀失敗；
`TestFD2TrimsINITrailingCharacter` 由固定原版 LE entry 自然執行
`0x3F374`，確認零值寫入 `SS:[ESP+ESI+0x118]` 並抵達 `0x3F37B`。
後續一次性有界探針已刪除，裁切與下一段 leading-space scan 已通過；下一
阻塞移至 `0x3F3F0` 的 `89 84`。
