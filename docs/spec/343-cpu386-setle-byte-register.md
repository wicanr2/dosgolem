# 343：byte暫存器的SETLE

狀態：**CONFORMED，限定SETLE／原SS byte實際寫入**
日期：2026-10-03
範圍：cpu386裸0F9E的八個byte暫存器，沿既有SETE／SETNE register解碼與setReg8。記憶體SETcc、前綴與其他新SETcc不擴張。

## 阻塞與公開CPU契約

沿[342-cpu386-test-dword-memory](342-cpu386-test-dword-memory.md)正常輸入，工具0c6e871167259c382c6dac2288ace552c33e2319、CPU SHA-256 330baaf9917f4534906797e9eb484136b7297fc6a94343252db9b6f8da97c5f7。固定DOS1.31 ORION2.EXE SHA-256 4e11be14217b4aafa1839f333bf5eba037f98b0c44e9e4752c96c464c260419f，所有原位址dosgolem_high_le。113628909於17D536 bytes0F 9E C0 88 45 FC 89 C8 99 31 D0 29 D0 0F BF 55拒絕，錯誤後EIP17D538不當作指令起點。原畫面為「Generating Universe...」，不是完整開局。flags206h／EAX28已見，完整未改CPU初態與下一原SS byte目的窗待唯讀取得。

