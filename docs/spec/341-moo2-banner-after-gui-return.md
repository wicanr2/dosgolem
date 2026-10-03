# 341：旗幟GUI按鍵返回後的原正常分支

狀態：**CONFORMED，限定原後段按鍵返回與正常放開**
日期：2026-10-03
範圍：在[340原GUI按鍵返回](340-moo2-banner-pressed-consumer.md)同一正式輸入下，只保存原214104 RET到20DB5B且返回AX1後最多192個非callback／IRQ正常CPU步。查證原20DB5E實際分支與後續消費，解釋旗幟仍未選中的原因。初始兩份private trace保持340原輸入；證據足夠後READY修訂本fixture的後段按鍵返回閘門，不改主庫玩法或CPU／平台。

## 原來源與比較點

工具0b141e6cf2a00d86c028054c68d247c9e526a5c0，主庫f93e17de0f1268a2bd6251da3afe1fc654ff6c4b；位址空間dosgolem_high_le。官方1.31 ORION2.EXE SHA-256 4e11be14217b4aafa1839f333bf5eba037f98b0c44e9e4752c96c464c260419f。Go1.24.13、映像sha256:1a6d4452c65dea36aac2e2d606b01b4a029ec90cc1ae53890540ce6173ea77ac。340 source bef46d84f24b674d90e3e84e535485a538bf3e2dba9c0e41f4c08fc27898ab56，原workplace/moo2-probe-340-red.txt.gz a57ac9a85155154f0e026391a09fd547791b95b56cf2afa488212329ee36c02c。

已證實340固定99M按下、99083819原AX3讀到1、99083999原214104 RET到20DB5B返回1、99084000首次合法放開；120M仍SELECT BANNER COLOR，沒有新CPU拒絕。原20DB5E及之後分支尚未記錄，不能從返回1猜旗色已選。持久名稱／旗色writer、完整開局與remake同狀態未知。

## 可丟棄唯讀prototype與停止線

先複製340 tracked probe到既有忽略workplace，只增加五個有界診斷區塊。剝除後整份來源逐位元等於340 HEAD；既有輸入／release閘門／120M完全保持。原GUI RET valid後只臂一次192步，沿340探索的正常步觀測契約，前後排除mouse callback、IRQ0／IRQ1活動及失敗、未完成的IRQ7 passdown；不寫EIP／RAM／按鍵或原選擇。

每步保存原EIP與16bytes、完整R／六段／flags、SS:ESP-16的64bytes、DS:2A121A的16bytes、DS:2A11EC的4bytes、DS:26C4A6的2bytes；這些只是原始定位，不猜正式語意。activationPeek加observer前後RAM SHA證明readonly，保存readable與step error。正常原指令的狀態變化不叫readonly，budget只計入前後無callback／IRQ的原步。不得因trace尚未走出原helper就逐行考古，下一步只追玩家可見消費與原返回框架。

剝除新增trace列後全部340原執行列除既有mtime／DTA四byte／PNG路徑／各次readonly RAM SHA正規化逐值保持；全部340 PNG逐位元保持。只重生本fixture一次，不重跑未受影響六個舊基準或Go全套。若證據足夠修正輸入／平台契約，先審查READY才修改正式source；未知不猜補，不提高cap、不改按下時點或盲重送。

## 初態、執行與權利

新鮮417根檔／MOX.SET／官方EXE，原ZIP／patch唯讀；44M Esc、所有340原輸入、99M press與120M cap完全保持。固定日期1996-01-01不是seed。Docker外層300s／2GiB／2CPU／128pids／UID1000／network none。原素材／PNG／LOG／RAM與可丟棄prototype留本機忽略workplace，不公開。新增本規格同次掛000-index；有解出的舊未知就同次回填340及缺證據guard。主庫RE-first與整款remake／中文化目標保持。

## 首次192步的已證實證據與第二個窄比較點

首次private source SHA-256 567a769ac4d0419cb9f82745652705577015ea56bc83773bc893b8c312f83973，workplace/moo2-probe-341-trace.txt.gz SHA-256 90f6c34305a588cc42759639f771065b3c59ec4128fef445c0e9b91b219cee47。剝除194新增列後全部9968原340列與32PNG保持，192步完整readable／readonly／前後非callback與IRQ、一次臂後耗盡；剝除五區塊source保持340。正常RET與press-release逐值不變，原版仍旗幟頁。

