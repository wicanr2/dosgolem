# 339：旗幟頁的正常紅色輸入與按鍵查詢

狀態：**CONFORMED**，限於正常裝置輸入與原按鍵查詢，選色消費未完成
日期：2026-10-03
範圍：固定1.31旗幟頁十筆原表、99M正常press、原首次pressed輪詢後release。觀察上限120M僅明示旗標啟用；原版仍停旗幟頁，選色與下一頁未完成。紅色只是重播fixture，不改正式預設或主庫玩法。

## 首次來源與初版契約

工具起點0c88d04cb59d2b7bfc716e953cc529c54b5fb6e2，主庫起點f079ef801ab20d3ab1949ad9ee8d5e9693ee9eea。[338-moo2-ruler-name-normal-confirmation](338-moo2-ruler-name-normal-confirmation.md)已證實正常名稱確認與SELECT BANNER COLOR；官方1.31 ORION2.EXE SHA-256為4e11be14217b4aafa1839f333bf5eba037f98b0c44e9e4752c96c464c260419f，位址空間為dosgolem_high_le。

沿338新鮮417根檔／MOX.SET、44M Esc／1996-01-01、兩組NEW GAME、80M設定ACCEPT、90M Humans與95M名稱ACCEPT。固定日期不是seed。既有workplace/moo2-probe-338-ruler.txt.gz SHA-256 21e94e53b1a4f43b3a6fe736490412ea291bb2c3796c922c1940a6dbd2a5dcbb。原100M旗幟表count10／pointer298848／bias0／stride55，完整550bytes SHA-256 978afb91aaed9e0cb37672a66352e2ae515b382d5b6f472da9238e09fb650dfb；index1範圍96／144／179／242，對照紅色為強推論。100M終態不能當同cap內的輸入前置。

先以可丟棄的唯讀探針在預先選定98M／99M保存原畫面、當時實際表、完整R／六段／EIP／flags／FPU／VBE與callback／IRQ；原輸入與100M cap保持。剝除新唯讀列後核對338 ruler全部原行及圖，準備READY。138／190與139／190是由原矩形決定的候選，語意在正常消費前保持強推論；其餘表欄位未因指標殘值推定用途。

## 初版READY後的正常輸入與待驗範圍

明示DOSGOLEM_MOO2_BANNER_RED_CLICK=1，只接受值1與完整RULER_NAME_ACCEPT_CLICK依賴。固定時點先核對原table／header／DS／RGB、已完成名稱按下／放開、IF／callback／IRQ及完整核心。原前置不符即停止，不延後挑可通過的初態。

一次正常press／release只能經InjectMouseEvent，至少20ms且回呼完成後首次可送放開。按下前mask2B；放開時保留同target、IF、IRQ及pending條件，允許已按下後mask1或2B，沿338已證實平台事件契約，不代寫原選擇或EIP。若其他mask或未完成回呼，明示未放開，不猜補結果。

最多觀察第一筆原20DDDB共享選擇store；未命中就明示未知，不用下一頁捏造持久旗色writer。保存實際下一頁與新CPU拒絕。正式規則／資料包／存檔鏈未到達的部分仍未知，不宣稱remake同狀態或整款開局完成。

## 測試比例、工具與權利

CPU／startup／provider／matcher／mouse callback保持335，不重跑未受影響的CPU全套或六條較早原版基準。原探針除339旗標與有界分支外，完整來源必須逐位元保持338 HEAD；旗標關閉的原ruler流程由本輪唯讀蒐證核對全部原行及PNG。新增red情境正式重生一次，額外輸入前核對同ruler獨立來源，結果按實際證據判定。六條較早基準仍保留338原來源、收據與驗收範圍，不冒充339已重生。

原資料 → 唯讀10筆表 → 原正常選擇 → 原消費／下一UI；typed旗色、持久player／save與remake路徑未知。證據足夠後停止，不深挖renderer／helper，不重開已完成名稱放開／設定ACCEPT／SCASW。

