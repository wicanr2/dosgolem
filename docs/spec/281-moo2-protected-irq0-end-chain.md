# 281 — MOO2 保護模式 IRQ0 的預設結束鏈

狀態：**CONFORMED**
日期：2026-10-02
範圍：原版執行器已登錄的 DOS08h 預設入口、私有最外層返回框架；平台規格近似（hardware-spec approximation），不建立完整 DOS/4GW 核心或跨權限鏈。

## 原始定位與平台前提

已證實：工具基線 3e260dcf2215228d840b98242c1226ab7f902456，固定官方 1.31 ORION2.EXE SHA-256 4e11be14217b4aafa1839f333bf5eba037f98b0c44e9e4752c96c464c260419f、417 原檔與 MOX.SET，工具／映像與完整自然收據沿 [280-cpu386-far-ret32.md](280-cpu386-far-ret32.md)。兩個自然排程第 1,231,392 步，第六次 IRQ0 都已從高位 LE 線性 0x244E9C 的 CB 返回合成平台 CS:EIP 0108:00326008；SS:ESP=0110:00000FF4，flags=246h。此處是 dosgolem 私有平台入口，不是原版 LE 位址；原版的對應預設入口為 DOSBox-X CS:EIP 0080:00000D49。

[Open Watcom 1.9 Programmer’s Guide](https://open-watcom.github.io/open-watcom-1.9/pguide.html) 的 DOS/4GW 25h／35h、32-bit gates、chaining、getting handler：處理器必須 IRET／IRETD 或鏈至前任；35h 即使未掛其他同位元處理器也返回非空 dummy，看起來以 IRET 結束鏈。核心有私有堆疊，不能將框架視為被中斷玩家位置。這是公開平台介面，不是原版核心內部或逐週期證據。

## 候選型別、狀態與拒絕條件

- typed input 為附接 CPU、活動 IRQ0、目前 CS:EIP、SS:ESP、已配置的 default／stack 描述子、原始私有 12-byte 返回框架。只接受自身預設 DOS08h 的精確 selector／offset，描述子保持且入口 CF 可讀；其他預設 vector slot、未知鏈與巢狀框架拒絕。
- 保留規格 277 的最外層 IRETD 完整框架檢查：SS 與原始私有描述子保持、ESP=4096−12、框架每 byte 未污染。全部檢查通過後，先完成受限預設 BIOS08h 計數與 EOI，再標記已返回，由既有 dispatch 恢復全部外層 R／段／EIP／旗標／FPU 與完成計數。不逐條執行合成 CF，不根據遊戲地址猜寫返回或等待值。
- 預設 DOS08h 結束鏈會執行原先 BIOS 計時服務；本次原版邊界已觀察 BDA+1。按公開 BIOS 契約，046Ch dword 增加一次，達 1800B0h 歸零並增加 0470h byte，沿既有捨入／溢位模型；預設 INT1C 視為空回呼，客製者拒絕。先檢查 BDA 可寫範圍及原有／DPMI 實模式 08h／1Ch 均無客製鏈、保護模式 1Ch 為零或未污染預設入口，再增加計數、以既有共享 PIC 發出非指定 EOI（20h 埠寫 20h）。EOI 只沿現有優先序結束當下最高服務中位元；不在一般 IRETD 路徑自動 EOI，也不額外增加 IRQ Deliveries、消耗 Pending 或重派送。
- 已污染入口／描述子、錯誤 slot／堆疊／12-byte 框架、客製保護或實模式 INT1C、未知指令、有界未返回仍失敗即關閉。所有錯誤都恢復外層 CPU，但原版已執行的記憶體／埠輸出不倒帶，沿 277 的失敗契約。
- 垂直鏈為原版 AH35h 保存前任 → 原版 CB／尾鏈 → 平台 dummy → 返回 → 原版自然繼續；不是 Go remake 玩法／UI／存檔規格，不更改主庫玩法 RE 閘門。

## 驗收與停止線

先有限擷取原版 CB 後的 12-byte 中斷框架及公開 dummy 返回邊界，核心黑箱自然執行，只讀 BDA 與框架；不追 timer driver／ISR／busy-wait 或內部核心指令。公開契約及樣本審查後才 READY。

READY 後測合法尾鏈在兩 CPU 模式、完整外層／FPU 保持、原子污染拒絕、錯誤 slot、與既有 EOI／IRQ7／滑鼠／PIT 共存及全部固定 EXE 回歸。兩個原版自然排程由 dosgolem 自行重生，必須明列是否完成第六次返回、模式 2 等待、事件注入與下一停點；有無正常玩家路徑另外判定。281 的受限模型通過不能自動把 277 完整等待、255 座標／游標或整款 remake 標完成。

## 有限原版反例與 READY 審查

已證實：同固定原檔、DOSBox-X 2026.07.02 SDL2 重型除錯器／映像 sha256:659e8abbf93646f59a1586341769bd4b8f3cd1c707859d7a5de4c56e4672b582，startup_probe_131.py --irq0-end-chain 在 300 秒容器乾淨退出 0。原版從 0180:00378E9C 的 CB，SS:ESP=00D0:6830h，20 bytes：49 0D 00 00 80 00 00 00 16 0F 00 00 80 00 00 00 46 00 00 00。CB 後 0080:00000D49、ESP=6838h、旗標246h，其餘暫存器／段保持；餘下原始 12-byte 框架為 16 0F 00 00 80 00 00 00 46 00 00 00。

核心黑箱自然執行到該框架給出的 0080:00000F16，ESP=6844h，BDA 原始五 bytes 41 27 02 00 00→42 27 02 00 00，確實增加一次 tick。通用暫存器及 DS／ES／FS／GS 保持，SS=d0h→a8h、flags=246h；框架最後 dword 46h→246h。欄位順序 CS EIP EAX EBX ECX EDX ESI EDI EBP DS ES FS GS SS ESP EFLAGS；CB 後為 80 d49 22741 22741 0 3ebc74 470 3a2090 3ebbc0 188 188 0 20 d0 6838 246，返回邊界只改 EIP=f16、SS=a8、ESP=6844。這是核心邊界，並非已回到玩家主線；布局／旗標內部改寫不搬入受限橋接，完整外層保存／恢復仍沿 277 的明示現代近似。

DRAFT 早期「dummy 只完成返回，尚不增加 BIOS tick」候選被這份 BDA+1 的直接邊界證據否定；保留該形成原因，READY 契約現增加一次受限 BIOS08h。沒有為此反組譯遊戲 ISR／核心／driver；無近似逐週期聲明。原版 JSON／有限 CB LOG／終端 SHA-256：7959ede54826e0c4244ad9e3d25e8f9e5a49484caee477001cec26d98db386d1／394684f0e7201f309dd4c9bc5c90dd3d7ee4a64ac2ab0ec88bf3e1ae60b566b5／caf902af15b63504c87e82b9114086046cf7d974428026a3e692a3ea34e4b0ae，完整資料只留本機 workplace。

公開成熟模擬器的 [DOSBox-X BIOS INT8_Handler](https://dosbox-x.com/doxygen/html/bios_8cpp_source.html)（原始來源行 5374 起）定義 tick／午夜 rollover；[CB_IRQ0](https://dosbox-x.com/doxygen/html/callback_8cpp_source.html)（原始來源行 379 起）包含 INT1Ch、20h 非指定 EOI 與返回。這是公開平台參考，網站來源版本不冒稱與本機 binary 逐值一致；BDA 的有限原版變化另有以上固定輸入佐證。原版 EOI 埠序列未另擷取，EOI 選擇是平台規格近似；不深入硬體或遊戲 driver。

候選已審查為 READY：精確預設入口／完整私有框架先驗證，預設 BIOS tick 與 EOI 有平台來源及原版 BDA 證據。只擴充該受限鏈；一般最外層 IRETD、未知 slot／客製 INT1C 等仍遵守 277。CONFORMED 仍待平台全套、原版自然收據與跨規格回填驗證。


## 受限實作與 dosgolem 自行重生

限定 CONFORMED：自身 DOS08h 精確入口、原私有堆疊／描述子及 12-byte 框架全部通過才執行預設 BIOS 計時與非指定 EOI，再交回既有完整外層恢復。一般 IRETD 不因此自動 tick 或 EOI；未知 slot、污染、客製 INT1C／實模式鏈仍拒絕。完整核心布局／IF 改寫與原版 EOI 指令序列仍是明示近似，不因返回閉合宣稱逐週期或整段同狀態。

Go 1.24.13／映像 sha256:1a6d4452c65dea36aac2e2d606b01b4a029ec90cc1ae53890540ce6173ea77ac，GOMAXPROCS=2 go test -buildvcs=false ./internal/machine -run 'Test(ProtectedIRQ0|LEMouseCallback|BIOSClock|PIT0)' -count=1 -v 通過；收據 workplace/moo2-281-platform-tests.txt SHA-256 7a0fa28b388f9e66aa01d5781a3737db395e92b01ac05d2d28f6e1ca17fa7375。涵蓋兩 CPU 模式／完整外層與 FPU 恢復、每一框架 byte／堆疊／預設入口／客製鏈拒絕、BDA rollover、既有 IRQ7 非指定 EOI 優先、提前 EOI 與 pending 不重入。固定官方 EXE 全套 DOSGOLEM_MOO2_EXE=/tmp/game/ORION2.EXE go test -p 2 -buildvcs=false ./... -count=1 通過，workplace/full-test-281.txt SHA-256 e700f8b9eabd39261c6e1eb2519d4d0ba21b80e0c5b930fa9932753d681517b9。

依 277 的 50M 上限／分離 DOS／417 原檔與固定 MOX.SET 命令，兩個排程均自行完成 1,595 次原版 IRQ0 返回，started=completed=1595、active=false／failed=false。等待來源 **高位 LE DS:offset** 0188:00271148 原始四 bytes 變為 01 00 00 00，原版自行退出模式 2 等待，沒有注入等待值。事件條件在第 1,612,067 步實際注入 x=657／y=189／buttons=0；這是受控工具事件，仍不代表真實鍵鼠手感或 255 完整座標／游標消費已驗收。受限 277 的原版返回／模式 2 等待閘門因此閉合，277 限定 CONFORMED；完整跨權限鏈與正常玩家路徑另判。

兩條第 20,634,818 步停 **dosgolem 高位 LE 線性** 0x239B3A，bytes E6 43 EB 00 E4 40 88 C4 E4 40 86 C4 25 FF FF 00；EAX=0、OUT 43h 控制字 00h 尚未處理。此為標準 PIT 通道 0 計數鎖存的平台缺口，不是 IRQ0 返回失敗。CS=8、DS／ES／SS=188h、flags=246h；PIT Mode=2／Reload=5966／Generation=5、Micros=21092406、Deliveries=1615、Pending=false／InService=false。下一最小行動依公開 PIT count latch 契約建立窄規格；不拆遊戲 timer driver／ISR／busy-wait，不追硬體 wall-clock。

自然 gzip SHA-256 13d30875d8eb0f90c126705fdf996d560fe24ba627410d3bd12cad50adee0346／6a2373334cbd427b62d1415ec8eefae8cf7c8943b22492d61a3e0b711eb828b1。VBE BankSets=441／Writes=4046164／DisplaySets=6，最終 indexed SHA-256 7818f5542a0404157573be6cffc0e0c8e68ce3c0f5d17d07ccdd9313fb700baf、RGB 0b150fd32588b1daca5569992ebe559c0102c837306b1af4c44d35128ec58366、兩 PNG 1610444d26adb3135e7e933dd44912044bb636728af59c70e614945278d3c622 與已檢視黑圖相同，不重做目視。主選單／正常玩家操作、音效、受控亂數及 Go remake 玩法同狀態仍未完成，255 保持 READY。原版完整資料／終端／gzip／PNG 只留本機，公開庫只存自製工具與文字證據。
