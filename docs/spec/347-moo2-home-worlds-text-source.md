# 347：配置母星文字的資料來源與原正常消費

狀態：**CONFORMED，限定診斷驗收**
日期：2026-10-03
範圍：沿[346固定160M續行](346-moo2-universe-160m-normal-continuation.md)，維持原160M預算，定位150M至160M畫面文字變化的資料來源與最小原呼叫端。只蒐證，不猜生成規則、不改主庫玩法。

## 固定輸入与目前證據

- 主庫6e32fabf24833d6fa2811160d0bfcaed46420e56；工具92d25f387d2ff18919ec65da2d85c7ce1566d7db。
- 官方DOS1.31 ORION2.EXE SHA-256 4e11be14217b4aafa1839f333bf5eba037f98b0c44e9e4752c96c464c260419f；CPU b5c8bd900047e8f018ad66edaf5ae8e1009e9f6d04ec89dea4000c27f3326b30；probe b389f6c6b661e534b92e0be060c31921c2251735594187c7ed842a2f1f12f7e2。
- 已證實：346原150M仍Generating Universe...；160M為Placing home worlds...，第三例120083995正常RET。160M終態17FD04／unique_sites39434，完整生成／開局未驗。所有原位址dosgolem_high_le，不與IDA或檔案offset混列。
- 已證實，檔案搜尋：兩字串均出現於HESTRNGS.LBX與MSGENG.LBX，原ZIP與官方1.31 patch對應檔SHA一致。精確archive offset、blob與record形狀待獨立核對；檔案命中不證明實際使用哪份。
- 固定日期1996-01-01不是RNG seed；原正常輸入、99M press／99084355 release不變。

## 最小蒐證與停止線

1. 核對兩LBX實際archive count／offset／邊界與兩字串raw位置，不以副檔名猜格式。
2. 優先使用既有IDA9.4 locked-v1，以原始DOS1.31唯讀輸入建立一次性DB。每個image／UID先驗最小非空JSON與輸入雜湊；保留原名、原位址、operand與bytes，語意附推論等級，不重新命名。
3. 先辨認與排除loader／runtime，僅追實際兩段文字索引取用／原呼叫端與最小生成狀態。必要時由dosgolem正常160M流程取得readonly RAM／完整核心／FPU／VBE窗口。private可丟棄觀察不當正式功能。
4. 原160M完整收據／36PNG及所有既有原輸入保持；新觀察有界且不代寫狀態、不換Bus／hooks，不增加cap。若尚未定位，保存未知與最小下一步，不藉綠測試稱原版exact。
5. CPU／平台與主庫Go／Ebitengine玩法不改；本輪證據足夠後才審查READY，production path前依RE→spec→READY→同狀態驗證，限定CONFORMED。
6. 原LOG／RAM／PNG／binary／一次性.i64與私有腳本留本機忽略workplace，公開只保存定位、證據等級、雜湊與回填。

## 工具與原版輸入

Go1.24.13 image sha256:1a6d4452c65dea36aac2e2d606b01b4a029ec90cc1ae53890540ce6173ea77ac。IDA9.4 image sha256:6f6d59af49d0008c4109a5295b5f374bdc007e2d1ab28cb9de08779584de2780。Docker network none／UID1000／有界資源與timeout，原ZIP／patch／正式.i64唯讀；輸出只寫現有忽略workplace／容器tmp。新來源寫入前與收尾驗UID/GID、root-owned、.md目錄與容器清理。

原ZIP3a28a52f5953ff6d8fc251548940500236752ee19b52b51581e71ec1a3373c2f、patch908d6b7b37ad580039c5d108bab2c64b28f51ba735485287d284d5f5242b98e5，417根檔／MOX.SET553bytes bfd6855a41760b31156b96114b5b33c88f442ab8f8aae020c1740b3b486a3a80。原版完整開局、存檔、正式writer與remake同狀態仍未知，主庫RE-first保持。

## READY 證據審查

