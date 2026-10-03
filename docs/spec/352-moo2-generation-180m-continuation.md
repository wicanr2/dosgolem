# 352：固定180M正常續行與160M同狀態核對

狀態：**CONFORMED，限定診斷驗收**
日期：2026-10-03

沿[351原連續進度與截斷](351-moo2-generation-completion-boundary.md)。起點主庫6379a13d3121fbaaca0eb82137c08a4bc3849ecb／工具ffc7e16a6ed34a279418982dbe5131e328ec7162。主庫RE-first保持，本輪只擴充原版診斷預算，不修改玩法、CPU、平台、正常輸入或亂數。

## 原來源與續行依據

官方DOS1.31 ORION2.EXE SHA-256 4e11be14217b4aafa1839f333bf5eba037f98b0c44e9e4752c96c464c260419f；CPU b5c8bd900047e8f018ad66edaf5ae8e1009e9f6d04ec89dea4000c27f3326b30；正式probe6cc3c65f7648e9582713a69fd51fa13a2d2a824cdec79204a7c88e90b22a183c。固定IDA349 JSON0c16013bacada3769016549c3a30cfe12ddc2ea72b3829d20ff77256969b0dec只讀重用，原名／IDA linear EA／file offset／bytes／operand保持，不重建正式DB。

351已證實原SI1..26連續，head153880145到159926951共6046806原步，25個間隔151326..338347；160M只續第26次73049步。原signed界限36，出口／後段／RET未見。下一次加20M至固定180M，是依工作量選定的有限探索餘量，不保證後段／重入或完整開局完成。

## 有界原版實驗契約

1. 新旗標DOSGOLEM_MOO2_UNIVERSE_CONTINUE_180M只接受值1，要求原160M旗標、MAX_STEPS=180000000與既有完整banner正常路徑。未明示新旗標時160M及舊CLI契約不改；缺依賴／非精確值在讀EXE前exit2，不能自動套用預算。
2. 新旗標僅擴充被拒絕的180M情況，舊MAX_STEPS範圍、44M Esc、calendar、417檔／MOX.SET、99M按下／99084355放開、設定／種族／名稱／旗色輸入全部保持。固定日期不是seed，不改guest、hook、Bus或跳過原CALL。
3. 新180M收據及舊觀察器totals寫實際maxSteps，max_steps與budget的宣告欄位明示不同；舊160M輸出仍相同。按10M既有節點延伸畫面到170M／180M，不另推測或代寫渲染。
4. 160M新增一次readonly checkpoint，保留原R／seg／EIP／flags／FPU、code16byte、stack96byte、frame128byte、caller原input64byte／return slot、VBE／RGB及裝置callback／IRQ。descriptor／RAM邊界檢查，activationPeek與完整RAM before／after相等；不改既有觀察器計數或guest狀態。
5. 舊351有10598列／36PNG。核對所有到160M的共通正常事件，僅正規化已列出的預算宣告、mtime／DTA／每次RAM／path；160M core／原code／stack／frame／input／return slot／VBE／像素與舊cap終態逐欄比較。180M後段及新的cap footer不能冒稱與160M相同，也不能重包舊圖片當新收據。
6. 180M以原351最多72head／11邊界追原SI36比較、16AF29出口、16AF35..16AF6E後段、16B01A epilog、16B01F RET及16BAE3 caller；沒有命中即明示未知。若原返回AL與caller分支已見，保留原值／flags／下一原位址，不從RET alone宣稱新遊戲成功。
7. DRAFT private先實測／獨立核算，再READY審查，正式只更名352標記／原替換列表的已驗語句。正式關閉8M與舊CLI、新180M參數拒絕、來源逆轉與回填通過後限定CONFORMED；不能把工具可續行、原單一callee返回或綠測試當整款remake完成。

Go1.24.13既有image，Docker --rm／network none／UID1000／2GiB／2CPU／128pids及600s外層原版逾時，原ZIP／patch唯讀，輸出只用已有忽略workplace／tmp；本輪較長預算相應有界。原EXE／LBX／RAM／LOG／PNG／私有腳本不入Git；本規格同次列入000-index。收尾核對輸出UID1000／root-owned與.md目錄、兩庫精確HEAD／遠端及專案Docker清理。原正式writer、完整配置／開局、存讀檔、RNG及remake同狀態保留未知。

## READY 證據審查

