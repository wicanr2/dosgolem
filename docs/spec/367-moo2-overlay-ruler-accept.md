# 367：可寫名稱頁的正常ACCEPT確認

狀態：**CONFORMED，限定隔離正常名稱確認與99M旗色頁**
日期：2026-10-04

## 範圍與原來源

沿[366](366-moo2-name-ready-boundary.md)首個自然readiness與[338](338-moo2-ruler-name-normal-confirmation.md)正常名稱確認。本輪只驗隔離profile保留Strader、一次正常ACCEPT與99M後續畫面，不改正式預設／typed名稱或主庫玩法。持久名稱writer／旗色／存檔與完整開局仍未知，主庫RE-first保持。

官方1.31 ORION2.EXE SHA-256 4e11be14217b4aafa1839f333bf5eba037f98b0c44e9e4752c96c464c260419f，原ZIP417根檔另加官方EXE共418檔，MOX.SET／1996-01-01／180M參數及44M Esc、兩組NEW GAME、80M ACCEPT與90M Humans保持。日期不是seed；原位址dosgolem_high_le。起始公開工具94b15847606ff4c90635ba90d2c38388a86a2535，可寫overlay沿362試作與365私有profile，公開CPU／DOS／provider／probe保持。

366原收據6e1eff8cb6b876047130faf0388c575a38096c58b2304c9315c7968212e9707d已保存95000065／EIP234A49／R=[3 1 210160 2BD8C8 2BD8DC 2BD8E4 2843A5 28439D]／段=[8 188 188 0 20 188]／flags216h，FPU127F／status0／depth0／八stack bits0，VBE Active／Bank4／StartY0／DisplaySets48。95M flags12h／IF關閉未送輸入，原程式自然65指令後到首個ready。原target8:2136D1與IRQ非活動／非failed由ready判定核對，callback mask1／pending0／非活動／8／8另由同guest只讀終態核對；終點精確IRQ計數未知，不能猜22338。

## 資料與預定輸入契約

原pointer298848／count3／bias0／stride55，完整165bytes SHA-256 db67c471ba6a34a66146e083662f4c134c192e2371a807ec9647327558a7fd1b。完整globals192byte／header16byte沿366原收據核對hash；index2+24原候選28439D的32byte為Strader補零。RGB2646a7ef25939869ede60aa35bcd97d649b906fd2cc287d08deb6c2d8058cc12，95M原PNG與338相同。index1原矩形273,225–371,253，320,239與321,239唯一命中ACCEPT。完整物件／正式名稱writer不由候選字串推定。

私有DOSGOLEM_MOO2_OVERLAY_RULER_ACCEPT_PROFILE=1只接受值1與完整可寫Humans、RULER_NAME_ACCEPT_CLICK及180M game-dir空state依賴；無效值／缺依賴在讀EXE前拒絕。模式關閉保留365在95M停止，原338唯讀guard不改。啟用時95M先保存原前置與候選，原CPU.Step繼續，不改IF、不skip原指令；固定95000065核對完整CPU／FPU／globals／header／165byte表／32byte候選／RGB與原IF、target、mask1、pending0／非活動／callback8／8、IRQ非活動非failed且started=completed。任何不符即停，不挑後續時點或放寬欄位。

沿338座標，一次InjectMouseEvent x640／y239／buttons1，映射320,239；由目前游標位置改變與左鍵按下產生flags3，3&mask1=1，可排既有平台回呼。這只證平台契約，原ACCEPT結果仍須實測。原至少20000µs、正常回呼完成一次後首次可送放開x642／y239／buttons0，映射321,239，沿同target／IF／IRQ／pending安全條件與mask1或2B。原20DDDB六byte66A3A6C42600共享store若命中只保存首筆；未命中明示未知，不代寫名稱／選擇／EIP／RAM。

99M先保存原後續GUI、VBE／RGB／PNG、完整CPU／FPU、globals／header與實際count≤64的完整55byte表、callback／IRQ與整RAM前後只讀，再明示diagnostic stop，不送BANNER_RED。成功／失敗按實際收據，未放開或頁面未轉移也保存；解析180M參數保持，但99M stop不稱180M完成。

## 驗收與入口

