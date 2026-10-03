# 360：word CWD符號延伸

狀態：**CONFORMED，限定CPU word CWD與原SUB／SAR消費，非完整開局**
日期：2026-10-04
範圍：cpu386的66 99、AX符號延伸至DX，保留EDX高word／EAX／其他R／完整flags／六段／FPU／RAM；不擴張其他prefix，既有裸99 CDQ逐byte保持。主庫RE-first保持。

## 原阻塞與公開契約

沿[359](359-cpu386-sub-word-register-source.md)，工具11d9aad0d10bcf51ac75f9ee23611acfe2fd7e9b／CPU SHA-256 995f059949b7caac9618ab8b2513999b64b4cb2c928bd608da96eff171d3a8d1。固定官方DOS1.31 EXE SHA-256 4e11be14217b4aafa1839f333bf5eba037f98b0c44e9e4752c96c464c260419f。原164984957於dosgolem_high_le:1D2A33 bytes66 99 66 2B C2 66 D1 F8 98 01 C7 81 FF FF 7F 00拒絕word CWD，after1D2A35只取prefix／opcode。R=[1 0 0 5A2EED 2BDB24 2BDB58 5A2EED A]／段=[8 188 188 0 20 188]／flags246h；AX0001／DX0000，不猜資料欄位或helper語意。

