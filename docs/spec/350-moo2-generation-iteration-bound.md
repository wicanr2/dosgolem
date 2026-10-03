# 350：後續生成的外層迭代上限與重繪邊界

狀態：**CONFORMED，限定診斷驗收**
日期：2026-10-03

沿[349原入口與等待](349-moo2-generation-entry.md)。起點主庫10f8dccc8d33cd311bc45b5d4c21cd24f06a4ffa／工具07611f5808e53eb40a61cc8a94489045b0ff0686。原160M預算、99M press／99084355 release及1996-01-01保持，固定日期不是seed。主庫RE-first保持。

## 來源與原始問題

官方DOS1.31 ORION2.EXE SHA-256 4e11be14217b4aafa1839f333bf5eba037f98b0c44e9e4752c96c464c260419f。CPU b5c8bd900047e8f018ad66edaf5ae8e1009e9f6d04ec89dea4000c27f3326b30，正式probe f2826d243f01ff4b6659aa45d4faae635d218dc779e8357b486f04b5870471be。原349全部10556列／36PNG為核算基準。

已證實：16BADE→16AD13正常入口，三子呼叫及首距離返回；160M外層returned=false。既有固定IDA linear EA原sub_7AD13的7AF1B INC ESI、7AF1C CMP SI,word_19199A、7AF23 JL 7AD7D，及7AE07 CALL sub_7C78E，是本輪定位。原界限實際值、迭代數與重繪尚未動態核對，不以名稱猜原資料用途。

## 有界蒐證契約

1. 只用349固定input SHA／IDA9.4／原名／EA／file offset／bytes／operand匯出，核對既有檔案雜湊與三關鍵指令；不重建或修改正式.i64，不深入runtime／繪圖helper。
2. private沿原160M正常路徑，在原callee框架內取首四個不同SI迭代。每組最多五事件：16AD7D、16AF1C、16AF23、16AE07及其真正返回16AE0C；只有先見重繪CALL才記重繪返回。終態另一次，共最多21事件。
3. 原CMP的記憶體位址由原指令的relocated operand取得，按描述符／RAM邊界唯讀實際bound。保留原code與bound raw word，不能先填猜值或外推其他版本。原SI／flags／JL結果、重繪參數與stack保存；沒有命中即明示未知。
4. 每事件以activationPeek及完整RAM before／after核對readonly，不裝hook、不換Bus、不代寫guest或重送輸入；CPU／平台／主庫玩法不改。
5. 全部10556原349列／36PNG及終態按既有mtime／DTA／每次RAM規則比較。DRAFT private不直接接production，證據足夠後先READY審查，再限定診斷驗收。
6. 完整母星配置／開局、外層RET、正式writer、RNG與remake同狀態未知。先依迭代及原界限估足夠的續行預算，不盲提高cap。

沿既有Go1.24.13 image／UID1000，Docker --rm／network none／2GiB／2CPU／128pids及450s外層timeout；原ZIP／patch／正式DB唯讀，輸出只寫已有忽略workplace／tmp。原EXE／LBX／RAM／LOG／PNG／私有腳本不入Git，公開只存定位／等級／雜湊與回填；同次列入000-index。新來源寫入前與後核對UID1000／root-owned／.md目錄，收尾清理專案容器。

## READY 證據審查

private160M收據核對全部10556原349列／36PNG與終態保持。13事件readonly，四組head SI依序1、2、3、4，各原CMP後SI為2、3、4、5；原JL signed比較成立，前3個JL後下一原步回16AD7D。原relocated CMP bytes66 3B 35 9A 19 28 00，讀DS188:28199A的2400，signed36；既有IDA linear EA:7AF1C原operand word_19199A／bytes66 3B 35 9A 19 19 00分開保留，不混淆基準。四次head至CMP為151324、151553、166358、167587原步。只證實這些樣本前進與該次實際上限，不稱全部迭代或原欄位語意已知。

首四組16AE07 CALL／真正16AE0C返回未見。observer到第5個索引即飽和，full=true，只保存首4組，不宣稱之後皆未走重繪。160M仍17FD04、外層returned=false；terminal ESI屬nested框架，不能解讀為外層SI。完整配置／開局未知，四次耗時不能保證剩餘工作的完成預算。

證據足以批准兩個有界readonly診斷區塊。正式只更名350標記，執行語句與已驗private相同；universe160關閉不讀guest、不輸出defer，最多4組×5事件加終態一次。正式建置、關閉8M／舊CLI、來源逆轉與回填通過後，限定CONFORMED。CPU／平台／輸入／160M cap／主庫玩法不改。下一步核對原外層JL不跳的出口、後段與RET／回返，而非由四次耗時盲提高cap。

## 限定驗收：原迭代前進與實際界限

