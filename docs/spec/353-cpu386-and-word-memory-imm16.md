# 353：word記憶體與完整立即值的AND

狀態：**CONFORMED，限定CPU契約／原三步／正常續行至新停止**
日期：2026-10-03
範圍：cpu386的66 81 /4 iw記憶體word，沿32位ModRM／SIB解碼。不擴張81其他memory群組、83 memory、段覆寫、67、F0／F2／F3；既有word register與memory CMP保持。

## 阻塞與公開契約

沿[352-moo2-generation-180m-continuation](352-moo2-generation-180m-continuation.md)，工具8ef52b1fc372bf96267d964f8b1ba903594d95c6、CPU SHA-256 b5c8bd900047e8f018ad66edaf5ae8e1009e9f6d04ec89dea4000c27f3326b30。固定官方DOS1.31 ORION2.EXE SHA-256 4e11be14217b4aafa1839f333bf5eba037f98b0c44e9e4752c96c464c260419f。原dosgolem_high_le:103BF9在163795435拒絕，bytes66 81 63 0C 7F FE C1 E2 07 09 53 0C；目的是DS:[EBX+0Ch]、imm16 FE7Fh。EBX5AA044／DS188，segment offset5AA050；原來源word未知，錯誤後103BFC是解碼停點。欄位用途未知，不追helper內部。

