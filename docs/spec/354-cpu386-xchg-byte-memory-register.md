# 354：byte記憶體與暫存器的XCHG

狀態：**CONFORMED，限定CPU契約／原兩步／正常續行至新停止**
日期：2026-10-03
範圍：cpu386的86 /r byte記憶體交換，沿32位ModRM／SIB與DS／SS解碼。原86 register交換保持；不擴張87 memory、operand16、段覆寫、67或F0／F2／F3。

## 阻塞與公開契約

沿[353-cpu386-and-word-memory-imm16](353-cpu386-and-word-memory-imm16.md)，工具c7086292bf476be63a406131632c9cf8c4780ab6、CPU SHA-256 1bfd9e0c7a63453549a7ab1e081d7d669ea8f38c0388e94743f9e0b0049793b1。固定官方DOS1.31 ORION2.EXE SHA-256 4e11be14217b4aafa1839f333bf5eba037f98b0c44e9e4752c96c464c260419f。原dosgolem_high_le:223E93在164321317以86 06拒絕，後續bytesAA 46 4A 75 F7 07 C3。原R=[FF 0 2 2BD976 2BD730 2BD888 2BD8A8 2BD97A]／段=[8 188 188 0 20 188]／flags202h，來源是DS188:ESI2BD8A8 byte與ALFF；後續STOSB寫ES188:EDI2BD97A。兩側byte與相鄰資料未知，不追helper內部。

