# 313：word 暫存器與完整立即值的 CMP

狀態：**CONFORMED**
日期：2026-10-03
範圍：CPU386 的66 81 /7 iw，暫存器來源；只補平台CPU，不修改主庫玩法。

## 原始定位與公開契約

工具基線f2d982a7d9383a2b536d9540cb5b8b9e860f6284；固定官方1.31 ORION2.EXE SHA-256 4e11be14217b4aafa1839f333bf5eba037f98b0c44e9e4752c96c464c260419f。312兩個明示epoch1996-01-01、48M正常Esc／50M上限流程在第48919460步、高位LE0x14E3DE拒絕66 81 F9 D4 00。CX0001h，完整R為FFFFFFFF／1／958／29BE7C／2BDBA4／2BDBD0／2600CD／2BDC2C，六段8／188／188／0／20／188，flags293h；EIP14E3E1僅部分解碼，不是CMP成功。

公開來源：[Intel 80386 Programmer’s Reference，CMP](https://www.ardent-tool.com/CPU/docs/Intel/386/manuals/prref386/CMP.htm)及[Jcc](https://www.ardent-tool.com/CPU/docs/Intel/386/manuals/prref386/Jcc.htm)。81 /7 iw以word來源減完整imm16，僅更新CF／PF／AF／ZF／SF／OF，結果不寫回任何R或RAM；不按83的imm8符號延伸。原始後續0x14E3E3的0F 8C 45 FF FF FF是JL rel32，SF與OF不同才跳，目標由下一EIP14E3E9＋signed FFFFFF45h得到0x14E32E；不以runtime函式名推定用途。

## 契約與驗收候選

- typed input為來源uint16與立即值uint16，按差值決定六旗標，其餘R／高word／六段／非算術flags／完整浮點狀態／RAM保持。完整iw取得後才改flags；EIP成功到5bytes後。
- 沿既有81 word前綴閘門與sub16；只加入暫存器group7，既有word memory CMP不改。segment／REP／REPNZ／67／LOCK／重複66與其餘未支援group保持拒絕，截短不發布核心變更，既有partial fetch EIP不冒稱transaction rollback。
- 八來源×全部65536低word對原版立即值212，49組word邊界及完整iw、獨立借位／低半byte／popcount／有號範圍驗六旗標；高word非零、完整非零FPU／R／RAM不寫、截短／prefix／未知group與既有word SUB／ADD／OR／AND、word memory CMP與dword CMP回歸。
- 自製原始CMP後真正執行既有JL，驗taken／fallthrough／相等／正負溢位及signed對unsigned的差異。原版唯讀觀測限最前兩組CMP／JL，及CX首次自然到212的邊界各一組，最多六筆，不改流程或挑骰；另計數全部外層該固定EIP到訪，不以總數宣稱未捕捉迭代的逐值對拍。
- 固定EXE全套後，兩同明示日期／正常Esc原版流程、完整CMP旗標與JL真正消費、兩初態滑鼠0／1分開、未設定日期第三控制流程、畫面／時計／IRQ後才限定CONFORMED。

## 邊界與回填

不可變鍵為固定EXE＋dosgolem高位LE0x14E3DE＋66 81 F9 D4 00。不同IDA線性／檔案偏移不混用。312／309／310／311同一CMP停點閉合後追加勘誤與護欄，保留其原拒絕收據。原始資料／完整RAM／終端／PNG只留本機忽略目錄。CX與00D4h的遊戲欄位用途未知，CPU公開規格足以實作，不為runtime helper深挖。AH2Ch／RNG、255／299自然OF=1、人耳、完整鍵盤、主選單／玩家路徑與主庫玩法RE閘門保持。

## READY審查

兩312正式收據與固定CPU來源已重新核對，原始CX／完整立即值／後續JL bytes已保存；不需要另跑同一停點或猜資料來建立規格。81既有word memory CMP已用sub16，但word暫存器group7在iw之前拒絕，根因為group whitelist缺7。公開CMP／Jcc與既有fetch／sub16足以READY：完整iw後只呼叫sub16、不寫R。原版第一筆0001h−00D4h的六旗標預期CF1／PF1／AF1／ZF0／SF1／OF0，保留其他旗標得到293h→297h；JL應跳0x14E32E。自然CX到00D4h時六旗標為CF0／PF1／AF0／ZF1／SF0／OF0，JL應落0x14E3E9。這些是平台規格推導預期，尚待正常原版重生，不稱已觀測到邊界。

## 實作與目標測試

限定81的word暫存器whitelist加group7，完整iw讀成功後只呼叫既有sub16，不寫回R。word memory CMP、dword、前綴與其他group保持。CPU SHA-256 538abd53a40d65cc07b33cbeaf272d3541cd6dd4622c4fa17a5d8244a0521a77；internal/cpu386/cmp_word_register_imm16_test.go c5c98f56408f5d08657a712ccb9301b20537b395087b2b095999aeff8b8eaf8a；自製有界探針workplace/moo2-probe/main.go cfc80807525210345be4686b910a3d07a2b6b92389f5fdef6516667c91aedafc。

三個新CMP與六個既有word ADD來源／SUB完整立即值回歸全PASS：八來源×全部65536低word對212、49組邊界；預期使用311測試的獨立借位／popcount／有號差值公式，未用production sub16當答案。R／六段／高word非零、非算術flags、非零FPUControl／FPUStatus／FPUStack／FPUDepth、來源與RAM保持。自製CMP後真正JL依int16關係驗九種來源與五種立即值的45組taken／fallthrough，包括1／2／211、相等212、213及正負／有號溢位邊界；JL本身不改flags或其他核心。截短／prefix／重複66／未支援group2／3／6拒絕，既有word SUB／ADD／OR／AND、readonly memory CMP與dword CMP回歸通過。

命令：go test -p 2 -buildvcs=false ./internal/cpu386 ./internal/machine -run 'Test(CMPWordImm16|SUBWordImm16|ADDWordMemorySource)' -count=1 -v，180s／2GiB／2CPU／128pids／UID1000／network none，Go1.24.13固定映像。workplace/moo2-313-cmp-imm16-tests.txt SHA-256 fd354238907ab735a9bcae1dfd197eae39fd03757751e9b4798338d656f7100f。machine包目標篩選沒有匹配測試，不冒稱其新測試通過；正式固定EXE全套另驗。

正常觀測只讀原始EIP／CX與核心、指令及SS:ESP32byte窗；樣本固定最前兩組及CX首次到212的一組，最多六筆，外層EIP到訪另計數。沒有資料代寫、跳指令、改亂數或依通過結果挑樣本。正式600s Docker／固定EXE全套與三流程沿312的命令，輸出312改313，原ZIP／patch唯讀、/tmp/game新鮮417根檔加官方1.31 EXE。兩設定epoch1996-01-01，48M正常Esc、50M上限、SEPARATE_DOS=1，第二組加既有受控滑鼠，第三不設日期。狀態保持READY直到正式收據審查。

## 正式原版CMP與分支消費

固定EXE全套PASS，workplace/full-test-313.txt SHA-256 e1bf0db1fb8c96e8f971ed6635b73fd0ad0d5f601f1905480de0f924e380cf73。兩設定日期／有無受控滑鼠及第三未設定流程，由同一有界Docker命令完成，原始素材與輸入不改。

已證實，以下為dosgolem高位LE的真正正常入口觀測，兩設定流程六筆完整R／六段／flags及SS0188:002BDBA4的32byte窗口核對；CMP／JL都不改來源或窗口：

| 外層CMP步／CX | 真正CMP與下一JL |
| --- | --- |
| 48919460／0001h | 0x14E3DE→0x14E3E3，66 81 F9 D4 00，flags293h→297h；48919461的0F 8C 45 FF FF FF從0x14E3E3跳0x14E32E，flags297h保持 |
| 48919797／0002h | 同一CMP與flags293h→297h；48919798同一JL跳0x14E32E，完整核心保持 |
| 48992578／00D4h | 同一CMP、flags293h→246h；48992579的JL不跳，落0x14E3E9，flags246h保持 |

前兩筆CF1／PF1／AF1／ZF0／SF1／OF0，邊界CF0／PF1／AF0／ZF1／SF0／OF0，均以獨立無號借位／低半byte借位／結果byte popcount／有號差值範圍計算；JL分支使用實際SF與OF，位移從原始bytes解出。原版高word0，非零高word由八來源／全word測試補足。SS窗口3733200038E03C00D0DB2B0000000000A403000010000000BC8A440038403E00逐筆保持；不把窗口內值加上推測名稱。

唯讀計數cmp_word_immediate_totals observed_site=14E3DE total=212 sample_groups=3 boundary212_observed=true，僅證明外層到訪，未捕捉迭代不冒稱逐值原版對拍。全部312停點前基線，除mtime／DTA四bytes與PNG名稱外逐列保持；兩流程六筆核心、音訊／時計／IRQ與VBE同狀態。已有受控滑鼠初態各有／無事件，mouse_started=1／mouse_completed=1與mouse_started=0／mouse_completed=0分開驗，不要求刻意不同初態全部相同。未設定第三流程完整保持312 AH2A拒絕，新增唯讀totals=0／sample_groups=0／boundary212_observed=false，不默認日期。

新停點兩設定皆第49564005步、高位LE0x24C31B，CD 33，AX0014h的滑鼠服務未支援。完整R為14／1／2136D1／0／2BDA88／2A0000／0／0，六段8／0／8／0／20／188，flags6h；真正input與output及flags保持handled=false，EIP24C31D是INT fetch，不當作服務成功。此處的功能分類與返回契約留下一平台切片，不猜遊戲欄位或直接跳過。原始下一C3返回尚未執行。

兩時計61027457、IRQ0 started8251／completed8251／failedfalse，IRQ7 started357／completed357；DMA完成357／剩1490／credit984300／current4A2Eh／count05D1h。裝置irq7_deliveries358含先前16位傳輸，不混為保護模式IRQ7完成。VBE Bank2／StartY0／BankSets738／Writes6003978／DisplaySets10。兩PNG SHA-256 5145cdfe5e66f25f9c78f9460256152cfa914d2ac22b17a89dded2b1717f3a37，逐位元相同，已實際檢視Master of Orion II標題背景、太空站與中央游標，主選單按鈕未見。未設定第三圖仍黑色過場1610444d26adb3135e7e933dd44912044bb636728af59c70e614945278d3c622；不將三圖當作相同結果或主選單已完成。

正式兩gzip workplace/moo2-probe-313-full-game.txt.gz SHA-256 9ecb69d4db8d3e563ecf0437aa6c94ef20bf81cae8616054f3fcb50429380785；workplace/moo2-probe-313-mouse-event.txt.gz 024cb32aacccb303f6b58054e277c5a14e989ca79d426f8a25ef02790c40f895；workplace/moo2-probe-313-unconfigured.txt.gz 1d9d785ac3189d58aab71527bb80f8335655a4d024e28e72b395f6926241e958。正式收據及CPU／探針／測試來源雜湊重新核對通過，未調時鐘／亂數／輸入或上限，沒有失敗測試需放寬。

## 解析回填與範圍

word CMP完整立即值停點已由規格 313 接通。309-sb16-pause-resume-dma8.md、310-moo2-dos-calendar-date.md、311-cpu386-sub-word-register-imm16.md與312-cpu386-add-word-memory-source.md的同一固定EXE／0x14E3DE未知同次追加本標記與本檔連結；其舊拒絕收據及各自範圍保持。護欄入口apps/moo2/tools/startup_probe_131.py --check-cmp-word-imm16-spec-backlinks，缺原始CMP／六旗標／JL兩方向／完整窗口／收據／任一舊文件回填即拒絕。

限定CONFORMED只補此CPU形狀與上述原版正常消費。滑鼠AX0014h、255完整座標／游標、AH2Ch／RNG、299自然OF=1、人耳、完整鍵盤、主選單／正常玩家路徑與remake同狀態未驗，主庫玩法RE閘門保持。原始素材與完整收據留本機忽略目錄，不公開。下一步按公開滑鼠API核對AX0014h與既有callback儲存，READY後接平台服務，不深入driver、代寫遊戲資料或提高50M上限。

同次55回填函式、33新增缺定位／狀態／正式收據／較早標記／連結負例與313 CLI通過。全部受驗CPU／測試／探針與正式收據保持；索引與新檔UID:GID1000:1000，工具root-owned／異形.md目錄自檢空，本輪有界容器已退出移除。

AX0014h交換停點已由規格 314 接通，見[314-moo2-protected-mouse-callback-exchange.md](314-moo2-protected-mouse-callback-exchange.md)。同一固定EXE的24次交換與前三次C3返回已由正常入口重生；舊拒絕與本檔原範圍保持。兩流程目前到50M上限、高位LE0x23856E，主選單面板部分滑入，完整主選單／玩家路徑未驗。
