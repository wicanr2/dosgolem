# 351：後續生成的外層出口、後段與返回邊界

狀態：**CONFORMED，限定診斷驗收**
日期：2026-10-03

沿[350原迭代與界限](350-moo2-generation-iteration-bound.md)。起點主庫ac2784ad7fcac666da2b6f2aa0779d88163d6b79／工具025629e9fe5de8e29abe81aa08c67ad37a8ff161。主庫RE-first保持，只擴充dosgolem有界唯讀診斷，不修改玩法、CPU或平台。

## 來源與蒐證契約

官方DOS1.31 ORION2.EXE SHA-256 4e11be14217b4aafa1839f333bf5eba037f98b0c44e9e4752c96c464c260419f；CPU b5c8bd900047e8f018ad66edaf5ae8e1009e9f6d04ec89dea4000c27f3326b30；正式probe0cbe29a3e93145f2c7ee71036e0fe76902b6dbc76809571df761a3269d7e70d3。固定IDA349 JSON SHA0c16013bacada3769016549c3a30cfe12ddc2ea72b3829d20ff77256969b0dec，保留原名、IDA linear EA、file offset、bytes與operand；本輪只讀核對，不重建DB。

原350已驗首四SI迭代與該次signed界限36；160M原外層仍pending。下一步只追原sub_7AD13的外層出口／後段／RET及caller16BAE3，不深入runtime或繪圖helper。

1. 原417檔與MOX.SET、99M按下／99084355放開、1996-01-01及160M cap不改；固定日期不是RNG seed。全部10570原350列／36PNG按既有mtime／DTA／每次RAM規則比較。
2. private在原callee框架記16AD7D首72個head，保留實際SI與完整原狀態；最多72筆，之後飽和。此上限只限定觀察量，不代寫迭代或跳過呼叫。
3. 在本次350已量得的SI36，首次16AF1C／16AF23另記；並取首例16AF29出口、16AF35後段起點、16AF68後段出口、16AF6E選取分支、16B014／16B018兩返回結果入口、16B01A epilog、16B01F RET與caller16BAE3返回，共最多11筆；終態另一次，總最多84事件。非命中即明示未知。
4. 出口與後段要求原EBP=callerSP-24；RET要求原ESP=callerSP-4；caller返回要求原ESP／EBP恢復caller。不以保存返回槽宣稱RET已執行，也不從nested終態ESI猜外層索引。
5. 沿activationPeek核對核心／Bus／FPU／VBE與完整RAM before／after；讀取描述符／RAM邊界檢查。原code／bound operand導出的raw word、stack96byte、frame48byte有界唯讀；不裝hook、不換Bus或guest代寫，不重送輸入。
6. 對照固定IDA linear EA7AF29／7AF35／7AF68／7AF6E／7B014／7B018／7B01A／7B01F原bytes與實際事件。首尾未命中時不能稱完成，只依工作量證據決定下一次有限預算。
7. DRAFT private先實測／核算，證據足夠才READY審查，正式只能更名351標記而不改已驗執行語句；關閉8M／舊CLI、來源逆轉、回填與擁有權通過後限定CONFORMED。完整配置／開局、正式writer、RNG與remake同狀態未知。

沿既有Go1.24.13 image／UID1000，Docker --rm／network none／2GiB／2CPU／128pids及450s原版外層timeout；原ZIP／patch只讀，輸出只用既有忽略workplace／tmp。原EXE／LBX／RAM／LOG／PNG／私有腳本不入Git，同次公開索引掛350及351入口；收尾核對輸出UID1000／root-owned與.md目錄、兩庫HEAD與遠端及清理專案容器。

## READY 證據審查

private原160M已核對全部10570原350列／36PNG與終態保持，27事件readonly。26個head的SI嚴格為1..26，沒有回跳／重複，153880145到159926951共6046806原步，相鄰head間隔151326..338347；160M只讓第26次再續73049步，觀察未飽和full=false。原CMP界限13+27等既有事件都仍raw2400／signed36；本輪沒有見到SI36、外層出口、後段或RET，不將靜態定位升格為動態返回。

證據足以批准有界readonly診斷，正式只能更名351標記，原執行語句不變。原frame／RET／caller各有獨立驗證條件，universe160關閉不讀guest或defer輸出；最多72heads＋11邊界＋終態1筆。CPU／平台／原正常輸入／160M cap／主庫玩法不改。正式關閉8M／舊CLI、來源逆轉與回填通過後只限定CONFORMED於已量到的索引進度、截斷點與未見邊界。

下一次可依本輪26次進度與耗時增長，建立固定180M續行診斷。相較160M加20M，作為尚餘SI26..35與原後段的有界探索預算；這是依已量工作量選定的實驗預算，不保證後段、重入或完整開局完成，不更改正式遊戲。

