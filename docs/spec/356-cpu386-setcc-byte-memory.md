# 356：標準SETcc的byte記憶體目的

狀態：**CONFORMED，限定CPU契約／原SETE與JMP／正常續行至新停止**
日期：2026-10-03
範圍：cpu386裸0F90..0F9F memory byte目的，沿32位ModRM／SIB與DS／SS，沿344既有16標準條件；register／Jcc保持，不擴張operand16、段覆寫、67或F0／F2／F3。

## 原阻塞與公開契約

沿[355](355-cpu386-add-byte-memory-source.md)，工具8273d887f5387c23d9ae13056ae2ad1e263aeee0、CPU SHA-256 125674de469ef82d4abe457190e28eb0c75b88aca8a876349b395165e77c79e1。固定官方DOS1.31 ORION2.EXE SHA-256 4e11be14217b4aafa1839f333bf5eba037f98b0c44e9e4752c96c464c260419f。原164561579在dosgolem_high_le:1CE387的0F 94 45 F4拒絕，after1CE38A只解碼。R=[5AA5F4 0 0 256 2BD36C 2BD840 5AA5E8 5AA614]／段=[8 188 188 0 20 188]／flags246h，ZF1；SS188:[EBP-12] offset2BD834目的byte未知。後續原E9 8E 02 00 00為相對跳躍，不猜欄位或helper用途。

