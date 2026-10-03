# 359：word暫存器來源SUB

狀態：**CONFORMED，限定CPU word SUB與原三POP／RET，非完整開局**
日期：2026-10-04
範圍：cpu386的66 29 /r、r/m16目的／r16來源，register與memory形狀；沿32位ModRM／SIB及DS／SS，不擴張2B、段覆寫、67或F0／F2／F3。主庫玩法RE-first保持。

## 原阻塞與公開契約

沿[358](358-cpu386-neg-word.md)，工具a995e5d62249aef97f73cc52e11176acc6e3218b／CPU SHA-256 9840611ea0e3ae22ece69fd1f6f545dd08a316d1ed87247bbe061bf3f7f09522；固定官方DOS1.31 ORION2.EXE SHA-256 4e11be14217b4aafa1839f333bf5eba037f98b0c44e9e4752c96c464c260419f。原164610300於dosgolem_high_le:1D0944 bytes66 29 83 E9 00 00 00 5A 59 5B C3 53 51 52 56 57拒絕word SUB，after1D0946只解碼，未取ModRM／位移／來源與目的。原R=[0 64 0 5AA5E8 2BDB4C 2BDBA0 5AA614 5AA5F4]／段=[8 188 188 0 20 188]／flags206h。目的DS188:[EBX+E9h] offset5AA6D1 word未知，來源AX0000；後續POP EDX／ECX／EBX與RET堆疊值未知，不猜目的欄位或helper語意。

