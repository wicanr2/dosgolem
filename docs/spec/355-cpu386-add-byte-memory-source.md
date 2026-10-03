# 355：byte記憶體來源ADD

狀態：**CONFORMED，限定CPU契約／原五步零值／正常續行至新停止**
日期：2026-10-03
範圍：cpu386的02 /r ADD r8,r/m8 memory來源，沿32位ModRM／SIB與DS／SS；原02／22 register保持，22 memory、operand16／段覆寫／67／F0／F2／F3不擴張。

## 原阻塞與公開契約

沿[354](354-cpu386-xchg-byte-memory-register.md)，工具1f155175b2c77e6ee133ef609f43b758d7e0eed8／CPU SHA-256 3b4f3dc4e3054bd92ce252f54202414c47dcc501257be7d0cf538c02ea449132。官方DOS1.31 ORION2.EXE SHA-256 4e11be14217b4aafa1839f333bf5eba037f98b0c44e9e4752c96c464c260419f。164560803在dosgolem_high_le:1CDD0F原02 45 F8拒絕，後續02 45 E4／02 45 FC／02 45 E0／00 43 07。AL0、SS188:EBP2BDB44，四memory來源offset2BDB3C／2BDB28／2BDB40／2BDB24未知；第五步DS188:EBX5AA5E8+7目的一byte未知。不猜資料欄位或helper用途。