沿Go1.24.13映像sha256:1a6d4452c65dea36aac2e2d606b01b4a029ec90cc1ae53890540ce6173ea77ac；Docker外層300s／2GiB／2CPU／128pids／UID1000／network none，原ZIP／patch唯讀。新規格同次加入000-index；限定驗收後回填338與缺證據檢查器。主庫RE-first保持。原版素材／LOG／PNG／RAM留忽略workplace，不公開；輪末核對擁有權與Docker清理。

## 339輸入前置審查與READY

唯讀prototype SHA-256 f30e296ee52f359a1cf04308ffa7c10bd643d57a9e70239d39a0eede912017e8；workplace/moo2-probe-339-input.txt.gz SHA-256 3f1e74f87087527db0dab08a7ae877015f021bf26b098c18e7d067c52a64d93c。98M只有count1／55bytes與初始化畫面；不作輸入初態。99M才有count10與八色旗幟，完整550bytes與100M相同。剝除6新增唯讀列，全部7874原ruler列與30PNG逐值／逐位元保持338。實際開啟99M原PNG確認SELECT BANNER COLOR與八色。

固定99M輸入前置：EIP238599；R依EAX／ECX／EDX／EBX／ESP／EBP／ESI／EDI為35B824／47／55／9／2BD9B8／2BDA14／6ABC38／35B82D；段CS／DS／ES／FS／GS／SS為8／188／188／0／20／188；flags206h；FPU control127F、status0、depth0、stack全0。VBE Bank4／StartY0／BankSets1511／Writes35010112／DisplaySets48，RGB SHA-256 8c2c573c08ebf4bce0b4e166d4268fb6fcf90f35dd27fbbb2de614b26f187f15；PNG SHA-256 96efbd1ce6538c27b019cc6713fe7397d6d0fde7a63c7e01cb82023f75614c67。callback mask2B、pending0、active false、10／10；IRQ active false、failed false、22553／22553。虛擬時間177207342微秒。

table pointer298848／count10／bias0／stride55，550bytes SHA-256 978afb91aaed9e0cb37672a66352e2ae515b382d5b6f472da9238e09fb650dfb；原globals192bytes／header16bytes逐位元保留並核對其雜湊。index1 rect96／144／179／242，raw x276與278在原縮放後均命中index1；y190相同。正常press固定99M；release在callback完成且20ms後首次合法時點，mask1或2B。尚未執行新輸入。以上證據足夠以有界工具探針實作，不允許修改主庫玩法；本節審查通過才轉READY。

## 339觀察窗不足與有界延長修訂

首次來源4e628d389e2910f7dea1c5bdacff3372e863b49524487125c4dfff3586268662另存workplace/moo2-banner-339-early.go；首次workplace/moo2-probe-339-early.txt.gz SHA-256 a88b8631f68504f9a373f07a1a77910736ca4a58e09285cc9981369ee0483cc4；首次PNG fdcfce7eb7a39bfa24641a15309ec035208efa205c394173c5cdde37e5e6ec11。99000000按下、99019204放開，相差20000微秒，callback12／12完成，但100M仍在旗幟頁，共享20DDDB未命中。這只證明正常裝置輸入，不能稱原選色消費或下一頁成功。

目前沒有新CPU拒絕。100M終態仍EIP238591並有原繪製更新；99M至100M只給1M步觀察，不足以判定輸入被接受或被清除。原因維持未知，不推測原持久旗色writer。不更換seed或重送點擊，不挑選碰巧成功的結果。

退回DRAFT，只延長同一次固定99M press-release情境到120M，保留所有輸入與原99M完整前置。120M只可由BANNER_RED_CLICK=1及完整固定依賴啟用，其他情境仍1..100M上限；不接受101M、119M、120M無旗標或任意更高cap。maxSteps只影響診斷loop停止與只讀輸出排程，不能改CPU、DOS、時間或guest state。新增110M／120M只讀GUI與120M完整表，遇新CPU拒絕按實際收據停止。較早六條基準不重跑。