350限定CONFORMED，只驗首四組原迭代／實際比較界限／signed JL及160M等待。完整生成／開局與remake同狀態仍未驗。13事件、4組，max_events21／full=true；首四組重繪未見不代表後續未重繪。

| 原head SI | head原步／dosgolem_high_le:16AD7D | CMP原步／16AF1C | CMP時SI | JL原步／16AF23 | head至CMP原步 |
| --- | --- | --- | --- | --- | --- |
| 1 | 153880145 | 154031469 | 2 | 154031470 | 151324 |
| 2 | 154031471 | 154183024 | 3 | 154183025 | 151553 |
| 3 | 154183026 | 154349384 | 4 | 154349385 | 166358 |
| 4 | 154349386 | 154516973 | 5 | 154516974 | 167587 |

**已證實，原比較界限**：本次dosgolem_high_le:16AF1C原relocated指令66 3B 35 9A 19 28 00讀DS188:28199A，raw2400／signed36；13事件皆相同。既有固定IDA9.4 linear EA:7AF1C原66 3B 35 9A 19 19 00／operand word_19199A分別記載，檔案SHA-256 0c16013bacada3769016549c3a30cfe12ddc2ea72b3829d20ff77256969b0dec／官方EXE4e11be14…。原operand定位經實際bytes導出，不預設relocation或替原欄位命名。36的正式語意、其他設定及版本未知。

**已證實，原迭代前進**：每組CMP後一原步到JL；下一SI2、3、4、5皆小於signed36，SF xor OF為1。前3組JL後下一原步回16AD7D，原frame EBP2BDB60／ESP2BCF60保持。原IDA7AF1B 46 INC ESI、7AF23 0F 8C 54 FE FF FF JL 7AD7D與實際指令對應。原呼叫仍在產生不同資料，不能因最終畫面相同宣稱卡死。

**已證實，觀察範圍**：各組只記首五個phase，見head／CMP／JL，16AE07及有先見CALL才可記的16AE0C都未命中。第5個不同SI令full=true，後續不蒐組；因此不能把「首四組未見」寫成整段未見。終態另外group=-1 phase5，160000000／17FD04／unique_sites39434／outer_returned=false，無新CPU拒絕。terminal EBP2BCF04／ESI2BDB44是nested框架，不充當外層SI。

**強推論**：前四次耗時在151324..167587之間，顯示有限樣本有進度。這不是剩餘工作的完成時間保證；外層還有原後段控制流與可能重入，未量到全部出口，不能按35倍耗時斷言160M足夠。

**目前未知**：後續配置／重入、後續重繪、正式writer、原欄位語意、完整母星配置／開局、RNG與remake同狀態。原第一callee的全部35迭代、JL不跳出口、後段與RET／16BAE3返回已由352驗證。下一步依352的word記憶體AND拒絕建立窄CPU切片，再用同180M正常情境續行，不深挖繪圖／runtime helper。

## 保持、接線與重生

全部10556原349列／36PNG保持，沿既有mtime／DTA／每次RAM規則；新13事件核心／Bus／FPU／VBE及完整RAM before／after相等。final PNG c7534b8f40b51b8377d255d66e6dd759dfb3d427fa6fccc6ee7d8c3999b32bca／RGB4a1d9efc9d15da575e330128f22d27e97f6d6616ac1b4184ec94efc2bd27f2c9，仍配置母星。probe exit0只代表cap。

READY審查後正式只更名350標記，兩區塊逆轉逐byte保持349；CPU／平台不改。private b7c6d33772360ad748ce19d6dc20758da3ea30dadbf9e248af0f1d701955efa7，正式0cbe29a3e93145f2c7ee71036e0fe76902b6dbc76809571df761a3269d7e70d3。universe160關閉不讀guest或新增defer，8M啟動1693原列／PNG保持，沒有generation_iteration_新增觀察。68舊CLI負例與100M／120M／160M正對照保持；本輪不重跑120M或無關CPU全套。

原160M收據SHA-256 d2c9130ac36766528d7a955a2711e45a863cbee5ca631bea36ad1f4085135231。原Go1.24.13 image sha256:1a6d4452c65dea36aac2e2d606b01b4a029ec90cc1ae53890540ce6173ea77ac，Docker --rm／network none／UID1000／2GiB／2CPU／128pids及450s原版timeout，正式8M／CLI180s。原ZIP／patch唯讀，固定IDA349匯出只讀核對，本輪沒有新IDA執行。原input、calendar、press／release及160M cap保持。

容器入口：bash workplace/new-game-350-run.sh／new-game-350-off-run.sh；python3 workplace/new-game-350-verify.py／new-game-350-source-verify.py PASS，go build -p 2 -buildvcs=false -o /tmp/moo2-probe ./workplace/moo2-probe。原EXE／LBX／RAM／LOG／PNG／私有腳本留忽略workplace，不入Git。

