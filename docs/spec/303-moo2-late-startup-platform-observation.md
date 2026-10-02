# 303：晚期啟動的平台狀態與正常輸入入口

狀態：**DRAFT**
日期：2026-10-02
範圍：固定MOO2 1.31自然啟動的有限唯讀觀測；不修改CPU、鍵鼠、DMA、IRQ或玩法行為。

## 原始定位與問題

工具基線9e6ee8cea7e40fdf13528fdae6b7959361708aaa。固定官方1.31 ORION2.EXE SHA-256 4e11be14217b4aafa1839f333bf5eba037f98b0c44e9e4752c96c464c260419f；417原檔、ZIP／patch／MOX.SET與自然命令沿 [302-cpu386-xchg-ax-word-register.md](302-cpu386-xchg-ax-word-register.md)。兩自然至50M上限、根CPU高位LE0x22FCD2，沒有CPU拒絕，IRQ0 started7789／completed7789，已見星空片段但未見主選單。32筆尾端中，高位LE0x231AE4比較DS:002A8E54，0x231AEB分支到0x22FCD2；欄位語意未知，不猜作鍵盤或音訊旗標。

已證實的程式接線：MOO2StartupDOS.AttachMachine安裝BIOS時計／受保護IRQ0／滑鼠回呼與LEOPLPorts，未安裝LEBIOSKeyboard。探針只可注入x657／y189／buttons0的既有滑鼠事件，沒有鍵盤注入。LEBIOSClock.advance增加BIOSClock.Micros，LEOPLPorts.virtualMicros與DMA進度只在AdvanceRealMode增加。這是靜態工具缺件線索，是否造成原版目前等待仍未知。

## 觀測契約

- 固定outer_step0／42347255／42603292／48000000及50M終態各一筆平台快照；49M之後三個caller邊界位址0x231AE4／0x231AEB／0x22FCD2各最多兩筆。StepHook原樣轉送既有hook，不改CPU／Bus／時計／橋接、不注入資料或按鍵、不改上限。
- 每筆保存完整R／段／flags、BIOS時計、既有LEDeviceState值快照、IRQ7派送數、PCM16長度、鍵盤是否安裝／讀取及既有BDA queue bytes、60h／61h／64h埠讀取數、實模式INT09向量及滑鼠回呼狀態。高位LE線性0x2A8E40–0x2A8E57只保存24個原始bytes與定位，不賦予推測名稱。
- CPU與平台服務來源保持302版本。兩自然終態完整R／段／flags、原三組XCHG／ROR、PNG／RGB／indexed及上限與302收據核對；差異先分類為觀測問題，不改正確期望。
- 綠色觀測只證明收據可重播。原版等待因果、正常鍵鼠的真正消費、保護模式DMA／IRQ7、主選單／玩家路徑仍未知；有充分公開契約與原版呼叫證據才另走READY實作，不在DRAFT猜補。

## 停止線與入口

本檔是303觀測入口，索引見 [000-index.md](000-index.md)。只查玩家可見的啟動阻塞與平台呼叫邊界，不反編譯helper／runtime／driver／ISR或硬體busy-wait。PCM／DAC／PIT／DMA時間採公開規格與既有近似，不求逐週期一致。主庫玩法RE閘門、255、299原版自然OF=1、人耳／受控亂數及整款remake限制保持。原版素材及完整終端／記憶體／gzip／PNG留本機。


## 自然觀測收據與證據等級

觀測完成，平台實作契約仍為DRAFT。兩次均由417原檔與固定官方1.31 EXE乾淨重建，沿302的Go1.24.13 linux/amd64與映像ID sha256:1a6d4452c65dea36aac2e2d606b01b4a029ec90cc1ae53890540ce6173ea77ac。容器限制600秒／2GiB／2CPU／128pids／UID:GID1000:1000／network none，GOMAXPROCS=2、GOCACHE=/tmp/go-cache。原版輸入只讀掛載，不注入CPU、IRQ、資料、亂數或按鍵；兩命令：

```text
DOSGOLEM_MOO2_MAX_STEPS=50000000 DOSGOLEM_MOO2_SEPARATE_DOS=1 DOSGOLEM_MOO2_VBE_PNG=/src/workplace/moo2-vbe-303-full-game.png go run -buildvcs=false ./workplace/moo2-probe /tmp/game/ORION2.EXE --game-dir /tmp/game
DOSGOLEM_MOO2_MAX_STEPS=50000000 DOSGOLEM_MOO2_SEPARATE_DOS=1 DOSGOLEM_MOO2_MOUSE_EVENT_AFTER_POSITION=1 DOSGOLEM_MOO2_VBE_PNG=/src/workplace/moo2-vbe-303-mouse-event.png go run -buildvcs=false ./workplace/moo2-probe /tmp/game/ORION2.EXE --game-dir /tmp/game
```

已證實的工具狀態：兩次各11筆有限快照，四個sample、六個caller及一個terminal，完整CPU／段／flags均保存。C6返回後DMA8已啟動，BIOS時計繼續前進，音訊時計、DMA8 block與取樣進度保持：

| outer_step | BIOSClock.Micros | LEDeviceState.VirtualMicros | DMABytesLeft／DMACompletions／PCMBytes |
| --- | --- | --- | --- |
| 42347255 | 43985659 | 1375 | 2048／0／0 |
| 42603292 | 44292269 | 1375 | 2048／0／0 |
| 48000000 | 49985912 | 1375 | 2048／0／0 |
| 50000000 | 52095937 | 1375 | 2048／0／0 |

