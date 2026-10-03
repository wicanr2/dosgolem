# 357：立即值IMUL的word目的

狀態：**CONFORMED，限定CPU契約／原零值IMUL與兩MOV／正常續行至新停止**
日期：2026-10-03
範圍：cpu386的66 6B /r ib與66 69 /r iw，word register／memory來源，沿32位ModRM／SIB與DS／SS；既有dword立即值與其他乘法保持，不擴張段覆寫、67或F0／F2／F3。

## 原阻塞與公開契約

沿[356](356-cpu386-setcc-byte-memory.md)，工具442eef487fa03be9ef0f793e233396120c56973b、CPU SHA-256 17854f07854ba4d59e70b9ac4ed4b1ec2b5e1a01b4df4336c60ef3bbed2ffe66。固定官方DOS1.31 ORION2.EXE SHA-256 4e11be14217b4aafa1839f333bf5eba037f98b0c44e9e4752c96c464c260419f。原164567987在dosgolem_high_le:1CF90A bytes66 6B 7B 40 05 C7 45 D8 00 00 00 00 C7 45 E0 00拒絕operand16，after1CF90C只解碼，未取ModRM／來源。原R=[5A2044 5AA5F4 5AA5E8 5A2044 2BD920 2BDB54 5AA614 5AA5F4]／段=[8 188 188 0 20 188]／flags206h。DI目的，DS188:[EBX+40h] offset5A2084來源word未知，imm8 05h；三運算元不把舊DI當乘數。後續原兩個C7的SS:[EBP-40]／[EBP-32] dword與原值未知，不猜用途。