sample1，99093561，原20DB5B TEST AX,AX時AX1；sample2原20DB5E 0F855B050000確實跳到20E0BF。sample3的20E0BF又CALL214075；sample89，99093649原24C31B AX3返回BX0／CX278／DX190，sample139，99093699原214104 RET到20E0C4返回0。20E0CB CMP EAX,2後，20E0CE JNE20E12B確實跳。原20E12B CALL213ABA，213AE6 RET到20E130返回8Bh=139；20E130存原local，20E133 CALL213AE7，靜態原CALL立即數對應返回20E138。192步最後停213AF1→213AF2，後續Y返回與選色判定未知。不從第三次按鍵返回0推論點擊被丟棄。

仍為DRAFT，正式probe不改。第二份可丟棄prototype只在340原GUI返回1後，第一次原20E138正常入口臂192步。這是第一段已證實的CALL213AE7原自然返回位置；不代寫EIP、不延長原cap、不更改按下／放開或重送。保留獨立第一次prototype與原收據，再以同初態取得下一窄視窗，不逐步延長同一budget或深入Watcom helper。

第二視窗沿完整R／段／flags／bytes／stack64／原DS三區的只讀契約，再保存原SS:EBP-64的64bytes raw locals及原offset；每步以before原框架讀前後同一範圍，記錄readable，不把raw locals自動命名為旗色。五個新增區塊剝除後逐位元保持340。仍要求全部9968原340列與32PNG保持，只有這份fixture重生一次，外層300s與120M完全保持。若原位置返回後已足夠解釋阻塞，就停止、審查READY；未達到正式旗色結果，保留未知。

## 第二192步證據、修訂DRAFT與READY審查

第二private source d6c6213e03b1f427da6feef78180255ff2ccc22ea7e396f7543bc02acadae31d，workplace/moo2-probe-341-tail.txt.gz 5add8c37fbd9849eddfca09950f4d0c4dfca077cd439f038cdf144633ec55618。這是另一個獨立192步視窗；全部9968原340列與32PNG仍保持，五區塊剝除後source保持340。原輸入／release與120M未改，資料完整、observer readonly、前後非callback／IRQ。

已證實99093771原20E138保存AX190；SS188:EBP2BDA04，原SS:2BD9C4 locals偏移8／12為139／190。20E13B原CMP DS:26C4E2 word,0為等，20E143 JNZ不跳；20E145寫原word1，20E151／20E15A把原139／190寫DS:26C4DE／26C4E0。這些只標原定位與實際bytes，不把它們命名成正式旗色狀態。

20E160 CALL214075後，99093865原24C31B AX3返回BX0／CX278／DX190。99093915原214104 C3 RET到20E165，SS188:ESP2BD998原top65E12000，返回EAX0；完整R只ESP+4、六段／flags246h保持。20E165 TEST AX,AX，99093917原20E168 0F847D030000 JZ確實跳20E4EB。已證實這個按鍵0會跳到後段路徑；該路徑仍未選色。持按到20E165返回1能否選色為待驗強推論，不從caller名稱或局部branch推定持久旗色writer。

兩份trace與獨立bytes／stack／分支核算足以修訂本測試輸入契約。341轉READY，只修改明示BANNER_RED_CLICK正常持按：保留原340第一個RET20DB5B的完整觀測與原閘門，再增「首個原214104自然RET20E165返回AX1」只讀觀測。原SS188 stack top65E12000、完整R只ESP+4、六段／flags保持、readable／readonly、原step nil，第一個匹配caller若不是1或框架不符即失敗，不挑後續成功返回。只有證據 valid 後才在原首次合法IF／回呼完成／IRQ／20ms條件放開。原99M press／座標／120M與全部既有輸入保持，不寫EIP／RAM／原選擇，不重送，不修改CPU／平台或主庫玩法。

正式source只增加三個有界return觀測區塊與一個本旗標release前置，逆轉後逐位元等於340 HEAD。同新版binary重驗既有338／339 CLI；只重生新旗幟fixture一次，正常畫面與原選擇結果據實記錄。若仍未選色，保留負收據並回RE，不繼續猜持按時間。原程式的後段branch指向後續selection仍強推論，原callers已證實，不冒稱完整GUI規格。

## 限定驗收：原後段按鍵返回與正常放開，後續CPU拒絕

