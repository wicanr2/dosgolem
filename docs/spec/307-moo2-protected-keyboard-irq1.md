# 307：MOO2 硬體鍵盤與保護模式 IRQ1

狀態：**CONFORMED**
日期：2026-10-03
範圍：原版輸入的有界平台入口；主庫玩法 RE 閘門不變。

## 已有證據與公開契約

固定官方1.31 ORION2.EXE SHA-256 4e11be14217b4aafa1839f333bf5eba037f98b0c44e9e4752c96c464c260419f；417原檔／MOX.SET553bytes、Go1.24.13 linux/amd64與50M正常無鍵盤基線沿[306](306-cpu386-xor-al-imm8.md)。工具基線4eb97f6277121e528a2d3ce594dbb1967d16382e。

已證實：dosgolem高位LE0x239801的AH3509返回108:326009；0x239833的AH2509成功安裝DS=8、EDX=21C4D8。此為原始定位，尚未證明原版處理器返回、遊戲欄位語意或正常玩家操作。

公開平台：[OpenWatcom Programmer’s Guide](https://open-watcom.github.io/open-watcom-1.9/pguide.html)「Hardware Interrupts」與「Interrupt Handling」說明AH25安裝的08h至2Eh處理器可接兩模式轉送、核心私有堆疊與IRET返回。[IBM PC/AT Technical Reference 1502243 MAR84](https://www.minuszerodegrees.net/manuals/IBM_5170_Technical_Reference_1502243_MAR84.pdf)印刷頁1-11、1-38至1-41明定鍵盤輸出滿連到IRQ1、64h bit0及讀60h清除。診斷的已初始化64h bit2、61h初值0與PC相容01／81輸入是明示hardware-spec approximation，尚未證明完整8042、掃描轉譯或週期時序。

## 可丟棄診斷

入口workplace/moo2-probe/irq1_prototype.go；DOSGOLEM_MOO2_IRQ1_PROTOTYPE=1僅在正式50M終態／PNG之後執行。不恢復主迴圈，不稱正常輸入收據。從實際AH25捕捉目標，借用原CPU／既有clock hook執行原始處理器，每碼最多20000步；私有0501配置與額外descriptor均為診斷副作用。保存並恢復完整R／六段／EIP／旗標／FPU及三hook。

唯一override是單byte60h、61h暫存與64h output-full；其他I/O沿既有裝置。讀取、EOI、最外層原始CF與12byte框架均需實際驗證；遇預設鏈、未知指令／埠、超時或框架差異停止。記錄入口、最後16步、最多64I/O及最多64個原始記憶體差異。遊戲欄位不猜名稱，不反組譯ISR內部。

## 初始DRAFT閘門

初始DRAFT時未知：原版handler能否完整讀取／EOI／IRETD，兩碼原始寫回、正式隊列／PIC優先序、兩模式派送及正常主選單／玩家消費。診斷未通過前不寫正式鍵盤橋接。正式驗收須有公開PIC／controller邊界、有效向量／IF／遮罩／服務中閘門、錯誤停止與核心保持，然後從正常外層步入隊、自然送到原版IRQ1。BIOS直接入隊與測試座標注入不能代替。

原始EXE／RAM／PNG及執行收據只留本機workplace，不提交；本檔與自製觀測程式可公開。


## 已定位的預設鏈與第二段診斷

兩307初次診斷在0x21C4EE的FF 1D DC 42 2A 00停止。此CPU缺件由[308](308-cpu386-call-far-indirect-absolute.md)接線。308兩次同基線診斷確認實際DS:2A42DC六byte 09 60 32 00 08 01，遠CALL寫回8byte F4 C4 21 00 08 00 00 00，前一PUSHFD的12 00 00 00在後；實際抵達108:326009預設CF stub。第14步明確停止，沒有I/O與遊戲RAM寫入；此308邊界當時兩碼返回仍未知，後續公開BIOS診斷另列。

可選DOSGOLEM_MOO2_IRQ1_BIOS_PROTOTYPE=1僅擴充此可丟棄診斷的預設鏈，不能成為production。公開來源[IBM PS/2 and PC BIOS Interface Technical Reference Apr87](https://bitsavers.trailing-edge.com/pdf/ibm/pc/ps2/PS2_and_PC_BIOS_Interface_Technical_Reference_Apr87.pdf)印刷頁2-8至2-9記載INT09讀60h、按下的ASCII／scan pair入40:1E環形緩衝區、EOI；頁4-24鍵表用於01h／1Bh。診斷只接受無Shift／Ctrl／Alt的01／81，從真正遠CALL框架驗證12byte，讀controller鎖住的碼，再走既有BIOS BDA緩衝區；放開不加入字元。處理後只還原巢狀EIP／CS／flags與ESP+12，讓原版wrapper繼續。這是hardware-spec approximation，未稱原版BIOS硬體時序／副作用exact，原版遊戲欄位保持由原始指令寫入。


## 證據審查與正式契約

狀態審查：**READY**。兩有／無既有滑鼠事件的BIOS診斷均返回原始高位LE0x21C573的CF。01h為97步、81h為77步；實際default鏈60h讀01／81、20h EOI，nested返回21C4F4。按下僅原始RAM差異41E／41F為1B／01、2A42AC／AD為1B／01、2A42E2／E4為01；放開無新差異。遊戲欄位不附推測名稱。gzip SHA-256 844621a4b6b698333a16ea127c4ea83b3dc77ffab3fc153e034147b5aa22bb81／0bf4410164d0a8c761f7a4e198963af2d74c8416722ea418b8ac069b1faecc56。首次BIOS診斷誤用Enqueue的error型別而編譯失敗，只修自製探針，同容器契約／同自然命令乾淨重跑；不列原版缺陷。

正式入口internal/machine/le_hardware_keyboard_irq1.go與MOO2StartupDOS.QueueHardwareScan，現階段僅接受01／81，最多128個隊列byte。01／81是Esc的PC相容輸入，不支援修飾／擴充鍵；其他碼與64h控制器命令明確拒絕。此限制不代表完整鍵盤完成。

- controller有獨立output byte／full與PIC request／in-service，讀64h只觀察、讀60h清full，IN20h依0A／0B讀IRQ1的IRR／ISR bit1。只有實際20h非指定EOI清服務中，優先序IRQ0、IRQ1、IRQ7。[Intel 8259A 231468-003](https://www.pcjs.org/documents/datasheets/intel/INTEL_8259A_PIC.pdf)頁15–16為優先／EOI／遮罩來源。IRQ1待處理不因IF或遮罩關閉而丟失。
- 正式host配置4KiB私有保護模式堆疊，重用且不增加client0501block。只讀實際AH2509向量；沒有或恢復預設鏈時不把輸入注入遊戲。客製IVT／DPMI09、未建模default／未知CS、改變私有descriptor、未知指令／埠／框架／超時均停止。
- IF、IRQ1 mask、IRQ0服務中／active、IRQ1 active及實模式IRQ7 active限制派送。兩CPU模式在同一裝置時間入口呼叫同一protected IRQ1橋接，realCPU保持，protected caller全R／六段／EIP／flags／FPU在成功或失敗均恢復。同一ISR不重入；IRQ1服務中阻止IRQ7。暫存61h與已初始化64h、1µs／步及bridge內限制巢狀派送是明示平台近似，不稱週期或DOS/4GW核心exact。
- default09服務只在真正原版遠CALL的私有12byte框架抵達既有default CF時啟動，先驗證完整BDA環形隊列、零修飾狀態與返回框架，再讀60h，按下寫標準011Bh、放開不寫字元，EOI後還原真實nested返回地址／CS／flags／ESP+12。沒有跳過遊戲wrapper或猜補它的記憶體。最外層原始CF、已讀60h、EOI與不變host框架為返回必要條件。
- 欄位level只稱公開契約hardware-spec approximation；原始handler由CPU執行的入口／出口／寫回為已證實有限實驗。原版輸入與影像留本機，正式Go remake無修改。

驗收：自製ISR讀60h／寫共享byte／EOI／外層CF，完整caller與FPU／原堆疊／host重用、IF／mask／IRQ0／active閘門／隊列容量與非法碼、兩CPU模式／PIC優先與失敗持續停止；公開default09服務／nested框架另驗。固定原檔全套再兩正常啟動，固定外層第48000000步排01／81，不修改CPU或遊戲狀態，記錄實際IRQ1返回及原caller下一指令／後續畫面。固定排程先訂下，不換鍵或重擲到結果通過。主選單、後續玩家路徑／存檔／亂數仍須獨立驗證。

FF 1D遠呼叫停點已由規格 308 接通：[308](308-cpu386-call-far-indirect-absolute.md)保留原始診斷與真正frame。


## 正式正常輸入驗證

狀態：限定**CONFORMED**，只涵蓋無修飾Esc的01／81、兩CPU模式平台橋接與此固定正常排程，不涵蓋完整鍵盤／主選單／玩法。

GOMAXPROCS=2 GOCACHE=/tmp/go-cache go test -p 2 -buildvcs=false ./internal/machine -run TestHardwareIRQ1 -count=1 -v通過；全部CPU／機器層與固定EXE的go test -p 2 -buildvcs=false ./... -count=1通過。首次PIC測試發現IRQ1 bit插到DMA讀取分支而未接到20h，依同公開契約修正讀取位置後同命令重跑通過，保留637bc622f6339c39b5d3fec21430f88d6e4256bede2d2dee9fc184270d546a16失敗收據。較高優先IRQ0真實巢狀、私有frame／descriptor污染與失敗停止另通過；IRQ1自己的step不處理IRQ0私有堆疊。DRAFT BIOS編譯失敗收據46fbf8754208d46040516c30c0cff93c02a3802d5f12e4eb49e618300307d032保留。

兩固定自然，DOSGOLEM_MOO2_HARDWARE_ESCAPE_AT_48000000=1，以[305](305-moo2-irq7-real-mode-passdown.md)完整Docker／乾淨417檔／固定EXE命令換307-normal-final輸出名，最大步數仍50M。第48000000步先排controller 01／81；此前無鍵盤基線逐列沿308保持，只有解壓mtime／DTA四bytes排除。原始handler由8:21C4D8開始，01／81分別97／77步抵達原始CF 8:21C573，真實60h讀取與20h EOI各一次，started2／completed2。原版wrapper真實寫2A42AC／AD為1B／01、2A42E2／E4為01，沒有宿主注入遊戲欄位。

正常caller未跳過：48000000步高位LE0x215880的ADD EAX,EDX，輸入EAX=2C00D4、EDX=D4、flags206h；IRQ1返回後實際得到2C01A8／flags202h、EIP215882。48000001步高位LE0x215882的MOV AX,[EAX]，下一碼返回後原指令得到2CFFFF、EIP0x215885、flags202h。兩步完整其他R／六段保持；兩原版軌跡的IRQ1及後續終態一致，既有滑鼠回呼0／1分列。

第48354467步，外層高位LE0x217AD8，IRQ7實模式1201:05DA第77步，真正OUT022C=D0拒絕。IRQ7 started303／completed302／failed後持續拒絕，先前已實際EOI／22E；IRQ0 started7927／completed7927、failed=false。兩時計58057009，DMA完成303、block剩2045／信用461100，正常60h讀兩次、61／64零次、BIOS入隊一次且wrapper已清BDA head／tail為1E。這是新的音訊平台缺件，不是鍵盤或CPU拒絕；DPMI RealModeLast是較早已返回封包，不拿它冒稱D0新入口。下一步只依公開DSP D0暫停8位DMA契約，保留原版I/O邊界，不深挖driver／ISR／DAC時序。

兩PNG SHA-256 1610444d26adb3135e7e933dd44912044bb636728af59c70e614945278d3c622，實際檢視為黑色過場；VBE indexed SHA-256 2331f0dbc616c56c2582e5309a55b4bdc5989f56dfcaa101ffabb8d2faebf5fa、RGB 0b150fd32588b1daca5569992ebe559c0102c837306b1af4c44d35128ec58366，Bank2／StartY0／DisplaySets8。未見主選單，不宣稱Esc已完成整段過場或正常玩家驗收。

| 自製來源或本機收據 | SHA-256 |
| --- | --- |
| internal/machine/le_hardware_keyboard_irq1.go | 27965d57295352847ef76f0d2c3324c816593a7ad9f74cd60204e9ed6e5aa06c |
| internal/machine/le_hardware_keyboard_irq1_test.go | 09db527c22638c7807bea6df32e02d1ca193281abc460544c4b438831af9bac7 |
| internal/machine/le_opl_ports.go | 0acd20649aab1dfc1c0d05afc926a83a74755f5097f2fcd25b9632b205bd2018 |
| internal/machine/le_dma_clock.go | dd909a7bdd543753c8a42285436fbfecb5f864d5be0a4aaaef50468e77d06ffd |
| internal/machine/le_startup.go | 166b6c80688a2e69359ce9e467d05817cf8521a26f025baad33c337046881224 |
| workplace/moo2-probe/main.go | e6043d0f9d9585b50e4835245b5f27205d6e91280c7c1f1dc8d49ef824c05329 |
| workplace/moo2-probe/irq1_prototype.go | ab623d301e1507a35e8a2c05ee81769d4dc88df87c6c184a8bf2f33befe82a0d |
| workplace/moo2-307-hardware-irq1-tests-final.txt | d3d8bfd194725a9ce0154893df91ceadbf9c85638a5cb91c2ccedbc904260903 |
| workplace/full-test-307-normal-final.txt | 4a9eb936bc0c33fe18376e11297e806d994421c44831bcbbd5d070b88b18900a |
| workplace/moo2-probe-307-normal-final-full-game.txt.gz | 9a8aad65b88d1748431eb8ddfb17733a7a342cf255f4f0147db6a4b49a957753 |
| workplace/moo2-probe-307-normal-final-mouse-event.txt.gz | 430349cf9d921862d40f73bce9d2f0e65bb9a36a571ed4b51072ba1688801e59 |

## 解析回填

不可變鍵：固定1.31 EXE雜湊＋高位LE0x239833的AH2509＋8:21C4D8。303、305、306追加「正常Esc IRQ1已由規格 307 接線」與本檔連結；較早未知限定回到當時，其他鍵、等待欄位、255／299自然OF=1、人耳／主選單／玩家流程仍未知。入口apps/moo2/tools/startup_probe_131.py --check-hardware-keyboard-spec-backlinks，缺實際向量／輸入排程／讀取EOI返回／caller正常續行／兩收據／平台近似及舊標記即拒絕。
