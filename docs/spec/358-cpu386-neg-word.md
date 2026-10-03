# 358：word暫存器與記憶體NEG

狀態：**CONFORMED，限定CPU契約／原零值NEG與EB09／正常續行至新停止**
日期：2026-10-04
範圍：cpu386的66 F7 /3、register／memory word目的；沿32位ModRM／SIB與DS／SS，不擴張段覆寫、67或F0／F2／F3。主庫玩法RE-first保持。

## 原阻塞與公開ISA

沿[357](357-cpu386-imul-word-immediate.md)，工具a7f175f3bfadcc6d9d56a78e23c0eabf2cbe2442／CPU SHA-256 94fab7c6e4389ce205c485dd498607f1442c20b3b7999212812f7e622fff4bf2。固定官方DOS1.31 ORION2.EXE SHA-256 4e11be14217b4aafa1839f333bf5eba037f98b0c44e9e4752c96c464c260419f。原164568139於dosgolem_high_le:1CFD3F的66 F7 5B 38 EB 09 8B 55 E4 29 C2 66 89 53 38 83拒絕word NEG，after1CFD42只解碼ModRM，未取disp8或source。原R=[0 0 4 5A2044 2BD920 2BDB54 0 0]／段=[8 188 188 0 20 188]／flags246h。DS188:[EBX+38h] offset5A207C來源word未知，後續EB09預期自然從1CFD43跳1CFD4E，下一83與後續分支待擷取。不猜資料欄位或helper用途。

