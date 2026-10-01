# 277 — MOO2 DOS/4GW 保護模式向量與 IRQ0 派送

狀態：**READY**
日期：2026-10-01
範圍：DOS/4GW 的 25h／35h 保護模式向量、共享 PIT IRQ0 與受限中斷返回橋接；硬體／extender 規格近似（hardware-spec approximation）。只修原版工具平台，不改 Go remake 玩法、原版計時驅動程式或等待來源，不追 ISR／忙碌等待內部。

## 已證實的原始定位與缺口

固定官方 1.31 ORION2.EXE SHA-256 `4e11be14217b4aafa1839f333bf5eba037f98b0c44e9e4752c96c464c260419f`，417 原檔及 MOX.SET 沿 [276-pit0-mode2-shared-clock.md](276-pit0-mode2-shared-clock.md)。Go 1.24.13、映像 `sha256:1a6d4452c65dea36aac2e2d606b01b4a029ec90cc1ae53890540ce6173ea77ac`，自然 20,120,000 步只讀向量觀測：**dosgolem 高位 LE 線性** `0x24501B` 的 AH=3508h 返回 ES:EBX=0:0；`0x245048` 的 AH=2508h 輸入 DS=8／EDX=244D9Ah、flags=46h。實作前基線 `7b288510f8ff578079e73be0435784a9a2e7f720` 的 `s.dosVectors[8]` 保存保護模式目標，共享 LEBIOSClock 卻只查絕對 IVT，不消費該表。等待來源 **DS:offset** `0188:00271148` 仍零；原版採用該向量已證實，派送與等待未閉合。

本機 `workplace/moo2-probe-277-vectors.txt.gz` SHA-256 `f9fbdbd5358dbdf6683cc966607216a3850edff10ed86858dd3350344874d39b`。工具兩 CPU 模式共用 PIT，模式 2 設定本身已由 276 接通，不重做平台時鐘常數或遊戲 driver 逆向。

