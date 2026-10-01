# 244 — PC AT 第二組 DMA 頁暫存器

狀態：**CONFORMED（平台硬體規格近似；僅限頁暫存器）**
日期：2026-10-01
範圍：dosgolem 平台層保存第二組 DMA 通道 4–7 的外部頁暫存器；不建立資料傳輸、頁位址合成、PCM 輸出或計時。

## 證據與分級

- **已證實，固定原檔 dosgolem 自生停點**：官方 1.31 `ORION2.EXE` SHA-256 `4e11be14217b4aafa1839f333bf5eba037f98b0c44e9e4752c96c464c260419f`、正版 ZIP SHA-256 `3a28a52f5953ff6d8fc251548940500236752ee19b52b51581e71ec1a3373c2f`，內含 `MOX.SET` SHA-256 `bfd6855a41760b31156b96114b5b33c88f442ab8f8aae020c1740b3b486a3a80`。以隔離 dosgolem 基線 `7281a38fe2c7872fe59827b4ed40551def5d6f48` 加當輪未提交的規格 243 實作、明示高位 LE 映射與合成 PSP／環境，自原檔入口至第 1,166,995 步的**dosgolem 高位 LE 線性位址** `0x2454AE`，在 `DPMI 0300h → INT 66h` 的實模式第 338 步、**dosgolem segment:offset** `1201:028F` 寫 `OUT 008Bh,00h` 時拒絕。私有 `workplace/moo2-probe-243-full-game.txt.gz` SHA-256 `8b967af9e5c7b8c994e586b3e0229ca1510f7cab6493196ac3f1389b5d9ceb38`。這是 dosgolem 自生路徑，尚無原版 DOSBox-X 同狀態音效收據。
- **已證實，平台映射**：[IBM PC AT 技術參考手冊，1985 年版](https://www.minuszerodegrees.net/manuals/IBM_5170_Technical_Reference_6280070_SEP85.pdf)列出 DMA 通道 5 的頁暫存器在 `008Bh`；[DOSBox-X `dma.cpp`](https://github.com/joncampbell123/dosbox-x/blob/master/src/hardware/dma.cpp)寫入／讀取表分別以 `8Fh→4`、`8Bh→5`、`89h→6`、`8Ah→7` 路由至第二組控制器。頁暫存器是 PC AT 的外部位址擴展，不是 8237A 內部時鐘規則。本規格的儲存與回讀是 **hardware-spec approximation**，不宣稱遊戲已實際傳輸資料。
- **未知**：MOO2 在這筆頁設定後是否啟動通道 5、傳輸多少資料或何時完成；不由 `OUT 8Bh,00h` 猜測。

## 擬議契約與驗收

`LEOPLPorts` 在第一 DMA 頁暫存器路由之外，為第二 `DMA8237` 實例加精確四埠白名單：`8Fh→相對通道 0`、`8Bh→1`、`89h→2`、`8Ah→3`。輸出逐通道保存原始 byte 與 known 位；已寫入後的輸入返回原 byte，未寫入前拒絕，其他 `80h..8Fh` 未定義埠仍拒絕。第一控制器的頁與地址、遮罩、mode 均不變；唯值快照顯示第二控制器四個頁 byte。所有成功操作保留原始埠日誌。此功能不改 remake 規則、UI、存檔與資產。

合成測試需驗證四通道對應、零值與非零值、讀回、未寫入／未知埠拒絕及兩控制器隔離。固定 EXE 的全套 Go 測試須通過，正版資料自 LE entry 自然越過 `8Bh` 並記錄下一具體停點。成功僅代表 dosgolem 平台程式設定路徑前進，不代表原版畫面、音效或玩法同狀態對拍。

## READY 證據審查

原檔只證實這次 `8Bh/00h` 輸入；四埠路由由 IBM PC AT 手冊與 DOSBox-X 表格直接決定，且只將原始 byte 儲存在獨立控制器中。未知的傳輸行為明確不在契約內，未觀測的 MOO2 設定值不會進測試真值。因此由 DRAFT 轉 READY，可進入受限實作。

## 實作與驗收

- `LEOPLPorts` 只為 `8Fh／8Bh／89h／8Ah` 接入第二 `DMA8237` 的 page byte 與 known 位，原始 I/O 埠仍保留在讀寫日誌；唯值快照新增 `SecondaryDMAPage`。測試核對四通道、零／非零值、未寫入讀取拒絕、未知埠拒絕與兩控制器隔離。
- 固定官方 EXE 的全套 `go test -buildvcs=false ./... -count=1` 通過，私有 `workplace/full-test-244.txt` SHA-256 `6b91e417a4d0839fe83500e40834a58672b6367a8d0999e73701008d22662e6b`。
- 同一正版資料自 LE entry 自行越過 `8Bh/00h`，在 `DPMI 0300h → INT 66h` 的實模式第 457 步、**dosgolem segment:offset** `1201:05D9` 的 `OUT 022Ch,B0h` 失敗即關閉。私有 `workplace/moo2-probe-244-full-game.txt.gz` SHA-256 `337e94af41018d907487430fc80bab5db6a965ce93972048bc1a19152b33e96b`。依[Creative Sound Blaster 硬體程式設計指南，命令 Bxh](https://www.ardent-tool.com/sound/Sound_Blaster_HW_Programming_Guide_1st.pdf)，`B0h` 是 16 位元單次 D/A DMA 命令家族；本規格不把命令視為成功，也不聲稱已播放音訊。
