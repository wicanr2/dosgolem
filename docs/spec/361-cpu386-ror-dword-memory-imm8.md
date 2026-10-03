# 361：dword記憶體ROR與立即數

狀態：**CONFORMED，限定CPU memory ROR與原A1消費，非完整開局**
日期：2026-10-04
範圍：cpu386裸C1 /1、32位memory目的與全部imm8、32位ModRM／SIB及DS／SS；297既有register分支逐byte保持。不擴張word／D1／D3／ROL／其他shift或segment／67／F0／F2／F3。主庫RE-first保持。

## 原阻塞與ISA

沿[360](360-cpu386-cwd-word.md)，工具a16e94c4c15e5c78760cc670cb4f6766457a5f67／CPU SHA-256 ed94eaf7e9c363e8a2c89d5410b30a9e653a60532d31654ee91c14d042f75337；固定DOS1.31 EXE SHA-256 4e11be14217b4aafa1839f333bf5eba037f98b0c44e9e4752c96c464c260419f。原168496272於dosgolem_high_le:2376CB bytesC1 0D C4 0F 27 00 08 A1 C4 0F 27 00 4E 74 46 25拒絕memory dword ROR，after2376CD只取opcode／ModRM，未取disp32／imm8／source。R=[18181818 0 110 5C 2BD5D8 2BD648 69 34B94C]／段=[8 188 188 0 20 188]／flags202h。目的DS188:270FC4、imm08，原dword未知，不以EAX18181818猜來源；下一A1同址load可作真正消費端。

