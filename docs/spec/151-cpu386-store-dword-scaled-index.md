# 151 — CPU386 base＋scaled-index dword store

狀態：**CONFORMED**  
日期：2026-09-06  
前置：[`RE 103`](../re/103-fd2-setiomode-writes-file-record.md)

- 擴充 opcode `89` 的 ModRM `14h`、SIB `98h`：
  `MOV [EAX+EBX*4],EDX`。
- 在任何寫入前，以原 EAX base 與 EBX index 計算有效位址；將 EDX 的
  little-endian dword 經 DS selector 寫入，不修改 registers 或 flags。
- 只接受這個固定 SIB；其他 SIB 維持既有失敗即關閉契約。DS descriptor
  不可寫或超界時不得部分寫入。

驗收：CPU 單元測試驗證 scale 4、完整 dword 寫入、來源／索引／旗標不變、
唯讀與越界失敗；固定原版自然執行 `0x463B6`，確認
`table_base + handle*4` 等於 EDX 並抵達 `0x463B9`。

本規格不宣告 FILE record 其他欄位的完整語意。

驗收收據（2026-09-06）：`TestStoreDwordToBaseScaledIndex` 驗證 scale 4、
dword 寫入、來源／索引／旗標不變與唯讀失敗；
`TestFD2WritesOpenedFileRecord` 由固定原版 LE entry 自然執行 `0x463B6`，
確認 `table_base + handle*4` 等於 EDX 並抵達 `0x463B9`。後續有界探針又
自然前進 89 步，下一阻塞移至 `0x46C6E` 的 opcode `7E`。