private180M明示預算已量到原35次head、signed SI36的CMP／JL不跳、出口／後段／epilog與真正RET／caller返回。163778787原16B01F C3，163778788到16BAE3，ESP2BDB74→2BDB78、EBP2BDBA0，AL0；163779084原16BAEC ZF1，次步16BB00。不由AL0或parent分支猜完整開局成功。

160M前全部10532共通正常事件保持，只將兩預算宣告的max_steps正規化，沿原mtime／DTA／每次RAM／path契約；35舊frames與原final共36PNG保持。160M原R／seg／EIP／flags／完整FPU／code／stack／frame／input／slot／VBE／像素及callback／IRQ逐欄保持，checkpoint readonly。新46生成事件readonly，head35／full=false／outer_returned=true。

原版在163795435以dosgolem_high_le:103BF9原66 81 63 0C 7F FE拒絕，尚未達180M cap；這是word記憶體AND缺口，requested budget不冒稱實際已執行180M。probe exit0不等於完成。CPU／平台仍保持，本輪只證實延長診斷預算與原生成單一callee返回，不稱母星配置完成。

證據足以批准352預算驗證、實際budget宣告與單次160M readonly checkpoint。正式只能更名352標記，執行語句與已驗private相同；替換列表逆轉逐byte保持351。關閉8M／舊CLI與新180M依賴負例、source／回填通過後限定CONFORMED。原word記憶體AND後續已由[353](353-cpu386-and-word-memory-imm16.md)接通；此段保留352當時的審查邊界。

核算初版在IRQ欄位迴圈把共通事件列表變數a覆蓋，末尾len(a)驗證誤拒絕。已只更名核算變數、保留初版並重現exit1；相同原版收據重新核算全通過，private／CPU未改，不列產品缺陷。

## 限定驗收：原35迭代出口、真正返回與新CPU停止

352限定CONFORMED，只閉合明示180M預算、160M同狀態與原第一個生成callee的正常出口／返回。完整生成／開局與remake同狀態仍未驗。requested budget180M與實際CPU停止步數分開；本次並未達180M cap，不能稱180M全段已跑完。

| 原事件 | 原步 | dosgolem_high_le EIP | 原結果 |
| --- | --- | --- | --- |
| 原head SI27 | 160267006 | 16AD7D | 接續351原SI1..26 |
| 原head SI35 | 163345051 | 16AD7D | 全部head SI1..35連續 |
| 原SI36 CMP | 163755070 | 16AF1C | raw2400／signed36 |
| 原JL不跳／出口 | 163755071／163755072 | 16AF23／16AF29 | flags246h，ZF1，SF xor OF0 |
| 原後段起點／完成 | 163755075／163778142 | 16AF35／16AF68 | 出口DX48h=72 |
| 原選取分支 | 163778144 | 16AF6E | EAX22h=34，原JLE不跳 |
| 原AL0／epilog | 163778780／163778782 | 16B014／16B01A | XOR AL,AL後原AL0 |
| 真正原RET／caller返回 | 163778787／163778788 | 16B01F／16BAE3 | C3，ESP2BDB74→2BDB78，EBP2BDBA0 |
| 原caller零分支 | 163779084／163779085 | 16BAEC／16BB00 | ZF1，JZ命中 |
| 新CPU拒絕 | 163795435 | input103BF9／after103BFC | 66 81 63 0C 7F FE，word記憶體AND未支援 |

**已證實，原出口與返回**：35個head SI1..35連續，SI36比較等於原signed界限36，CMP後JL不跳，下一原步到16AF29。後段以DX72結束，EAX34的選取分支未跳；原16B014 30C0、EB02跳過16B018 B001，到16B01A LEAVE。原16B01F C3執行後下一步16BAE3，AL0／ESP2BDB78／EBP2BDBA0，是真正返回，不能再把第一callee列未返回。原caller較晚16BAEC ZF1、下一步16BB00已見。原參數、候選正式名稱、後續配置與持久writer未知，不由AL0或caller分支命名完整開局成功。