[Intel 80386 Rotate](https://www.ardent-tool.com/CPU/docs/Intel/386/manuals/prref386/RCL.htm)定義C1 /1 r/m32,imm8，count取低五位，單bit向右循環，CF只在非零count為結果最高bit；count1 OF為結果最高兩bit不同，其他flags保持。多位OF未定義，沿[297](297-cpu386-ror-dword-register-imm8.md)保留工具模型，不稱硬體逐值一致；count0結果／所有flags保持。沿297引用的Intel SDM2B頁4-524／4-525，count0語意與memory例外分開：本工具先decode完整address／imm，驗四byte可讀與可寫，count0讀目的但不寫回；實際硬體count0總線讀寫次數不列本輪parity。

## DRAFT擷取與READY

CPU未修改，同180M單次正常輸入，在2376CB最多兩步只讀診斷code16／完整R／六段／flags／FPU原bits、固定DS188:270FC2八bytes與全部RAM差異／callback／IRQ。activationPeek與RAM雜湊驗只讀，錯誤即停；360全部舊列／36PNG保持。原目的可讀／相鄰值及完整初態、公開ISA足夠才READY。

READY在既有297 register branch前加獨立C1 group1 memory分支，不改舊guard／register演算法／flags helper。先decodeAddress32／fetch完整imm8，再segmentLinear四byte可寫與readSegment32，masked count0不寫不改flags；其他count先計算rotate結果，writeSegment32全部成功後才發布CF及count1 OF。四byte逐次Bus晚期失敗可能已改前bytes，但R／段／flags／FPU不發布；不稱任意Bus rollback或硬體restart。純memory全部R保持，只有目的四bytes可改。

工程oracle以逐bit整除加回高bit，全部256 imm×64算術flags×邊界／每bit／補集、count0 no-write／其他count精確四write、全ModRM／SIB／DS／SS相異來源、signed位移／繞回／unaligned／last dword、未知／唯讀／段外／線性溢位、四byte每次read／write失敗、完整指令每byte截短與合法memory目的prefix。全R／段／nonzero FPU／相鄰RAM／非CF及非count1 OF的flags保持；多位OF保留只驗工具模型。297三個舊memory負例明示限定未知DS，新正例接完整合法目的，D3與prefix／截短保留；四byte錯誤不靠截短RAM掩蓋。

固定原EXE乾淨Go全套、關閉8M前輪360原收據／CLI與全舊規格守衛必須通過。正式最多兩步獨立按實際原dword核算ROR／CF與保持flags、下一A1讀真正新dword／EAX與flags保持、精確EIP及全部RAM索引。不增加cap／代寫／跳指令／重送／深入helper，固定1996日期不是seed。count8 OF未定義、原其他count／非零CF與count1 OF若未命中，不稱原動態已驗；完整開局與remake同狀態未知。

## 工具與入口

主要執行器/home/anr2/cht/dosgolem隔離副本workplace/dosgolem；支援／用法見README.md／CLAUDE.md。本檔同次加入000-index，原輸入入口workplace/new-game-361-input-run.sh，Go1.24.13 image sha256:1a6d4452c65dea36aac2e2d606b01b4a029ec90cc1ae53890540ce6173ea77ac，600s／2GiB／2CPU／128pids／UID1000／network none，原ZIP／patch唯讀。原EXE／LOG／PNG／RAM與私有腳本留忽略workplace，只公開自製CPU／測試／probe／spec／索引／守衛及雜湊。

## READY：未修改CPU的原目的核對

READY於2026-10-04。原輸入收據SHA-256 40eb10b9ba8e3fd07c94d2a9e2bd632b9cb990e5d96da65e6e007aa6360f6e96，全部11019原360列／36PNG保持，CPU仍ed94eaf7；三observer逆轉為a16e94c原probe。DS188:270FC2八bytes000018180B000000，目的DS188:270FC4 dword000B1818、左右兩byte0000／0000，來源與EAX18181818確實不同。完整R／段／flags202h／FPU127F／status0／depth0／八stack bits0、全部RAM與原拒絕保持；callback12／12／IRQ43075／43075非active／非failed／pending0。

原目的可讀及ISA充分，ROR8應000B1818→18000B18、CF0，SF／ZF／AF／PF及非算術flags保持；多位OF未定義，只沿工具保留OF0，工具完整flags202h。EIP2376D2；下一A1同址讀新dword，EAX18181818→18000B18／flags保持／EIP2376D7。原目的四byte由18 18 0B 00變18 0B 00 18，若DS188平坦base0則RAM差異僅270FC5／270FC6／270FC7，首byte與相鄰保持；實際四write Bus次數另由工程驗證，不以三個RAM差異宣稱只寫三byte。正式原版與全套仍待驗，READY限工具CPU，主庫玩法RE-first不變。

## 限定驗收：memory ROR與原A1真正消費

**已證實**：DRAFT未改CPU全部11019原360列／36PNG保持；原目的000B1818與當時EAX18181818不同，直接讀目的才READY。正式首遇前10949共通正常列／35既有frames、原完整R／段／flags／FPU原bits與code16／固定八byte窗口保持。

原168496272在2376CB執行C1 0D C4 0F 27 00 08，DS188:270FC4 dword000B1818 ROR8→18000B18、CF0、EIP2376D2，全部R／六段／FPU保持。SF／ZF／AF／PF及非算術flags保持；count8 OF未定義，完整flags202h只驗沿297保留工具模型，不稱硬體OF逐值parity。目的bytes18 18 0B 00→18 0B 00 18，全部RAM差異僅270FC5／270FC6／270FC7，首byte與兩側0000／0000保持，印證本筆DS188平坦映射。三個RAM差異不代表只寫三byte；原Bus四write次數未取，工程另驗同值byte也寫。

原168496273的A1 C4 0F 27 00從真正新目的load，EAX18181818→18000B18、EIP2376D7，flags202h／其餘R／六段／FPU與全部RAM保持；相鄰與固定窗口保持。兩步readonly=true／error nil，FPU127F／status0／depth0／八stack bits0保持，callback12／12、IRQ43075／43075非active／非failed／pending0。原count0／count1 OF與非零CF未由這條原路徑驗證，不以工程測試替代原實測。

**已證實，工程契約**：1212416個74邊界／每bit／補集×256完整imm8×64算術flags初態，沿297未改的單bit整除循環oracle，與CPU合併位移公式獨立。全ModRM／SIB／DS與SS相異來源、六種count與四目的值、signed位移／繞回／unaligned／last dword、完整flags／nonzero FPU／全R／段／相鄰RAM保持；masked0精確零write，其他count精確四write，包括來源與結果相同。工具masked0仍驗四byte可讀可寫、read目的但不寫，真硬體count0總線行為未驗，不稱exact。

未知／唯讀／段外／最後三byte／線性溢位，四byte各read與write失敗／count0無write失敗、完整指令每byte截短、合法目的的全部prefix與其他group／D1／D3拒絕通過。四byte晚期部分寫模型允許已寫前bytes，但不發布R／段／flags／FPU，不稱Bus rollback或硬體exception restart。CPU只加獨立C1 /1 memory分支，297 register／ROL／D1／D3／word與flags helper逐byte保持；逆轉為360。297三舊memory負例明示限定未知DS，原oracle不改、其他拒絕護欄不刪，新增合法memory完整正例。三observer與這項舊負例限制可逆轉為a16e94c，十四舊測試保持。

窄測1.025s、固定官方原EXE乾淨Go全套CPU38660.001s／machine1.824s通過；缺8088語料不算386硬體驗收。關閉8M1693原列／PNG另比前輪360原收據、68舊CLI＋32新180M負例及100M／120M／160M／180M正對照保持。正式核對腳本原arr不接受空[]，改允許空陣列後以同收據重驗，原guest未重跑挑選；CPU／原版正式重播均通過。

CPU SHA-256 b8c1844163fddd7e3557e19fc51abcb9bcb9c1d021fd415dae376b2afbb9c72b；新測試96bee4520e99c09b5edb8cda11de95fbf745811f15c1b9545c1ccc56f4663ab1；297修改後測試8e0e36c37f5f3a0469475ec4f5b6c19693964986cf437acafc1f6e2cfc3a1bca；正式probe4fd257a6edf9a3460af7f7d792ab2624474f0fd7070f3eb5ef9877d2357cd787。原正式收據40fb333d1aaeb9160779b6fe56b439b390d19b9d6f6ddfb7d50ec7d577eff512；乾淨全套965899b1f79e4a78f356155613b1da31feb50f65f3104a74f3f48c040a1aa081。

### 歷史361端點與當輪玩家阻塞

同一輸入自然達step_limit180000000，EIP215DCA／R=[FFFFFFFF D6 2C0864 A5 2BD630 2BD654 FC D8]／段=[8 188 188 0 20 188]／flags202h，bytes89 45 F4 83 7D F4 FF 74 35 8B 55 EC C1 E2 02 A1；無guest_cpu_stop／step_error／DOS exit。沒有增加cap，不能把exit0或無CPU拒絕稱完整開局。

finalPNG已改變，SHA-256 679f08239fe789c2f1ae82b5fa9a72a08cad884aa8d12f756a3fd9badf7f4c3a／RGB0d093a071c686fc389622b2bc2d80a30d2a051a8162ef819fb2d4976484c57c5。2026-10-04人工檢視：SELECT BANNER COLOR旗色選單上覆Error saving game／Permission denied對話框及CLOSE按鈕；未見完整星圖，不再將本輪終圖稱主要黑底或與360相同。只證玩家存檔路徑遇錯，不推測原存檔正式writer或存檔內容。

**已證實，工具政策**：probe原321／323使用machine.ReadOnlyFileProvider與OpenDirectoryReadOnlyFiles；internal/machine/le_startup.go的writeFile只接受io.Writer／可寫覆蓋層，唯讀提供者拒絕，不能假裝資料已落地。已有internal/machine/overlay_files.go的OpenDirectoryOverlayFiles(basePath,statePath)及來源保持／寫入／截斷測試，下一步沿用此隔離能力，不建立重複檔案系統。**強推論**：終圖Permission denied與唯讀政策相關；真正失敗DOS呼叫／檔名／mode／errno尚未擷取，不能斷言已定位唯一真因或直接代寫存檔。先以有界只讀診斷取錯誤輸入及返回，再審查只在容器新state目錄接既有覆蓋層，原版資料維持唯讀。

固定1996日期不是seed；原其他count／CF1／count1 OF、硬體count0 Bus行為與多位OF、失敗存檔呼叫／檔名／內容、正式writer／RNG、完整生成／開局與remake同狀態仍未驗，主庫RE-first保持。完成ROR只解這一CPU契約與真正load，不關閉整體玩家對拍。

### 回填帳與實際命令

不可變解析鍵：固定DOS1.31 EXE 4e11be14…＋dosgolem_high_le:2376CB＋C1 0D C4 0F 27 00 08。原2376CB memory ROR與下一A1已由規格361接通；360及十三份較早入口、297的scope／舊負例限制同次回填。000-index與--check-ror-dword-memory-spec-backlinks守衛限定目的／真正load／多位OF及180M存檔錯誤，不稱完整原版parity。

以下在Docker與隔離工具workplace/dosgolem執行；DRAFT輸入為CPU a16e94c未改時的歷史擷取／核對。
```text
bash workplace/new-game-361-input-run.sh
python3 workplace/new-game-361-input-verify.py
  未改CPU11019原360列／36PNG／目的000B1818與原拒絕 PASS
go test -p 2 -buildvcs=false ./internal/cpu386 -run 'TestRORDword' -count=1
  1.025s PASS
bash workplace/new-game-361-full-run.sh
  固定官方原EXE乾淨Go全套 PASS
bash workplace/new-game-361-formal-run.sh
python3 workplace/new-game-361-formal-verify.py
  10949正常前綴／35frames／原ROR及真正A1／同180M cap PASS
python3 workplace/new-game-361-source-verify.py
  只加memory分支／三observer及297舊負例限制可逆轉360，十四舊測試保持 PASS
bash workplace/new-game-361-off-run.sh
  8M1693原列／PNG、前輪360原收據與CLI負例／正對照 PASS
python3 workplace/new-game-361-backlink-verify.py
python3 apps/moo2/tools/startup_probe_131.py --check-ror-dword-memory-spec-backlinks
```

98項規格回填正對照、新361的39＋28缺證據負例與所有既有負例通過。公開守衛入口以上述命令執行；覆蓋限定ROR／真正A1、OF未知邊界、同180M存檔錯誤及十五份較早規格回填。

## 362存檔拒絕回填

原237024 SAVE10.GAM唯讀拒絕已由規格362定位；可寫正常路徑仍DRAFT，見[362](362-moo2-save-permission-boundary.md)。原165025480的INT21／3D01／DS188:2BDB68／SAVE10.GAM回AX5／CF1及RAM保持已取，拒絕源於唯讀provider無WriteFileProvider。先前呼叫與檔名未知已解；尚不稱原存檔成功。隔離overlay試跑改變前段流程，在80M完整表閘門停止，設定頁RGB相同而record11–15的+44四byte窗口各增8000h，欄位與消費未知；可寫試作只留本機，不接公開玩家path，不改原336 guard或點擊時刻。下一步依362有界讀初段開檔與這五窗口候選值所指內容，再審查正常輸入，完整開局／RNG與remake同狀態未驗。
