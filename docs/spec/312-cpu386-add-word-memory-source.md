# 312：word ADD 的記憶體來源

狀態：**CONFORMED**
日期：2026-10-03
範圍：CPU386 的 66 03 /r，記憶體來源、word 暫存器目的；不修改主庫玩法。

## 原始定位與平台契約

工具基線ba3239ce0696f2e9bf898b543cee04a8ff455cab；固定官方1.31 ORION2.EXE SHA-256 4e11be14217b4aafa1839f333bf5eba037f98b0c44e9e4752c96c464c260419f。311兩個明示1996-01-01、同48M正常Esc／50M上限原版流程，在第48797763步、高位LE0x210C7E拒絕66 03 05 A4 BE 29 00。部分解碼後EIP210C81非成功；EAX0000000F、DS0188、flags202h。311基線未保存來源DS:[0029BEA4]的word，後續66 A3 A2 BE 29 00當時尚未執行；來源與寫回由下方診斷及正式收據補齊。

公開來源：[Intel 80386 Programmer’s Reference，ADD](https://www.ardent-tool.com/CPU/docs/Intel/386/manuals/prref386/ADD.htm)。03 /r是ADD r16,r/m16或r32,r/m32；此處66選word，結果是目的低word加來源word，更新CF／PF／AF／ZF／SF／OF。來源只讀、不修改，高16位與其他核心保持。

## READY候選與驗收

- typed input為目的uint16、既有32bit effective address／預設DS或SS、來源uint16；結果mod65536。保留目的高word、其他R／六段／FPU／非算術flags與RAM；完整讀取來源後才發布算術結果。
- 沿既有decodeAddress32與readSegment16，包括一般基址、disp8／disp32、SIB與no-base；segment override／REP／REPNZ／67／LOCK仍依原prefix閘門拒絕。不擴張權限／page fault或完整386。
- 來源兩bytes皆必須能讀，缺descriptor／越界／bus錯誤／截短解碼不得改R／flags／RAM。既有fetch的部分EIP明示，不冒稱全面rollback。合法readonly來源可讀。
- 驗八目的／非零高word、word邊界與全部低word對原版來源、獨立六旗標、DS／SS／SIB／不對齊、來源只讀、拒絕與既有word register ADD／01 memory目的及dword03回歸。
- 固定EXE全套，兩組同日期／Esc正常原版輸入，完整ADD核心與實際66 A3四周8byte窗口、後三條consumer、時計／IRQ與畫面驗證後才限定CONFORMED。不以自製測試代替原版正常入口。

## 邊界與回填

不可變鍵固定EXE＋dosgolem高位LE0x210C7E＋66 03 05 A4 BE 29 00。地址不能混用IDA線性或檔案偏移；DS來源／寫回另標selector:offset。日期／AH2Ch／RNG／255／299／人耳／主選單與整款remake限制沿310／311。CPU缺件與資料欄位用途分開，未知欄位不猜語意，不深入runtime helper。原版資料與完整收據只留本機。閉合後回填311停點及apps/moo2/tools/startup_probe_131.py護欄，保留歷史拒絕證據。

## 原版唯讀診斷與READY審查

已證實：同一正常48M Esc／明示epoch流程在既有停點讀DS0188:0029BEA0的8bytes為0F 00 00 00 02 00 02 00。來源DS0188:0029BEA4的word=0002h，AX=000Fh，完整R／六段／flags202h／窗口均保持拒絕前值；未寫回原版資料。CPU來源SHA-256 cfe5bc387aee00907acd897c3c6b77a5e4186828250e53fbc0513d0e40b504e1保持311。診斷gzip SHA-256 8f0c14193c36e21a76b16af706dd7d7a3ce44d72fd1288421c95181d2b97a778；與311正式全終端除一筆唯讀觀測、PNG名稱與解壓mtime／DTA四bytes外逐列相同。Go1.24.13／固定映像與原版輸入沿310／311，600s、network none、UID1000、2GiB／2CPU／128pids，沒有更改日期、Esc排程或50M上限。

公開算術與現有地址／segment框架足以READY：只補03的word記憶體來源，不改位址、日期或遊戲欄位以避開指令。獨立公式預期000Fh+0002h=0011h，六旗標CF0／PF1／AF1／ZF0／SF0／OF0，flags202→216h；下一66 A3預期只改DS0188:0029BEA2兩bytes為11 00。這些是公開CPU規格導出預期，尚待真正原版執行驗收。欄位語意未知且不阻塞限定CPU缺件。

## 實作與目標測試

沿既有03 word暫存器分支，記憶體形狀使用decodeAddress32／readSegment16，完整兩bytes取得後才用add16與低word寫回。既有暫存器與dword行為保持。readonly來源可讀；未取得完整operand前不發布R／flags／RAM。readSegment16既有平台callback／descriptor與fetch錯誤語意保持，不宣稱新增完整CPU權限或一般回滾。

三個新CPU測試與三個既有word ADD回歸全PASS：八目的×全部65536低word對原版來源2、49組word邊界、獨立無號進位／低半byte進位／byte popcount／有號和範圍的六旗標；高word、全部R／六段／非算術flags與非零完整FPUControl／FPUStatus／FPUStack／FPUDepth、整個RAM保持。DS／SS不同base、一般基址／SIB／no-base／disp8／disp32／32bit wrap與不對齊、readonly來源、截短opcode／disp／SIB、缺descriptor／限界跨越／bus第二byte失敗／linear超界／prefix拒絕、既有register word ADD／01 memory目的／dword03與真正66 A3兩byte寫回全通過。缺原版資料的machine包本次目標篩選沒有匹配測試，不冒稱其新測試通過；固定EXE全套另驗。

CPU SHA-256 0604f00fae843fc162ce970cef7479ca4d733d0c3f4a66a5fc412030d5c628b7；internal/cpu386/add_word_memory_source_test.go a5d79ad349fd3f6a9daa750c93ee413993ed0aad64bba6246c1dcc1d8407528e；workplace/moo2-probe/main.go 1e57c9190cb35568e24557bdc62267a6fc00d34790d773eb37a4120b3bcf121c。目標收據workplace/moo2-312-add-memory-tests.txt SHA-256 ca5f3b75d5f344dd147483f67a9bfdefeee1afe7c5cb8fdef3e75a36bd1afe9d；go test -p 2 -buildvcs=false ./internal/cpu386 ./internal/machine -run 'Test(ADDWordMemorySource|WordADD)' -count=1 -v，180s、2GiB／2CPU／128pids、UID1000、network none。

首輪自製測試誤用不存在c.FPU欄位，編譯失敗；只修測試以實際四欄完整快照並設定非零值，同一命令乾淨重跑，未改CPU運算或放寬斷言。workplace/moo2-312-add-memory-tests-compile-failure.txt SHA-256 db6ffbbbe6e0faf831fa5a5c053a3aa1b4093666fad421c56ed2eaeef5ab0c7f。

## 正式驗收命令

固定EXE全套與三組正常命令沿[310-moo2-dos-calendar-date.md](310-moo2-dos-calendar-date.md)的600s Docker入口，原檔417根層與官方EXE在/tmp/game乾淨組合，MOX.SET來源不改。本次所有輸出檔名中的311改312；全套workplace/full-test-312.txt，三原版gzip workplace/moo2-probe-312-full-game.txt.gz／moo2-probe-312-mouse-event.txt.gz／moo2-probe-312-unconfigured.txt.gz與對應PNG。兩設定DOSGOLEM_MOO2_CALENDAR_EPOCH=1996-01-01，固定DOSGOLEM_MOO2_HARDWARE_ESCAPE_AT_48000000=1、DOSGOLEM_MOO2_MAX_STEPS=50000000、DOSGOLEM_MOO2_SEPARATE_DOS=1，第二組加既有DOSGOLEM_MOO2_MOUSE_EVENT_AFTER_POSITION=1；第三組不設定epoch。go test -p 2 -buildvcs=false ./... -count=1後順序go run -buildvcs=false ./workplace/moo2-probe /tmp/game/ORION2.EXE --game-dir /tmp/game，不啟用私有IRQ1診斷。

## 正式原版驗收與限定CONFORMED

固定EXE全套已PASS，workplace/full-test-312.txt SHA-256 a8ed7290acc8531000959f85d968bfa137237a9d8fa448707838d7f56ccebfc7。兩設定日期與未設定第三流程皆由同一有界Docker命令完成，原始ZIP／patch／EXE／MOX.SET雜湊沿310，不增加RNGseed、私有IRQ1直入或遊戲資料注入。

已證實，兩正常設定流程各四筆真正CPU觀測，全部R／六段／flags與8byte窗口核對：

| 外層步／dosgolem高位LE | bytes與真正執行結果 |
| --- | --- |
| 48797763／0x210C7E→0x210C85 | 66 03 05 A4 BE 29 00，AX000Fh＋DS0188:0029BEA4的0002h→0011h；EAX0000000F→00000011，flags202h→216h，來源窗口0F00000002000200保持 |
| 48797764／0x210C85→0x210C8B | 66 A3 A2 BE 29 00，真正寫DS0188:0029BEA2兩bytes為11 00，窗口0F00000002000200→0F00110002000200，來源及其餘六bytes／全部核心保持 |
| 48797765／0x210C8B→0x210C90 | BB 00 01 00 00，MOV EBX,100h；只有EBX8→100，flags216h與窗口保持 |
| 48797766／0x210C90→0x210C93 | 8B 45 F4，MOV EAX,SS:[EBP-0Ch]，觀測EAX11→4；其他核心／flags／DS窗口保持，SS來源未另存窗口，不把4的欄位用途解成已證實 |

第一筆六旗標以獨立加法／有號範圍／popcount驗CF0／PF1／AF1／ZF0／SF0／OF0，不以CPU helper當預期。原版目的高word0，高word非零由全word／八目的測試補足。全部311停點前基線除解壓mtime／DTA四bytes與PNG名稱外保持，兩設定的四caller／核心、時計、音訊與IRQ7／VBE終態一致。兩初態刻意有／無既有受控滑鼠，現在有事件的mouse_started=1／mouse_completed=1，無事件mouse_started=0／mouse_completed=0；callback mask1／pending0／activefalse。首次稽核誤要求這兩計數相同，依已明示輸入與真實收據改為各自0／1，其餘全部被比較欄位仍嚴格一致，未修改CPU／平台／遊戲資料。完整座標／游標255與玩家路徑仍未知。

新停點兩組皆第48919460步、高位LE0x14E3DE，bytes66 81 F9 D4 00 0F 8C 45 FF FF FF E8 D0 04 0A 00，CMP CX,00D4h的word完整立即值尚未支援。R為FFFFFFFF／1／958／29BE7C／2BDBA4／2BDBD0／2600CD／2BDC2C，六段8／188／188／0／20／188，flags293h。EIP14E3E1只是fetch，非CMP成功；下一0F 8C的有號分支尚未執行，不猜遊戲欄位／helper用途。兩時計58965328，IRQ0 started8059／completed8059／failedfalse；IRQ7 started312／completed312，DMA完成312、剩269、credit95400、current46F3h／count090Ch。裝置irq7_deliveries313另含先前16位傳輸，不混為保護模式IRQ7完成數。

兩PNG SHA-256 2d0d564f814e49eab6081862f52467b1a7b232c026d3632fd2c7d97c42545e65，逐位元相同，已實際檢視原版Loading Master of Orion II載入畫面及中央游標；不再稱為全黑過場，不稱主選單或255游標exact。VBE Bank7／StartY512／BankSets731／Writes5696552／DisplaySets9。未設定日期第三流程的完整終端除mtime／DTA與PNG名稱外等於311未設定基線，在0x240A32拒絕且完全沒有word ADD觀測；PNG仍為黑色過場1610444d26adb3135e7e933dd44912044bb636728af59c70e614945278d3c622。

正式兩gzip workplace/moo2-probe-312-full-game.txt.gz SHA-256 35b61654a285c65c619fee17cd49c86720e2fc0def4d405c06552bdfe04d4fc9；workplace/moo2-probe-312-mouse-event.txt.gz 32274bc6d48f28dad1567c74ee515e3a7867f4a8c12b2764c830c79abe6ebb14；未設定workplace/moo2-probe-312-unconfigured.txt.gz d2d1475f15c94cccd43c012f98427571475c444548decff883f34a73aa7de904。輸入日期及AH2Ch／RNG近似、完整鍵盤、255／299自然OF=1、人耳、主選單／正常玩家流程、remake同狀態與主庫玩法RE閘門均保持。

## 解析回填

word ADD記憶體來源停點已由規格 312 接通。309-sb16-pause-resume-dma8.md、310-moo2-dos-calendar-date.md與311-cpu386-sub-word-register-imm16.md中0x210C7E同一固定EXE鍵的未知須同次追加此勘誤與本檔連結；歷史拒絕收據仍保留。不擴張309音訊、310日期或311其他word SUB語意。驗證入口apps/moo2/tools/startup_probe_131.py --check-add-word-memory-source-spec-backlinks，缺原始bytes／來源／flags／真正寫回／受控滑鼠0與1／收據／任一舊文件回填即拒絕。

同次54個回填函式與31個新增移除定位／狀態／正式收據／較早標記／連結負例、312 CLI全通過。工具來源與三組正式收據保持；所有新檔與收據UID:GID1000:1000，工具root-owned／異形.md目錄自檢空，本輪有界容器已退出移除。

## 2026-10-03 CMP完整立即值停點勘誤

word CMP完整立即值停點已由規格 313 接通，見[313-cpu386-cmp-word-register-imm16.md](313-cpu386-cmp-word-register-imm16.md)。固定EXE／高位LE0x14E3DE的六旗標與後續JL兩方向、兩正常外層212到訪已驗。原拒絕收據保留；新停點0x24C31B的INT33／AX0014h滑鼠服務，已見原版標題背景，主選單按鈕未見，其他CPU／日期／音訊／玩家路徑範圍未擴張。
