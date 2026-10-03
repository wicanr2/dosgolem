# 340：旗幟首次pressed查詢後的原正常GUI消費

狀態：**CONFORMED，限定原GUI按鍵返回與正常放開**
日期：2026-10-03
範圍：先保存[339-moo2-banner-red-normal-click](339-moo2-banner-red-normal-click.md)同一輸入下最多192個非callback／非IRQ正常CPU步；查證原caller返回框架後，經READY修訂本fixture的放開閘門。只驗證原GUI按鍵返回，不把它算成選色、主庫玩法或完整開局。

## 蒐證起點與當時未知

工具7d569e0392fd9061576da8395d3c8b4d10e0886d／主庫3526b6da8334626a992cfa3c13689b318befb664，Go1.24.13，位址空間dosgolem_high_le。官方1.31 ORION2.EXE SHA-256 4e11be14217b4aafa1839f333bf5eba037f98b0c44e9e4752c96c464c260419f。339來源d131475620cda95f77a6c9846dc7a494a5a60bae778196c6ea49555a79b5d4f3、原workplace/moo2-probe-339-red.txt.gz SHA-256 9505df160613d748632e9e43a7e47e73945e9ed56948a9e66f951fbd9ed49965、終態PNG fdcfce7eb7a39bfa24641a15309ec035208efa205c394173c5cdde37e5e6ec11。

已證實99083819原callsite24C31B的INT33 AX3返回BX1／CX276／DX190、99083854放開；120M仍旗幟頁。共享20DDDB未命中。原選色consumer、返回值與持久旗色writer未知。紅旗對應index1仍強推論，正常座標命中不算原選色結果。沒有新CPU拒絕，不能用裝置callback完成代替GUI消費。

## 可丟棄唯讀prototype

先複製339 tracked probe到既有忽略workplace，增加有界readonly診斷；剝除所有新增區塊後完整來源逐位元保持339 HEAD。首次pressed查詢只臂一次192步。CPU.Step前及後都排除callback／IRQ活動；記錄未觸發的剩餘budget，不從中途synthetic dispatch猜原指令。不得額外讀取CPU hook／改Bus來擷取，只用既有直接peek與activationPeek，保留原handler及輸入。

每步保存原EIP、16bytes指令、完整R／六段／flags、原stack offset與64bytes、DS:2A121A的16bytes事件、DS:2A11EC的4bytes計數、DS:26C4A6的2bytes共享word；標示readable、observer readonly及原step error。這些是原始定位，未證明用途的欄位不另造語意名。observer前後RAM SHA與FPU／VBE／Bus／核心保持；正常CPU指令的前後變化不叫readonly。最多192筆，不逐行翻譯完整renderer或標準函式庫。

依原trace找GUI分支與原正常返回框架，再決定最小static slice；未解出原因就保持DRAFT，不寫production flag或猜正式規則。若新證據足以修正輸入契約，先證據審查、READY再實作，重新核對原正常路徑與實際下一頁。

## 驗證、停止線與權利

剝除新trace列後，完整339原列除mtime／DTA四byte／PNG路徑／各次RAMhash的既定正規化保持，所有339 PNG逐位元保持。只重生本旗幟fixture，不重跑未受影響六條舊基準或CPU全套；CPU／平台與正常輸入不變。初態新鮮417根檔／MOX.SET／官方EXE、44M Esc／固定日期1996-01-01及338全原輸入保持；日期不是seed。

沿Go1.24.13映像sha256:1a6d4452c65dea36aac2e2d606b01b4a029ec90cc1ae53890540ce6173ea77ac，Docker外層300s／2GiB／2CPU／128pids／UID1000／network none、原ZIP／patch唯讀。原素材、PNG、RAM、LOG及prototype留本機忽略workplace，不公開。新規格同次接入000-index；若解出舊未知，同次回填339與缺證據guard。主庫RE-first保持，完整開局／RNG／remake同狀態仍未知。

IRQ排除契約：CPU.Step前後IRQ0／IRQ1均非活動、非failed且started等於completed；既有OPL ports的IRQ7Passdowns等於IRQ7Returns。IRQ7的原dispatch已在入口計數、正常返回後計return，不從EIP名稱猜ISR。觀測不改任何dispatcher。

## 原正常消費證據與修訂DRAFT

private prototype SHA-256 5bb1ec1da69f80a4b7831e05f0a48aa355d8a21383a6c890ae057289d00709ea；workplace/moo2-probe-340-trace.txt.gz SHA-256 dec37ddf557a2e35092e006d91b529966064a8f3f81efa5132f19f571d2b59d9。剝除194新增列，全部10012原339執行列與32PNG保持；192筆資料完整可讀、observer readonly、前後非callback／IRQ，budget一次臂／耗盡。CPU／平台與輸入未改。

已證實：sample32，99083851，原22F26F RET到2139D2；sample34原234A48 STI到234A49。339在下一步99083854放開，原callback排送期間事件word由0100變0000。sample35於99093415原234A49 RET返回2139D7；sample43原2139DF RET到20DB56。

