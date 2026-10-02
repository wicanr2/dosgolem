# 309：DSP D0h／D4h 的 8 位元 DMA 暫停與恢復

狀態：**CONFORMED**
日期：2026-10-03
範圍：已啟動的 8 位元 DMA 平台契約，hardware-spec approximation；主庫玩法 RE 閘門不變。

## 原版停點與來源

工具基線00ad7c645b19b51a8697e2deae85d8a5019dd657。固定官方1.31 ORION2.EXE SHA-256 4e11be14217b4aafa1839f333bf5eba037f98b0c44e9e4752c96c464c260419f；417原檔、MOX.SET553bytes、ZIP／patch雜湊、Go1.24.13 linux/amd64與容器命令沿[307-moo2-protected-keyboard-irq1.md](307-moo2-protected-keyboard-irq1.md)。

已證實：兩組正常固定48000000步的controller Esc 01／81，都於外層48354467步、高位LE0x217AD8的原版IRQ7轉送，實模式1201:05DA第77步遇OUT 022Ch=D0h拒絕。實際IVT1201:0682，IRQ7 started303／completed302；先前20h EOI與22Eh確認已真正執行。當時VirtualMicros58057009，8位DMAActive／Auto／Stereo／FIFO=true、Signed=false、C6 20 FF 07，rate22050/1、block2048／剩餘2045、DMA1 current4803h／count07FCh／page1、sampleCredit461100。原始I/O／caller與兩gzip雜湊見307；不由driver反組譯推測硬體細節。

