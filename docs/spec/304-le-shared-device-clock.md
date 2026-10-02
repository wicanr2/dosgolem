# 304：兩種 CPU 模式共用的裝置時間

狀態：**READY**
日期：2026-10-02
範圍：修正dosgolem保護模式缺少DMA／音訊時計的接線；採既有1微秒／指令的硬體規格近似（hardware-spec approximation）。

## 證據與平台前提

基線90b9f4acb3c7e55829973c51a39fe12cb2129df3。[303-moo2-late-startup-platform-observation.md](303-moo2-late-startup-platform-observation.md)保存固定官方1.31 ORION2.EXE SHA-256 4e11be14217b4aafa1839f333bf5eba037f98b0c44e9e4752c96c464c260419f、417原檔、Go1.24.13映像與兩自然50M收據。已證實：高位LE0x2454AE／INT31的C6 20 FF 07在實模式成功返回，0x2454B0完整caller狀態保持；DMA1 page1／地址4000h／countFFFh／模式58h、DSP block2048、rate22050/1。BIOSMicros43985659到52095937，音訊VirtualMicros1375與DMA8剩2048／完成0／PCM0固定，保護模式hook沒有推進此裝置。

[Creative硬體程式指南](https://www.ardent-tool.com/sound/Sound_Blaster_HW_Programming_Guide_1st.pdf)頁3-5、3-11、6-14／6-15及6-23／6-24給出auto DMA按取樣率前進、每個DSP block中斷、來源確認／EOI／IRET契約。[296-sb16-c6-auto-init-dma.md](296-sb16-c6-auto-init-dma.md)已實作mono／stereo、40h／41h、原始sample、DSP block與DMA ring重載；CPU模式不能讓獨立裝置停住。本輪只接時間，不重證硬體或反編譯driver／ISR。

[Intel80386中斷程序](https://www.ardent-tool.com/CPU/docs/Intel/386/manuals/prref386/s09_06.htm)與[Open Watcom DOS/4GW中斷服務](https://open-watcom.github.io/open-watcom-1.9/pguide.html)說明保護／實模式各自框架與轉送。既有實模式IRQ7使用實際IVT；保護模式IRQ7的實際向量與轉送尚未驗，不能用實模式CPU.Interrupt操作保護模式暫存器，也不能丟掉pending繼續跑。

## 時間與失敗契約

- typed input：既有LEOPLPorts／LEMachine與CPU，使用原有DMA／DSP／PIC狀態。InstallLEBIOSClock的保護模式StepHook每次加一微秒裝置時間，BIOS時計亦一次；DPMI實模式各步仍各一次，不從外層再重複計同一實模式步。CPU來源不修改，既有hook原樣轉送。
- 將DMA8／16的取樣、來源驗界、block完成與pending更新拆成兩模式共用的advanceDMA，不另增時鐘。保留既有速率分數、每tick至多一sample、PCM容量、DMA遮罩／reset／地址／重載語意。裝置時間先加、BIOS其次、DMA再次，沿既有實模式順序。
- IF或PIC遮罩只阻擋中斷派送，不阻擋DMA取樣。IRQ7 pending、in-service及IRQ0服務中的既有互斥沿原模型；保護模式IRQ0橋接尚active時也暫不派送較低IRQ。實模式IRQ7邊界保持，不改IVT／stack／IF／來源確認／EOI模型。
- 首次符合派送條件的保護模式IRQ7明確停止並保存原始向量／裝置狀態。保留pending與DSP來源，不增加IRQ7Deliveries、不修改CPU、堆疊／遊戲資料，不將它記為CPU未知opcode；前一個真實sample和block完成保留。不默默抑制這個中斷來跑過等待。
- 失敗診斷保存完整CPU狀態、實際absolute IVT0F、DPMI實模式0F與DOS保護模式0F。它們是不同向量來源，原樣保留；本輪不選擇未證實轉送目標，也不命名caller原始欄位。

## 驗收與正常路徑影響

兩模式交錯執行真實指令，逐tick以總步數×rate×channels整除1000000獨立核對sample／原始byte／block／ring及BIOS／裝置時計只加一次。覆蓋四mode、5000／22050／45000Hz、40h含channels、DMA遮罩／reset、DMA16單word、來源超界、既有hook及IRQ0內部有界CPU步。另驗IF／PIC／in-service／IRQ0互斥、pending保持與未建模IRQ7停止時CPU／FPU／堆疊保持。

機器層／固定EXE全套通過後，使用303相同原始輸入與兩自然命令重播，只換304輸出名、不注入CPU／時計／資料／鍵盤或提高50M上限。獨立核對第一筆真實PCM來源、速率信用／第一block、完整原版呼叫邊界與下一停點；新IRQ7停止保留READY並另補派送規格，不能稱連續PCM／IRQ7或等待解除完成。這是原版oracle工具依賴；不改remake資料／規則／UI／save，不能取代正常玩家路徑驗收。

## 停止線與索引

索引見[000-index.md](000-index.md)。主庫RE閘門、255／299自然OF=1、人耳、主選單／正常鍵盤／受控亂數及整款remake限制保持。原版素材、完整終端／記憶體／gzip／PNG留本機，不公開原版資產。公有契約足以接兩模式時間；保護模式IRQ7仍須原始向量／輸入與明確轉送契約，禁止猜ISR或深入DAC／PIT／DMA逐週期考古。

## READY 審查

已由兩自然303的完整收據與五來源雜湊再核對，原版C6返回及音訊時鐘固定1375成立。公開取樣率／block與既有1µs近似足以接時間；保護模式IRQ7目標未知以明確停止保存，不猜轉送。輸入、順序、共享狀態、遮罩與pending、失敗、來源驗界及獨立驗收均已定義，304審查READY後才改平台來源。正常玩家流程與主選單仍未驗，整款remake不因本規格轉READY而完成。

## 自然首區塊與停止收據

2026-10-03驗證：304維持READY。兩自然均完成首個DSP block並停在尚未建模的保護模式IRQ7，平台時間接線已驗，派送鏈仍待補，不稱整款remake完成。

沿303的Go1.24.13映像／600秒／2GiB／2CPU／128pids／UID:GID1000:1000／network none與fresh417原檔、固定1.31 EXE，GOMAXPROCS=2與GOCACHE=/tmp/go-cache。相同自然命令只換304輸出：

```text
DOSGOLEM_MOO2_MAX_STEPS=50000000 DOSGOLEM_MOO2_SEPARATE_DOS=1 DOSGOLEM_MOO2_VBE_PNG=/src/workplace/moo2-vbe-304-full-game.png go run -buildvcs=false ./workplace/moo2-probe /tmp/game/ORION2.EXE --game-dir /tmp/game
DOSGOLEM_MOO2_MAX_STEPS=50000000 DOSGOLEM_MOO2_SEPARATE_DOS=1 DOSGOLEM_MOO2_MOUSE_EVENT_AFTER_POSITION=1 DOSGOLEM_MOO2_VBE_PNG=/src/workplace/moo2-vbe-304-mouse-event.png go run -buildvcs=false ./workplace/moo2-probe /tmp/game/ORION2.EXE --game-dir /tmp/game
```

已證實：兩次C6返回的outer_step42347255、高位LE0x2454B0完整R／段／flags206h及裝置狀態與303相同，只將VirtualMicros接到BIOSMicros43985659。既有C6 20 FF 07、rate22050/1、block2048、首block信用926100、DMA1物理14000h／4096byte ring保持。

第42356668步，高位LE0x257FC9首次待派送IRQ7，完整R依EAX ECX EDX EBX ESP EBP ESI EDI為120 325B80 3258C8 120 2BDB58 2BDB6C 325BA0 325CE1、段8 188 188 0 20 188、flags206h。兩時計都44032078，即C6返回後46419個工具微秒；獨立整除 (926100+46419×44100)／1000000=2048 samples、餘數4000。DMACompletions1／PCMBytes2048，current地址4800h／count7FFh，DSP block已重載2048而DMA ring尚未重載，DMA8SampleCredit4000。此前實模式21µs與本段合計首block46440µs，僅是硬體規格近似。

實際PCM2048個80h，SHA-256 88ed1a04cb43fe65827d1cd9ef6d24a736108730b1ce6315d4d3ca79b6a0d140，16byte prefix同80h，獨立以2048個80h核對雜湊，符合296已保存的原版靜音buffer來源。這只證明已傳輸的首block原始byte；不把靜音buffer或技術解碼當人耳驗收。

DSPIRQPending／PICPending=true、PICInService=false，保護模式CPU與堆疊保持，IRQ7Deliveries仍1，先前DMA16Completions1／PCM16Bytes2保持；新IRQ7沒有假稱成功派送。錯誤是明示平台缺件，不是未知CPU opcode。原始向量：absolute_ivt0f=12010682，DPMI實模式0F=0000:0000，DOS保護模式0F=0000:00000000。實際IVT段位移1201:0682，實模式線性0x12692；入口16bytes=2E FF 06 30 00 E8 5D FD 72 03 E9 95 00 E8 69 FD。只保留入口，不解driver／ISR內部；下一步核對公開DOS/4GW的這種實模式IRQ向保護模式執行的轉送契約。

IRQ0 active=false／failed=false／started6176／completed6176，BIOS Deliveries6196，等待原始DS:00271148=E6 11 00 00。兩PNG仍同較早已檢視的黑圖，因首次IRQ7在星空繪製前停止，不把它當新版星空功能消失的證據。尚未見主選單；302／303的星空與50M屬較早未推裝置時計基線，不能混作304終態。

### 驗證、來源與收據

機器層：go test -buildvcs=false ./internal/machine -count=1 -v；四mode／三rate與交錯真實指令、40h已含channels、遮罩／reset／來源超界、16位單word、IRQ0巢狀時計及IRQ7保留現場皆通過。初次新增fixture漏設實模式CS=0，核對cpu.Reset與匯流排後只修測試初態並驗bus.err，未改CPU／平台。失敗收據d54dfd73fd85f02a1da16cffa7406cd4f56651bff389c745fa0432e6f5fd4a14保存，同映像／命令重跑通過。

固定EXE全套：DOSGOLEM_MOO2_EXE=/tmp/game/ORION2.EXE go test -p 2 -buildvcs=false ./... -count=1，首次通過。CPU及啟動來源保持302版本；最後只補PCM雜湊與16原始入口bytes的唯讀診斷，平台來源未改。第一次自然已有相同時間／CPU／DMA／向量／停點但沒有PCM bytes收據，保留before-pcm-diagnostic；兩自然同命令重生，完整收據僅新增兩列、解壓mtime與DTA時間／日期四bytes，其他逐列相同，不重跑未變平台／CPU測試。

| 自製來源或本機收據 | SHA-256 |
| --- | --- |
| internal/cpu386/cpu.go | 72e5ee4954b0b4616e32b3b3844b6ee8836b210c0746cd4b8a20c0b9b5113326 |
| internal/machine/le_dma_clock.go | d7d232e1782d5a1e9009fa75146877da6067e2f2e78ea434504522e5ea497a84 |
| internal/machine/le_bios_clock.go | c2106a56e4a57f8c229dbc8671c91406393b4fe85e5c4c892910ba422c7a8b98 |
| internal/machine/le_startup.go | 1d7a36d5093995e5261bd6b1e555a17fd9d5ddcccb68ee479064eb92f5a50121 |
| internal/machine/le_shared_device_clock_test.go | 691f4af8919ef2b0b728ba072f2618c3723ef4a50bb83cc0691c979188e8fc54 |
| workplace/moo2-probe/main.go | ae685ca02a3ce4209ca6b4b632b50569ec231b1a842e1053675126d0215483b5 |
| workplace/moo2-304-machine-tests.txt | bff247d74ad4200f631ebd7edcccfebed4f72a47f325b7359396fc2a2d6d7c0e |
| workplace/full-test-304.txt | 163b0f2bfa8e8ad2b6efe1f831c7ab35173e1682f82dbf134f4b9fe63c1c79c2 |
| workplace/moo2-probe-304-full-game.txt.gz | ec4abf4f565e6cf1ea3ec1b210e414b459d00a6ac8e80da0307e3c323dcb3de0 |
| workplace/moo2-probe-304-mouse-event.txt.gz | d89716bc0d1482670ea1eb5cf1709475ef31bbeff0930cb86af34ef407369fda |
| full-game before-pcm-diagnostic gzip | b4cecb373d1818498d07e60798de9d5aead29bc818534d24a06e3214d915a4df |
| mouse-event before-pcm-diagnostic gzip | ee201deade48188ee2ab05c810ac362d8989d2c055a6999cbb36e929ad3ae553 |
| 兩張304 PNG | 1610444d26adb3135e7e933dd44912044bb636728af59c70e614945278d3c622 |

## 解析回填

不可變鍵：固定1.31 EXE雜湊＋高位LE0x2454AE／INT31／C6 20 FF 07＋LEOPLPorts.AdvanceRealMode／保護模式時計接線。241、293–298、300–303的十一份平台音訊邊界同次追加「保護模式裝置時計缺口由規格 304 接線」與本檔連結；只解時間到首block，保護模式IRQ7派送、連續PCM及人耳仍未知。299的自然OF=1、其他CPU／平台契約與245的16位單字檢測範圍不受影響。回填驗證入口 apps/moo2/tools/startup_probe_131.py --check-shared-device-clock-spec-backlinks，缺定位、兩時計／首block、真正PCM來源／IRQ7向量與pending／未完成限制或任一舊標記須拒絕。

304保持READY，首block與明確停止已驗，IRQ7正式轉送／返回仍未知。下一步以實際IVT1201:0682與公開DOS/4GW框架建立窄派送契約，保留CPU／共享裝置與來源確認、EOI／IRET；不翻譯driver、不猜正常鍵盤輸入或等待值，不提高50M上限。主庫玩法閘門不變，正常玩家路徑與完整remake未驗收。
