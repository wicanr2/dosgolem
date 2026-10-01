# 243 — 第二組 DMA 控制器的獨立暫存器與位元組指標

狀態：**CONFORMED（平台硬體規格近似；僅限暫存器程式設定）**
日期：2026-10-01
範圍：dosgolem 的 AT 第二組 8237A 控制器程式設定埠；不進行 16 位元資料傳輸、音效輸出或硬體時鐘模擬。

## 問題與證據

- **已證實，固定原檔的 dosgolem 自生停點**：官方 1.31 `ORION2.EXE` SHA-256 `4e11be14217b4aafa1839f333bf5eba037f98b0c44e9e4752c96c464c260419f`、正版 ZIP SHA-256 `3a28a52f5953ff6d8fc251548940500236752ee19b52b51581e71ec1a3373c2f`，其中 `MOX.SET` SHA-256 `bfd6855a41760b31156b96114b5b33c88f442ab8f8aae020c1740b3b486a3a80`。隔離 dosgolem 基線提交 `7281a38fe2c7872fe59827b4ed40551def5d6f48`，明示高位 LE 映射、合成 PSP／環境；第 1,166,995 步由**dosgolem 高位 LE 線性位址** `0x2454AE` 呼叫 `DPMI 0300h → INT 66h`，實模式第 305 步於 **dosgolem segment:offset** `1201:0244` 寫 `OUT 00D8h,00h` 時拒絕。私有 `workplace/moo2-probe-242-full-game.txt` SHA-256 `142f0d05f0feea086353bd9cfbd5679f0dda095b9644da82cd8007376c187c06`。這是執行器內的真原檔路徑，尚非原版 DOSBox-X 同狀態音效樣本。
- **已證實，平台位址與動作**：[IBM PC AT 技術參考手冊](https://www.minuszerodegrees.net/manuals/IBM_5170_Technical_Reference_1502243_MAR84.pdf)將 `C0h..DFh` 分配給第二 DMA 控制器；[Intel 8237A 資料表，第 9 頁「Clear First/Last Flip-Flop」與圖 6](https://www.pcjs.org/documents/datasheets/intel/INTEL_8237A_DMA.pdf)定義索引 `0Ch` 的寫入將低／高位元組指標重設為初態，不依賴資料匯流排值。[DOSBox-X `dma.cpp`](https://github.com/joncampbell123/dosbox-x/blob/master/src/hardware/dma.cpp)將第二控制器偶數埠映為 `(port-C0h)>>1`，在索引 `0Ch` 清除該控制器自己的 `flipflop`。所以 `D8h` 對應索引 `0Ch`，`00h` 只是本次原檔寫入值，非命令條件。此為 **hardware-spec approximation**，不宣稱原版音訊時序或波形。
- **已證實，dosgolem 既有模型**：`internal/machine/dma8237.go` 的 `DMA8237` 已保存每一控制器的位址／計數低高位順序、遮罩及 mode；`LEOPLPorts` 目前只替第二組保存 `D4h` 的獨立遮罩。可直接建立第二個 `DMA8237` 狀態實例，避免第一、第二控制器共享指標或遮罩。
- **未知**：固定 MOO2 原檔在這次 `D8h` 後依序寫入哪些地址、計數、模式或頁暫存器；不得把尚未觀測的設定值寫成 MOO2 專用規則。

## 擬議契約與審查條件

在 `LEOPLPorts` 新建與第一控制器完全獨立的第二 `DMA8237`。只映射標準偶數埠：`C0h..CEh` 對應 0..7 的四通道位址／計數，`D4h` 對應單通道遮罩，`D6h` 對應 mode，`D8h` 對應低／高位指標清零，`DAh` 對應主清除，`DCh` 對應清遮罩，`DEh` 對應全遮罩。`D0h` 命令、`D2h` request、奇數埠與 16 位元傳輸仍明確拒絕；第二控制器 page register 不在本規格範圍，後續依[規格 244](244-secondary-dma-page-registers.md)獨立驗收。只有資料表已定義且既有 `DMA8237` 能保存的程式設定暫存器才接受。第二控制器位址／計數讀取只在已知 byte 時按同一指標返回；未寫入或不支援埠拒絕。所有成功操作繼續記錄原始 I/O 埠與值。

`LEDeviceState` 的 `SecondaryDMAMask` 改由第二實例的遮罩產生，並公開第二控制器原始位址／計數與 mode 的唯值快照。第一控制器的任何欄位不得因第二控制器指令改變；`D8h` 只重設第二控制器的位元組指標，`D4h` 舊規格行為保持。此為 dosgolem 平台層；MOO2 remake 的 parser、玩法、UI 與存檔無變更。

## 驗收

1. 合成測試確認 `D8h` 後首筆寫入是低 byte、次筆是高 byte；低高位指標在兩組控制器間獨立，`D4h` 遮罩與前規格相同，mode／mask／位址／計數快照正確，未知埠拒絕。
2. 固定 EXE 的全套 Go 測試通過；正版資料自 LE entry 自行越過 `D8h`，保留下一具體停點與收據。
3. 驗收只涵蓋標準暫存器程式設定與受限自然前進，不能宣稱有音訊播放、GUI 或玩法同狀態對拍。

## READY 證據審查

固定原檔只證明 `D8h/00h` 這筆輸入與先前 `D4h/05h`；埠映射、指標與其他暫存器語意由 IBM／Intel 平台規格及 DOSBox-X 實作界定。未知的 MOO2 後續值不進測試真值。兩個獨立控制器、明確埠白名單及失敗即關閉可由現有 `DMA8237` 模型無猜測地實作，故此平台切片可由 DRAFT 轉 READY；未驗證音訊與玩法的部分保持未知。

## 實作與驗收

- `internal/machine/le_opl_ports.go` 將第二組偶數埠映入獨立 `DMA8237`，不再以單一遮罩 byte 假裝整組控制器；`internal/machine/dma8237.go` 的模型註解改為單一控制器通用。合成測試核對 D8h 重設後的低／高位順序、兩組指標與遮罩隔離、位址／計數／mode 快照及拒絕邊界。
- 固定官方 EXE 的全套 `go test -buildvcs=false ./... -count=1` 通過，私有 `workplace/full-test-243.txt` SHA-256 `bfc30c39cbdb8ba04f060ac45749ae61bf3806d027c3d9f2125ed25056425f00`。
- 同一正版 ZIP 根層 417 檔自 LE entry 自行越過 `D8h/00h`，實模式第 338 步在 **dosgolem segment:offset** `1201:028F` 的 `OUT 008Bh,00h` 失敗即關閉。私有 `workplace/moo2-probe-243-full-game.txt.gz` SHA-256 `8b967af9e5c7b8c994e586b3e0229ca1510f7cab6493196ac3f1389b5d9ceb38`。這限於平台暫存器程式設定，未驗證 DMA 傳輸或原版音效。
