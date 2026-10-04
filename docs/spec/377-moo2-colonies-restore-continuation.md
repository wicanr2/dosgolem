# 377：COLONIES降色後的首個色彩恢復與195M邊界

狀態：**CONFORMED，限定185M前置與實際CPU停止觀察**
日期：2026-10-04

## RE來源與觀察目的

[376](376-moo2-colonies-dac-write-journal.md)已證實正常COLONIES輸入後原DAC經11輪單調降色，首次全0與末write同為device sequence290512／loop182566943／419464025µs。185M原EIP223A71、DAC全0但indexed有12723非0；此時仍未有恢復寫入，列表正常畫面未知。官方1.31 ORION2.EXE SHA-256 4e11be14217b4aafa1839f333bf5eba037f98b0c44e9e4752c96c464c260419f，工具3295ddcac19dfbbebed167a490cebed7859c86a2，CPU保持1d8a4d8252372c97d8e873ba13c3ab3670796527dcd6d74de52e8cbf226e068c，位址空間dosgolem_high_le。

本輪依已觀察降色與原185M繼續執行中的核心，只開一次明示10M窗口至195M，找185M後首個非0 DAC色值事件、其可見頁與實際終態。不修改玩家輸入、CPU、DAC或driver，不假定195M應完成轉頁，不達事件不重啟或再加碼求過。主庫玩法RE-first保持，這是原版觀察工具契約。

## 模式與邊界

私有DOSGOLEM_MOO2_COLONIES_RESTORE_CONTINUE只接受1或未設定。新模式依賴完整COLONIES／HOME／banner／可寫state及原MAX_STEPS=185000000；原maxSteps所有guard保持，另runSteps=195000000並輸出baseline185M／cap195M／window10M設定。mode off全部376入口、185M上限與參數拒絕保持；新模式不得用195M、186M或其它值繞過baseline參數檢查。

正常180M來源、press／poll／release、原選取10與兩RET保持。原loop在185M起點先沿相同順序取20表／color source及376完整DAC journal；將185M黑PNG另存baseline專用檔。只有前置逐值與只讀guard通過才進新窗口，baseline設定與實際run cap分開記錄。

376共通原列比較到185M journal摘要，不把尚未停止的guest稱為185M終止，也不拿195M變動的terminal totals假稱原185M相同。保留39個原frame及baseline黑PNG，baseline journal獨立重播與舊376全部groups逐值核對，RAMhash逐輪正規化但只讀保持。新增continue設定與baseline標記單獨核對，原MAX_STEPS預算宣告仍185M，實際runSteps另195M。

## 首恢復與終態只讀

沿376同VBE device完整PortLog增量、最大delta4096／groups與events262144，保留全部原序列與group核心／clock。185M之後首次rawDAC非0時，只記錄一次group來源、原code16／目前SS:ESP stack16、DAC768／palette／mask、可見indexed與RGB hash／非0數／256bin histogram、PNG、globals192／header16和count≤64的原表。group是loop觀察邊界，不猜內部write逐週期時間或私有語意名稱。

可見頁每pixel驗palette映射，公開私有getter仍純讀且只在/tmp隔離編譯；保存完整核心／FPU／clock／VBE／callback／IRQ／device／ports map／journal／DAC及RAM取樣前後保持。原表超出64不讀，明示table_readable=false，不猜新表形狀。首恢復PNG不當完整列表驗收，實際終圖須另看。

195M取得相同只讀來源與完整journal。如果原CPU先拒絕或DOS退出，保留原failure收據，只保存當時觀察邊界，不改成step_limit=195M，不稱guest成功；probe exit0也不當驗收。原guest一次，不因未達事件、黑圖或觀察失敗再起guest。state仍以原終態／同guest有界副本核對；若自然變化，分開保存並查來源，不預填相等。

## READY與驗收

先核對376完整185M／11輪降色與首0末write、baseline20表與色彩、原正常輸入、工具sourcehash，審查private loop上限與所有原MAX_STEPS guard保持，再轉READY才實作新模式與observer。新patch可逆回376，全部public internal／CPU／DOS／probe保持3295ddc；沿372固定官方EXE全套，另建置probe，無新CPU行為，不重跑無關測試。

CLI原13拒絕與3正對照保持，追加restore值／缺COLONIES／缺HOME／錯baseline／缺state拒絕，以及restore mode off原185M正對照。獨立序列重播核對185M前groups、首非0write、終DAC／mask／index／phase／ports累計，原pixel映色與PNG可見結果分開。所有未知保留，恢復事件不外推完整玩家流程或remake同狀態。

Docker Go1.24.13 image sha256:1a6d4452c65dea36aac2e2d606b01b4a029ec90cc1ae53890540ce6173ea77ac，network none／600s／2GiB／2CPU／128pids／UID1000，原ZIP／patch唯讀、owned監測550s／trap收尾。私有入口workplace/new-game-377-ready-review.py、new-game-377-run.sh、new-game-377-verify.py；baseline與continued journal／PNG／LOG／rawDAC／state／probe不入Git。公開本規格／000-index與376回填，收據連主庫既有docs/re/dosgolem-moo2-intake-20260930.md。

下一步由實際首恢復、正常畫面或CPU停止決定最小玩家阻塞，不用重複擴cap代替來源，不深挖DAC／PIT／driver或renderer helper。列表操作／正式存讀／完整開局／RNG與remake同狀態未驗，固定日期不是seed。

## READY審查