**已證實，160M保持**：原35110598列中，160M前全部10532共通正常事件保持；僅兩預算宣告的max_steps180M→160M及既有mtime／DTA／每次RAM／path正規化。舊35frames與160M原final共36PNG逐byte保持，新的160M checkpoint與舊cap的R／seg／EIP／flags／完整FPU／原code／stack／frame／input／return slot／VBE／像素／callback與IRQ逐欄相同。RAM只驗各次before／after一致，不宣稱跨次完整RAM hash逐bit相等。160M原畫面PNG c7534b8f40b51b8377d255d66e6dd759dfb3d427fa6fccc6ee7d8c3999b32bca／RGB4a1d9efc9d15da575e330128f22d27e97f6d6616ac1b4184ec94efc2bd27f2c9保持。

**已證實，觀察範圍**：46個生成事件readonly，heads35／full=false／outer_returned=true，原boundary phase1..7、9..11皆命中，phase8的AL1入口未見；終態phase12另記。原349的seen0..16皆true，第三原返回及其caller零分支已取得收據。不是從保存return slot或相同最終畫面推定完成。

**已證實，新CPU停止**：原input dosgolem_high_le:103BF9 bytes66 81 63 0C 7F FE，原ModRM63h為/4、[EBX+0Ch]與word immediate FE7Fh；原EBX5AA044，DS188 offset5AA050。cpu386在解碼途中以「81 word形狀尚未支援」拒絕，after EIP103BFC，沒有執行AND寫回；不能把after位址當下一指令已到達。actual stop163795435，診斷defer step163795436、requested budget180000000。probe exit0只是main的錯誤收尾，不代表cap或成功。停止後PNG d493c2b5628d55381176c9e676586ab8940fd62544302195b59570b6136e6ba6是本次dosgolem自行生成，不代替正常開局驗收。

原103BF9 word記憶體AND與三步消費已由規格353接通，見[353](353-cpu386-and-word-memory-imm16.md)。原word0000、完整iw／SHL EDX0／OR dword0、flags246h及精確EIP已驗；未改CPU10793原列／正式入口前10723原列與36PNG保持，CPU全套通過。原223E93 byte記憶體XCHG與STOSB兩步已由規格354接通，見[354](354-cpu386-xchg-byte-memory-register.md)。原164560803的1CDD0F memory byte ADD已由355接通；完整母星配置／開局、正式writer、RNG、人耳與remake同狀態仍未知。下一步依359回填帳核對原1D2A33的CWD與下一SUB／shift，沿同180M，不增加cap或深入helper。

## 保持、接線與重生

READY審查後正式只更名352標記，原替換列表全部逆轉逐byte保持351；CPU／平台不改。private7e03730549d3b9d74f160becde2780e31251b9b1d1437ad3c5181240b191152b，正式c49edc0afcb44dfec22139043afb887a72a70b83d172a165d6869ac9a54be4a1。原續行收據SHA-256 db7630f02daaac46f0a2d45da1813a319d27e14e48c77c7b07a75a97b338c60f；原CPU b5c8bd900047e8f018ad66edaf5ae8e1009e9f6d04ec89dea4000c27f3326b30保持。

新旗標DOSGOLEM_MOO2_UNIVERSE_CONTINUE_180M=1要求原160M旗標、MAX_STEPS180M與完整banner正常情境。原旗標關閉時8M啟動1693原列／PNG與68舊CLI負例、100M／120M／160M正對照保持；新180M32負例在讀EXE前exit2，完整180M與原160M正對照通過。沒有generation_continuation_新增觀察。各舊totals記實際budget，舊160M宣告保持，不放寬未明示續行。本輪不重跑120M或無關CPU全套，尚未修改CPU修補原81拒絕。

Go1.24.13 image sha256:1a6d4452c65dea36aac2e2d606b01b4a029ec90cc1ae53890540ce6173ea77ac，Docker --rm／network none／UID1000／2GiB／2CPU／128pids及600s原版timeout、正式8M／CLI180s。原ZIP／patch只讀，417檔／MOX.SET、44M Esc、99M按下／99084355放開與1996-01-01保持；日期不是seed。本輪IDA只重用固定349匯出，原名／IDA linear EA／file offset／bytes／operand與input SHA不改，沒有新IDA執行。

容器入口：bash workplace/new-game-352-run.sh／new-game-352-off-run.sh；python3 workplace/new-game-352-verify.py／new-game-352-source-verify.py PASS；go build -p 2 -buildvcs=false -o /tmp/moo2-probe ./workplace/moo2-probe。核算初版IRQ迴圈覆蓋共通事件列表變數，僅更名核算變數後通過；初版與stderr保留並重現exit1，原private／CPU未改，不列遊戲缺陷。原EXE／LBX／RAM／LOG／PNG／私有腳本不入Git。