[Intel SDM 2B](https://www.intel.com/content/dam/www/public/us/en/documents/manuals/64-ia-32-architectures-software-developer-vol-2b-manual.pdf)印刷頁4-596..4-598：0F9E SETLE／SETNG於ZF=1或SF≠OF時設目的byte為1，否則0；所有旗標不變。ModRM reg欄未用，r/m指定目的。[80386 SETcc原廠手冊鏡像](https://www.ardent-tool.com/CPU/docs/Intel/386/manuals/prref386/SETcc.htm)的SETLE列誤寫and，同行SETNG正確用or；依Intel現行SDM與原CPU已有JLE條件採or，不從錯列反推遊戲規則。

## 最小契約與READY閘門

typed輸入是原EFLAGS的ZF／SF／OF與ModRM目的byte；只完整fetch ModRM且mod3後setReg8。低byte保留高24位，AH／CH／DH／BH保留所屬dword其餘24位；其他R／六段／FPU／EFLAGS／RAM保持，EIP加3。CF／PF／AF／DF／IF等不參與判定或被改。記憶體與66／67／段覆寫／F0／F2／F3仍明確拒絕，不發布目的byte；EIP可依既有error模型前進，不稱硬體exception重啟。

DRAFT只用private可丟棄probe於原17D536首遇臂一次最多兩步，保存完整R／六段／flags／16code bytes、SS:EBP-5四bytes窗及SS:ESP四bytes堆疊，observer核對CPU／FPU／VBE與RAM唯讀。原88 45 FC的byte目的位於窗index1即SS:[EBP-4]，欄位用途未知。未改CPU仍應同拒絕；剝除新列後全部342舊列與PNG保持，private source三區塊逆轉後保持342。來源與CPU契約足夠後才READY。

READY後實作通用SETLE register條件，不按遊戲位址特例、不注入資料或重送。八個byte目的／全部unused reg欄／所有六算術旗標組合與非算術旗標、所有初始byte／高位保持；獨立8列真值表、不呼叫CPU條件helper。以有號32位減法的數學比較核算SF／OF／ZF，再驗CMP結果接SETLE，涵蓋溢位與等值／大於／小於。兩方向下一MOV byte store、DS／SS分離、寫入失敗／段末既有MOV契約回歸；既有SETE／SETNE保持，截短／memory／前綴拒絕不改核心／RAM。

固定EXE乾淨來源Go全套通過，再同342原正常輸入／1996calendar／120M cap重播。原SETLE／下一88 45 FC最多兩步，用原目的窗核算AL與正式byte寫入及未變鄰居；其他R／段／flags保持。新CPU前可比舊列／PNG與原全部輸入保持，後續正常畫面／新CPU拒絕按實際保存。原部份helper只跨過，不深入其語意；生成完成、正式writer／RNG與remake同狀態仍未知，主庫RE-first保持。

原EXE／ZIP／PNG／LOG／RAM／private probe留本機忽略workplace，公開只提交通用CPU、自製測試、spec／索引／回填。沿342固定Go1.24.13 image sha256:1a6d4452c65dea36aac2e2d606b01b4a029ec90cc1ae53890540ce6173ea77ac，原ZIP／patch唯讀、新鮮417根檔／固定MOX.SET，UID1000／network none／2GiB／2CPU／128pids／原版300s／全套600s；輪末核對Git／擁有權／Docker清理。固定日期不是RNG seed；沒有外部386／8088實機語料，不稱硬體逐週期或語料全通。

## 唯讀初態與READY審查

未改CPU的private原版重播，全部8237原342列與31PNG保持；三private區塊逆轉後source保持342。workplace/moo2-probe-343-input.txt.gz SHA-256 3d7697eeec4e3bf79f6fa933c9c2461ca922a6383f2b9638f709cfaed6383aaa，private source f7bdd7a9f44b6b94aaa8f8430e9c0391d8bb8ff91dca988eb138805a76d49622；python3 workplace/new-game-343-input-verify.py PASS。

113628909原17D536完整R=[28 FFFFFFDA 0 FFFFFFC2 2BDA08 2BDA38 0 1]、六段=[8 188 188 0 20 188]、flags206h。SS188:EBP2BDA38-5的四bytes01000000，目的byte SS:2BDA34即窗index1已為0；原SS188:ESP2BDA08四bytes040F0000。observerreadonly／RAM保持、callback12／12、IRQ26735／26735非活動，pending0。未改CPU同0F9E拒絕，EIP17D538；完整初態與窗口可讀，來源欄位用途未知。

公開條件與原初態足夠，343審查READY：ZF／SF／OF均0，預期SETLE AL0、EAX28→0，EIP17D539、flags206h與其他核心／RAM保持。下一原88 45 FC應寫0到SS:2BDA34，EIP17D53C、所有R／flags與其他byte保持。目的原本0，所以RAM不變不足以證實真實write；正式兩步額外在既有publishBusObserver加有界write事件，只轉呼叫原Bus一次、保存實際linear／value／error，於同兩步budget才啟用，不改Bus身分或請求。正式source為原三observer區塊加兩個write觀測區塊，逆轉五區塊後保持342；初態private三區塊歷史source與收據不重寫。這些後態是預期，尚未當成驗收。

## 限定驗收：SETLE暫存器與原SS byte實際寫入

343限定CONFORMED，只涵蓋標準SETLE register與原17D536／下一88 45 FC兩步消費；正式writer、生成完成／完整開局與remake同狀態未驗。主庫玩法／平台／原input／calendar／120M cap未改；固定日期不是RNG seed。

**已證實，原條件與byte替換**：113628909原17D536的0F9EC0成功，flags206h的ZF／SF／OF均0，獨立八列真值表結果0。AL28→0、EAX28→0、EIP17D539，其他七個R／六段／FPU／所有flags206h與RAM保持。113628910下一原17D539 bytes88 45 FC成功，EIP17D53C、R與flags保持。SS188:EBP2BDA38-4即2BDA34原byte已為0，窗2BDA33四bytes01000000前後相同；原SS:ESP2BDA08四bytes040F0000也保持。兩筆readonly／step_ram_unchanged／readable真、error nil，callback12／12、IRQ26735／26735非活動、pending0。

**已證實，真實write而非只看同值**：唯一setle_bus_write在113628910、input_eip17D539、linear2BDA34、value0、error nil。位於SETLE完成紀錄與MOV完成紀錄之間，SETLE沒有任何Bus write事件，兩步budget耗盡後不再臂。observer只在原Bus.Write8轉呼叫一次後記錄address／value／error，沒有代寫或第二次forward。原byte是0→0，若只看RAM不變不能宣稱原指令確實寫入；本次另有實際請求證據。欄位用途未知。

private未改CPU重播全部8237原342列與31PNG保持，正式新CPU入口前8177原342列與30PNG保持；8177為舊late_startup_platform label=stop前的可比原列，不比較舊終態與新續行。所有正常press／release、原341後段RET與342 TEST三步保持。private三區塊／正式五區塊逆轉後source保持不可變342；CPU只增加SETLE register條件，逆轉兩條件式與註解後逐位元保持342。python3 workplace/new-game-343-input-verify.py／new-game-343-source-verify.py／new-game-343-formal-verify.py PASS。沒有重送、改座標、提高cap或解helper內部。

**已證實，新CPU負結果**：113628944於dosgolem_high_le:17D5A0 bytes0F 9F C0 88 C2 80 7D FC 00 75 0E 80 7D F4 00 75拒絕，error=0F 9F 尚未支援。原起點17D5A0，解碼後EIP17D5A2；EAXE6／EBXFFFF0000／ECX3／EDX1FA、flags293h。尚未120M，probe shell exit0不能當完整開局通過。原終圖逐位元保持342的640×480「Generating Universe...」，沒有證實新玩家頁或生成完成。PNG SHA-256 e975c476784977be084da3abb7601363bf290c74b96b26d56fc1af0994dd7dd0，RGB353171a9bce8ad97f55e3ee444a0f9f8017bf44e531b5616f42393ff976ddf69，沿342同hash人工確認，不重複把相同畫面列成新功能。共享20DDDB未命中，正式writer未知。

**獨立CPU測試**：2,097,152個組合涵蓋八個byte目的／全部unused reg欄／256初始byte／64算術旗標組合／兩種非算術context。以8列字面真值表推導條件，little-endian byte陣列替換驗高低byte別名與其餘24位、其他R／六段／FPU／所有旗標／RAM保持與零writes。77筆32位值的全部配對×八個目的，共47,432組CMP→SETLE；以int64有號差與溢位範圍獨立核算ZF／SF／OF，再以數學有號≤判定目的值，涵蓋等值、正負邊界與溢位。沒有呼叫CPU condition helper當oracle。

343當輪原SETE／SETNE八目的與ignored欄保持；截短、memory全ModRM、11前綴、其他未支援SETcc仍拒絕且不發布目的值。下一SS byte store兩方向、相異DS未動、鄰居與完整核心保持，未知段／唯讀／段末／Bus寫入失敗按既有MOV契約拒絕。自製測試是公開CPU契約推導，不稱386實機語料或逐週期對拍，外部8088實機語料也未取得。

go test -p 2 -buildvcs=false ./internal/cpu386 -run 'TestSETLERegister|TestTESTDwordMemory|TestTESTDwordRegisterImmediate' -count=1 -v PASS，cpu3864.352s。固定DOSGOLEM_MOO2_EXE=/tmp/game/ORION2.EXE的go test -p 2 -buildvcs=false ./... -count=1 PASS，cpu386189.431s、machine5.732s；全套從/tmp/test-src乾淨版控輸入加本輪新自製測試與現存testdata執行，避免忽略workplace的探索main污染。乾淨輸入仍以342的git ls-files -z | tar --null -T - -cf - | tar xf - -C /tmp/test-src建立，再cp internal/cpu386/setle_byte_register_test.go，原EXE取自fresh official patch。沒有改歷史探索檔或hash。

兩次原版各由新鮮417根檔／固定EXE／MOX.SET重生，第一次private初態、第二次READY後正式CPU；Go1.24.13固定image與342一樣、原ZIP／patch唯讀，原版Docker300s／全套600s／2GiB／2CPU／128pids／UID1000／network none。go build -p 2 -buildvcs=false -o /tmp/moo2-probe ./workplace/moo2-probe；原341全部環境及120M cap保持。舊338 CLI17無效值／正對照、339 CLI22無效值與120M／100M正對照通過。不另重跑未受影響六舊情境，正常可比前綴與固定EXE全套已充分驗證此CPU分支。

CPU SHA-256 76f4b7b3f97156f9422d32e894e55579286f85d2c8b39a26eb90887fa22cc88f，自製setle_byte_register_test.go 91f13f6f7f5773b6f27367d3f94a9299600ce84f4437750a4f8390dbdff99d5c，正式probe c304dbac8a0689e83560529b1fb7acc4cd46ef87298c5077125ea2d2cbd6fca9。原素材／圖／LOG／RAM／private source全部留本機忽略workplace。

正式原workplace/moo2-probe-343-red.txt.gz SHA-256 fffa5bf9edfa2a4c3fdbece2852a0069a3307fe6b96bf9ded086fb73cf35ed9b；固定EXE全套workplace/full-test-343.txt SHA-256 025ca761a9bffc3a248275548ec218ceac829409b5ed63f54991057c97a8e19b。

### 回填帳與下一步

| 不可變鍵 | 已證實語意 | 證據 | 較早規格 | 必須回填 |
| --- | --- | --- | --- | --- |
| DOS1.31／ORION2.EXE 4e11be14…／dosgolem_high_le:17D536→17D539→17D53C／SS188:2BDA34 | 原SETLE AL0、所有旗標保持，下一MOV真實write 0 | 兩步／Bus唯一write與全套／343正式收據 | 342 | 原17D536 SETLE與SS byte寫入已由規格343接通 |

原17D5A0 SETG與下一MOV已由規格344接通，見[344](344-cpu386-setcc-byte-register.md)。同輸入120M到17FCE4且無新CPU拒絕，仍宇宙生成圖，生成完成／完整開局與正式writer未驗；主庫RE-first保持。 343當輪其餘SETcc拒絕邊界由344擴張為完整16個裸暫存器條件；記憶體與前綴拒絕保持，歷史343收據不重寫。 原17FCC3迴圈進度與兩個正常返回已由規格345驗證，見[345](345-moo2-universe-loop-progress.md)。三組576步／兩RET、全部8503原列與32PNG保持；第三組120M仍pending，生成完成／完整開局未驗。下一步另以固定160M明示診斷分支蒐證，先規格審查再續行，主庫RE-first保持。

### 本機忽略證據索引

| 收據／核算 | SHA-256 |
| --- | --- |
| workplace/moo2-setle-343.go | f7bdd7a9f44b6b94aaa8f8430e9c0391d8bb8ff91dca988eb138805a76d49622 |
| workplace/moo2-probe-343-input.txt.gz | 3d7697eeec4e3bf79f6fa933c9c2461ca922a6383f2b9638f709cfaed6383aaa |
| workplace/moo2-vbe-343-input.png | e975c476784977be084da3abb7601363bf290c74b96b26d56fc1af0994dd7dd0 |
| workplace/new-game-343-input-verify.py | 8d45747a105e964ba682da28c7449403278b96e4c22a42c71060dd9a0b628f4a |
| workplace/new-game-343-input-tests.txt | 947caf2ec23f6bbaa20eb0892119ad8d9f304a5dddb43660b42144715a88fcb4 |
| workplace/moo2-probe-343-red.txt.gz | fffa5bf9edfa2a4c3fdbece2852a0069a3307fe6b96bf9ded086fb73cf35ed9b |
| workplace/moo2-vbe-343-red.png | e975c476784977be084da3abb7601363bf290c74b96b26d56fc1af0994dd7dd0 |
| workplace/moo2-343-cpu-narrow-tests.txt | 58fc527d088242338c7e26d45213492c39b3e9c5e2e6eb1b61e52deadf87c46d |
| workplace/full-test-343.txt | 025ca761a9bffc3a248275548ec218ceac829409b5ed63f54991057c97a8e19b |
| workplace/new-game-343-formal-verify.py | d894d095885725194902ad71eb43674fea881039d1b09ead0968d630a246e2d9 |
| workplace/new-game-343-formal-tests.txt | 02411ef852fd4264190bc32c83c36267888f0859cea877fe35fafc9ef84e17e4 |
| workplace/new-game-343-source-verify.py | ea31be2f0852cfb10aac272c595fc5cfc34726e833015620c9b759197dfec617 |
| workplace/new-game-343-source-tests.txt | 272def4e6dfc55e47d6889da34ad152d83d57d68be3fbff995794dc29ed7fcba |
| workplace/new-game-343-backlink-verify.py | 1cfd101d8f127aa499bbbfd768266b55dfd672ba6642202ea63cc14a60911b59 |
| workplace/new-game-343-backlink-tests.txt | e2ca5ba480a18704f04880ac5dbfc90cb5dbcdf2dd185996635a52833a183161 |
| workplace/new-game-343-old338-cli-tests.txt | c5d03a8a5858cdeb915036ef21e2a8f0f3627b6007bfbe3e54c3745fb624b6b1 |
| workplace/new-game-343-old339-cli-tests.txt | e4461c40e971daf4de8338567b57b68f0945355ed595b7c5c09620a2590be774 |

80項回填正對照、343新增26負例、342的25負例與340另兩缺回填負例、341的29／340的28／338及339各26負例均通過。來源與新檔1000:1000。
