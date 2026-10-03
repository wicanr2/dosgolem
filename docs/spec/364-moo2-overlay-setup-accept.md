# 364：可寫覆蓋層設定頁 ACCEPT 正常輸入

狀態：**CONFORMED，限定隔離可寫ACCEPT與90M選族頁；完整可寫玩家路徑未驗**
日期：2026-10-04

## 範圍與來源

沿[363](363-moo2-overlay-startup-and-setup-source.md)與[336](336-moo2-setup-accept-normal-click.md)，只處理既有可寫overlay在80M的正常ACCEPT與90M選族頁蒐證。這是原版執行器的輸入驗證，不改主庫玩法，不預定存檔或完整開局成功。主庫RE-first保持。

固定官方1.31 ORION2.EXE SHA-256 4e11be14217b4aafa1839f333bf5eba037f98b0c44e9e4752c96c464c260419f，原ZIP417根檔另加官方EXE共418檔，原MOX.SET／1996-01-01／完整180M參數及44M Esc與已有正常NEW GAME／50M選單點擊。1996不是seed。原位址為dosgolem_high_le。固定公開工具04a96f09538a6b01907685f6f56a8d7de154cde1；overlay沿362本機試作及原OpenDirectoryOverlayFiles，state須既有空目錄，來源唯讀。

## 已證實初態與資料

363原overlay收據72f5cc29637de198914526676953f53c99fb333c20f5dafa449d921fe7c11811，80M原表pointer298848／count17／stride55／bias0，header11000000000000000000000000000000，完整935bytes SHA-256 49374b4c6dfd2d1d8231cfc137e1b5b7d86c7ec49f6fdf3be7417480d0da948b。RGB SHA-256 3bbe6c339cfbc74e84b3210bccfdc4574073b4d308ef9b90a1720cc5c74e623e。record15幾何433,392–527,414完整保持336且親看原640×480畫面確認ACCEPT；480,400與481,400均只落此有效矩形。

原80M EIP2349D5／R=[0 C 3DC66D 29BE7C 2BD870 2BD8D4 3DC66D 366276]／段=[8 188 188 0 20 188]／flags246h；FPU127F／status0／depth0／八stack bits0。callback mask2Bh、started4／completed4、pending0／非活動；IRQ17901／17901非活動且非failed，IF已開。原ready press／release及選擇store已完成。五個+44 raw值004B7A3C／004B8D14／004B6634／004BA084／004BB294，原DS188 descriptor所指128byte各可讀；完整值與128byte SHA須逐項核對363，不由NUL截斷。完整物件／角色／consumer未知，不列為此次輸入必須補完的allocator或renderer考古。

## 預定工具契約

新明示私有旗標DOSGOLEM_MOO2_OVERLAY_SETUP_ACCEPT_PROFILE=1，其他值或缺完整180M／game-dir／SAVE_STATE_DIR依賴在讀EXE前拒絕。未啟用時不改原336的唯讀表hash guard。只在獨立profile中使用已觀察完整overlay935byte hash；保留原輸入可用性、callback／IRQ／IF、ready來源、DS／header／pointer／count、完整RGB及只讀檢查，另核對五個實際raw值與128byte SHA和完整80M CPU／FPU／IRQ初態。任何不符停止，禁止mask欄位、任意加位移、調點擊時刻或重送。

80M固定一次InjectMouseEvent x960／y400／buttons1，原callback映射480,400；同正常available條件、press回呼至少完成一次、虛擬時間至少20000µs後首次送release x962／y400／buttons0，映射481,400。維持原20DDDB／66A3A6C42600的選擇store觀察，只保存首筆，不代寫DS188:26C4A6。

90M先保存既有正常原表／R／段／flags／FPU／callback／IRQ／RAM前後只讀及原PNG，再明示diagnostic stop，不送RACE_HUMANS或後續輸入。解析180M參數保持，但exit0不能稱180M完成。原store未命中或原GUI未轉移皆據實保存，不調參數追成功。

## 驗收、未知與停止線

- 前80M保持363 overlay各自6271列的352既有mtime／DTA／每輪診斷RAMhash正規化比較，既有27PNG逐byte保持；新diagnostic使用獨立前綴，原336唯讀guard仍可回查。
- 對私有profile必要依賴及無效值做最小CLI檢查，核對原80M guards與五窗口拒絕分支；公開CPU／DOS／provider／probe保持，私有變更可逆為362 prototype。
- 原guest一次正常ACCEPT，保存實際press／release、virtual micros、真正store前後與原bytes及原選族頁快照。原90M完整16筆若可讀只蒐證，不猜新Humans guard。
- 原418檔guest前後逐檔SHA-256保持；state清單／大小／雜湊／擁有權留本機。原資料／LOG／PNG／資產窗口不公開，公開只交自製規格、索引及證據雜湊。
- 完整存檔／RNG／音訊人耳／整體開局／remake同狀態未知。正常ACCEPT不外推後續正常選族，也不把工程測試當玩法parity。