## 不可變鍵與較早回填

| 不可變鍵 | 已證實／未知 | 較早規格 | 必須回填 |
| --- | --- | --- | --- |
| DOS1.31／EXE4e11be14…／dosgolem_high_le:16AD13..16B01F／16BAE3 | 原35迭代／signed36出口、AL0與真正RET／caller返回已證實；新81 word停止與完整開局未知 | 349／350／351 | 原生成35迭代與真正RET及caller返回已由規格352驗證 |

89項規格回填正對照、新352缺證據／限定範圍／狀態／三份較早回填／索引負例及既有負例通過。公開入口：python3 apps/moo2/tools/startup_probe_131.py --check-generation-180m-spec-backlinks，只驗實際續行與原返回／新CPU停止及回填，不啟動DOSBox-X。主庫RE-first保持，remake／中文化仍未完成。

## 本機忽略證據索引

| 收據／核算 | SHA-256 |
| --- | --- |
| moo2-349-ida-ui-ret.json | 0c16013bacada3769016549c3a30cfe12ddc2ea72b3829d20ff77256969b0dec |
| moo2-352-continuation.go | 7e03730549d3b9d74f160becde2780e31251b9b1d1437ad3c5181240b191152b |
| moo2-probe-352-continuation.txt.gz | db7630f02daaac46f0a2d45da1813a319d27e14e48c77c7b07a75a97b338c60f |
| moo2-vbe-352-continuation.png | d493c2b5628d55381176c9e676586ab8940fd62544302195b59570b6136e6ba6 |
| new-game-352-run.sh | 94f2aa11cd4897af8ed9337adb0d2bcb48a35f9ef845e57eb1b6085b7f2a01b4 |
| new-game-352-verify.py | e31817cc2b71b7df2fb1944dce6166318aa1237ece1c76d8e275417135ff0dbb |
| new-game-352-tests.txt | 798aa04bf026fa1b99af5815065506f1189a7b2e56d04886414453e7d5b65754 |
| new-game-352-source-verify.py | f53b017083b2749e04ccba39da8e2307f785ca5e2af566748dc806f330c48d9c |
| new-game-352-source-tests.txt | f9ab870f8ce6d2eee6f91be4fecb86fc51eab23a23a9890826b2e2cce48a7cff |
| new-game-352-off-run.sh | e6dd0e59c2d3cdf15024697454fe138df6c42f661c1f46f3d81062e6c8740246 |
| new-game-352-off-cli-tests.txt | a5eabaf6f165680e4e73a808efc13d87da68f29a4c55ca63bd6bf4ec96c09dff |
| moo2-probe-352-off-old.txt | d5d6210c5e646dd9b39e5547c986cd552099d9ababead60d858d9a3cfe7a82cf |
| moo2-probe-352-off-new.txt | cf37120a71d5f990a3018b630c4b3a6454e397d2fa235b24679ad811defa4d36 |
| moo2-vbe-352-off-old.png | dd4c21dd11e57ec86ef759182fd3ebadd6b53870b54de1ef192e2553820286db |
| moo2-vbe-352-off-new.png | dd4c21dd11e57ec86ef759182fd3ebadd6b53870b54de1ef192e2553820286db |
| new-game-352-backlink-verify.py | 29323d3879ef89ef69b0f13f8426fa2c75b1c72aab1af638c5547cbf006c2863 |
| new-game-352-backlink-tests.txt | 72a022f287c95347684deb10b1c9dd627ebd184962880627cbd186ae77c6f66f |
| new-game-352-frames.json | e20e80211f7a9a28d974451ac23ce6f43cd4ae93e7e69d5c56ed8b1e33f13ff9 |
| new-game-352-patches.json | cdd4bd25a4836607eb32381a2fb9bdf7607910e8059cccb20c7cdfce72e2a33f |
| new-game-352-cli-verify.py | aa13e74dfee1e0b0119650bf5bcb5af87eeedba0ae236e4a7d1a64c4c02973ef |
| new-game-352-cli-tests.txt | 0c39c7d30d26388473df3e31ad7be38651d38f98677857d9e5a9de59a4aaac43 |
| new-game-352-attempt1-verify.py | c88d35f658a09cf5663c16e0744e82e78eb70e22e09a93f908e57f5774ad7163 |
| new-game-352-attempt1-tests.txt | 38ff0861e4b9e06c158d148eab8af7eab4804d8ee9f5ff4c84167051611abcf9 |
| new-game-352-attempt1-verify-output.txt | 9f7b7da0a8992dd918da5c9800a341794e870d3d3bdbc28ff5129b5539963772 |