[Intel 80386 IMUL](https://www.ardent-tool.com/CPU/docs/Intel/386/manuals/prref386/IMUL.htm)定義word來源乘signed imm8的符號延伸或signed imm16，低16bits存目的word；完整signed積可放int32。CF／OF在signed積超出[-32768,32767]時皆1，否則皆0，SF／ZF／AF／PF未定義。沿既有dword立即值與[268](268-cpu386-imul-word-register.md)工具政策保留未定義flags，標為工具相容近似，不稱原硬體逐旗標exact；原核對只驗定義CF／OF，原其餘flags只作工具觀測。

## 最小實作與驗收

DRAFT保持CPU，以同180M正常輸入首遇最多三步診斷，code24、完整R／六段／flags、DS:[EBX+3Fh]四bytes及SS:[EBP-44]二十bytes，activationPeek及完整RAM雜湊證只讀。每Step記全部RAM差異索引／callback／IRQ，錯誤即停；全部356舊列／PNG保持。word與原後續MOV立即數可讀並公開ISA充分後才READY。

READY在既有69／6B分支允許operand16，新增獨立width16路徑，register取低word或readSegment16；完整fetch imm8／imm16後才發布低word與CF／OF，目的高16bits／其他R／段／FPU／RAM保持。source readonly，base／index／目的別名先取得位址與值，既有dword路徑逐byte保持。全部ModRM／SIB／DS／SS與唯讀來源、last word／越界／線性溢位／Bus第一或第二byte讀失敗／截短立即數與prefix均須驗，不發布R或flags，EIP依工具錯誤模型可已前進，不稱硬體exception restart。

獨立oracle以signed數學範圍判overflow，用LittleEndian兩byte視圖寫低word，與CPU截斷符號延伸判準分開。6B全部65536來源×256 imm8，八目的與八來源別名另抽所有signed邊界；69所有65536立即數×充分signed邊界來源及八目的。兩種flags初態／nonzero FPU／source與相鄰memory不寫，undefined flags只驗工具保留。既有F7／0FAF／dword69／6B與舊register、全套必須保持。

原正式驗word×5結果／DI低16bits與高16bits保持、CF／OF及下一兩MOV的逐byteRAM變更與相鄰資料保持；兩MOV不消費DI，不當乘積reader。首遇前正常前綴／frames保持，finalPNG按實際核對。固定原EXE乾淨Go全套必須全過，缺8088語料不算386實機驗收。不跳原指令／代寫guest／重送input／增加180M。固定1996日期不是seed，原非定義flags／正式DI reader、完整生成／開局與remake同狀態未知，主庫RE-first保持。

## 工具與入口

Go1.24.13 Docker image sha256:1a6d4452c65dea36aac2e2d606b01b4a029ec90cc1ae53890540ce6173ea77ac，UID1000／network none／2GiB／2CPU／128pids／600s，原ZIP／patch唯讀。原EXE／LOG／PNG／RAM與私有腳本留忽略workplace，公開自製CPU／測試／診斷／spec／索引／守衛與雜湊。本檔同次加入000-index；原版入口workplace/new-game-357-input-run.sh。主要原版執行器/home/anr2/cht/dosgolem的隔離副本workplace/dosgolem，其能力／用法見README.md／CLAUDE.md。

## READY：原來源已取得

狀態轉移：DRAFT → **READY**。未改CPU的正常180M輸入收據47e10ec4128cb451881b59741a9f46ebdff374cdd27409b8834edafad4d4fc0b，10837舊列／36 PNG逐byte保持，三observer逆轉保持356。原164567987的DS188:5A2083四byte 00000000，真正來源word DS188:5A2084=0000；0000×signed 05=0，DI A5F4→0000，EDI預期005A0000，高word005A保持。CF／OF皆0；其餘旗標只作工具保留近似，無原硬體undefined flags parity。

原SS188:2BDB28二十byte FFFFFFFF00000000000000000000000080091D00；下一兩C7分別在1CF90F／1CF916，把SS188:2BDB2C／2BDB34的dword 0寫0，預期EIP1CF916／1CF91D且RAM保持。兩MOV不讀DI，不證明乘積的後續消費。原只讀擷取ram_changes=[]，callback12/12、IRQ41948/41948、pending0／非active／非failed。原來源為零，非零積／負值／overflow依公開ISA由獨立測試覆蓋，原範圍不外推。

## 2026-10-04 限定驗收：word立即值IMUL與原兩MOV零寫

**已證實**：DRAFT未改CPU全部10837原356列／36PNG保持；首遇來源word與frame由同一原EXE擷取，才審查READY。正式首遇前10767共通正常列／35既有frames保持，原輸入R／段／flags／code24／來源四byte與frame二十byte逐欄相同。

原164567987在1CF90A的66 6B 7B 40 05，DS188:5A2084 word0000×5=0；DI A5F4→0000，EDI005AA5F4→005A0000，高word005A保持，EIP1CF90F。CF／OF皆0；未定義旗標只屬工具保留近似，flags206h保存不稱原硬體逐旗標一致。最初READY文字將六位hex初值的高word誤寫為5AA5，現已更正005A；獨立little-endian視圖與正式原輸出皆為005A0000，CPU實作無此錯誤。

原164567988／164567989的C7 45 D8 00000000／C7 45 E0 00000000，SS188:2BDB2C／2BDB34 dword0→0，EIP1CF90F→1CF916→1CF91D；兩MOV不消費DI。三步全部R除首步DI／六段／flags／source與frame保持，ram_changes=[]，readonly=true／error nil，callback12／12、IRQ41948／41948非活動／非failed／pending0。原來源為零，非零積／負值／overflow與正式DI reader未由這條原路徑驗證；不把兩MOV算成乘積reader，也沒有原Bus寫次數trace。

**已證實，工程契約**：獨立signed數學範圍與little-endian低word視圖驗33554432個word來源×imm8×兩旗標案例，2097152個imm16×16邊界來源×兩旗標案例，全部八目的×八register來源與別名；memory全ModRM／SIB／DS／SS、signed位移與32位地址繞回、唯讀來源／最後word、nonzero FPU／相鄰RAM、未知selector／越界／線性溢位／兩byte讀失敗／截短立即數與非法prefix均通過。只驗undefined flags的工具保留；來源不寫，完整取值後才發布低word／CF／OF。

第一次截短測試失敗：7.631s的op6B cut1為nil，測試誤用CS描述符限制取指。取指直接讀Bus；改為Bus拒絕第一個缺byte，保持完整可讀的memory來源與其他狀態後，同命令乾淨重跑4.372s通過。保留attempt1失敗輸出，不視為乘法產品缺陷或挑收據。CPU變更移除word早拒絕並增加獨立width16分支，逆轉逐byte保持356；既有dword／F7／0FAF及六份舊測試不改。三observer逆轉逐byte保持356。

固定官方原EXE乾淨Go全套通過：CPU386121.065s／machine1.556s。缺8088語料是證據限制，不算386實機驗收。關閉診斷8M1693原列／PNG保持；68舊CLI負例、32新180M負例與100M／120M／160M／180M正對照保持。

CPU SHA-256 94fab7c6e4389ce205c485dd498607f1442c20b3b7999212812f7e622fff4bf2；新測試628915d9b97a671bce0639d43e7644ec91a07563123c09e5cc4ce6c225291036；正式probe e08cf955cfb1b543f62a8cb573d2b6784630dc2bdc44f06fe47820a008cba2a7。原正式收據464f05e17bb736116a05a8f18d81edf45a9edcd0f40e4d3fd82c41b937822b71；全套114cc77fa894252e3fcc3a20b57a4992690101f642e00d1532f9c6c733730a16。

### 357歷史停止與已回填unknown

原164568139在dosgolem_high_le input1CFD3F的66 F7 5B 38 EB 09 8B 55 E4 29 C2 66 89 53 38 83，拒絕word memory NEG，after1CFD42只解碼，未取disp8或source。R=[0 0 4 5A2044 2BD920 2BDB54 0 0]／段=[8 188 188 0 20 188]／flags246h；DS188:[EBX+38h] offset5A207C來源word未知，尚未達180M。來源與原NEG已由358接通；下一步依360回填帳取原2376CB的DS188:270FC4 dword ROR來源／相鄰資料與下一A1 load，沿同180M，不增加cap或深入helper。

finalPNG逐byte保持356，SHA-256 1f757f5b16fe492795accf198c8db851443a062450e7f37ecf465c7eae3b6457／RGB04fef4b6a6d6c6c485ef1ce0a82ea71591956cdd3b7cd37b8d1082a023e20e17。沿354人工檢視仍主要黑底與小型方形圖形，未見完整地圖，不當開局完成。固定1996日期不是seed；正式DI reader、原非零／signed overflow、NEG來源／後續消費、資料語意、正式writer、完整生成／開局與remake同狀態仍未驗，主庫RE-first保持。

### 回填帳

不可變解析鍵：固定DOS1.31 EXE 4e11be14…＋dosgolem_high_le:1CF90A＋66 6B 7B 40 05。原1CF90A word立即值IMUL與兩MOV零寫已由規格357接通；356／355／354／353／352／344／343及268／269的入口與現行unknown同次回填，保持各自歷史定位與限制。索引000-index及--check-imul-word-immediate-spec-backlinks守衛驗證此限定範圍，不能用綠測試宣稱完整開局或remake parity。

### 實際命令

以下在Docker、隔離副本workplace/dosgolem執行，原ZIP／patch唯讀；DRAFT兩命令是CPU442eef4時的歷史擷取，重生DRAFT須使用該CPU與357只讀observer。
```text
bash workplace/new-game-357-input-run.sh
python3 workplace/new-game-357-input-verify.py
  未改CPU10837原356列／36PNG／word0000與frame／原拒絕 PASS
go test -p 2 -buildvcs=false ./internal/cpu386 -run TestIMUL -count=1
  attempt1 截短fixture失敗，Bus缺byte修正後4.372s PASS
bash workplace/new-game-357-full-run.sh
  乾淨固定原EXE Go全套 PASS
bash workplace/new-game-357-formal-run.sh
python3 workplace/new-game-357-formal-verify.py
  10767共通正常列／35frames／原word IMUL及兩MOV零寫／新NEG停止 PASS
python3 workplace/new-game-357-source-verify.py
  CPU新增width16分支／三observer逆轉保持356、六舊測試不變 PASS
bash workplace/new-game-357-off-run.sh
  8M1693列／PNG與68舊＋32新CLI負例／正對照 PASS
python3 workplace/new-game-357-backlink-verify.py
python3 apps/moo2/tools/startup_probe_131.py --check-imul-word-immediate-spec-backlinks
```

首次回填守衛拒絕文件的164567988／989縮寫缺完整164567989，補完整步號後同命令重跑；未修改CPU或原收據。

94項規格回填全過；新357的38項缺證據／限定範圍／狀態／356回填／索引負例、其餘八份較早回填另16及舊負例全過。首次步號縮寫拒絕摘要另存attempt1，CPU與原收據不變。來源／收據均1000:1000，工具root-owned／.md目錄零。

| 本機來源／收據 | SHA-256 |
| --- | --- |
| moo2-imul-word-357.go | e08cf955cfb1b543f62a8cb573d2b6784630dc2bdc44f06fe47820a008cba2a7 |
| moo2-probe-357-input.txt.gz | 47e10ec4128cb451881b59741a9f46ebdff374cdd27409b8834edafad4d4fc0b |
| moo2-vbe-357-input.png | 1f757f5b16fe492795accf198c8db851443a062450e7f37ecf465c7eae3b6457 |
| new-game-357-input-run.sh | d8e2e70d86ed132223d836591af364cc94798a7c79e97c0c64c059d27064d0c3 |
| new-game-357-input-run-output.txt | de768f16a275eed1b0aea16376f2b912c69c608592a2f607ad2d199f4840bb39 |
| new-game-357-input-verify.py | 211548c27443824d4c0776e47045459a99ebd44e595a1575d05cc5d1fc421ff6 |
| new-game-357-input-tests.txt | 315f1767ae0a5d03a245c1d87759f9a16d9007539b6ce04cce3e472f0b977f56 |
| moo2-probe-357-formal.txt.gz | 464f05e17bb736116a05a8f18d81edf45a9edcd0f40e4d3fd82c41b937822b71 |
| moo2-vbe-357-formal.png | 1f757f5b16fe492795accf198c8db851443a062450e7f37ecf465c7eae3b6457 |
| new-game-357-formal-run.sh | 5baa5359f7b8a6ae721585a475e9da3d33df08809a53c800d4dcdc494aa80cf1 |
| new-game-357-formal-run-output.txt | 5c4821ee66c0580a83900be3c096977f43471946a3767e1a9adb19f88a04c89b |
| new-game-357-formal-verify.py | 765061983c4fcc3e906389efe7fd982d415916ef03106d5cbe6d1432fd2b14ce |
| new-game-357-formal-tests.txt | 2835387eca804693752d275f41e7c34a8c227ae074a788181058ee77edd9654a |
| new-game-357-unit-tests.txt | 33d4dc610a69ab1ba9663ad9ad2863e0b6c8a37cbf1e38e617ee693c3dba5883 |
| new-game-357-full-run.sh | 2277cfdca014a70dbd902dc3eae95e3268c7cb2789ff0037035e3020b30d565b |
| full-test-357.txt | 114cc77fa894252e3fcc3a20b57a4992690101f642e00d1532f9c6c733730a16 |
| new-game-357-source-verify.py | eade6f79cc4ad877e18a912ec5b5c02ac496d0cb0857922e84976d433ae03e99 |
| new-game-357-source-tests.txt | 0207dbe9a870631abca1548e17d591ba6e559d9c1f05f6537313bd2762afb4de |
| new-game-357-off-run.sh | f697bb45bc9c9bc9d71847e7ee9fc6e0a01d98083fa20e17db25c3ffaf8c50ad |
| new-game-357-off-cli-tests.txt | a5eabaf6f165680e4e73a808efc13d87da68f29a4c55ca63bd6bf4ec96c09dff |
| moo2-probe-357-off-old.txt | 3b1453f36f0ce3d75a1bee99c2d5de4aec4ac463b60417134e753130e3297b36 |
| moo2-probe-357-off-new.txt | f438eea44fd91d5267b2a6f77693434fe039a638dc21fa58f69a81a5448ffb09 |
| moo2-vbe-357-off-old.png | dd4c21dd11e57ec86ef759182fd3ebadd6b53870b54de1ef192e2553820286db |
| moo2-vbe-357-off-new.png | dd4c21dd11e57ec86ef759182fd3ebadd6b53870b54de1ef192e2553820286db |
| new-game-357-cli-tests.txt | 0c39c7d30d26388473df3e31ad7be38651d38f98677857d9e5a9de59a4aaac43 |
| new-game-357-backlink-verify.py | 9c20c505262dc71b14e1f9ddbc7f07c6d85c770b340fa2773bbc52b898b9d44d |
| new-game-357-backlink-tests.txt | 0870d186f33b7775bac8d228a186e72275f5e1c95d2ff8791d4f047a84801ec6 |
| new-game-357-unit-tests-attempt1.txt | d31e60ddb220607f0f5ab62749843d20e1113f177505cc8003c514bb9f373e29 |
| new-game-357-backlink-tests-attempt1.txt | a11e1282fe16a014f6d6f7f79e9d61d37ff991d4f1122fc96c3fb3ef8e38aa9e |

## 2026-10-04 word NEG回填

原1CFD3F word NEG與下一EB09已由規格358接通，見[358](358-cpu386-neg-word.md)。原DS188:5A207C word0000→0000、六定義flags246h與下一EB09到1CFD4E已驗；第三CMP只觀測flags246h→206h，SS:[EBP-564]來源未取，數值／NEG word reader不列驗收，第四JE按觀測ZF0不跳。10770正常前綴／35frames及固定EXE全套保持，晚期Bus第二byte失敗可部分寫但不發布flags。325舊66負例限定未知selector、268舊word register NEG負例加segment prefix，新全值域正例接合法word。歷史358於164610300在1D0944的word SUB memory目的拒絕，after1D0946只解碼、當時DS188:5AA6D1目的word未知／來源AX0，現已由359核對；下一步依360回填帳取原2376CB的DS188:270FC4 dword ROR來源／相鄰資料與下一A1 load，沿同180M，不增加cap或深入helper。原非零NEG／溢位與完整開局、資料語意／正式writer／RNG及remake同狀態未知，保留本檔原定位與收據。

## 359回填

原1D0944 word SUB與三POP及RET已由規格359接通，見[359](359-cpu386-sub-word-register-source.md)。原DS188:5AA6D1 word0003-AX0000=0003、六flags206h，SS188真正槽的EDX0000000E／ECX005AA5E8／EBX00000000與RET001D1E0B／ESP2BDB5C已驗，POP／RET不是目的word reader。10775正常前綴／35frames／固定EXE全套保持，既有ADD／其他SUB及十一舊測試不改，第二byte晚期部分寫不發布flags。歷史359於164984957在1D2A33的66 99 word CWD拒絕，after1D2A35，AX0001／DX0000，現已由360核對；下一步依360回填帳取原2376CB的DS188:270FC4 dword ROR來源／相鄰資料與下一A1 load，沿同180M，不增加cap或深入helper。原非零SUB來源／借位／溢位、目的word reader／欄位語意、正式writer／RNG／完整開局與remake同狀態未知，保留原歷史定位與收據。

## 360回填

原1D2A33 word CWD與下一SUB／SAR已由規格360接通，見[360](360-cpu386-cwd-word.md)。原AX0001／DX0000與CWD完整flags246h保持，真正SUB讀DX以1-0=1／六flags202h，SAR讀AX1→0與五定義flags／EIP1D2A3B已驗，AF不列原版parity。三步全部RAM／FPU原bits保持、10789正常前綴／35frames／固定EXE全套通過；裸CDQ／word SUB與SAR／十三舊測試不改。原168496272在2376CB拒絕C1 /1 memory ROR，DS188:270FC4來源未知／imm08、after2376CD未取disp／imm或source；下一步依360回填帳取原dword／相鄰資料與下一A1 load，沿同180M。原負AX／EDX高word、新ROR來源與消費、正式writer／RNG／完整開局與remake同狀態未知，保留原歷史定位與收據。