## 限定驗收：原連續迭代與160M截斷點

351限定CONFORMED，只驗原連續SI1..26、實際步數、160M截斷與未見出口的範圍。完整生成／開局與remake同狀態仍未驗。原SI36的CMP／JL、16AF29出口、16AF35..16AF6E後段、16B014／16B018結果入口、epilog／RET與caller返回都未命中；靜態定位不是動態完成證據。

| 原head | 原步 | dosgolem_high_le EIP | 原結果 |
| --- | --- | --- | --- |
| SI1 | 153880145 | 16AD7D | 原callee EBP2BDB60／ESP2BCF60 |
| SI2 | 154031471 | 16AD7D | 間隔151326原步 |
| SI4 | 154349386 | 16AD7D | 與350前四組保持 |
| SI10 | 155465215 | 16AD7D | 原連續索引無回跳 |
| SI20 | 158010634 | 16AD7D | 仍在第一段外層迭代 |
| SI25 | 159588604 | 16AD7D | 原連續索引保持 |
| SI26 | 159926951 | 16AD7D | 間隔338347原步 |
| 固定終態 | 160000000 | 17FD04 | 第26次僅續73049步，outer_returned=false |

**已證實，原連續進度**：26個head的SI為1..26，無重複／回跳，時間序列嚴格遞增；head1到26共6046806原步，相鄰間隔151326..338347。25個間隔完整保存，不能從少量樣本當作固定每圈耗時。每個head EBP2BDB60／ESP2BCF60，原bound讀DS188:28199A raw2400／signed36。terminal ESI2BDB44是nested框架，外層索引用最後實際head26，不用terminal ESI猜值。

**已證實，截斷範圍**：27事件readonly，heads26／events27／full=false，所有首例boundary seen皆false，沒有觀察飽和。原160M只在第26次再跑73049步便停止，離SI36的比較及出口尚未取得收據。原外層returned=false／17FD04／unique_sites39434／無新CPU拒絕；不將退出碼0當作完成。

**已證實，靜態邊界定位**：重用固定IDA9.4 linear EA sub_7AD13的原7AF29 B9FFFFFFFF、7AF35 E884390800、7AF68 0FBFC1、7AF6E 0F8EA4000000、7B014 30C0、7B018 B001、7B01A C9及7B01F C3，原名／file offset／operand保持。IDA JSON SHA0c16013bacada3769016549c3a30cfe12ddc2ea72b3829d20ff77256969b0dec，官方EXE4e11be14…。與dosgolem_high_le定位分別記載，本輪沒有新的IDA執行；未命中出口不升格為返回已證實。

**強推論，下一次實驗預算**：26次連續進度及相鄰耗時整體增長，說明160M不足以量到整段出口。下一次固定180M，加20M供SI26..35及後段探索；20M約為10個最大已量間隔338347的5.9倍。這是有界試驗的選定餘量，不是剩餘工作的時間上限或完成保證。後段／重入可能需要更多，屆時依新證據判定，不盲加cap。

**目前未知**：後續配置／重入、後續重繪、正式writer、原欄位語意、完整配置／開局、RNG及remake同狀態。原第一callee SI1..35／出口／後段／RET及caller16BAE3已由352驗證。下一步依352的新81 word停止接窄CPU支援，再用同180M正常情境核對實際寫回與後續玩家畫面，保持原輸入與資料。主庫RE-first保持，不修改玩法或跳過生成。

## 保持、接線與重生

全部10570原350列／36PNG保持，沿既有mtime／DTA／每次RAM規則。27事件核心／Bus／FPU／VBE及完整RAM before／after相等，無guest代寫、hook、換Bus或重送。final PNG c7534b8f40b51b8377d255d66e6dd759dfb3d427fa6fccc6ee7d8c3999b32bca／RGB4a1d9efc9d15da575e330128f22d27e97f6d6616ac1b4184ec94efc2bd27f2c9仍配置母星。

READY審查後正式只更名351標記，兩區塊逆轉逐byte保持350；CPU／平台不改。private185d611e3cccf08c5b4a0d2c423766e7dcb5bf0e1119bcb02475a9edda3b5465，正式6cc3c65f7648e9582713a69fd51fa13a2d2a824cdec79204a7c88e90b22a183c。原160M收據SHA-256389a328d590e406bf2a09134cbc864476bcee5ee31e09288e82a1c51fb65db95。universe160關閉不讀guest或defer輸出，正式8M啟動1693原列／PNG與68舊CLI負例及100M／120M／160M正對照保持。沒有generation_completion_新增觀察；本輪不重跑120M或無關CPU全套。

原Go1.24.13 image sha256:1a6d4452c65dea36aac2e2d606b01b4a029ec90cc1ae53890540ce6173ea77ac，Docker --rm／network none／UID1000／2GiB／2CPU／128pids，原版timeout450s、正式8M／CLI180s。原ZIP／patch只讀，原417檔／MOX.SET／99M press與99084355 release／1996-01-01及160M cap保持；日期不是seed。

