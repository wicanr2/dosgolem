# 389：殖民地職業列一次正常按下與放開

狀態：**CONFORMED，限定原版正常輸入觀察**
日期：2026-10-04

## 玩家範圍與固定來源

承接[388首輸入raw及裝置範圍](388-moo2-colonies-mouse-source.md)、[387持按及放開來源](387-moo2-colonies-input-coordinates.md)及[383職業列控制來源](383-moo2-colonies-job-control-source.md)。只觀察一次正常職業列按下、原持按選取、實際場景回呼與安全放開，保存完整原人口record差異；不猜暫存值與職務語意，不修改主庫玩法或CPU／DOS服務。

官方1.31 ORION2.EXE SHA-256 4e11be14217b4aafa1839f333bf5eba037f98b0c44e9e4752c96c464c260419f。工具基線011fe510aa8cf74a00d26b7bbc7d65b6094f04bc；388 private Go SHA-256 b154cdaa17035a0a9a69025f8b7ef88972f6984eaae52a6a70ec2f27166e528f；完整首輸入JSON ddf8a2eea05482b2c3abc105c05d0a38b3d2252b6da3b467851b7ffbf0f0c791，mouse-source JSON 650876e32fe41394fe85ae2b7dc871ccc7de551a72153fe16d2f0fee6beee49d。IDA Pro9.4 linear EA與file bytes／fixups沿383／387索引；dosgolem_high_le程式及資料投影加F0000h。平台近似與固定1996日期保持，日期不是RNG seed。

## 原證據與一次正常輸入

已證實：原205804505／1B0845完整frame、兩旗標0、width／height640／480、GUI43,48／事件43,48、button word0、持按閘門1、共享active0；裝置86,48／buttons0、X range0..1278及Y0..479，callback8:2136D1，mask2B／pending0／inactive／18完成，IRQ54850完成。

原36表index1為kind6、signed含端點310,62..518,92，原ECX低word SAR1使裝置660,77成GUI330,77，first-match1。這是輸入候選，實際選取與人口結果尚未驗。只有完整原首輸入、388新raw及device全匹配，IF與callback／IRQ可用時，才用既有InjectMouseEvent(660,77,1,0,0)送一次press。

原11E1A7呼叫113FB9，11E1AC接收結果。1192D1在1192F3／FF1540881A00間接呼叫正確callback；runtime2092F3的operand應為298840，原當次callback1AED21。原1192FF／C3返回。使用真SS incoming stack slot與ESP+4追蹤場景近返回，不以pressed AX3 poll取代職業列消費。

只有實際選取器返回、場景callback進入並依真stack返回，且完成原裝置回呼、IF／pending0／inactive／IRQ安全、虛擬時間至少20ms，才送同660,77／buttons0的一次release。原11E4EB是AX3為0分支；kind6的11E508呼叫1192D1，11E50D清共享active。保存是否到達，不用未知結果強行提前release。

## 私有觀察、預算與失敗

新DOSGOLEM_MOO2_COLONIES_POP_PRESS只接受1，要求原MOUSE_SOURCE／CALLBACK／SCENE／JOB／upper／row／restore、185M參數與可寫state。mode off精確逆回388；mode on保持原210M上限，不增加試跑、不重新seed。原94CLI保持，增加新mode拒絕及正對照。只有兩次正常裝置輸入；不Step額外指令、不改guest RAM，不派送點選ID或直接呼叫玩法。

新獨立pop-event／terminal收據保存完整frame／PNG、8窗／current／pool／UI暫存、361byte record、device與before／after狀態及RAM全等證明只讀。事件最多24，不逐步dump；不存在、descriptor越界、守衛漂移立即拒絕。持按選取器入／返回、場景入／返回、release輸入、AX3零分支、kind6release CALL及active clear按實際觸發記錄；未觸發也保留有界終態，不用預設值補完成。

新增輸入前保留原388全部首輸入及9窗，原journal在press以前的相同前綴、PNG與存檔來源核對。press以後結果允許由原程式改變，不能沿用原無人口輸入210M末態相等守衛；只在新mode跳過舊captureJobSource的原210M比較，保存實際終態與新pop收據。388及較早receipt／Go／public CPU與DOS不改。

## 驗收與停止線

先審查固定首輸入／raw／device、原36表first-match、原選取CALL及callback bytes、真stack返回及release安全條件，才READY及private observer。原guest一次，210M或實際CPU／DOS停止；同一次收據獨立核對，禁止為綠色延長或重跑。