固定1.31 IDA與正常160M private v2六事件已驗；全部10523原346列／36PNG保持，兩原查詢返回與終止NUL複製已證實。原copy尾端POP EDI前且ESI／EDI皆已越過NUL，flags246h／AL0，不以既有零值當完成。原home框架五PUSH再ENTER，EBP+24保存返回位址16B98A，與IDA caller7B985逐項定位。

批准把兩有界readonly區塊接到正式診斷probe；只在universe160分支啟用，關閉時不讀guest且不新增totals。正式來源與已驗v2只可多一個defer的universe160 guard；旗標開啟時語句相同，原CPU／輸入／160M cap／Bus不改。驗證來源逆轉、private v2完整原流程、正式建置／舊CLI與旗標關閉8M啟動基準，不重跑無關Go全套。正式回填／語法／擁有權與容器清理後限定CONFORMED。

## 限定驗收：原文字索引、兩次查詢與NUL複製

347限定CONFORMED，只閉合兩段進度文字的原資料、正常查詢返回、終止NUL複製與實際呼叫端。生成完成／完整開局及remake同狀態未驗；CPU／平台不改，主庫RE-first保持。

### 原始資料與位址基準

已證實，兩LBX皆archive count1、entry起點2048、entry終點等於實際檔長，entry內4-byte header的count×size恰好覆蓋全部payload：

| 檔案與SHA-256 | entry終點 | raw count／size | 文字 | 檔案offset | payload offset／位置 |
| --- | --- | --- | --- | --- | --- |
| HESTRNGS.LBX／a3193f56d12ae512ab8a78cc50aba9df44d6d614a4cc23b14ec7b9f1b09cbf30 | 15573 | 1／13521 | Placing home worlds ... | 9646 | 7594／NUL ordinal161 |
| 同上 | 15573 | 同上 | Generating Universe ... | 11729 | 9677／NUL ordinal242 |
| MSGENG.LBX／ee70bd446054139101b5187861af24590bc2b46a30736a52b9a09eb27b6d3096 | 410244 | 384／1063 | Placing home worlds ... | 173258 | record161＋63 |
| 同上 | 410244 | 同上 | Generating Universe ... | 259361 | record242＋63 |

原ZIP與官方1.31 patch的兩檔逐byte一致。這些是檔案offset，不是IDA或CPU位址。索引以實際header／邊界／NUL核對，不以版本參數猜archive形狀。

已證實，固定1.31 IDA原sub_7A990有12原指令，以word_1912FC索引取得offset，再加byte_18B41C。IDA原名保留，不改名；負值／397以上的靜態分支存在，但本輪只實測161與242，不把未實測邊界升為動態已證實。

| 功能 | IDA linear EA | dosgolem_high_le | 原證據 |
| --- | --- | --- | --- |
| 原查詢函式 | 7A990..7A9B0 | 16A990..16A9B0 | 固定EXE與12指令／正常兩次CALL |
| 原offset表／blob基底 | 1912FC／18B41C | 2812FC／27B41C | 原operand及實際兩個返回指標 |
| 原目的緩衝區 | 1942F4 | 2842F4 | 原兩caller的EDI與實際copy |
| 生成文字caller | sub_8DAE8／8DCA5 | 17DAE8／17DCA5 | E8 E6 CC FE FF |
| 配置母星文字caller | sub_7C78E／7C8A3 | 16C78E／16C8A3 | E8 E8 E0 FF FF |

各列分開標明工具與基準，以固定入口eb76、原RET c21400、原PUSH56及實際CALL／資料指標逐項核對。本輪映射差F0000只套用已核對的這些code／data定位，不宣稱所有工具／版本都可同樣換算。

### 正常原流程

已證實，private v2沿原160M／99M press與99084355 release，六事件皆核心／Bus／FPU／VBE及完整RAM readonly：