## 不可變鍵與較早回填

| 不可變鍵 | 已證實／未知 | 較早規格 | 必須回填 |
| --- | --- | --- | --- |
| DOS1.31／EXE4e11be14…／dosgolem_high_le:16AD7D／16AF1C／16AF23 | 首四原迭代與signed界限36已驗，全部出口／RET未知 | 349 | 原首四迭代與實際比較界限已由規格350驗證 |

87項規格回填正對照與新350缺證據／限定範圍／狀態／較早回填／索引負例通過，既有負例保持。公開入口：python3 apps/moo2/tools/startup_probe_131.py --check-generation-iteration-spec-backlinks，只驗首四原迭代／signed界限36與回填，不啟動DOSBox-X。主庫RE-first保持，整款remake／中文化尚未完成。

## 本機忽略證據索引

| 收據／核算 | SHA-256 |
| --- | --- |
| moo2-349-ida-ui-ret.json | 0c16013bacada3769016549c3a30cfe12ddc2ea72b3829d20ff77256969b0dec |
| moo2-350-iteration.go | b7c6d33772360ad748ce19d6dc20758da3ea30dadbf9e248af0f1d701955efa7 |
| moo2-probe-350-iteration.txt.gz | d2c9130ac36766528d7a955a2711e45a863cbee5ca631bea36ad1f4085135231 |
| moo2-vbe-350-iteration.png | c7534b8f40b51b8377d255d66e6dd759dfb3d427fa6fccc6ee7d8c3999b32bca |
| new-game-350-run.sh | 62cf6bd9e6e606f087dfd06ec38c64cd898aafc21a7296b20ceabb30b12cb82b |
| new-game-350-verify.py | 9a006bb260d23ce0f4e7415c24cc55d586181c87bbe5d480b728dcf08b38b265 |
| new-game-350-tests.txt | f24b7714b79603c1868f293204fe82d6a9ca42cd7b01ac7878f8407fe172f45a |
| new-game-350-source-verify.py | 1fc55322c19b2ba4bb64e84a15fc27a8786c9c9f0d2dc0fe5d64d4ff6263152c |
| new-game-350-source-tests.txt | 75ad232d938073707ae1fd1c698391bfb883538453b39813e24010cfdba0f649 |
| new-game-350-off-run.sh | db2087e5f4655be47413d6fac125b4298f9bfc03740dc0736493afed0283bb92 |
| new-game-350-off-cli-tests.txt | 7d720ece028de31c0f318e14ac3aa0eaf3a62300093ab5410538b8b4007df7f1 |
| moo2-probe-350-off-old.txt | f710a4306e74e7424e99444b9a469d4f0ccd80b51cd746dbbae4794e88cca567 |
| moo2-probe-350-off-new.txt | 288af6c93c3cd4ed571eda12995791e16cd313570ec7f7150983aec0af5b7b7e |
| moo2-vbe-350-off-old.png | dd4c21dd11e57ec86ef759182fd3ebadd6b53870b54de1ef192e2553820286db |
| moo2-vbe-350-off-new.png | dd4c21dd11e57ec86ef759182fd3ebadd6b53870b54de1ef192e2553820286db |
| new-game-350-backlink-verify.py | 7a35c9000023e5f5a9d094b29682518a9d695f4d1c6a4d763e4f36b9f820333e |
| new-game-350-backlink-tests.txt | 38f6a1121a9a05b982ddf8055ef80a0ba810a5a76da19ea0f999e75b3b0c822a |
| new-game-350-frames.json | 53151cd1754678ba3c5a702351d0c629782aa222d9cfb01b8dc3131ff6de6bc3 |

## 351後續回填

原連續SI1到26與160M截斷已由規格351驗證，見[351進度與截斷](351-moo2-generation-completion-boundary.md)。26個head無回跳／重複，153880145到159926951共6046806原步；160M只再續73049步，full=false，原出口／後段／RET未見。先前維持160M蒐證已完成，下一次依實測工作量設定有限180M，不從前四次耗時推定完成。

## 352後續回填

原生成35迭代與真正RET及caller返回已由規格352驗證，見[352原出口／返回與新停止](352-moo2-generation-180m-continuation.md)。163755071原SI36的JL不跳，163778787的16B01F C3後次步16BAE3，原AL0／caller ESP與EBP恢復；163779084的16BAEC ZF1，次步16BB00。原第一callee的全部35迭代、出口／後段／RET已閉合，本文160M未見是當時範圍。完整配置／開局與後續writer仍未知；新103BF9的66 81 /4 word記憶體AND拒絕未解。下一步接該CPU缺口，再用同180M正常情境核對，不重開已驗原返回。