Docker沿Go1.24.13映像sha256:1a6d4452c65dea36aac2e2d606b01b4a029ec90cc1ae53890540ce6173ea77ac；600s／2GiB／2CPU／128pids／UID1000／network none；原ZIP及patch唯讀。私有入口workplace/moo2-overlay-accept-364.go／new-game-364-run.sh，規格入口同次掛000-index，後續審查與收據掛主庫研究入口docs/re/dosgolem-moo2-intake-20260930.md。

## READY證據審查

以同363原guest收據執行workplace/new-game-364-ready-review.py通過。完整935bytes與header／RGB、五個真raw值及128byte SHA、完整R／六段／EIP／flags／FPU、callback4／4與IRQ17901／17901均吻合；原ACCEPT幾何唯一匹配480,400。沒有新guest或重選資料。證據足夠描述本輪正常輸入與有限觀察，轉READY；完整物件未知不影響已讀原幾何與既有正常InjectMouseEvent契約。這只允許忽略workplace中的隔離profile，不接公開玩家path，主庫玩法閘門保持。

## 限定CONFORMED：隔離可寫ACCEPT與選族頁

**已證實**：READY後同一原guest只送一次press／release。80M press virtual_micros133462143／x960 y400 buttons1；原80013765於133507499放開x962 y400 buttons0，差45356µs，正常回呼5／5完成後首次可送。原20DDDB於80119128執行六byte指令66A3A6C42600，DS188:26C4A6 word0000→0F00，下一EIP20DDE1，原R=[F 0 339 0 2BDB14 2BDB7C 0 2B0001]／段=[8 188 188 0 20 188]／flags297h保持，callback6／6，IRQ17934／17934非活動且非failed，error=nil。16byte觀察窗口包含六byte指令及後續bytes，核對只取此指令六byte，不把整窗口誤稱一條指令。

90M原SELECT RACE選族頁，親看dosgolem原PNG確認十四個種族／Custom按鈕。RGB 9d8c0a1acb3b96200296f789bb13c6877067f6165ec608a7e019506036832dfc及PNG 7aec4ca6aad1f948560e184695bd3b415b1145ad9778e536da14a60e11d61861與既有336原選族頁相同，並非已選Custom或任何種族。原表16筆／pointer298848／stride55／bias0，完整880bytes SHA-256 eca22108024dbc103f8de8257b32321b0b2e39fcd6bb8435cb4dc0ba73e29396，較唯讀336的表不同，下一正常Humans輸入須獨立審查。此處明示diagnostic stop，沒有race press／release，不稱180M完成。

**驗證**：前80M除363新增八早期診斷及原precondition自身，6270共通原列依352既有正規化保持，27PNG逐byte保持。80M所有原precondition欄位保持，valid只由獨立profile真實完整表／五窗口／CPU／FPU／IRQ檢查變true，原336唯讀guard仍false、表hash不改。三個私有區塊逆轉為362原試作，公開CPU／DOS／provider／probe及全部internal不變。四個無效值／缺依賴CLI在讀EXE前exit2，合法值越過閘門後缺EXE明確失敗，未混稱原guest成功。初次產生腳本括號筆誤在啟動guest前修正，未重跑原guest。

兩側資料邊界保持：本輪原418來源檔逐檔SHA-256不變；state清單與363完全相同，只有sound.lbx4250888bytes，與原ZIP相同，UID及GID1000。沒有SAVE10.GAM state。完整開局／存檔成功／RNG／音訊與remake同狀態未驗。

### 命令與私有收據

```text
python3 workplace/new-game-364-ready-review.py
bash workplace/new-game-364-run.sh
python3 workplace/new-game-364-verify.py
```

以上均在既有Docker內執行並通過，各項欄位與所有舊PNG均直接核對同一次guest收據。原版圖片／LOG／資產與state不公開，私有11份收據雜湊同次記入主庫研究入口。公開只交本規格／索引／336、362、363回填。下一步依本輪已保存原90M全表及既有337正常輸入契約，另立可寫Humans profile；不改舊唯讀race guard、不mask位址或調輸入時刻。主庫RE-first保持。
