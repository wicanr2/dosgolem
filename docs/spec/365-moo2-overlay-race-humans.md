# 365：可寫覆蓋層正常選取 Humans

狀態：**CONFORMED，限定隔離Humans選擇與95M名稱頁；完整可寫玩家路徑未驗**
日期：2026-10-04

## 範圍、來源與證據

沿[364](364-moo2-overlay-setup-accept.md)已驗可寫ACCEPT及原90M SELECT RACE，對照[337](337-moo2-race-humans-normal-click.md)正常輸入契約。本輪只送一次測試情境的Humans選擇，蒐集95M名稱頁；不改正式種族預設、不接主庫玩法、不預定完整開局或存檔成功。主庫RE-first保持。

原官方1.31 ORION2.EXE SHA-256 4e11be14217b4aafa1839f333bf5eba037f98b0c44e9e4752c96c464c260419f；原ZIP417根檔另加官方EXE共418檔。MOX.SET／1996-01-01／完整180M參數與正常44M Esc、NEW GAME／50M選單／80M ACCEPT保持。1996不是seed；位址空間dosgolem_high_le。起始公開工具98950c28961c0f89ed63304cd131477d083c85b7；可寫overlay沿362既有試作與364私有profile，不改公開CPU／DOS／provider／probe。

364原收據cba0542dce180527e2a6a258e7977b9c2b5b153231e1d7f89bb8ac17ce93fc8d已保存90M完整880bytes表，pointer298848／count16／stride55／bias0，header10000000000000000000000000000000，SHA-256 eca22108024dbc103f8de8257b32321b0b2e39fcd6bb8435cb4dc0ba73e29396；RGB9d8c0a1acb3b96200296f789bb13c6877067f6165ec608a7e019506036832dfc，PNG與既有原337選族頁相同。原index7幾何351,330–473,374；412,352與413,352唯一命中此筆，已親看Humans按鈕。完整指標用途／typed特性未知，不從圖片猜新規則。

90M原EIP228E0B／R=[77 116 FA 95 2BD9F0 2BDA4C 2C0C4A 2C04BC]／段=[8 188 188 0 20 188]／flags287h，IF開；FPU127F／status0／depth0／八stack bits0。callback mask2Bh／pending0／非活動／6／6，IRQ20869／20869非活動且非failed；正常ACCEPT press／release及真正store已驗。原正常輸入契約要求target8:2136D1，本輪在90M送輸入前實讀並核對，不從舊80M記錄外推目前target。

## 預定工具契約

獨立私有旗標DOSGOLEM_MOO2_OVERLAY_RACE_HUMANS_PROFILE=1，其他值或缺364 profile／完整180M game-dir空state／RACE_HUMANS_CLICK依賴在讀EXE前拒絕。模式關閉保留364在90M只讀停止，原337唯讀race guard不改。啟用時只改獨立profile分支：固定90M，核對實際完整880byte表／header／DS／RGB／完整原CPU與FPU／IRQ20869／20869，以及已完成ACCEPT、callback6／6、target／mask、IF和只讀檢查；不符合就停止，不mask位址、加任意位移或調時刻。

沿337，90000000一次正常InjectMouseEvent x824／y352／buttons1，原映射412,352。正常回呼至少完成一次、虛擬時間至少20000µs與原available條件第一次可送時release x826／y352／buttons0，映射413,352。保留原20DDDB六byte66A3A6C42600選擇store觀察，首筆完整初態／寫回／error保存，不代寫DS188:26C4A6或種族。

95M在原名稱ACCEPT前置列保存後停止，禁止送ruler或banner輸入。只讀保存原165byte表／CPU／FPU／callback／IRQ／RGB／PNG；另按原index2+24 raw值取原descriptor所指32byte候選，若不可讀明示失敗，不猜位址。原候選與完整物件／持久名稱writer仍未知，不因字串相同外推存檔。解析180M參數保持，95M diagnostic stop不稱180M完成。

## 驗收、權利與入口

- 前90M保持364所有共通原列與28PNG，僅新增profile診斷；352原mtime／DTA／每輪診斷RAMhash正規化保持。額外輸入後不同狀態不作同狀態逐列比較。
- 合法profile／無效值與缺依賴做最小CLI核對，私有來源可逆為364，公開internal／CPU／DOS／provider／probe逐byte保持。
- 原guest一次正常選族，保存實際press／release／virtual micros與真正原store、95M原表及畫面；成功與失敗據實，不反覆重跑挑結果。
- 原418檔guest前後SHA-256保持，state清單／大小／雜湊／擁有權核對。原資料／PNG／LOG／RAM留忽略workplace，不公開；公開只交自製規格／索引／雜湊與回填。
- 完整開局／正式存檔、typed種族／名稱／旗色、RNG、人耳與remake同狀態仍未驗。不追allocator／renderer／runtime helper。

