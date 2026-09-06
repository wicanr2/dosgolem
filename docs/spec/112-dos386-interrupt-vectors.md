# 112 — 32 位元 DOS 中斷向量服務

狀態：**CONFORMED**
日期：2026-09-06
前置：[`RE 072`](../re/072-fd2-ail-dos-interrupt-vectors.md)

- `INT 21h/AH=35h` 以 AL 為中斷號，從 256 項 protected-mode DOS
  向量表回傳 ES:EBX=selector:offset，清除 CF。
- `INT 21h/AH=25h` 以 AL 為中斷號，把 DS:EDX=selector:offset 寫入同表，
  清除 CF。
- 這張表與 DPMI `0200h` 實模式向量表分離；未設定項目為零。
- 兩項服務不改變固定啟動序列的 `Calls()`；其他未列 DOS 服務仍失敗即關閉。
- 不模擬 BIOS/PIT 或 handler 的硬體執行時序。

驗收：單元測試覆蓋非零向量設定後讀回、CF、上半暫存器、兩表隔離與
`Calls()` 不變；固定雜湊 FD2 必須由 LE entry 自然執行 `AH=35h` 並保存
舊向量，再以 `AH=25h` 安裝 `CS:sub_3E73E`。

驗收收據（2026-09-06）：`TestFD2StartupDOSInterruptVectors` 驗證非零向量
讀寫、CF、兩表隔離與 Calls 不變；`TestFD2ReplacesTimerDOSVector` 由固定
原版 LE entry 自然執行至 `0x3E9EF`，確認舊向量保存為零，且向量 8 已改為
`CS:0x3E73E`。
