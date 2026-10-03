# 344：完整標準SETcc暫存器條件

狀態：**CONFORMED**，限定範圍見驗收節
日期：2026-10-03
範圍：cpu386裸0F90..0F9F全部16條件的八個byte暫存器；沿既有mod3／setReg8。不擴張memory SETcc或前綴，原SETE／SETNE／SETLE與Jcc保持。

## 原阻塞與證據

沿[343-cpu386-setle-byte-register](343-cpu386-setle-byte-register.md)正常input，工具eddee109e0d0d59e311f5c30e26961f84ee36560、CPU SHA-256 76f4b7b3f97156f9422d32e894e55579286f85d2c8b39a26eb90887fa22cc88f。固定DOS1.31 ORION2.EXE SHA-256 4e11be14217b4aafa1839f333bf5eba037f98b0c44e9e4752c96c464c260419f，原位址dosgolem_high_le。原113628944於17D5A0 bytes0F 9F C0 88 C2 80 7D FC 00 75 0E 80 7D F4 00 75拒絕；錯誤後EIP17D5A2不當作原起點，原圖仍為「Generating Universe...」。

343不可變workplace/moo2-probe-343-red.txt.gz SHA-256 fffa5bf9edfa2a4c3fdbece2852a0069a3307fe6b96bf9ded086fb73cf35ed9b已保存終態完整R=[E6 3 1FA FFFF0000 2BDA08 2BDA38 0 1]、六段=[8 188 188 0 20 188]、flags293h／FPU control127F／status0／depth0／stack全0。這是停止後的動態收據；只在確認既有0F9F拒絕尚未fetch ModRM／寫目的或flags後，回查為解碼器輸入。它不是新的StepHook之前caller捕捉。重用充分的既有收據，不只為保存同一CPU拒絕再跑原版。正式344另外保存首個原SETG與下一88 C2兩個Step的完整before／after與observer readonly，確認原解碼器與caller狀態可比。

## 公開CPU契約

