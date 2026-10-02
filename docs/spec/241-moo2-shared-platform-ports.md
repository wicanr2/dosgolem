# 241 — MOO2 保護／實模式共用平台埠

狀態：**CONFORMED（共用埠接線，非音訊對拍）**
日期：2026-10-01
範圍：隔離 dosgolem 的明示 MOO2 啟動設定，讓保護模式 CPU 與 DPMI `0300h` 實模式 handler 共用既有 `LEOPLPorts` 的硬體狀態。不新寫 DSP、DMA、PCM 計時或音效資料規則。

## 證據與邊界

- **已證實，固定原檔在 dosgolem 的阻塞**：官方 1.31 `ORION2.EXE` SHA-256 `4e11be14217b4aafa1839f333bf5eba037f98b0c44e9e4752c96c464c260419f`、正版 ZIP SHA-256 `3a28a52f5953ff6d8fc251548940500236752ee19b52b51581e71ec1a3373c2f` 根層 417 檔及 `MOX.SET` SHA-256 `bfd6855a41760b31156b96114b5b33c88f442ab8f8aae020c1740b3b486a3a80`。原檔自然進至第 1,166,995 步，**dosgolem 高位重定位 LE 線性位址** `0x2454AE` 的 DPMI `0300h` 指向實模式中斷 `66h`，向量 `1201:016A`；handler 至 `1201:0611` 寫 `OUT 0226h,01h` 時，因 `RealModeIO` 未接線而失敗。私有 `workplace/moo2-probe-240-full-game.txt` SHA-256 `0f00b6396e07eece1707f25932eb5450c46b385add27c4a8d5637171ab88d064`。兩側完整狀態不同，尚無 DOSBox-X 對這筆音效呼叫的同次收據。
- **已證實，已有平台模型**：`internal/machine/le_opl_ports.go` 的 `LEOPLPorts` 實作受限 DSP `0226h` 重設、OPL、PIT、第一 DMA 控制器與明確拒絕未知埠；`internal/machine/fd2_driver_startup_test.go` 的 FD2 入口已讓 CPU `PortIn/PortOut` 與 `DPMI.RealModeIO` 共用同一實例。`internal/machine/sb_dsp_test.go` 覆蓋 DSP 重設握手。這是 dosgolem 既有工程契約，不是 MOO2 原版逐波形證據。
- **已證實，明示可丟棄原型**：在未提交的診斷探針中，以 `NewLEOPLPorts()` 同一實例賦予 `m.CPU.PortIn/PortOut` 與 `services.DPMI.RealModeIO`，並用 `DOSGOLEM_MOO2_PORT_PROTOTYPE=1` 明示。相同原檔及資料自 LE 入口重跑，DPMI `0300h → INT 66h` 由實模式第 55 步的 `OUT 0226h,01h` 前進至第 299 步 `OUT 00D4h,05h`，仍失敗即關閉。私有 `workplace/moo2-probe-241-port-prototype.txt` SHA-256 `2052024dc313db8d12bc8a1ea0aaf908859a037f29f094b472eb4b17c35dbaa2`。原型已移除；正式接線以本規格實作及下一筆自生收據為準。`00D4h` 是獨立的第二 DMA 控制器缺口。
- **已證實，平台停止線**：[Intel 8237A 資料表](https://www.pcjs.org/documents/datasheets/intel/INTEL_8237A_DMA.pdf)及 dosgolem 既有 FD2 平台規格 186 描述 DSP／DMA 的標準契約。此規格只接現有模型，屬 **hardware-spec approximation**；不重做 DAC／PIT／DMA 的逐週期考古，未核對音訊播放時長、波形或人耳聽感。

## 擬議契約與 READY 審查

只在 `MOO2StartupDOS.AttachMachine(m)` 建立一份 `LEOPLPorts`，賦予 `m.CPU.PortIn`、`m.CPU.PortOut` 與 `s.DPMI.RealModeIO` 同一實例；一般 `FD2StartupDOS.AttachMachine`、原 loader 與非 MOO2 設定不變。`LEOPLPorts` 自身對未知埠仍拒絕；不得因 MOO2 要繼續而回傳合成成功值。`MOO2StartupDOS` 不複製或修改 DSP／DMA 實作，也不把 `00D4h` 納入本規格。既有 FD2 回歸路徑與原型證明同實例接線方法可行，故可進正式平台接線。

## 驗收

1. 合成測試：同一 MOO2 實例的保護模式與實模式埠輸入／輸出共享 DSP 重設狀態，未知埠拒絕；一般 FD2 不自動接線。
2. 含官方 EXE 的全套 Go 測試通過；明示高位 LE 載入與完整正版資料自原檔入口重跑，原 `0226h` 停點自然前進，下一未實作服務保留具體底層錯誤。
3. 此驗收只涵蓋共用平台埠接線，不宣稱音訊、正常玩家畫面或玩法同狀態對拍。

## 實作與驗收

- `internal/machine/le_startup.go` 的 `MOO2StartupDOS.AttachMachine` 把同一 `LEOPLPorts` 接到保護模式 CPU 與 DPMI 實模式埠，既有 FD2 設定保持原樣。`internal/machine/le_startup_test.go` 驗證跨模式 DSP 重設回覆 `AAh`、未知 `00D4h` 埠拒絕及一般 FD2 不被接線。
- 含官方 1.31 EXE 的全套 `go test -buildvcs=false ./... -count=1` 通過；私有 `workplace/full-test-241.txt` SHA-256 `06c75ae6fa9adec96d5b6153f731499bea5cdc3b9fb46abe6ae3153fb0a6894a`。
- 不開原型旗標、相同正版資料及明示高位 LE 載入，自原檔入口自然重跑；原 DSP `0226h` 停點前進，`INT 66h` 的實模式第 299 步停在 `OUT 00D4h,05h`。私有 `workplace/moo2-probe-241-full-game.txt` SHA-256 `7338276b9565381915d397c6bdebdfc3d26aafa11e56b22e4396a63744f0faf9`。後續第二 DMA 控制器服務仍未知，本規格不把它填為成功；目前沒有音效輸出或正常玩家畫面驗收。


## 共用裝置時間後續

保護模式裝置時計缺口由規格 304 接線，見[304-le-shared-device-clock.md](304-le-shared-device-clock.md)。2026-10-03兩自然首block真正PCM2048個80h／兩時計44032078已驗；第42356668步以absolute IVT1201:0682停在未建模的保護模式IRQ7，pending保留。只解時間到首block，IRQ7轉送／連續PCM、人耳及正常玩家路徑仍未知，其他原有證據與限制保持。
