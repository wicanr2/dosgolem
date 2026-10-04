# 379：殖民地Sol II行的原選取來源

狀態：**CONFORMED，限定既有378收據與原選取來源**
日期：2026-10-04

## 玩家範圍與來源

[378](378-cpu386-sub-al-immediate.md)同195M終點已可見Sol II殖民地列表，但正常行操作未送。本規格只核對既有378完整收據、可見位置與原選取器的第一命中順序，為下一次正常裝置輸入建立來源；不新增guest、不改cap／CPU／DOS／probe，也不實作主庫玩法。

官方1.31 ORION2.EXE SHA-256 4e11be14217b4aafa1839f333bf5eba037f98b0c44e9e4752c96c464c260419f，工具基線d2df07fb795875ffdcdd2b8566ae6ecadc073070。Go1.24.13既有378 journal SHA-256 825e88b325d7ee2b0fe5c2f99c35dd6a693216e953e7052336a5486d1943ae0c；完整20表DS188:298848／stride55／1100bytes SHA-256 3c6bd2afa22a4ac6500ce62df53ea9bfe88dd4713423922949d23f179820471c。378終PNG d3c775f1b8594e8313c27b5bd9e363dfb6e3210ca05f0f17136b36409b7a67ba已親看，不量圖猜矩形。

IDA Pro9.4 image sha256:6f6d59af49d0008c4109a5295b5f374bdc007e2d1ab28cb9de08779584de2780，固定官方EXE唯讀／tmp一次性DB／UID1000／network none／120s／2GiB／2CPU／128pids。非空JSON schema1、原EXE hash、5365函式與擁有權核對；idat exit1不作失敗或成功證據。正式.i64未改。IDA linear EA與dosgolem_high_le分別標示，不並列為同一位址。

## 原bytes與規則

**已證實，靜態控制流**：IDA linear EA sub_11CEF5範圍11CEF5..11E718。11DC51 C745BC01000000從index1開始，11DC5B比較原count、11DC62 JGE離開；11DC6C 6BD037使用37h stride。原前8bytes四signed word與原有符號bias做含端點矩形比較，11DCD1／11DD02 JL拒絕下界、11DCE7／11DD18 JLE接受上界。首次命中非raw type14，11DD3B 8945EC保存index，11DD3E EB76直接到11DDB6，不再掃較後物件；type14另有既有圓形距離分支，本行不依賴，不展開。

11DDCB讀record+8的raw type、11DDD1 CMP0Bh與11DDD4 JNZ；非type11在11DDDB 66A3A6C41700寫IDA word_17C4A6。原runtime較早COLONIES consumer為dosgolem_high_le:20DDDB／66A3A6C42600，資料operand由LE重定位加F0000h；原指令EA映射也是加F0000h。真正dosgolem_high_le:214104 C3對應IDA linear EA124104 C3，屬sub_124075 124075..124105／38指令；該函式設INT33 AX3並取得按鍵結果。callback runtime8:2136D1對應IDA linear EA sub_1236D1 1236D1..1237F3／74指令。保留原名、EA、file offset、operand與bytes，不替helper改名或追runtime內部。

**已證實，既有raw表**：index13前8bytes0C00230065004100，矩形12,35–101,65，record+8為7，非14且非11。index19前8bytes000000007F02DF01，矩形0,0–639,479，同樣含蓋名稱區。幾何命中不唯一，但靜態第一命中順序已證實；不能要求整表只命中一項而錯拒正常行。

**已證實，解析與靜態導出**：邏輯43,48與44,48均幾何命中13／19，第一項13；四個內角含端點仍先命中13。11,48、43,34、43,66不命中13；102,48先命中16，不能把行外全部當背景。已核對表hash與原比較方向。這是原規則和收據導出的輸入前置，未稱原guest已選取13。與畫面Sol II名稱區對應為強推論，須正常poll／store才能驗收。

record13的+24原值261716、+28原值0132h、+32原值2801A9、+44原值4015EC只作原定位，不命名正式欄位；未取新pointer窗口，不稱306為typed colony id或callback。type7後續用途、共享選取word生命週期與轉入殖民地的玩家結果未知。

## 195M輸入前置

原378 actual_boundary195000000／reason step_limit，dosgolem_high_le:EIP22C8BA，R=[264C6C 4C 3E7D8BA 26A98C 2BD4E0 2BD50C 5 1]、段=[8 188 188 0 20 188]、flags283h／IF1，FPU127F／status0／depth0／八stack bits0，clock447368391µs。原table count20／bias0與完整bytes可讀，終DAC588個非0／indexed296428／RGB759775非0。

callback target8:2136D1／mask2B／pending0／inactive、started completed16／16；IRQ51746／51746、inactive／非failed。只核對既有readonly snapshot與上述前置，不呼叫INT33或InjectMouseEvent來觀察，不以舊180M caller的stack解釋現在22C8BA。

下一次正常操作使用既有2:1橫向裝置尺度，logical43,48→physical86,48，release logical44,48→physical88,48。先有獨立READY的裝置輸入與明示後續固定預算，第一次前置不匹配即拒絕；一次press／原AX3 pressed poll／首安全release，保存原選取store及實際玩家結果。不得預填word13、跳handler、寫guest RAM或把同cap的source核對當點擊完成。新玩家輸入的觀察預算另列，不能靠重啟／重擲／加cap挑結果。

## READY與只讀驗收