| 文字索引 | 原lookup CALL步 | 原返回步／EAX | 原NUL複製退出步／EIP |
| --- | --- | --- | --- |
| 242／生成宇宙 | 102875194／17DCA5 | 102875207／27D9E9 | 102875330／17DCBA |
| 161／配置母星 | 152598605／16C8A3 | 152598618／27D1C6 | 152598741／16C8B6 |

兩CALL以signed rel32計算均真正進16A990，原CALL返回各13步；返回指標恰為27B41C＋9677或＋7594。48-byte原來源窗口連同後一段文字逐byte等於HESTRNGS的對應窗口，原目的2842F4實際被原CPU寫入。lookup_call時source_offset=0只是尚未取得來源指標的佔位，不把零窗口當成查詢結果。

NUL複製完成只在原POP EDI之前判定，ESI=source＋24、EDI=28430C、AL0、flags246h，原CMP ZF1／JNZ不跳，且來源與目的皆含完整23字元及NUL。原資料未代寫，沒有跳過callee、重送或提高cap；兩copy窗口只各256步、各最多三事件。

已證實，配置母星的原prolog為53 51 52 56 57 C8 10 00 00，五PUSH再ENTER。原SS188:EBP2BDB5C＋24的8AB91600指16B98A，與IDA caller7B985的E8 04 0E 00 00→sub_7C78E對上；本次正常呼叫來自16B985，不把另一靜態caller16AE07當實際命中。這只定位返回位址，尚未捕捉16C78E最終RET。

**強推論，實際英文來源檔**：IDA sub_7A816的HESTRNGS.LBX原字串引用、34D1h長度及本輪正常來源的相鄰文字布局共同支持英文blob來自HESTRNGS。本輪沒有另攔截檔案read呼叫，因此只把已比對的來源bytes稱為已證實，不把完整載入服務列為動態對齊。

### 保持與正式診斷接線

全部10523原346列／36PNG與160M終態保持，依既有mtime／DTA／每次RAM雜湊規則正規化，沒有把原terminal改名作新收據。正式只比已驗private v2多defer的universe160關閉守衛；旗標1的觀察語句相同，兩347區塊逆轉後逐byte保持346。旗標關閉8M正式啟動1693原列／PNG保持，沒有新增觀察紀錄。8M只驗啟動與關閉守衛，不取代正常160M原流程；本輪未重跑120M或無關Go全套，既有346的120M同狀態是歷史已驗。

正式建置及舊338的17／339的22／346的29負例、100M／120M／160M正對照通過。CPU b5c8bd900047e8f018ad66edaf5ae8e1009e9f6d04ec89dea4000c27f3326b30保持；private v2 SHA-256 644805a449403684a68da0b2a0ee85c987d2e8a7e5e0bd4099189c0bce3059ce；正式source ca5643c8d598cb705914cbbcfdc79ef571c6d603960b9395f552c7defe52c546。160M收據3e90425b497fcb452e72a5066a88fcc402042e7fe163342fe6c697a08954d87e，終態17FD04／unique_sites39434／無新CPU拒絕，原圖保持Placing home worlds...；probe exit0只是固定上限。

### 環境與初次觀察勘誤

- 初始可丟棄LE匯出器把RAM上限猜為400000，超過初始2874576-byte容量，未執行CPU；記錄保留，不用該失敗建立原版結論。
- 主庫既有IDA DB的輸入SHA為7ae2ac2e5904ca330009af2827279d889906b0b9b7a8854c38eb707a56e955b5，與固定1.31不符，拒作本輪oracle。重新用官方1.31建立一次性DB；非空JSON／schema／5365函式／input SHA／UID1000驗通過。idat exit1與stdout空不作判準。
- 初次IDAPython把get_fileregion_offset放在idc而出錯；保留attempt1腳本與log，改用image內ida_loader API後乾淨重跑。未改原DB或推測性命名。
- private v1僅看目的文字已含NUL，生成文字的既有零值使判定早八步；v1沒有讀完NUL卻標copy_complete。獨立ESI／EDI核算抓出並拒絕；原log／腳本保留。v2必須走到原POP EDI且兩指標均跨NUL才完成，同160M預算乾淨重跑通過，不當CPU缺陷。

