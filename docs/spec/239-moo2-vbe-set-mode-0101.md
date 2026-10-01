# 239 — MOO2 設定 VBE 模式 0101h

狀態：**CONFORMED（固定模式設定，非畫面對拍）**
日期：2026-10-01
範圍：隔離 dosgolem 的 MOO2 啟動設定，僅處理保護模式直接 `INT 10h/AX=4F02h、BX=0101h`。不實作通用 VBE 模式切換、VRAM 或顯示畫面。

## RE／平台證據

- **已證實，固定原版 DOSBox-X 輔助執行**：官方 1.31 `ORION2.EXE` SHA-256 `4e11be14217b4aafa1839f333bf5eba037f98b0c44e9e4752c96c464c260419f`；正版 ZIP SHA-256 `3a28a52f5953ff6d8fc251548940500236752ee19b52b51581e71ec1a3373c2f` 根層 417 檔，`MOX.SET` SHA-256 `bfd6855a41760b31156b96114b5b33c88f442ab8f8aae020c1740b3b486a3a80`。DOSBox-X 2026.07.02 SDL2 heavy debugger 映像 ID `sha256:659e8abbf93646f59a1586341769bd4b8f3cd1c707859d7a5de4c56e4672b582`。版控 `apps/moo2/tools/startup_probe_131.py --video-mode-4f02`，私有暫存器收據 `video-mode-4f02-registers.json` SHA-256 `5b6d94309ba8bcc5e9de221ed6b8545c7a5c8f1cb0b86ceae7dac2b9de762dea`；終端 SHA-256 `74496864a917395ed27ae73de730e840d076a809376dbefc36171b608ebf26b3`。
- **已證實，呼叫與返回**：**DOSBox-X CS:EIP** `0180:003802B2 → 0180:003802B4` 的 `CD 10` 前，EAX=`4F02h`、EBX=`0101h`、ECX=EDX=ESI=EDI=`0`，DS=ES=SS=`0188h`、EFLAGS=`0216h`；返回 EAX=`004Fh`，其餘所擷取暫存器、段及 EFLAGS 不變。
- **已證實，caller 可見消費**：同次返回後，**DOSBox-X CS:EIP** `0180:003801B8` 將 EAX=`004Fh` 寫入回傳 record；上層 `0180:00368A5C` 讀取該值。私有 `video-mode-4f02-caller-logcpu.txt` SHA-256 `f86782937cadc66d2988dbbfdeed7fdcb90cc872eb6f42f178a8e9f289259be8`。此 LOG 未擷取畫面或 VRAM 差分。
- **已證實，平台契約**：[VESA VBE Core Functions 2.0 Rev 1.1，Function 02h](https://www.phatcode.net/res/221/files/vbe20.pdf)以 `AX=4F02h`、BX 模式號設定模式；成功為 `AX=004Fh`。原版的 `BX=0101h` 對應前一筆 `4F01h` 查詢的模式。
- **已證實，dosgolem 自生停點**：固定 EXE 與正版資料於**高位重定位 LE 線性位址** `0x24C2B2`、第 190,517 步停在直接 `INT 10h/AX=4F02h`，EBX=`0101h`；私有 `workplace/moo2-probe-238-full-game.txt` SHA-256 `6f5df2e26b65c5935db63cff4751b103182ff75bab09c18314d8a42ed2c70284`。兩側絕對記憶體與完整狀態仍不同。

## 擬議受限契約與 READY 審查

僅在 `NewMOO2StartupDOS` 且精確 EAX=`00004F02h`、EBX=`00000101h`、ECX=EDX=`0` 時接受直接 `INT 10h`；記錄合成模式 `0101h` 已設定，回 EAX=`0000004Fh`，其他暫存器、段、EFLAGS 保持。一般 FD2、其他模式、未知高位或額外參數拒絕。模式狀態只供執行器追蹤，不能冒充 VGA 記憶體或玩家畫面。現有 `FD2StartupDOS.Handle` 已分離 MOO2 專屬的模式 03h 與 `4F07h`，可加入精確 `4F02h` 分支而不改一般服務；新測試核對返回、不變欄位與拒絕。原版返回和 consumer 足以決定這一筆服務，像素輸出仍未知。

## 驗收

1. 合成測試覆蓋精確返回、記錄狀態、未知參數／一般 FD2 拒絕及既有模式 03h／`4F07h` 回歸。
2. 含官方 1.31 EXE 的全套 Go 測試通過；完整正版資料由 dosgolem 自行越過第 190,517 步並記錄下一自然結果。
3. 僅這個固定模式設定可改為 CONFORMED；不能藉此宣稱畫面或玩法同狀態對拍。

## 實作與驗收

- `internal/machine/le_startup.go` 僅於 MOO2 設定記錄合成 VBE 模式 `0101h` 並回 `EAX=004Fh`；`internal/machine/le_startup_test.go` 核對暫存器／段／旗標不變、未知模式與額外參數拒絕、一般 FD2 拒絕。
- 含官方 1.31 EXE 的 `go test -buildvcs=false ./... -count=1` 全通過；私有 `workplace/full-test-239.txt` SHA-256 `e78f1025caaaacd4ec286585cfc18a0e92afe07dad03de08e18aad186322cb5f`。
- 相同 EXE 及正版根層 417 檔，在 `DOSGOLEM_MOO2_SEPARATE_DOS=1` 下由 dosgolem 自 LE 入口自然越過第 190,517 步；第 1,151,730 步停於**dosgolem 高位重定位 LE 線性位址** `0x25025F` 的未支援 opcode `04h`，原始立即數與來源語意尚待核對。私有 `workplace/moo2-probe-239-full-game.txt` SHA-256 `96af49b38b75df1cfd62c08ea9d30ba2f57624841bd9db6851e0b78f13507da6`。此收據證明平台啟動鏈前進，不代表已出現畫面或玩家路徑同狀態。
