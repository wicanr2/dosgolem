# 237 — MOO2 VBE 顯示起點歸零

狀態：**CONFORMED（受限平台服務，非畫面對拍）**
日期：2026-10-01
範圍：隔離 dosgolem 的 `NewMOO2StartupDOS` 設定、保護模式直接 `INT 10h/AX=4F07h`，僅固定原檔首筆 `BL=0、CX=0、DX=0`。不實作通用平移、垂直同步、掃描線或畫面呈現。

**非零起點已由規格 266 接通**：[266-moo2-vbe-display-start.md](266-moo2-vbe-display-start.md) 延伸已附掛且啟用模式 0101h 的垂直起點、讀回與索引／RGB 消費。以下是歷史全零返回收據；未啟用／未附掛仍保留其精確全零邊界，不能從此歷史返回宣稱像素對拍。

## RE／平台證據

- **已證實，固定原版輔助執行**：官方 1.31 `ORION2.EXE` SHA-256 `4e11be14217b4aafa1839f333bf5eba037f98b0c44e9e4752c96c464c260419f`、正版 ZIP 根層 417 檔與 `MOX.SET` SHA-256 `bfd6855a41760b31156b96114b5b33c88f442ab8f8aae020c1740b3b486a3a80`；DOSBox-X 2026.07.02 SDL2 heavy debugger 映像 ID `sha256:659e8abbf93646f59a1586341769bd4b8f3cd1c707859d7a5de4c56e4672b582`。版控 `apps/moo2/tools/startup_probe_131.py --video-display-4f07` 於 **DOSBox-X CS:EIP** `0180:003802B2 → 0180:003802B4` 擷取 `EAX=4F07h、EBX=ECX=EDX=0`；返回 `EAX=004Fh`，其他暫存器、段及 EFLAGS=`0216h` 保持。私有 `workplace/dosbox-moo2/video-display-4f07-registers.json` SHA-256 `427b0a884df5aff6f2d6d6b53d2ab0febb4c1816b0500416122d351d72b252e5`，終端 SHA-256 `81f95858fc83c004d2ea71f86588ecae6a69bc17e71c9511f9bc55ce61b8aea9`。
- **已證實，caller 可見消費**：同次返回後，caller 在 **DOSBox-X CS:EIP** `0180:003801B8` 將返回 EAX=`004Fh` 寫入回傳 record；後續轉入上層流程。私有 `video-display-4f07-caller-logcpu.txt` SHA-256 `d8a887fd435ba501a4c4a62975a7a51b4af54ad2b6a081fab7c2a7a698625e`。尚無當次實際 VRAM 位元或畫面差分。
- **已證實，平台契約**：[VESA VBE Core Functions 2.0 Rev 1.1，第 11–12 頁](https://www.phatcode.net/res/221/files/vbe20.pdf)的 `4F07h`、`BL=00h` 表示設定顯示起點，CX／DX 是座標；成功值 AL=`4Fh`、AH=`0`。零座標的玩家可見輸出仍須獨立查證。
- **已證實，合成停點**：規格 236 的 `4F00h` 受限服務後，dosgolem 第 189,937 步於**高位重定位 LE 線性位址** `0x24C2B2` 遇 `CD 10`，輸入 EAX=`4F07h`、EBX=ECX=EDX=`0`，先前 `4F00h` 的 `RealModeLast` 已返回，未實作 DPMI 計數為空。私有 `workplace/moo2-probe-236-full-game.txt` SHA-256 `db6c5b63cbcf9297c0e9a8c5c27a74d77e6895dbac435773b955d2e4b162f444`。兩側完整狀態仍不同。

## 擬議受限契約

只在 MOO2 啟動設定且 `EAX=00004F07h、EBX=ECX=EDX=0` 時接受直接 `INT 10h`；記錄合成顯示起點 `(0,0)`，回 `EAX=0000004Fh`，其餘暫存器、段、EFLAGS 保持。其他座標、BL 值、高位雜訊或一般 FD2 設定一律拒絕，不以記錄座標冒充 VRAM／顯示器畫面已更新。這是公開 VBE 契約與固定 DOSBox-X 回傳支撐的**平台規格近似**。

## 驗收與邊界

1. 合成測試：精確零輸入回傳、座標記錄、非目的暫存器與 EFLAGS 不變，未知形狀失敗即關閉。
2. 固定 1.31 EXE／正版資料從高位 LE entry 自然抵達第 189,937 步，單步越過此服務並記錄下一自然結果；含原檔全套 Go 測試通過。
3. 遊戲仍需 VBE 模式資訊、模式設定、顯示記憶體與 GUI 路徑驗收；此規格只處理一筆零座標請求，不能列為畫面或玩法對拍。

## READY 證據審查

已核對 `FD2StartupDOS.Handle` 的 `INT 10h` 分支：目前僅在 `moo2Profile` 下接受精確 EAX=`3`、記錄模式 03h 並保留全部暫存器與旗標；一般 FD2 呼叫拒絕。於同一明示 MOO2 分支加入精確 `4F07h`／全零參數、記錄獨立的合成顯示起點，不須修改一般服務或資料模型。既有 `TestMOO2ProtectedVideoMode03` 可保證模式 03h 不受影響；新增測試直接核對零輸入、非零拒絕、返回與狀態。原版暫存器、consumer 與 VBE 2.0 契約已足以決定此一請求；真正像素輸出仍列未知，故可進入受限實作。

## 實作與驗收

- `internal/machine/le_startup.go` 只在 MOO2 設定接受精確 `EAX=00004F07h、EBX=ECX=EDX=0`，記錄合成顯示起點 `(0,0)`，回 `EAX=0000004Fh`；其他形狀及一般 FD2 設定拒絕。`internal/machine/le_startup_test.go` 核對成功、狀態不變、非零參數與 FD2 拒絕。
- 官方 1.31 EXE 的全套 `go test -buildvcs=false ./... -count=1` 通過；私有 `workplace/full-test-237.txt` SHA-256 `c6d7d07eed001320c387bbd1258154756a7335e1ebdbceb4e96879032268a1e6`。
- 原檔及 417 份正版資料從高位 LE 入口自然重跑後，dosgolem 通過本服務，於第 190,214 步停在下一筆 DPMI `0300h → INT 10h/AX=4F01h`。私有 `workplace/moo2-probe-237-full-game.txt` SHA-256 `a95ad7d1ea617396f8a4276bf1984a0a493f8c756667a20bdbb74c79c167d148`。此收據只證明受限平台服務與下一停點，尚無可見畫面或原版同狀態對拍。
