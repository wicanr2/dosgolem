# 342：記憶體dword與暫存器的TEST

狀態：**CONFORMED，限定CPU契約／原三步／正常宇宙生成畫面**
日期：2026-10-03
範圍：cpu386的85 /r記憶體dword唯讀邏輯比較，沿既有32位ModRM／SIB解碼。原暫存器85的word／dword契約保持；不擴張byte84、記憶體word、段覆寫、67、F0／F2／F3。

## 阻塞與公開契約

沿[341-moo2-banner-after-gui-return](341-moo2-banner-after-gui-return.md)正常輸入，工具b07cda7188d7139da612f143ba68ea13cca21958、CPU SHA-256 967de02753e0dce276413fe67a085e6df16789b52c948999d30ed26c3bb4e6a4。官方DOS1.31 ORION2.EXE SHA-256 4e11be14217b4aafa1839f333bf5eba037f98b0c44e9e4752c96c464c260419f。位址空間dosgolem_high_le。99415524於184694 bytes85 82 19 52 26 00拒絕；錯誤後EIP184696不當作原指令起點。ModRM82的原operand為DS:[EDX+265219]、EAX。其後原bytes0F95C0／C3對應SETNE AL／RET，尚未執行。欄位用途未知，不追helper內部。

[Intel SDM 2B](https://www.intel.com/content/dam/www/public/us/en/documents/manuals/64-ia-32-architectures-software-developer-vol-2b-manual.pdf)印刷頁4-679／4-680：85 /r讀r/m32及r32、AND結果捨棄；CF／OF清零，SF依bit31、ZF依零值、PF依低byte偶同位，AF未定義。[80386原廠TEST](https://www.ardent-tool.com/CPU/docs/Intel/386/manuals/prref386/TEST.htm)同意唯讀與五旗標。AF清除只沿工具模型，不能聲稱硬體定值。既有[287-cpu386-test-dword-register-imm32](287-cpu386-test-dword-register-imm32.md)的獨立交集與定義旗標oracle可重用。

## 最小實作與驗收契約

無前綴memory85沿decodeAddress32取得DS／SS及32位有效位址，完整readSegment32成功才setLogicFlags，沒有writeSegment32。未知selector、段末或逐byte bus讀取失敗不發布新旗標；EIP可已解碼前進，保留現有工具error模型，不聲稱硬體exception重啟。所有R／六段／FPU／資料保持，正常EIP依完整指令長度前進。ESP／EBP實際base用SS；無base absolute用DS，SIB的ESP index忽略。暫存器85保留原prefix規則，F2在Step早期已拒絕；新memory形式拒絕word／F2，F3／段覆寫／67／F0沿原閘門拒絕。

DRAFT先保存同341輸入原184694首遇、完整R／段／flags／16code bytes／原DS來源四bytes／SS:ESP四bytes，observer唯讀。未改CPU仍應相同拒絕，全部341舊列與PNG保持。來源實際可讀且公開契約充分後才READY。

READY後加入獨立bit交集／五旗標oracle測試：全部來源暫存器、單bit／補數／高位／零、全部低byte配對、不同初旗標；全部ModRM與SIB／相異DS及SS、32位繞回、非對齊及最後完整dword。唯讀descriptor應成功；未知selector／段末／bus逐byte／截短／前綴失敗保持R／flags／RAM且零writes。既有85暫存器word／dword與F2拒絕保持。

固定原EXE的Go全套必須全部通過；同341完整正常輸入120M上限續行，至184694前所有舊列／PNG保持，再用最多三個原步保存TEST／SETNE／RET與caller地址。不改input、calendar、cap或遊戲資料，不加位址特例。結果／後續畫面或新CPU拒絕按實際記錄；日期不是RNG seed，三步消費不等於旗色writer或完整開局。主庫RE-first保持。

原素材／LOG／PNG／RAM留忽略workplace；公開只提交通用CPU、測試、spec、索引與證據回填。Go1.24.13固定Docker image sha256:1a6d4452c65dea36aac2e2d606b01b4a029ec90cc1ae53890540ce6173ea77ac，原ZIP與patch唯讀，UID1000／network none／2GiB／2CPU／128pids／有界逾時。輪末核對Git、擁有權與Docker清理。

## 未改CPU初態與READY審查

未改CPU的單次正常重播，private三區塊逆轉後source保持341。全部7849原341列、29PNG保持；原來源／stack可讀，observer完整CPU／FPU／VBE及RAM保持。收據workplace/moo2-probe-342-input.txt.gz SHA-256 923a2fe5b6f669c83579aecfb3b2b14e84fc143948600855133dbdc85e12845e；source ebc395db43be26ea4423639f785ceb2f0e8753f44ef5f90cf6cf7f45a5434481。python3 workplace/new-game-342-input-verify.py PASS。

99415524原184694完整R=[0 0 0 0 2BDB68 2BDB70 0 0]、段=[8 188 188 0 20 188]、flags202h；DS188:265219四bytes01000000即1，mask EAX0。SS188:ESP2BDB68四bytesE3461800即返回1846E3。callback12／12、IRQ22676／22676，非活動、pending0，readonly=true；原CPU仍拒絕在184696、flags202h保持。欄位用途未知。

公開TEST契約與實際來源足夠，342審查READY。預期交集0，flags246h、EIP18469A、核心／RAM保持；下一SETNE AL應0、EIP18469D，RET只ESP加4並EIP1846E3、flags保持。這些是驗收預期，不寫成已通過。初讀opcode局部guard曾疑F2暫存器可接受；核對Step早期prefix閘門證實F2已拒絕，DRAFT文字在READY前修正，沒有改CPU放行F2。

## 限定驗收：記憶體TEST／SETNE／RET與正常宇宙生成畫面

342限定CONFORMED只涵蓋公開CPU契約、原三步消費與正常輸入後的宇宙生成畫面；正式writer、完整開局與remake同狀態未驗。沒有改主庫玩法／平台／輸入／calendar／120M cap；固定日期不是RNG seed。

**已證實，原最小消費**：99415524原184694的85 82完整讀DS188:265219原四bytes01000000即1，EAX0，交集0，定義旗標CF／OF／SF=0、ZF／PF=1，flags202h→246h、EIP18469A，完整R／段／FPU與RAM保持。AF未定義，清除只驗工具模型。99415525原0F95C0將AL設0、EIP18469D，其他R與flags246h保持。99415526原C3讀SS188:ESP2BDB68四bytesE3461800，ESP加4到2BDB6C，原EIP1846E3；其餘核心／旗標／RAM保持。三筆readonly／step_ram_unchanged／來源／stack readable真，error nil，callback12／12、IRQ22676／22676非活動／pending0。欄位用途未知，不深入helper。

正式重播在CPU新入口前7789原341列與28PNG保持，所有press／release及原後段RET保持。7789為舊late_startup_platform label=stop前的可比原列；舊stop診斷不是CPU新入口前資料，不把新續行和舊終態比較。private初態探針仍保持全部7849原341列／29PNG。python3 workplace/new-game-342-input-verify.py／new-game-342-formal-verify.py／new-game-342-source-verify.py均PASS。CPU只增加85 memory分支，逆轉分支後CPU逐位元保持335；三有界observer逆轉後source逐位元保持不可變341。沒有位址特例、資料代寫或亂數重擲。

**已證實，正常玩家畫面**：同341輸入原99M選色按下與99084355放開後，原版實際進到640×480「Generating Universe...」畫面，人工確認原邊框／綠色格線／文字，終圖不是全黑。PNG SHA-256 e975c476784977be084da3abb7601363bf290c74b96b26d56fc1af0994dd7dd0，RGB 353171a9bce8ad97f55e3ee444a0f9f8017bf44e531b5616f42393ff976ddf69。只確認畫面，不猜原持久旗色值或生成完成。

原版113628909於dosgolem_high_le:17D536 bytes0F 9E C0 88 45 FC 89 C8 99 31 D0 29 D0 0F BF 55拒絕，error=0F 9E 尚未支援；原指令起點17D536，解碼後EIP17D538。EAX28／EBXFFFFFFC2／ECXFFFFFFDA／EDX0、flags206h，終態完整R=[28 FFFFFFDA 0 FFFFFFC2 2BDA08 2BDA38 0 1]、段=[8 188 188 0 20 188]。尚未120M，不把probe shell exit0當正常開局通過。共享20DDDB未命中，持久writer未知；callback12／12、IRQ26735／26735終態均非活動。

**CPU測試**：全部來源暫存器、32位單bit／補數／高位／零、全部低byte配對與不同初旗標；全部ModRM／SIB相異DS／SS、負disp8／無base absolute／ESP忽略index、32位繞回／非對齊／最後完整dword通過。預期交集與五旗標重用287逐bit獨立oracle，AF模型另驗；整體RAM／R／六段／FPU保持且寫入拒絕bus零writes。唯讀descriptor成功，未知／段末／逐byte讀失敗／截短與前綴失敗不發布新旗標，既有register85 word／dword與F2拒絕、byte84 memory拒絕保持。自製SETNE／RET兩方向通過，原三步另有上述自然收據。沒有386實機語料，不稱硬體逐週期對拍。

go test -p 2 -buildvcs=false ./internal/cpu386 -run 'TestTESTDwordMemory|TestTESTDwordRegisterImmediate|TestORDwordMemory' -count=1 -v PASS，cpu386 0.909s。固定DOSGOLEM_MOO2_EXE=/tmp/game/ORION2.EXE的go test -p 2 -buildvcs=false ./... -count=1 PASS，cpu386 55.710s、machine2.914s。第一次全套因忽略workplace多份探索main重複編譯失敗，屬驗證輸入污染，CPU386本身61.132s已PASS。修正後同映像／命令在/tmp/test-src乾淨輸入重跑：git ls-files -z | tar --null -T - -cf - | tar xf - -C /tmp/test-src，再複製本輪新自製測試並沿現存testdata；未改歷史探索檔或其hash。8088外部實機語料未取得，CPU386是公開契約推導測試與固定原版續行。沒有把skip當硬體驗收。

兩次原版各以新鮮417根檔／官方EXE／固定MOX.SET、Go1.24.13建置；第一次DRAFT原來源，第二次READY後CPU正常續行。Docker原ZIP／patch唯讀、network none／UID1000／2GiB／2CPU／128pids，原版300s／全套600s。首個CPU編譯未正確使用readSegment32的selector／bool簽名，建置即拒絕、未跑原版；修正後窄測／全套與正式重播通過。初版formal核算把舊stop診斷列算進入口前，修正邊界後重讀同原收據通過；沒有為核算重跑原版。兩CLI舊338的17拒絕與正對照、339的22拒絕與120M／100M正對照保持。不另跑六舊情境，正常可比前綴／全套已充分驗證本分支。

CPU SHA-256 330baaf9917f4534906797e9eb484136b7297fc6a94343252db9b6f8da97c5f7，自製test_dword_memory_register_test.go 58bfb07a0836f04c5600a214ac4c53efac67cddc2a0ed45698622b58a1357e5f，probe ebc395db43be26ea4423639f785ceb2f0e8753f44ef5f90cf6cf7f45a5434481。所有原素材／PNG／LOG／RAM留忽略workplace。

### 回填帳與下一步

| 不可變鍵 | 已證實語意 | 證據 | 較早規格 | 必須回填 |
| --- | --- | --- | --- | --- |
| DOS1.31／ORION2.EXE 4e11be14…／dosgolem_high_le:184694→18469A→18469D→1846E3 | 原DS來源1、mask0、TEST五旗標／SETNE AL0／RET，後續正常宇宙生成圖 | 原三步與全套／342正式收據 | 341、340 | 原184694記憶體TEST與三步消費已由規格342接通 |

下一步僅核對0F9E SETLE的公開CPU條件與既有SETcc，保存原17D536完整初態／flags／AL與下一原byte store最小消費，經DRAFT→READY補通用能力，再保持同輸入／120M續行。原旗色／姓名正式writer、typed種族特性、生成完成／完整開局、正式RNG與remake同狀態未知；主庫RE-first與整款remake／中文化目標保持。

正式原workplace/moo2-probe-342-red.txt.gz SHA-256 9181e24bfbd29931985b3e3966c5081cd6b6f367d6faaac8d13e33b9dd155181；乾淨全套workplace/full-test-342.txt SHA-256 2877e41d8c9448a37a589fe7e6e35ed8be2059b21cb6a4c5cac7d86101ca663c。

### 本機忽略證據索引

| 收據／核算 | SHA-256 |
| --- | --- |
| workplace/moo2-test-memory-342.go | ebc395db43be26ea4423639f785ceb2f0e8753f44ef5f90cf6cf7f45a5434481 |
| workplace/moo2-probe-342-input.txt.gz | 923a2fe5b6f669c83579aecfb3b2b14e84fc143948600855133dbdc85e12845e |
| workplace/moo2-vbe-342-input.png | 1610444d26adb3135e7e933dd44912044bb636728af59c70e614945278d3c622 |
| workplace/new-game-342-input-verify.py | 989a598e4839891f998b4fc0f13b2140fb6ab56d5952160a70a2b6d3950deb79 |
| workplace/new-game-342-input-tests.txt | 544fe5b937e8859b5d01a98c1610047b75b3e2fca75e793da0f8eeb14930c1eb |
| workplace/moo2-probe-342-red.txt.gz | 9181e24bfbd29931985b3e3966c5081cd6b6f367d6faaac8d13e33b9dd155181 |
| workplace/moo2-vbe-342-red.png | e975c476784977be084da3abb7601363bf290c74b96b26d56fc1af0994dd7dd0 |
| workplace/moo2-342-cpu-narrow-tests.txt | 44fe2a818c8adf0c69205fc8f6e572d21b83b18f32a40c27d3d521333957b807 |
| workplace/full-test-342.txt | 2877e41d8c9448a37a589fe7e6e35ed8be2059b21cb6a4c5cac7d86101ca663c |
| workplace/full-test-342-unclean.txt | 06d8759f8cd6a26869658099871025f69b90c7c2ca6d94493f334a91e53fbfba |
| workplace/new-game-342-formal-verify.py | d6598337b5a5ca39bb98650bf39b1f1065d3f02c1a5778be55072f3614e47872 |
| workplace/new-game-342-formal-tests.txt | c2922c3e4f2ae1fbf278e3eaffca267bafaf8fdf23e32fb7b26d07b3a3f15dde |
| workplace/new-game-342-source-verify.py | 990afbbaf981fb20697695cfce90bfbeb179512a9bf6fb0a97033fc170042571 |
| workplace/new-game-342-source-tests.txt | 600a3b0bd26b0c5f0019a819b844c50ca9cd07e8b01aad46872d5324d3059365 |
| workplace/new-game-342-backlink-verify.py | b783c3b7d664d386db0c9e7d05d6ccdae3ec0f6bd330abf348bb9553b483198e |
| workplace/new-game-342-backlink-tests.txt | 4a23e596a6b1b0adbc563ba4d22dbed8a872575bdebfd49b3158b09a891eebf1 |
| workplace/new-game-342-old338-cli-tests.txt | c5d03a8a5858cdeb915036ef21e2a8f0f3627b6007bfbe3e54c3745fb624b6b1 |
| workplace/new-game-342-old339-cli-tests.txt | e4461c40e971daf4de8338567b57b68f0945355ed595b7c5c09620a2590be774 |

79項回填正對照、342新增25負例與340另兩缺回填負例、341的29負例、340的28負例、338／339各26負例均通過。來源與新檔1000:1000。
