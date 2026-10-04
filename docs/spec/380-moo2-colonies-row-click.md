# 380：Sol II列表行的一次正常裝置輸入

狀態：**CONFORMED，限定正常行輸入與原選取13**
日期：2026-10-04

## 原來源與玩家範圍

[379](379-moo2-colonies-row-source.md)核對了378的195M可見列表及原第一命中順序。官方1.31 ORION2.EXE SHA-256 4e11be14217b4aafa1839f333bf5eba037f98b0c44e9e4752c96c464c260419f，工具c6d319edf0f8a8bacfdc1a53d2eb39a5a53205c1，CPU SHA-256 736d95e801e8e9658078671af8148a79896d805cc7a8baa42f8881574e562080。原195M source為dosgolem_high_le，IDA比較／first-match來源另標IDA linear EA，不混同。

原195M EIP22C8BA、R=[264C6C 4C 3E7D8BA 26A98C 2BD4E0 2BD50C 5 1]、段=[8 188 188 0 20 188]、flags283h／IF1、FPU127F／status0／depth0／八stack bits0、clock447368391µs。原完整20表DS188:298848／stride55／1100bytes hash3c6bd2afa22a4ac6500ce62df53ea9bfe88dd4713423922949d23f179820471c，RGB hash4ad732442e76257d2d74a475026832ed4e6eb14bc5a9faffb63ac9a49211fea1、終PNG d3c775f1b8594e8313c27b5bd9e363dfb6e3210ca05f0f17136b36409b7a67ba。

raw index13矩形12,35–101,65／type7；logical43,48與44,48雖同時命中19，原按索引先取13。Sol II名稱區對應原為強推論，正常pressed poll、共享store與畫面結果尚未驗。本規格只驗一次正常行點擊，不預設單／雙擊語意、不改人口或正式存讀、不實作主庫玩法。

## DRAFT裝置輸入與預算

新private mode DOSGOLEM_MOO2_COLONIES_ROW_CLICK只接受1，要求完整restoreContinue／COLONIES／HOME／BANNER profile、原MAX_STEPS185M guard與可寫state；缺任一即在EXE前拒絕。mode off維持378全部195M，mode on原runSteps195M後明示200M，195M後固定5M作新玩家輸入觀察；不是重啟或逐次加cap求過。

原正常guest只跑一次，所有既有輸入／185M前置／378四consumer／DAC序列及39frames保持。195M第一個既有loop邊界先只讀保存完整frame／20表／PNG、完整24601 DAC前綴及callback IRQ；match失敗立即拒絕，不挑後面時點。來源guard固定上述完整核心／FPU／clock、table與RGB／PNG hash，以及target8:2136D1／mask2B／pending0／inactive、callback16／16、IRQ51746／51746 inactive／非failed。不從舊caller的stack猜目前frame。

前置通過後用既有InjectMouseEvent一次physical86,48／buttons1／delta0,0。原INT33 AX3真的返回BX1／CX86／DX48、callback非active且handled後才標pressed poll。按下後至少20000µs、callback已返回且pending0／IF1／IRQ非active及非failed，允許原mask2B或既有mask1，在首個安全邊界一次physical88,48／buttons0／delta0,0。不重送、不跳handler、不寫guest RAM／核心／選取結果。裝置輸入例外與source observer的只讀性分開。

## 原consumer與畫面收據

按下後只讀觀察第一個dosgolem_high_le:20DDDB／66A3A6C42600原store及最多三個214104／C3 RET。每項保存指令前／後完整R／段／flags／FPU bits、code16、真實SS:ESP stack16、共享word原bytes、callback IRQ、只讀取樣前後RAM保護；原writer對RAM的正常效果不當observer改寫。RET按原stack與ESP+4核算，原return AX按實際記錄，不預填13；store按原AX低word與DS188:26C4A6核算，預期13須自然命中才已證實。沿既有raw32候選窗口只保存，仍不當typed資料。

195M baseline-row來源與後續終點／新PNG分開，完整DAC序列獨立重播到實際終態並獨立PNG解碼與palette histogram映色。原reason step_limit200M、CPU stop或DOS exit按實際保存；probe exit0不作成功證據。首恢復既有快照不冒稱新點擊結果；新的final附row_input_sent，不能仍假稱無新輸入。

