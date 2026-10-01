# 279 — 從 CS 絕對 word 位址載入 ES

狀態：**CONFORMED**
日期：2026-10-01
範圍：`66 2E 8E 05 disp32` 及兩個前綴對調，沿 [256-cpu386-cs-absolute-ds-load.md](256-cpu386-cs-absolute-ds-load.md) 的受限 selector 模型；不更改玩法或解 ISR。

## 已證實定位與候選契約

基線 `fabbed1a8bfd1c010b66009bdffe4a6812eb9924`，固定官方 1.31 ORION2.EXE SHA-256 `4e11be14217b4aafa1839f333bf5eba037f98b0c44e9e4752c96c464c260419f`；417 原檔、MOX.SET、Go 與 Docker 版本沿 [278-cpu386-cs-memory-cmp-imm8.md](278-cpu386-cs-memory-cmp-imm8.md)。兩個自然排程條件首次 IRQ0，**dosgolem 高位 LE 線性** `0x244DBA`，bytes `66 2E 8E 05 06 1A 27 00`，來源為 CS:offset `00271A06` 的 word，目的是 ES。既有 CS 8E 閘門只允許 ModR/M 1Dh 的 DS，因而在讀取前拒絕。來源內容及原版完整前後狀態仍待有限擷取。

[原廠 Intel 80386 手冊](https://read.seas.harvard.edu/~kohler/class/aosref/i386.pdf) 的 MOV 段載入契約沿規格 256；[MOV 說明鏡像](https://www.ardent-tool.com/CPU/docs/Intel/386/manuals/prref386/MOV.htm) 定義從 word 載入段暫存器，旗標不變，非零 selector 需描述子檢查。鏡像與該重排 PDF 的 MOV 表格均誤列 8D；同 PDF 第 413 頁的段編碼例明列 8C／8E，第 414 頁單 byte 表區分 8D 的 LEA 與 8E 的 MOV Sw,Ew。編碼以此表及原始 8E bytes 為準，不把誤植寫入解碼器。

- 只解除 CS 的 ES 絕對 word 形狀限制；保留 256 已驗的 DS。operand16、無 repeat、ModR/M 05h／1Dh，其餘 CS 8E 仍拒絕。
- 32 位 disp32 指向 CS 描述子的 Base＋offset；只讀兩 bytes，完整越界檢查，可唯讀來源。word 後雜訊不得當作來源高半部。
- 使用既有 canLoadSegment：支援 null 0、已知描述子或既有 resolver 明示允許的 selector。未知 selector 拒絕，未建模的 1–3 null 別名／DPL／present／段類型與精確 #GP／#NP 不因本項擴充；這是既有工具模型，不宣稱完整 x86 保護檢查。
- 成功只改目的 ES、EIP 指向下一指令；通用暫存器、其他段、全部旗標與記憶體保持。失敗解碼 EIP 可前進，其他狀態不得部分改寫。LOCK／REP／16 位位址／重複段前綴、截短／來源越界仍拒絕。
- 有限原版只在標準 MOV ES 停點觀察一條指令前後；其餘處理器黑箱執行，不解計時邏輯。原版樣本與公開契約審查後才 READY，實作與全部 CPU／固定 EXE 回歸及自然重播後才能限定 CONFORMED。原版 IRQ0 返回與等待、玩家／音畫／亂數及 remake 玩法同狀態另判。

## 第一輪有限樣本與命令收尾

原版有限樣本已捕獲：DOSBox-X 0180:00378DBA 的 CS:0039FA06 原始四 bytes 為 88 01 FF FF，MOV 只載入 word 0188h；下一 EIP=00378DC2，ES 原本就是 0188h，仍為 0188h，資料及完整其他狀態不變，flags=46h。原版樣本不包含 ES 值變化，正式 CPU 測試另以不同 ES 初值確認目的選擇。只觀察該標準指令，不推測其 ISR 用途。

第一輪完整 JSON、LOG 與自然等待退出已取得，但命令最後回傳外層 timeout 的 124；不把它記作原版／CPU 缺陷，也不宣稱命令乾淨成功。保留 workplace/cs-word-es-load-timeout-registers.json SHA-256 ba65afbaa06cfb55cf22b867fb8fb3ec0980d5dfb10a8e1faf0a1884d0886816、單指令 LOG ea8e7f652fc1be6aa1c04a1aef2406a1117496e8dcefb9d07ce030d296b7203a、終端 6e9732e305d4facf21974f10f4b543cea6ee520702ca6d04740c3ed7f5fd36aa。收尾因果尚不能精確歸到哪個程序；僅把外層上限 240 改 300 秒，用同一映像、原檔與命令乾淨重跑，仍維持有界 trap 與 --rm。

## 乾淨重跑與 READY 審查

相同固定原檔、DOSBox-X 2026.07.02 SDL2 重型除錯器與規格 277 映像，startup_probe_131.py --cs-word-es-load 在 300 秒外層限制內退出碼 0；樣本及自然等待退出與首輪一致。JSON SHA-256 ba65afbaa06cfb55cf22b867fb8fb3ec0980d5dfb10a8e1faf0a1884d0886816，單指令 LOG ea8e7f652fc1be6aa1c04a1aef2406a1117496e8dcefb9d07ce030d296b7203a，乾淨終端 d4f037c00d4778f846f33f1288971d4ba0a8e3b4bba24e16e96e7aca0e485b50。完整輸出只留本機 workplace。

完整欄位順序 CS EIP EAX EBX ECX EDX ESI EDI EBP DS ES FS GS SS ESP EFLAGS：180 378dba 0 174e 0 174e c8 8 6808 188 188 0 20 d0 6808 46。下一位置只改 EIP=378dc2。公開 CPU 與原始有限樣本審查支持候選契約，轉為 READY；尚未實作前不得 CONFORMED。有限 CPU 測試會保留此完整輸入，另以不同 ES 值驗目的選擇。

## 受限實作與自然回歸

限定 CONFORMED：只擴充 CS 絕對 word 的 ES 目的，沿既有 256 的 selector 模型。兩個前綴次序、有效／null 0／未知 selector、唯讀來源／DS 混淆／word 後雜訊、全部未核准 ModR/M、截短／前綴／部分讀取拒絕、resolver 原有介面與完整原版有限樣本全部通過。舊 DS 測試把現在已核准的 ES 拒絕樣本改成仍未核准的 SS；DS 回歸及 CPU 套件全部通過，未放寬其他指令形狀。

Go 1.24.13 與規格 277 映像，GOMAXPROCS=2 go test -buildvcs=false ./internal/cpu386 -count=1 通過；workplace/moo2-279-cpu-tests.txt SHA-256 18f9e249b85400a59bb1e55526c443d4fb0eec029d9f14df33b96294b1d68e84。固定 EXE 的 go test -p 2 -buildvcs=false ./... -count=1 通過，workplace/full-test-279.txt SHA-256 95a06d5dfbbbe65897ad443fc05910667b54aa54753aca15ce886683d037dfaf。

兩個自然排程按 277 命令重跑，輸出名稱改 279。已證實：第 1,231,392 外層步停止於原版第六次 IRQ0，started=6／completed=5；前五次原版最外層返回已由受限橋接檢查完成。現行內層高位 LE 線性 0x244E9C，bytes CB FF 0D FE 19 27 00 8E 15 14 24 27 00 8B 25 18，拒絕 CB 遠返回，解碼 EIP=244E9Dh；完整外層上下文恢復至 0x246596。內層 R 順序 EAX ECX EDX EBX ESP EBP ESI EDI：3D0300 80 3D0700 FFD23C00 FEC FFD23C00 80 FFFF8C00；CS DS ES FS GS SS：8 188 188 0 20 110，flags=246h。沒有為了解讀它而拆解處理器內部。

PIT 模式 3／Reload=14916／Generation=3，Micros=1233786、Deliveries=26、InService=false；DS:00271148 仍四 bytes 零，完整模式 2 等待未閉合，所以 277 仍 READY。事件條件 requested=true／injected=false；不能算兩條玩家輸入分支驗收。兩份自然 gzip SHA-256 f32bf14adca2a5053edcfe93854fe7e4919d57c2f0823d276a4da7e3dcb9ee3a／19163a9a44954cfa04054eba31f3e69682ac21a6143ee3073516f3075e702163。VBE Writes=0；PNG SHA-256 1610444d26adb3135e7e933dd44912044bb636728af59c70e614945278d3c622 同已檢視黑圖，不再做重複目視。

下一步只按公開 CPU RET 契約建立標準 CB 的有界輸入／返回驗證，再讓原版自然重跑。不解 ISR、timer driver／busy-wait，不猜寫等待值；主選單／玩家路徑、音效、受控亂數與 Go remake 玩法同狀態未完成。