[Open Watcom Programmer’s Guide 的 DOS/4GW 25h／35h、32-bit gates、chaining、getting handler](https://open-watcom.github.io/open-watcom-1.9/pguide.html)：25h 安裝保護模式處理器；08h..2Eh（21h 除外）可自實模式向上派送。35h 預設也返回非空的結束鏈入口。平台以私有堆疊轉送，不可把 caller 原堆疊當原版中斷堆疊；返回以 IRET 或鏈接完成。文件不是原版 DOS/4GW 精確版本內部實作證據。

[Intel 80386 中斷程序](https://www.ardent-tool.com/CPU/docs/Intel/386/manuals/prref386/s09_06.htm) 與 [IRET](https://www.ardent-tool.com/CPU/docs/Intel/386/manuals/prref386/IRET.htm) 作受限 32 位元框架／旗標的前提，不建完整 IDT、權限切換或平台核心。既有 cpu386 未實作 CF，不能直接假稱 IRQ0 已能返回。

## 候選位址擷取勘誤

第一次有限擷取使用錯誤載入差值 0x133400，沒有命中，不能作原版向量證據。已校準的 276 原版 0180:0036DAE8 與 dosgolem 高位 LE 0x239AE8 差值是 0x134000；上述候選已訂正，仍須以實際 bytes／輸入核對。失敗終端只留本機 workplace/dos-timer-vector-address-failure-terminal.raw，SHA-256 ce46f0a04cb3985e52b3039d01f4be7905071a5ac8ff0ad51b07510ab4856a68。

## 有限原版審查與 READY 契約

DOSBox-X 2026.07.02 SDL2 重型除錯器、映像 `sha256:659e8abbf93646f59a1586341769bd4b8f3cd1c707859d7a5de4c56e4672b582`，同固定原檔自然啟動，沒有注入暫存器、時計或遊戲記憶體：AH=3508h 在 `0180:0037901B` 的 `CD 21` 返回 `0080:00000D49`；只有 ES／EBX 改變。AH=2508h 在 `0180:00379048` 的 `CD 21` 安裝 `0180:00378D9A`，下一指令 `0037904A` 的完整狀態不變。這些是已證實的介面樣本，不要求合成平台的核心位址相同。

IRQ0 自然入口 `0180:00378D9A`，bytes `2E 83 3D FE F9 39 00 00 0F 87 11 01 00 00 60 1E`；SS:ESP=`00D0:00006838`，24 bytes 為 `16 0F 00 00 80 00 00 00 46 00 00 00 48 68 00 00 5A 68 7A 0C 80 00 E8 00`。前 12 bytes 為 32 位返回位址、CS、flags=46h；這是核心呼叫框架，並非被中斷玩家位置。入口 flags=46h，IF=0。PIT 三筆 OUT 後，原版 DS:offset `0188:0039F148` 從四個零變成 `01 00 00 00`，自然到 `0180:0036DB0B`。只觀察入口與退出，不拆解 ISR 內部。

收據 `workplace/dos-timer-vector-registers.json` SHA-256 `3d30e7a7498d7b9207bbad2b37257f6b9dc5ab2b082681db09464bec539e4727`，終端 `98bffa98c01fe466e2c1847f111c2abd043e7d5bb3a6dc6f01d0e22db2614315`，get／set LOG SHA-256 分別 `18c745f5098897ddcb61d844ecfbfa22595767bfb581d3846b979c0db9d9623c`／`5b5b76db9313052820f0a894c6705501efed2ea50f6340a579d1746871c3be6a`。LOG 只保存呼叫與進核心的邊界，不拿它當服務返回；返回另由遊戲下一指令斷點確認。原版完整記憶體、終端與圖片只留本機。

### 型別、轉移與失敗模式

- 只在明示 MOO2 adapter 安裝。DOS 向量保存 16 位 selector＋32 位 offset，只接受附接的同一保護模式 CPU。35h 對尚未登錄的向量惰性配置非空、可讀的 CF 結束鏈入口，回寫 ES／完整 EBX、清 CF，保存其他狀態。25h 保存有效可讀、Base=0 的目標，清 CF；無效／零／非平坦／保留 sentinel 拒絕且不改原向量。可保存、替換與恢復 35h 的入口；FD2 舊 profile 不受影響。
- 私有結束鏈區 256 bytes 及堆疊 4096 bytes 使用既有 DPMI 配置器，不占遊戲存檔欄位。合成 selector／offset、記憶體配置及 32 位堆疊為現代近似；不聲稱精確核心布局或原版 16 位堆疊尋址。
- 共享 PIT 待處理 IRQ0 在來源 CPU 的 IF=1、主 PIC bit0 未遮罩、IRQ0 未服務／未重入時接受。若保護模式 DOS08h 尚未掛接或恢復有效預設入口，沿既有 BIOS 計數；已污染／不可讀預設入口仍拒絕且保留待處理狀態；若已掛接，兩 CPU 模式都同步執行該原版處理器。實模式來源先保存懸停的保護模式 CPU，返回後完全恢復；實模式 CPU 不被改寫。DPMI／絕對 IVT 的原版核心 trampoline 位址不模擬，非零客製實模式 IVT 仍拒絕。
- 每次原版處理器 CPU 步進也按既有一微秒近似計時。私有堆疊壓入 12 bytes：保留 return sentinel、合成核心 CS、清 IF／TF 的框架旗標；入口只改 CS／SS／ESP／EIP／IF／TF，DF 與其他暫存器不猜補。最多 100000 步，不重入，待處理邊緣合併。未知 opcode 或無返回立即拒絕，不合成遊戲等待值。
- 只接納裸 CF、私有堆疊描述子未改、ESP 位於原框架、12 bytes 完全未污染的最外層返回；恢復全部通用暫存器、段、EIP、EFLAGS 及 FPU。巢狀 IRETD、權限／任務／VM 切換、完整實／保護模式鏈均不在本項；原版若依賴它們，保留明確停點，不默默推進。合成預設入口在客製 IRQ 中被鏈接時也先拒絕，避免把未知 BIOS 鏈假裝完成。
- IRQ0 在接受時設服務中；主 PIC 非指定 EOI 20h 優先結束 IRQ0，否則保留既有 IRQ7 行為。ISR 可讀 bit0、IRR 可讀待處理 bit0；IRQ0 尚服務時不派送較低優先 IRQ7。處理器未 EOI 返回仍維持服務中，不能以返回偷清。滑鼠回呼在 IRQ0 執行中不再派送；IRQ0 可在既有滑鼠回呼上下文執行並完整恢復。

### 驗收、垂直鏈與停止線

本項是原版執行器平台，鏈為原版 AH25 向量 → 共享 PIT → 處理器 CPU → 原版返回／等待；不更改 Go remake 規則、UI、存檔或資料格式。測試涵蓋向量往返／256 個預設入口／完整保存／污染拒絕／有界執行／遮罩／IF／EOI／IRQ7 與滑鼠共存／實模式來源。固定官方 EXE 全套與兩條自然路徑重跑，若仍有標準 CPU 缺件，只報新位置和原始 bytes，不逆向 ISR 內部。

只有原版處理器在 dosgolem 自行返回並讓等待來源變化，才能宣稱本項完整原版路徑閉合；合成處理器測試只證明平台自洽。完整鏈、核心位址／布局、週期／波形與正式玩家輸入仍未知或近似。255 保持 READY，277 不外推主選單／正常玩家路徑或整款完成。

## 受限實作、驗證與目前前沿

READY 後新增 `le_protected_irq0.go`，共用 PIT 在兩 CPU 模式轉送有效 DOS08h，最外層 CF 受限返回，私有框架完整檢查及保存／恢復、EOI／PIC 優先、滑鼠共存、失敗後停止皆有測試。CPU 核心沒有因本項放寬前綴或未知 opcode。合成處理器證明橋接自洽，不能冒稱原版完整 IRQ 鏈已通。

固定 EXE 全套 `DOSGOLEM_MOO2_EXE=/tmp/game/ORION2.EXE GOMAXPROCS=2 go test -p 2 -buildvcs=false ./... -count=1` 通過，`workplace/full-test-277.txt` SHA-256 `f63d2055a0a486999163a33fafb3c8a2b65bea964e674eb39f70542b23d68892`。最後補齊跨 CPU 拒絕、被污染預設入口不得走 BIOS，以及穩定診斷／IRQ7 優先測試後，以同映像乾淨重跑平台、固定 EXE 全套與兩個自然排程條件。

`DOSGOLEM_MOO2_MAX_STEPS=50000000 DOSGOLEM_MOO2_SEPARATE_DOS=1 DOSGOLEM_MOO2_VBE_PNG=<本機 PNG> go run -buildvcs=false ./workplace/moo2-probe /tmp/game/ORION2.EXE --game-dir /tmp/game`；另一條件加 `DOSGOLEM_MOO2_MOUSE_EVENT_AFTER_POSITION=1`。**dosgolem 自行重生的已證實事實**：AH3508 返回合成 `0108:00326008`，AH2508 安裝 `0008:00244D9A`；兩條件第 1,160,098 步第一次真正派送原版 IRQ0，started=1／completed=0，停 **高位 LE 線性** `0x244D9A`，bytes=`2E 83 3D FE 19 27 00 00 0F 87 11 01 00 00 60 1E`。CPU 拒絕 CS override 的 83h 比較，內層解碼後 EIP=244D9Ch；外層上下文完整恢復到 `0x234341`。Micros=1160100、Deliveries=21、InService=true、PIT 模式 3／Reload=19887／Generation=2。這是先前未派送而隱藏的更早 CPU 缺口，不是新通過 20.1M 等待。

完整原版輸入 SHA-256 沿本規格；自然 gzip `30a8bc2eda975584afbac7c5d3398b389f98c4a6a2826f5040c840c74ff92148`／`6562fcd7ea5a08935c44a88b7820429298aff922397898f9d75c64f6dd59cf82`。事件條件明示 requested=true／injected=false；兩條都在位置設定與事件注入之前停止，不能當事件分支已驗收。當前 VBE active、Writes=0；PNG SHA-256 `1610444d26adb3135e7e933dd44912044bb636728af59c70e614945278d3c622`，與先前已檢視黑圖相同，不重做目視。早期 Simtex／MicroProse 收據維持歷史證據，現行有 IRQ0 的設定尚未重達它們。

277 保持 **READY**：原版 IRQ0 已進入但沒有返回，DS offset 271148h 四 bytes 仍零；不得標 CONFORMED。下一步只用公開 CPU 契約補足上述標準 CS 記憶體比較，再自然重跑；不翻譯 ISR 邏輯、不解計時驅動或修改遊戲等待值。保護／實模式完整鏈仍未知，若遇到未建模鏈要保留明確停止，不猜補。255 仍 READY，主選單／正常玩家輸入、受控亂數、音效、Go remake 玩法同狀態未完成。

## 擷取護欄勘誤

上一輪 276 的字串替換誤改 `capture_cmp_memory_register` 的消費端選取條件，會把有效 `00368CA6／00368EA7` 拒絕；真正 PIT 分支仍只驗 CS。已在各自程式區間還原 CMP 條件並把 PIT 固定為 `0180:0036DAFB`。以既有原版 CMP JSON SHA-256 `af5583d7dd5477aba5b864b1c48f345e6b8c3b8c53c3269f88a7af2ce505726d` 與 PIT JSON `c522c19a88f1ea333ec2491befcb3e23b1dafab284e98b39e7728bc92f93dd74` 評估實際 AST 護欄，正例通過、PIT 誤位址拒絕；原有實際 PIT 收據已在正確位址，不因腳本護欄較鬆而失效。

本輪單步 INT 的首次返回擷取實際停在 DOS/4GW 核心 `0080:00000084`，不當服務返回；改遊戲下一指令斷點後成功。失敗終端 SHA-256 `cf6ae41238e5a2b8e837723b8350dda09a6e29f21ec4869afe442c0a10de6043` 留本機。候選差值與服務返回腳本問題均已分類、保留失敗後同映像乾淨重跑；沒有為此追核心內部。新時鐘診斷排除主機指標，避免把每次配置位址混入重播收據。

最新平台測試（含 IRQ7 優先與穩定診斷）命令為 GOMAXPROCS=2 go test ./internal/machine -run 'Test(ProtectedIRQ0|LEMouseCallback|BIOSClock|PIT0)' -count=1 -v；本機 workplace/moo2-277-platform-tests.txt SHA-256 5f87e0db5676d44e348a359c64a07da0972cac7a887340f308544ba9706d867b。