366額外輸入前的原95M資料／核心完整對接，365前95M全部共通列及29PNG依352既有mtime／DTA／每輪只讀RAMhash正規化保持。私有變更可逆為365，公開internal／CPU／DOS／provider／probe保持。四個profile無效值／缺依賴與合法值做最小CLI核對。原guest一次，不重跑挑結果；原418檔前後SHA-256保持，state清單／大小／雜湊／UID／GID核對。

原資料→只讀表／候選→正常輸入／原consumer→後續UI；正式持久名稱／存檔／typed種族／旗色／RNG／音訊與remake同狀態不外推完成。不深入allocator／renderer／runtime helper，原PNG／LOG／RAM與state不公開，只交自製規格／索引／雜湊。

Docker沿Go1.24.13映像sha256:1a6d4452c65dea36aac2e2d606b01b4a029ec90cc1ae53890540ce6173ea77ac，600s／2GiB／2CPU／128pids／UID1000／network none，原ZIP／patch唯讀。私有入口workplace/moo2-ruler-accept-367.go／new-game-367-run.sh，公開同次掛000-index，收據掛主庫docs/re/dosgolem-moo2-intake-20260930.md。

## READY審查

直接核對366同guest首個readiness原收據，workplace/new-game-367-ready-review.py通過。完整CPU／FPU／IF、165byte表、唯一ACCEPT矩形／候選／RGB，以及同guestmask1／callback8終態吻合；globals SHA-256 bf5c3bdbaf0fc25ba949ce5518e4389e8886eb56336228b948e9fbfa414111d2，header SHA-256 1eb307e414824282e81d17faf0ac5ab6752ae0f074ecd526f3cb4ce16847011c，32byte候選SHA-256 75046c4161f488b190cf445f0778471a5e4ed277c03286e9bd136eccbd8f2276。既有平台flags3&mask1回呼契約足以允許一次可丟棄正常輸入驗證，不預定原名稱結果；固定95000065只取原已驗首個自然readiness，不調時刻試到成功。IRQ精確計數不猜，送前實讀並核對非活動／非failed及started=completed。此READY只允許隔離profile，原338唯讀guard與主庫玩法RE-first保持。

## CONFORMED限定驗收與勘誤

狀態：**CONFORMED，限定隔離預設名稱ACCEPT與原旗色頁**。已證實原95000065完整前置通過，95M未送名稱輸入；原338唯讀guard false保持，由獨立profile核對原CPU／FPU與完整表。原callback mask1、8／8、IRQ22338／22338非活動且非failed；一次press x640／y239／buttons1，175812740µs，放開95013578／175858124µs／x642／y239／buttons0。相差45384µs，放開前原正常callback9／9已完成，不補寫guest IF、RAM或選擇。

已證實原95008897、dosgolem_high_le:20DDDB六byte66A3A6C42600真正執行，下一EIP20DDE1，DS188:26C4A6 word0000→0100；原R／六段／flags297h與32byte Strader候選保持，callback9／9、IRQ22340／22340非活動且非failed。這只證名稱確認的共享GUI選取欄位，不證正式名稱持久writer。

已證實原99M的640×480 SELECT BANNER COLOR親看，實際count10／stride55完整550bytes可讀、完整CPU／FPU與RAM前後只讀保持，RGB8c2c573c08ebf4bce0b4e166d4268fb6fcf90f35dd27fbbb2de614b26f187f15。原EIP23857C、R=[347B20 0 4B 1 2BD9B8 2BDA14 6AE528 347B6C]、六段=[8 188 188 0 20 188]、flags206h，FPU127F／status0／depth0／八stack bits0，callback mask2B／10／10、IRQ23503／23503非活動且非failed。原339唯讀旗色guard false保持，先保存真實初態即99M diagnostic stop，沒有送BANNER_RED。旗色正常選擇另立契約，不靠換hash或時刻放寬舊guard。

首次執行因生成器把99M診斷區塊插入50M menu分支，在名稱輸入前提早正常diagnostic return；這是驗證腳本定位錯誤。首次私有Go／替換／run／CLI／原LOG／state／24PNG共32檔保留原樣，new-game-367-first-receipts.json逐份SHA-256與UID／GID核對。改以唯一執行期旗色條件定位並核對99M與完整550byte取樣，再用同Docker／參數乾淨重跑；正常名稱輸入只在修正後guest送一次，沒有重擲名稱結果。首次不登錄為99M或玩家路徑完成。

