# 282 — PIT 通道 0 模式 2 的計數鎖存

狀態：**CONFORMED**
日期：2026-10-02
範圍：已配置、已載入的二進位模式 2／先低後高通道 0，43h 控制字 00h 與 40h 的兩次鎖存讀取；硬體規格近似（hardware-spec approximation）。不建立 gate、完整 read-back／status、其他通道／模式或實機時序。

## 已證實的原始停點與公開契約

基線工具 9b8d1a07121fab929fc94a7f529449beb0d49742；固定官方 1.31 ORION2.EXE SHA-256 4e11be14217b4aafa1839f333bf5eba037f98b0c44e9e4752c96c464c260419f、417 原檔／MOX.SET、Go 1.24.13／映像與自然收據沿 [281-moo2-protected-irq0-end-chain.md](281-moo2-protected-irq0-end-chain.md)。兩個自然排程第 20,634,818 步均在 **dosgolem 高位 LE 線性** 0x239B3A 的 E6 43 拒絕，AL=00h、PIT Mode=2／Reload=5966／Generation=5，Micros=21092406。原始有限 bytes 為 E6 43 EB 00 E4 40 88 C4 E4 40 86 C4 25 FF FF 00；足夠定位一筆鎖存命令與兩筆 IN 40h 的標準埠介面，不拆其後 timer driver／busy-wait 控制流。原版返回／等待已由 277／281 接通，本項不猜寫遊戲等待值。