[Intel 80386原廠XCHG](https://www.ardent-tool.com/CPU/docs/Intel/386/manuals/prref386/XCHG.htm)列86 /r為r/m8與r8交換，旗標全部保持；不可寫或段外memory拒絕。memory XCHG即使沒有F0也有硬體bus lock。工具無匯流排週期／多CPU仲裁模型，本切片只證單一CPU的單Step語意與中間沒有機器tick，不稱實機lock波形／跨執行緒atomic或硬體fault精確。明示F0仍拒絕，不能因原opcode本身可執行而宣稱LOCK prefix已支援。

## 最小實作與驗收契約

memory分支先保存reg8的來源byte，decodeAddress32取得DS／SS／effective offset，完整readSegment8，再writeSegment8成功後才setReg8發布舊memory byte。reg8索引0..7分別AL／CL／DL／BL／AH／CH／DH／BH，未選的24bits與其他R／六段／全部旗標／FPU保持。保存來源與有效位址必須在寫入來源暫存器前完成，覆蓋AL／AH等與base／index同暫存器的別名。只有一個byte read／write，相鄰bytes不動。

唯讀／未知selector／段外／線性溢位／讀寫Bus拒絕不發布reg8／旗標；一byte Bus拒絕的自製fail-before-write模型RAM保持，不聲稱任意外部Bus失敗必可回滾。EIP可已解碼前進，沿既有工具error模型，不假稱硬體exception重啟。機器中斷／虛擬時間在Step邊界，CPU分支沒有任何遊戲位址或資料代寫。

DRAFT未改CPU以原353完整180M正常輸入捕捉首遇：完整R／段／flags／FPU舊終態、code16、DS:ESI及ES:EDI的byte與左右相鄰資料、readonly與完整RAM。最多兩原步，遇錯停止；保持全部353舊列／PNG。原byte可讀且公開契約充分後才READY。

READY後獨立交換oracle：8個來源byte register的所有256×256配對、全部ModRM／SIB、相異DS／SS、地址base／index與AL／AH等別名、負disp8／32位繞回／最後byte；全部旗標組合及非零FPU，完整R／段／flags／FPU／RAM／精確EIP保持；成功相同byte亦必有一寫。逐byte讀寫拒絕、未知／唯讀／段外與prefix／截短，原86 register的全部64配對保持。固定原EXE的乾淨Go全套必須全部通過；缺8088語料與無386實機資料另列，不假稱硬體驗收。

同180M正常輸入重播，首遇前可比舊列／PNG及同一DS byte／AL／ES:EDI資料保持，再驗XCHG後AL／DS寫回及原STOSB的ES寫回／EDI依DF增減。正式writer、資料語意、完整生成／開局、RNG與remake同狀態未知，主庫RE-first保持。原日期不是seed，不跳原呼叫、不重送輸入、不提高cap。

Go1.24.13固定Docker image sha256:1a6d4452c65dea36aac2e2d606b01b4a029ec90cc1ae53890540ce6173ea77ac，原ZIP／patch唯讀，UID1000／network none／2GiB／2CPU／128pids與有界逾時。原EXE／LOG／PNG／RAM與私有腳本留忽略workplace；公開只保存通用CPU、測試、診斷、規格、守衛與雜湊，同次加入000-index。收尾核對Git／擁有權／Docker清理。

## 未改CPU初態與READY審查

DRAFT未改CPU的正常180M原重播，全部10812原353列／36PNG保持；三觀察區塊逆轉source逐byte保持353，CPU仍1bfd9e0c…。python3 workplace/new-game-354-input-verify.py PASS；原輸入收據workplace/moo2-probe-354-input.txt.gz SHA-256 27f670f8c55613750722c2e6535f1f56b6a1dbf9864dda8bd4f9141853871b0c。

164321317原223E93的86 06，DS188:2BD8A7三bytes FF0E00，原來源byte DS188:2BD8A8為0E，ALFF；ES188:2BD979三bytes00FFFF，STOSB目的ES188:2BD97A為FF。R／段／flags202h與原353停止相同，DF0。readonly=true／RAM保持，callback12／12、IRQ41873／41873非活動、pending0；原仍拒絕在after223E95，尚未交換。FPU／終態沿原353舊列保持。

原完整byte與相鄰資料可讀，公開ISA充分，354審查READY。預期原交換成功後AL0E／DS:[ESI] FF，source window FFFF00；其他R／段／flags202h保持、EIP223E95，只有目的一byte資料可改。次步原AA把AL0E寫到ES188:2BD97A，destination window000EFF、EDI2BD97B／EIP223E96；來源與旗標保持。兩步完整RAM差異應各限一byte，不猜資料語意或caller用途。

上述為驗收預期，不稱正式原兩步已完成。硬體implicit bus lock的電氣／多CPU仲裁不在本工具模型，固定單CPU Step內不插入機器tick只驗邏輯原子性；顯式F0支援仍未知／拒絕。沒有提高180M或跳過原指令。

## 限定驗收：byte記憶體XCHG與原STOSB

354限定CONFORMED只涵蓋通用CPU byte交換、原XCHG／STOSB兩步與正常輸入續行至新停止。完整生成／開局與remake同狀態仍未驗；主庫RE-first保持。沒有改平台、正常輸入、1996日期、180M cap或原資料，不插入guest代寫或CPU位址特例。

**已證實，原兩步**：164321317原223E93的86 06，以DS188:2BD8A8 byte0E與ALFF交換，DS0E→FF／ALFF→0E，EIP223E95；source window FF0E00→FFFF00，destination window00FFFF保持。原RAM只有index2BD8A8改0E→FF。164321318原223E95的AA把AL0E寫到ES188:2BD97A，ESFF→0E，destination window00FFFF→000EFF，EDI2BD97A→2BD97B，EIP223E96；source windowFFFF00保持，原RAM只有index2BD97A改FF→0E。兩步完整R變化各限AL或EDI、六段／flags202h保持，readonly=true／error nil，完整RAM差異各一byte。callback12／12、IRQ41873／41873非活動、pending0；DF0。這是實際零以外的byte寫回，不以相同資料無變化冒稱交換完成。

原首遇前10742共通正常列／35既有frames保持，同一原DS byte／AL／ES目的與相鄰窗口已驗。DRAFT未改CPU則全部10812原353列／36PNG保持；兩個比較分母分開，舊stop四診斷不列正常前綴。正式finalPNG改為1f757f5b16fe492795accf198c8db851443a062450e7f37ecf465c7eae3b6457／RGB04fef4b6a6d6c6c485ef1ce0a82ea71591956cdd3b7cd37b8d1082a023e20e17，屬通過XCHG後的較晚原畫面，不聲稱仍保留舊finalPNG。640×480人工檢視主要為黑底，僅小型方形圖形可見；其用途未知，沒有完整地圖或完整開局UI驗收，不當作GUI已完成。

**已證實，當輪停止**：164560803於dosgolem_high_le input1CDD0F bytes02 45 F8 02 45 E4 02 45 FC 02 45 E0 00 43 07 8A拒絕「byte運算記憶體形式尚未支援」。首三bytes是ADD AL, SS:[EBP-8]，SS188／EBP2BDB44／來源offset2BDB3C、AL0；原來源byte未知，after1CDD11只解碼，沒有執行ADD。原R=[0 5A2044 5AA044 5AA5E8 2BDB18 2BDB44 2 0]／段=[8 188 188 0 20 188]／flags202h。requested budget180000000，actual stop164560803，尚未達180M；probe exit0只是錯誤收尾，不是正常開局。

**已證實，CPU回歸**：8來源byte register各全部256×256配對，以獨立little-endian byte視圖選lane與交換，不重用CPU遮罩函式；所有ModRM／SIB、相異DS／SS、base／index與AL／AH等來源別名、ESP忽略index／無base DS／負disp8／32位繞回／最後byte／相鄰資料、64旗標組合與非零FPU通過。成功相同byte仍一寫；未知selector／唯讀／段外／線性溢位／讀或寫Bus拒絕不發布來源register／旗標，自製fail-before-write Bus的RAM保持。所有prefix與截短拒絕，原86 register的64配對保持。

go test -p 2 -buildvcs=false ./internal/cpu386 -run 'TestXCHGByte' -count=1 -v PASS，0.710s。固定DOSGOLEM_MOO2_EXE=/tmp/game/ORION2.EXE的乾淨go test -p 2 -buildvcs=false ./... -count=1 PASS，CPU386150.111s／machine1.864s；從git ls-files -z建乾淨/tmp/test-src，再複製新CPU測試，未帶入歷史探索main。缺8088語料不算硬體驗收，沒有386實機語料。關閉8M1693原列／PNG、68舊CLI與32新180M負例及100M／120M／160M／180M正對照保持。

CPU只用16行memory分支替換1行拒絕，逆轉逐byte保持353；原register交換不改。正式probe與已驗private相同，三觀察區塊逆轉逐byte保持353；平台／8088 CPU不改。CPU SHA-256 3b4f3dc4e3054bd92ce252f54202414c47dcc501257be7d0cf538c02ea449132，自製測試527846c29fc2b3da8043053ed8bdbf587c616db3216a6a4ccc13a1d9e51359ce，probe9c15f43b426eef78dbc983cf840df926b73817ecbbc5eea3d364a4fbb7e82d26；正式原收據7fe5e0408b1a24d44fcb8b02d3f618f218370aaa917648519d2d518cc1be5e43，全套ee150537f153e610e107754d74cb6de3abca7daad880a7927495d5fd80449393。

硬體implicit bus lock只引用原廠契約；工具單CPU Step內不插入機器tick，沒有跨執行緒或多CPU仲裁／實機lock波形驗收，顯式F0仍拒絕。資料語意、正式writer、完整生成／開局、RNG、人耳與remake同狀態未知。

### 回填帳與下一步

| 不可變鍵 | 已證實語意 | 較早規格 | 必須回填 |
| --- | --- | --- | --- |
| DOS1.31／EXE4e11be14…／dosgolem_high_le:223E93→223E95→223E96 | 原DS0E／ALFF交換及ES STOSB，兩步各一byte RAM差異、flags保持 | 353、352 | 原223E93 byte記憶體XCHG與STOSB兩步已由規格354接通 |

原1CDD0F byte加法已由355接通；下一步依357回填帳擷取原1CFD3F的DS188:5A207C word NEG來源與後續消費，沿同180M，不增加cap或深入helper。

### 本機忽略證據索引與命令

91項規格回填正對照、新354的34缺證據／限定狀態／353回填／索引負例及352另2負例通過，較早負例保持。所有來源／收據1000:1000；工具root-owned／.md目錄零。

```text
bash workplace/new-game-354-input-run.sh
python3 workplace/new-game-354-input-verify.py
  未改CPU10812原353列／36PNG、原DS byte0E／ALFF／ES目的FF PASS
go test -p 2 -buildvcs=false ./internal/cpu386 -run 'TestXCHGByte' -count=1 -v
  所有byte配對／位址別名／旗標保持／拒絕邊界 PASS
bash workplace/new-game-354-full-run.sh
  乾淨固定原EXE Go全套 PASS
bash workplace/new-game-354-formal-run.sh
python3 workplace/new-game-354-formal-verify.py
  10742共通正常列／35frames／原兩步／新ADD停止及finalPNG改變 PASS
python3 workplace/new-game-354-source-verify.py
  CPU16行替換1行拒絕與三observer區塊／逆轉逐byte保持353 PASS
bash workplace/new-game-354-off-run.sh
  關閉8M1693列／PNG、68舊CLI＋32新CLI負例／正對照 PASS
python3 workplace/new-game-354-backlink-verify.py
python3 apps/moo2/tools/startup_probe_131.py --check-xchg-byte-memory-spec-backlinks
  91項回填／新354的34負例／352另2負例及較早負例 PASS
```

均在固定Go1.24.13 Docker執行，原版兩次各600s／2GiB／2CPU／128pids，乾淨全套600s，8M／CLI與窄測180s；network none／UID1000／原ZIP與patch唯讀。沒有CPU或驗證失敗後挑選重跑結果；formal新停止與PNG變化依同一收據補嚴格斷言重讀。原EXE／LOG／PNG／RAM與私有腳本留忽略workplace；公開只保存自製CPU、測試、診斷、規格、索引、守衛與雜湊。

| 收據／核算 | SHA-256 |
| --- | --- |
| moo2-xchg-byte-354.go | 9c15f43b426eef78dbc983cf840df926b73817ecbbc5eea3d364a4fbb7e82d26 |
| moo2-probe-354-input.txt.gz | 27f670f8c55613750722c2e6535f1f56b6a1dbf9864dda8bd4f9141853871b0c |
| moo2-vbe-354-input.png | d493c2b5628d55381176c9e676586ab8940fd62544302195b59570b6136e6ba6 |
| new-game-354-input-run.sh | d4be4a474c9dd88c40570c0825ab18bbec7c0b466f201f845fcb8a8042d09ece |
| new-game-354-input-run-output.txt | 63317953747d1219878a0a322dec995adcc33e6a389b782dee9739f843819e1b |
| new-game-354-input-verify.py | 14c3819d8c6d19c899af56b362fc85310b4172628042490c9b8223825d503564 |
| new-game-354-input-tests.txt | 1066544c02d6d09839a7f95527787382420b1a5c69db490fe59d0e0be629db1f |
| moo2-probe-354-formal.txt.gz | 7fe5e0408b1a24d44fcb8b02d3f618f218370aaa917648519d2d518cc1be5e43 |
| moo2-vbe-354-formal.png | 1f757f5b16fe492795accf198c8db851443a062450e7f37ecf465c7eae3b6457 |
| new-game-354-formal-run.sh | 56b50477e63a82e9156df24235b1c94aa9404027f81296cac36b92867715b838 |
| new-game-354-formal-run-output.txt | 8a2ee512fb30d871f36f75de0a6ff8fdba1bc42031be389fdb8984edd1d57d4c |
| new-game-354-formal-verify.py | 913a781bb8c07d74fa686f8ffa9e5f75e214c723b7975fa22bac82bff1ac8a73 |
| new-game-354-formal-tests.txt | 890a4ad081f9bdeb99f02628a7b0ffcafa0b1a338ed57c4b90dccb40076b18a3 |
| moo2-354-cpu-narrow-tests.txt | ac15f0602ad562d675a37433a727637d4f2c8d652ebec28545d981b139893d48 |
| new-game-354-full-run.sh | 29b29794f8b03e782186b3424e82ca2b2e030a43e1cccc7fd32bd1859f2d1925 |
| full-test-354.txt | ee150537f153e610e107754d74cb6de3abca7daad880a7927495d5fd80449393 |
| new-game-354-source-verify.py | e3db16dcf472815aab97aa3964f4a9e5ac68315a4e61d1f05d1b2aa67eccc462 |
| new-game-354-source-tests.txt | 11cb4362cd90f937579beeced8b5c8c20ec3663fb01925a7ec6ecded0f1092d0 |
| new-game-354-off-run.sh | 6cb00aa78d1c491d49d1e752f427ad65bffb57b84bbc46b93fb460bc28df4a38 |
| new-game-354-off-cli-tests.txt | a5eabaf6f165680e4e73a808efc13d87da68f29a4c55ca63bd6bf4ec96c09dff |
| moo2-probe-354-off-old.txt | 4c797d9053666d7d54370c71a72af0ddf3dc3eacd36d3c0cb6556fdf83697624 |
| moo2-probe-354-off-new.txt | 2e1bcbf17cd47568407dc42200aa8c0431ebc25ded2ba66134f76316f3b914dc |
| moo2-vbe-354-off-old.png | dd4c21dd11e57ec86ef759182fd3ebadd6b53870b54de1ef192e2553820286db |
| moo2-vbe-354-off-new.png | dd4c21dd11e57ec86ef759182fd3ebadd6b53870b54de1ef192e2553820286db |
| new-game-354-cli-tests.txt | 0c39c7d30d26388473df3e31ad7be38651d38f98677857d9e5a9de59a4aaac43 |
| new-game-354-backlink-verify.py | 2800ded3b650d67cb6a16bb91ceb3a4acfffc2a349a24a4d08418ce2390130d6 |
| new-game-354-backlink-tests.txt | 1212830700af6cfbab65c597d0856168ae0b35fc00dfecc858cb5688e28f1b2b |

### 355的後續勘誤回填

原1CDD0F byte記憶體來源ADD與五步零值消費已由規格355接通，見[355](355-cpu386-add-byte-memory-source.md)。四SS來源00／AL00與第五目的00→00、flags202h→246h及RAM保持已驗，原非零ADD／進位未驗；10759正常前綴／35frames與固定EXE全套保持。原164561579的1CE387記憶體SETE已由356接通；下一步依357回填帳擷取原1CFD3F的DS188:5A207C word NEG來源與後續消費，沿同180M，不增加cap或深入helper。完整生成／開局、正式writer、RNG與remake同狀態未知。

### 356的後續勘誤回填

原1CE387記憶體SETE寫回與下一JMP已由規格356接通，見[356](356-cpu386-setcc-byte-memory.md)。原SS188:2BD834 byte41→01／唯一RAM差異、flags246h保持與原JMP到1CE61E已驗；第三CMP數值與byte1 reader未驗。通用16條件memory純寫、343／344舊memory負例限定未知selector及完整新正例、固定EXE全套通過。原356於164567987停在1CF90A的word IMUL，來源word0000與原低word寫回已由357驗證；下一步依357回填帳擷取原1CFD3F的DS188:5A207C word NEG來源與後續消費，沿同180M，不增加cap或深入helper。完整生成／開局、正式writer、RNG與remake同狀態未知。

## 2026-10-04 word立即值IMUL回填

原1CF90A word立即值IMUL與兩MOV零寫已由規格357接通，見[357](357-cpu386-imul-word-immediate.md)。原DS188:5A2084 word0000×5=0、EDI005AA5F4→005A0000／高word005A保持、定義CF／OF0與下一兩MOV dword0→0已驗；兩MOV不消費DI，undefined flags保存只屬工具近似。10767正常前綴／35frames／固定EXE全套保持，正式DI reader／原非零與overflow未知。新164568139在1CFD3F的66 F7 /3 word memory NEG拒絕、after1CFD42只解碼、DS188:5A207C來源word未知；下一步依357回填帳，維持180M。完整生成／開局、正式writer、RNG與remake同狀態未知，保留本檔原歷史定位與收據。
