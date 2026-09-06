# 156 — CPU386 基址間接 dword 壓棧

狀態：**CONFORMED**  
日期：2026-09-06  
前置：[`RE 107`](../re/107-fd2-fill-buffer-qread-arguments.md)

- 擴充無前綴 opcode `FF /6`，只接受 `mod=0`、`r/m` 為一般基址暫存器的
  `PUSH dword ptr [r32]`。
- EBP 的 `mod=0,r/m=5` 是絕對 disp32 編碼，已由既有路徑處理；ESP 的
  `r/m=4` 是 SIB，仍維持失敗即關閉（fail-closed）。其餘基址使用 DS。
- 必須先完整讀取來源，再檢查 ESP 下溢並寫入 SS:ESP。來源讀取或堆疊寫入
  失敗時，ESP 與堆疊內容不得修改。
- operand-size、segment／repeat prefix、SIB 與其他 `FF` 形狀不在本切片。

驗收：CPU 單元測試覆蓋成功、來源越界與堆疊寫入失敗；固定雜湊 FD2 必須
由 LE entry 自然抵達並執行 `0x3DAE1`，確認新堆疊頂值等於執行前 `[EBX]`，
且 EIP 抵達 `0x3DAE3`。

驗收收據（2026-09-06）：`TestPushBaseDword` 通過成功、來源越界與唯讀
堆疊失敗案例；`TestFD2PushesFillBufferPointer` 由固定原版 LE entry 自然
執行 `0x3DAE1`，確認壓入值等於 `[EBX]` 並抵達 `0x3DAE3`。後續一次性
有界探針已刪除，下一阻塞是 `__qread` 內 `0x3D9A2` 的 `INT 21h/AH=3Fh`。