sample44原20DB56 CALL214075。sample130在99093510又經24C31B INT33 AX3，返回BX低word0、CX278、DX190。sample168原2140D9 bytes66A1E4382A00從DS:2A38E4讀AX0；sample169原2140DF AND EAX,3仍0，2140E4保存於原local。sample172原2140F9取local返回EAX0，sample180原214104 RET返回20DB5B；其原SS:ESP堆疊top為20DB5B。sample181原20DB5B TEST AX,AX，sample182原20DB5E JNZ20E0BF不跳，落20DB64。後續213C48未知consumer只作下一分支入口，不深挖Watcom 22F1F3的stack helper。

已證實「首個裝置AX3 pressed查詢」與「20DB56 caller使用的214075返回」是兩個比較點；前輪放開在後者之前，後者確實讀到0。強推論：以原214104自然RET到20DB5B且EAX低word1作正常放開閘門，能保留 caller 將使用的按下結果；是否選色成功及下一頁仍待原正常重播，不從分支位址猜持久旗色writer。

修訂只適用BANNER_RED_CLICK明示情境。維持99M press／原完整前置／座標／120M cap／20ms／同target／IF／回呼完成／IRQ條件；先觀察原214104 RET、完整核心、原stack top與返回20DB5B，再於首次合法點放開。原CPU指令自行返回，probe不寫EAX、EIP、stack、原選擇或device平台。原返回未達到1或錯框架就明示未放開，絕不改初態或重送。

## 修訂證據審查與READY

192原正常步已完整核算，選擇相關最小鏈為20DB56 CALL214075 → 24C31B原AX3 → 2140D9讀DS:2A38E4 → 2140DF AND3 → 2140F9取local → 214104 RET20DB5B → 20DB5B TEST AX → 20DB5E JNZ20E0BF。原0結果及不跳均已證實；按下維持至caller得到1的下一畫面仍為待驗強推論，不推定正式旗色資料欄位。

固定BANNER_RED_CLICK原99M初態與20ms／原callback／IRQ／120M保持。正式probe只增加原214104自然RET證據：before SS188:ESP堆疊top 20DB5B、原opcode C3、EAX低word1；after EIP20DB5B，完整R只允許ESP+4、六段／flags不變，observer只讀。首筆正常RET到該caller後若框架或值不符即失敗，不延後挑成功；符合才允許下一個首次合法正常release。不寫任何原選擇或CPU資料。

以上足以建立工具輸入契約，340轉READY；只實作本明示分支，不修改主庫玩法。若原版仍未選色，保留未知與本收據，再由下一個具體原消費證據開窄任務，不增加或改點擊來湊成功。正常fixture只重生一次，原192筆探索probe／負收據及339原基準保持本機可回查。

## 限定驗收：原GUI按鍵返回與正常放開，選色未完成

340限定CONFORMED只涵蓋正常GUI按鍵讀取返回、原框架、正常放開與負結果。原版仍未選色，不把返回1當作旗色選中或完整開局。192步舊輸入探索已證實首個AX3之後還有第二次查詢；339放開後第二次讀到0，caller也收到0。原20DB5E JNZ20E0BF在那份負收據不跳，落20DB64。340當輪正式重播沒有記錄該分支；後續原20DB5E分支已由341記錄，旗色正式寫回仍未證實。

固定99M初態與99M press保持。99083819原24C31B第一次AX3仍返回BX1／CX276／DX190；99083999原214104自然RET到20DB5B，EAX低word1、原SS188:ESP2BD998 top5BDB2000。完整R只ESP加4，六段與flags202h保持，stack observer readable／readonly／valid均true。原C3 opcode出自同固定EXE的192步私有trace；正式RET收據以原入口、返回位址、stack與完整核心驗證，未另取opcode。99084000首次合法mask1放開，177452779微秒，相較按下177207342微秒差245437微秒；callback11／11完成，正常原caller即將使用AX1。不改按下座標、cap、seed或原選擇資料，不重送。

正式來源SHA-256 bef46d84f24b674d90e3e84e535485a538bf3e2dba9c0e41f4c08fc27898ab56，workplace/moo2-probe-340-red.txt.gz SHA-256 a57ac9a85155154f0e026391a09fd547791b95b56cf2afa488212329ee36c02c。原版120M到228E00，R=[74 EE BE 89 2BD9B8 2BDA14 2C0B58 2C03CC]、段=[8 188 188 0 20 188]、flags287h；callback12／12、IRQ28866／28866完成，非活動／未failed。無新CPU拒絕，共享20DDDB未命中，原持久旗色writer未知。終態PNG仍SELECT BANNER COLOR，與339終圖逐位元一致：PNG fdcfce7eb7a39bfa24641a15309ec035208efa205c394173c5cdde37e5e6ec11，RGB 8f140b5eb4f65e2ed42744f269e5e2505a395feefb91b7dc2c829a81e398cb97。

