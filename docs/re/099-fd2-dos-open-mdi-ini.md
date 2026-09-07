# 099 — FD2 DOS AH=3Dh 開啟 MDI.INI

日期：2026-09-06
證據等級：呼叫位址、暫存器、路徑來源與檔案身分為**已證實**

固定原版 `FD2.EXE`（大小 `357074`；MD5
`b97caf2239a27a896069d03549d96e1e`；SHA-256
`222b7d067ad4450eb9c5f6e6bce1797d54bb050417ba39ced6067f8039f28c4f`）
由 LE entry 自然執行至 IDA 線性位址 `0x3CD73` 的 `int 21h`。呼叫前
`AH=3Dh`、`AL=0`、`EDX=0x515B0`；該位址的 NUL 字串來源已由
[`RE 085`](085-fd2-mdi-ini-stack-frame.md) 證實為 `MDI.INI`。

原版 `MDI.INI` 大小 `218`，SHA-256
`d42a9f90e09585eee203db863c664d5cae763e00749325c91b028ee3bcac55d1`。
`sopen` 在 DOS 呼叫後以 CF 判斷成敗，成功時零擴展 AX 並保存為本地
handle；因此服務端必須回傳真正可供後續 read／close 使用的 handle，不能只清 CF。

DOS `AH=3Dh` 的公開介面契約作為平台規格；本 RE 只記錄 FD2 選用的
唯讀模式、路徑指標與 consumer。