341限定CONFORMED只涵蓋兩個獨立192步原GUI消費、原後段按鍵返回與正常放開；不把它稱為旗色選中、下一頁或完整開局。本規格source沒有修改CPU／平台／主庫玩法，持久旗色writer仍未知。原來源沿既有固定EXE、位址空間與Docker契約。

兩個private prototype各由新鮮417根檔重生一次。每份剝除194新增列後，全部9968原340列與32PNG保持；完整384筆raw資料可讀、observer readonly、前後非callback／IRQ，每份budget192只臂一次並耗盡。五個新增區塊剝除後source逐位元等於340。第一視窗證實20DB5E跳20E0BF，第三次查詢及RET20E0C4為0；第二視窗證實Y190、原X／Y locals139／190、後段RET20E165為0，原20E168 JZ跳20E4EB。首次GUI收到1不足以確保後段仍讀到1，此結論已回填340。

READY後正式fixture只重生一次。99000000按下、99083819首個AX3返回1、99083999首個RET20DB5B返回1均保持340。新增99084354原214104 RET20E165返回EAX低word1，SS188:ESP2BD998 top65E12000，完整R只ESP加4、六段與flags202h保持，readable／readonly／valid true、error nil。99084355首次合法mask1放開，177453134微秒，較按下177207342微秒差245792微秒；原callback11／11已完成，終態callback12／12。沒有代寫EIP／RAM／選擇、不重送、不改原99M按下或120M cap。

正式source SHA-256 16f40cba5262cb7765fdfd8c8478b95d31a7ff38f40438dc255e2d96b6fcef2c，原workplace/moo2-probe-341-red.txt.gz 9e6dc06295aaf43383350abb53e8e9a4e0f883ff261ebbc54cc728e2173e7656。原版沒有跑到120M上限：step99415524在原184694 bytes85 82 19 52 26 00拒絕，error=TEST dword ModRM 82 尚未支援。這是CPU能力缺口；舊340未走到這個位置，不把新覆蓋揭露的拒絕當成按鍵observer突變。靜態ModRM82對應原TEST [EDX+265219],EAX，拒絕收據EAX=EDX=0；原來源與位址保持，不推測該記憶體欄位用途。

probe該次shell exit0，但明確guest_cpu_stop／step_error存在，因此是原版續行失敗收據。沒有以exit0宣稱正常流程通過。終態原PNG人工確認全黑，SHA-256 1610444d26adb3135e7e933dd44912044bb636728af59c70e614945278d3c622，RGB 0b150fd32588b1daca5569992ebe559c0102c837306b1af4c44d35128ec58366等於640×480×3全零像素。共享20DDDB未命中，原旗色選擇／正式writer、下一頁與完整開局未驗。

python3 workplace/new-game-341-formal-verify.py PASS只核算原按鍵返回、首次合法release與實際CPU負結果：7708原列與28PNG、完整550bytes表／核心／RGB保持99M前置，兩個RET與release順序、未達cap／全黑／CPU拒絕均吻合，不聲稱新玩家頁成功。兩份private trace及各自最小bytes／stack／分支語意核算通過；驗證只對不可變340 baseline，不要求當前已修訂source等於舊source。正式source的三區塊／一個release前置逆轉後逐位元保持340；初版source核算腳本逆轉字串誤寫，修正腳本重讀同來源PASS，未重跑原版，也不是產品缺陷。

同新版binary舊338 CLI17無效值及正對照、339 CLI22無效值與120M／100M正對照通過。不重跑未受影響六個舊情境或Go全套，CPU仍保持335。原192步資料、負收據與探索source全部保留本機忽略workplace。

### 回填帳與下一步

| 不可變鍵 | 已證實語意 | 證據 | 較早規格 | 必須回填 |
| --- | --- | --- | --- | --- |
| DOS1.31／ORION2.EXE 4e11be14…／dosgolem_high_le:20DB5E／20E138／20E160→214104→20E165 | 原有按鍵分支、位置返回與後段按鍵TEST／JZ；340此caller讀0，341讀1後才放開 | 兩份原192步與正式重播 | 340 | 原20DB5E分支與20E165後段按鍵返回已由規格341接通 |

原184694記憶體TEST與三步消費已由規格342接通，見[342](342-cpu386-test-dword-memory.md)。原版同輸入已進到宇宙生成圖，後續於17D536的0F9E拒絕；完整開局與正式writer未驗。下一步只補該公開CPU契約，保持120M／輸入，主庫RE-first保持。

