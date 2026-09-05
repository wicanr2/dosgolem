# 077 — 386 byte 暫存器 CMP

狀態：**CONFORMED**  
日期：2026-09-06  
前置：[`RE 054`](../re/054-fd2-watcom-delay-init-time.md)

- 支援無前綴 `38 /r` 且 `mod=3` 的 `CMP r/m8,r8`，包含 AH／CH／DH／BH。
- 沿用 byte subtraction 旗標，不修改任一暫存器。
- 記憶體 ModRM 與所有新前綴形狀維持失敗即關閉。
- 固定 FD2 在 `0x3DCC0` 比較 BL 與 DH，離開第二個 delay 校準迴圈。

2026-09-06：高／低 byte 暫存器比較與固定 delay 第二迴圈測試通過。