[Intel 80386原廠AND](https://www.ardent-tool.com/CPU/docs/Intel/386/manuals/prref386/AND.htm)列81 /4 iw：完整16bit立即值與word目的逐bit交集並寫回；CF／OF清零，SF依bit15、ZF依零值、PF依低byte偶同位。[Intel 80386第3.4.1節](https://www.ardent-tool.com/CPU/docs/Intel/386/manuals/prref386/s03_04.htm)明列AF未定義，清除只沿工具模型，不稱硬體定值。非可寫段與段外operand拒絕。沒有386實機語料，不稱逐週期／實機驗收。

## 最小實作與驗收契約

只增加operand16且memory group4分支，decodeAddress32／fetch16／readSegment16／writeSegment16，最後成功寫回才setLogicFlags16。R／六段／FPU保持；完整iw不按83的imm8符號延伸。DS／SS選擇沿既有base解碼，ESP／EBP base用SS，無base absolute用DS，SIB ESP index忽略。拒絕未審查prefix及截短，EIP可已解碼前進，沿工具error模型。

讀取失敗、descriptor唯讀／越界／未知selector不發布旗標且不寫RAM。沿既有write16低byte再高byte：受控Bus第二byte拒絕可以保留已寫低byte，旗標仍保持；明示此工具模型，不假稱硬體exception原子重啟。不更改共用write16或其他CPU路徑。

DRAFT先用原352完整180M正常輸入與未改CPU捕捉首遇103BF9：完整R／段／flags／FPU、原code16、DS來源與相鄰byte、stack4、readonly與RAM前後；最多三原步，遇錯即停。全部352舊列與PNG保持。實際來源可讀與公開CPU契約充分後才READY，禁止guest注入或CPU位址特例。

READY後自製獨立逐bit16bit交集與五旗標oracle：所有低byte配對、16bit單bit／補數／高位／零、64個初旗標組合；所有ModRM／SIB相異DS／SS、非對齊、負disp8、32位繞回、最後完整word與相鄰byte；未知段／唯讀／界限／逐byte讀寫失敗、截短與prefix。既有81 word register各群組、81 memory CMP／83 AND register保持。每次成功檢查完整RAM、R／段／FPU與精確EIP，AF另驗工具模型。

CPU386及固定原EXE的乾淨Go全套必須全部通過；8088外部語料缺檔skip另列，不算386實機證據。同180M正常輸入重播，至103BF9前所有可比舊列／PNG保持，再驗AND／原SHL EDX,7／OR [EBX+0Ch],EDX三步寫回及限定RAM差異。後段停止、畫面或新CPU拒絕據實記錄，180M不是完成保證。固定1996日期不是seed；正式writer、完整生成／開局、RNG與remake同狀態未知。主庫RE-first保持。

原版素材／LOG／PNG／RAM留忽略workplace；公開只保存通用CPU、測試、spec、索引、守衛與雜湊。Go1.24.13固定Docker image sha256:1a6d4452c65dea36aac2e2d606b01b4a029ec90cc1ae53890540ce6173ea77ac，原ZIP／patch唯讀，UID1000／network none／2GiB／2CPU／128pids與有界timeout。輪末核對Git、擁有權、Docker清理。

## 未改CPU初態與READY審查

DRAFT未改CPU以原352相同180M正常輸入重播，全部10793原352列／36PNG保持。private三區塊逆轉後probe逐byte保持352，CPU仍b5c8bd90…；python3 workplace/new-game-353-input-verify.py PASS。原輸入收據workplace/moo2-probe-353-input.txt.gz SHA-256 bf7cf698bf7c6cb1bc8e8e4cdf60f2f748801332b6375fdd83f0535204625c20。

163795435原103BF9：完整R=[0 0 0 5AA044 2BDB08 2BDB2C 0 0]、段=[8 188 188 0 20 188]、flags246h。DS188:5AA04F八bytes全零，目的word DS188:5AA050為0000；SS188:2BDB08四bytes01000000即1。callback12／12、IRQ41715／41715，皆非活動、pending0。readonly=true、step_ram_unchanged=true、ram_changes空；原拒絕後EIP103BFC／R／段／flags保持，完整FPU／終態沿舊收據保持。無額外玩家輸入或資料代寫。

公開ISA與實際完整word來源可讀，353審查READY。預期word0000 & FE7F = 0000、五旗標仍ZF／PF真、CF／OF／SF假，工具flags246h、下一EIP103BFF。其後原C1E207的EDX0左移7仍0、EIP103C02；原09530C以dword OR同DSoffset的原0與EDX0，結果仍0、EIP103C05。最多三步，不深入欄位用途。零→零的RAM差異不能單獨證明Bus實際寫入，另以CPU受控Bus成功兩次寫入／逐byte拒絕測試證明通用寫回；不宣稱原版Bus trace已測得寫次數。

上述三步是驗收預期，尚未正式續行。未改CPU原版收據已足夠，才開始通用CPU實作。

## 限定驗收：word記憶體AND與原三步

353限定CONFORMED只涵蓋CPU契約、原AND／SHL／OR三步與正常輸入續行至新停止點。完整生成／開局與remake同狀態仍未驗。主庫RE-first保持；不改平台、輸入、calendar、180M cap或原遊戲資料，沒有CPU位址特例或guest代寫。

**已證實，原三步**：163795435原103BF9的66 81 63 0C 7F FE，DS188:5AA050 word0000、immFE7F，結果0000、flags246h，EIP103BFF。163795436原C1 E2 07將EDX0左移7仍0、flags246h、EIP103C02；163795437原09 53 0C讀同DSoffset dword0與EDX0，結果0、flags246h、EIP103C05。三步完整R／段、DS來源與相鄰八bytes、stack1保持，readonly=true／ram_changes=[]／完整RAM保持，step error nil。callback12／12、IRQ41715／41715非活動，pending0。AF清除及多位SHL的OF只驗工具模型，不宣稱硬體定值。原零→零沒有Bus寫次數trace；通用CPU成功零／非零皆寫兩bytes由受控Bus測試證明，不能拿RAM未變冒稱沒有寫入。

原首遇前10723共通正常列／36PNG保持，與未改CPU原來源／flags／R／段／stack同狀態。DRAFT未改CPU保持全部10793原352列／36PNG；兩者邊界分開，正式續行不與舊stop診斷比較。原finalPNG仍d493c2b5628d55381176c9e676586ab8940fd62544302195b59570b6136e6ba6／RGB5a416d1db0fa55dc3523c99212ceb813056dda787a301b5e8d26bf799d058b59；沒有新正常開局或人眼畫面驗收。

**已證實，新停止**：164321317於dosgolem_high_le input223E93 bytes86 06 AA 46 4A 75 F7 07 C3 56 57 06 0F A0 0F A8拒絕「XCHG byte僅支援暫存器」。ModRM06為DS:[ESI]與AL的memory byte XCHG，原交換尚未執行；after223E95只解碼。原R=[FF 0 2 2BD976 2BD730 2BD888 2BD8A8 2BD97A]／段=[8 188 188 0 20 188]／flags202h已記錄；原DS來源0E／ALFF交換與後續ES STOSB已由[354](354-cpu386-xchg-byte-memory-register.md)驗證；不猜欄位用途。actual stop164321317，requested budget180000000，尚未達180M。probe exit0只代表錯誤收尾，不當作完整開局。

**已證實，CPU與回歸**：全部65536 word來源配原mask、65536低byte配對、16位單bit／補數／高位／零與64個初旗標組合通過；全部ModRM／SIB、DS／SS相異base、ESP忽略index／無base DS／非對齊／負disp8／32位繞回／最後word與相鄰byte保持。獨立逐bit交集／五定義旗標，AF工具模型另驗；完整R／段／FPU／RAM與精確EIP核算。未知selector／唯讀／段末／線性溢位／首末byte讀寫／截短／prefix與未審查memory群組拒絕；第二Bus寫入失敗保留低byte的既有工具模型，旗標不發布。原81 word各register群組與memory CMP、83 AND register保持。

go test -p 2 -buildvcs=false ./internal/cpu386 -run 'TestANDWordMemoryImmediate|TestANDWordRegisterSignedImmediate|TestCMPWord|TestSUBWord|TestADDWord' -count=1 -v PASS，0.312s。固定DOSGOLEM_MOO2_EXE=/tmp/game/ORION2.EXE的乾淨go test -p 2 -buildvcs=false ./... -count=1 PASS，CPU38677.939s／machine1.449s；從git ls-files -z建乾淨/tmp/test-src並複製新自製測試，未複製歷史探索main。缺8088外部實機語料不算硬體驗收；沒有386實機語料。關閉8M的1693原列／PNG、68舊CLI與32新180M負例及100M／120M／160M／180M正對照保持。

CPU只新增353的21行word記憶體AND分支，逆轉後逐byte保持352；正式probe與已驗private相同，三個observer區塊逆轉保持352。平台／8088 CPU不改。CPU SHA-256 1bfd9e0c7a63453549a7ab1e081d7d669ea8f38c0388e94743f9e0b0049793b1；自製測試02d55bbfac7846cc578101052f5186b62720b6fd0dd3faf95c2e387a56553ef5；probe848dba4c436357de62902e5f4185177bf2b5912decc832d0cd22ba1f34d8624f。正式原收據5e9ca79374c2a67298872b3b2d04d210d9241035d2644899182ebff3b28433a9，全套16f9e86bd4b984eef315f5e5fb4497cf7bc77dd55b4a719061ad76ff035128fb。

初版formal核算切在舊guest_cpu_stop，誤把四項stop診斷算進首遇前：late_startup_platform／irq7_passdown_state／protected_dma_pcm／irq7_real_entry。10727對10723的長度斷言拒絕，沒有原正常列差異；保留初次腳本／tests／stderr並重現exit1。以實際late_startup_platform label=stop切點及四項精確類型斷言修正後，原10723列與同三步嚴格核算通過；沒有更改CPU／原收據或重跑原版來挑結果。這是驗證腳本邊界問題。

### 回填帳與下一步

| 不可變鍵 | 已證實語意 | 較早規格 | 必須回填 |
| --- | --- | --- | --- |
| DOS1.31／EXE4e11be14…／dosgolem_high_le:103BF9→103BFF→103C02→103C05 | 原word0000／AND完整iw／SHL EDX0／OR dword0與flags246h三步 | 352 | 原103BF9 word記憶體AND與三步消費已由規格353接通 |

原223E93 byte記憶體XCHG與STOSB兩步已由規格354接通，見[354](354-cpu386-xchg-byte-memory-register.md)。原DS0E→FF／ALFF→0E及ESFF→0E、EDI增1／flags保持與兩步單byte RAM差異已驗，10742正常前綴／35frames與全套保持。原164560803的1CDD0F memory byte ADD已由355接通；下一步依361回填帳捕捉180M旗色選單存檔錯誤的真正DOS呼叫／檔名與返回，審查既有隔離覆蓋層，沿同180M，不提高cap或深入helper。完整生成／開局、正式writer、RNG與remake同狀態未知。

### 本機忽略證據索引與命令

90項規格回填與新353的33項缺證據／限定狀態／352回填／索引負例通過，較早守衛與352其餘349／350的另4負例保持。所有來源／收據1000:1000，工具root-owned／.md目錄零。

```text
bash workplace/new-game-353-input-run.sh
python3 workplace/new-game-353-input-verify.py
  未改CPU10793原352列／36PNG與原word初態 PASS
go test -p 2 -buildvcs=false ./internal/cpu386 -run 'TestANDWordMemoryImmediate|TestANDWordRegisterSignedImmediate|TestCMPWord|TestSUBWord|TestADDWord' -count=1 -v
  獨立CPU窄測與舊word契約 PASS
bash workplace/new-game-353-full-run.sh
  乾淨固定原EXE Go全套 PASS
bash workplace/new-game-353-formal-run.sh
python3 workplace/new-game-353-formal-verify.py
  10723共通前綴／36PNG／原三步／新XCHG停止 PASS
python3 workplace/new-game-353-source-verify.py
  僅21行CPU分支與三區塊observer／逆轉逐byte保持352 PASS
bash workplace/new-game-353-off-run.sh
  關閉8M1693列／PNG與68舊CLI＋32新CLI負例及正對照 PASS
python3 workplace/new-game-353-backlink-verify.py
python3 apps/moo2/tools/startup_probe_131.py --check-and-word-memory-spec-backlinks
  90項回填／新353的33負例與較早負例 PASS
```

以上命令均在固定Go1.24.13的Docker中執行。原版兩次各600s／2GiB／2CPU／128pids，全套600s，8M／CLI及窄測180s；network none／UID1000／原ZIP與patch唯讀。所有原EXE／RAM／LOG／PNG與私有腳本留忽略workplace，公開僅自製CPU、測試、診斷、規格、守衛與雜湊。

| 收據／核算 | SHA-256 |
| --- | --- |
| moo2-and-word-353.go | 848dba4c436357de62902e5f4185177bf2b5912decc832d0cd22ba1f34d8624f |
| moo2-probe-353-input.txt.gz | bf7cf698bf7c6cb1bc8e8e4cdf60f2f748801332b6375fdd83f0535204625c20 |
| moo2-vbe-353-input.png | d493c2b5628d55381176c9e676586ab8940fd62544302195b59570b6136e6ba6 |
| new-game-353-input-run.sh | d948eee16e1bd8995ae2ff705efa813a5ec3be89e19e6c9e5f74515fa95af50d |
| new-game-353-input-run-output.txt | d361590f9c5d03f14079df3afb60b85517179650af054dc4c717f91d54667963 |
| new-game-353-input-verify.py | 5c35378e2be730e0af658635c890d7dcfb4b561f2f41fad95ffeae5273d8c8b7 |
| new-game-353-input-tests.txt | e936db7ab38cc5afb5708afd958d578e0062337af64252a65f747291d8c8c92e |
| moo2-probe-353-formal.txt.gz | 5e9ca79374c2a67298872b3b2d04d210d9241035d2644899182ebff3b28433a9 |
| moo2-vbe-353-formal.png | d493c2b5628d55381176c9e676586ab8940fd62544302195b59570b6136e6ba6 |
| new-game-353-formal-run.sh | a98d928282e41760ac89009df9513396149cff59decf74fb2aec7d3fa94325e1 |
| new-game-353-formal-run-output.txt | 1acc8ab3b8f6ac60b66cf8193af680905f20d45b9fe497df0e9c32b69b391f99 |
| new-game-353-formal-verify.py | b1754dec76a12174c7bc0e81cca5845b0e3822bd8a1d16cbc833a6879367a667 |
| new-game-353-formal-tests.txt | 9d7617a2e3deb70e4bd7a284bbb840eda16bc3b8876f75edc09bcfa3c618a42c |
| moo2-353-cpu-narrow-tests.txt | 62d999ac687acb10705a7dfdfcd1e3ab2a0d97d9e1b548392d01d7b23d872ec7 |
| new-game-353-full-run.sh | 9acf4d81d65ecbafc793d6d57dc142e3d3b8d54e9e53ed549e27500bdfbec11d |
| full-test-353.txt | 16f9e86bd4b984eef315f5e5fb4497cf7bc77dd55b4a719061ad76ff035128fb |
| new-game-353-source-verify.py | 3f20a61abf8b010d0090e1daa423dc5cabc4e7b71f87cb4174a7638d8d6dbbb2 |
| new-game-353-source-tests.txt | b185ef0fa306a8519c42b6a59f072f5a45f14f80f869a25d24cc3838aeb0b5af |
| new-game-353-off-run.sh | a535eea06ec74fa44e764bd80d7ffab4a591ca9fdbc2b7fa7bddf48144578cd8 |
| new-game-353-off-cli-tests.txt | a5eabaf6f165680e4e73a808efc13d87da68f29a4c55ca63bd6bf4ec96c09dff |
| moo2-probe-353-off-old.txt | ca3296c1e76ebca60c1bdd7edec596b0aa8ed73611874183256b3d865c1e9cdc |
| moo2-probe-353-off-new.txt | 721b10698c61acb1cf5a86901e8e220ae1975f7e58b3e1ca9f0638d3a538e662 |
| moo2-vbe-353-off-old.png | dd4c21dd11e57ec86ef759182fd3ebadd6b53870b54de1ef192e2553820286db |
| moo2-vbe-353-off-new.png | dd4c21dd11e57ec86ef759182fd3ebadd6b53870b54de1ef192e2553820286db |
| new-game-353-cli-tests.txt | 0c39c7d30d26388473df3e31ad7be38651d38f98677857d9e5a9de59a4aaac43 |
| new-game-353-attempt1-verify.py | ec899e6027caf0852ecf2fdf86f9ab40bf77bbc902dd6bbaa8098e3d481ddcdb |
| new-game-353-attempt1-tests.txt | a2a4c3afb986d052f64f24b2d54748628518941f36b592a2c44cc1b7573435fc |
| new-game-353-attempt1-verify-output.txt | 40d9fbbed66ee080b1f6d5e4107ebdb0e6ced366041f54704429354839aee198 |
| new-game-353-backlink-verify.py | 08adcb15b06cfa3436dc1640a2d4e8b4c6b0a492c36c2e55d53594a8438df934 |
| new-game-353-backlink-tests.txt | 0f8df807f7b5c18644d12f066cff38d3c7d9628b43d23fef4826cbe5fef361d2 |

### 355的後續勘誤回填

原1CDD0F byte記憶體來源ADD與五步零值消費已由規格355接通，見[355](355-cpu386-add-byte-memory-source.md)。四SS來源00／AL00與第五目的00→00、flags202h→246h及RAM保持已驗，原非零ADD／進位未驗；10759正常前綴／35frames與固定EXE全套保持。原164561579的1CE387記憶體SETE已由356接通；下一步依361回填帳捕捉180M旗色選單存檔錯誤的真正DOS呼叫／檔名與返回，審查既有隔離覆蓋層，沿同180M，不提高cap或深入helper。完整生成／開局、正式writer、RNG與remake同狀態未知。

### 356的後續勘誤回填

原1CE387記憶體SETE寫回與下一JMP已由規格356接通，見[356](356-cpu386-setcc-byte-memory.md)。原SS188:2BD834 byte41→01／唯一RAM差異、flags246h保持與原JMP到1CE61E已驗；第三CMP數值與byte1 reader未驗。通用16條件memory純寫、343／344舊memory負例限定未知selector及完整新正例、固定EXE全套通過。原356於164567987停在1CF90A的word IMUL，來源word0000與原低word寫回已由357驗證；下一步依361回填帳捕捉180M旗色選單存檔錯誤的真正DOS呼叫／檔名與返回，審查既有隔離覆蓋層，沿同180M，不提高cap或深入helper。完整生成／開局、正式writer、RNG與remake同狀態未知。

## 2026-10-04 word立即值IMUL回填

原1CF90A word立即值IMUL與兩MOV零寫已由規格357接通，見[357](357-cpu386-imul-word-immediate.md)。原DS188:5A2084 word0000×5=0、EDI005AA5F4→005A0000／高word005A保持、定義CF／OF0與下一兩MOV dword0→0已驗；兩MOV不消費DI，undefined flags保存只屬工具近似。10767正常前綴／35frames／固定EXE全套保持，正式DI reader／原非零與overflow未知。原357的1CFD3F word NEG來源0000與原零值結果／下一分支已由358驗證；下一步依361回填帳捕捉180M旗色選單存檔錯誤的真正DOS呼叫／檔名與返回，審查既有隔離覆蓋層，沿同180M，不提高cap或深入helper。完整生成／開局、正式writer、RNG與remake同狀態未知，保留本檔原歷史定位與收據。

## 2026-10-04 word NEG回填

原1CFD3F word NEG與下一EB09已由規格358接通，見[358](358-cpu386-neg-word.md)。原DS188:5A207C word0000→0000、六定義flags246h與下一EB09到1CFD4E已驗；第三CMP只觀測flags246h→206h，SS:[EBP-564]來源未取，數值／NEG word reader不列驗收，第四JE按觀測ZF0不跳。10770正常前綴／35frames及固定EXE全套保持，晚期Bus第二byte失敗可部分寫但不發布flags。325舊66負例限定未知selector、268舊word register NEG負例加segment prefix，新全值域正例接合法word。歷史358於164610300在1D0944的word SUB memory目的拒絕，after1D0946只解碼、當時DS188:5AA6D1目的word未知／來源AX0，現已由359核對；下一步依361回填帳捕捉180M旗色選單存檔錯誤的真正DOS呼叫／檔名與返回，審查既有隔離覆蓋層，沿同180M，不提高cap或深入helper。原非零NEG／溢位與完整開局、資料語意／正式writer／RNG及remake同狀態未知，保留本檔原定位與收據。

## 359回填

原1D0944 word SUB與三POP及RET已由規格359接通，見[359](359-cpu386-sub-word-register-source.md)。原DS188:5AA6D1 word0003-AX0000=0003、六flags206h，SS188真正槽的EDX0000000E／ECX005AA5E8／EBX00000000與RET001D1E0B／ESP2BDB5C已驗，POP／RET不是目的word reader。10775正常前綴／35frames／固定EXE全套保持，既有ADD／其他SUB及十一舊測試不改，第二byte晚期部分寫不發布flags。歷史359於164984957在1D2A33的66 99 word CWD拒絕，after1D2A35，AX0001／DX0000，現已由360核對；下一步依361回填帳捕捉180M旗色選單存檔錯誤的真正DOS呼叫／檔名與返回，審查既有隔離覆蓋層，沿同180M，不提高cap或深入helper。原非零SUB來源／借位／溢位、目的word reader／欄位語意、正式writer／RNG／完整開局與remake同狀態未知，保留原歷史定位與收據。

## 360回填

原1D2A33 word CWD與下一SUB／SAR已由規格360接通，見[360](360-cpu386-cwd-word.md)。原AX0001／DX0000與CWD完整flags246h保持，真正SUB讀DX以1-0=1／六flags202h，SAR讀AX1→0與五定義flags／EIP1D2A3B已驗，AF不列原版parity。三步全部RAM／FPU原bits保持、10789正常前綴／35frames／固定EXE全套通過；裸CDQ／word SUB與SAR／十三舊測試不改。歷史360於168496272在2376CB拒絕C1 /1 memory ROR，當時DS188:270FC4來源未知／imm08、after2376CD未取disp／imm或source，現已由361核對；下一步依361回填帳捕捉180M旗色選單存檔錯誤的真正DOS呼叫／檔名與返回，審查既有隔離覆蓋層，沿同180M，不提高cap或深入helper。原負AX／EDX高word、新ROR來源與消費、正式writer／RNG／完整開局與remake同狀態未知，保留原歷史定位與收據。

## 361回填

原2376CB memory ROR與下一A1已由規格361接通，見[361](361-cpu386-ror-dword-memory-imm8.md)。原DS188:270FC4 dword000B1818 ROR8→18000B18、CF0與保持flags、三RAM差異270FC5／270FC6／270FC7、下一A1真正load到EAX18000B18已驗；count8 OF未定義，保留只驗工具模型。10949正常前綴／35frames與全套保持，十四舊測試不改，297舊memory負例限定未知DS並有完整新正例。沿同180M已無CPU拒絕，但終圖為旗色選單的Error saving game／Permission denied，完整開局未驗。呼叫／檔名與返回已由362定位；可寫試作80M前置仍DRAFT，下一步見362，不提高cap／代寫／重送／深入helper。原其他count／CF1／OF、存檔writer／內容、RNG與remake同狀態未知，保留原定位與收據。

## 362存檔拒絕回填

原237024 SAVE10.GAM唯讀拒絕已由規格362定位；可寫正常路徑仍DRAFT，見[362](362-moo2-save-permission-boundary.md)。原165025480的INT21／3D01／DS188:2BDB68／SAVE10.GAM回AX5／CF1及RAM保持已取，拒絕源於唯讀provider無WriteFileProvider。先前呼叫與檔名未知已解；尚不稱原存檔成功。隔離overlay試跑改變前段流程，在80M完整表閘門停止，設定頁RGB相同而record11–15的+44四byte窗口各增8000h，欄位與消費未知；可寫試作只留本機，不接公開玩家path，不改原336 guard或點擊時刻。下一步依362有界讀初段開檔與這五窗口候選值所指內容，再審查正常輸入，完整開局／RNG與remake同狀態未驗。

## 363前段差異與資料窗口回填

原SOUND3D02與五個+44窗口前128bytes已由規格363核對；可寫玩家路徑仍DRAFT，見[363](363-moo2-overlay-startup-and-setup-source.md)。原1192795／237024的SOUND.LBX讀寫開檔，兩側同原初態但唯讀AX5 CF1／overlay真handle5 CF0。80M五候選值各增8000h，指向的前128byte及RGB相同；兩側各27PNG／各自舊列保持，原418來源檔未變，state僅有內容未變的sound.lbx副本。之前「所指內容未取」已限定解出開頭128bytes，完整物件／角色／原指標消費仍未知。下一步依363另立DRAFT可寫profile正常ACCEPT前置，不忽略位址／調時刻／改舊336唯讀guard。完整存檔／音訊／RNG與remake同狀態未驗。