原418來源與state、同guest副本及UID1000按實際核對，不預填存檔相同或持久欄位語意。列表行source→正常裝置輸入→原poll／writer／RET→實際UI→state影響為本輪垂直鏈；沒有正常UI或原writer時只報限定收據，不稱remake對拍完成。

## READY與驗收

核對379來源驗證、固定195M完整前置、原第一命中與真實store／RET、既有device API／mask／安全release方式後才READY並實作private probe。所有新增patch逆轉精確等於378，public internal／CPU／DOS／原probe保持c6d319e；沿378固定官方EXE CPU386／machine全套，不重跑無關測試。舊18CLI拒絕4正對照保持；新mode錯值／缺依賴與錯baseline拒絕，mode on／off合法缺EXE正對照分開。

378共通原列至195M實際點擊前保存；原舊terminal與其後總量不拿來比新的200M。新mode config另核對，逐byte比39frames、185M／首恢復PNG及195M baseline-row PNG，完整185M與195M DAC前綴核對。每輪RAMhash與mtime／DTA沿既有正規化，RAM changed_bytes及前後hash相等關係保持。實作發現未知回到來源，不猜玩法。

Go1.24.13 image sha256:1a6d4452c65dea36aac2e2d606b01b4a029ec90cc1ae53890540ce6173ea77ac；network none／600s／2GiB／2CPU／128pids／UID1000，原ZIP／patch唯讀，owned監測550s／trap收尾。private入口workplace/new-game-380-ready-review.py、moo2-colonies-row-380.go、new-game-380-run.sh、new-game-380-verify.py。公開本規格／000-index與379回填，原LOG／PNG／RAM／journal／state／probe在忽略workplace，收據連主庫既有docs/re/dosgolem-moo2-intake-20260930.md。

主庫玩法RE-first保持；人口調整／正式存讀／完整開局／RNG與remake同狀態未驗，固定日期不是seed。

## READY審查

379完整195M與IDA來源、原API／mask與20ms安全release、明示一次200M／5M後續預算及有界原store／RET審查通過，轉READY後才修改private probe。原點擊結果保持未知。

## CONFORMED限定結果

狀態：**CONFORMED，限定正常Sol II行press／poll／release與原選取13**。殖民地畫面仍未驗，200M終圖為黑，不稱完整行操作或開局完成。

原378共通12701列至195M保持，只有新row config與195M來源標記另核對。完整185M baseline及39frames、首恢復PNG與24601 DAC前綴保持；新195M baseline-row PNG逐byte等於378可見列表終圖，完整core／FPU／clock／VBE／table／code／stack與device來源保持。沒有拿不同終點的callback18或DAC35876稱195M同狀態。

195000000／447368391µs source_ready／valid／readonly true，target8:2136D1／mask2B／pending0／inactive、callback16／16與IRQ51746／51746已返回。一次press physical86,48／buttons1／delta0,0；原195216883在24C31B的INT33 AX3返回BX1／CX86／DX48，段與flags16h保持。195216918／447838123µs首安全release physical88,48／buttons0，持按469732µs；mask1／pending0／IF1、callback17／17與IRQ51807／51807完成且inactive。只送此一次press與release，不注入結果。

原195225486、dosgolem_high_le:214104 C3按SS188:ESP2BD920原stack首word5BDB2000返回20DB5B，ESP+4、AX0／flags246h與其它核心／FPU保持，word0000保持。原195226311在20DDDB／66A3A6C42600真正寫DS188:26C4A6 word0000→0D00，EIP20DDE1；R=[D 0 2CB 400 2BD924 2BD98C 0 2BDC2C]、flags297h及完整其它核心／FPU保持。原store13已證實，Sol II名稱區的正常選取鏈已驗，不把13當typed colony id或持久欄位。raw32候選窗口仍為原Sol字串／原bytes，保持不命名正式資料。最多三RET只命中一筆，不預填額外返回。

實際step_limit200000000／EIP223A23／unique_sites60185，無guest_cpu_stop／step_error／dos_exit；完整35876 DAC事件獨立重播至實際末態，三埠3C6／3C8／3C9計36／8960／26880筆。新點擊後11275事件為11輪maskFF／index0..255／768色值，從196292723／450713469µs開始，逐component單調不增；首全0與末write同在196376553／450990327µs、device sequence315113、當前EIP222D1C。200M前未恢復色彩；只稱本收據降色，不追DAC／PIT／driver逐週期。

