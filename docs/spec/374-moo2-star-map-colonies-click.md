# 374：正常星圖COLONIES一次裝置輸入

狀態：**CONFORMED，限定正常輸入與原選取word**
日期：2026-10-04

## 原前置與限定範圍

沿[373](373-moo2-star-map-colonies-source.md)官方1.31正常星圖180M。ORION2.EXE SHA-256 4e11be14217b4aafa1839f333bf5eba037f98b0c44e9e4752c96c464c260419f，工具f92dd15be1f5bf94d193d9bfc0367f2baa5793a8，CPU保持1d8a4d8252372c97d8e873ba13c3ab3670796527dcd6d74de52e8cbf226e068c，位址空間dosgolem_high_le。原373完整180M R／段／flags202h／FPU bits、虛擬414027099µs、VBE／RGB／表／八窗口及target8:2136D1／mask2B／callback14／14／IRQ47425／47425均已取得，pending0／inactive／非failed／IF1。

原DS188:298848 count23／stride55／1265bytes、邏輯43,450唯一命中index10矩形17,434–79,471，畫面COLONIES對應強推論。原圖及窗口hash見373；+24首byte00，不猜標籤。一次正常裝置輸入與原consumer後才判定COLONIES是否開啟，不代寫選取word／核心／RAM。

## DRAFT模式與預算

私有DOSGOLEM_MOO2_COLONIES_CLICK只接受1，依賴全部373完整可寫profiles、HOME_NAME_ACCEPT_CLICK、state目錄及日期／輸入。新模式cap固定185000000，180M以前不送新輸入，180M後固定5000000步觀察，不增加cap求通過。模式關閉維持373所有180M參數與guard；185M只能由明示新模式使用，CLI缺依賴或非法值在EXE前拒絕。原UNIVERSE_180M只是baseline階段，舊180M config保留，另外明示COLONIES後續預算；既有maxSteps gate只加新模式限定例外，可逆回373，不放寬正式工具。

180M在既有checkpoint與dumpSetupTable後取相同star_map_source_snapshot。核對完整核心／FPU／clock／VBE／RGB／globals／header／完整表及八原窗口SHA-256、callback／IRQ均對接373；一次不匹配即拒絕，不能延後挑狀態。source_ready只記私有觀察布林，不改guest狀態。

## 正常輸入與只讀consumer

一次press180000000、physical x86,y450、buttons1／delta0。沿既有橫向2:1尺度為邏輯43,450，只命中index10。待原正常INT33 AX3查詢在callback inactive時真正返回BX1／CX86／DX450，記錄完整前後R／段／flags及callsite；不假定下一RET的AX。

press callback已完成、至少20000虛擬µs、IF1、target相同、pending0／callback inactive／IRQ inactive且非failed／started=completed時，首次安全release physical x88,y450、buttons0／delta0，邏輯44,450仍在index10內。只送一次，不重按或注入鍵盤；沒有polled或safe release就如實留下未完成與原停點。

只讀保存首個原20DDDB shared word store與polled後最多三筆214104 RET。原word地址DS188:26C4A6、code16／SS:ESP stack16、完整前後R／段／flags／FPU bits與callback／IRQ保存。store結果按原AX及原bytes核算，RET按真實stack核算EIP／ESP+4，不預填AX10或名為持久欄位。未命中不猜consumer，畫面另以實際終PNG驗證。五百萬步內遇新CPU拒絕即保存，不開硬體driver／renderer／runtime考古。

## READY、驗收與權利

直接審查373完整前置及八窗口、原點唯一矩形、舊cap guard與原input API，轉READY後才新增私有模式。逆轉全部新patch等於373；全部公開internal／CPU／DOS／probe保持f92dd15，沒有新CPU行為，沿372固定官方EXE全套，另建置本probe。

373到180M source snapshot為止共通原列／39PNG保持，只正規化兩處maxSteps budget宣告為已驗180M baseline，完整新cap185M另核對；mtime／DTA及每輪RAMhash沿既有契約，IMUL A2仍保持changed_bytes與hash是否相等。舊terminal rows只屬180M cap，不混入新輸入後比較。

原guest一次、全部舊正常輸入／1996日期及可寫state保持，原418來源RO前後hash、SAVE10.GAM／MOX.SET／sound.lbx終態與同guest有界副本及UID GID核對；state新增或變化按實際收據，不要求永遠相同。日期不是RNG seed。

Go1.24.13 image sha256:1a6d4452c65dea36aac2e2d606b01b4a029ec90cc1ae53890540ce6173ea77ac；Docker network none／600s／2GiB／2CPU／128pids／UID1000，原ZIP／patch唯讀，owned監測550s／trap收尾。私有入口workplace/new-game-374-ready-review.py、new-game-374-run.sh、new-game-374-verify.py；公開本規格／000-index與373回填，原probe／LOG／PNG／RAM／state不入Git，收據連主庫既有docs/re/dosgolem-moo2-intake-20260930.md。主庫玩法RE-first保持；COLONIES內容與正常player path、正式存讀、完整開局及remake同狀態按實際結果分開。

## READY審查

原373完整180M來源、八window hashes、唯一press／release矩形及target／IF1／callback／IRQ已直接核對，CPU hash及原InjectMouseEvent API保持。新模式唯一185M與固定5M後續預算、原AX3後首安全release与有界原store／RET契約審查通過；原結果仍未知。轉READY後才修改私有probe，不改公開internal／主庫玩法。

## CONFORMED限定結果

狀態：**CONFORMED，限定正常COLONIES座標輸入與原shared選取word**。殖民地列表畫面尚未驗，原固定5M觀察終圖為黑。

