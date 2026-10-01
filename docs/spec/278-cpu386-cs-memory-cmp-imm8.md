# 278 — CS 記憶體目的與帶符號 imm8 的比較

狀態：**CONFORMED**
日期：2026-10-01
範圍：cpu386 的 2E 83 /7 ib 記憶體 CMP，32 位位址、16／32 位運算元。只補標準 CPU 契約，不改遊戲、時計或中斷處理器。

## 原始定位與證據

固定官方 1.31 ORION2.EXE SHA-256 `4e11be14217b4aafa1839f333bf5eba037f98b0c44e9e4752c96c464c260419f`；固定 417 原檔、MOX.SET 與工具版本沿 [277](277-moo2-dos4gw-protected-irq0.md)。基線 `777472b02a561740e11558ecb9852b7579541454` 的 dosgolem 自然首次 IRQ0 在高位 LE 線性 `0x244D9A` 遇到 `2E 83 3D FE 19 27 00 00`，尚未執行 CMP。目的為 CS:offset `002719FE`，不能誤當 DS 或檔案偏移。

277 的 DOSBox-X 輔助入口為 `0180:00378D9A`，bytes `2E 83 3D FE F9 39 00 00`，目的 CS:offset `0039F9FE`。這是不同載入空間的同指令；沒有原版資料與完整旗標前後樣本前，不能宣稱原版一致。本項只允許該入口一條 CMP 的有限觀察，處理器其餘部分繼續黑箱執行。

[Intel 80386 CMP](https://www.ardent-tool.com/CPU/docs/Intel/386/manuals/prref386/CMP.htm) 定義 83 /7 的 imm8 符號擴展至運算元寬度，再做減法並只更新 OF／SF／ZF／AF／PF／CF，不保存差值。[17.2 指令格式](https://www.ardent-tool.com/CPU/docs/Intel/386/manuals/prref386/s17_02.htm) 定義 2E 選擇 CS、66 改運算元寬度，32 位 ModR/M 與 SIB 算出偏移；預設 DS／SS 不得覆蓋明示 CS。這是公開架構契約，並非遊戲 ISR 語意。

## 候選契約與驗收

- 只放行 CS 覆寫的 83 /7 記憶體形狀；暫存器及其他群組仍拒絕，防止落入舊回退分支。ES／DS／SS 的 83 與其他未知前綴不因本項放寬。
- 使用既有 32 位位址解碼，包含絕對、基底、位移及 SIB；讀取 CS 描述子的 Base＋offset，完整檢查 2／4 bytes，允許唯讀資料。既有指令擷取仍是平坦 LE 模型，本項不建立非平坦程式擷取。
- imm8 符號擴展，依寬度更新六旗標，保持其他旗標、全部暫存器、段與記憶體；成功 EIP 指向下一指令。失敗為工具錯誤，不宣稱精確 #GP；解碼 EIP 可前進，但資料／暫存器／旗標不得部分修改。
- 重複前綴、LOCK、REP、16 位位址、截短指令、越界讀取均拒絕。測試涵蓋所有 imm8、邊界值、DS／SS 混淆、SIB／位移、唯讀與部分讀取。
- 原版有限樣本審查後才 READY／實作；實作測試與 dosgolem 自行重生的自然收據後才能限定 CONFORMED。277 的完整返回另外判定，不隨 CPU 測試升格；正常玩家路徑／亂數／音畫與整款 remake 未完成。

## 有限原版樣本審查

已證實：同固定輸入、DOSBox-X 2026.07.02 SDL2 重型除錯器與規格 277 的映像，自然進入 0180:00378D9A，CS:0039F9FE 四 bytes 為零；只執行一條 CMP 後在 0180:00378DA2，資料仍四個零，完整 EFLAGS 46h 不變，所有通用暫存器與段不變。原版當次 PIT 是模式 2；dosgolem 首次 IRQ 在模式 3，兩者整段狀態並不相同，本項只比同指令的輸入資料與旗標。

原版有限樣本完整順序 CS EIP EAX EBX ECX EDX ESI EDI EBP DS ES FS GS SS ESP EFLAGS：180 378d9a 0 174e 0 174e c8 8 8 188 188 0 20 d0 6838 46。下一位置只改 EIP=378da2。資料與欄位均保留原始定位，不猜測其 ISR 用途。

本機 workplace/cs-memory-cmp-registers.json SHA-256 245fb2c1eedf83260c2f748ab66a67715a10867f73f92374dbb68ba2057d56d0；單指令 LOG 8353f1908d8d52d176b752db49fd0963481a60cd1ed9732f2171e6c87ccffd34；終端 2831fac43b52b6ccfdb76382e4605a7116ef31d5d61c31309ee581a32c64e2a2。命令為既有 DOSBox-X 容器入口執行 apps/moo2/tools/startup_probe_131.py --cs-memory-cmp，原始完整輸出不進 Git。公開契約、原始 bytes 與完整樣本審查一致，候選契約轉為 READY；沒有實作／自然重跑前仍不可 CONFORMED。

## 受限實作與 dosgolem 自行重生

限定 CONFORMED：32 位位址／16 與 32 位資料 CMP、所有 256 個 imm8、獨立六旗標、全記憶體 ModR/M／SIB、唯讀 CS、DS／SS 混淆、截短／群組／前綴與部分讀取拒絕，及搬入 CPU 的完整原版有限單指令樣本通過。後者是相同輸入的有限 CPU 比較，不是原版整段 ISR 同狀態或正常玩家路徑。

Go 1.24.13 與規格 277 的映像；GOMAXPROCS=2 go test -buildvcs=false ./internal/cpu386 -count=1 通過，workplace/moo2-278-cpu-tests.txt SHA-256 45eb88ae7a5950abb9495b7377f3a2ab1c727ff826f2df7f41653b40c05cb22c。DOSGOLEM_MOO2_EXE=/tmp/game/ORION2.EXE go test -p 2 -buildvcs=false ./... -count=1 通過，workplace/full-test-278.txt SHA-256 be0f1aeef7ff23f09990183ddbd3218003d6d1138ff7c16c8a94b0b709b37822。

兩條自然排程按 277 命令重跑，PNG／gzip 名稱改 278。均在外層第 1,160,098 步、內層高位 LE 0x244DBA 停止，bytes 66 2E 8E 05 06 1A 27 00 FF 05 FE 19 27 00 8C 15；原因是既有 CS word 段載入只允許 DS，當前目標是 ES。已越過 0x244D9A 比較；不是返回或等待閉合。內層解碼 EIP=244DBEh，完整上下文恢復至外層 0x234341；IRQ started=1／completed=0，Micros=1160110、Deliveries=21、InService=true，PIT 模式 3／Reload=19887。DS:00271148 四 bytes 仍零。

自然 gzip SHA-256 26582e61ecf36a808d296f4122ee51a3af9666b033c272cc7b3d116bfae190f4／73435ec83d414aef7d1c5d5e14f981113e75645a4f76a86e7c3a2244c0095a5f。事件條件 requested=true／injected=false，仍不可當兩條玩家分支驗收。VBE Writes=0，PNG SHA-256 1610444d26adb3135e7e933dd44912044bb636728af59c70e614945278d3c622 與已檢視黑圖相同。277／255 仍 READY；下一步只補公開標準 MOV ES 的段載入契約，再黑箱自然重跑，禁止解 ISR、driver／busy-wait 或注入等待值。