終indexed307200bytes全0、DAC768bytes全0／maskFF、RGB921600bytes全0，獨立PNG filter／CRC／RGB hash及palette histogram映色核對。PNG親看全黑，hash1610444d26adb3135e7e933dd44912044bb636728af59c70e614945278d3c622。VBE bank5／startY512／sets3031／writes57388844／display97，與195M分開記錄。原表DS188:298848 count1／bias0／stride55、record0的55bytes全0，hash02779466cdec163811d078815c633f21901413081449002f24aa3e80f0b88ef7；不是舊20表不變，也不猜新的正式控制項。

原200M R=[2A375C 0 A4 15 2BD908 2BDB68 30E76A57 2B0000]、段[8 188 188 0 20 188]、flags202h／IF1、FPU127F／status0／depth0／八stack bits0、clock458239660µs，callback8:2136D1／mask2B／pending0／inactive、18／18與IRQ53180／53180完成且非failed。當前code16=0345A88A0025FF000000C1E0028A805A，SS188:ESP2BD908 stack16=F3A6210000010000AE01000000010000；只保存定位，不把首stack值21A6F3當已證實caller。

14private patches逆轉精確回378，所有公開internal／CPU／DOS／原probe保持c6d319e。18舊CLI拒絕／4正對照保持，新增10拒絕／2正對照共34通過；mode off仍195M，mode on是明示新玩家輸入的200M。完整DAC／PNG／195M前置與原store／RET只讀保護通過。原guest一次，沒有首失敗或重啟，不增加cap挑結果。沒有新CPU行為，沿378固定官方EXE CPU386／machine全套，不重跑無關測試。

原418來源前後保持，SAVE10.GAM208000bytes／0cf71fc5368861758da5a5dffbd791f8b711a48cfd295572eb9a21d27dea0f4d、MOX.SET553bytes／de8554b3284c9f9065efe6e4f2dfa71ab0c7e732e75eaea0d0ecf465ab86409f與sound.lbx4250888bytes／3f0354ac5c1b13a3c5c4fd098c2cbc22af37b71e74fc95e582782c22024c449d保持；同guest副本與終state／UID GID1000核對。這不證正式玩家存讀內容或持久選取欄位。

| 不可變鍵 | 新語意與等級 | 舊規格 | 回填 |
|---|---|---|---|
| 官方1.31／195M表hash3c6bd2…／index13；dosgolem_high_le20DDDB／DS188:26C4A6 | 正常行press／poll／release與原writer13已證實；200M黑圖，新UI未知 | 378、379 | 分開來源預期、原選取與未驗殖民地畫面，保留舊收據 |

378／379正文保留並追加380，索引與backlink同次驗證；較早其它選取上下文不由本row13外推。原LOG／PNG／RAM／journal／state及private probe維持本機，Docker清理與收據連主庫研究入口。

下一步依200M原完整核心、count1／55零bytes、當前223A23 code與SS188:2BD908 stack16，只追正常畫面建立所需的最小上層來源；先核對21A6F3是否真為該路徑的return定位與其原呼叫邊界，再由來源決定下一個有界畫面觀察，不盲目加cap或深挖renderer／DAC／PIT helper。主庫玩法RE-first保持，人口調整／正式存讀／完整開局／RNG與remake同狀態未驗，固定日期不是seed。

## 381 原框架回填

[381：200M末態的堆疊框架與直接呼叫來源](381-moo2-colonies-frame-source.md)已在相同200M輸入與cap下只讀原32bytes框架。原sub_1338C9配置260h區域空間，ESP2BD908首值21A6F3不在本函式返回槽；真正SS188:EBP2BDB68+10h的2BDB78讀得22341C，吻合IDA linear EA133417的E8AD040000→sub_1338C9。caller屬sub_133237，当前呼叫鏈強推論，未觀察自然RET或新UI。原13032列、35876 DAC事件、完整非新增final、PNG及state保持。原380正文及黑圖邊界保留，未把回填當殖民地畫面驗收。