120M來源審查必須把所有新旗標區塊剝除並逆轉明示旗標限定的五個guard後，逐位元等於338 HEAD。正式新red只重生一次；100M以前全部原執行列與PNG保持首次early收據，除了cap排程記錄與新終態；不把改cap後不同停止行當差異。這是有界觀察修訂，尚不稱選色完成或整體CONFORMED。

修訂READY：已直接核對maxSteps解析、唯一for loop、既有GUI／表快照guard。120M擴充只改有界診斷停止與唯讀輸出，不改原輸入、CPU／平台或主庫玩法。固定99M前置沿已審查原收據，沒有新猜補規則。外層仍300s。

## 339正常按鍵輪詢閘門修訂

同99M輸入延長120M的原收據workplace/moo2-probe-339-extended.txt.gz SHA-256 559b388072eb7c8d9976b8de4f9c137b2d17dde9ef748630ce7a887100b1df8b，來源6c63547a372fd329bc58351928d0e8dc2a7806a45461ce9d5a049e659c05fe63另存workplace/moo2-banner-339-extended.go。全部7837原100M前執行列保持首次early；120M仍旗幟頁、callback12／12，沒有新CPU拒絕，不能稱選色消費。120M PNG仍fdcfce7eb7a39bfa24641a15309ec035208efa205c394173c5cdde37e5e6ec11。

直接解析同收據：press到release之間原INT33 AX3正常輪詢為0次，release後有492次，均讀到buttons0；不能從callback完成推定原GUI已見pressed。internal/machine/le_startup.go 的現存INT33 AX3把mouseButtons返回BX低word、x／y返回CX／DX；InjectMouseEvent更新此正常裝置狀態，見internal/machine/le_mouse_callback.go。平台保持335，不修改或重證已知契約。GUI生命週期路由已重查。

退回DRAFT。保留固定99M press／座標與120M cap，只把放開前置增為「原正常callsite24C31B首次INT33 AX3確實讀到BX低word1、CX低word276、DX低word190」，並保留原IF／回呼完成／IRQ／20ms條件。新observer只在明示旗標且pressed未released時讀services.Handle正常輸出與記錄，非callback、handled且精確callsite；不寫CPU、RAM、選色、DOS或持久資料，不改按下時間或重送事件。若未讀到按下，120M明示未放開，絕不以重試／隨機延後代替。

這是測試持按至原版可讀到按鍵的正常滑鼠動作；只據實證明原選色與下一頁。原持久旗色writer、正式RNG、remake同狀態仍未知。不把observer新名稱當原函式語意，不猜玩家資料offset。

輪詢修訂READY：已核對既有INT33 AX3與InjectMouseEvent原始實作，正常收據已證實首次release前沒有AX3輪詢。本修訂只添加首次正常pressed查詢的readonly觀測與放開閘門，固定99M初態及120M停止仍保持。

## 限定驗收：原旗幟表／正常按鍵查詢與放開／選色消費未完成

本規格CONFORMED只涵蓋原10筆表／完整99M初態／正常裝置press與原pressed查詢／release，選色消費與下一頁仍未證實。原持久旗色writer仍未知；共享20DDDB未命中，不能因座標命中index1就聲稱原index1被選定。原120M仍旗幟頁，沒有新CPU拒絕。不計入主庫玩法完成分母，不宣稱正式RNG、整段開局或remake同狀態。

按下99000000、177207342微秒、x276／y190／buttons1／mask2B；首個正常callsite24C31B的INT33 AX3在99083819、177452599微秒返回BX低word1／CX276／DX190，完整R／段／flags已保存。放開99083854、177452633微秒、x278／y190／buttons0／mask1，差245291微秒。原查詢已返回pressed，回呼11／11完成且至少20ms後，首次合法條件才放開。終態callback12／12及IRQ28866／28866完成、非活動且未失敗，不能拿callback完成當選色消費。