[Intel 80386 NEG](https://www.ardent-tool.com/CPU/docs/Intel/386/manuals/prref386/NEG.htm)定義F7 /3 word為0減去來源再寫回word，CF僅零來源為0。[Intel SDM Volume 2B NEG 4-161](https://www.intel.cn/content/dam/www/public/cn/zh/documents/64-ia-32-architectures-software-developer-vol-2b-manual-cn.pdf)列OF／SF／ZF／AF／PF按減法結果定義。以16位結果模65536，OF只在原8000、SF是結果負、ZF是結果零、AF是低nibble借位、PF是低byte偶數個1。其他flags／全部R除register低word／六段／FPU保持；memory不改R，只目的兩bytes可改。沿[325](325-cpu386-neg-dword-memory.md)既有逐byte Bus模型，晚期第二byte寫失敗可已改第一byte但不發布flags，不宣稱任意Bus rollback或硬體exception restart。缺8088語料不當386實機驗收。

## 擷取、READY與驗收閘門

DRAFT保持CPU。同180M正常輸入首遇只讀observer最多四步，code24／完整R／六段／flags、DS:[EBX+37h]十二bytes與SS:[EBP-44]二十bytes，activationPeek及完整RAM雜湊驗只讀。每Step記完整RAM變更索引／callback／IRQ，錯誤即停，357全部舊列／36PNG保持。來源word、相鄰資料與下一原code可讀、公開ISA充分才READY，不先猜NEG source／結果。

READY新增operand16且group3分支：register取低word，或decodeAddress32／readSegment16；0-value的兩byte成功寫回後才sub16發布定義六旗標。register先保存source，sub16與低word寫回不破壞高16bits；既有F7 word TEST、F7 word MUL／IMUL、byte／dword NEG及其他群組不改。來源／地址與寫回全部須驗，prefix仍拒絕。325舊66 prefix負例改為未知selector的word memory拒絕，新完整正例接有效word，268舊word暫存器NEG拒絕案例改為帶segment prefix的拒絕，新全值域正例驗裸word NEG；其他prefix／截短不刪。

獨立oracle用整數modulo與signed範圍、nibble借位／低byte位元計數及little-endian視圖。memory全部65536 source×64初始算術flags，register全部八目的×65536 source×兩初態／高word哨兵，nonzero FPU／其他R與六段／鄰接memory保持；全ModRM／SIB、相異DS／SS、signed位移／繞回／last word，未知selector／唯讀／段外／線性溢位、每byte read及write失敗與部分寫模型、完整合法memory的截短定址／prefix、不放寬其他word F7群組。固定官方原EXE乾淨Go全套全過。

正式原四步按實際source及code獨立核算NEG六旗標、唯二目的byteRAM差異與下一EB09自然跳躍，下一比較／分支以實際擷取為準。若未擷取真正比較來源，不稱數值parity或新word reader。首遇前共通正常列／35frames保持，finalPNG按實際核對；固定1996日期不是seed，不增加180M、不跳指令／代寫或重送input。正常完整生成／開局、RNG／正式writer、資料語意與remake同狀態仍未知。

## 工具與入口

Go1.24.13 Docker image sha256:1a6d4452c65dea36aac2e2d606b01b4a029ec90cc1ae53890540ce6173ea77ac；原版與全套600s／2GiB／2CPU／128pids／UID1000／network none，原ZIP／patch唯讀。私有原EXE／LOG／PNG／RAM與腳本留忽略workplace，只公開自製CPU／測試／probe／spec／索引／守衛及雜湊。本檔同次加入000-index；原入口workplace/new-game-358-input-run.sh。主要原版執行器/home/anr2/cht/dosgolem的隔離副本workplace/dosgolem，能力／用法見README.md／CLAUDE.md。

## READY：原來源與消費範圍審查

DRAFT未改CPU，同180M正常輸入全部10840原357列／36PNG與readonly／RAM保持。input收據9c323583a0c88033b62fe9f27a73b3bf76ea15a51cc215ce8335ff613c144bfd；三observer逆轉逐byte保持a7f175f，CPU仍94fab7…。原164568139 DS188:5A207B十二bytes全00，真正來源word DS188:5A207C=0000，0-0000的word結果0000；定義CF0／OF0／SF0／ZF1／AF0／PF1，flags246h保持，R與六段／FPU保持，目的0→0不應有RAM差異。原callback12／12、IRQ41948／41948非active／非failed／pending0。source及公開ISA充分，審查DRAFT→READY。

原code24=66F75B38EB098B55E429C26689533883BDCCFDFFFF000F84；NEG長四byte預期EIP1CFD43，下一EB09預期自然跳1CFD4E。1CFD4E的83 BD CC FD FF FF 00是SS:[EBP-564] dword CMP imm8 0，offset2BD920不在擷取的SS:2BDB28二十bytes內；比較來源未知，第三CMP只觀測，第四0F84只按實際取得條件／code驗自然分支，不稱CMP數值驗收或NEG word reader。它會重定義算術flags，EB09本身不讀NEG flags，不開新的helper逆向或第二次DRAFT收據。

原SS188:2BDB28二十bytes=6901000001000000000000000800000000000000，與NEG來源分開。來源為零，原非零NEG／8000溢位／實際兩byte Bus write次數未驗，由公開ISA工程測試覆蓋而非升格原動態；現行實作允許同值仍逐byte寫回。兩舊拒絕案例改成明確未知selector word memory／segment prefix word register拒絕，不刪測試；新完整正例接有效形狀，其他F7與byte／dword NEG保持。

## 限定驗收：word NEG與原零值寫回／下一EB09

**已證實**：DRAFT未改CPU全部10840原357列／36PNG保持；來源word／相鄰資料與ISA充分才READY。正式首遇前10770共通正常列／35既有frames保持，原R／六段／flags／code24、DS十二byte與SS二十byte初態逐欄相同。

原164568139在1CFD3F的66 F7 5B 38，DS188:5A207C word0000→0000，0減0的完整六旗標CF0／OF0／SF0／ZF1／AF0／PF1，flags246h保持，EIP1CFD43。164568140原下一EB09自然跳1CFD4E，全部R／六段／flags／RAM保持。目的初值為零，原同值寫回Bus次數未取，不以無RAM差異代替原寫次數trace；工程測試另驗0仍兩write。原非零NEG／8000溢位未由這條原路徑驗證。

164568141的83 BD CC FD FF FF 00在1CFD4E，SS:[EBP-564] dword比較imm0，來源offset2BD920未在本次窗口，CMP數值不列驗收；只觀測flags246h→206h、R／段／RAM保持、EIP1CFD55。164568142的0F84 BC ED FF FF以觀測ZF0不跳，EIP1CFD5B，flags206h保持。第三CMP／第四JE不稱NEG word reader，JE只驗觀測旗標的條件；未重生原完整比較來源。四步readonly=true／error nil、ram_changes=[]，callback12／12、IRQ41948／41948非active／非failed／pending0。

**已證實，工程契約**：4194304個memory來源word×64初始六旗標、1048576個八register×65536來源×兩旗標案例，獨立整數modulo、signed邊界／nibble借位／低byte位元計數、little-endian低word視圖驗收。全memory ModRM／SIB與DS／SS相異來源、signed位移／繞回／最後word、唯讀／未知selector／段外／線性溢位、兩byte各read及write失敗／第二byte晚期部分寫模型、截短定址與合法目的prefix、nonzero FPU及相鄰RAM保持。register高word／非目的R與段／FPU、非算術flags保持；memory全部R保持。

CPU只新增獨立word group3分支，memory兩byte寫回成功後才sub16發布六flags；register保持高word。逆轉逐byte保持357，既有F7 TEST／MUL／IMUL、byte／dword NEG與七份舊測試不改。325舊66負例明確限定未知selector，268舊word NEG register負例明確加segment prefix；新全值域正例接合法word，兩修改可逆轉，不刪測試或其他prefix／截短。

窄測6.224s、固定官方原EXE乾淨Go全套CPU38666.221s／machine1.331s通過；缺8088語料不算386硬體驗收。關閉8M1693原列／PNG、68舊CLI＋32新180M負例與100M／120M／160M／180M正對照保持。CPU／原版／驗證無失敗後挑選收據。

CPU SHA-256 9840611ea0e3ae22ece69fd1f6f545dd08a316d1ed87247bbe061bf3f7f09522；新測試ab96fa39e050cca58f1f0b8c46275eeb0e3f3f56d9aa3210e9148f58a51839e3；兩舊測試NEG dword3ec49cd5357fdb65d416b92da4f13519155c0138572f8357a0ad68ccfaf60307／word IMUL9edb03bb303a26ec1f3437820c59d2d7f3cd558c80a308f8f9d7ec0cd5074010。正式probe07243e3f82540f6ffb9c99a3134e187b20dc27194593661f677587010df7633c；原正式收據a334a424881733be645038a99b2d7f36a724ae3a833a66caa77737f45905d25e；乾淨全套8a600babc81848f516e97569ee5098953c560ffa6cd5bc6e147278522f392020。

### 歷史358停止與unknown

原164610300於dosgolem_high_le input1D0944 bytes66 29 83 E9 00 00 00 5A 59 5B C3 53 51 52 56 57拒絕word SUB memory目的，after1D0946只解碼，未取ModRM／目的或source。R=[0 64 0 5AA5E8 2BDB4C 2BDBA0 5AA614 5AA5F4]／段=[8 188 188 0 20 188]／flags206h；DS188:[EBX+E9h] offset5AA6D1目的word未知，來源AX0000。尚未達180M；目的與原SUB／POP／RET已由359接通；下一步依361回填帳捕捉180M旗色選單存檔錯誤的真正DOS呼叫／檔名與返回，審查既有隔離覆蓋層，沿同180M，不提高cap或深入helper。

finalPNG逐byte保持357，SHA-256 1f757f5b16fe492795accf198c8db851443a062450e7f37ecf465c7eae3b6457／RGB04fef4b6a6d6c6c485ef1ce0a82ea71591956cdd3b7cd37b8d1082a023e20e17，沿354人工檢視仍主要黑底與小型方形圖形，未見完整地圖。固定1996日期不是seed；原非零NEG／8000溢位、CMP數值與NEG word reader、新SUB目的／POP／RET已見359、目的word reader與資料語意／正式writer、完整生成／開局與remake同狀態仍未驗，主庫RE-first保持。

### 回填帳與實際命令

不可變解析鍵：固定DOS1.31 EXE 4e11be14…＋dosgolem_high_le:1CFD3F＋66 F7 5B 38。原1CFD3F word NEG與下一EB09已由規格358接通；357及九份較早入口與325的scope／舊負例限制、現行unknown同次回填；000-index與--check-neg-word-spec-backlinks守衛限定原NEG／EB09與CMP限制，不把綠測試稱為完整原版parity。

以下在Docker與隔離工具workplace/dosgolem執行。DRAFT兩命令是CPUa7f175f時的歷史擷取，重生DRAFT須用該CPU與358只讀observer。
```text
bash workplace/new-game-358-input-run.sh
python3 workplace/new-game-358-input-verify.py
  未改CPU10840原357列／36PNG／word0000／原拒絕 PASS
go test -p 2 -buildvcs=false ./internal/cpu386 -run 'TestNEG|TestNeg|TestIMUL|TestMUL|TestF7|TestTEST' -count=1
  word NEG全值域／六flags／拒絕／高word／既有乘法與NEG 6.224s PASS
bash workplace/new-game-358-full-run.sh
  乾淨固定原EXE Go全套 PASS
bash workplace/new-game-358-formal-run.sh
python3 workplace/new-game-358-formal-verify.py
  10770正常前綴／35frames／原NEG零值與EB09／CMP限制／新SUB停止 PASS
python3 workplace/new-game-358-source-verify.py
  word NEG分支／三observer與兩舊負例明確限制逆轉保持357、七舊測試不變 PASS
bash workplace/new-game-358-off-run.sh
  8M1693列／PNG與68舊＋32新CLI負例／正對照 PASS
python3 workplace/new-game-358-backlink-verify.py
python3 apps/moo2/tools/startup_probe_131.py --check-neg-word-spec-backlinks
```

95項規格回填全過；新358的41項缺證據／限定範圍／狀態／357回填／索引負例、其餘十份較早回填另20及舊負例全過。來源／收據1000:1000，工具root-owned／.md目錄零。

| 本機來源／收據 | SHA-256 |
| --- | --- |
| moo2-neg-word-358.go | 07243e3f82540f6ffb9c99a3134e187b20dc27194593661f677587010df7633c |
| moo2-probe-358-input.txt.gz | 9c323583a0c88033b62fe9f27a73b3bf76ea15a51cc215ce8335ff613c144bfd |
| moo2-vbe-358-input.png | 1f757f5b16fe492795accf198c8db851443a062450e7f37ecf465c7eae3b6457 |
| new-game-358-input-run.sh | 3e04ede51d6ab24a39508e3d78c72e20d44acc3755e8a2d387a215519a6e0c50 |
| new-game-358-input-run-output.txt | 4ac677bbdf9f4a579fc88a9a38199682c5f558cf33d906c6146e92751541d891 |
| new-game-358-input-verify.py | b1690e006c5a1be106b15814caa75033fa344511bcfaa53b81a65cdb697cc09b |
| new-game-358-input-tests.txt | c4f76167e13434999fb6090b08e2fe8340f9378b157ea4377885d98196d7f5ed |
| moo2-probe-358-formal.txt.gz | a334a424881733be645038a99b2d7f36a724ae3a833a66caa77737f45905d25e |
| moo2-vbe-358-formal.png | 1f757f5b16fe492795accf198c8db851443a062450e7f37ecf465c7eae3b6457 |
| new-game-358-formal-run.sh | 2bf94975bfcca9d618b4e75ca47f21d130cfc9bc6ac30d46b0124bc480f1b8c4 |
| new-game-358-formal-run-output.txt | 4b7651bd0e452ae221567624329c0afbcd94f4c6b522563602e5246fa48065a4 |
| new-game-358-formal-verify.py | d9bc9b87429f5da9a665b26cdd9425d553bbfe5d04e637545206b2f1fc8fdb3f |
| new-game-358-formal-tests.txt | b3a36d75334bee537a87f4c1c7a532694a3cd50a1e20507395cabf0bc63027f0 |
| new-game-358-unit-tests.txt | b21e27933d33b6c56def7b143991c1672e18f73842a711888c0dbda3352e8be9 |
| new-game-358-full-run.sh | c57f83b67b2ea9fe12253ea75f31e6447fe8b870b56150a38be0a765eb0b2230 |
| full-test-358.txt | 8a600babc81848f516e97569ee5098953c560ffa6cd5bc6e147278522f392020 |
| new-game-358-source-verify.py | 5f6f4a6da59b40fca52395537c8d8e891496404e0d710fc01f17b08054b0507f |
| new-game-358-source-tests.txt | d4e7e1d223bb949d809985b4f0eb6a1f9698ca78668989da21cfba3c026ae99c |
| new-game-358-off-run.sh | 4d05336977f159fe680ba9387de60e646c6e53e783edc16ac034d9276cfcfcf8 |
| new-game-358-off-cli-tests.txt | a5eabaf6f165680e4e73a808efc13d87da68f29a4c55ca63bd6bf4ec96c09dff |
| moo2-probe-358-off-old.txt | 40043f24aceea509507d0aa217cb3e349783b4acaf4ab5bf4fd00237b3631c96 |
| moo2-probe-358-off-new.txt | 27d1b0979f14131c1dd33cc8d15b959f72aee09e6ddd77d3ec0122432865876a |
| moo2-vbe-358-off-old.png | dd4c21dd11e57ec86ef759182fd3ebadd6b53870b54de1ef192e2553820286db |
| moo2-vbe-358-off-new.png | dd4c21dd11e57ec86ef759182fd3ebadd6b53870b54de1ef192e2553820286db |
| new-game-358-cli-tests.txt | 0c39c7d30d26388473df3e31ad7be38651d38f98677857d9e5a9de59a4aaac43 |
| new-game-358-backlink-verify.py | aca396e00133a792a03645020d61830f4f15b5e30c3883b681639ca57cb137ce |
| new-game-358-backlink-tests.txt | 460a5e9c5b2a48db1f4dc21138eefdf436c303f1b34f787c4a10c170f6b329b9 |

## 359回填

原1D0944 word SUB與三POP及RET已由規格359接通，見[359](359-cpu386-sub-word-register-source.md)。原DS188:5AA6D1 word0003-AX0000=0003、六flags206h，SS188真正槽的EDX0000000E／ECX005AA5E8／EBX00000000與RET001D1E0B／ESP2BDB5C已驗，POP／RET不是目的word reader。10775正常前綴／35frames／固定EXE全套保持，既有ADD／其他SUB及十一舊測試不改，第二byte晚期部分寫不發布flags。歷史359於164984957在1D2A33的66 99 word CWD拒絕，after1D2A35，AX0001／DX0000，現已由360核對；下一步依361回填帳捕捉180M旗色選單存檔錯誤的真正DOS呼叫／檔名與返回，審查既有隔離覆蓋層，沿同180M，不提高cap或深入helper。原非零SUB來源／借位／溢位、目的word reader／欄位語意、正式writer／RNG／完整開局與remake同狀態未知，保留原歷史定位與收據。

## 360回填

原1D2A33 word CWD與下一SUB／SAR已由規格360接通，見[360](360-cpu386-cwd-word.md)。原AX0001／DX0000與CWD完整flags246h保持，真正SUB讀DX以1-0=1／六flags202h，SAR讀AX1→0與五定義flags／EIP1D2A3B已驗，AF不列原版parity。三步全部RAM／FPU原bits保持、10789正常前綴／35frames／固定EXE全套通過；裸CDQ／word SUB與SAR／十三舊測試不改。歷史360於168496272在2376CB拒絕C1 /1 memory ROR，當時DS188:270FC4來源未知／imm08、after2376CD未取disp／imm或source，現已由361核對；下一步依361回填帳捕捉180M旗色選單存檔錯誤的真正DOS呼叫／檔名與返回，審查既有隔離覆蓋層，沿同180M，不提高cap或深入helper。原負AX／EDX高word、新ROR來源與消費、正式writer／RNG／完整開局與remake同狀態未知，保留原歷史定位與收據。

## 361回填

原2376CB memory ROR與下一A1已由規格361接通，見[361](361-cpu386-ror-dword-memory-imm8.md)。原DS188:270FC4 dword000B1818 ROR8→18000B18、CF0與保持flags、三RAM差異270FC5／270FC6／270FC7、下一A1真正load到EAX18000B18已驗；count8 OF未定義，保留只驗工具模型。10949正常前綴／35frames與全套保持，十四舊測試不改，297舊memory負例限定未知DS並有完整新正例。沿同180M已無CPU拒絕，但終圖為旗色選單的Error saving game／Permission denied，完整開局未驗。呼叫／檔名與返回已由362定位；可寫試作80M前置仍DRAFT，下一步見362，不提高cap／代寫／重送／深入helper。原其他count／CF1／OF、存檔writer／內容、RNG與remake同狀態未知，保留原定位與收據。

## 362存檔拒絕回填

原237024 SAVE10.GAM唯讀拒絕已由規格362定位；可寫正常路徑仍DRAFT，見[362](362-moo2-save-permission-boundary.md)。原165025480的INT21／3D01／DS188:2BDB68／SAVE10.GAM回AX5／CF1及RAM保持已取，拒絕源於唯讀provider無WriteFileProvider。先前呼叫與檔名未知已解；尚不稱原存檔成功。隔離overlay試跑改變前段流程，在80M完整表閘門停止，設定頁RGB相同而record11–15的+44四byte窗口各增8000h，欄位與消費未知；可寫試作只留本機，不接公開玩家path，不改原336 guard或點擊時刻。下一步依362有界讀初段開檔與這五窗口候選值所指內容，再審查正常輸入，完整開局／RNG與remake同狀態未驗。