容器入口：bash workplace/new-game-351-run.sh／new-game-351-off-run.sh，python3 workplace/new-game-351-verify.py／new-game-351-source-verify.py PASS；go build -p 2 -buildvcs=false -o /tmp/moo2-probe ./workplace/moo2-probe。原EXE／LBX／RAM／LOG／PNG／私有腳本留忽略workplace，不入Git。

## 不可變鍵與較早回填

| 不可變鍵 | 已證實／未知 | 較早規格 | 必須回填 |
| --- | --- | --- | --- |
| DOS1.31／EXE4e11be14…／dosgolem_high_le:16AD7D／160M | 原SI1..26連續、無飽和與第26次截斷已證實，出口未見 | 350 | 原連續SI1到26與160M截斷已由規格351驗證 |

88項回填正對照與新351缺證據／限定範圍／狀態／較早回填／索引負例、既有負例通過。公開入口：python3 apps/moo2/tools/startup_probe_131.py --check-generation-completion-spec-backlinks，只驗原連續進度／截斷／未見出口與回填，不啟動DOSBox-X。整款remake／中文化尚未完成。

## 本機忽略證據索引

| 收據／核算 | SHA-256 |
| --- | --- |
| moo2-349-ida-ui-ret.json | 0c16013bacada3769016549c3a30cfe12ddc2ea72b3829d20ff77256969b0dec |
| moo2-351-completion.go | 185d611e3cccf08c5b4a0d2c423766e7dcb5bf0e1119bcb02475a9edda3b5465 |
| moo2-probe-351-completion.txt.gz | 389a328d590e406bf2a09134cbc864476bcee5ee31e09288e82a1c51fb65db95 |
| moo2-vbe-351-completion.png | c7534b8f40b51b8377d255d66e6dd759dfb3d427fa6fccc6ee7d8c3999b32bca |
| new-game-351-run.sh | 25acc19b3a0b9cf04d5eb82b1c9df952dba988c433ac463cddcb965f3165beae |
| new-game-351-verify.py | 6910cd202514cf6b5a5ac333ae3507feebd86cbdf85d012dfbf18d26825cbe92 |
| new-game-351-tests.txt | 83c52d22e97fb63c4a13bebdec6d2158af7fc5a012c8cc8894ba6c0f3fcfd2ae |
| new-game-351-source-verify.py | 90b8631a7ec629c9b305f4dfd19da6d9852fa0ea3ae6c7bd21d31e7c0f52f2b4 |
| new-game-351-source-tests.txt | c5399648a97935aa06578278eedee83a817f58f9e725a8a501688b94eef9e202 |
| new-game-351-off-run.sh | 923783a3c35a606ecc795e92dc3d3d4188d90f7f314273f4d8a2fd80c9eced0e |
| new-game-351-off-cli-tests.txt | 7d720ece028de31c0f318e14ac3aa0eaf3a62300093ab5410538b8b4007df7f1 |
| moo2-probe-351-off-old.txt | b26779d13b2b12e3bc94db2813b50f0b1920e1dc2b4d4ade45e62e480e6b44c7 |
| moo2-probe-351-off-new.txt | bd693fc70584776eadc2835d35d072a58b625781a3ae52def9e81c51a510ed69 |
| moo2-vbe-351-off-old.png | dd4c21dd11e57ec86ef759182fd3ebadd6b53870b54de1ef192e2553820286db |
| moo2-vbe-351-off-new.png | dd4c21dd11e57ec86ef759182fd3ebadd6b53870b54de1ef192e2553820286db |
| new-game-351-backlink-verify.py | 48c5f0b652b55799313da611b25391d0c239df15f1aa17b33447aab8f08fc2de |
| new-game-351-backlink-tests.txt | afd425de1526b803e70c395ff059efbf2e9eee1b63420ba946d29b44e8a1e694 |
| new-game-351-frames.json | 77c5cc266bb737831784fc814d9d3fbd356db8a74e192d53c201ab59fd27d1b7 |

## 352後續回填

原生成35迭代與真正RET及caller返回已由規格352驗證，見[352原出口／返回與新停止](352-moo2-generation-180m-continuation.md)。163755071原SI36的JL不跳，163778787的16B01F C3後次步16BAE3，原AL0／caller ESP與EBP恢復；163779084的16BAEC ZF1，次步16BB00。原第一callee的全部35迭代、出口／後段／RET已閉合，本文160M未見是當時範圍。完整配置／開局與後續writer仍未知；新103BF9的66 81 /4 word記憶體AND拒絕未解。下一步接該CPU缺口，再用同180M正常情境核對，不重開已驗原返回。