驗收腳本首次把95M後的額外自然INT33h紀錄納入舊365的停止前綴，列數7662／7663；兩側共同95M候選快照之前皆相同。改以唯一共同快照終點核對7662列，沒有改guest／重跑。365前95M的7662共通列依352既有mtime／DTA／每輪只讀RAMhash正規化、29PNG逐byte保持；366首個95000065的CPU／完整表／globals／header／候選／RGB保持。原418來源前後SHA-256保持，state同366僅sound.lbx4250888bytes／原ZIP同bytes／UID與GID1000，沒有SAVE10.GAM。四CLI拒絕與合法缺EXE正對照通過；九私有替換可逆為365，公開internal／CPU／DOS／provider／probe保持。

原資料→完整表／候選→正常名稱press／release→原選取store→旗色UI已驗。未知：typed名稱、正式名稱持久writer／存檔／旗色選取／完整開局／RNG／音訊與remake同狀態。1996固定日期不是seed，180M只是解析參數；本輪99M停止不稱180M完成。主庫玩法RE-first保持。

## 跨規格回填台帳

| 不可變鍵 | 本輪語意與等級 | 舊規格 | 回填 |
|---|---|---|---|
| 官方1.31 EXE／dosgolem_high_le:20DDDB／66A3A6C42600、DS188:26C4A6 | 可寫預設名稱ACCEPT共享選取store，已證實word0000→0100 | 338 | 唯讀338未命中仍有效；可寫367正常命中，持久writer未知 |
| 官方1.31 EXE／dosgolem_high_le:234A49／95000065 | mask1首個自然ready後正常名稱確認，已證實 | 365、366 | 原各輪未送輸入仍有效；後續正常press／release與旗色UI由367限定接通 |

三份舊spec同次附367回填；私有驗證檢查精確位址／bytes／舊檔存在與所需回填，缺項即失敗。正式玩法與其他唯讀分支不外推。

## 收據與重現

私有入口：

```text
python3 workplace/new-game-367-ready-review.py
bash workplace/new-game-367-run.sh
python3 workplace/new-game-367-verify.py
```

既有Docker執行，READY審查先於profile實作，修正後正常guest一次。首次錯置收據與修正後收據均連到主庫研究入口；原PNG／LOG／RAM與版權資料只留忽略workplace，不入Git。下一步以99M真實完整550byte表／CPU／FPU／RGB／callback與IRQ，先審查獨立正常紅旗press／poll／GUI selection／release契約，保留339唯讀guard。

## 368可寫紅旗正常輸入與寫檔回填

[368-moo2-overlay-banner-red](368-moo2-overlay-banner-red.md)已驗可寫99M真實完整550byte表後一次正常press，原24C31B pressed查詢、214104 RET20DB5B／20E165返回AX1後release，原160M生成UI親看。原237024的SAVE10.GAM 3D01返回handle9／CF0，237093 truncate及238310共208000bytes正常寫入並close，隔離state的SAVE10.GAM與MOX.SET副本hash已核對。362唯讀拒絕與339／341原輸入收據仍有效，不外推可寫same-state。367原99M未送旗色的歷史保持，後續由368限定接通。原169E49／20D0的新CPU拒絕尚未修正；旗色正式持久語意、讀檔、180M與完整開局未驗，主庫RE-first保持。

## 369 byte AND能力與原續行回填

本文件原有收據與驗收範圍保持。官方1.31／dosgolem_high_le:169E49／20D0的368 CPU拒絕，已由[369](369-cpu386-and-byte-register-source.md)解出：原AND、下一POP ECX及RET依真實stack通過，相同輸入實際達180M，無新CPU停止。AF清0只屬工具模型。終態是母星命名視窗，尚未正常確認；不把此新續行當舊唯讀same-state、正式讀檔或完整開局。

## 371正常母星名稱確認回填

[371](371-moo2-home-name-normal-accept.md)已沿370原170M完整前置一次正常press，原24C31B查詢讀到BX1／CX550／DX260，首次安全release後，170024419原20DDDB／66A3A6C42600真正寫DS188:26C4A6 word0000→0100；32byteSol候選保持，命名視窗消失、回到星圖已親看。這是母星確認上下文，統治者名稱與正式持久writer不由共享buffer／store推定。後續174213914於1749C0／F6EC遇新CPU拒絕，未達180M／完整開局；原CPU與本文件舊收據保持。