[Intel 80386原廠ADD](https://www.ardent-tool.com/CPU/docs/Intel/386/manuals/prref386/ADD.htm)列02 /r為ADD r8,r/m8；結果寫第一運算元，更新CF／PF／AF／ZF／SF／OF。memory來源只讀，其他旗標與未選register byte保持。工具錯誤可已解碼前進EIP，不聲稱硬體exception restart。

## 最小實作與驗收

DRAFT保持CPU，用同180M正常輸入只觀察首遇五原步。保存完整R／段／flags／code16、SS:[EBP-33]的34bytes與DS:[EBX+6]的3bytes；peek以activationPeek與完整RAM雜湊證只讀。每Step記錄完整RAM變更索引、callback／IRQ與錯誤；原拒絕仍停，全部354舊列／PNG保持。原byte可讀並驗舊前綴後才READY。

READY後通用02 memory先保存舊reg8並decodeAddress32／readSegment8成功，才發布byte結果與六算術旗標，source沒有write。覆蓋全部八byte暫存器、ModRM／SIB與相異DS／SS、base／index和AL／AH／CL／CH別名、負位移／繞回／最後byte。未知selector／越界／線性溢位／Bus讀失敗／prefix／截短不發布R／段／flags／FPU／RAM；唯讀來源必須成功，寫入拒絕Bus也不得被呼叫。

獨立oracle用較寬unsigned和、低四位進位、有號範圍、popcount驗全部256×256與八register；完整未選狀態／非零FPU保持。固定原EXE乾淨Go全套必須全過；缺8088語料不當硬體驗收，沒有386實機資料。

正式沿同180M正常輸入，不跳呼叫、不重送點擊、不改cap；驗首四ADD的原來源、AL累加／六旗標與RAM保持，再驗既有00目的ADD的唯一byte寫回。首遇前可比列／既有frames保持，finalPNG依較晚實際狀態核對。日期1996不是RNG seed，完整生成／開局、remake同狀態、資料語意未知，主庫RE-first保持。

## 工具與入口

Go1.24.13 Docker image sha256:1a6d4452c65dea36aac2e2d606b01b4a029ec90cc1ae53890540ce6173ea77ac，UID1000／network none／2GiB／2CPU／128pids／600s，原ZIP與patch唯讀。原EXE／LOG／PNG／RAM與私有腳本留忽略workplace，公開自製CPU／測試／診斷／規格；本檔同次加入000-index。原版主要執行器是/home/anr2/cht/dosgolem的隔離branch副本workplace/dosgolem，能力與用法見其README.md／CLAUDE.md；本切片入口workplace/new-game-355-input-run.sh。

## 未改CPU來源與READY審查

DRAFT未改CPU正常180M重播，全部10829原354列／36PNG保持；三observer逆轉逐byte保持1f15517的probe，CPU仍3b4f3dc4…。輸入收據SHA-256 0c301fbf7d191d27e123b5a0ce826460f88e3bf9d711c2a1046eb14d8d17e443；python3 workplace/new-game-355-input-verify.py PASS。

164560803原1CDD0F，R=[0 5A2044 5AA044 5AA5E8 2BDB18 2BDB44 2 0]／段=[8 188 188 0 20 188]／flags202h。SS188:2BDB23的34bytes前33皆00、最後A0；四來源2BDB3C／2BDB28／2BDB40／2BDB24皆00，AL0。DS188:5AA5EE三bytes000000，第五目的5AA5EF為00。readonly=true／完整RAM保持；callback12／12、IRQ41945／41945非活動、pending0，原拒絕仍after1CDD11。peek不改CPU／FPU，舊終態保持。

原來源可讀且ISA充分，355審查READY。原五步預期AL保持00；首步flags202h→246h，後四246h保持。四02來源只讀，第五既有00 ADD目的一byte以00加00寫回00，全部RAM保持；依序EIP1CDD12／1CDD15／1CDD18／1CDD1B／1CDD1E。零值原切片只能驗解碼、零結果／ZF／PF與自然續行，不宣稱原非零寫回／進位已驗。自製窮舉測試另驗非零／CF／AF／OF，不注入guest資料挑結果。

## 限定驗收：byte記憶體來源ADD與原五步

355限定CONFORMED只涵蓋通用02 memory CPU契約、原五步零值消費與正常續行至新停止。完整生成／開局與remake同狀態仍未驗，主庫RE-first保持。無原版資料代寫／跳指令／重送輸入／增加180M。

**已證實，原五步**：164560803..164560806於dosgolem_high_le:1CDD0F／1CDD12／1CDD15／1CDD18的02 45 F8／E4／FC／E0，SS188:2BDB3C／2BDB28／2BDB40／2BDB24四byte皆00，加到AL00仍00。首flags202h→246h，後三246h保持；EIP依序1CDD12／1CDD15／1CDD18／1CDD1B。164560807原1CDD1B的00 43 07，以AL00加到DS188:5AA5EF byte00，結果00、EIP1CDD1E／flags246h。全部R／六段／source窗口與目的三byte保持，完整RAM差異皆空、readonly=true／error nil，callback12／12、IRQ41945／41945非活動、pending0。原零結果不是原非零加法／進位證據，沒有Bus寫次數trace，不把00→00冒稱非零寫回。

首遇前10759共通正常列／35既有frames與同一原R／四來源byte保持；DRAFT未改CPU全部10829原354列／36PNG保持。正式finalPNG仍1f757f5b16fe492795accf198c8db851443a062450e7f37ecf465c7eae3b6457／RGB04fef4b6a6d6c6c485ef1ce0a82ea71591956cdd3b7cd37b8d1082a023e20e17，逐byte與354相同，沿354人工檢視主要黑底與小型方形圖形，未見完整地圖。PNG保持不代表正常開局完成。

**已證實，新停止**：164561579於dosgolem_high_le input1CE387 bytes0F 94 45 F4 E9 8E 02 00 00 83 EF 04 F6 47 01 02拒絕「0F 94 ModRM 45 尚未支援」。after1CE38A只解碼，memory目的SS188:[EBP-12] offset2BD834 byte未知，原未執行。R=[5AA5F4 0 0 256 2BD36C 2BD840 5AA5E8 5AA614]／段=[8 188 188 0 20 188]／flags246h，ZF1。requested budget180000000、actual stop164561579尚未達180M；probe exit0只是錯誤收尾。資料語意、後續消費與完整開局未知。

**已證實，CPU回歸**：八register全部256×256及兩種算術flags初態、64flags組合，以規格324獨立較寬加法／nibble進位／signed範圍／popcount與規格354獨立little-endian byte視圖驗證，未呼叫CPU add8。所有ModRM／SIB、相異DS／SS、AL／AH等base／index別名、負disp8／32位繞回／最後byte／相鄰資料及非零FPU保持。唯讀memory成功且Bus零寫；未知selector／段外／線性溢位／Bus讀失敗／prefix／截短不發布register／flags／RAM；原02／22 register的64配對各保持，22 memory仍拒絕。

窄測go test -p 2 -buildvcs=false ./internal/cpu386 -run 'TestADDByteSource' -count=1 -v PASS，4.089s；固定原EXE乾淨Go全套PASS，CPU386123.004s／machine2.959s。git ls-files -z建乾淨/tmp/test-src，複製新測試，不納入歷史探索main或並行暫存probe；缺8088語料不算硬體驗收，沒有386實機資料。關閉8M1693原列／PNG、68舊CLI＋32新180M負例與100M／120M／160M／180M正對照保持。

CPU16行02 memory分支替換1行拒絕，三observer區塊逆轉逐byte保持354；22 memory拒絕、原register、平台／8088 CPU不改。CPU SHA-256 125674de469ef82d4abe457190e28eb0c75b88aca8a876349b395165e77c79e1，自製測試3ccaf15df8f8ee1163465e7b41ae862ff26352f92a8f477fa554ba87e131c2c1，probe c872e72ed84611c4bb98fa8d9958e8e0036eafed28663d1172f209edf2666d54，正式原收據58c0834c577fe6a8a32b64ad82567f231e3ae56b314ce21a616d506475bb4454，Go全套336f8b09bdd31fba5da14c5b72a2fbc594cf0729f50b87953f548b9ff062467d。

8M首次驗證腳本以套件目錄建置，誤納進行中formal的probe355_formal.go，重複main拒絕，未執行8M。已保存attempt1腳本／輸出，改明列main.go／irq1_prototype.go／irq7_prototype.go，同Docker／同8M與CLI命令重跑通過。這是共享暫存檔造成的建置輸入問題，不是CPU或產品缺陷；formal及全套收據沒有換選。

### 回填帳與下一步

| 不可變鍵 | 已證實語意 | 較早規格 | 必須回填 |
| --- | --- | --- | --- |
| DOS1.31／EXE4e11be14…／dosgolem_high_le:1CDD0F→1CDD1E | 四SS byte來源00的ADD AL與第五目的ADD零結果／flags／RAM及正常續行 | 354、353、352 | 原1CDD0F byte記憶體來源ADD與五步零值消費已由規格355接通 |

下一步只接原1CE387的0F 94 memory目的；先捕捉SS188:2BD834 byte與相鄰資料、flags246h及後續原消費，核對既有標準16條件ISA，再審查通用memory byte目的與寫回失敗不發布。沿同180M正常輸入，不增加cap或深挖helper。原非零ADD／進位、完整生成／開局、正式writer、RNG與remake同狀態仍未知。

### 本機忽略證據索引與命令

92項規格回填正對照、新355的32缺證據／限定範圍／狀態／354回填／索引負例、353與352另4負例及較早負例通過。所有來源／收據1000:1000，工具root-owned／.md目錄零。

```text
bash workplace/new-game-355-input-run.sh
python3 workplace/new-game-355-input-verify.py
  未改CPU10829原354列／36PNG／四00來源與原拒絕 PASS
go test -p 2 -buildvcs=false ./internal/cpu386 -run 'TestADDByteSource' -count=1 -v
  全byte配對／六flags／地址別名／唯讀無寫與拒絕邊界 PASS
bash workplace/new-game-355-full-run.sh
  乾淨固定原EXE Go全套 PASS
bash workplace/new-game-355-formal-run.sh
python3 workplace/new-game-355-formal-verify.py
  10759共通正常列／35frames／原五步零值／新0F94停止／finalPNG保持 PASS
python3 workplace/new-game-355-source-verify.py
  CPU16行分支與三observer逆轉逐byte保持354 PASS
bash workplace/new-game-355-off-run.sh
  修明列來源後同8M1693列／PNG與68舊＋32新CLI負例／正對照 PASS
python3 workplace/new-game-355-backlink-verify.py
python3 apps/moo2/tools/startup_probe_131.py --check-add-byte-source-spec-backlinks
  92項回填／新355的32＋4負例與較早負例 PASS
```

原版兩次與乾淨全套各600s；8M／CLI與窄測180s，Docker固定Go1.24.13 image／network none／UID1000／原ZIP與patch唯讀。attempt1腳本建置輸入失敗已保存；CPU、正式原重播與Go全套無失敗後挑選結果。原EXE／LOG／PNG／RAM／私有腳本留忽略workplace，公開自製CPU／測試／診斷／規格／索引／守衛與雜湊。交接核對Git與Docker清理。

| 收據／核算 | SHA-256 |
| --- | --- |
| moo2-add-source-355.go | c872e72ed84611c4bb98fa8d9958e8e0036eafed28663d1172f209edf2666d54 |
| moo2-probe-355-input.txt.gz | 0c301fbf7d191d27e123b5a0ce826460f88e3bf9d711c2a1046eb14d8d17e443 |
| moo2-vbe-355-input.png | 1f757f5b16fe492795accf198c8db851443a062450e7f37ecf465c7eae3b6457 |
| new-game-355-input-run.sh | f2ed9c42ca381d4f451864fc3ffedd2658e8ad2dacecfdcf27f8247ef1684f1a |
| new-game-355-input-run-output.txt | 3fa48b72e10393532f945d84d58a269dc42307a767ccdc38eddd6de30bde588d |
| new-game-355-input-verify.py | 903433d9d1df6e0aaa436a4d487754b32d2a9be8ea3b7464aacad99a867e8ad8 |
| new-game-355-input-tests.txt | 70fa7d7740252ef9f42c743e0c191502e5192a44e8f9c5bfd4b8fb89af47406c |
| moo2-probe-355-formal.txt.gz | 58c0834c577fe6a8a32b64ad82567f231e3ae56b314ce21a616d506475bb4454 |
| moo2-vbe-355-formal.png | 1f757f5b16fe492795accf198c8db851443a062450e7f37ecf465c7eae3b6457 |
| new-game-355-formal-run.sh | 1de0400f464ed73539d32a1da7370d8d803550fc6d3702e1b34edd8827448f30 |
| new-game-355-formal-run-output.txt | d216742120da1aade2164ebc5d38306dfe3979dd0c866b5a8036e0378d7765a0 |
| new-game-355-formal-verify.py | 339f7cd6d40024c31ba87369debee4b1ac63f54cffc68b30819ab352209a6d4b |
| new-game-355-formal-tests.txt | 4dc98de74d75ad7bb000a46b9ee42d19c56397e4736cbc089b65c71e8ba6ea27 |
| moo2-355-cpu-narrow-tests.txt | 1b71347efae7958e54a7fadb8169a9b8e56a57af24108a64dbf5fbccda959385 |
| new-game-355-full-run.sh | 67aed4dd8cb4d7d31f9e1fa04f1a21b7383f3c6528893cf4f465f92dc0b8ecc4 |
| full-test-355.txt | 336f8b09bdd31fba5da14c5b72a2fbc594cf0729f50b87953f548b9ff062467d |
| new-game-355-source-verify.py | dee62839f9620cee20d06b2bd772345642603f22ebc28530ebae1dcf18829568 |
| new-game-355-source-tests.txt | 14e7d812eb4f30208c887780b1c80af65e9be489436c91e8b94a5576e4abf4b6 |
| new-game-355-off-run.sh | 65df3aff5021caa6d926c3e5690b6605a3d2ba1e6e50d38b18bf70f935fe9b43 |
| new-game-355-off-cli-tests.txt | a5eabaf6f165680e4e73a808efc13d87da68f29a4c55ca63bd6bf4ec96c09dff |
| moo2-probe-355-off-old.txt | 30515f0274bda6c1bfd233531dee79e0dcedd18e11e98d12223ffa260324f028 |
| moo2-probe-355-off-new.txt | e009646a55c98984e0535808d87441277c50d0fe97a7beade9658514d1f15374 |
| moo2-vbe-355-off-old.png | dd4c21dd11e57ec86ef759182fd3ebadd6b53870b54de1ef192e2553820286db |
| moo2-vbe-355-off-new.png | dd4c21dd11e57ec86ef759182fd3ebadd6b53870b54de1ef192e2553820286db |
| new-game-355-cli-tests.txt | 0c39c7d30d26388473df3e31ad7be38651d38f98677857d9e5a9de59a4aaac43 |
| new-game-355-backlink-verify.py | 2fa45fae3fbdb0e5f3cd47b6b59b86391d95a7c3993dfea4bc36a088ac2f694e |
| new-game-355-backlink-tests.txt | 2d37edfccf2a0a4d2fda48942e960387daa915af59b2604c3ba4dd6cfc096db0 |
| new-game-355-off-attempt1-run.sh | 2ac94e8e0ba001a75813c882a457dc0ac2cd58e208e594ca1abbd66580524faf |
| new-game-355-off-attempt1-output.txt | da277346d8fa7f358ee1d958e3d5c6ab6edb08bc4a13f9bba3eb54810c78eb32 |