正式新來源d131475620cda95f77a6c9846dc7a494a5a60bae778196c6ea49555a79b5d4f3；原版同固定初態新情境120M到228DDC，R=[74 EE CF 89 2BD9B8 2BDA14 2C0B9C 2C040E]、段=[8 188 188 0 20 188]、flags216h。原PNG人工確認仍SELECT BANNER COLOR，只有游標移動；workplace/moo2-vbe-339-red.png SHA-256 fdcfce7eb7a39bfa24641a15309ec035208efa205c394173c5cdde37e5e6ec11，RGB SHA-256 8f140b5eb4f65e2ed42744f269e5e2505a395feefb91b7dc2c829a81e398cb97。原120M完整10筆表、FPU與VBE／RAM只讀收據保存，表未因游標更新推定正式旗色。

python3 workplace/new-game-339-input-verify.py PASS：全部7874原ruler列與30PNG保持338，98M表仍count1、99M才完整count10。python3 workplace/new-game-339-verify.py PASS：正式99M額外輸入前7708原列保持獨立readonly ruler；全部28既有PNG逐位元保持；原完整550bytes表、globals／header、核心與RGB一致，首次pressed查詢的完整輸出、正常press-release順序獨立核算。相同99M短按延長120M的7837原100M前列保持首次early，但仍未成功；兩份負結果與來源另存，不冒充成功。

新版CLI 22無效值／缺依賴在讀EXE前exit2，120M與舊100M有效正對照、同binary舊338的17拒絕及正對照通過；120M僅明示旗標，101M／119M／120000001及120M無旗標拒絕。python3 workplace/new-game-339-source-verify.py PASS：六個明示339區塊與五個有界guard／訊息逆轉後，全部來源逐位元保持338 HEAD。未改CPU／平台，CPU／startup／provider／matcher／mouse callback保持335；六個較早原版基準沿338已有收據，未重跑、不冒稱339全部重生。Go全套沿335，這輪未重跑。

Go1.24.13、原ZIP／patch唯讀、417新鮮根檔／官方EXE／MOX.SET、44M Esc／固定日期1996-01-01及所有338原輸入保持；日期不是seed。Docker外層300s／2GiB／2CPU／128pids／UID1000／network none，go build -p 2 -buildvcs=false -o /tmp/moo2-probe ./workplace/moo2-probe。原版PNG／LOG／RAM與可丟棄probe／驗證腳本留本機忽略workplace，不公開。

首次pressed查詢後的原GUI按鍵返回與正常放開已由規格340接通，見[340](340-moo2-banner-pressed-consumer.md)。339舊放開後第二次查詢返回0，340以原RET到caller且AX1才放開，原版仍未選色。下一步只保存340的20DB5B後有界正常步與實際分支；不盲改cap或重送。持久名稱／旗色writer、完整開局及remake同狀態未知，主庫RE-first保持。

