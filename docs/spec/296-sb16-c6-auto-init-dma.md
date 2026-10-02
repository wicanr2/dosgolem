# 296：SB16 C6h 的 8 位元自動初始化 DMA

狀態：**CONFORMED**
日期：2026-10-02
範圍：MOO2 啟動遇到的DSP C6h命令及公開平台DMA契約，hardware-spec approximation。

## 原始定位與公開平台前提

工具基線418ca3cf6d874e24127da24c0ba66e9fafecf6e7。固定官方1.31 ORION2.EXE SHA-256 4e11be14217b4aafa1839f333bf5eba037f98b0c44e9e4752c96c464c260419f，417唯讀原檔／ZIP與patch雜湊、命令及兩自然收據沿 [295-cpu386-or-dword-memory-register.md](295-cpu386-or-dword-memory-register.md)。已證實：兩自然第42,347,254步高位LE線性0x2454AE的DPMI AX0300h／BX0066h，實模式INT66入口1201:016A，230步停1201:05D9，OUT 022Ch／C6h未支援，Returned=false。後三byte與DMA／取樣率初態待有限診斷，不猜。

[Creative原廠硬體程式指南](https://www.ardent-tool.com/sound/Sound_Blaster_HW_Programming_Guide_1st.pdf)印刷頁6-23／6-24的Bxh／Cxh定義：C6h為8位D/A、自動初始化、FIFO開啟，後接mode、長度低／高byte；mode的bit5為立體聲、bit4為有號，其餘須0，長度是sample數減1。每個DSP block完成發IRQ，自動初始化重複資料傳輸，控制器DMA重載與DSP block邊界分開。FIFO只採平台近似，不重建FIFO／DAC／DMA逐週期。

## 候選契約與驗收

- 可丟棄C6參數診斷僅在明示環境旗標啟用，攔截首個C6及後三筆22Ch寫入；前3筆暫時接受以取得後續值，第4筆強制拒絕，不啟動DMA、不改原版資料，不作正式成功或玩家路徑收據。其餘埠與既有AdvanceRealMode原樣轉送。
- 正式命令最後一byte才啟動；公開語法可驗mode00／10／20／30與16位長度，啟動按已知DMA通道1／模式58h、完整位址／count／page、未遮罩及正取樣率驗證，未知／越界拒絕。具體接線範圍待初態審查。
- 沿既有8位auto DMA的byte地址、DSP block／控制器terminal count分離、IRQ8來源與22Eh確認；立體聲每秒傳2×rate個8位sample，有號／無號保留原始PCM byte與明示格式，reset取消。完成時間用公開sample數／rate與既有虛擬時鐘近似，不稱原版wall-clock或音訊聽感。
- 有效／拒絕／截短命令與啟動時點、兩種寬度IRQ隔離、mono／stereo取樣率、有號／無號原始byte、block與DMA重載／遮罩／reset／來源邊界測試。全部CPU／固定EXE全套、有無受控事件的兩自然完整命令／資料效果及原版caller後，才限定CONFORMED。

## 停止線

先審查READY才改平台實作。只追實際參數與caller，不反組譯driver／ISR／busy-wait、DAC／PIT／DMA硬體時序；原始素材與完整記憶體／終端／PNG留本機。主庫玩法RE閘門不變，255／主選單／正常玩家路徑、音效／受控亂數及整款remake另驗。

## 有限參數與 READY 審查

已證實，未改DSP／DMA的有限診斷：DSP SHA-256 5e890522ff7f84b613a0432b6507d8380797245174d53c5879de9532d171030e，DMA時鐘SHA-256 3532f6907e5edabf7b452ba421f58e3033e038c6cd0de1e219ad815ca42e96cc。首份診斷缺取樣率，唯讀LEDeviceState補既有rate／DMA known／active與計數後同命令重跑；不是修改平台行為。精確四byte C6 20 FF 07，即unsigned／stereo、DSP block2048個8位sample。第一DMA通道1已解除遮罩mask0Dh／mode58h，base=current地址4000h／count0FFFh、page01h，known[2／3]=3／page known=true，即物理14000h的4096byte ring，兩個DSP block才到控制器terminal count。DSP RateNumerator22050／RateDenominator1，TimeConstantKnown=false；8／16 DMA皆未執行、IRQ皆無pending，先前16位完成次數1、PCM8長度0。

診斷首C6到最後參數的VirtualMicros1273／1300／1327／1354；第4筆強制拒絕，Returned=false，實模式1201:05D9的步數311，不當自然成功。240秒／2GiB／2CPU／UID:GID1000:1000／network none／golang:1.24-bookworm，重新展開417原檔與固定EXE；命令 DOSGOLEM_MOO2_C6_PARAMETERS=1 DOSGOLEM_MOO2_MAX_STEPS=50000000 DOSGOLEM_MOO2_SEPARATE_DOS=1 go run -buildvcs=false ./workplace/moo2-probe /tmp/game/ORION2.EXE --game-dir /tmp/game。有限gzip SHA-256 74b7533097e8f22d193a467e4e29326079dffb78762fbd31d373e20c3710f22b；補rate前gzip4fdd40cb2fb61a0c8cb79bb5444ddb6142dca0122943fe8c3114eb49466d5cf9。探針只供蒐證，正式收據必須移除後由平台自行接受命令。

公開規格足夠，296轉READY才實作：C6的mode只接受00／10／20／30，保存完整16位長度，最後一byte呼叫專用8位auto DMA入口，啟動成功才發布block／格式。先驗DMA1完整known／解除遮罩／mode58h、有效rate且無另一筆DMA；DMA地址／count以既有8位平台邊界，來源讀取時驗範圍。保留一般auto DMA的mono／unsigned模型，C6才記錄FIFO／stereo／signed格式；PCM只為原始byte收據，格式快照只描述目前命令，不把混合歷史PCM當可播放音檔。

[同一本原廠指南](https://www.ardent-tool.com/sound/Sound_Blaster_HW_Programming_Guide_1st.pdf)頁6-14／6-15區分兩種取樣率：41h的rate是每channel Hz，stereo應2×rate byte/s；40h TimeConstant已包含channels，不再乘2。C6限定有效每channel5000–45000Hz；8位傳輸沿既有1µs／實模式指令與累積分數，每tick傳0或1個sample；DSP block與DMA ring重載分開。原版首block時間2048×1000000／44100，整數微秒向上取整46440；硬體規格近似，不稱原版實機時間。block完成設8位IRQ／PIC pending，22Eh確認只清8位來源，reset取消與清格式。

目前PCM只由AdvanceRealMode推進，保護模式StepHook只推BIOS／IRQ0；此既有邊界明示，不能宣稱保護模式連續音訊或IRQ7全路徑已完成。此切片先核對命令、原版實模式返回／caller與條件時鐘下的資料、block／IRQ／重載；若後續玩家等待依賴保護模式PCM／IRQ7，另以已證實caller與公開平台契約補閘門，不深挖ISR硬體時序。全CPU／固定EXE、兩自然實際命令與返回／caller、獨立sample數／duration及邊界回歸後才限定CONFORMED。


## 實作、條件時鐘與原版成功返回

限定CONFORMED：四種mode及全部65536個length語法、截短／拒絕／reset、DMA known／mask／模式／另一筆傳輸／取樣率／來源邊界已驗。條件AdvanceRealMode測試以獨立整除取樣數核對mono／stereo、41h每channel與40h已含channels、有號／無號原始byte。原版形狀的自製4096byte pattern在46439µs為2047個sample，46440µs完成首DSP block、地址4800h／count7FFh且不重載；92880µs完成第二block才重載4000h／FFFh。8／16位IRQ來源隔離、IF／遮罩／真實IVT派送／22Eh確認／EOI／IRET及reset通過。時間、FIFO與格式僅為hardware-spec approximation，不稱硬體逐週期或人耳驗收。

DRAFT／READY審查後只接專用StartSB16AutoDMA回呼、C6完整參數解析及既有8位DMA格式／速率；其他命令與CPU不改。實作SHA-256：

| 檔案 | SHA-256 |
| --- | --- |
| internal/machine/sb_dsp.go | 7ab177a0dede3929291ab84bcacc7f7f5d44dd28e36e6044534b1bb89ee3817b |
| internal/machine/le_dma_clock.go | 548df45c799012d6f553945f1aa7fee0a7854acd81e137562b892ea5e2b3350b |
| internal/machine/le_opl_ports.go | bea08afabe51229c68c5d8b1a8ea789720106e347d90de9e02ff11da75b75fa2 |
| internal/machine/sb16_c6_test.go | 2221a90652d1883f91df67d7f51b155049a1e07a2f7e13e7637198f481c80136 |
| workplace/moo2-probe/main.go | 6d795308a7f7dc25f8d260590bafbbaa1e0f7abd2b357e6d5bbc77bc12a08aea |

全部CPU／機器層首次通過：GOMAXPROCS=2 go test -buildvcs=false ./internal/cpu386 ./internal/machine -count=1 -v；本機moo2-296-cpu-tests.txt SHA-256 bdc4a5adc9db1fc42bb1ca13fccfb680c750146c7e4aa5feb3a1e23c128bfb2b。固定官方EXE全套首次通過：DOSGOLEM_MOO2_EXE=/tmp/game/ORION2.EXE go test -p 2 -buildvcs=false ./... -count=1；full-test-296.txt SHA-256 eaa30a613877d185e9ba095078f73c32bee44c18cdd61f00cdf5a3f9688545bd。最後僅改唯讀狀態與ring診斷，兩自然以同一容器命令重生，不改DSP／DMA或補推時鐘。

600秒／2GiB／2CPU／UID:GID1000:1000／network none／golang:1.24-bookworm／Go1.24.13，重新展開417原檔、固定1.31 EXE與MOX.SET。正式探針移除參數攔截器，保留RealModeIO／Bus原身分，未設定DOSGOLEM_MOO2_C6_PARAMETERS：

```text
DOSGOLEM_MOO2_MAX_STEPS=50000000 DOSGOLEM_MOO2_SEPARATE_DOS=1 DOSGOLEM_MOO2_VBE_PNG=/src/workplace/moo2-vbe-296-full-game.png go run -buildvcs=false ./workplace/moo2-probe /tmp/game/ORION2.EXE --game-dir /tmp/game
DOSGOLEM_MOO2_MAX_STEPS=50000000 DOSGOLEM_MOO2_SEPARATE_DOS=1 DOSGOLEM_MOO2_MOUSE_EVENT_AFTER_POSITION=1 DOSGOLEM_MOO2_VBE_PNG=/src/workplace/moo2-vbe-296-mouse-event.png go run -buildvcs=false ./workplace/moo2-probe /tmp/game/ORION2.EXE --game-dir /tmp/game
```

已證實：兩自然高位LE 0x2454AE都接受完整C6 20 FF 07。實模式INT66從1201:016A前進333步，Stop FFFF:FFF0、Returned=true、Error空；DPMI未實作表空。完整外層R依EAX ECX EDX EBX ESP EBP ESI EDI為300 0 2 66 2BDA90 2BDAE0 2B5D50 2BDAA6、段8 188 188 0 20 188、flags206h，呼叫前後保持。50byte DPMI封包僅offset29由04h→00h，AX0401h→0001h，不猜其他欄位。

呼叫前第一DMA仍未設定，VirtualMicros1042；handler內才寫DMA1地址4000h／count0FFFh／page1／mode58h／mask0Dh，known[2／3]=3、page known=true。返回VirtualMicros1375，DMAActive／Auto／Stereo／FIFO=true、Signed=false、block及剩餘2048；rate22050/1、TimeConstantKnown=false。最後參數1354到返回1375為21µs，獨立21×44100=926100個信用單位，小於1000000，PCMBytes=0、DMA完成0、無8位IRQ，地址／count未前進；前一16位完成仍1。不能把來源快照當第一個已傳送sample。

來源診斷明示source_timing=after_real_mode_return，以返回後DMABase／page擷取物理14000h的4096byte ring。全ring為80h，SHA-256 78aacbc3fb34efb8ffa5467b931291ec2bdf5e19564fc45fe97b5affbc893dc6，16byte prefix同80h。第一次正式診斷錯用呼叫前尚未設定的current=0／count=0，只描述地址0的1byte，不當C6來源證據；原收據保留before-dma-ring-diagnostic，兩gzip SHA-256 fdbdbc4af23e971a42a6981be6478be9889babfb2bd5a4a0470f8901042e497c／b46a26cbede40dad157635fcdbb7f343df79a4cc0e100edcdb2a8c0c743d0fab。修正唯讀診斷後兩自然的執行步數／返回／caller與新停點不變。實際PCM傳輸只由自製pattern的條件時鐘驗證，原版保護模式連續PCM／IRQ7仍未知。

| 步數／高位LE線性 | 原版caller |
| --- | --- |
| 42,347,255／0x2454B0 | INT31後完整R／段／flags206h保持 |
| 42,347,256／0x2454B3 | MOV EBX,[EBP+14h]讀取成功值0，完整其他R／段／flags保持 |
| 42,347,257／0x2454B6 | CMP EBX,0令flags246h，R／段保持 |
| 42,347,258／0x2454E7 | JZ +2F自然跳入成功分支，完整R／段／flags保持 |

四筆保護模式caller的DMA狀態及信用保持，沒有診斷器額外推PCM時鐘。有效兩gzip SHA-256 51a438fd826c55922c2115a1c77918be0d313ef14cf0e6c5b06c72a8689b8f42／a05791a08f8ee36df2cbc244076ffb005fed3b31cafa054291d19f1cc82272c9。兩自然第42,347,639步轉停高位LE0x257662，bytes C1 CA 10 A2 C0 26 27 00 66 8B C2 C1 CA 10 39 11；dword ROR立即數10h未支援。IRQ0 started=completed=6173、active=false／failed=false，等待值E3 11 00 00即4579；PIT mode2／Reload5966／Generation5，BIOSClock Micros43986044／Deliveries6193／Pending=false／InService=false。

VBE Bank7／StartY512／BankSets447／Writes4353364／DisplaySets7未變，兩PNG同已檢視黑圖SHA-256 1610444d26adb3135e7e933dd44912044bb636728af59c70e614945278d3c622。受控事件仍在第1612067步注入x657／y189。255座標／游標、主選單／正常玩家路徑、保護模式音訊／IRQ7、人耳與受控亂數及整款remake未驗收；本規格只閉合命令、啟動、原版返回／成功caller與條件實模式DMA模型。

## 解析回填

不可變鍵：固定官方EXE雜湊＋高位LE0x2454AE的INT31/0300h／BX0066h＋實模式1201:05D9的OUT 022Ch／C6h。293／294／295必須保留「SB16 C6h 停點已由規格 296 接通」及本檔連結；它們舊收據的歷史停點不重寫。245的B0單次16位範圍不受影響，241／243／244的同caller早期其他埠／命令停點不受影響，262／263的03C6h為VGA埠、不同鍵，不回填為DSP。驗證入口 apps/moo2/tools/startup_probe_131.py --check-sb16-c6-spec-backlinks，核對命令／原始定位／返回／caller、46440µs／TimeConstant界限與舊回填；缺定位、證據或舊標記須拒絕。