### 可重生入口與回填

容器內：go build -p 2 -buildvcs=false -o /tmp/moo2-probe ./workplace/moo2-probe；private三檔建置沿IRQ1／IRQ7既有入口。兩次160M與正式8M／CLI的完整命令保存為本機workplace/new-game-347-run.sh／new-game-347-off-run.sh。python3 workplace/new-game-347-verify.py／new-game-347-source-verify.py／new-game-347-off-verify.py通過。IDA每次由固定EXE建立一次性DB，log／JSON／私有腳本不入Git；正版EXE／LBX／RAM／PNG不散布。

| 不可變鍵 | 本輪已證實 | 較早規格 | 必須回填 |
| --- | --- | --- | --- |
| DOS1.31／EXE4e11be14…／dosgolem_high_le:16C8A3→16A990／27D1C6→2842F4／16B985 | 索引161正常返回、NUL複製與實際caller | 345／346 | 原配置母星文字來源與實際caller已由規格347驗證 |

原進度函式正常RET與上層分支已由規格348驗證，見[348](348-moo2-home-worlds-return.md)。本347的「尚未捕捉16C78E最終RET」是當時收據邊界，後續153214295已正常RET。下一步維持160M，觀察16BADE→16AD13的正常參數與生成呼叫邊界，欄位用途未知，不猜補玩法。文字renderer、母星配置規則、完整生成／開局、正式writer、RNG、人耳與remake同狀態仍未知；主庫RE-first保持。

84項規格回填正對照、新347的28缺證據／狀態／較早回填／索引負例與345另兩負例、全部舊負例通過。Python語法／diff與擁有權通過；輸出1000:1000，工具樹無root-owned／.md目錄。所有容器有界並於收尾移除。

### 本機忽略證據索引

