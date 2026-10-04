# 386：原回呼地址校正與首輸入只讀前置

狀態：**CONFORMED，限定原版只讀觀察**
日期：2026-10-04

## 問題、來源與邊界

承接[385地址勘誤](385-moo2-colonies-scene-ready.md)。385原BF456／setup真返回及首輸入已驗，但2A8840回呼投影被原A3否定，整份觀察契約仍DRAFT。本契約只修正私有只讀來源，不改CPU／DOS或主庫Go玩法，不送人口輸入。舊385及384收據／Go均不改，不執行其被拒絕的回呼模型作新驗收。

官方1.31 ORION2.EXE SHA-256 `4e11be14217b4aafa1839f333bf5eba037f98b0c44e9e4752c96c464c260419f`，工具基線3842529eb5dff704adf3d33b6c4f5720ebe9cbb4。IDA Pro9.4 linear EA 1191EA／A340881A00寫dword_1A8840；dosgolem_high_le runtime2091EA／A340882900指定DS188:298840，符合加F0000h。原file bytes／offset／fixup分列385 bytes index，不能混為同一位址基準。Go1.24.13與既有鎖版Docker，原ZIP與patch唯讀，UID1000／network none／有界程序及trap。

**已證實**：385原store在203219451，EAX1AED21，原第一正常輸入在205804505／1B0845，code E861690500，完整frame／PNG保存。**強推論**：原A3將EAX寫至298840。**未知**：正確四bytes讀回、實際store之外RAM保持與人口操作。此輪不能預填原callback結果或猜人口語意。

## 輸入與只讀行為

新增私有DOSGOLEM_MOO2_COLONIES_CALLBACK_READ只接受1，要求原SCENE_READY／JOB／upper／row／restore、185M參數與state完整前置。缺項或非法值在EXE讀取前拒絕。mode off精確回385，保留原被拒絕定位供可逆審查；不以mode off啟動原guest。mode on固定210M，沿同原輸入，取消385額外10M；只觀察200–210M，最多32筆原store，截斷明示。

peekScene在新mode讀298840四bytes，其他核心／FPU／clock／callback IRQ／VBE／DAC、原code／stack／enable保持既有只讀驗收。對callback-store先以原code驗A3及moffs32=298840、DS及descriptor界限，再用實際moffs與descriptor Base求linear。原Step只執行一次。只在observer自己的RAM副本覆蓋實際四bytes，與Step後整RAM hash比較，不能改guest RAM、EAX或回呼值。

原首輸入captureRestoreFrame取得後，以固定385 scene-input.json SHA-256 `4e0a112574c9fae4e1e5bb6b27a625a23ca887d07859dfa9dd1ea0abb7412f84`作完整守衛，僅按既有跨run規則排除per-run ram_sha256，其餘全部欄位必須相同；不是只查EIP／step／PNG。守衛成功才取控制前置。原8窗口與361byte record／三pointer保持，額外第9窗298840四bytes另標原callback；2A8840保留為未分類舊raw，不當callback或CPU缺陷。snapshot before=after、RAM前後hash與descriptor-aware peek只讀。

210M在finishRestore由monitorScene取原完整來源守衛及legacy job raw，再取terminal；只有新增mode呼叫，off保持385。原382／384完整終態、200M來源及parent RET、36表／DAC／PNG／journal、418檔與state副本保持。此輪不新增猜測座標、人口輸入或正式存讀。

## 審查與驗收

先核對固定385 store原bytes、LE投影、首輸入完整frame及PNG與本契約索引，才READY及生成可逆private patch。原CLI70逐項保持，增加新mode拒絕與正對照；source verify只審查385 private bytes與逆向還原，不執行其DRAFT來源閘門或guest。

原guest僅一次，最大210M，不重啟、重擲或延長求過。同一收據核對完整原382及384 journal／PNG／state，完整385首輸入guard、原四事件時序與RET、A3實際moffs／EAX／讀回及全RAM唯一四bytes、首輸入第9窗與其他前置。實際差異保留；達不到即DRAFT，不能刪差異求CONFORMED。原資料／EXE／JSON／PNG／LOG／RAM／Go留本機忽略workplace，公開只有自撰文件與回鏈。

停止於正確回呼讀值及首輸入前置。主庫RE-first保持；後續人口選取／放置需原座標消費端證據及獨立觀察契約。固定日期不是RNG seed，正常人口變更／正式存讀／完整開局／remake同狀態未知。