[Intel 80386 CWD/CDQ](https://www.ardent-tool.com/CPU/docs/Intel/386/manuals/prref386/CWD.htm)定義word AX符號為負時DX=FFFF，否則DX=0000，沒有flags改變；高word不由16位目的寫入。裸99原CDQ保持。下一66 2B C2是既有word SUB AX,DX，第三66 D1 F8是既有單位SAR AX；依[Intel SUB](https://www.ardent-tool.com/CPU/docs/Intel/386/manuals/prref386/SUB.htm)與[SAR](https://www.ardent-tool.com/CPU/docs/Intel/386/manuals/prref386/SAL.htm)獨立核對來源、結果與定義flags。SAR只驗CF／OF／SF／ZF／PF，AF工具清除不列原版parity。

## DRAFT擷取與READY

CPU未修改，同180M正常輸入，在1D2A33最多三步只讀診斷code16／完整R／六段／flags／FPU控制與狀態／depth及八stack原bits、全部RAM差異／callback／IRQ。activationPeek與RAM雜湊驗只讀，錯誤即停；359全部舊列／36PNG保持。原AX／DX／高word與完整初態充分才READY，不代寫、跳指令、重送、增加cap或深入helper。

READY只移除operand16的99早拒絕，加獨立CWD分支後break，裸CDQ原一行不改。工程獨立oracle以signed整數範圍與little-endian word視圖驗65536個AX、正負EAX高word與非零EDX高word／原低word哨兵、完整64算術flags初態及非算術flags、全R／段／FPU／RAM保持；截短每byte與完整合法來源的段prefix／67／F0／F2／F3拒絕。舊CDQ與word SUB／SAR保持性核對，固定原EXE乾淨Go全套與關閉8M／CLI必須通過。

原最多三步正式核對CWD DX寫入／EAX與全flags保持、下一真正SUB讀DX低word與六flags、第三SAR讀SUB低word與五定義flags／高word保持；全部RAM與FPU保持。原負數AX與EDX非零高word若未命中，不宣稱原動態已驗。固定1996日期不是seed，完整開局與remake同狀態仍未知。

## 工具與入口

主要執行器/home/anr2/cht/dosgolem隔離副本workplace/dosgolem，用法／支援範圍見README.md／CLAUDE.md；本檔同次加入000-index。歷史原輸入入口workplace/new-game-360-input-run.sh，Go1.24.13 image sha256:1a6d4452c65dea36aac2e2d606b01b4a029ec90cc1ae53890540ce6173ea77ac，600s／2GiB／2CPU／128pids／UID1000／network none，原ZIP／patch唯讀。原EXE／LOG／PNG／RAM與私有腳本留忽略workplace，只公開自製CPU／測試／probe／spec／索引／守衛與雜湊。

## READY：未改CPU的原三步初態

READY於2026-10-04。原輸入收據SHA-256 4143ae1c60ec3957ca373d4ac1be397ec9932daaa384ad997ab07f2b3820cd99；全部10859原359列／36PNG保持，CPU仍995f0599，三只讀observer逆轉為11d9aad原probe。首遇AX0001／DX0000，全部R=[1 0 0 5A2EED 2BDB24 2BDB58 5A2EED A]／段=[8 188 188 0 20 188]／flags246h、FPU127F／0／depth0／八stack bits0，原拒絕前後全部RAM保持。callback12／12／IRQ42073／42073非active／非failed／pending0，原CWD尚未執行。

原輸入與ISA充分，CWD應DX0000→0000、完整flags246h與其他狀態保持，EIP1D2A35；下一SUB AX,DX以1-0=1，六定義flags202h、EIP1D2A38；第三SAR AX,1以1→0，CF1／OF0／SF0／ZF1／PF1、EIP1D2A3B，AF不列原版parity，工具AF0。只實作99 word，SUB／SAR原分支與helper逐byte保持。這是工具CPU READY，主庫玩法RE-first不變。

## 限定驗收：word CWD與原SUB／SAR消費

**已證實**：DRAFT未改CPU全部10859原359列／36PNG保持。正式首遇前10789共通正常列／35既有frames保持，原同一AX／DX／完整R／段／flags／FPU原bits與code16逐欄相同。

原164984957在1D2A33執行66 99，AX0001符號正、DX0000→0000，完整flags246h與EAX／其他R／六段／FPU／RAM保持，EIP1D2A35。原164984958的66 2B C2讀真正DX低word，SUB AX0001-DX0000=0001，六定義flags CF0／OF0／SF0／ZF0／AF0／PF0、flags202h、EIP1D2A38，全部R／段／FPU／RAM保持。原164984959的66 D1 F8消費AX0001，SAR AX0001→0000、CF1／OF0／SF0／ZF1／PF1，EIP1D2A3B／EAX00000000；其餘R／段／FPU／RAM保持。SAR AF不列原版parity，只觀測工具完整flags202h→247h，不把AF工具清除稱原版定義。

三步FPU控制127F／status0／depth0／八stack bits0不改，ram_changes=[]／readonly=true／error nil／全部RAM保持，callback12／12、IRQ42073／42073非active／非failed／pending0。原AX正／DX原已0，此路徑不證明原負AX、舊DX非零寫回或EDX非零高word；工程另驗這些形狀，不冒充原動態逐值對拍。

**已證實，工程契約**：16777216個65536 AX×64六算術flags初態×四EAX／EDX高word哨兵，AX與EAX高位符號相反也依AX；獨立signed數值範圍與little-endian低word視圖，不複製CPU的shift／mask公式。另全部32個flags位元×16邊界AX、截短每byte／合法來源的段prefix／67／F0／F2／F3拒絕、裸CDQ與十組CWD→SUB→SAR正負奇偶來源保持性驗證。EAX／EDX高word、其餘R／六段／全部flags／nonzero FPU與完整RAM保持；純暫存器CWD不取記憶體操作數，不虛造memory目的測試。

CPU只移除99的operand16早拒絕並加五行CWD分支，裸CDQ原一行、word SUB／SAR與flags helper逐byte保持。逆轉為359，三observer逆轉為11d9aad原probe，十三份舊測試完全保持。窄測0.828s、固定官方原EXE乾淨Go全套CPU38699.385s／machine1.679s通過，缺8088語料不算386硬體驗收。關閉8M1693原列／PNG另比上一輪359原收據保持，68舊CLI＋32新180M負例與100M／120M／160M／180M正對照通過。本輪CPU／原版／驗證未失敗後挑選guest收據。

CPU SHA-256 ed94eaf7e9c363e8a2c89d5410b30a9e653a60532d31654ee91c14d042f75337；新測試bad93e84a75a9e4a7e77c74e904a10a74ed23e82c0919b0ae006581c28b72181；正式probe8d65d37030f58fb8fd034b2396d9da00c3617987211e750da517cdc8b3c1bd9c。原正式收據a05e0f46fe83e5fb048112cef174471599185d7a1c20f11d3a623565ddb156e3；乾淨全套7a638098bb95b91ee21387d09949d76e590ba9225be9eb0a5cbf07eba236936f。

### 歷史360停止與unknown

原168496272於dosgolem_high_le input2376CB bytesC1 0D C4 0F 27 00 08 A1 C4 0F 27 00 4E 74 46 25拒絕memory dword ROR，after2376CD只取opcode／ModRM，未取disp32／imm8／source。R=[18181818 0 110 5C 2BD5D8 2BD648 69 34B94C]／段=[8 188 188 0 20 188]／flags202h。指令目的DS188:270FC4、imm08，目的原dword未知；不能以當時EAX18181818猜目的來源。尚未達180M；原ROR與A1已由361接通；下一步依361回填帳核對存檔權限錯誤，沿同180M，不增加cap／跳指令／代寫或重送／深入helper。

finalPNG逐byte保持359，SHA-256 1f757f5b16fe492795accf198c8db851443a062450e7f37ecf465c7eae3b6457／RGB04fef4b6a6d6c6c485ef1ce0a82ea71591956cdd3b7cd37b8d1082a023e20e17，沿354人工檢視主要黑底與小型方形圖形，未見完整地圖。固定1996日期不是seed；原負AX／EDX高word、原ROR與A1已見361、存檔呼叫／檔名與正式writer／RNG、完整生成／開局與remake同狀態仍未驗，主庫RE-first保持。

### 回填帳與實際命令

不可變解析鍵：固定DOS1.31 EXE 4e11be14…＋dosgolem_high_le:1D2A33＋66 99。原1D2A33 word CWD與下一SUB／SAR已由規格360接通；359及十二份較早入口同次回填，保留歷史CWD定位與原收據。000-index與--check-cwd-word-spec-backlinks守衛限定CWD與真正消費，不稱完整原版parity。

以下在Docker與隔離工具workplace/dosgolem執行；DRAFT擷取與輸入驗證是CPU 11d9aad時的歷史執行，CPU未改時先通過。
```text
bash workplace/new-game-360-input-run.sh
python3 workplace/new-game-360-input-verify.py
  未改CPU10859原359列／36PNG／原AX及完整初態 PASS
go test -p 2 -buildvcs=false ./internal/cpu386 -run 'TestCWDWord|TestCDQSignExtension' -count=1
  0.828s PASS
bash workplace/new-game-360-full-run.sh
  固定官方原EXE乾淨Go全套 PASS
bash workplace/new-game-360-formal-run.sh
python3 workplace/new-game-360-formal-verify.py
  10789正常前綴／35frames／原CWD與SUB／SAR／新ROR停止 PASS
python3 workplace/new-game-360-source-verify.py
  限定CWD與三observer逆轉359，十三舊測試保持 PASS
bash workplace/new-game-360-off-run.sh
  8M1693原列／PNG、前輪359原收據與CLI負例／正對照 PASS
python3 workplace/new-game-360-backlink-verify.py
python3 apps/moo2/tools/startup_probe_131.py --check-cwd-word-spec-backlinks
```

### 本機收據雜湊

原LOG／PNG／RAM與腳本留本機忽略workplace，只公開以下雜湊。

| 本機檔名 | SHA-256 |
|---|---|
| moo2-cwd-word-360.go | 8d65d37030f58fb8fd034b2396d9da00c3617987211e750da517cdc8b3c1bd9c |
| moo2-probe-360-input.txt.gz | 4143ae1c60ec3957ca373d4ac1be397ec9932daaa384ad997ab07f2b3820cd99 |
| moo2-vbe-360-input.png | 1f757f5b16fe492795accf198c8db851443a062450e7f37ecf465c7eae3b6457 |
| new-game-360-input-run.sh | 26e87efec02f945cd3b7f5fe59f5782f8a3106e2251b426ac082b93994686eae |
| new-game-360-input-run-output.txt | bf41f1ab66bfc8d33bb80e36994128fd89aab7928e5e88c89af923bcff1927d2 |
| new-game-360-input-verify.py | c261f469f17d48aefa68c1253d8f221037cff2180638248b6f89e47a3f8d6990 |
| new-game-360-input-tests.txt | c1a14e9f83aab4afbe67045390dd2a317758fe6b2db3535853378fc2c488588e |
| moo2-probe-360-formal.txt.gz | a05e0f46fe83e5fb048112cef174471599185d7a1c20f11d3a623565ddb156e3 |
| moo2-vbe-360-formal.png | 1f757f5b16fe492795accf198c8db851443a062450e7f37ecf465c7eae3b6457 |
| new-game-360-formal-run.sh | cef6c7cdf5dab4880504c41409153d8604f9a5caaa6360b4500fb54a9c69fcac |
| new-game-360-formal-run-output.txt | ceac0b93c9af0ec069eb7063359f029b6e129ed430b6062b0672afd39f187178 |
| new-game-360-formal-verify.py | 0aa89c540754008531d111fba662e1a0a8d3af3898980942f56c4f46fd3b743d |
| new-game-360-formal-tests.txt | d8696c2be76aa43b67e8003b078cb92dbb858d027b762cd4ee5a83426b12b90a |
| new-game-360-unit-tests.txt | dc141c466dd16697abf74895e64e160216a95629cabec5296b4fedb500ccbe89 |
| new-game-360-full-run.sh | 6a790107b572abfa549210758fcea49907a2e0c281f6466df0b9aa706a9659dd |
| full-test-360.txt | 7a638098bb95b91ee21387d09949d76e590ba9225be9eb0a5cbf07eba236936f |
| new-game-360-source-verify.py | 6127647b58f12bb3680f3f87e38f5dd7a941335a84ea83d42a2cbaec5cc29e5e |
| new-game-360-source-tests.txt | bf3d0a64c1d6c6b50911a3241c276a668ad1ae4e38a5ad0385bc853124d0a200 |
| new-game-360-off-run.sh | d46319f88606f40c6ad16affc6346146380fdfd0ac090cd31bcc447282228ce5 |
| new-game-360-off-cli-tests.txt | a5eabaf6f165680e4e73a808efc13d87da68f29a4c55ca63bd6bf4ec96c09dff |
| moo2-probe-360-off-old.txt | c0b9bdf2b2e95878b6d6a34053f9da63395e752b3925ab774ea2a207acaa5726 |
| moo2-probe-360-off-new.txt | 67c25bdf682de6e28276dd5437be8f412d70deed8ec3c0b6ac5a495bb29b0aff |
| moo2-vbe-360-off-old.png | dd4c21dd11e57ec86ef759182fd3ebadd6b53870b54de1ef192e2553820286db |
| moo2-vbe-360-off-new.png | dd4c21dd11e57ec86ef759182fd3ebadd6b53870b54de1ef192e2553820286db |
| new-game-360-cli-tests.txt | 0c39c7d30d26388473df3e31ad7be38651d38f98677857d9e5a9de59a4aaac43 |
| new-game-360-backlink-verify.py | a12abc12f0390921488c2be3a32f5e508daf528461918e95df6fe349c650edd4 |
| new-game-360-backlink-tests.txt | 2f15caa9dd2a0509fc267e29461e2bf074c6446ca62cc43a8bc5325cb746f390 |

## 361回填

原2376CB memory ROR與下一A1已由規格361接通，見[361](361-cpu386-ror-dword-memory-imm8.md)。原DS188:270FC4 dword000B1818 ROR8→18000B18、CF0與保持flags、三RAM差異270FC5／270FC6／270FC7、下一A1真正load到EAX18000B18已驗；count8 OF未定義，保留只驗工具模型。10949正常前綴／35frames與全套保持，十四舊測試不改，297舊memory負例限定未知DS並有完整新正例。沿同180M已無CPU拒絕，但終圖為旗色選單的Error saving game／Permission denied，完整開局未驗。呼叫／檔名與返回已由362定位；可寫試作80M前置仍DRAFT，下一步見362，不提高cap／代寫／重送／深入helper。原其他count／CF1／OF、存檔writer／內容、RNG與remake同狀態未知，保留原定位與收據。

## 362存檔拒絕回填

原237024 SAVE10.GAM唯讀拒絕已由規格362定位；可寫正常路徑仍DRAFT，見[362](362-moo2-save-permission-boundary.md)。原165025480的INT21／3D01／DS188:2BDB68／SAVE10.GAM回AX5／CF1及RAM保持已取，拒絕源於唯讀provider無WriteFileProvider。先前呼叫與檔名未知已解；尚不稱原存檔成功。隔離overlay試跑改變前段流程，在80M完整表閘門停止，設定頁RGB相同而record11–15的+44四byte窗口各增8000h，欄位與消費未知；可寫試作只留本機，不接公開玩家path，不改原336 guard或點擊時刻。下一步依362有界讀初段開檔與這五窗口候選值所指內容，再審查正常輸入，完整開局／RNG與remake同狀態未驗。

## 363前段差異與資料窗口回填

原SOUND3D02與五個+44窗口前128bytes已由規格363核對；可寫玩家路徑仍DRAFT，見[363](363-moo2-overlay-startup-and-setup-source.md)。原1192795／237024的SOUND.LBX讀寫開檔，兩側同原初態但唯讀AX5 CF1／overlay真handle5 CF0。80M五候選值各增8000h，指向的前128byte及RGB相同；兩側各27PNG／各自舊列保持，原418來源檔未變，state僅有內容未變的sound.lbx副本。之前「所指內容未取」已限定解出開頭128bytes，完整物件／角色／原指標消費仍未知。下一步依363另立DRAFT可寫profile正常ACCEPT前置，不忽略位址／調時刻／改舊336唯讀guard。完整存檔／音訊／RNG與remake同狀態未驗。