376完整185M色彩／20表／核心、11輪降色與末write及兩private source hash核對通過。原MAX_STEPS guard保持185M、新runSteps195M與一次10M明示、baseline共通列範圍／先只讀核對再續跑、首非0與CPU停止／實際上限分開保存，契約審查通過，轉READY後才實作。

## CONFORMED限定結果

狀態：**CONFORMED，限定185M前置與一次195M上限觀察**。首色彩恢復未命中，原CPU在188532362停止；不稱195M完成或列表驗收。

原376共通12188列到185M journal摘要保持，完整11275 baseline groups、39frames及185M黑PNG逐byte保持。新增run config明示baseline185M／runLimit195M／window10M，原MAX_STEPS guard與budget宣告185M保持；mode off原185M正對照通過。新mode沒有追加玩家輸入，原COLONIES press／poll／release、共享選取10與兩RET保持。

185M後又有1025個DAC事件：3C6一次FF、3C8 index0..255及3C9共768個零值，從loop188259170／430366579µs到188265776／430373460µs。rawDAC一直全0，沒有首非0write或first-restore PNG。完整180M至停止序列共12300group／12300事件，三埠為12／3072／9216筆，全部獨立重播至終DAC／mask／index／phase及ports累計一致。

原188532362在dosgolem_high_le:1F455D拒絕opcode2C，原16bytes=2C173C080F87ED0000000FB6C02EFF24。CPU錯誤標明opcode尚未支援；工具EIP抓opcode後為1F455E，不能當作SUB已執行。這是原SUB AL,17h的待支援入口，不猜較後CMP／Jcc／jump table語意，也不把decoder後移當正常返回。新工具只讀收據先保存reason=cpu_stop／actual_boundary188532362，原guest_cpu_stop與step_error保留，沒有step_limit=195M；probe exit0不作guest成功證據。

停止R=[1A 0 2BD800 D 2BAFC8 2BD8A0 29E1D8 2BD348]、段=[8 188 188 0 20 188]、flags206h／FPU127F／status0／depth0／八stack bits0，clock430866468µs，callback target8:2136D1／mask2B／pending0／inactive／16／16與IRQ49858／49858已返回且非failed。原code16在已後移1F455E為173C080F87ED0000000FB6C02EFF2485，SS188:ESP2BAFC8 stack16=1AD82B00000000000D000000F5401F00。只保存原定位與bytes，不推定frame欄位名稱。

終indexed／RGB／PNG與185M相同，12723非0索引、DAC全0／maskFF、RGB全0；PNG親看全黑。VBE bank4／startY0／sets2941／writes55575758／display94保持。原20物件表1100bytes終SHA-256 3c6bd2afa22a4ac6500ce62df53ea9bfe88dd4713423922949d23f179820471c，與185M有10個raw byte變更，語意未知；不以同header20冒稱整表保持。完整table／globals／header與current code／stack均只讀取得。

## 驗證、失敗分類與回填

八私有patch逆轉精確等於376，public internal／CPU／DOS／probe保持3295ddc；18CLI拒絕與4正對照通過。baseline與終PNG用獨立PNG解碼、filter／CRC／RGB hash／非0數核對，palette獨立算術及256bin histogram映色一致；首恢復不存在也核對，而非偽造成功畫面。全部新取樣核心／FPU／clock／RAM／device／ports／journal／DAC及callback IRQ前後保持。

原418來源、state與同guest有界副本／UID GID1000保持；SAVE10.GAM208000bytes／0cf71fc5368861758da5a5dffbd791f8b711a48cfd295572eb9a21d27dea0f4d、MOX.SET553bytes／de8554b3284c9f9065efe6e4f2dfa71ab0c7e732e75eaea0d0ecf465ab86409f與sound.lbx4250888bytes保持。原guest一次，未重啟、重擲或再加窗口。沿372固定官方EXE全套，沒有新CPU行為，不重跑無關測試。

私有生成腳本第一次外層here-document與內嵌PY重名，Python收到截斷內容而syntax失敗；尚未實作或啟動guest。更名外層為PY_GEN_377後，同內容成功且八patch逆轉通過。首失敗摘要另留，不歸因產品／CPU；native只啟動一次。

| 不可變鍵 | 新語意與等級 | 舊規格 | 回填 |
|---|---|---|---|
| 官方1.31／185M至195M上限／1F455D 2C17 | 首色彩恢復未命中；原188532362缺2C已證實，實際未到195M | 376 | 追加實際CPU邊界與下一指令，不改既有降色歷史 |

376正文保留並追加377，索引與backlink同次核對；原LOG／PNG／journal／RAM／state、probe與getter維持本機。相關Docker容器清理，私有收據與命令連主庫既有研究入口。

下一步依原1F455D／2C17與完整188532362來源，建立CPU386的SUB AL,imm8規格，核對公開ISA契約與既有byte SUB旗標模型；READY後實作2C並用獨立256×256輸入、保留EAX高24bit／其它暫存器／段／FPU／非算術flags及立即數fetch失敗驗收，再重播同377窗口。原consumer自然恢復與正常畫面另驗，不再擴cap。主庫玩法RE-first保持，列表正常操作／正式存讀／完整開局／RNG與remake同狀態未驗；固定日期不是seed。

ISA名稱交叉核對：[Intel SDM Vol.2B](https://www.intel.com/content/dam/www/public/us/en/documents/manuals/64-ia-32-architectures-software-developer-vol-2b-manual.pdf)，SUB條目4-654頁列2C ib／SUB AL,imm8；這只確認待支援指令名稱，原guest尚未執行該指令，旗標與實作驗收留下一規格。