完成聲明限於實際正常輸入、選取與回呼生命週期。record及UI暫存有差異時保存原bytes／位移與推論等級，不命名職務數量或宣稱換職成功。正式放置、人口語意、存讀、完整開局、RNG及remake同狀態仍待驗。遇CPU缺口先保留失敗，依原opcode建立窄修正契約，不向renderer／DAC／PIT或平台helper擴散。主庫RE-first保持。

原EXE／JSON／PNG／LOG／RAM／state與private Go本機忽略，不公開；公開只提交自撰契約與回鏈。Go1.24.13／一次性Docker、UID1000／network none、原ZIP及patch唯讀；native600s／2GiB／2CPU／128pids、capture550s與owned PID trap，驗證90s／2GiB／1CPU，來源與文件30s／512MiB／1CPU／64pids，完成後清理。

READY審查：固定388完整首輸入與raw/device、原first-hit1、持按CALL／callback operand／near RET／release CALL／CB及安全裝置條件通過，先於私有實作。

## CONFORMED限定正常輸入結果

**已證實，原正常輸入及回呼生命週期**：原205804505完整首輸入／raw／device守衛後，正常裝置660,77／buttons1按下；205813953進入runtime203FB9，205814268依真SS返回20E1AC，EAX1。205815045進入1AED21，206207980依真SS及ESP+4返回2092F9；同一步IF、pending0／inactive／IRQ安全及至少20ms條件成立後送buttons0放開。206264647到20E4EB零按鍵分支，206264656到20E508 kind6 CALL；206264681再次場景入、206658139返回2092F9。206658146／147執行20E50D清共享active並到20E516。15事件與所有取樣前後狀態、RAM及device均只讀，19個裝置回呼完成。

**已證實，原record及UI差異；語意未知**：原DS188 current4／pool5B2044／record5B25E8的361bytes，在press至active-clear-after206658147全部保持；210M終態出現8個差異：+0Bh FF→02、+0Dh／11h／15h／19h 02→00、+C8h 49→B9、+C9h 00→FE、+E7h 08→00。三個UI pointer words由310／310／310變330／310／310。這是原raw實測，正式職務、選取群及放置語意仍未知。

正常玩家路徑的原PNG人工核對：首輸入顯示Colony of Sol II、Pop 8,000k (+73k)；210M顯示Research Colony of Sol II、Pop 4,000k (-327k)。終圖SHA-256 6be8a5cf20dbdfaca8c2471e407e9e70607c8201ff3633c261ebd9e5b71cdf26，末態EIP1A5042／482658319µs。顯示差異已觀察，不用它命名原欄位或宣稱人口配置完成。

106CLI含原94逐項保持、88拒絕及18正對照。原guest一次／210M／step_limit；新增輸入以前的完整388首輸入／raw／device、原36表、回呼與日誌前綴保持。新增輸入以後保存實際終態，不與舊無人口輸入210M末態冒稱相同。418原輸入不變，SAVE10.GAM／MOX.SET實際副本hash／size／UID1000與388相同，正式存檔語意未驗。

獨立驗證首兩次漏掉解壓縮mtime及DOS DTA時間／日期。連續同類拒絕後回查平台規格入口、既有338比較規則及le_startup.go的0x16／0x18欄位；最後只排除mtime和DTA0x16..0x19，保留attribute、size、name及其餘bytes。完整首輸入／raw／device、13,233列共同日誌與49,210個press前DAC groups另行比對；每次RAM雜湊只依既有跨run規則排除，不遮玩法結果。三次驗證都讀同一批原收據，沒有guest重跑或原observer／CPU改動。

下一步390沿同輸入及210M，追查206658147之後原C086E→C02F9與B9C3D／B9E94的最小正式寫入鏈，定位這8個record差異與可放置狀態；取得證據才訂一次跨職業列放置。不假設8,000k→4,000k已完成換職或刪除人口，不盲增cap或深挖renderer／平台helper。

原population收據入口為本機忽略的workplace/new-game-389-pop-terminal.json、new-game-389-verify.py及new-game-389-state-verify.py；重生用new-game-389-run.sh與本契約Docker邊界。原JSON／PNG／RAM／LOG／state與private Go不公開。主庫玩法RE閘門保持。
