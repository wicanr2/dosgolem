# 246 — MOO2 啟動時安裝 BIOS 資料區

狀態：**CONFORMED（受限平台初始化）**
日期：2026-10-01

## 證據與邊界

- **已證實，dosgolem 固定原檔**：官方 1.31 `ORION2.EXE` SHA-256 `4e11be14217b4aafa1839f333bf5eba037f98b0c44e9e4752c96c464c260419f`；本機正版 ZIP SHA-256 `3a28a52f5953ff6d8fc251548940500236752ee19b52b51581e71ec1a3373c2f`。於 `DPMI 0300h → INT 66h` 的 **dosgolem 實模式位址** `1201:0317`，原始位元組 `B8 40 00 8E D8 8B 16 63 00 80 C2 06 EC A8 08`：讀 `0040:0063` 後加 `06h` 並讀埠。BDA 未初始化時會錯讀 `0006h`；記錄見私有 `workplace/moo2-probe-245-in06-window.txt.gz`，SHA-256 `6a7f46d0afebd74ab0de6f729425cc7656380bc2ade14c0cadac065ba7057654`。此為平台位址事實，未推斷遊戲玩法。
- **已證實，既有平台規格**：`docs/spec/186-fd2-platform-gap-continuation.md` 批次 38 與 `internal/machine/le_bios_data.go` 已定義 DOS/4GW BIOS selector `0040`、`0040:0063=D4 03 29 30` 及衝突拒絕。MOO2 啟動接線先前未呼叫該顯式安裝函式。
- **強推論**：在彩色 VGA 設定下 `03D4h+06h=03DAh` 是原檔欲讀的狀態埠；具體等候時間不由本切片宣稱。既有 BDA 的其餘欄位源自通用合成平台預設，未逐欄核對 MOO2 原版。

## 契約與驗收

MOO2 專用 `AttachMachine` 先呼叫既有 `InstallDOS4GWBIOSData`；成功後才接上 DPMI 與保護／實模式共用埠。安裝失敗須回傳錯誤，不覆寫既有 `0400h–04FFh` 或 selector `0040h`，亦不完成其他接線。FD2 與通用 LE 載入器行為不改。這是重用已 CONFORMED 的平台規格，沒有新增遊戲規則。

合成測試驗證 MOO2 接線後 `0040:0063=03D4h` 與衝突無副作用；固定 EXE 全套測試輸出 SHA-256 `7fa4b1acb412bf35c9110a5e94a7349f0dc40a951a396b7b008708b8b5b46f08`。正式接線後以正版資料從 LE entry 重跑，自然越過錯讀 `0006h` 的停點；私有自生收據 `workplace/moo2-probe-246-full-game.txt.gz` SHA-256 `0b3d80dc9aba63f4973089d855384e24080d8e0f529f18f4caca31e0b508667c`，下一停點為 **dosgolem 實模式** `1201:073F` 的 `IN 0225h`。先前直接安裝的可丟棄試跑 `workplace/moo2-probe-246-bda-trial.txt.gz` SHA-256 `ade62e024f0553d23c14d910ea1784651edf0571d07cf98bce75641fbb26b265` 只作診斷。這僅證明平台啟動前進，尚非同狀態玩家畫面對拍。