| 本機忽略收據／來源／核算 | SHA-256 |
| --- | --- |
| workplace/moo2-probe-339-input.txt.gz | 3f1e74f87087527db0dab08a7ae877015f021bf26b098c18e7d067c52a64d93c |
| workplace/moo2-banner-339-input.go | f30e296ee52f359a1cf04308ffa7c10bd643d57a9e70239d39a0eede912017e8 |
| workplace/new-game-339-input-verify.py | 5e86ed732e266dacec88154f2c92fcfd37196c76fa4ccbd83874e2a6b4224341 |
| workplace/new-game-339-input-tests.txt | 70c2f1af8e5a1edfd4a5088311d1db6108ed0c5aaa85c68cf31de637d9c14794 |
| workplace/moo2-banner-339-early.go | 4e628d389e2910f7dea1c5bdacff3372e863b49524487125c4dfff3586268662 |
| workplace/moo2-probe-339-early.txt.gz | a88b8631f68504f9a373f07a1a77910736ca4a58e09285cc9981369ee0483cc4 |
| workplace/moo2-vbe-339-early.png | fdcfce7eb7a39bfa24641a15309ec035208efa205c394173c5cdde37e5e6ec11 |
| workplace/new-game-339-early-parity-tests.txt | 9cb29e6a9e5947c177fc20d4beff735caad31af02b8c8e97f28fe04068360237 |
| workplace/moo2-banner-339-extended.go | 6c63547a372fd329bc58351928d0e8dc2a7806a45461ce9d5a049e659c05fe63 |
| workplace/moo2-probe-339-extended.txt.gz | 559b388072eb7c8d9976b8de4f9c137b2d17dde9ef748630ce7a887100b1df8b |
| workplace/moo2-vbe-339-extended.png | fdcfce7eb7a39bfa24641a15309ec035208efa205c394173c5cdde37e5e6ec11 |
| workplace/new-game-339-extended-parity-tests.txt | 2030fffc23bec8d03e485feaab79e3f97847720dfdfd149c2c3a1ed23fc8b478 |
| workplace/moo2-probe-339-red.txt.gz | 9505df160613d748632e9e43a7e47e73945e9ed56948a9e66f951fbd9ed49965 |
| workplace/moo2-vbe-339-red.png | fdcfce7eb7a39bfa24641a15309ec035208efa205c394173c5cdde37e5e6ec11 |
| workplace/new-game-339-verify.py | 9e0f1b9faaaf01f793b8c899146d64d285fe33e3c7327487ec3998dd46fecfcb |
| workplace/new-game-339-parity-tests.txt | f89eb1f61495c194d3889c4536b0bcedd720189ea957b9431dda1c108a2d26c8 |
| workplace/new-game-339-cli-verify.py | d4b4430e1654c5b2e1170394e19a05642ce229d4658e3ce44084c6b599a8b441 |
| workplace/new-game-339-cli-tests.txt | e4461c40e971daf4de8338567b57b68f0945355ed595b7c5c09620a2590be774 |
| workplace/new-game-339-source-verify.py | 80b75e90c3304f4259a52697c2b0d757de7c474881d2fa6461b0a4b56e0e37d4 |
| workplace/new-game-339-source-tests.txt | 5b209298ebaf57ffe253112aa7357875b4443d71c126a9d9f8f1038821fc7f48 |

76項規格回填正對照、新339 26缺證據負例、338 26負例與兩CLI通過；限定正常查詢與放開，選色消費未知。

| 本機忽略核算 | SHA-256 |
| --- | --- |
| workplace/new-game-339-backlink-verify.py | e5e11057102f59e863b5b21491248f275500049ce6933da9b102eecbc32650a5 |
| workplace/new-game-339-backlink-tests.txt | 3499c8deb32ad69c4bfb6acea4332039308806d913a265b3cec0cd34463fc4b1 |

## 368可寫紅旗正常輸入與寫檔回填

[368-moo2-overlay-banner-red](368-moo2-overlay-banner-red.md)已驗可寫99M真實完整550byte表後一次正常press，原24C31B pressed查詢、214104 RET20DB5B／20E165返回AX1後release，原160M生成UI親看。原237024的SAVE10.GAM 3D01返回handle9／CF0，237093 truncate及238310共208000bytes正常寫入並close，隔離state的SAVE10.GAM與MOX.SET副本hash已核對。362唯讀拒絕與339／341原輸入收據仍有效，不外推可寫same-state。367原99M未送旗色的歷史保持，後續由368限定接通。原169E49／20D0的新CPU拒絕尚未修正；旗色正式持久語意、讀檔、180M與完整開局未驗，主庫RE-first保持。