[Intel SDM 2B](https://www.intel.com/content/dam/www/public/us/en/documents/manuals/64-ia-32-architectures-software-developer-vol-2b-manual.pdf)印刷頁4-596..4-598，條件符合設目的byte1，否則0，全部EFLAGS保持。ModRM.reg不用、r/m指定目的。標準CPU條件可以依公開規格批次補齊，不為每個條件另解遊戲helper；未在原流程實測的條件只稱契約推導測試，原版實測限定SETG／下一MOV。

| opcode | 條件 |
| --- | --- |
| 90／91 | OF／非OF |
| 92／93 | CF／非CF |
| 94／95 | ZF／非ZF |
| 96／97 | CF或ZF／非CF且非ZF |
| 98／99 | SF／非SF |
| 9A／9B | PF／非PF |
| 9C／9D | SF≠OF／SF=OF |
| 9E／9F | ZF或SF≠OF／非ZF且SF=OF |

## 最小契約與驗收

typed輸入為CF／PF／ZF／SF／OF和ModRM.byte目的，完整fetch ModRM且mod3成功才發布byte。沿八個高低byte別名保持所屬dword其餘24位、其他R／六段／FPU／全部flags／RAM；EIP加3。AF／DF／IF／其他EFLAGS不參與條件也不被改。66／67／段覆寫／F0／F2／F3與所有memory ModRM仍拒絕，不改目的與flags，EIP可以解碼前進，沿工具error模型，不稱硬體exception重啟。

既有343的「其他SETcc拒絕」邊界由344明確擴張；原負例必須轉成有確切預期byte的16條件正例，或由新完整literal truth表覆蓋。不能讓舊錯誤待辦殘留，也不靜默刪除其他memory／prefix負例。Jcc原程式保持，不為共用條件改其資料流。

DRAFT審查343既有收據／拒絕路徑與完整CPU契約，足夠後READY。正式probe最多兩步，原17D5A0首遇臂一次，保存完整R／六段／flags／16code bytes、FPU／VBE與RAM readonly；後續88 C2只將AL複製到DL，不需記憶體目的窗或write特例。正式observer三區塊逆轉後source保持343，原343的兩步與唯一SS write仍須保持。

獨立測試使用16個32-bit字面真值位圖，index由CF／PF／ZF／SF／OF五bit組成，與CPU布林條件表獨立。全部32種定義旗標、AF兩向與兩種非算術context、八目的／八unused reg欄／代表byte；另驗全部256初byte、高低byte鄰居與完整核心／RAM保持／零writes。signed與unsigned CMP數學比較覆蓋92..97／9C..9F、溢位／等值邊界；各Jcc與SETcc都對字面真值表，不能以兩個相同實作互相比就稱獨立驗收。截短、全memory ModRM、11prefix與相鄰非SETcc邊界保持拒絕；原SETE／SETNE／SETLE與其SS byte消費回歸。

固定原EXE乾淨來源Go全套必須通過，然後同343原正常輸入／120M cap續行。核對原SETG的ZF0／SF1／OF0應false、AL E6→0／EIP17D5A3；下一88 C2應DL FA→0、完整EDX1FA→100／EIP17D5A5，所有flags293h與其他R／段／FPU／RAM保持。這些是預期，不提前記成已驗。新CPU前可比8180原343列／30PNG與全部原輸入保持；新畫面、cap或新拒絕按實際保存。生成完成、正式writer、完整開局、RNG與remake同狀態仍未知，主庫RE-first保持。

原素材／LOG／PNG／RAM留忽略workplace，公開只提交通用CPU／自製測試／spec／索引／回填。Go1.24.13固定Docker image sha256:1a6d4452c65dea36aac2e2d606b01b4a029ec90cc1ae53890540ce6173ea77ac，原ZIP／patch唯讀，fresh417根檔／固定MOX.SET、UID1000／network none／2GiB／2CPU／128pids／原版300s／全套600s。沒有外部386／8088實機語料，不稱硬體語料／逐週期一致；固定1996calendar不是RNG seed。收尾核對Git、擁有權與Docker清理。

## READY 證據審查

2026-10-03：new-game-344-input-verify.py 通過。343不可變停止收據的SHA-256、完整R／六段／flags／FPU與readonly重新核對；現有0F9F先fetch extended後拒絕，未fetch ModRM或發布目的／flags。此資料足以建立解碼器輸入，正式344再捕捉caller與兩個Step。條件以Intel SDM為準；測試使用獨立真值位圖與signed／unsigned數學比較。未改玩法、亂數或輸入。允許以下最小CPU與診斷實作。

## 限定驗收：標準SETcc暫存器與原SETG／MOV兩步

344限定CONFORMED：公開CPU契約的16條件暫存器SETcc及原17D5A0／17D5A3兩步已驗。未在原流程逐條實測的條件只稱契約推導測試；生成完成／完整開局與remake同狀態未驗，正式writer／種族特性／正式RNG仍未知。主庫RE-first保持。

**已證實，原兩步**：113628944原0F9FC0成功；flags293h的ZF0／SF1／OF0使SETG false，AL E6→0、EAXE6→0、EIP17D5A3。完整before R=[E6 3 1FA FFFF0000 2BDA08 2BDA38 0 1]與343不可變解碼器初態相同。113628945下一原88C2成功，DL FA→0、EDX1FA→100、EIP17D5A5。除此目的外所有R／六段／全部flags293h保持，FPU control127F／status0／depth0／八stack bitpattern全0、RAM及VBE保持。兩筆readonly真，callback12／12、IRQ26735／26735非活動、pending0，error nil，剩餘budget1→0；一次臂兩步，不代寫或修改輸入。

**已證實，原前綴與來源**：新CPU入口前8180原343列／30PNG逐位元保持；原343 SETLE兩步與唯一SS byte真實write、原342 TEST三步及341原99M按下／99084355放開／245792微秒保持。CPU只擴張裸register SETcc完整16條件；逆轉小區塊後CPU逐byte保持343，Jcc未改。probe三有界observer逆轉後逐byte保持343，原Bus／hooks／calendar／120M cap與正常input未改；舊343其他條件的負例明確轉為字面真值正例，所有原SETLE／prefix／memory／CMP／store測試保持。

**已證實，獨立CPU契約**：524,288組完整32旗標×AF兩向×兩context×16opcode×八byte目的×八unused reg欄×四代表初byte；另65,536組全部256初byte×16opcode×八byte目的×兩flags，驗高低byte鄰居／其餘24位、完整R／六段／FPU／flags與RAM保持、Bus零writes。16個字面32bit真值位圖獨立於CPU布林表。77筆32bit值全部配對×10比較條件×AL／AH，共118,580組數學signed／unsigned CMP→SETcc，涵蓋相等、正負與溢位邊界；512組Jcc直接對同一字面真值表。原SETLE的2,097,152旗標／byte組合與47,432 signedCMP仍通過。全memory ModRM、11prefix、截短及相鄰未支援0FA2／0FA3保持拒絕。

**已證實，實際終態**：同120000000步上限到dosgolem_high_le:17FCE4，unique_sites37433，無新CPU拒絕；終R=[48 8 1 5 2BD9D0 2BD9EC 2BDA74 F]、flags297h。最後32步顯示17FCC3..17FD13迴圈及計數指令，尚不足以證實宇宙生成完成、持久writer或完整開局。原640×480終圖人工檢視仍「Generating Universe...」，PNG d0e387dc900c9955d82dc9c6b21a533d581ac383ba36abca9ce2de28b290f13a、RGB3eeb511abe9d33ff110ce8e478b7d2c5073d36d6775a56622e64651ca8c63fce；不再沿343逐位元相同圖的舊斷言，顯示游標／畫面bytes有變也不當新玩家頁。共享20DDDB仍未命中。probe exit0只表示已到診斷上限，不當完整開局通過。

實際測試：go test -p 2 -buildvcs=false ./internal/cpu386 -run 'TestSETcc|TestSETLE' -count=1 PASS，5.386s；固定DOSGOLEM_MOO2_EXE=/tmp/game/ORION2.EXE的go test -p 2 -buildvcs=false ./... -count=1 PASS，cpu386143.652s／machine5.679s。全套先以git ls-files -z | tar --null -T - -cf - | tar xf - -C /tmp/test-src建立乾淨版控輸入，再cp internal/cpu386/setcc_byte_register_test.go及沿用現存testdata，避免忽略探索main污染；不重寫歷史來源。外部386／8088實機語料未取得，不稱硬體語料或逐週期驗收。

原版本輪只重生一次，先前343充分完整停止收據直接審查為READY輸入。go build -p 2 -buildvcs=false -o /tmp/moo2-probe ./workplace/moo2-probe，沿341全部固定calendar／44M Esc／分離DOS／NEW_GAME／MENU_READY／SETUP_ACCEPT／RACE_HUMANS／RULER_NAME_ACCEPT／BANNER_RED旗標與MAX_STEPS120000000。原版Docker300s／全套600s／2GiB／2CPU／128pids／UID1000／network none，Go1.24.13固定image與fresh417根檔／官方EXE／MOX.SET，原ZIP／patch唯讀；固定1996日期不是RNG seed。舊338 CLI17無效值／正對照與339 CLI22無效值／120M及100M正對照通過。python3 workplace/new-game-344-input-verify.py／new-game-344-source-verify.py／new-game-344-formal-verify.py PASS。前兩者在相應未改CPU／已改CPU階段核對，immutable343來源與收據不重寫。

CPU SHA-256 b5c8bd900047e8f018ad66edaf5ae8e1009e9f6d04ec89dea4000c27f3326b30，setcc_byte_register_test.go 722ea60398d646b4934148ba31067493e0123daca13cc06cfe1d2a3cb93a4cf2，原343測試擴張版a282a1b7b4fc188dc450d4cffcffadd9157e0d8667db5fb0c02573e852ca0b60，正式probe5909237597f3dd80599d85597180bd44e628372f92dcd62573f493d0298dfcd4。workplace/moo2-probe-344-red.txt.gz SHA-256 2724628fa913674f41df2af004c3ec4863b02cfb105695c6864f1bd35f4610a9；workplace/full-test-344.txt SHA-256 b7bcf138095b48d20b4a8ca1f43b8f60531a80ae59c6243463f3de52d6e38554。原素材／圖／LOG／RAM留忽略workplace，不入Git。

### 回填帳與下一步

| 不可變鍵 | 已證實語意 | 證據 | 較早規格 | 必須回填 |
| --- | --- | --- | --- | --- |
| DOS1.31／ORION2.EXE4e11be14…／dosgolem_high_le:17D5A0→17D5A3→17D5A5 | 原SETG AL0、下一MOV DL0、flags保持 | 兩步完整收據／契約真值／固定EXE全套 | 340／341／342／343 | 原17D5A0 SETG與下一MOV已由規格344接通 |

原17FCC3迴圈進度與兩個正常返回已由規格345驗證，見[345](345-moo2-universe-loop-progress.md)。三組576步／兩RET、全部8503原列與32PNG保持；第三組120M仍pending，生成完成／完整開局未驗。下一步另以固定160M明示診斷分支蒐證，先規格審查再續行，主庫RE-first保持。

81項回填正對照與344新增25缺證據負例／其餘三份較早回填6負例、343的26／342的25與340另兩負例、341的29／340的28／338及339各26負例通過。來源與新收據1000:1000。

### 本機忽略證據索引

| 收據／核算 | SHA-256 |
| --- | --- |
| workplace/new-game-344-input-verify.py | 804bdded426e4d3ff9beab8279158b73adacc0236140ce79c5a1e3f5e17619af |
| workplace/new-game-344-input-tests.txt | a173f81e49deb943e361509369a6fa35568ddbe776db8e02e18486dd0e146a88 |
| workplace/moo2-probe-344-red.txt.gz | 2724628fa913674f41df2af004c3ec4863b02cfb105695c6864f1bd35f4610a9 |
| workplace/moo2-vbe-344-red.png | d0e387dc900c9955d82dc9c6b21a533d581ac383ba36abca9ce2de28b290f13a |
| workplace/new-game-344-cpu-tests.txt | 477b24311933a5af41b1c84e2cdcec56003384c8738b7380b302860e1401a88b |
| workplace/full-test-344.txt | b7bcf138095b48d20b4a8ca1f43b8f60531a80ae59c6243463f3de52d6e38554 |
| workplace/new-game-344-formal-verify.py | ab6233d11b6cccabc11027275f4d9183aba27402205659e7e5068368f22c25fe |
| workplace/new-game-344-formal-tests.txt | 8de44fc29170e33c7d1bc9508405fcbf26e66b4379e5841ce3028d9f00ce8947 |
| workplace/new-game-344-source-verify.py | 71f92f1579a000ff8509835534e36dde253188cba1a79e80d6415ca4ea8f3c67 |
| workplace/new-game-344-source-tests.txt | 92d21a4cfb9882b5c2438a99150fbefc53f87235092ebad131a66e6a60245da0 |
| workplace/new-game-344-backlink-verify.py | b866c737eabfa1f96f8ef323d4eeb387ce8ae97673013ab5a0a06c4612962d29 |
| workplace/new-game-344-backlink-tests.txt | 8cd1a7082b4ef0ae71f0338672a08a9b25758462d05f275b45e50d80ee4bd85e |
| workplace/new-game-344-old338-cli-tests.txt | c5d03a8a5858cdeb915036ef21e2a8f0f3627b6007bfbe3e54c3745fb624b6b1 |
| workplace/new-game-344-old339-cli-tests.txt | e4461c40e971daf4de8338567b57b68f0945355ed595b7c5c09620a2590be774 |