DMAActive／Auto／Stereo／FIFO為true，22050/1 rate、block2048、sampleCredit926100亦保持。DMA16Completions1、PCM16Bytes2、IRQ7Deliveries1來自先前實模式檢測；不能宣稱整段未發生IRQ7。靜態接線與自然快照合起來確認「保護模式執行未推進既有音訊裝置時計」這項工具缺件。時鐘數字屬既有工具近似，不是硬體wall-clock對齊；原版目前等待是否因此造成仍未知。

已證實的輸入邊界：全部快照keyboard_installed=false，讀取／入隊數零，60h／61h／64h埠讀取零，BDA queue bytes=1E001E00，實模式INT09與absolute IVT09皆零。無事件滑鼠回呼started0／completed0；受控事件在第1612067步注入x657／y189／buttons0，started1／completed1，沒有鍵盤注入。原版302／303收據的DOS AH2509呼叫，高位LE0x239833，以DS=8、EDX=21C4D8h安裝保護模式IRQ1；此前高位LE0x239801的AH3509查詢回傳ES=108h、EBX=326009h。原版受保護IRQ1消費與鍵盤埠契約尚未接通，不用BIOS入隊代替它，也不追handler內部。

高位LE0x231AE4／0x231AEB／0x22FCD2各兩筆晚期快照的原始0x2A8E54–57皆零。這只確認已觀測的CMP／分支條件，欄位語意與等待因果未知。0x2A8E40在sample中為2／0，晚期caller與terminal為1，其餘原始bytes保留；不據此命名用途。

兩次完整收據與302逐列相同，排除新增11快照、輸出PNG檔名、MOX.SET解壓mtime及DOS DTA內時間／日期四bytes。檔案內容與尺寸未變；DTA其餘bytes／完整CPU初終態、三組XCHG／ROR消費、所有既有橋接與時鐘列均一致。兩次均到50M診斷上限，高位LE0x22FCD2、flags246h，IRQ0 started7789／completed7789，無CPU拒絕。無事件／受控事件unique_sites各17667／17697，沿302的正常輸入分支差異，未改期望。

### 雜湊與重播限制

| 來源或本機收據 | SHA-256 |
| --- | --- |
| internal/cpu386/cpu.go | 72e5ee4954b0b4616e32b3b3844b6ee8836b210c0746cd4b8a20c0b9b5113326 |
| internal/machine/le_startup.go | 1d7a36d5093995e5261bd6b1e555a17fd9d5ddcccb68ee479064eb92f5a50121 |
| internal/machine/le_bios_clock.go | e05d8c90e29321ca7e43d0b36758a778e3e4a095705c6333156d4d0ef123aba0 |
| internal/machine/le_dma_clock.go | 548df45c799012d6f553945f1aa7fee0a7854acd81e137562b892ea5e2b3350b |
| workplace/moo2-probe/main.go | 6fdc72fbf6a79e748a8936acce8993e6a6c6680fbf8adc776708aa01cc8dcd39 |
| workplace/moo2-probe-303-full-game.txt.gz | a4920940f3006218f9fdfb6d33085d7d7e18c4bc6095adffe8e31c3e62e23a28 |
| workplace/moo2-probe-303-mouse-event.txt.gz | c8f8517db2c90c06e9f109f8a9789fd392d7d430b941841ac18fca896f945db5 |
| 兩張moo2-vbe-303 PNG | d648932f847a2fe5b87723b6537f76e816d64fb60e05c13321d15ede50f3b21b |

CPU及三份平台來源與302保持相同雜湊。唯讀probe已由兩次自然編譯／執行與完整收據獨立比較驗證，不重跑未變的CPU全套。indexed／RGB雜湊保持302值，PNG同已檢視星空片段，主選單仍未見。收據、原始記憶體與PNG不公開；公開自製觀測程式、契約與有限文字證據。

下一步先以公開Sound Blaster／PIC契約及原版IRQ7向量呼叫邊界，審查保護模式與實模式共用裝置時間、避免重複推進及派送錯誤的窄平台規格。達READY才修改平台服務；本檔不證明等待解決、鍵盤正常消費、主選單、連續PCM、人耳或整款remake已完成。


## 共用裝置時間後續

保護模式裝置時計缺口由規格 304 接線，見[304-le-shared-device-clock.md](304-le-shared-device-clock.md)。2026-10-03兩自然首block真正PCM2048個80h／兩時計44032078已驗；第42356668步以absolute IVT1201:0682停在未建模的保護模式IRQ7，pending保留。只解時間到首block，IRQ7轉送／連續PCM、人耳及正常玩家路徑仍未知，其他原有證據與限制保持。

## 保護模式IRQ7轉送由規格 305 接線

[305-moo2-irq7-real-mode-passdown.md](305-moo2-irq7-real-mode-passdown.md)以固定1.31 EXE、實際IVT1201:0682／實模式線性0x12692及公開DOS/4GW契約閉合14次原版73步、EOI／22E／IRET；兩自然PCM29175及兩時計44647204已驗。較早IRQ7未派送／返回的記錄屬該舊基線，現行限定轉送依305。平台寄存器映射與1µs時鐘是近似，人耳、完整音訊、鍵盤IRQ1、主選單／正常玩家路徑與整款remake仍未知；其他CPU／255／299邊界保持。

正常Esc IRQ1已由規格 307 接線：[307](307-moo2-protected-keyboard-irq1.md)驗證正常controller的01／81、97／77步返回與原caller續行；完整鍵盤、主選單／玩家路徑仍未知，後續OUT022C=D0平台缺件另列。