READY審查：固定原A3／IDA投影、385完整首輸入及被拒絕地址、原輸入210M與索引通過，先於private實作。

## 實際限定結果與下一步

原guest僅一次，session78051 exit0，actual_boundary與run_limit均210000000／step_limit；末態228DF6／483821442µs。公開CPU／DOS／internal／原probe及主庫玩法保持，沒有新人口輸入、重啟或加cap。11個private patches精確逆回385，82CLI為68拒絕及14正對照，原70逐項保持。

**已證實，原A3及全RAM**：203219451在2091EA執行A340882900，DS188:298840由55451700變21ED1A00，raw由174555變1AED21，等於原EAX；下一2091EF。以實際moffs加descriptor Base求store_linear298840，Step前整RAM副本只替換這四bytes後與Step後hash完全相等，其他RAM保持。原guest沒有由observer代寫。兩個原真RET及原四事件時序與385一致。

**已證實，首正常輸入**：205804505／1B0845完整frame通過固定385原收據守衛；僅排除per-run ram_sha256，其他欄位全等。第9只讀窗口298840四bytes為21ED1A00；原8窗口、current raw4／pool5B2044／record5B25E8、三word310全部保持，2A8840零raw保留未分類。before=after、RAM前後hash及完整descriptor-aware peek只讀核對通過。

獨立驗證session12161 exit0，386 CONFORMED SCOPE PASS限定原回呼讀值、實際四bytes寫入及首輸入前置。完整382／384原210M核心／FPU／clock／callback IRQ／36表／DAC／journal／39frames／PNG／418來源／state及副本保持，完整385首輸入與新PNG相同。原385與384收據不改；385被拒絕觀察器仍DRAFT，本輪新觀察器不替它補成CONFORMED。

生成器前兩次因原Go縮排與定位字串不符，在寫Go前拒絕；連續同類失敗後回查閘門路由並檢查全部原縮排，再生成。失敗版本與分類摘要保留，並非guest或產品故障。來源檢查、CLI、native及獨立驗證均沿同原資料，沒有重跑guest。

383／384／385按同一不可變定位追加回填，索引同步。下一步387從原首輸入CALL的sub_1171AB及既有kind6消費端，補齊滑鼠座標到職業列熱區與暫存的正常來源鏈、按下／放開契約；證據足夠才建立一次正常人口選取／放置觀察，不猜倍率或job語意。主庫RE-first保持，人口操作／正式存讀／完整開局／RNG與remake同狀態仍未知。

## 387 職業列座標來源回填

見[387條件座標與持按／放開來源](387-moo2-colonies-input-coordinates.md)。不可變定位為官方1.31／IDA linear EA sub_1236D1的123729／12372D／12372F與123738、1237F2／CB；runtime callback8:2136D1及DS188:2A3A38／2A3A36。原回呼只有在word_17C51A及+2皆0時，將ECX低word作signed SAR1成GUI X，Y保存EDX低word；初始化range為2×(width−1)及height−1。原2:1列表來源未被推翻，這輪補齊其旗標及range條件，不改原列表輸入／收據。

持按實際選取器是sub_113FB9，與原事件來源的11DC掃描分開；原11E160再次AX3，為0後到11E4EB，kind6在11E508呼叫1192D1並於11E50D清共享選取。條件控制流已證實，人口操作是否觸發仍未知；不能只用原pressed poll當作職業列已消費。386完整首輸入沒有取得本輪新旗標／座標／range，下一步388先補只讀前置，不送人口輸入。

## 388 原首輸入動態前置回填

見[388只讀raw及裝置範圍](388-moo2-colonies-mouse-source.md)。不可變鍵為官方1.31／IDA linear EA word_17C51A／17C51C、17C534／17C538、1B3A38／1B3A36、1B121A及17C4E4；dosgolem_high_le投影見388原8窗。原205804505的8窗與裝置range已由388只讀補驗，完整前後狀態及既有210M結果保持。

較早正文只描述當時收據，保留其未知邊界；當次205804505的上述raw前置由388補驗。原持按／放開真正消費及人口變更仍未知。下一步389固定本次完整首輸入、原旗標及range，建立一次正常職業列press觀察契約；候選裝置660,77按原signed SAR1為GUI330,77，原36表先命中kind6 index1。先追原持按選取與1192D1／場景回呼，依實際消費點安全release；不把候選命中當人口變更或預設職業語意。