先審查固定378 journal／PNG／table／完整核心與callback來源、IDA原比較／第一命中／真正RET與重定位bytes，才轉READY。private verifier只讀重用378與新IDA JSON，以獨立signed byte解析與矩形集合核算第一命中、角點及相鄰邊界，不直接照抄反編譯控制流。拒絕hash／範圍／IF／callback／原opcode漂移，保留原位址。

本規格的CONFORMED僅表示上述來源及只讀核對通過，沒有新原版實測。公開規格／000-index與378回填，原EXE／IDA JSON／LOG／PNG／RAM／state／private腳本維持忽略workplace。原CPU與全部internal／probe保持d2df07f；沿378固定官方EXE完整CPU386／machine收據，不重跑無關測試。

私有入口workplace/moo2-379-ida-list-input.py、new-game-379-source-verify.py，外層IDA重建命令與收據連主庫既有docs/re/dosgolem-moo2-intake-20260930.md。主庫玩法RE-first保持，列表點擊／人口調整／正式存讀／完整開局／RNG與remake同狀態未驗，固定日期不是seed。

## READY審查

固定378完整195M來源／20表hash／終PNG／IF1／callback與IRQ已核對。IDA原index起點、signed比較／含端點、首命中直接離開、非type11共享store與真正RET／callback定位、operand重定位均通過；轉READY後才寫只讀來源驗證器。

## CONFORMED限定結果

狀態：**CONFORMED，限定既有378收據與IDA原選取來源的只讀核對**。沒有新原版實測，沒有送行點擊或新增cap。

初版邊界oracle漏列右上角101,35同時命中16，實際解析拒絕該預期；按原完整表修正為13／16／19，首失敗收據保留。所有內角的第一項仍13，不改原表或玩家輸入。

獨立signed16解析20表，logical43,48與44,48幾何命中13與19；四個矩形內角均按原索引順序先命中13，右上角101,35另同時命中16。相鄰邊界11,48／43,34／43,66只命中19；102,48先命中16。raw type7、pointer原值及bias0保持；13組IF／表／callback／IRQ與raw bytes漂移均拒絕。原opcode／EA／file offset、378完整核心／FPU、PNG／table與來源hash及UID1000通過。

原靜態第一命中規則及本來源的預期index13已證實；Sol II名稱區對應保持強推論。實際pressed poll、shared store13、單／雙擊、人口調整或轉入殖民地仍未知，不把幾何核算當正常玩家驗收。

首次IDA查詢錯把runtime214104的EA寫成114104，callback anchor也換算錯；實際RET位元組不吻合，原查詢／JSON／LOG／stdout另留first-query。按runtime減F0000h改為124104／1236D1後，相同官方EXE重新建立一次性DB，原C3及函式邊界吻合。原11DDDB主選取定位未受影響。這是研究定位問題，未改CPU或原版輸入，未啟動第二個guest。

| 不可變鍵 | 新語意與等級 | 舊規格 | 回填 |
|---|---|---|---|
| 官方1.31／378的195M／DS188:298848表hash3c6bd2…／index13；IDA11DD3B、11DDDB | raw矩形與原first-match已證實；預期13，實際行點擊未知 | 378 | 分開可見列表與只讀選取來源，保留原四consumer／恢復歷史 |

378正文保留並追加379、索引及backlink同次核對；全部公開internal／CPU／DOS／原probe保持d2df07f。沿378固定官方EXE完整CPU386／machine通過收據，未重跑無關測試。Docker容器清理，私有收據與實際IDA命令連主庫研究入口。

下一步以本195M完整來源建立一次正常Sol II行press／原AX3 pressed poll／安全release的獨立READY契約，明示新玩家輸入的固定後續預算，先驗原13選取store及實際畫面。不能以再加cap、重啟、代寫word13或跳原handler求過。主庫玩法RE-first保持，正式存讀／完整開局／RNG與remake同狀態未驗。

## 380正常行輸入回填

[380](380-moo2-colonies-row-click.md)在本完整195M來源通過後送一次正常physical86,48 press／原AX3 pressed poll／首安全physical88,48 release。原195226311、dosgolem_high_le:20DDDB／66A3A6C42600真正寫DS188:26C4A6 word0000→0D00；原195225486 RET按真實stack回20DB5B，AX0與其它核心保持。正常index13選取已證實，未將13命名為typed colony id或持久欄位。明示一次200M新輸入窗口，末圖仍黑／table count1，新的殖民地UI未驗；本379來源正文與收據保持。

## 387 職業列座標來源回填

見[387條件座標與持按／放開來源](387-moo2-colonies-input-coordinates.md)。不可變定位為官方1.31／IDA linear EA sub_1236D1的123729／12372D／12372F與123738、1237F2／CB；runtime callback8:2136D1及DS188:2A3A38／2A3A36。原回呼只有在word_17C51A及+2皆0時，將ECX低word作signed SAR1成GUI X，Y保存EDX低word；初始化range為2×(width−1)及height−1。原2:1列表來源未被推翻，這輪補齊其旗標及range條件，不改原列表輸入／收據。

持按實際選取器是sub_113FB9，與原事件來源的11DC掃描分開；原11E160再次AX3，為0後到11E4EB，kind6在11E508呼叫1192D1並於11E50D清共享選取。條件控制流已證實，人口操作是否觸發仍未知；不能只用原pressed poll當作職業列已消費。386完整首輸入沒有取得本輪新旗標／座標／range，下一步388先補只讀前置，不送人口輸入。
