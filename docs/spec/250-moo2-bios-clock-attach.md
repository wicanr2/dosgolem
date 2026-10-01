# 250 — MOO2 啟動時安裝既有 BIOS 時鐘

狀態：**CONFORMED（重用預設 BIOS 時鐘接線）**
日期：2026-10-01
範圍：MOO2 平台接線；不改 Go remake 玩法、不反組譯 PIT／ISR 內部。

## 證據

- **已證實，原檔在合成平台的等待**：官方 1.31 `ORION2.EXE` SHA-256 `4e11be14217b4aafa1839f333bf5eba037f98b0c44e9e4752c96c464c260419f`、正版 ZIP SHA-256 `3a28a52f5953ff6d8fc251548940500236752ee19b52b51581e71ec1a3373c2f`、`MOX.SET` SHA-256 `bfd6855a41760b31156b96114b5b33c88f442ab8f8aae020c1740b3b486a3a80`。隔離 dosgolem `8f6bf14` 加規格 248／249 實作，Go 1.24.13／`golang:1.24-bookworm`，高位 LE／低位 DOS arena，自 LE entry 執行八百萬步。**dosgolem 高位 LE 線性位址** `0x222AAD..0x222AB5` 的 raw bytes `BE 6C 04 00 00 AD 3B C3 74 F6` 重複，載入 `046Ch`、與 EBX 比較，相同便回圈；EAX=EBX=0、DS=`0188h`、IF=1。私有 `workplace/moo2-probe-249-loop.txt.gz` SHA-256 `03cf7b8a41f86a6192b91fc4beb6977b81e1a3378b47ef397abb8cfbe53bb2a2`。未實作 DPMI 清單為空，這是等待狀態，不是 CPU 未支援指令。
- **已證實，接線缺口**：`MOO2StartupDOS.AttachMachine` 安裝 BIOS 資料區與共用平台埠，但未呼叫 `InstallLEBIOSClock`。既有 `le_bios_clock.go`／其測試及 [規格 186 批次 95](186-fd2-platform-gap-continuation.md) 已提供預設計時服務、IF／遮罩、午夜與客製向量拒絕契約。
- **公開平台前提**：[IBM PC AT 技術參考手冊](https://www.minuszerodegrees.net/manuals/IBM_5170_Technical_Reference_6280070_SEP85.pdf) 的 BIOS 計時欄位，以及規格 186 引用的 [DOSBox-X BIOS 來源](https://dosbox-x.com/doxygen/html/bios_8cpp_source.html)。不在 MOO2 重證平台常數。既有實作每道保護模式指令一微秒、PIT 預設重載 65536，採分數時基 `315000000/(264*1000000)` 推進 credit；其時間精度是 **hardware-spec approximation**，不是實機或原版 wall-clock。

## 契約與驗收

成功安裝 BIOS 資料區後，以新建的同一份 `LEOPLPorts` 呼叫 `InstallLEBIOSClock`；安裝失敗回傳錯誤，未連接 DPMI／平台 I/O。成功後再沿用既有兩模式共用接線。既有 StepHook 保留；既有 IF／PIC／自訂 INT08／INT1C 拒絕規則不變，不偽造遊戲 ISR。

測試須證明 MOO2 正式接線存在時鐘、每步只推進一次、既有 hook 不遺失、54926 個預設微秒後 BIOS tick 增加，保留 CPU 暫存器／旗標；客製向量及 BIOS 衝突仍拒絕。固定 EXE 全套測試後，由相同正版資料重跑自然啟動，記錄離開等待或下一拒絕。實模式／音訊時間耦合沿既有能力限制；無逐週期、畫面或玩法同狀態聲明。

## READY 審查

原檔等待的是既有 BIOS 計時欄位；平台契約、現有預設時鐘與接線缺口均可回查，不需猜測玩法或擴充客製 ISR。重用已有測試的服務，時間近似與未知向量拒絕邊界已明示；據此 DRAFT 轉 READY。

## CONFORMED 收據

首次建置的新錯誤訊息漏了套件匯入；改用已有 `errors.New` 後，以同一容器／命令乾淨重跑。正式接線、既有 hook、54926 微秒 tick、CPU 狀態保持與客製向量拒絕測試通過；固定 EXE 的全套 `go test -buildvcs=false ./... -count=1` 私有輸出 `workplace/full-test-250.txt` SHA-256 `d57b7d3b9da005b9ae06105bfa2e3bfcf7b11389c92a6c830c5cda109741b91e`。相同正版 417 檔由 LE entry 自然離開 `046Ch` 等待，第 1,545,396 步於 **dosgolem 高位 LE 線性位址** `0x24C31B` 的 `CD 33` 拒絕，EAX／EBX／ECX／EDX=`0`、EFLAGS=`0012h`，未實作 DPMI 清單為空。私有 `workplace/moo2-probe-250-full-game.txt.gz` SHA-256 `74b73985b73a1f077f2005343fef0168df1d05dbe49d03d8f824369f5105a1b8`。下一服務輸入／返回／consumer 待核對，沒有原版畫面或玩法同狀態宣稱。