公開來源：[Creative Sound Blaster Series Hardware Programming Guide](https://www.ardent-tool.com/sound/Sound_Blaster_HW_Programming_Guide_1st.pdf)，印刷頁6-24、6-26，PDF頁109、111：D0h為單byte命令，停止8位DMA請求，適用single-cycle／auto-init；D4h恢復被D0h暫停的傳輸。頁6-27另定義D5h／D6h為16位控制，本切片不接受。公開契約已證實；分數信用、FIFO與1µs時計仍是近似，不稱原版wall-clock／逐波形或人耳確認。

## 候選契約與 READY 閘門

- DSP只在無待收參數、非reset且022Ch寫D0／D4時呼叫8位控制回呼；缺回呼、沒有active8位傳輸、未知命令明確拒絕。參數位置的D0／D4仍按既有命令解析，不能誤執行暫停。未啟動／已結束傳輸的行為未知，採拒絕而不猜補。
- 暫停獨立於active／auto：保留DMA地址／count／page／模式／遮罩、block／剩餘、格式／速率、sampleCredit、既有DSP／PIC IRQ與16位狀態。8位信用累積與資料讀取都停止；BIOS與裝置時間、其他裝置／16位傳輸保持正常前進。
- D4只取消pause，從相同位置與分數信用續行，不重載ring／block、不丟pending IRQ、不補暫停期間sample。已active傳輸重複D0／D4為明示冪等近似；新啟動與reset清pause，reset仍按既有取消契約。D0不能當D3 speaker切換或DA退出auto-init。
- 唯讀LEDeviceState補DMA8Paused；有界DMA8ControlLast記錄命令與前後狀態，計數僅供收據，不增加客體I/O／記憶體寫入或時鐘。程式專屬探針印出實際命令後原版IRQ7返回、caller與下一指令，不代執行或跳過ISR。
- 機器測試涵蓋DSP參數／reset／回呼拒絕，single-cycle與四種auto格式、兩模式混合時間、非整數信用／來源讀取／block及ring邊界，重複控制、IRQ／16位隔離、mask與取消。全套固定EXE通過後，同一417原檔、50M上限、正常48M Esc以及原有有／無滑鼠兩排程重生；核對D0前基線保持、真正D0、原版handler返回與caller／新停點才限定CONFORMED。

## 玩家鏈與停止線

本切片只補原版執行器的平台依賴。主庫規則、typed data、UI、存檔不改。D4原版自然使用若未出現仍未知；主選單／完整鍵盤／玩家路徑、255游標、受控亂數與整款remake另驗。不反組譯音訊driver／ISR／busy-wait或DAC／PIT／DMA硬體逐週期。原版EXE、RAM、PNG與完整收據只留忽略的workplace，僅提交自製程式與證據文件。

不可變鍵：固定EXE雜湊＋高位LE0x217AD8＋實模式IVT1201:0682／Stop1201:05DA／OUT 022Ch=D0h。閉合後須回填307並建立startup_probe_131.py護欄；舊歷史收據不重寫，其他早期DSP／VGA埠與停點不受影響。

## 證據審查

2026-10-03轉READY：原廠D0停止請求／D4恢復已足以處理這個active auto8停點。沿既有DMA寄存器與分數時計，保留pause中的位置與剩餘，沒有音訊時序RE依賴。idle控制未知採拒絕，其他命令與CPU不擴張。上述正常輸入／全套／回填驗收尚待實作完成，不先宣稱CONFORMED。

## 實作與正常原版續行

限定CONFORMED：DRAFT／READY審查後只改DSP D0／D4控制回呼、獨立pause旗標及兩個8位取樣閘門；CPU指令、鍵盤、IRQ轉送、Go remake玩法均不改。idle／其他位寬仍明確拒絕。四個控制測試涵蓋DSP完整／參數／reset／回呼拒絕、四種auto格式與三種CPU排程、single-cycle、來源不讀、分數信用／block／ring／mask／重複控制／IRQ隔離／取消。首次自製測試因未使用import編譯失敗；第二次誤把40h信用單位當41h單位。只修測試的import與獨立預期25,000,000，不改平台或放寬斷言，同容器契約乾淨重跑通過；兩失敗收據保留。

| 自製檔案 | SHA-256 |
| --- | --- |
| internal/machine/sb_dsp.go | 6cadc48f54ca161a02d60597b31b2388dc5fa502ec3372595beb97210cb092b3 |
| internal/machine/le_dma_clock.go | fe0678daa4f2719f982aa065ac34f56cc8bb932a342181a6a053ce7530e72831 |
| internal/machine/le_opl_ports.go | 92231ba12e7be40c3f834ddea441fbd650441bec19c7d6349725ea69744be822 |
| internal/machine/sb16_dma8_control_test.go | 1f31d0413472361d3f4bac88bfb9857f0e703ecfa3528be6544453969eddc2d7 |
| workplace/moo2-probe/main.go | 2d2194c5c3904794732e53ede125364aa5737fcf0f1ebc4db9e44d52b552841d |

golang:1.24-bookworm映像sha256:1a6d4452c65dea36aac2e2d606b01b4a029ec90cc1ae53890540ce6173ea77ac，Go1.24.13 linux/amd64；UID:GID1000:1000／network none／2GiB／2CPU／128pids，外層600秒，以唯讀ZIP與patch重新展開417檔案、固定EXE與MOX.SET。完整命令沿307的隔離掛載／解壓，收據與PNG改309；不啟用私有ISR／BIOS prototype、不加時鐘或修改原版狀態：

```text
GOMAXPROCS=2 GOCACHE=/tmp/go-cache DOSGOLEM_MOO2_EXE=/tmp/game/ORION2.EXE go test -p 2 -buildvcs=false ./... -count=1
DOSGOLEM_MOO2_HARDWARE_ESCAPE_AT_48000000=1 DOSGOLEM_MOO2_MAX_STEPS=50000000 DOSGOLEM_MOO2_SEPARATE_DOS=1 DOSGOLEM_MOO2_VBE_PNG=/src/workplace/moo2-vbe-309-full-game.png go run -buildvcs=false ./workplace/moo2-probe /tmp/game/ORION2.EXE --game-dir /tmp/game
DOSGOLEM_MOO2_HARDWARE_ESCAPE_AT_48000000=1 DOSGOLEM_MOO2_MAX_STEPS=50000000 DOSGOLEM_MOO2_SEPARATE_DOS=1 DOSGOLEM_MOO2_MOUSE_EVENT_AFTER_POSITION=1 DOSGOLEM_MOO2_VBE_PNG=/src/workplace/moo2-vbe-309-mouse-event.png go run -buildvcs=false ./workplace/moo2-probe /tmp/game/ORION2.EXE --game-dir /tmp/game
```

| 本機收據 | SHA-256 |
| --- | --- |
| workplace/moo2-309-dma8-tests-first.txt，未使用import失敗 | 42e391f419352053780c07fe59b6e8e8b5a8e767bdfd2a6a8eaaddfb2b363e96 |
| workplace/moo2-309-dma8-tests.txt，40h信用預期單位失敗 | fab7c122f489a40c39ae8c412d733afe42bca887d2a765842c915c523e2ace82 |
| workplace/moo2-309-dma8-tests-final.txt，四測試通過 | a2fca6c3cf8d5f374c8bf4171603a5123602828df0c65e270c1283bd010f585d |
| workplace/full-test-309.txt，全部CPU／固定EXE全套通過 | ad62917158674edccdd3df730566735e16845a757c1fc8ac5f3c5f7d61e52da0 |
| workplace/moo2-probe-309-full-game.txt.gz | a33b5da6a93a996cd1cf6653455c39605fc4aaad9e539cf0cefcfce5d6abe28a |
| workplace/moo2-probe-309-mouse-event.txt.gz | 07a99506c2c6789cabf3ada4d365431cf977aa37e5509114302bd28708faf25e |

已證實：兩正常排程在D0之前的完整逐列收據與307-normal-final保持，僅排除mtime／DTA四bytes與新加的DMA8Paused:false。原版48M Esc、97／77步IRQ1與原caller保持。第48354467步真正D0於VirtualMicros58057009只使DMA8Paused false→true；其餘完整LEDeviceState保持，current4803h／count07FCh、剩2045、credit461100、completed303。原版IRQ7實際102步到FFFF:FFF0、Returned=true，started303／completed303；InputR／OutputR、InputSeg／OutputSeg及flags216h完全保持，真正EOI／22E與OUT022C=D0均由原版執行。返回後VirtualMicros58057033而來源與信用仍保持；高位LEcaller0x217AD8真正續行到0x217ADF，EAX由03E7E07Dh變0000C2A6h，其他完整R／六段與flags216h保持。

| 外層步數／高位LE | 原版後續caller，完整其他R／六段保持 |
| --- | --- |
| 48354468／0x217ADF→0x217AE5 | 8B 15 8C 42 2A 00，MOV EDX,[2A428C]得到03E71038h，flags216h保持 |
| 48354469／0x217AE5→0x217AE7 | 01 C2，ADD EDX,EAX得到03E7D2DEh，flags216h→206h |
| 48354470／0x217AE7→0x217AEB | 6B 45 F8 0B，IMUL EAX,[EBP-8],0Bh得到00000D9Fh，flags206h保持 |

已證實：原版後續正常送D4。第48782970步返回高位LE0x2454B0，命令前VirtualMicros58507105，與D0相隔450096µs；DMA位置、剩餘、信用與完整其餘device狀態保持，只VirtualMicros不同、pause=true。D4只改pause=false，未重載。真正INT66實模式1201:016A執行93步成功返回FFFF:FFF0，封包AX0401h→0001h；完整原callerR為300 0 2 66 2BDA14 2BDA64 2B0E5A 2BDA2A、六段8 188 188 0 20 188、flags206h。返回時間58507126，21µs與保留461100信用獨立推算傳1個sample、current4804h／count07FBh、剩2044、credit387200。後三條實際MOV EBX,[EBP+14h]得到0、CMP EBX,0令flags246h與JZ成功到0x2454E7，完整其餘核心保持。D4已具這條自然使用收據，其他呼叫／idle情況仍未知。

兩自然第48796894步，高位LE0x240A32的CD 21因DOS AH2Ah尚未支援而停止；EAX002B2AA8h，下一EIP240A34，不能當作服務成功。IRQ0 started8022／completed8022／failed=false，IRQ7仍303／303。兩時計58553364；恢復後samples=floor(((58553364-58507105)×44100+461100)/1000000)=2040、餘credit483000，current4FFBh／count0004h／剩5，完全符合實際狀態。PCM快照65536是既有上限，不稱總播放sample。兩PNG均1610444d26adb3135e7e933dd44912044bb636728af59c70e614945278d3c622，與已檢視黑色過場逐位元相同；未見主選單。未受控骰序不列本輪驗收；1µs／步、FIFO、分數信用與停／續時點仍為hardware-spec approximation，不稱實機wall-clock／人耳或整款remake parity。

獨立收據審查逐項驗證完整前基線、僅pause的前後快照、102步IRQ7核心／I/O／真正返回、450096µs暫停來源保持、D4成功／三caller、恢復後2040samples公式與兩排程完整受驗狀態一致。全部固定EXE全套通過，309限定CONFORMED。下一最小切片為0x240A32的DOS INT21／AH2Ah公開日期服務契約，先確認可重播時計來源，不能猜當天日期、注入存檔或跳過呼叫。

## 解析回填

D0 DMA暫停停點已由規格 309 接通。303／305／306／307均須保留此標記與309-sb16-pause-resume-dma8.md連結；它們較早D0未知只是當時基線。不可變鍵仍為固定EXE＋高位LE0x217AD8＋實模式1201:05DA／OUT022C=D0，不擴張255／299／其他CPU或玩家路徑。驗證入口 apps/moo2/tools/startup_probe_131.py --check-dma8-control-spec-backlinks；缺原始定位、公開契約、102步真實返回、D4／caller／收據、限定範圍或舊回填必須拒絕。

全部51個現行回填函式與27個新增缺證據／較早標記或連結移除負例通過；309 CLI通過。既有308索引正對照與309入口同樣可查，受驗DSP／clock／ports／測試／探針SHA-256未變，CPU SHA-256 6dffa60e7bb5b5c15c1b669a1b683dd8a2c2eb379cdcc6fc92e5a5bdb5cca09b保持。檔案與輸出均UID:GID1000:1000；本批Go容器正常退出且已移除，未清理其他專案或映像。

## 日期服務後續

DOS AH2Ah日期停點已由規格 310 接通，見[310-moo2-dos-calendar-date.md](310-moo2-dos-calendar-date.md)。2026-10-03明示epoch1996-01-01的兩正常48M Esc排程真正日期返回／原始SUB與兩次堆疊寫回已驗；未設定日曆仍維持本規格原拒絕。新停點0x210C7E word ADD記憶體來源，時間／RNG與玩家流程邊界未擴張。