沿Go1.24.13 Docker映像sha256:1a6d4452c65dea36aac2e2d606b01b4a029ec90cc1ae53890540ce6173ea77ac，600s／2GiB／2CPU／128pids／UID1000／network none，原ZIP／patch唯讀。私有入口workplace/moo2-overlay-humans-365.go／new-game-365-run.sh，公開同次掛000-index，完整收據掛主庫docs/re/dosgolem-moo2-intake-20260930.md。

## READY證據審查

以原364同guest收據執行workplace/new-game-365-ready-review.py通過。完整880bytes／header／RGB、index7唯一幾何、完整CPU／FPU／IF／callback6／6／IRQ20869／20869及已完成ACCEPT來源均吻合；原90M沒有race輸入。固定座標、正常InjectMouseEvent與回呼完成／20ms契約沿337，target在新90M實讀核對。證據足以實作隔離正常輸入及有限原版觀察，轉READY，不接公開玩家path，未驗typed種族／名稱／存檔，主庫RE-first保持。

## 限定CONFORMED：隔離Humans選擇與95M名稱頁

**已證實**：READY後原guest一次正常選族。90000000／162623172µs按下x824／y352／buttons1，90008107／162662935µs放開x826／y352／buttons0，相差39763µs，回呼7／7完成後首次送。原90066074於dosgolem_high_le:20DDDB執行66A3A6C42600，DS188:26C4A6 word0000→0700，下一20DDE1，原R=[7 50A004 181 298848 2BDA2C 2BDA94 D 1A]／段=[8 188 188 0 20 188]／flags297h保持；callback8／8與IRQ20889／20889非活動且非failed，error=nil。只核對六byte指令，16byte觀察窗口其餘bytes仍保存。

95M原Enter Ruler Name／預設Strader／ACCEPT已親看原640×480 PNG，與338原95M圖逐byte相同。RGB2646a7ef25939869ede60aa35bcd97d649b906fd2cc287d08deb6c2d8058cc12／PNG e1c739f5aeaf47a6cfdca4749b14509ec2592b65974fa6669e9347c491a52a36。完整3筆165bytes表SHA-256 db67c471ba6a34a66146e083662f4c134c192e2371a807ec9647327558a7fd1b，與舊唯讀表不同；index2+24原值28439D，原DS descriptor所指32byte為Strader與補零，FPU127F／status0／depth0／八stack bits0，CPU／FPU／RAM／VBE只讀保持。這是原文字編輯候選，不稱正式持久名稱writer。原338 ruler guard false，保持不改；95M明示停止，未送ruler或banner，不稱180M完成。

**驗證**：90M前6956共通原364列依352既有mtime／DTA／每輪診斷RAMhash正規化保持，包含自身只讀profile診斷RAMhash；28舊PNG逐byte保持。原337唯讀race guard仍false，獨立profile核對真實完整880bytes／CPU／FPU／IRQ才true；新90M實際target8:2136D1已由press／release收據核對。三區塊與兩個有界診斷條件可逆為364，公開CPU／DOS／provider／probe及全部internal逐byte保持。四CLI無效值／缺依賴在讀EXE前拒絕，合法正對照只因缺EXE失敗。原guest一次，沒有重跑挑成功。

原418來源檔guest前後SHA-256保持。state與364完全相同，僅sound.lbx4250888bytes／原ZIP同bytes／UID及GID1000，沒有SAVE10.GAM。typed種族／正式持久名稱／存檔／完整開局／RNG／音訊與remake同狀態仍未驗。

### 命令與收據入口

```text
python3 workplace/new-game-365-ready-review.py
bash workplace/new-game-365-run.sh
python3 workplace/new-game-365-verify.py
```

均於既有Docker執行並通過。12份本機自製腳本／原版收據雜湊同次保存主庫研究入口，原PNG／LOG／資產窗口不入Git。公開只交本規格／索引／337、338、364回填。原95M實際EIP22F263／flags12h、IF關閉、callback mask1／8／8與IRQ22338／22338。原338固定95M前置不可沿用；下一步依[366](366-moo2-name-ready-boundary.md)有界只讀捕捉首個自然恢復IF的可接受輸入狀態，再審查名稱ACCEPT，不改原guard或以任意時刻試到成功，主庫RE-first保持。

## 366名稱readiness回填

可寫95M IF關閉與首個自然恢復平台readiness已由[366](366-moo2-name-ready-boundary.md)核對。原95000065／EIP234A49／flags216h，完整165byte表／候選與RGB保持；原mask1／callback8／8終態已取，無名稱press／release，原338唯讀guard保持。下一步依新真實初態另立正常確認契約，按下mask1的原玩家結果／持久名稱與存檔仍未知。