[Intel 8254，231164-005，1993-09](https://www.cs.cmu.edu/~410/doc/8254.pdf) 的頁 7–8／圖 9：通道 0、RW=00 是 count latch；不改設定或停止計數。按原先的低高次序讀完整計數後解除鎖存；未讀完再鎖存忽略，重新編程控制字清鎖存。讀寫序列可交錯。模式 2 的 N 個輸入時脈週期見頁 10–11；本工具已使用 250／276 的分數時基，不再追硬體邊緣、ISR、driver 或 wall-clock。

## 候選型別、轉移與拒絕

- 型別資料為 LEPIT0 的 Mode／Configured／Loaded／Reload／Generation、獨立 read latch 的 count16／pending／highNext，以及共享 LEBIOSClock 的 generation／credit；寫入半筆 highNext 不可與讀取半筆混用。
- 共享 LEOPLPorts 在 43h←00h 時取一次模式 2 計數。只接受有共享時鐘、已配置載入的 Mode=2、Reload=2..65536；未知／未載入／模式 3／無時計拒絕且不登錄成功埠紀錄。00h 以外的 latch don’t-care bits 仍採受限拒絕；其他通道／BCD／模式／read-back／未鎖存直接讀取拒絕。
- 已有 pending latch 時忽略新鎖存，不重設讀取低高次序；成功命令仍登錄一筆。首次鎖存以同一時鐘相位計算 count=Reload−floor(credit/(264×1000000))，credit 屬於當前世代且必須小於 Reload×264×1000000；剛完整換除數但時鐘尚未推進的世代視為相位零，不改時鐘。65536 轉為 16 位編碼 0。計數來源不是另一個主機時計；此公式是既有一指令一微秒／立即重載相位的明示近似，不冒稱硬體逐 tick。
- 40h 第一讀只返回凍結 count 的低 byte，第二讀高 byte 並清 pending；兩讀間虛擬時間推進或新鎖存不改凍結值。寫入新的 count bytes 可交錯且不消費讀取半筆；34h／36h 重新編程控制字清 read latch，既有半筆寫入／合法值及拒絕契約不變。完整換除數後的新鎖存只在前筆讀完或重新編程後取新值。
- 鎖存／讀取消費只改 read latch 及共享埠紀錄，不改 Mode／Reload／Generation／寫入半筆、BIOSClock／IRQ0／BDA、遊戲記憶體、CPU 其他暫存器／旗標。失敗保持全部 state／紀錄；不額外推進虛擬時間。
- 垂直鏈為原版標準 OUT／IN → 共享 PIT typed state → 凍結計數／兩次讀取 → 原版自然繼續；不是主庫玩法／UI／存檔規格。

## 驗收與停止線

公開原廠契約與既有自然停點狀態審查後才 READY。測全低高值／0 編碼、count 範圍與相位、重複 latch／兩讀間時鐘／低高寫入交錯／重配置取消、缺時計／未知模式／非法相位拒絕，共享實／保護 CPU 的最小 OUT／IN 序列與暫存器／旗標保持、埠紀錄及既有 BIOS／IRQ0 回歸。固定官方 EXE 全套及兩個自然排程自行重生後，才可限定 CONFORMED。

實機計數／兩執行器未控時間不同，不要求自然 IN 值逐次相等；不得用單元測試聲稱原版同狀態。下一停點只保存地址空間、bytes、平台缺口；若剩餘差異在硬體相位、DAC／ISR 或 wall-clock 即停止該深挖。主選單／正常玩家路徑、255 完整座標／游標、受控亂數與整款 remake 另行驗收。


## READY 審查

固定官方 EXE 的自然收據已直接證實 AL=0／模式 2／5966 及 OUT 43h 停點；count latch 的低高凍結／重複忽略／控制字取消有公開原廠契約。逐次自然計數是硬體時序，本項採上述既有共享分數相位近似，不為了取得同數值追驅動程式或再開 ISR 切片。初始候選不引入另一個時計、不更新遊戲記憶體；mode 3／未鎖存讀回仍拒絕。證據足以審查為 READY，後續實作及 CONFORMED 必須分別以平台與自行自然收據驗證。


## 受限實作與自行自然收據

限定 CONFORMED：LEOPLPorts 在 43h←00h 用共享 BIOSClock 取一次 Mode=2 計數，LEPIT0 的 read latch 與 write 半筆隔離；兩次 40h 返回凍結低高值，重複 latch 忽略，34h／36h 清 latch。read-back、未鎖存直接讀、模式 3／其他通道／未配置載入或未知相位仍拒絕。共享虛擬時計、BIOS／IRQ、資料記憶體不因 latch／read 額外推進或改寫。

平台 GOMAXPROCS=2 go test -buildvcs=false ./internal/machine -run 'Test(PIT0|ProtectedIRQ0|LEMouseCallback|BIOSClock)' -count=1 -v 通過，SHA-256 1c7da0875eaf21fe4ba2b51051ad423b239572d90a14f3f7df86767988997013。遍歷合法除數起點／末 tick 與所有 count16 編碼、分數相位／換世代、凍結／重複／讀寫交錯／重新編程、未知形狀原子拒絕；兩 CPU 模式先用既有 EC 的 DX 埠輸入確認共享接線，E4 是獨立 283 CPU 規格。初次樣板只有 BDA 記憶體容量而未容納程式，擴充測試樣板／I/O 接線後揭露 E4 缺件；兩份失敗收據保留，SHA-256 d9a77898a2e29222f2f30172820f6e73f737d42d11ca98f6ba99cc45b2a56130／d749bfe9fa7b1ad4b4ba179c41b2cb5904087094a3effcbc568541e6a03203f2。同映像／同平台命令乾淨重跑通過；第一項是測試環境問題，第二項是實際 CPU 缺件，不歸咎 latch。

固定官方 EXE 全套 go test -p 2 -buildvcs=false ./... -count=1 通過，full-test-282.txt SHA-256 f5fe6a43116c4138f75a3348e151d8f151064bf9f3b1fbb20625fe8705141d7a。兩個自然排程先在第 20,634,820 步停高位 LE 0x239B3E 的 E4 40，latch 已保存 5681、pending=true／readHighNext=false，尚未第一讀；gzip SHA-256 1a929e805bbf6f95dd1f3b5c990990c56abede7ed967505b1d479a4328d7325a／c24408b3ab822f01e771b505e7cd143876669c8722eeae753a4bcaff7370329b。平台 self-test 不偷當原版完整讀取完成。

立即埠輸入缺件已由規格 283 接通，見 [283-cpu386-in-al-imm8.md](283-cpu386-in-al-imm8.md)。283 的 dosgolem 自行收據證實原版首次 latch=5681（1631h），第一讀 AL=31h／pending=true、第二讀 AL=16h／pending=false，再自然繼續；兩讀間共享 credit 仍增加，凍結值不變。後兩筆不同自然相位又取 5302／5180，證明未永久鎖成測試結果。有限只讀診斷已加入受版控 workplace/moo2-probe/main.go，完整 CPU／平台及原版回歸合格後，282 限定 CONFORMED。

原版及工具使用不同時序，5681 等值只證明本工具依公開近似計算的自行轉移，不宣稱跨執行器逐值／週期一致。最終 PNG 同已檢視黑圖；下一標準 CPU byte SUB 停點、完整 283 自然收據與未完成玩家範圍只在 283。255 READY；主選單、正常操作、音效、受控亂數與 Go remake 玩法對拍未完成。