180000000原373完整前置與八window hashes一次通過，source_ready／valid true，target8:2136D1、mask2B、pending0／inactive、callback14／14與IRQ47425／47425完成。正常press x86,y450／buttons1、虛擬414027099µs。原180010886、dosgolem_high_le:24C31B的INT33 AX3返回BX1／CX86／DX450，段與flags16h保持。180010921／414069959µs首次安全release x88,y450／buttons0，持按42860µs；IF1、mask1、callback15／15、IRQ47428／47428完成且inactive，沒有重送。

原180019489、214104 C3依SS188:ESP2BD44C真實stack返回20DB5B，ESP+4、AX0／flags246h保持。原180020238、20DDDB六byte66A3A6C42600真正寫DS188:26C4A6 word0000→0A00，EIP20DDE1，R=[A 0 226 FFFFFFFF 2BD450 2BD4B8 FFFFFFFF 2BDC2C]、段／flags202h／FPU保持；原選取10已證實，不稱持久欄位。第二RET180144100由214104返回174742，ESP2BD3A0→2BD3A4、AX0／flags246h及word0A00保持。兩RET按原stack核算，不預填AX10。三consumer observer只讀、原code／stack／word與既有raw32候選窗口前後保存，raw窗口保持不推定新列表資料；最多三RET上限只命中兩筆。

實際step_limit=185000000／EIP223A71／unique_sites55872，無guest_cpu_stop／step_error／dos_exit。R=[0 0 0 0 2BD720 2BD980 0 2BDC2C]、段=[8 188 188 0 20 188]、flags206h／IF1、FPU127F／status0／depth0／八stack bits0，虛擬424485517µs，callback16／16、IRQ48857／48857完成且inactive／非failed。colonies_terminal pressed／released／polled／store_seen true、returns2。

終圖已親看全黑，PNG SHA-256 1610444d26adb3135e7e933dd44912044bb636728af59c70e614945278d3c622。RGB SHA-256 0b150fd32588b1daca5569992ebe559c0102c837306b1af4c44d35128ec58366等於921600個零byte；indexed 307200bytes SHA-256 4d46c5beedd237ddba268a74a01d8c33a4a0323fb52213a2e5d5b6ea4d688d43，與全零indexed的7818f5542a0404157573be6cffc0e0c8e68ce3c0f5d17d07ccdd9313fb700baf不同。已證實索引資料非全零、輸出RGB全零；色盤與轉頁時序未知，不能稱為殖民地列表已正常開啟，也未證產品缺陷。VBE bank4／startY0／sets2941／writes55575758／display94。

原終header count20／stride55／pointer298848，舊count≤16觀察器不擷取，records空；1100byte完整新表未取，既有cap snapshot入口在185M未命中10M取樣條件。這是觀察範圍，不是原表read失敗。舊menuFrame／return定位仍屬較早caller，不能套用到目前223A71。只保存末ring及目前完整核心，不命名renderer helper或追DAC driver。

## 驗證、回填與下一步

373正常COLONIES輸入前11981共通原列與39PNG保持；明示新colonies_continuation_config baseline180M／cap185M／window5M，只有universe_continuation_config與hardware_keyboard_schedule兩處maxSteps在比較前歸回180M baseline。新實際185M另按原收據核對，沒有忽略新預算。舊180M cap的terminal rows不混入新輸入後比較。mtime／DTA／每輪RAMhash沿既有契約，IMUL ram_effect保留changed_bytes與hash是否相等。

八私有patch逆轉後精確等於373，所有公開internal／CPU／DOS／probe保持f92dd15。13CLI拒絕與新mode on185M／舊mode off180M home on／off三正對照通過；185M只能由新模式完整依賴使用，mode off仍拒絕185M。沒有新CPU行為，沿372固定官方EXE全套，另建置本probe，不重跑無關CPU測試。

初版原輸入驗證通過；擴充終態檢查時沿用非空field parser不能讀records=空而失敗。改用明示空欄位判準後同收據通過，首失敗另留new-game-374-tests-terminal-parser-first-failure.txt，沒有native重跑、改結果或放寬guard。

原guest一次、原418來源前後保持。SAVE10.GAM208000bytes／0cf71fc5368861758da5a5dffbd791f8b711a48cfd295572eb9a21d27dea0f4d、MOX.SET553bytes／de8554b3284c9f9065efe6e4f2dfa71ab0c7e732e75eaea0d0ecf465ab86409f、sound.lbx4250888bytes保持，終態與373一致；同guest有界副本與終態／UID GID1000核對，owned監測550s／outer600s／trap收尾，相關Docker容器清理。固定日期不是seed。

| 不可變鍵 | 新語意與等級 | 舊規格 | 回填 |
|---|---|---|---|
| 官方1.31／DS188:298848 index10／180M；20DDDB／66A3A6C42600 | 正常座標press／原poll／release與選取word10已證實；列表未驗 | 372、373、367、371 | 同shared writer新星圖上下文，保留其他點擊歷史與未驗持久語意 |

372／373／367／371同次追加回填，索引及回填由私有驗證核對；原正常輸入與新窗口不外推到其他玩家路徑。主庫紀錄仍沿既有研究入口。

下一步保持本185M與相同輸入，先取目前20物件完整1100byte表、code16／當前SS:ESP stack16及有界索引使用集合／RGB對應。沿既有internal/machine/moo2_vbe_video.go的VBEIndexed／VBERGB只讀API及既有dumpSetupTable count≤64入口；另審查185M固定取樣條件，不放寬舊count≤16 guard。依來源判斷正常轉頁邊界，不盲目擴cap求過，不深入DAC／PIT／driver或renderer helper。主庫玩法RE-first保持，殖民地列表內容／正常操作／正式存讀語意／完整開局／RNG與remake同狀態未驗。