### 355的後續勘誤回填

原1CDD0F byte記憶體來源ADD與五步零值消費已由規格355接通，見[355](355-cpu386-add-byte-memory-source.md)。四SS來源00／AL00與第五目的00→00、flags202h→246h及RAM保持已驗，原非零ADD／進位未驗；10759正常前綴／35frames與固定EXE全套保持。原164561579的1CE387記憶體SETE已由356接通；下一步依359回填帳核對原1D2A33的CWD與下一SUB／shift，沿同180M，不增加cap或深入helper。完整生成／開局、正式writer、RNG與remake同狀態未知。

### 356的後續勘誤回填

原1CE387記憶體SETE寫回與下一JMP已由規格356接通，見[356](356-cpu386-setcc-byte-memory.md)。原SS188:2BD834 byte41→01／唯一RAM差異、flags246h保持與原JMP到1CE61E已驗；第三CMP數值與byte1 reader未驗。通用16條件memory純寫、343／344舊memory負例限定未知selector及完整新正例、固定EXE全套通過。原356於164567987停在1CF90A的word IMUL，來源word0000與原低word寫回已由357驗證；下一步依359回填帳核對原1D2A33的CWD與下一SUB／shift，沿同180M，不增加cap或深入helper。完整生成／開局、正式writer、RNG與remake同狀態未知。

## 2026-10-04 word立即值IMUL回填

原1CF90A word立即值IMUL與兩MOV零寫已由規格357接通，見[357](357-cpu386-imul-word-immediate.md)。原DS188:5A2084 word0000×5=0、EDI005AA5F4→005A0000／高word005A保持、定義CF／OF0與下一兩MOV dword0→0已驗；兩MOV不消費DI，undefined flags保存只屬工具近似。10767正常前綴／35frames／固定EXE全套保持，正式DI reader／原非零與overflow未知。原357的1CFD3F word NEG來源0000與原零值結果／下一分支已由358驗證；下一步依359回填帳核對原1D2A33的CWD與下一SUB／shift，沿同180M，不增加cap或深入helper。完整生成／開局、正式writer、RNG與remake同狀態未知，保留本檔原歷史定位與收據。

## 2026-10-04 word NEG回填

原1CFD3F word NEG與下一EB09已由規格358接通，見[358](358-cpu386-neg-word.md)。原DS188:5A207C word0000→0000、六定義flags246h與下一EB09到1CFD4E已驗；第三CMP只觀測flags246h→206h，SS:[EBP-564]來源未取，數值／NEG word reader不列驗收，第四JE按觀測ZF0不跳。10770正常前綴／35frames及固定EXE全套保持，晚期Bus第二byte失敗可部分寫但不發布flags。325舊66負例限定未知selector、268舊word register NEG負例加segment prefix，新全值域正例接合法word。歷史358於164610300在1D0944的word SUB memory目的拒絕，after1D0946只解碼、當時DS188:5AA6D1目的word未知／來源AX0，現已由359核對；下一步依359回填帳核對原1D2A33的CWD與下一SUB／shift，沿同180M，不增加cap或深入helper。原非零NEG／溢位與完整開局、資料語意／正式writer／RNG及remake同狀態未知，保留本檔原定位與收據。

## 359回填

原1D0944 word SUB與三POP及RET已由規格359接通，見[359](359-cpu386-sub-word-register-source.md)。原DS188:5AA6D1 word0003-AX0000=0003、六flags206h，SS188真正槽的EDX0000000E／ECX005AA5E8／EBX00000000與RET001D1E0B／ESP2BDB5C已驗，POP／RET不是目的word reader。10775正常前綴／35frames／固定EXE全套保持，既有ADD／其他SUB及十一舊測試不改，第二byte晚期部分寫不發布flags。原164984957於1D2A33的66 99 word CWD拒絕，after1D2A35，AX0001／DX0000；下一步依359回填帳取CWD／下一SUB與shift，沿同180M。原非零SUB來源／借位／溢位、目的word reader／欄位語意、正式writer／RNG／完整開局與remake同狀態未知，保留原歷史定位與收據。