| 本機忽略來源／收據／核算 | SHA-256 |
| --- | --- |
| workplace/moo2-gui-after-341.go | 567a769ac4d0419cb9f82745652705577015ea56bc83773bc893b8c312f83973 |
| workplace/moo2-probe-341-trace.txt.gz | 90f6c34305a588cc42759639f771065b3c59ec4128fef445c0e9b91b219cee47 |
| workplace/moo2-vbe-341-trace.png | fdcfce7eb7a39bfa24641a15309ec035208efa205c394173c5cdde37e5e6ec11 |
| workplace/new-game-341-verify.py | 250b770595bd52a5be409fab5e277392f5e0c85d214e247b500edd46bff44b4d |
| workplace/new-game-341-tests.txt | edde141d6857b5fbf8d77b78f92f94aea84589888c611e8bcef807536e93d6b6 |
| workplace/new-game-341-semantic-verify.py | ffee726e9651ebb7ed2aca47c7b0ce2e94a0dfb9acb49099087c0269cd199c32 |
| workplace/new-game-341-semantic-tests.txt | 8e96b2ccf83403b1ebe0f8ab3edc408bcc7774f14a291dff9d1edb41c4099da6 |
| workplace/moo2-gui-tail-341.go | d6c6213e03b1f427da6feef78180255ff2ccc22ea7e396f7543bc02acadae31d |
| workplace/moo2-probe-341-tail.txt.gz | 5add8c37fbd9849eddfca09950f4d0c4dfca077cd439f038cdf144633ec55618 |
| workplace/moo2-vbe-341-tail.png | fdcfce7eb7a39bfa24641a15309ec035208efa205c394173c5cdde37e5e6ec11 |
| workplace/new-game-341-tail-verify.py | 51ea3347af4071a2a7dc05e297a7fd74d88f46cdbaa5844375fe01eff97a25b7 |
| workplace/new-game-341-tail-tests.txt | 6ca1dc5ae73a71e2a9487dfe7d4bbe8d14c0702c5f2b2b48c7ba8b3279cac498 |
| workplace/new-game-341-tail-semantic-verify.py | 970c4d2557d1061103133c88e51f2a66d8c08cb4960f434fe3d6c6a3ddce52bc |
| workplace/new-game-341-tail-semantic-tests.txt | c3e9f640232e0c842ea31d7b89e324a1c561d035e8deef7c719541db0d7ccfa1 |
| workplace/moo2-probe-341-red.txt.gz | 9e6dc06295aaf43383350abb53e8e9a4e0f883ff261ebbc54cc728e2173e7656 |
| workplace/moo2-vbe-341-red.png | 1610444d26adb3135e7e933dd44912044bb636728af59c70e614945278d3c622 |
| workplace/new-game-341-formal-verify.py | 8d5b4e13b276d4453c5dc6035fb38d6e5a515b15eb1f788c02fbd3aea56d4c83 |
| workplace/new-game-341-formal-tests.txt | ed93fd3d40d0ed61dc392c709a21903e8d6c71b08c2f1a1f454468b17fcfe760 |
| workplace/new-game-341-source-verify.py | 3e536a4eb29413605a84c3c641a1bc5a5c631d50dad59f73143e6b4c7e8afc13 |
| workplace/new-game-341-source-tests.txt | 76ee9a8e95dc52a7624e81dee4afbce0c413d51c8f2221f83034d821aed13284 |
| workplace/new-game-341-backlink-verify.py | 0acc309787fa1c2e1bda21ba0a8d65810bd2e55d27f0454160159d66a98be58f |
| workplace/new-game-341-backlink-tests.txt | 878c94271001c71f87e930d866cd28f5429285707c00484543588ded3b48f7aa |
| workplace/new-game-341-old338-cli-tests.txt | c5d03a8a5858cdeb915036ef21e2a8f0f3627b6007bfbe3e54c3745fb624b6b1 |
| workplace/new-game-341-old339-cli-tests.txt | e4461c40e971daf4de8338567b57b68f0945355ed595b7c5c09620a2590be774 |

78項規格回填、341新增29缺證據負例、340 28負例、338／339各26負例與兩CLI通過。24份來源／收據與新檔1000:1000，gofmt／Git差異通過，工具root-owned／誤建.md目錄空，Docker兩工作區掛載篩選空，沒有本輪遺留容器。