[Intel 80386原廠SETcc](https://www.ardent-tool.com/CPU/docs/Intel/386/manuals/prref386/SETcc.htm)列0F94為ZF1寫byte1，否則byte0，全部旗標保持。完整16條件沿[344](344-cpu386-setcc-byte-register.md)已驗字面真值表與signed／unsigned CMP；不沿鏡像表中SETG／SETLE的排字錯誤改條件。[Intel SDM Volume 2](https://cdrdv2-public.intel.com/671110/325383-sdm-vol-2abcd.pdf)的SETcc 4-620明列ModRM.reg未用、目的不可寫或段外拒絕。其未用欄契約沿現工具register模型擴至memory，不稱386實機未用欄已驗。目的一byte純寫入，不先讀目的一byte。

## 最小實作與驗收

DRAFT未改CPU，以同180M正常輸入首遇觀察最多兩原步；原SS:[EBP-13]三bytes／code16／完整R／六段／flags與callback／IRQ，activationPeek及完整RAM雜湊證peek只讀。每Step保存全部RAM變更索引，錯誤即停。全部355舊列／PNG保持、byte可讀與ISA充分後才READY。

READY沿既有16布林條件計算0／1，不改Jcc，decodeAddress32／writeSegment8成功後結束memory路徑；全部R／段／flags／FPU保持，唯一目的一byte可改，相鄰資料保持。全部ModRM／SIB、相異DS／SS、8未用reg欄、負disp8／繞回／最後byte。有效目的是純write，即使Bus目的read拒絕仍成功；相同byte仍需一write。唯讀／未知段／段外／線性溢位／Bus寫失敗不發布其他狀態，自製fail-before-write Bus的RAM保持，不宣稱任意Bus回滾；工具錯誤可已解碼前進EIP。

獨立測試沿344字面32bit真值位圖，全部16opcode×64flags×8未用欄×256初byte，驗全部flags與非零FPU／RAM鄰居；地址全形狀與兩種結果、signed／unsigned CMP的memory消費、失敗邊界、prefix／截短與既有register／Jcc均須保持。343／344舊全memory早拒絕負例轉為未知selector負例，有效memory由新完整正例驗收；不得刪prefix／截短或靜默刪test。

固定原EXE的乾淨Go全套必須全過，缺8088語料不算386實機驗收。原正式兩步驗SS byte寫回0／1與下一E9自然跳到1CE61E、其餘狀態／RAM保持，首遇前可比列／既有frames保持，finalPNG按實際核對。不跳原指令／代寫guest／重送input／增加180M。固定1996日期不是seed，完整生成／開局、原其餘15條件動態、正式writer與remake同狀態未知，主庫RE-first保持。

## 工具與入口

Go1.24.13 Docker image sha256:1a6d4452c65dea36aac2e2d606b01b4a029ec90cc1ae53890540ce6173ea77ac，UID1000／network none／2GiB／2CPU／128pids／600s，原ZIP／patch唯讀。原EXE／LOG／PNG／RAM與私有腳本留忽略workplace，公開自製CPU／測試／診斷／規格／索引／守衛與雜湊。本檔同次加入000-index，原版入口workplace/new-game-356-input-run.sh，主要原版執行器/home/anr2/cht/dosgolem，隔離副本workplace/dosgolem，其能力與用法見README.md／CLAUDE.md。

## 未改CPU來源與READY審查

DRAFT未改CPU，同180M正常輸入全部10834原355列／36PNG保持；三observer逆轉逐byte保持8273d88，CPU仍125674de…。python3 workplace/new-game-356-input-verify.py PASS；原輸入收據SHA-256 1e0b74050f15e9c731e279ae1e5e2a843054c3fec207ad4a9f44fee118c0e9ff。

原164561579 input1CE387的0F9445F4，SS188:2BD833三byte004100，目的一byte2BD834為41，R／段／flags246h與355停止相同。readonly=true／RAM保持，callback12／12、IRQ41946／41946非活動、pending0；原仍拒絕after1CE38A。byte與ISA充分，356審查READY。

正式首步預期目的一byte41→01／窗口000100、EIP1CE38B，完整R／段／flags246h保持，全部RAM差異只index2BD834。次步原E98E020000應自然跳到1CE61E、完整RAM保持。診斷預算最小增為三步，第三步在原跳躍落點只保存當前code16／完整state及RAM差異；按實際opcode核對，不預填未知reader或用途，若CPU拒絕即停。此增量只有診斷，不改正式CPU／輸入或180M。

## 限定驗收：SETcc記憶體目的與原SETE／JMP

356限定CONFORMED只涵蓋通用16條件memory byte契約、原SETE實際寫回及下一JMP，並正常續行至新停止。完整生成／開局與remake同狀態仍未驗，主庫RE-first保持。原其餘15條件未逐條動態實測；原新byte1的reader與第三CMP數值不列驗收。

**已證實，原兩步**：164561579於dosgolem_high_le:1CE387的0F9445F4，以flags246h的ZF1，把SS188:2BD834 byte41→01、窗口004100→000100，EIP1CE38B；全部R／六段／flags246h保持，完整RAM差異只有index2BD834，相鄰bytes保持。164561580下一原E98E020000自然跳到1CE61E，全部R／段／flags／RAM保持。兩筆readonly=true／error nil，callback12／12、IRQ41946／41946非活動、pending0。

**已證實，第三步觀測**：164561581原1CE61E的3B7DE0為CMP EDI,SS:[EBP-32]，EIP1CE621，R／段／目的窗口000100及RAM保持，觀測flags246h→202h。SS188:2BD820來源dword未擷取，不能獨立核算CMP flags，不列原第三步CMP數值驗收；它不讀SS:2BD834 byte1，不稱新byte的reader。沒有為補此非阻塞unknown新增helper研究或注入資料。

DRAFT未改CPU全部10834原355列／36PNG保持；正式首遇前10764共通正常列／35既有frames與同一原SS目的41／R／flags保持。正式finalPNG逐byte與355相同，SHA-256 1f757f5b16fe492795accf198c8db851443a062450e7f37ecf465c7eae3b6457／RGB04fef4b6a6d6c6c485ef1ce0a82ea71591956cdd3b7cd37b8d1082a023e20e17。沿354人工檢視主要黑底與小型方形圖形，未見完整地圖，不當正常開局完成。

**已證實，新停止**：164567987於dosgolem_high_le input1CF90A bytes66 6B 7B 40 05 C7 45 D8 00 00 00 00 C7 45 E0 00拒絕「IMUL立即值prefix尚未支援」；after1CF90C只解碼，未fetch ModRM／source或執行乘法。R=[5A2044 5AA5F4 5AA5E8 5A2044 2BD920 2BDB54 5AA614 5AA5F4]／段=[8 188 188 0 20 188]／flags206h。66 operand16的6B 7B 40 05為word目的DI、DS188:[EBX+40h] offset5A2084與sign-extended imm8 05h；原word來源未知，不猜欄位。requested budget180000000、actual stop164567987尚未達180M，probe exit0是錯誤收尾。

**已證實，CPU回歸**：16 opcode×64 flags×8未用reg欄×256初byte×兩context，共4194304個memory寫回，以344字面32bit真值位圖核算，未調CPU／Jcc條件。所有ModRM／SIB與相異DS／SS、正負結果、負disp8／繞回／最後byte、相同byte仍一write、非零FPU與全部未選狀態保持；Bus目的一byteread拒絕仍成功，證純write、各一Bus寫。77個值全部配對×10signed／unsigned CMP條件的memory消費由數學大小比較核算，不把CMP輸出當自己的oracle。唯讀／未知段／段外／線性溢位／Bus寫失敗、11prefix的合法目的對照、各ModRM／SIB／disp截短拒絕均不發布R／段／flags／FPU，fail-before-write Bus的RAM保持，不稱任意Bus可回滾。

原register SETcc／SETLE／Jcc、CMP→register與SS store回歸保持；343／344舊memory負例只明確加未知selector，344函式名改標UnknownMemory。有效memory由新完整正例驗收，原prefix／截短／相鄰0FA2／0FA3及真值表不刪；逆轉兩行selector／註解與單一函式名後逐byte保持舊測試。

go test -p 2 -buildvcs=false ./internal/cpu386 -run 'TestSETcc|TestSETLE' -count=1 -v PASS，7.994s。固定原EXE乾淨go test -p 2 -buildvcs=false ./... -count=1 PASS，CPU386137.976s／machine1.755s；git ls-files -z建乾淨/tmp/test-src，再複製新memory測試，沒納入歷史探索main或並行暫存probe。缺8088語料不算386實機驗收。關閉8M1693原列／PNG、68舊CLI＋32新180M負例與100M／120M／160M／180M正對照保持；明列來源建置，沒有355的暫存main問題。

CPU只移除3行早拒絕並加11行通用write，原16條件／register／Jcc逐byte保持355；三observer逆轉保持355，平台／8088 CPU不改。CPU SHA-256 17854f07854ba4d59e70b9ac4ed4b1ec2b5e1a01b4df4336c60ef3bbed2ffe66，新測試ff6d3f8f64c7981391b935a67041f956efd998187dcfb742785799427f70d463，舊SETcc測試c123f9ff7e5cd167bf65240766f8053fdf9e7e220cf78f5de38bae8a60f2a93a／SETLE測試47c2013c857eb78c004b115b50031101b362f81105b6b04ea3b2911e8b845a7c，正式probe a72124624383618cc50c2ddc143efb3a92f9c3dd91cbf4bcfde9cffb658c4f55。正式原收據386ac92b1570e1f1f7d4956752f2b87606b06e41286bc1b8c11bafb87df54978，Go全套6315adceb9f636c4a9628b2850c6d753eaa6b59166e142bc12a853577259887c。

DRAFT初態probe為兩步，已保存moo2-set-memory-356-input.go SHA-256 daac78a496a13350fbfe38961c1cfc9c7541049a05718b4ef9ac9359c5b1db11；READY後正式三步probe只改診斷budget。初次實際run腳本內容保存new-game-356-input-original-run.sh，SHA-256 6d9362371bb4fa2701a5ee07e2481203f8fb419232268d3f6de9211e65d31ce4；重生入口new-game-356-input-run.sh改指向保存的初態source。舊LOG／PNG不重寫，全部CPU、正式原版與驗證通過；沒有失敗後挑選收據。

### 回填帳與下一步

| 不可變鍵 | 已證實語意 | 較早規格 | 必須回填 |
| --- | --- | --- | --- |
| DOS1.31／EXE4e11be14…／dosgolem_high_le:1CE387→1CE38B→1CE61E | 原SETE SS byte41→01／唯一RAM差異與下一JMP、flags保持 | 355、354、353、352、344、343 | 原1CE387記憶體SETE寫回與下一JMP已由規格356接通 |

下一步依361回填帳捕捉180M旗色選單存檔錯誤的真正DOS呼叫／檔名與返回，審查既有隔離覆蓋層，沿同180M，不提高cap或深入helper。第三CMP數值／byte1 reader、正式DI reader、資料語意、正式writer、完整生成／開局與remake同狀態未知。

### 本機忽略證據索引與命令

93項規格回填正對照、新356的37缺證據／限定範圍／狀態／355回填／索引負例、其餘五份較早回填另10及較早負例通過。所有來源／收據1000:1000，工具root-owned／.md目錄零。

```text
bash workplace/new-game-356-input-run.sh
python3 workplace/new-game-356-input-verify.py
  未改CPU10834原355列／36PNG／SS目的41／原拒絕 PASS
go test -p 2 -buildvcs=false ./internal/cpu386 -run 'TestSETcc|TestSETLE' -count=1 -v
  全memory真值／初byte／純write／拒絕邊界與舊register、Jcc PASS
bash workplace/new-game-356-full-run.sh
  乾淨固定原EXE Go全套 PASS
bash workplace/new-game-356-formal-run.sh
python3 workplace/new-game-356-formal-verify.py
  10764共通正常列／35frames／原SS41→01與JMP／新word IMUL停止 PASS
  第三CMP只觀測，來源未取，不列規則數值驗收或byte1 reader
python3 workplace/new-game-356-source-verify.py
  CPU11行write／三observer及兩舊負例限定未知selector，逆轉保持355 PASS
bash workplace/new-game-356-off-run.sh
  明列source，關閉8M1693列／PNG與68舊＋32新CLI負例／正對照 PASS
python3 workplace/new-game-356-backlink-verify.py
python3 apps/moo2/tools/startup_probe_131.py --check-setcc-byte-memory-spec-backlinks
  93項回填／新356的37＋10負例與較早負例 PASS
```

DRAFT實際input-run內容保存input-original-run，重生入口改指向已保存兩步初態source；正式為三步診斷。未改舊LOG／PNG。原版兩次與乾淨全套各600s，8M／CLI與窄測180s；Docker固定Go1.24.13 image／network none／UID1000／原ZIP／patch唯讀。沒有CPU／正式原版／驗證失敗後挑選收據。原EXE／LOG／PNG／RAM與私有腳本留忽略workplace，公開自製CPU／測試／診斷／spec／索引／守衛與雜湊。收尾核對Git與Docker清理。

| 收據／核算 | SHA-256 |
| --- | --- |
| moo2-set-memory-356.go | a72124624383618cc50c2ddc143efb3a92f9c3dd91cbf4bcfde9cffb658c4f55 |
| moo2-probe-356-input.txt.gz | 1e0b74050f15e9c731e279ae1e5e2a843054c3fec207ad4a9f44fee118c0e9ff |
| moo2-vbe-356-input.png | 1f757f5b16fe492795accf198c8db851443a062450e7f37ecf465c7eae3b6457 |
| new-game-356-input-run.sh | 01a8b807e0352e271deb2fea3d1df934ebaed1d4009b054d5f48841bdfc5b4e4 |
| new-game-356-input-run-output.txt | f7301b988f77ef2699421a569df215039f427ebf0f15407529dacf210090a8a3 |
| new-game-356-input-verify.py | 0794e79dcb65e0eaf83670d3958795659c2c69b173d8e96658018095a3e1aefa |
| new-game-356-input-tests.txt | 18b79b7ebac6885c214c3e4eba31bb096ed89425639ec1815f2a539cf7829e2b |
| moo2-probe-356-formal.txt.gz | 386ac92b1570e1f1f7d4956752f2b87606b06e41286bc1b8c11bafb87df54978 |
| moo2-vbe-356-formal.png | 1f757f5b16fe492795accf198c8db851443a062450e7f37ecf465c7eae3b6457 |
| new-game-356-formal-run.sh | 1a50457aea027fa68f7e5f719cf666307f63e5a2abd1df1bc2871e02155e2243 |
| new-game-356-formal-run-output.txt | f504b66c022d757ff38d36745bdb8e4071b3dcae17b30d76c5b44e7027c0273b |
| new-game-356-formal-verify.py | 2d0fbc24323252b84a0f3fc08f17673ba6b490d3f545f80143ff3478307bf8aa |
| new-game-356-formal-tests.txt | b41f7d6f821ca08b4e80ee14ca206dc16f34a46c5ba2ed53d215396f4b8bd850 |
| moo2-356-cpu-narrow-tests.txt | 5b42c680568d34298cb482dbee7785fd851048dde122815a44fa479a5b90a230 |
| new-game-356-full-run.sh | 1a6259c525794811b4942979fd5ab35f6392c59f67f8b71d87fd9869fc4dd71a |
| full-test-356.txt | 6315adceb9f636c4a9628b2850c6d753eaa6b59166e142bc12a853577259887c |
| new-game-356-source-verify.py | 0e2508824574782a0366c163dcad94b924573edebb107f9995dc1bc1226cd818 |
| new-game-356-source-tests.txt | 8c40e750c0ed61e4b28952d8bb2b67f8a04bd7a5b56442de748af902eb0bb0bf |
| new-game-356-off-run.sh | cfea9c865f9d931f96f74600effa2704456e02c42ff677d5fc056602e8709b42 |
| new-game-356-off-cli-tests.txt | a5eabaf6f165680e4e73a808efc13d87da68f29a4c55ca63bd6bf4ec96c09dff |
| moo2-probe-356-off-old.txt | 86bf4539d4864642eabccec65d8891583827743d5921ffdde9a6ff86d2e834f9 |
| moo2-probe-356-off-new.txt | 8caee365e8cc587f929964766ed721317ffd1dadf77fe139a0c29b74c7ca6512 |
| moo2-vbe-356-off-old.png | dd4c21dd11e57ec86ef759182fd3ebadd6b53870b54de1ef192e2553820286db |
| moo2-vbe-356-off-new.png | dd4c21dd11e57ec86ef759182fd3ebadd6b53870b54de1ef192e2553820286db |
| new-game-356-cli-tests.txt | 0c39c7d30d26388473df3e31ad7be38651d38f98677857d9e5a9de59a4aaac43 |
| new-game-356-backlink-verify.py | ee1be0a913ee83f78185312dabb98495d9af18ee76f94d86fa71886fd30193bd |
| new-game-356-backlink-tests.txt | c74c882fe7c2ec2ebac2c325c34fb17ca76c3590ffd20f6eb882d6794aeb026c |
| moo2-set-memory-356-input.go | daac78a496a13350fbfe38961c1cfc9c7541049a05718b4ef9ac9359c5b1db11 |
| new-game-356-input-original-run.sh | 6d9362371bb4fa2701a5ee07e2481203f8fb419232268d3f6de9211e65d31ce4 |

## 2026-10-04 word立即值IMUL回填

原1CF90A word立即值IMUL與兩MOV零寫已由規格357接通，見[357](357-cpu386-imul-word-immediate.md)。原DS188:5A2084 word0000×5=0、EDI005AA5F4→005A0000／高word005A保持、定義CF／OF0與下一兩MOV dword0→0已驗；兩MOV不消費DI，undefined flags保存只屬工具近似。10767正常前綴／35frames／固定EXE全套保持，正式DI reader／原非零與overflow未知。原357的1CFD3F word NEG來源0000與原零值結果／下一分支已由358驗證；下一步依361回填帳捕捉180M旗色選單存檔錯誤的真正DOS呼叫／檔名與返回，審查既有隔離覆蓋層，沿同180M，不提高cap或深入helper。完整生成／開局、正式writer、RNG與remake同狀態未知，保留本檔原歷史定位與收據。

## 2026-10-04 word NEG回填

原1CFD3F word NEG與下一EB09已由規格358接通，見[358](358-cpu386-neg-word.md)。原DS188:5A207C word0000→0000、六定義flags246h與下一EB09到1CFD4E已驗；第三CMP只觀測flags246h→206h，SS:[EBP-564]來源未取，數值／NEG word reader不列驗收，第四JE按觀測ZF0不跳。10770正常前綴／35frames及固定EXE全套保持，晚期Bus第二byte失敗可部分寫但不發布flags。325舊66負例限定未知selector、268舊word register NEG負例加segment prefix，新全值域正例接合法word。歷史358於164610300在1D0944的word SUB memory目的拒絕，after1D0946只解碼、當時DS188:5AA6D1目的word未知／來源AX0，現已由359核對；下一步依361回填帳捕捉180M旗色選單存檔錯誤的真正DOS呼叫／檔名與返回，審查既有隔離覆蓋層，沿同180M，不提高cap或深入helper。原非零NEG／溢位與完整開局、資料語意／正式writer／RNG及remake同狀態未知，保留本檔原定位與收據。

## 359回填

原1D0944 word SUB與三POP及RET已由規格359接通，見[359](359-cpu386-sub-word-register-source.md)。原DS188:5AA6D1 word0003-AX0000=0003、六flags206h，SS188真正槽的EDX0000000E／ECX005AA5E8／EBX00000000與RET001D1E0B／ESP2BDB5C已驗，POP／RET不是目的word reader。10775正常前綴／35frames／固定EXE全套保持，既有ADD／其他SUB及十一舊測試不改，第二byte晚期部分寫不發布flags。歷史359於164984957在1D2A33的66 99 word CWD拒絕，after1D2A35，AX0001／DX0000，現已由360核對；下一步依361回填帳捕捉180M旗色選單存檔錯誤的真正DOS呼叫／檔名與返回，審查既有隔離覆蓋層，沿同180M，不提高cap或深入helper。原非零SUB來源／借位／溢位、目的word reader／欄位語意、正式writer／RNG／完整開局與remake同狀態未知，保留原歷史定位與收據。

## 360回填

原1D2A33 word CWD與下一SUB／SAR已由規格360接通，見[360](360-cpu386-cwd-word.md)。原AX0001／DX0000與CWD完整flags246h保持，真正SUB讀DX以1-0=1／六flags202h，SAR讀AX1→0與五定義flags／EIP1D2A3B已驗，AF不列原版parity。三步全部RAM／FPU原bits保持、10789正常前綴／35frames／固定EXE全套通過；裸CDQ／word SUB與SAR／十三舊測試不改。歷史360於168496272在2376CB拒絕C1 /1 memory ROR，當時DS188:270FC4來源未知／imm08、after2376CD未取disp／imm或source，現已由361核對；下一步依361回填帳捕捉180M旗色選單存檔錯誤的真正DOS呼叫／檔名與返回，審查既有隔離覆蓋層，沿同180M，不提高cap或深入helper。原負AX／EDX高word、新ROR來源與消費、正式writer／RNG／完整開局與remake同狀態未知，保留原歷史定位與收據。

## 361回填

原2376CB memory ROR與下一A1已由規格361接通，見[361](361-cpu386-ror-dword-memory-imm8.md)。原DS188:270FC4 dword000B1818 ROR8→18000B18、CF0與保持flags、三RAM差異270FC5／270FC6／270FC7、下一A1真正load到EAX18000B18已驗；count8 OF未定義，保留只驗工具模型。10949正常前綴／35frames與全套保持，十四舊測試不改，297舊memory負例限定未知DS並有完整新正例。沿同180M已無CPU拒絕，但終圖為旗色選單的Error saving game／Permission denied，完整開局未驗。下一步依361回填帳捕捉失敗DOS呼叫／檔名與返回，審查既有DirectoryOverlayFiles的隔離接線，不提高cap／代寫／重送／深入helper。原其他count／CF1／OF、存檔writer／內容、RNG與remake同狀態未知，保留原定位與收據。