[Intel 80386 SUB](https://www.ardent-tool.com/CPU/docs/Intel/386/manuals/prref386/SUB.htm)定義29 /r word目的減來源再寫word與六算術flags。結果mod65536；CF為unsigned借位、AF為低nibble借位、OF為signed差超出[-32768,32767]、SF為低word負、ZF為低word零、PF為低byte偶數個1。全部非算術flags／六段／FPU保持；register只目的低word可改，來源先取得解決別名／高word保持，memory只目的兩bytes可改且全部R保持。沿358逐byte Bus寫回，全部成功後才sub16發布flags；第二byte晚期失敗可已改第一byte，不當任意Bus回滾或硬體exception restart。

## DRAFT擷取、READY與驗收

DRAFT不改CPU，同180M正常輸入首遇最多五步的只讀診斷：code24／完整R／六段／flags，首遇DS:[EBX+E8h]四bytes與首遇SS:[ESP]二十bytes；兩窗口位址只在首遇固定，POP改ESP／EBX也不漂移。activationPeek與完整RAM雜湊驗只讀，每Step記全部RAM變更索引／callback／IRQ，錯誤即停。358全部舊列／36PNG保持；目的word與POP／RET槽可讀、公開ISA充分才READY。

READY只移除word SUB早拒絕，加獨立operand16 op29分支，既有word ADD／dword ADD與SUB逐byte保持。register先取src與dstword，sub16後寫低word；memory decodeAddress32／readSegment16，writeSegment16(value-src)成功後才sub16發布flags。指令截短／目的未知selector、唯讀／段外／線性溢位／每byte read與write失敗不發布R／flags；EIP可已解碼，晚期部分寫按工具模型驗。來源／地址／目的別名要先取全部值，非法prefix仍拒絕。

獨立oracle以整數signed差範圍／modulo、unsigned與nibble大小關係、低byte位元計數、little-endian視圖驗低word。memory全部65536來源×16充分邊界目的×兩flags，全部八src與八dst別名／高word哨兵、別名全值域兩flags、完整算術flags初態與邊界配對；全ModRM／SIB／DS／SS、signed位移／繞回／unaligned與last word、nonzero FPU與相鄰RAM、唯讀／未知／越界／兩byte每次讀寫失敗／截短定址／完整合法memory的prefix。既有ADD／其他SUB與flags helper逐byte不改，固定官方EXE乾淨Go全套必須全過；缺8088語料不算386實機驗收。

正式原word目的-AX、六flags與唯二byteRAM改變按擷取值核算；之後三POP依真正SS槽核對EDX／ECX／EBX、ESP步進與其他R／段／flags，RET依第四槽核對EIP與ESP。POP／RET不是目的word reader，原後續word消費未知不猜補。首遇前可比正常列／35frames保持，finalPNG按實際核對，不增加180M／跳指令／代寫或重送input，不深入helper。固定1996日期不是seed；完整生成／開局、資料語意／正式writer、RNG與remake同狀態仍未知。

## 工具與入口

Go1.24.13 Docker image sha256:1a6d4452c65dea36aac2e2d606b01b4a029ec90cc1ae53890540ce6173ea77ac；原版／乾淨全套600s／2GiB／2CPU／128pids／UID1000／network none，原ZIP／patch唯讀。原EXE／LOG／PNG／RAM與私有腳本留忽略workplace，只公開自製CPU／測試／probe／spec／索引／守衛及雜湊。本檔同次加入000-index；原入口workplace/new-game-359-input-run.sh。主要原版執行器/home/anr2/cht/dosgolem的隔離副本workplace/dosgolem，其能力／用法見README.md／CLAUDE.md。

## READY：未修改CPU的原輸入核對

READY於2026-10-04。未改CPU的180M原輸入收據SHA-256 45694965a26be50c4581fa5cc5862d45adcf17a9c6d102efc323b2ce4dd749e6；358全部10845舊列／36PNG逐byte保持，三只讀observer逆轉後為a995e5d原probe，CPU仍9840611e。首遇固定DS188:5AA6D0四bytes0003000E，目的word DS188:5AA6D1為0003，AX0000，應得0003、CF0／OF0／SF0／ZF0／AF0／PF1，flags206h；相鄰00／0E保持。SS188:2BDB4C二十bytes0E000000E8A55A00000000000B1E1D00E8A55A00，POP EDX／ECX／EBX槽為0000000E／005AA5E8／00000000，RET槽001D1E0B。讀取前後全RAM／R／段／flags保持，callback12/12／IRQ41960/41960無pending或active，Step仍原prefix拒絕，未以診斷修guest。

公開ISA與原實際可讀輸入足以實作29 /r word，不猜目的欄位。預期SUB→1D094B，三POP→1D094C／1D094D／1D094E，RET→1D1E0B；ESP逐dword至2BDB5C。五步固定兩窗口，RAM應保持；它們不讀新目的word，不宣稱已取得word消費端。此READY僅工具CPU契約，主庫RE-first不變。

## 測試草案修正

首次窄測失敗僅因新增測試誤將既有66 2B word設為拒絕。回查cpu.go原4779分支與platform_gap_test.go原1765／2281，register與memory來源原已支援。移除錯誤負例，改核對既有2B結果／六flags與RAM保持；CPU原2B逐byte不改，不擴張範圍。更正後同一Docker／同一窄測命令乾淨重跑，不把測試草案錯誤寫成產品缺陷。

## 限定驗收：word SUB與原三POP／RET

**已證實**：DRAFT未改CPU全部10845原358列／36PNG保持；原目的word及真正SS槽可讀、ISA充分才READY。正式首遇前10775共通正常列／35既有frames保持，原完整R／六段／flags／code24及固定DS四byte／SS二十byte初態逐欄相同。

原164610300於1D0944的66 29 83 E9 00 00 00，DS188:5AA6D1 word0003-AX0000=0003，六定義flags為CF0／OF0／SF0／ZF0／AF0／PF1，flags206h保持，EIP1D094B。相鄰00／0E保持，原同值寫回Bus次數未取，工程另驗來源0仍兩write。原非零來源／借位／signed溢位未由此原路徑驗證。

原164610301／164610302／164610303的POP EDX／ECX／EBX按原SS188:2BDB4C的真正槽分別讀0000000E／005AA5E8／00000000，EIP1D094C／1D094D／1D094E，ESP依次2BDB50／2BDB54／2BDB58。原164610304的RET按第四槽001D1E0B，EIP1D1E0B／ESP2BDB5C。其他R／六段／flags206h、固定DS／SS窗口與全部RAM保持；五步readonly=true／error nil／ram_changes=[]，callback12／12、IRQ41960／41960非active／非failed／pending0。POP／RET不是目的word reader，不宣稱已取得後續word消費端或目的欄位語意。

**已證實，工程契約**：2097152個全部memory來源×16邊界目的×兩flags、1048576個八register別名全值域×兩flags，另全八src／八dst邊界配對及131072個memory八來源×64算術flags初態×邊界配對。獨立整數差／modulo、signed界限、nibble借位與低byte位元計數，不呼叫CPU flags helper；little-endian高word哨兵。全ModRM／SIB／DS與SS相異目的、signed位移／繞回／unaligned／last word、nonzero FPU／相鄰RAM保持、未知／唯讀／越界／線性溢位、兩byte各read及write失敗、截短與合法目的prefix。第二byte晚期部分寫模型只可已寫第一byte，不發布R／flags／FPU；不冒充任意Bus rollback或硬體exception restart。

CPU僅移除29 word早拒絕與新增獨立word SUB分支；來源／目的別名先取舊值，register保持高word，memory全部R保持／兩byte成功後才sub16發布flags。逆轉逐byte保持358，既有word ADD／dword ADD和SUB／其他SUB／flags helper與十一份舊測試保持，三observer逆轉為a995e5d原probe。既有66 2B word原已支援，本輪只加保持性核對不改原分支。

窄測0.685s、固定官方原EXE乾淨Go全套CPU38682.239s／machine3.104s通過；缺8088語料不算386硬體驗收。關閉8M1693原列／PNG、68舊CLI＋32新180M負例及100M／120M／160M／180M正對照保持。初次窄測的2B誤設負例已回查修正；關閉腳本初次錯用缺原始掛載入口，改完整既有入口重跑；正式驗證腳本空白分隔bytes取欄位修正後以同收據重跑。三者屬測試／環境問題，CPU與原版正式重播通過，不重跑挑選guest結果。

CPU SHA-256 995f059949b7caac9618ab8b2513999b64b4cb2c928bd608da96eff171d3a8d1；新測試a7a6cb7e44c56b3f49a035c0bcbf87978196c63a59f294ff88a0d9de57b6d0bd；正式probe1d72d00195ff24b6c9e2ba6e48385c6c40e222a5e2e4417f2a21e39984b8434d。正式原收據a5421d88e46dea38d24ad92c49d8abd1a46ed341d93e25ead73efcfcd09ec856；乾淨全套f341b54cc3b33ecc90c7900fb9b1a9a29033434b3d2c62e2a56868d799cec0c7。

### 歷史359停止與unknown

原164984957於dosgolem_high_le input1D2A33 bytes66 99 66 2B C2 66 D1 F8 98 01 C7 81 FF FF 7F 00拒絕operand16 CWD，after1D2A35只取prefix／opcode。R=[1 0 0 5A2EED 2BDB24 2BDB58 5A2EED A]／段=[8 188 188 0 20 188]／flags246h。原AX0001／DX0000可回查，尚未達180M；原CWD與SUB／SAR已由360接通；下一步依360回填帳取2376CB memory ROR來源與下一A1 load，沿同180M，不增加cap／跳指令／代寫或重送／深入helper。

finalPNG逐byte保持358，SHA-256 1f757f5b16fe492795accf198c8db851443a062450e7f37ecf465c7eae3b6457／RGB04fef4b6a6d6c6c485ef1ce0a82ea71591956cdd3b7cd37b8d1082a023e20e17，沿354人工檢視仍主要黑底與小型方形圖形，未見完整地圖。固定1996日期不是seed；原非零SUB來源／借位／溢位、新目的word reader與欄位語意、原CWD／SUB／SAR已見360、新ROR來源與消費、正式writer、RNG、完整生成／開局與remake同狀態仍未驗，主庫RE-first保持。

### 回填帳與實際命令

不可變解析鍵：固定DOS1.31 EXE 4e11be14…＋dosgolem_high_le:1D0944＋66 29 83 E9 00 00 00。原1D0944 word SUB與三POP及RET已由規格359接通；358及十一份較早入口同次回填，保留歷史原始定位與收據，不把當時目的未知寫成現行未取。000-index與--check-sub-word-register-source-spec-backlinks守衛驗限定SUB／真正SS槽與CWD停止，不稱完整原版parity。

以下在Docker與隔離工具workplace/dosgolem執行；DRAFT擷取為CPU a995e5d時的歷史執行，輸入驗證也於未改CPU時通過，後以/tmp舊CPU快照保存重驗輸出，沒有重跑guest。
```text
bash workplace/new-game-359-input-run.sh
python3 workplace/new-game-359-input-verify.py
  歷史未改CPU10845原358列／36PNG／word0003及SS槽／原拒絕 PASS
go test -p 2 -buildvcs=false ./internal/cpu386 -run 'TestSUBWord' -count=1
  0.685s PASS
bash workplace/new-game-359-full-run.sh
  固定官方原EXE乾淨Go全套 PASS
bash workplace/new-game-359-formal-run.sh
python3 workplace/new-game-359-formal-verify.py
  10775正常前綴／35frames／原SUB及三POP／RET／新CWD停止 PASS
python3 workplace/new-game-359-source-verify.py
  限定29 word及三observer逆轉保持358、十一舊測試保持 PASS
bash workplace/new-game-359-off-run.sh
  8M1693列／PNG及CLI負例／正對照 PASS
python3 workplace/new-game-359-backlink-verify.py
python3 apps/moo2/tools/startup_probe_131.py --check-sub-word-register-source-spec-backlinks
```

### 本機收據雜湊

原LOG／PNG／RAM與腳本留本機忽略workplace；以下只公開雜湊。

| 本機檔名 | SHA-256 |
|---|---|
| moo2-sub-word-359.go | 1d72d00195ff24b6c9e2ba6e48385c6c40e222a5e2e4417f2a21e39984b8434d |
| moo2-probe-359-input.txt.gz | 45694965a26be50c4581fa5cc5862d45adcf17a9c6d102efc323b2ce4dd749e6 |
| moo2-vbe-359-input.png | 1f757f5b16fe492795accf198c8db851443a062450e7f37ecf465c7eae3b6457 |
| new-game-359-input-run.sh | c4ad45371017abc0bf2e32552c81f73e2f86f6a538cd376c78985ae29c1b9f7f |
| new-game-359-input-run-output.txt | 93be53dc55c5eb22a08e4f683347c0965363ab7ce86ef29d9a5d558f2fc69958 |
| new-game-359-input-verify.py | 0799b673cd025fbf37ac0fb63602270899ba3d2401db362f91f968379020ba81 |
| new-game-359-input-tests.txt | 40ec9df86a3be0dc6d05a2b36adb1439cb1ce3fa6b2a50afa940891748885be3 |
| moo2-probe-359-formal.txt.gz | a5421d88e46dea38d24ad92c49d8abd1a46ed341d93e25ead73efcfcd09ec856 |
| moo2-vbe-359-formal.png | 1f757f5b16fe492795accf198c8db851443a062450e7f37ecf465c7eae3b6457 |
| new-game-359-formal-run.sh | 3e6927213c8f81cae873d1cb95ebb027d1a471dabb13f232ce0f93564558556b |
| new-game-359-formal-run-output.txt | 242535a7b416cfa9a90593cc8b9186aa835a80b6a2b7a3df27d781db04b5635f |
| new-game-359-formal-verify.py | 427c829e0ac23972ad1820ad2b41a5f4449fc95d5009f2e6331eaf8fdb9f3a37 |
| new-game-359-formal-tests.txt | 289344414b72db164dbe3bfe2a81a21f725bc26a0924a2d09a75f0e42ab45bc1 |
| new-game-359-unit-tests.txt | d82c13509fce2ecd11968c1b06f1e4786689734b359a0a2c532bb816d774277e |
| new-game-359-full-run.sh | 7a2b9ef3c96f6b59233f785e51090974c8becdbb1de5f675b7f1e492bd91b085 |
| full-test-359.txt | f341b54cc3b33ecc90c7900fb9b1a9a29033434b3d2c62e2a56868d799cec0c7 |
| new-game-359-source-verify.py | c6c4284f241d4ac1fb7a1327c1df25289c1d4b731003b2a39ea160ce1ede3de7 |
| new-game-359-source-tests.txt | 7d08bc04f2726e0a7f7e822f19a580eda7b98326ccba31c6d4252668913890b6 |
| new-game-359-off-run.sh | d87864b67b10fa9a206fb4ce98a29d7573ac7de562773fc149bc5c01f3b4a612 |
| new-game-359-off-cli-tests.txt | a5eabaf6f165680e4e73a808efc13d87da68f29a4c55ca63bd6bf4ec96c09dff |
| moo2-probe-359-off-old.txt | e488250f0b4f940dae079332df4ee18ad431691c799e8d0408511de55bf76c7f |
| moo2-probe-359-off-new.txt | 65d95947d29cf69464ede23a4a770b40c5dd6450ddc8dd045074b0002ca836d2 |
| moo2-vbe-359-off-old.png | dd4c21dd11e57ec86ef759182fd3ebadd6b53870b54de1ef192e2553820286db |
| moo2-vbe-359-off-new.png | dd4c21dd11e57ec86ef759182fd3ebadd6b53870b54de1ef192e2553820286db |
| new-game-359-cli-tests.txt | 0c39c7d30d26388473df3e31ad7be38651d38f98677857d9e5a9de59a4aaac43 |
| new-game-359-backlink-verify.py | a1e23ad060dd3d78f8905f402fb18ce9b2c1e36fdd0ade59090f1375482210d1 |
| new-game-359-backlink-tests.txt | 10dcf5f100ca93d8aeded6fc51ab5d6a627991944d2b66c8328907d8eecd6edf |

## 360回填

原1D2A33 word CWD與下一SUB／SAR已由規格360接通，見[360](360-cpu386-cwd-word.md)。原AX0001／DX0000與CWD完整flags246h保持，真正SUB讀DX以1-0=1／六flags202h，SAR讀AX1→0與五定義flags／EIP1D2A3B已驗，AF不列原版parity。三步全部RAM／FPU原bits保持、10789正常前綴／35frames／固定EXE全套通過；裸CDQ／word SUB與SAR／十三舊測試不改。原168496272在2376CB拒絕C1 /1 memory ROR，DS188:270FC4來源未知／imm08、after2376CD未取disp／imm或source；下一步依360回填帳取原dword／相鄰資料與下一A1 load，沿同180M。原負AX／EDX高word、新ROR來源與消費、正式writer／RNG／完整開局與remake同狀態未知，保留原歷史定位與收據。