| 收據／核算 | SHA-256 |
| --- | --- |
| workplace/new-game-347-text-search.py | 7a83ff0e1ae7d04567e08920eb6d1835794e873d9ea44d0886b8dc135bbf5e25 |
| workplace/new-game-347-text-search.json | 5286b807104ebc75644110c27bdb62dbb265c77b3420cd6a90c245fcf43d6657 |
| workplace/new-game-347-lbx-shape.py | c4bb727ac68e940ab6a4a6fc5e633e0634a346f6157e26f4ecf212821ee718b6 |
| workplace/new-game-347-lbx-shape.json | ebc5a68fc1d68da73d5179311037d5f3265a3996b1655a08082757ceda52cbbb |
| workplace/moo2-347-ida-existing-db.json | 3521023087e238592469a6dfb2d876e3652f6f9154e02efa9b6bb7aab68ae997 |
| workplace/moo2-347-ida-min.py | b2b9ec6cdbcbc8178d107073b41a0c822cedc76e471894ab85b3bbb0810468a1 |
| workplace/moo2-347-ida-min.json | 48e6ca489165f4ebac6731e4a9dd9b4260759e78e4e1883edd48e8eeda2125cc |
| workplace/moo2-347-ida-literals.py | b548cd1bda9d744f8d64080994a845cc34cdf7f0c1429e4b55435e149b660047 |
| workplace/moo2-347-ida-literals.json | 78b4e716b1426e89d9105574397cd1e6a4f555772a1ce55bea3bcb1d9d963ea9 |
| workplace/moo2-347-ida-literals-attempt1.py | 57de283eeea29f6bb292a6b0e41d8234f5455bf23a682ebd762d1e7e557964fd |
| workplace/moo2-347-ida-literals-attempt1.log | 5227c159fd7b0eea37983aafe45f972616ce9c70669923b0a993f9b7d93b511c |
| workplace/moo2-347-ida-text-consumer.py | 4648e3fc6b470a8d3c184f3f9e6098eb7a5e6d239dd01b7b85d0285d6ffe98d2 |
| workplace/moo2-347-ida-text-consumer.json | 5970160c8aa070bcb282ad8934a3b8213ff31baff74d94749d77df8e54a64b18 |
| workplace/moo2-347-ida-init-caller.py | 7fd53c6ba4f0252f68805b692242c50ca0c269781cefc441eb92cd2aadce95c2 |
| workplace/moo2-347-ida-init-caller.json | 84465efbc4891edadf700252a35891e53054372d30eb2487fb004e7de28db92e |
| workplace/moo2-347-load.go | 5e2a78987c12072862a70f70353c44beab31edae64a92f883863c63c4a8652c6 |
| workplace/moo2-347-text-attempt1.go | cced941c35c9011bd6f45190459d77433eba16e2be7cf0a973546befdf48d943 |
| workplace/moo2-probe-347-attempt1.txt.gz | ce652c3e6f0fd8040fd53c246c4c17f95460ffdf78942cbf2fc6a5050521a3c5 |
| workplace/new-game-347-attempt1-tests.txt | 1447c07cc2cdd16c13c8281a8173fec14b4108cd7f2f1d8a681b8e5ead06be99 |
| workplace/moo2-347-text.go | 644805a449403684a68da0b2a0ee85c987d2e8a7e5e0bd4099189c0bce3059ce |
| workplace/moo2-probe-347-text.txt.gz | 3e90425b497fcb452e72a5066a88fcc402042e7fe163342fe6c697a08954d87e |
| workplace/moo2-vbe-347-text.png | c7534b8f40b51b8377d255d66e6dd759dfb3d427fa6fccc6ee7d8c3999b32bca |
| workplace/new-game-347-verify.py | 5a466ebc9fa1311a12134811a994e5d3b699a6642d4048d07f2c97bd4a28c899 |
| workplace/new-game-347-tests.txt | 1799ba5f1447ed213c5b2003c75ee7bf6fe708e9c0447b6e1fff72926bb64f4c |
| workplace/new-game-347-source-verify.py | 260eea59425e2899a9b84e95139f8b3ba4174136a6eba044caa5192764a5181b |
| workplace/new-game-347-source-tests.txt | ba08223bfa0707adb11b84c60ef6df7555cb85efd218177e7c49c653f79e817a |
| workplace/new-game-347-off-verify.py | b9dff364d976d09a4f0ecf846d4b2ed09449caafbc16d2d5c7c716d65a9c9824 |
| workplace/new-game-347-off-cli-tests.txt | 12f8c09ab7c76599b2fc8ab0fdedadb9bec8c10f2b9ea2b1ed80bb5c67248505 |
| workplace/moo2-probe-347-off-old.txt | dccb3bbc64a3c1fd1b7d003274f4951676ac7c6ed0517903aedf574049930e39 |
| workplace/moo2-probe-347-off-new.txt | f651d1c0d023eed50f75fa62b191bbf7a93db4d2b6ae4ea8f8511b11becae460 |
| workplace/moo2-vbe-347-off-old.png | dd4c21dd11e57ec86ef759182fd3ebadd6b53870b54de1ef192e2553820286db |
| workplace/moo2-vbe-347-off-new.png | dd4c21dd11e57ec86ef759182fd3ebadd6b53870b54de1ef192e2553820286db |
| workplace/new-game-347-run.sh | 4755cd82aaf0ca4c5d51c7af229fd267c0fac2bcbcb966ab645671a33fe0742f |
| workplace/new-game-347-off-run.sh | c951a79e48e1645ed63ec27c9a6bf033f970df9b25dcddd2646c1229f08ecfdb |
| workplace/new-game-347-backlink-verify.py | c6f952b1823132ab87b5c353d3a863f73920edbb7166bee70ae5faf7f3e84eb8 |
| workplace/new-game-347-backlink-tests.txt | 517b4d757e0589df38f71a3eda0e74d9b34ff02b199cfd514afa3f200fea5e31 |
| workplace/new-game-347-frames.json | c5a76825087036f3581e0ebf03dd99147e834a659c323ae02502756f64d64ea3 |
