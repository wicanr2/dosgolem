# 280 — 32 位元同權限遠返回 CB

狀態：**CONFORMED**
日期：2026-10-02
範圍：cpu386 無前綴的 CB，32 位元堆疊與同 RPL、VM=0 的平坦已知 selector；受限平台模型，不建立完整保護模式權限／任務機制。

## 原始定位與公開 CPU 契約

固定官方 1.31 ORION2.EXE SHA-256 `4e11be14217b4aafa1839f333bf5eba037f98b0c44e9e4752c96c464c260419f`、417 原檔與 MOX.SET 沿 [279-cpu386-cs-absolute-es-load.md](279-cpu386-cs-absolute-es-load.md)。基線 `0e6a69b0ca1971bc9235fb9727bc26d6e57eb2ed`，Go 1.24.13；兩個自然排程第 1,231,392 步第六次 IRQ0 在 **dosgolem 高位 LE 線性** `0x244E9C` 的 `CB` 停止，started=6／completed=5。內層 SS:ESP=0110:00000FEC，flags=246h；實際返回框架仍待有限觀察，不能假定與最外層 IRET 框架相同。

[原廠 Intel 80386 RET 契約鏡像](https://www.ardent-tool.com/CPU/docs/Intel/386/manuals/prref386/RET.htm)：同權限的 32 位遠返回從堆疊載入 EIP，再載入 CS 的低 16 位，完整消費 8 bytes；旗標不變。非零可用 code selector、目標在段界內與堆疊可讀是前提；不同權限返回另有 SS／ESP 與資料段調整，不在本項。純 CPU 的語意來自公開契約，不逆向遊戲 ISR 或追逐週期。

DOSBox-X 的候選 **CS:EIP** `0180:00378E9C` 由已校準載入差值 134000h 導出；未命中並核對 CB bytes 與框架前，僅為候選，不是已證實原版返回。只准許一條標準 RET 的有限輸入／輸出擷取，其餘處理器黑箱自然執行。

## 候選型別、轉移與拒絕邊界

- typed input 為目前 CS、SS、ESP、SS 描述子與原始 8-byte 框架：EIP32／selector32。selector 的高 16 位丟棄，不把它當旗標；不額外清理參數。
- 只接受 32 位 operand／address、無 segment／REP／LOCK／其他前綴的 CB；16 位返回、CA 立即值清理、跨權限與中斷返回不因此放寬。
- 先依 80386 同權限保護模式契約完整檢查 SS:ESP 的前三個 word（6 bytes）及 ESP＋8 不溢位，讀取 EIP32 與 CS16；selector slot 的高 word 不讀取，仍消費 8 bytes。只接受目前／返回 CS 非 null、已知平坦 Base=0 描述子、相同 RPL；返回 EIP 在目標 Limit 內且可讀。不消費客製 SegmentRead8／SegmentRead16／SegmentLoadOK 回呼；只按 SS 描述子映射 CPU bus，與平坦程式擷取相同。未知 resolver、非平坦 code、VM=1 或未建模權限一律拒絕。現有 Descriptor 只有 Base／Limit／Writable，code／DPL／present 不在此模型；不宣稱完整 x86 保護檢查或精確 #GP／#SS。
- 成功只改 CS、EIP、ESP+=8；全部其他暫存器、段、旗標、FPU 與記憶體保持。所有讀取與檢查成功前不得提交返回；錯誤時解碼 EIP 可前進，但 ESP／段／資料不得部分修改。
- 對拍工具的垂直鏈為原版指令／返回框架 → CPU → 原版黑箱自然繼續；主庫玩法、UI、存檔、時鐘與等待來源不變。測試涵蓋段混淆、唯讀堆疊、EIP32 與 selector 低 word／高位丟棄、有效與無效 selector、目標界限／可讀性、6-byte 必要來源部分失敗／高 word 不讀取、ESP 溢位與前綴拒絕。
- 有限原版與公開契約審查後 READY 才實作；全部 CPU／固定 EXE 回歸與 dosgolem 自行重生的自然收據後才能限定 CONFORMED。277 完整 IRQ／等待與正常玩家路徑另外判定，不由 CB 一項推定完成。

### 讀取寬度審查

Intel 80386 保護模式 RET 的公開流程只要求前三個 word 可讀；32 位 selector slot 只載入低 word，而堆疊仍前進 8 bytes。DRAFT 的早期候選曾要求全部 8 bytes 並讀兩個 dword，已在實作前按公開契約訂正為 6-byte 必要讀取，另測被跳過的高 word 不可讀仍能返回。ESP 加法溢位拒絕及平坦 descriptor 模型是本工具明示限制，不冒稱硬體 ESP 繞回或完整保護檢查。

## 有限原版樣本與 READY 審查

已證實：同固定輸入、DOSBox-X 2026.07.02 SDL2 重型除錯器／映像 sha256:659e8abbf93646f59a1586341769bd4b8f3cd1c707859d7a5de4c56e4672b582，startup_probe_131.py --far-ret，300 秒有界容器退出 0。原版命中 0180:00378E9C 的 CB，SS:ESP=00D0:00006830，原始框架 49 0D 00 00 80 00 00 00；一條指令後返回 0080:00000D49，ESP=6838h，flags=246h、全部其他暫存器／段與框架保持。這是原版結束鏈入口，不把 0080 核心程式或布局搬入 CPU。

完整欄位順序 CS EIP EAX EBX ECX EDX ESI EDI EBP DS ES FS GS SS ESP EFLAGS：180 378e9c 1a988 1a988 0 3ebc74 470 3a2090 3ebbc0 188 188 0 20 d0 6830 246。下一位置 CS=80、EIP=d49、ESP=6838，其他欄位保持。框架與返回已足夠建立受限標準 CPU 契約，候選由 DRAFT 轉 READY；有限樣本在純 CPU 平坦記憶體重播，目標只放可讀佔位 byte，不執行或聲稱模擬原版核心入口。

本機 workplace/far-ret-registers.json SHA-256 a494d64654cec4198e065c0eac1e9981caf09d1002fbf01a6c18ea22842a17b0，LOG d76afc149acffdcb5b7f8ece204ed9396939aec8014ff9d9ba9b9885470847e2，終端 28cd8774ebd9000e41d45950e30a7bc978628e59cee16813814873ced69f302f。與 dosgolem 第六次 IRQ 的全部初態／地址不同，不能宣稱整段同狀態；原始框架、定位及有限轉移才是本項審查範圍。完整輸出不進 Git。

## 受限實作與自然收據

限定 CONFORMED：只新增裸 CB、32 位、VM=0、已知平坦 code selector／同 RPL 的返回。先按 SS 描述子檢查並讀 6 bytes，丟棄 selector slot 高 word，再檢查目標可讀性；全部通過才提交 CS／EIP／ESP。不修改近返回、不新增 CA／跨權限／核心鏈，也不依遊戲地址放特例。

完整 CPU 套件通過：唯讀非平坦 SS／DS 混淆、四種相同 RPL／selector 高 word 雜訊、必要 6 bytes 與每一來源 byte 失敗、完整 32 位目標／稀疏最高地址／ESP 邊界、未知／null／非平坦／不同 RPL／VM／前綴拒絕、不消費段回呼、FPU／全部其他狀態／框架保持及上述完整原版有限樣本。GOMAXPROCS=2 go test -buildvcs=false ./internal/cpu386 -count=1；workplace/moo2-280-cpu-tests.txt SHA-256 ccc7bfe98304b0363076b298b9b29a00a9b710ff39d39a9a9d7841ae489c7e36。

固定官方 EXE 的 DOSGOLEM_MOO2_EXE=/tmp/game/ORION2.EXE go test -p 2 -buildvcs=false ./... -count=1 通過；workplace/full-test-280.txt SHA-256 00c4cb5dda024faabe5768e0855109cbd1780f16fe2da3c80fc2e443c8a600e4。工具／Go 與映像沿上述基線，所有工作在有界 UID/GID 1000:1000 的 --rm／--network none 容器，原版 ZIP／patch 唯讀，原檔抽取至 /tmp/game。

按 277 的兩個自然排程命令重跑，輸出名稱改 280。已證實：兩條外層第 1,231,392 步、第六次 IRQ0 都已越過高位 LE 0x244E9C 的 CB，內層返回 **dosgolem 合成平台 CS:EIP** 0108:00326008；SS:ESP=0110:00000FF4，較前沿 FEC 增加 8，flags=246h。內層 R 順序 EAX ECX EDX EBX ESP EBP ESI EDI：3D0300 80 3D0700 FFD23C00 FF4 FFD23C00 80 FFFF8C00；段 CS DS ES FS GS SS：108 188 188 0 20 110。下一步進入規格 277 的既有「IRQ0 預設核心鏈尚未建模」護欄，沒有把合成 CF 當原版核心執行；此次不是新的未支援 CB 或原版高位 LE opcode 失敗。完整外層上下文恢復至 0x246596，started=6／completed=5 不變。

PIT 模式 3／Reload=14916／Generation=3，Micros=1233787、Deliveries=26、InService=false，等待來源 DS:00271148 四 bytes 仍零。這次比 279 多執行一次成功 CB，沒有完整第六次返回或模式 2 等待閉合。事件條件 requested=true／injected=false，不算實際玩家分支驗收。自然 gzip SHA-256 ee40229c9a12fff73978cf2eba833d690538fdc9a9c832033124540992231af5／d6454c281fe069a9784bb4ddcd46b78c2a7da57bfbe092253565f73596f5b45d；VBE Writes=0、PNG SHA-256 1610444d26adb3135e7e933dd44912044bb636728af59c70e614945278d3c622 同已檢視黑圖，不重做目視。完整原版資料／終端／gzip／PNG 只留本機，不入公開 Git。

下一步只依公開 DOS/4GW chaining／結束鏈介面契約建立新的受限平台規格，審查 READY 後再實作預設入口的合法完成／拒絕條件。不逆向 ISR、timer driver／busy-wait，不搬入核心位址或猜寫等待值。255／277 仍 READY；主選單／正常玩家路徑、音效、受控亂數及 Go remake 玩法同狀態尚未完成。


## 規格 281 的平台結束鏈後續

預設核心鏈停點已由規格 281 接通，見 [281-moo2-protected-irq0-end-chain.md](281-moo2-protected-irq0-end-chain.md)。本規格 CB 的 CPU 範圍保持；上述 started=6／completed=5 是 280 歷史。281 自行重生原版 1,595 次完整返回與等待來源 01 00 00 00，受限 IRQ0／模式 2 等待閉合；下一停點為 PIT count latch，不再是未建模核心鏈。核心布局／逐週期與正常玩家操作仍未知或近似，255 READY。
