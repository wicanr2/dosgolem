# 305：MOO2 保護模式 IRQ7 的實模式轉送

狀態：**CONFORMED**
日期：2026-10-03

## 已有證據與範圍

沿用[304](304-le-shared-device-clock.md)的固定官方1.31 ORION2.EXE，SHA-256 4e11be14217b4aafa1839f333bf5eba037f98b0c44e9e4752c96c464c260419f。工具基線089f51f13149dd2a1b2c08c6b24ec639d0c78788、Go1.24.13 linux/amd64。兩自然在outer_step42356668、高位LE0x257FC9首次待派送IRQ7。已證實：實際absolute IVT0F為1201:0682，實模式線性0x12692；DPMI實模式0F與DOS保護模式0F皆未安裝。DSP與PIC pending保持，IRQ7Deliveries1是較早實模式偵測，不是新中斷成功派送。

[Open Watcom Programmer Guide](https://open-watcom.github.io/open-watcom-1.9/pguide.html)的DOS/4GW中斷服務說明：未安裝保護模式處理器時，預設向實模式轉送；核心先切換至自己的16位元堆疊。這是公開平台契約。以實際IVT進入原版handler、私有低位堆疊與有界返回，屬平台規格近似（hardware-spec approximation），不稱重現DOS/4GW核心或硬體逐週期。

處理器內部、DAC／PIT／DMA時序不作逆向目標。只觀察入口、來源確認、EOI、IRET與原版caller保持。remake玩法RE閘門不變，不修改Go/Ebitengine規則、UI或存檔。

## 可丟棄診斷

`workplace/moo2-probe/irq7_prototype.go`僅在明示DOSGOLEM_MOO2_IRQ7_PROTOTYPE=1、304原始停止收據與PNG已輸出後執行。它不恢復保護模式主迴圈，不作正式正常路徑驗收。

- 以scratch CPU呼叫既有DPMI0100配置256段落的私有4KiB低位堆疊。這會增加配置／descriptor；不宣稱整台機器沒有變動。
- 暫存器低16位繼承caller、EAX高16位沿現有實模式核心；DS／ES由可表示為段落的descriptor基址轉換。這個診斷輸入映射是假說，正式契約尚待審查。核心不能表示其他暫存器高16位，遇未支援形式就停止。
- 以FFFF:FFF0返回哨兵與既有AdvanceRealMode派送真實pending IRQ7。派送另加1工具微秒，明示記錄，不冒稱原304同狀態正式收據。既有CPU.Interrupt讀實際IVT並建立FLAGS／CS／IP框架。
- 最多20000條實模式指令。原樣轉送已支援I/O及DOS中斷；未知指令、埠、巢狀中斷、HLT、超時或錯誤堆疊皆記錄失敗。不跳過handler或假寫來源確認／EOI。
- 僅保存入口、前64筆I/O、最後16個實模式位址／bytes、返回或停止、裝置狀態與保護模式caller完整暫存器／段／旗標保持。實際IRET及SS:SP恢復才算診斷返回。

## 診斷前的未知與驗收

原版handler能否在既有CPU／服務模型返回、是否經DPMI回呼及正確正式寄存器契約尚未知，維持DRAFT。診斷不能直接進production path。正式接線須先審查READY契約，再驗遮罩／IF／優先序、來源確認與EOI、全caller／FPU／堆疊保持、失敗現場、私有堆疊生命週期與兩自然流程。原始bytes、PNG及gzip只存本機，不公開原版資產。入口由[000-index.md](000-index.md)收錄。

## 診斷證據與 READY 審查

上節保存診斷前的未知。兩診斷已完成，正式契約依本節。原版入口1201:0682皆執行73條實模式指令，實際I/O依序OUT0020=0B、IN0020=80、OUT0020=20、OUT0224=82、IN0225=01、IN022E=00；原版自身完成EOI與8位元來源確認。最後IRET實模式1201:0729、線性0x12739，回FFFF:FFF0且SS:SP恢復。沒有巢狀INT、未知CPU或埠。實模式輸入低位暫存器／DS／ES皆由原版恢復，保護模式caller保持。這僅證實明示診斷條件下的原版處理器邊界，不能冒稱核心寄存器映射exact或正常路徑已完成。

| 本機診斷收據／自製來源 | SHA-256 |
| --- | --- |
| moo2-probe-305-irq7-prototype-full-game.txt.gz | 76c1f898bed676883f4f5ee1a5c0ec0b60aade9131f935370c235fcd5f2014a0 |
| moo2-probe-305-irq7-prototype-mouse-event.txt.gz | be3bbb45ecc5032284183eeb1c2e2ad4dc7de86465264e380f07d547b114f73a |
| workplace/moo2-probe/irq7_prototype.go | 21c472fbf414c2d440d29edb1ee0df2499e88ebaf682db497ccf5b06fba24d6a |
| 診斷 main.go | fd2946cf47fad8f8d53ca7efb8970f1ba0db518a6686c2169aff812e23011dc1 |

診斷時間多1派送tick，44032078→44032152；3個sample，PCM2051，信用267400，符合(4000+74×44100)整除。正式轉送不額外加派送tick，由304已計的外層tick開始，處理器73步各加一次；不把診斷數值直接當正式驗收。

### 正式契約

- 僅接受現有MOO2橋接上下文、未安裝DOS保護模式0F與DPMI實模式0F的情況。實際IVT0F須非零且可讀。其他路由仍明確停止，保留pending，不猜向量優先序。
- 沿304既有IF／PIC mask／in-service／IRQ0 active互斥；通過後使用獨立實模式CPU，實際IVT與既有CPU.Interrupt。抽出既有實模式IRQ7派送，兩入口共用frame／pending／delivery更新；正式派送不重加時間。
- 私有4KiB堆疊由主機allocDOS配置一次並重用，不走client0100，不改client DOS block／dosLast或descriptor。每次檢查堆疊範圍。FFFF:FFF0僅為host返回哨兵，不注入guest stub或執行該位置。
- 實模式暫存器低16位與EAX高16位沿現有核心。DS／ES取可表示descriptor基址；無法表示則拒絕派送。此映射明標平台近似。外層完整32位暫存器／六段／EIP／EFLAGS／FPU另存並在成功或失敗恢復，原保護模式堆疊不作中斷frame。處理器真實寫入原版低位資料保留。
- 最多20000步，既有實模式每步共用時計。來源確認／EOI必須由原版I/O完成；IRET、SS:SP與PIC不再in-service才算返回。未確認的DSP來源可仍為pending，不偽造確認或刪新到達中斷。
- 記錄第一／最後一次轉送入口、步數、完整實模式輸入／輸出、最多64筆I/O、返回／錯誤及started／completed。未知指令、I/O、巢狀INT、HLT、超時或錯誤frame立即停止；原版停止CS:IP／暫存器及裝置狀態保留。失敗後橋接持續拒絕執行，不讓已清pending的失敗被略過。

### 驗收與限制

獨立自製實模式ISR fixture檢查真實入口／框架、22E確認與20h EOI、完整caller與FPU／保護模式堆疊保持、73步式時間分工、私有配置重用、不更動client記憶體 bookkeeping；覆蓋IF／遮罩／IRQ0互斥、無效／其他向量、不可表示段、未知CPU／I/O／INT、HLT、錯誤IRET／EOI、超時及持續失敗。全部機器層與固定EXE全套通過後，兩自然重生第一IRQ7與返回caller／下一指令、連續DMA／IRQ與下一停點，原始收據明確區分診斷。

公開轉送／私有堆疊契約、兩實際入口與返回／I/O證據已足以審查READY。正式實作不抄診斷配置0100或extra tick，不深挖ISR。核心暫存器映射、1µs指令時間是近似；後續回呼、keyboard IRQ1、主選單、正常玩家流程、人耳與整款remake完成仍未知。

## 正式自然驗證與 CONFORMED 範圍

狀態依本節更新為CONFORMED。固定EXE全套CPU／機器層及三組IRQ7測試通過。首輪fixture沿舊測試映像而未配置低位DOS arena，正確停在堆疊配置失敗；核對DPMI.setLimits後只修自製fixture的DOSArenaBase，production未調特例。同映像／命令重跑通過。最初診斷編譯因64pids與Go預設平行度耗盡程序，分類為工具資源設定；同映像以GOMAXPROCS=2／-p 2乾淨編譯通過。

兩正式自然沿304固定417原檔、MOX.SET553bytes、官方EXE／Go映像與600秒／2GiB／2CPU／128pids／UID:GID1000:1000／network none。命令僅換305輸出名，沒有PROTOTYPE旗標、DPMI封包、鍵盤或遊戲資料注入，不提高50M上限。原304首IRQ前全部收據逐列保持，只排除新唯讀IRQ7列、解壓mtime及DTA時間／日期四bytes。

outer_step42356668，實際caller高位LE0x257FC9，完整R=120 325B80 3258C8 120 2BDB58 2BDB6C 325BA0 325CE1、段8 188 188 0 20 188、flags206h。正式真實IVT1201:0682經73條指令、相同六筆I/O與IRET返回，兩時計44032151=44032078+73，不多加診斷的1tick；3個sample，PCM2051／信用223300。低位暫存器／DS／ES／flags206h皆返回相同。完整caller恢復後正常下一指令到高位LE0x257FCD、全部R／段保持、flags202h，沒有跳過玩家指令。started1／completed1、IRQ7Deliveries2，pending與in-service=false。

兩自然各完成14次保護模式IRQ7轉送與返回。outer_step42488059、高位LE0x247BE1的原始bytes34 01 C3停止，未支援XOR AL,1；Error後EIP0x247BE2。這是下一CPU缺件，非IRQ7失敗。終態兩時計44647204，PCM29175／DMACompletions14／信用60600、block剩1545、DMA ring current41F7h／countE08h。由C6返回信用926100加(44647204−43985659)×44100獨立整除得29175／60600，14個2048byte block及4096byte ring重載一致。IRQ7 started14／completed14、IRQ7Deliveries15含較早一次，最後原版仍73步並完成EOI／22E／IRET。IRQ0 started6221／completed6221、failed=false。

原版PCM29175bytes SHA-256 4e31e4b56ae8316bad6029ad1811a2eeb38146084444ae6795036d6e67090f56，僅16byte prefix為80h，不把整段稱全靜音或人耳驗收。兩終態除已授權滑鼠回呼0／1外相同。PNG仍黑圖，VBE索引byte有後續寫入，主選單未見；不把較早無裝置時間的302／303星空當本輪終態。等待因果及原始2A8E40欄位語意仍未知，不能因通過ISR猜補。

正式驗證命令：GOMAXPROCS=2 GOCACHE=/tmp/go-cache go test -p 2 -buildvcs=false ./internal/machine -run TestRealIRQ7 -count=1 -v；DOSGOLEM_MOO2_EXE=/tmp/game/ORION2.EXE go test -p 2 -buildvcs=false ./... -count=1。兩自然沿304完整命令只替換輸出編號；診斷另設DOSGOLEM_MOO2_IRQ7_PROTOTYPE=1並使用305-irq7-prototype輸出名。source／CPU版本範圍如下。

| 自製來源或本機正式收據 | SHA-256 |
| --- | --- |
| internal/cpu386/cpu.go，未修改 | 72e5ee4954b0b4616e32b3b3844b6ee8836b210c0746cd4b8a20c0b9b5113326 |
| internal/machine/le_real_irq7.go | 28892e5595eea7d446991016f1e13dc2209cd29490deedcb4a66658a6cc1c041 |
| internal/machine/le_real_irq7_test.go，最後僅校對測試訊息繁體字 | e679de16462cc5b290ecb9198743f4406bffc1951c79dbaedf876bcf830f22c8 |
| internal/machine/le_dma_clock.go | 2983811ba6e2ba443d10d51b4bc57de9d60f8cc5b8ca1e61a0653d1e29810f38 |
| internal/machine/le_opl_ports.go | ea46e3c74322a0fe36f7a38f4862e1af548076208438ac7cc0b5dd29baad0b5b |
| workplace/moo2-probe/main.go | 8dba5ccebe5621865cc6a8968ae6ef353ce91fb0f685a0f17ff64910d527d517 |
| workplace/moo2-305-irq7-tests-first-failure.txt | 773c1619a6061eb50312a1561caa1e817cb38a62f3f14c77f135500c0de2afc0 |
| workplace/moo2-305-irq7-tests.txt | b59cf5b53a0c2a46d9f0f3330805b934ce5e29d9bc4066dda162955a693afcd4 |
| workplace/full-test-305.txt | 2faef0cabd45efa370dc5bc791921d717c8ea3b2f5d0165b3e7256cbcf983b1e |
| workplace/moo2-probe-305-full-game.txt.gz | 47169dd3acc1613268ffa4a3cef6121c7b3a00009bd7b7388795a16d57fefbdf |
| workplace/moo2-probe-305-mouse-event.txt.gz | c4e9db17c3b29249867db6eb41eac9bc2744cd5f09e0e21ba122d12718e3f8a1 |

## 解析回填

不可變鍵：固定1.31 EXE雜湊＋高位LE0x257FC9＋absolute IVT0F1201:0682／實模式線性0x12692＋EOI／22E／IRET。241、293–298、300–304共十二份較早平台音訊邊界同次追加「保護模式IRQ7轉送由規格 305 接線」與本檔連結。只閉合公開passdown近似下的14次原版轉送／來源確認／返回與PCM傳輸；人耳、全部音訊／後續回呼、正常鍵盤IRQ1、主選單／正常玩家路徑、受控亂數及整款remake仍未知。255／299與其他CPU契約不受影響。回填入口apps/moo2/tools/startup_probe_131.py --check-irq7-passdown-spec-backlinks，缺原始定位、返回／I/O／時計／兩收據／限定範圍或任一舊標記須拒絕。

## XOR AL立即值停點已由規格 306 接通

[306-cpu386-xor-al-imm8.md](306-cpu386-xor-al-imm8.md)以固定EXE與高位LE0x247BE1的34 01 C3閉合三組AL／旗標、真正RET到0x231ADF及0x231AE2的MOV ESI,EAX消費。兩自然到50M無CPU拒絕，386次IRQ7返回與星空片段已驗，PCM記錄限65536byte。較早14次IRQ7及34停點屬305原始基線，未被覆寫；現行下一步為正常鍵盤IRQ1，不稱主選單／完整玩家路徑或人耳完成。