python3 workplace/new-game-340-verify.py PASS：192筆完整readonly正常步、全部10012原339列與32PNG保持，剝除5個私有trace區塊來源保持339。python3 workplace/new-game-340-return-verify.py PASS：額外輸入前7708原列與28PNG保持，完整550bytes原表／核心／RGB、首個AX3以及原RET返回1與release順序吻合。首次核算誤從GUI checkpoint讀IRQ欄位，修正為同收據的setup_table_snapshot後乾淨重讀通過，未重跑原版，也不是產品缺陷。

python3 workplace/new-game-340-source-verify.py PASS：只增加3個有界原RET觀測及本明示旗標一個release閘門；剝除後其餘來源逐位元等於339 HEAD。未改CPU／平台／主庫玩法，不重跑未受影響六個舊情境或Go全套。正式同binary重驗舊338的17拒絕與正對照、339的22拒絕及120M／100M正對照通過。

### 回填帳與下一步

| 不可變鍵 | 已證實語意 | 證據 | 較早規格 | 必須回填 |
| --- | --- | --- | --- | --- |
| DOS1.31／ORION2.EXE 4e11be14…／dosgolem_high_le:20DB56→214075→214104→20DB5B | GUI讀按鍵的原RET；339返回0、340返回1，完整stack／核心已驗 | 本規格192步與正式重播 | 339 | 原GUI按鍵返回與正常放開已由規格340接通 |

原20DB5E分支與20E165後段按鍵返回已由規格341接通，見[341](341-moo2-banner-after-gui-return.md)。兩個獨立192步已證實原後段仍查按鍵，340返回0並JZ到20E4EB；341等該caller返回1才放開，實際走到184694的85 82記憶體TEST拒絕並全黑。原184694記憶體TEST與三步消費已由規格342接通，見[342](342-cpu386-test-dword-memory.md)；正常已到宇宙生成圖；SETLE後續由[343](343-cpu386-setle-byte-register.md)接通，當前0F9F拒絕。保持座標／cap／輸入。旗色選擇／持久writer、下一頁／完整開局及remake同狀態未知，主庫RE-first保持。

| 本機忽略來源／收據／核算 | SHA-256 |
| --- | --- |
| workplace/moo2-poll-340.go | 5bb1ec1da69f80a4b7831e05f0a48aa355d8a21383a6c890ae057289d00709ea |
| workplace/moo2-probe-340-trace.txt.gz | dec37ddf557a2e35092e006d91b529966064a8f3f81efa5132f19f571d2b59d9 |
| workplace/moo2-vbe-340-trace.png | fdcfce7eb7a39bfa24641a15309ec035208efa205c394173c5cdde37e5e6ec11 |
| workplace/new-game-340-verify.py | 3ecb7f2b3562c7a76967baaa2f8e599989ecaecc52971195e76171b2d1cce931 |
| workplace/new-game-340-tests.txt | 068dee19b4d062a8015fd0cf0cf9d339491f4b75f0ba4cd869e4814127500ade |
| workplace/moo2-probe-340-red.txt.gz | a57ac9a85155154f0e026391a09fd547791b95b56cf2afa488212329ee36c02c |
| workplace/moo2-vbe-340-red.png | fdcfce7eb7a39bfa24641a15309ec035208efa205c394173c5cdde37e5e6ec11 |
| workplace/new-game-340-return-verify.py | 39147738f42c0cd55e74387cab7d0326e3e0dbb65c309561f369033402cdb434 |
| workplace/new-game-340-return-tests.txt | a12ab0e6b44343af80eaf272c9804adfc95e29b19bb8fe03eb7c5061ee9cbe09 |
| workplace/new-game-340-source-verify.py | ddc521e2f33c0f9768459786ead5ebd0492024cc0052cfe5820d6e2a0fa45d57 |
| workplace/new-game-340-source-tests.txt | 5b856aaa7bf692c6070ed6c5a414261d7a0ad72ac44d14d67e1257ff39e904de |
| workplace/new-game-340-backlink-verify.py | 3f693b0fce6d2002a94d6e6ac7b7aa42adefe7b95b62ac689fa6e17f1634d1e7 |
| workplace/new-game-340-backlink-tests.txt | 5681c96b742c39d09b81e95d20a419670e70f1081cc30c9efb3d876d991af732 |
| workplace/new-game-340-old338-cli-tests.txt | c5d03a8a5858cdeb915036ef21e2a8f0f3627b6007bfbe3e54c3745fb624b6b1 |
| workplace/new-game-340-old339-cli-tests.txt | e4461c40e971daf4de8338567b57b68f0945355ed595b7c5c09620a2590be774 |

77項規格回填、340新增28缺證據負例、338／339各26負例與三CLI通過；不把原GUI按鍵返回當作選色完成。原來源／新檔1000:1000，gofmt／Git差異通過，工具root-owned／誤建.md目錄空，兩工作區掛載篩選沒有遺留容器。
