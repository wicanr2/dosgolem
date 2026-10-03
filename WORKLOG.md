# 工作歷程

## 2026-09-07

依使用者要求更新遠端預設主線 master，建立 feat/fd2-input-parity-20260907。
已重跑固定 FD2 啟動回歸與自然執行；目前仍受 36h 指令前綴缺口阻擋。
[目前狀態與重跑證據](docs/findings/2026-09-07-fd2-main-input-parity.md)。

## 2026-09-08

以 DOSBox-X 真實原版對照 dosgolem 首次 MDI.INI 定位回傳與兩條 SS store。
兩邊均回傳218；停止原因為 CPU prefix／MOV 支援缺口。未修改CPU。
[原因與證據](docs/findings/2026-09-08-fd2-dosbox-seek-diagnosis.md)。本輪容器已清理。

依使用者「修正囉」完成 spec 185 的 SS word store；段分離、邊界與狀態保留
測試通過，真實原版寫入結果000000DA與 DOSBox-X 相符。自然探針多前進95步，
新阻塞37381／09 C6另列，未擴大本次指令實作範圍。

使用者要求繼續補齊dosgolem後，完成規格186批次1–18的平台指令、heap背書頁與
Watcom DPMI0100轉接。批次17固定原版整體578案例（含子案例）通過，
批次18的CPU回歸另通過。原版自然到20795步，已載入並搬移15118-byte驅動，
註冊INT66，現在明確阻擋於DPMI0300。沒有畫面或玩家路徑完成宣稱。
目前唯一狀態見[186](docs/spec/186-fd2-platform-gap-continuation.md)。

### 2026-09-08 後續勘誤與平台初始化驗收

規格186批次19–49已逐項補齊並自然重跑。舊0300阻塞由真實模式CPU橋接取代，
OPL偵測／原版驅動初始化、BIOS0040與回掃等待、DSP重設及版本／配置查詢、
PIC遮罩皆已前進；目前為DMA埠000A，不是0300本身缺失。
DOSBox辅助量得0040基底0400及資料D4 03 29 30，PIC遮罩21=F8、A1=2C。
實模式0300→1的呼叫回傳相符，但進入時AF差一位，不冒稱完整同狀態。

批次48完整固定原版回歸621筆通過事件、0失敗，含原版入口至音源初始化與
回掃查詢；批次49 machine回歸另通過。新增測試曾誤用0388而非原版0228別名埠，
且未等待後續回掃查詢，均修正後乾淨重跑；DOSBox首次IN命令不被支援，
改INB後重跑。這些屬驗證腳本問題，失敗收據保留，不列產品缺陷。
原版無修改、沒有畫面／操作感一致宣稱。當前狀態與逐批來源雜湊見規格186及JSON。

### 2026-09-08 平台批次50–64與初始化回歸

延續186，原版自然入口已走完數位音效初始化：56820步返回AX8405；
本平台近似下13個DMA區塊、13次IRQ7，真正讀DMA記憶體並進原版IVT ISR。
這些次數及回傳值是執行器回歸，尚未取得DOSBox完整同狀態驗收。
DSP reset採DOSBox-X 22050Hz，有理數累積避免樣本逐次取整漂移；
仍遵守硬體時序停止線，沒有音訊輸出或微週期忠實度聲明。

固定原版回歸638筆通過事件、0失敗；63–64另通過CPU及machine回歸。
舊「DMA未實作」由本批受限通道1／IRQ7子集取代，其他模式仍拒絕。
最新自然停點0x406AF／2B 46 10 99 31 D0 29 D0 39 F8 7F 0E 89 C8 2B 46，未到遊戲畫面。
機械收據、輸入／來源雜湊與唯一現況統一見186及其JSON；不提升PLAYER-E2。

### 2026-09-08 批次65–70與非分頁契約勘誤

原版音效啟用AX0305已由dosgolem自行執行返回AX1。DOSBox-X在相對應
程式位置0158:001F6C29的EAX/EBX/ECX/EDX/ESI與dosgolem LE40C29一致，
但配置位址不同，不能宣稱完整同狀態，也未藉此推定AX8405全包一致。

Watcom轉接0600原先按Mem長度拒絕，與既有DPMIHost非分頁服務矛盾；
公開DPMI0600明定非虛擬記憶體host忽略且成功。批次69統一轉接，
不擴充heap或可讀Mem，實際存取邊界仍有效。批次10舊嚴格鎖定斷言被此契約取代。
本次也修正證據彙整器欄位名稱：讀dpmi_real_mode_last，避免50以後
摘要出現null；原始逐批JSON從未遺失。current省略大量OPL事件，原收據保持私有。

固定原版全套645筆通過事件、0失敗；
新停點0x495C5／FF 14 85 38 76 04 00 07 1F 5F 5E 5B C9 C3 55 8B。尚無遊戲畫面與AppImage驗收。

### 2026-09-08 配置分區回歸定位

批次81初次固定原版回歸發現DPMI0300的預設實模式堆疊仍按整個Mem尾端
推動DOS游標；高位近堆使其越過640KiB。修正只跟隨低位配置增長，
高位backing不影響DOS游標。另修環境表測試中舊合成位址634D8的假設，
實際驗收改為新高位表位置、tail=table+4及零終止內容；不是略過內容驗證。

## 2026-09-08：批次71–86平台接續

以DOSBox-X前後指令量測確認F2 MOVS相容行為，再由dosgolem自然重跑。
IDA定位原生_nfree與合成配置器不相容，補成對釋放、空閒重用及高位分區。
嚴格接通保護模式OUT後揭露PIT設定缺口；已補設定子集，尚無IRQ0。
原版入口真正到mode13，仍全零色號且未寫DAC，不是遊戲畫面或操作驗收。
批次85初次XOR測試漏算保留旗標，僅修預期值後相同容器重跑：
663筆通過事件、0失敗、2套件略過。批次86 CPU／machine通過，
現停89029步 LE25982的MOVZX來源缺件。未修改原版、未跳過初始化。
所有正式原版收據均由dosgolem自行重生，DOSBox僅輔助。

## 2026-09-08：批次87–101，原版標誌與時鐘等待

dosgolem由原版入口自行完成漢堂標誌淡入、等待、淡出與後續動畫。
DOSBox在對應迴圈觀測BDA時鐘+1、EAX0→1後退出，確認舊停滯原因；
已按公開PIT／BIOS契約接回dosgolem，而非改寫遊戲等待結果。
取樣率41h已依Creative規格補齊；DMA保護模式排程仍缺，不作音畫同步聲明。
完整指定回歸682筆通過事件、0失敗、2套件略過。
較長自然路徑51394659步停ADD記憶體SIB缺件，批次102接續。
PNG保留原生320×200與雜湊；未做DOSBox同狀態像素比較、未驗收選單至第一關。
Docker檢查：自有fd2-gap-run及fd2-full-regression均為--rm；完成後未留自有停止容器。

## 2026-09-08：批次102–118，標題輸入與王宮開場

原版標題由dosgolem自行重生；START與Down後LOAD的兩張320×200畫面，
對DOSBox輔助擷取皆64000像素、0差異。第一次DOSBox腳本未等到斷點，
第二次原生抓圖無輸出；第三次視窗擷取成功，失敗索引保留，沒有混用收據。
BIOS AH10空佇列等待與真實按鍵消費已接入，正常Down／Up／Enter進王宮，
後續演出至75423804步，停word DEC；尚未第一關與AppImage。

FD2.TMP開寫缺口由獨立覆蓋層補齊，唯讀原版不變。
unsafe-path舊測試把權限拒絕預期為缺檔2，依DOS契約改5後乾淨重跑通過；
缺檔仍2，不放寬路徑存取。每次原版重跑均新建state目錄。

批次118固定原版指定回歸702筆通過事件、0失敗、3套件略過；
受版控framecompare重跑兩張舊原始PNG仍零差異。
保護模式音效排程及長按／釋放仍缺，未宣稱完整手感或PLAYER-E2。

## 2026-09-08：批次119–124，對話輸入與戶外場景

word DEC／XOR、byte SUB／XCHG、word單位移與累加器符號擴展補齊，
逐批CPU／machine回歸通過。最新完整固定原版指定回歸仍是批次118的
702筆通過事件、0失敗、3套件略過；不把後續局部回歸冒稱新全套。

第一頁原版輪詢10620直接比較BIOS041A／041C，既有IDA證據已證實；
原探針只在AH10等待送鍵，故對話一直未收到輸入。補明示步數排程後，
第80000000步Enter由原版讀取並翻至父王第二頁，與DOSBox輔助圖64000像素零差異。
第一頁輔助圖仍有1298像素差異，尚未判定是否全屬動畫相位，不作整幀一致聲明。
DOSBox兩頁原始擷取及腳本保存於本機dosbox-dialogue-20260908研究目錄。

最新正常重播200000000步未遇unsupported，由預算上限停止；
43筆送入BIOS佇列、36次AH10讀取，不把差額一律當成遺失，
原版也有直接清除佇列路徑。畫面已從王宮進到戶外人物對話，
尚未驗收第一關戰場HUD／第五回合及AppImage操作感。
沒有修改原版、沒有直接進場或注入遊戲狀態。
本批自有FD2容器均已退出並移除，git diff --check通過。

## 2026-09-08：批次125–126，等待邊界逐頁擷取

補齊 AND byte 的 ESP/SIB 路徑，完整 CPU／machine 套件通過。
正常 START 逐頁擷取從38頁延伸到65頁；後者僅達步數預算，沒有新缺指令。
主證據：`docs/evidence/fd2-dialogue-prefix-20260908.json`。
這批收據揭露 remake 的跨頁清空前文與等待箭頭缺件，已回到 FD2 修正。
不把 BIOS 等待邊界自動按鍵稱為實體鍵盤手感完全一致；第一關第五回合仍未驗收。
自有逐頁擷取、回歸容器皆為 --rm，工作結束後已退出。

## 2026-09-08 FD2 首關對拍暫停交接

批次127–146補平台缺口，最新dosfile／machine／cpu386共720項通過、0失敗、0略過。批次145／146以原版動態IOMode上限20、舊handle150、FD2.TMP要求207360卻只寫10752的證據修正代號重用與完整ECX寫入。修正後r12從START自行擷取首攻返回，人物已正常，至第2回合後依使用者要求暫停。r11到城鎮但存在模式表表外寫入，不作最終收據。規格186與FD2文件94保存來源；未提交推送，未宣稱完整雙側通關。

## 2026-09-30：MOO2 內嵌 MZ 資料頁基址勘誤

MOO2 原檔 `0x26654` 的第二層 MZ 指向 LE `0x292E4`；資料頁偏移相對第二層 MZ。先前 195／196 以原檔零點讀錯頁，`66 3B`、5,359 步與 `POP EDX` 不再列為 MOO2 原版啟動證據，勘誤及新證據見規格 197。新增明示內嵌 MZ 載入與 `leprobe -mz-base`，兩版固定雜湊真檔均重生 `EB 76 WATCOM` 入口，與 IDA 原檔映射一致。

原版真入口未接 DOS 服務時，第 11 步要求 `INT 21h/AH=30h`。FD2 專屬 DOS 層僅作可丟棄診斷；其第 36 步揭露通用 `66 09 CA` 缺口，依規格 198 補 16 位暫存器 OR，合成旗標／保留高位測試通過。診斷第 45 步轉停 `66 26 8C 1D`，因仍用 FD2 selector／環境，不列為 MOO2 正式收據。原版檔唯讀；MOO2 服務、畫面與同狀態玩法對拍尚未完成。

## 2026-10-01：MOO2 DOS 向量與受限 IRQ0 轉送

基線 7b288510f8ff578079e73be0435784a9a2e7f720；規格 277 同次建檔入索引，原版非空向量、自然私有框架與等待退出及公開 DOS/4GW 契約具備後才 READY／實作。平台與固定官方 EXE 全套通過，但兩個自然排程條件均在首次 IRQ0 的 CS 記憶體比較停止，原版返回未閉合，277 保持 READY、255 READY。完整證據、命令與收據在 docs/spec/277-moo2-dos4gw-protected-irq0.md；沒有把合成處理器測試升格成原版玩法對拍。

保留錯誤候選位址／單步進核心的失敗收據後，同映像乾淨重跑成功；上一輪 CMP／PIT 擷取護欄誤替換以既有原版 JSON 與實際 AST 訂正。原版資料、完整終端、gzip 與 PNG 留本機；本輪原始碼、文件與輸出 UID/GID=1000:1000。一次性 Docker 容器收尾清空，不清理其他專案或映像。本輪提交推送 github/codex/moo2-parity-20260930，沒有推本機 origin；精確 HEAD 以 Git 紀錄及主專案交接為準。

## 2026-10-01：CS 記憶體 CMP 的帶符號 imm8

基線 777472b02a561740e11558ecb9852b7579541454；規格 278 同次建檔入索引，公開 Intel CPU 契約與原版入口單指令完整資料／旗標審查後 DRAFT→READY 才實作。全部 CPU 與固定官方 EXE 全套通過，兩條自然排程確實越過比較，轉停 0x244DBA 的 CS word MOV ES。278 限定 CONFORMED，277／255 READY，沒有宣稱原版 IRQ 返回、等待變化或玩家對拍；精確命令、位址、bytes、輸入與收據在規格 278。

錯猜規格索引 README.md、容器缺 rg 皆為讀取問題，改用實際 000-index.md 與 pathlib 後繼續；沒有另建工具映像或退回主機分析。有限原版擷取只觀察標準 CMP，未拆 ISR。原始輸出、gzip、PNG 留本機；新增文件入索引、277 舊停點回填有失敗即拒絕護欄。測試及原始碼 UID/GID=1000:1000；一次性容器清理依本批最後稽核，提交推送 github 的授權沿用，未推本機 origin。

## 2026-10-01：CS 絕對 word 的 ES 載入

基線 fabbed1a8bfd1c010b66009bdffe4a6812eb9924；規格 279 同次建檔入索引，有限原版來源 88 01 FF FF、ES／完整狀態保持與公開 MOV 契約審查後 READY 才擴充 ES 目的。CPU 全部、固定官方 EXE 全套與兩個自然排程通過工程驗證；自然原版已完成五次 IRQ0 返回，第六次停 CB，完整等待及玩家路徑仍未閉合。279 限定 CONFORMED，255／277 READY；原始定位、命令、完整輸入／收據在規格 279。

有限擷取第一輪有完整 JSON／等待退出，但外層 240 秒回傳 124；保留逾時收據，不歸為 CPU 失敗，只改外層 300 秒，同映像與輸入乾淨重跑退出 0。PDF 抓圖 Cache miss 改用同 PDF 文字／附錄交叉核對；MOV 鏡像與重排 PDF 的表格 8D 誤植另由附錄 8E 編碼與原始 bytes 校準，未改變 CPU 契約。256 舊拒絕 ES 的測試範圍由獨立 279 新測試取代，未知形狀拒絕保留；256／278 舊範圍與停點已回填，有失敗即拒絕護欄。

原版素材、完整終端／記憶體／gzip／PNG 只留本機；全部新增來源、文件與收據 UID/GID=1000:1000，本批 Docker 一次性容器收尾清空，無 root-owned 或 .md 空目錄。提交推送 github/codex/moo2-parity-20260930，未推本機 origin；精確 HEAD 由 Git 與主庫交接記錄。

## 2026-10-02：32 位元同權限遠返回 CB

基線 0e6a69b0ca1971bc9235fb9727bc26d6e57eb2ed，規格 280 同次建檔入索引。有限原版 CB 的框架與 CS／EIP／ESP 完整返回、公開 Intel RET 契約審查後 DRAFT→READY，才實作受限平坦同權限返回。審查將早期全 8-byte 來源改為必要 6-byte 讀取、仍消費 8；跳過 selector 高 word，不修改近返回或擴充權限／核心鏈。

全部 CPU 及固定官方 EXE 全套通過；兩個自然排程均越過 CB，返回合成 0108:00326008，ESP=FF4h，進既有預設核心鏈尚未建模護欄。started=6／completed=5、等待來源仍零、事件未注入；280 限定 CONFORMED，255／277 READY，完整 IRQ 等待與玩家路徑尚未完成。精確命令、原始定位／地址空間／bytes 與收據在規格 280；279／277 舊停點回填並有失敗即拒絕檢查入口。

有限原版擷取在 300 秒容器乾淨退出 0；首次 image inspect 缺欄位、查不存在 read32 與猜錯舊規格檔名均為讀取問題，改實際 Config／read16／檔名後繼續，未建重複映像或退回主機分析。原版素材、完整終端／記憶體／gzip／PNG 留本機；新增來源及輸出 UID/GID=1000:1000，回填稽核首輪只刪原始定位的第一次出現，另一處仍在，改為全刪後負例正確拒絕；這是稽核腳本問題。全部 23 個回填函式、新回填四個負例、原版實際 AST 返回保持正負例及既有 CMP／PIT 護欄通過，工具無 root-owned／誤建 .md 目錄，本批一次性容器已清空。沿用推送 github/codex/moo2-parity-20260930 授權，未推本機 origin；下一步只依公開 chaining／結束鏈契約建立受限平台規格，不逆向 ISR／driver 或猜寫等待值。


## 2026-10-02：預設 IRQ0 結束鏈與原版模式 2 等待閉合

基線 3e260dcf2215228d840b98242c1226ab7f902456；沿用規格閘門／平台優先／dosgolem 與文件職責／結論回填路由，主庫玩法 RE 閘門不變。281 同次 DRAFT 建檔入索引，有限原版黑箱邊界推翻「只返回、不增加 BIOS tick」候選；BDA+1 直接證據及公開 BIOS tick／EOI 契約審查後 READY 才實作。核心 SS／IF 布局仍明示近似，不追 ISR／driver／busy-wait 或硬體逐週期。

平台套件與固定官方 EXE 全套通過。dosgolem 兩個自然排程自行完成 1,595 次 IRQ0 返回，等待值變為 1、原版自行退出模式 2 等待；受控滑鼠事件已注入，仍不等於完整座標／游標消費或真實玩家操作。281 及受限 277 CONFORMED，255 READY；下一停點為 0x239B3A 的 PIT 控制字 00h 計數鎖存。精確命令、輸入／地址空間／bytes／收據與未知邊界集中在規格 281。主選單與整款 remake 未完成，未修改主庫 Go 玩法。

有限擷取 300 秒容器乾淨退出 0；初次 scope／舊規格檔名猜錯、公開固定 revision 原始碼 Cache miss 為讀取問題，改 CLAUDE 指定實際檔名／既有索引及官方公開 doxygen 參考，未另建映像或把網站版本冒稱本機 binary。有限原版返回護欄以既有實際收據／12項暫存器與 tick 負例審查；全部 24 個回填函式、四項新缺證據負例及既有 CMP／PIT 實際 AST 護欄通過。

276／277／280 舊邊界已回填、索引維持單一入口；原版素材、完整記憶體／終端／gzip／PNG 留本機。新檔／輸出 UID/GID=1000:1000，工具無 root-owned 或誤建 .md 目錄，git diff --check 通過。本批一次性 Docker 容器已退出移除，未清理其他專案或映像。沿用使用者推送 github/codex/moo2-parity-20260930 授權，未推本機 origin；精確提交由 Git 與主庫交接記錄。


## 2026-10-02：模式 2 計數鎖存與 byte 立即埠輸入

基線 9b8d1a07121fab929fc94a7f529449beb0d49742。281 完成推送後，沿規格／平台／文件及回填路由繼續原版正常啟動。282／283 分別同次 DRAFT＋索引，公開 Intel PIT／IN 契約及實際自然停點審查後 READY 才實作。PIT 鎖存只依既有共享分數相位取 Mode=2 的凍結計數，不開另一個時鐘，不逆向 driver／ISR／busy-wait；E4 只接裸 byte 立即埠，保持既有 EC 與其他未知形狀拒絕。

282 新 CPU 接線測試先用僅供 BDA 的舊樣板，程式容量不足而 panic；修正容量／I/O 接線後揭露 E4 缺件，實模式已通過。第二次失敗重查路由，平台先用已支援 EC 隔離驗證，兩份失敗收據保留；沒有用時鐘特例讓測試過。282 平台及固定 EXE 全套通過，原版自然 OUT 成功，實際 E4 停點確認後完成 283 READY 審查。

全部 CPU、平台與固定 EXE 全套通過；兩個自然排程自行讀出凍結 count=5681 的低 31h／高 16h 並解除 latch，後續另取 5302／5180，未把正式遊戲鎖成測試值。原版返回／等待已通，新的標準 CPU 停點是高位 LE 0x254249 的 byte 記憶體 SUB 80／ModRM 2D。282／283 限定 CONFORMED，255 READY；主選單與正常玩家路徑、音效／受控亂數及整款 remake 未完成。精確命令、原始定位／bytes／輸入雜湊與所有收據集中在規格 282／283；自然有限診斷加入既有受版控探針，原版資料未注入或修改。

276／281／282 舊停點回填、索引單一入口與失敗即拒絕護欄一併維護。原版素材與完整記憶體／終端／gzip／PNG 仍留本機；提交只有自製原始碼、測試、有限探針及文字證據。全部 26 個回填函式、六項新缺證據負例／兩個 CLI、兩排程三筆 latch／兩次 IN 狀態、語法／索引／繁體字／UID 稽核通過；工具無 root-owned／誤建 .md 目錄，本批一次性容器已退出移除。沿用 push 授權至 github 隔離分支，不推本機 origin，精確提交見 Git 與主庫交接。


## 2026-10-02：byte 記憶體 SUB

基線 c58709c5ffc8841a22112ad1ea8016890f87d9e5；沿用規格閘門／平台優先／dosgolem、文件職責與結論回填路由，主庫玩法 RE 閘門保持關閉。284 同次 DRAFT＋索引，先以唯讀有限探針保存原始 DS:002726C0 byte=16h，再依公開 Intel SUB 契約審查為 READY，才接通記憶體目的。byte 寫回成功後才發布算術旗標，不猜欄位用途或分析 runtime／driver／ISR。

完整 CPU 測試初次通過，審查發現前綴負例的未知段可能掩蓋錯誤接受；改成所有段均可寫後以同命令乾淨重跑通過。全部 byte 配對／兩種初始旗標、ModRM／SIB／DS／SS、地址繞回與拒絕邊界通過。固定官方 EXE 全套通過，兩個原版自然排程自行完成 16h→0Eh、0Dh→05h，完整 R／段保持，旗標符合 CPU 契約。284 限定 CONFORMED；第 20,637,097 步轉停高位 LE 0x25425F 的 byte 記憶體 ADD，主選單／正常玩家路徑、音效／受控亂數及整款 remake 未完成。

首次 image inspect 的 Entrypoint 欄位不存在，改讀實際 Config；文件輸出截短後按實際段落補讀。均為讀取問題，未新建映像或退回主機執行。精確命令、原始定位／bytes、輸入與測試／自然收據雜湊集中於規格 284；283 舊停點與索引同步回填。全部 27 個回填函式、新四項缺證據負例／CLI、兩排程 byte／完整狀態稽核通過。

本輪來源／輸出 UID/GID=1000:1000，工具無 root-owned／誤建 .md 目錄；原版素材與完整終端／記憶體／gzip／PNG 留本機。git diff --check 通過，一次性容器已退出移除。清理時一個 Go 容器先顯示執行中，查其歸屬前已自行移除，未停止其他專案。沿用推送 github/codex/moo2-parity-20260930 授權，不推本機 origin；下一步只按公開 ADD 契約與只讀目的 byte 建窄 CPU 規格。


## 2026-10-02：byte 記憶體 ADD

284 工具及主庫已推送並回讀 HEAD。285 同次 DRAFT＋索引，唯讀原版自然初態 byte=03h／imm8=18h 與公開 Intel ADD 契約審查 READY 後才實作。全部 CPU 與固定官方 EXE 全套通過，兩個原版自然排程自行完成 byte=03h→1Bh／flags=297h→206h、完整 R／段保持；285 限定 CONFORMED，轉停 0x254275 的 byte 暫存器 XOR。主庫玩法 RE 閘門未開，主選單／正常玩家路徑及整款 remake 未完成。

首次 ADD 地址繞回測試誤留 SUB 編碼 AF，改為 ADD 的 87 後以同映像／命令乾淨重跑通過，首次失敗收據保留。審查另發現索引 276／283／284 的歷史停點摘要未跟隨回填，285 初建索引沿用舊 byte／imm 值；已按實際收據修正摘要與後續連結，不重開已完成範圍。主庫 284 稽核首輪錯要求 WORKLIST 複製工具提交，依文件職責排除後重跑通過；文件本身沒有缺件。

精確命令、原始定位／bytes、輸入與回歸／自然收據雜湊集中於規格 285；284 舊停點與索引／失敗即拒絕護欄同步回填。原版素材與完整終端／記憶體／gzip／PNG 留本機，只有自製來源／測試／文字證據公開。擁有權與回填稽核、Docker 清理狀態隨本次提交核對，沿用 github 隔離分支推送授權，不推本機 origin；下一步僅按公開 XOR 定義／未定義旗標契約建窄規格。

28 個回填函式、新四項缺證據負例、兩排程原版 ADD 後態與 CLI 通過。首次繁體字稽核找到單一「執的簡體」字，修正後重跑；稽核命令加 set -e，避免末尾 CLI 成功遮蔽前段失敗。索引／語法／UID 與無誤建目錄檢查通過。本批一次性 Go 容器已退出移除，未留下自有執行中或停止容器。


## 2026-10-02：byte 暫存器 XOR

主庫 d51d7a67771d328d54924d9b28e5b0e10a0ae67d／工具 96aba2440ce181a1808a508ee897985a2fbcdc99 乾淨；沿用規格閘門／dosgolem、文件職責及後續回填路由，主庫玩法 RE 閘門保持關閉。286 同次 DRAFT＋索引，唯讀完整 CL 初態及公開 Intel XOR 契約審查 READY 後才實作。AF 未定義，沿既有清除模型並與定義五旗標分開驗，未作硬體全旗標對齊聲明。

八目的全部 byte 配對／兩種旗標、目的外完整狀態與拒絕邊界、其他 group 及既有 byte／word／dword XOR 通過；CPU 全套與固定官方 EXE 全套通過。兩條原版自然排程自行完成三筆 CL XOR，目的外 R／段保持；286 限定 CONFORMED，轉停高位 LE 0x254499 的 dword 暫存器 TEST。IRQ0 自行完成 1,598 次，等待值4；主選單／正常玩家路徑與整款 remake 未完成。

批次讀取輸出被截短後，按原始檔段落／gzip 有限行補讀；未把部分輸出當完整驗收。精確命令、原始定位／bytes、輸入與所有收據雜湊集中於規格 286；285 舊停點與索引同步回填。原版素材與完整終端／記憶體／gzip／PNG 留本機，只有自製來源／測試／文字證據公開。推送前核對回填護欄、UID 與 Docker 清理，沿用 github 隔離分支授權，不推本機 origin。

全部 29 個回填函式、新四項缺證據負例／CLI、兩排程三筆 CL 與定義旗標／完整外層保持通過；索引／語法／繁體字／UID 及無誤建目錄檢查通過，git diff --check 通過。本批一次性 Go 容器已退出移除，未清理其他專案或映像。


## 2026-10-02：dword 暫存器 TEST 與第一 JNZ

286 提交 30faf6eb5643aac18d197c64804f81c18293876b 推送後，沿同一規格／dosgolem／文件與回填路由繼續。287 同次 DRAFT＋索引，唯讀完整原版 ECX 初態與公開 TEST／旗標附錄審查 READY 後實作。只新增裸 F7 /0 暫存器形式，保持既有 word／記憶體 TEST 與其他 group，AF 清除明標工具近似；主庫玩法 RE 閘門未開。

全部 CPU／固定官方 EXE 全套通過，兩原版自然排程自行發布 flags=246h、完整 R／段保持，第一 JNZ 不跳，續行至 0x2544A1；後續較晚到達 0x2544CB 的狀態不混作第一分支樣本。287 限定 CONFORMED，轉停 0x254510 的 dword ROL／imm8 缺件。IRQ0 completed=1598／等待值4，主選單／正常玩家路徑與整款 remake 仍未完成。

精確命令、原始定位／bytes、輸入與 CPU／全套／兩自然收據雜湊集中於規格 287；286 舊停點與索引同步回填。批次讀取被截短後以有限 gzip 行補讀，不以截短資料作驗收。原版素材與完整終端／記憶體／gzip／PNG 留本機，只有自製來源／測試／文字證據公開；提交前核對回填護欄、UID 與 Docker 清理，沿用 github 隔離分支授權，不推本機 origin。


全部 30 個回填函式、新四項缺證據負例／CLI、兩排程 TEST／第一 JNZ／完整 R／段與定義旗標通過；索引／語法／繁體字／UID 與無誤建目錄檢查通過，git diff --check 通過。本批一次性 Go 容器已退出移除，未清理其他專案或映像。


## 2026-10-02：dword 暫存器 ROL 與原版自然後態

基線 4d7ae6c2176feda4c61e47ee2db41695a25c2208；沿用規格閘門／dosgolem、文件職責與結論回填路由，主庫玩法 RE 閘門保持關閉。288 同次 DRAFT＋索引，有限唯讀完整 R／段與公開 Intel 計數／旗標契約審查 READY 後才實作。只新增裸 C1 /0 dword 暫存器形式，零計數全部保持，其他形狀與既有 word／ROR 範圍不擴張；多位 OF 未定義、保留明列工具近似。

全部 CPU 初次通過，審查補強保持旗標的全清／全設初態後，同容器／命令乾淨重跑通過；八目的／256計數／所有 bit 與補集／CF／OF 四組合及保持旗標兩初態、完整外層／拒絕／既有 word／ROR 已驗。固定官方 EXE 全套通過，兩自然排程自行完成三筆 ROL／CF 與下一 word 比較消費，限定 CONFORMED。第 26,396,706 步轉停高位 LE 0x2545EF 的 byte CL NEG；IRQ0 completed=2810、等待值1216，主選單／正常玩家路徑與整款 remake 未完成。

首輪索引／稽核路徑猜錯，改查實際 000-index.md 與 apps/moo2/tools/startup_probe_131.py 後繼續；是讀取問題，未另建映像或退回主機執行。精確命令、原始定位／bytes、輸入及 CPU／全套／兩自然收據雜湊集中於規格 288，287 舊停點／索引與回填護欄同步維護。原版素材與完整終端／記憶體／gzip／PNG 留本機；提交前核對回填／繁體字／UID／Docker 清理與權利輸入，沿用 github 隔離分支推送授權，不推本機 origin。


全部31個回填函式、新四項缺證據負例／CLI、兩排程三筆 ROL／完整目的外 R／段／CF 與下一比較通過；索引／語法／繁體字／UID／無誤建目錄及 git diff --check 通過。本批一次性 Go 容器已退出移除，未清理其他專案或映像。原版輸出仍忽略，不加入公開提交。

提交前差異檢查找到新增空白行的尾空白；修正後才暫存與提交，失敗閘門已停止後續步驟。

暫存後檢查另找到新規格表格前的尾空白，先前未暫存檢查不涵蓋新檔；第二次失敗重查文件職責／回填路由，修正新檔並以暫存差異檢查重跑，兩次均未繼續提交。


## 2026-10-02：byte 暫存器 NEG 與原版自然消費

288 工具 c605a7f13e00c061d09f14bbf7583c201335f980／主庫 5bc5b26e2846232e54b9a8e52af54b01bb13475b 已推送並回讀一致。沿用規格／dosgolem／文件與回填路由，289 同次 DRAFT＋索引，完整唯讀初態與公開 Intel 六旗標審查 READY 後實作。只新增裸 F6 /3 byte 暫存器形式，目的外完整狀態保持；主庫玩法 RE 閘門不變。

READY 修改前自動核准審查逾時，工具明示可重試一次；唯讀核對確認未執行，重試後成功。首次 CPU 全套遇舊測試把新支援的 /3 NEG 當未知 group，改以仍未支援的 /2 byte NOT 保留拒絕護欄，同映像／命令乾淨重跑通過，失敗收據保留。

八目的／256來源／64算術旗標初態、完整保持／拒絕／既有 F6 與 F7 NEG 回歸通過；固定官方 EXE 全套通過。兩原版自然排程自行完成三筆 byte NEG、完整目的外 R／段／六旗標與 MOV DL,CL 消費，289 限定 CONFORMED。第 38,427,368 步轉停高位 LE 0x254A04 的 D2 E5 SHL CH,CL；IRQ0 completed=5346、等待值3752。主選單／正常玩家路徑、音效／受控亂數與整款 remake 未完成。

精確命令、原始定位／bytes、輸入及全部收據雜湊集中於規格 289；288 舊停點／索引同步回填。原版素材與完整終端／記憶體／gzip／PNG 留本機，只有自製來源／測試／有限診斷與文字證據公開。提交前核對回填護欄、繁體字／UID、權利輸入與 Docker 清理；沿用 github 隔離分支推送授權，不推本機 origin。


全部32個回填函式、新四項缺證據負例／CLI、兩排程三筆 NEG／完整目的外 R／段／六旗標與 MOV 消費通過；索引／語法／繁體字／UID／無誤建目錄通過。本批一次性 Go 容器已退出移除，未清理其他專案或映像。


## 2026-10-02：CL 計數 byte 左移與記憶體 OR 資料鏈

基線工具 78738dc77eeaee6a1578f7ee98027d679408226f／主庫 0bdc75854b86eb8311eabea5002e314cbba007ee 乾淨；上輪 ROL／NEG 已推送、屬實際進展。沿用規格閘門／dosgolem、文件職責及結論回填路由與逆向重製技能，主庫玩法 RE 閘門保持關閉。

290 同次 DRAFT＋索引，完整唯讀 CH／CL／下一 OR 目的初態與公開 Intel 計數／旗標／未定義近似審查 READY 後才實作。重用既有 D0／C0 byte shift，只加裸 D2 /4、八目的與全部 CL 計數，保留 CL／CH 別名語意。CPU／固定 EXE 全套通過，但原版停下一記憶體 OR；290 保持 READY，未把尚未消費的資料鏈冒稱完成。

291 同次 DRAFT＋索引，以兩份未改 OR 的自然初態、完整 R／段／byte 目的及公開 OR 契約審查 READY 後實作。byte 寫回成功才發布五定義旗標，AF 清除沿既有工具近似；全部 CPU、固定官方 EXE 全套及兩自然排程三組 SHL→OR→ADD 完整資料鏈通過，290／291 才限定 CONFORMED。啟動推進至第 39,983,174 步，下一缺件 F3 AF 的 REPE SCASD；IRQ0 completed=5675、等待值4081，主選單／正常玩家路徑與整款 remake 未完成。

精確命令、原始定位／bytes、輸入／源碼雜湊與全部收據集中於規格 290／291；289／290 舊停點與索引／回填護欄同次維護。首次讀取查 C0 分支的字面字串未命中，改實際 D0／C0 共同分支後繼續，屬讀取問題；沒有另建映像或退回主機執行。原版素材與完整終端／記憶體／gzip／PNG 留本機，只有自製來源／測試／有限探針與文字證據公開。提交前核對回填護欄、繁體字／UID、權利輸入與 Docker 清理，沿用 github 隔離分支推送授權，不推本機 origin。


全部34個回填函式、兩份新規格各四項缺證據負例／CLI、兩排程三組 SHL／OR 寫回／ADD 與完整 R／段／定義旗標通過；索引／語法／繁體字／UID／無誤建目錄與 git diff --check 通過。本批一次性 Go 容器已退出移除，未清理其他專案或映像。所有回歸首次通過，未增加位址／資料特例。

## 2026-10-02：REPE SCASD 與原版真實讀取消費

工具基線0c6d53b869cb52fd716d95c868326244614abbf1，主庫紀錄fbedb135060c021b9dac8ff70007f669c784c68c已推送回讀一致。沿用平台規格優先／規格閘門、dosgolem、文件職責與回填路由。292同次 DRAFT＋索引，未改 CPU 的完整 R／段／8192 bytes掃描資料與公開 Intel契約審查 READY 後才實作。只接32位F3 AF；零計數不讀資料，六算術旗標全定義，初始ZF不限制第一次比較。主庫玩法RE閘門不變。

Intel80386 REP頁面偽碼退出條件與文字矛盾，以正式SDM2B確認比較後ZF=0退出。READY文件寫入的自動權限審查逾時且未執行，改用工作區檔案編輯完成授權範圍；沒有退回主機執行測試。先前讀取F3分支字面搜尋未命中，改實際前綴迴圈定位，屬讀取問題。

全部 CPU與固定官方EXE全套首次通過。兩自然排程從未改CPU的8192 bytes資料獨立推導並比對，原版比較59次後發布ECX=7C5h／EDI=6BBD4Ch／flags=206h，再由SUB EDI,4及MOV EAX,[EDI]讀取FFFFFBFFh；完整其餘R／段保持。292限定CONFORMED，第39,983,178步轉停高位LE0x25489C的83 F0 FF，暫存器XOR／符號延伸imm8缺件。IRQ0completed=5675／等待值4081，主選單／正常玩家路徑、音效／受控亂數與整款remake未完成。

精確命令、原始定位／bytes、輸入／來源與全部收據雜湊集中於規格292；291舊停點、索引與回填護欄同步維護。單次Step不含內部IRQ，硬體時鐘與Error／restart為明示工具模型，沒有逐週期聲明。原版素材及完整終端／記憶體／gzip／PNG留本機，只有自製來源／測試／有限探針與文字證據公開。提交前核對回填、繁體字／UID、權利輸入與Docker清理；沿用github隔離分支推送授權，不推本機origin。

全部35個回填函式、新四項缺證據負例與 python3 apps/moo2/tools/startup_probe_131.py --check-repe-scasd-spec-backlinks 通過；兩排程完整 SCASD→SUB→MOV 資料鏈、索引／語法／繁體字／UID／無誤建目錄及 git diff --check 通過。docker ps -a 的映像與專案名稱分開核對，本批一次性 Go 容器已退出移除，未清理其他專案或映像。

## 2026-10-02：dword XOR 與 BSF 的原版資料鏈

主庫aa598d09ff9d0394d4e0f2b0bf5485ebe3e30cf3／工具e914184166c2397dba5041617793a58e42f2f927乾淨。上一輪已提交推送SCASD與真實資料鏈，屬實際進展。沿用平台規格優先／規格閘門、dosgolem、文件職責與回填路由、逆向重製技能，主庫玩法RE閘門保持關閉。

293同次DRAFT＋索引，未改CPU的完整R／段與公開XOR符號延伸／定義五旗標及AF清除近似審查READY後實作。首次CPU失敗是新附帶ADD回歸把低四位進位的AF預期算錯，修正207h且CPU未改，同映像／命令乾淨重跑全部通過，失敗收據保留。全部CPU與固定官方EXE全套、兩自然XOR後態通過，但BSF缺件，293保持READY。

294同次DRAFT＋索引，以兩份未改BSF的完整原版初態與正式Intel SDM的ZF／未定義目的和旗標邊界審查READY後實作。只接裸32位暫存器來源／目的，來源目的別名安全；五未定義旗標與零來源目的保留是工具模型。全部CPU／固定EXE全套及兩自然XOR→SHL→BSF→ADD→word MOV完整鏈通過，293／294才一併限定CONFORMED。原版把索引Ah消費成EDI=74Ah並存到DS:002726D0，第41,223,220步轉停0x23C36B的09記憶體dword OR；IRQ0 completed=5936、等待值4342，主選單／正常玩家路徑及整款remake未完成。

80386 BSF網頁轉錄的ZF文字與偽碼矛盾，以正式SDM2A核對；只有ZF可稱定義旗標。首次C1文字搜尋假設獨立case未命中，改實際C1／D1共同分支；image inspect缺Entrypoint欄位改讀實際Config後沿用同映像，沒有重建工具鏈。一次編輯呼叫的JS字串語法錯誤未執行，以完整原始字串修正；這些均為工具／讀取問題，沒有退回主機工作負載。

精確命令、固定輸入／來源、原始定位／bytes、測試與兩自然收據SHA集中於293／294；292／293停點與索引／回填護欄同次維護。原始ZIP／patch／EXE雜湊已重查一致；原版素材與完整終端／記憶體／gzip／PNG留本機，公開只有自製來源／測試／有限診斷與文字證據。沿用github隔離分支推送授權，不推本機origin。

全部37個回填函式、293／294各四項缺證據負例及 --check-xor-dword-imm8-spec-backlinks／--check-bsf-dword-spec-backlinks 兩CLI通過；兩自然完整資料鏈、索引／語法／繁體字／UID／無誤建目錄與git diff --check通過。docker ps -a的Go映像與moo2名稱分開核對，本批一次性容器已退出移除，未清理其他專案／映像。

## 2026-10-02：記憶體 dword OR 與原版完整讀取

主庫0b0faf6df8f46823eab81f620aa380e670e56923／工具a899e9e7e4f397d17054bdccae97faa0f6cf963c乾淨且遠端一致；上一輪XOR／BSF資料鏈已推送，屬實際進展。沿用平台規格優先／規格閘門、dosgolem、文件職責與回填路由及逆向重製技能，主庫玩法RE閘門不變。

295同次DRAFT＋索引，未改CPU的完整R／段／目的dword及公開OR／五定義旗標、AF清除與逐byte錯誤模型審查READY後才實作。只將09 /r記憶體解碼改用既有decodeAddress32；CPU與固定官方EXE全套通過，兩自然40h→2040h寫回及原版MOV EAX讀取完整2040h後，才限定CONFORMED。沒有遊戲位址特例，也沒有修改主庫玩法。

首次CPU失敗是測試借用的Bus拒絕全部byte寫入，改指定byte才失敗的測試Bus，CPU不變，同映像／命令乾淨重跑通過。第一次自然診斷包裝Bus破壞DPMI的Bus身分契約，兩份提早INT31/0500h收據不計OR驗收；改用原樣轉送SegmentRead8返回值，保留Bus，CPU與DPMI不改再跑。最早只觀察到未變的高byte，不當成新增bit13的消費，收窄低兩byte／dword取址端後兩自然真的由MOV EAX讀取完整2040h。精確命令、版本、所有有效／無效收據雜湊及錯誤模型在295；未把診斷錯誤列為產品缺陷。

兩自然第42,347,254步停高位LE0x2454AE的INT31/0300h，實際內層INT66從1201:016A行230步到1201:05D9，OUT 022Ch／C6h尚未支援。IRQ0 started=completed=6173、等待值4579，最終PNG同已檢視黑圖；255／主選單／正常玩家路徑、音效／受控亂數及整款remake未完成。下一步公開SB16 DSP／DMA與sample duration契約加有限原版參數，審查READY再實作；不深挖driver／ISR／DAC／PIT時序。

293／294舊停點與索引／回填護欄同次維護，原始素材與完整終端／記憶體／gzip／PNG留本機，只公開自製來源／測試／有限診斷及文字證據。沿用github隔離分支推送授權，不推本機origin。

全部38個回填函式、293／294兩份各四項缺證據負例、--check-or-dword-memory-spec-backlinks CLI、兩自然完整OR／JMP／MOV／TEST與新停點的獨立算術核對通過。原始ZIP／patch／固定EXE雜湊再核對一致，索引／語法／繁體字／UID／無誤建目錄及git diff --check通過。Docker按Go映像與moo2名稱分開核對，本輪一次性容器已退出移除，未清理其他專案或映像。讀取環境無rg改grep；幾次不存在檔案的讀取假設及一次同檔多段patch拒絕均未改檔，依已定位來源修正。

## 2026-10-02：SB16 C6h 命令與原版成功返回

開工主庫eac4a998ba1a4dc4d0a515eb9ab510c3670652da／工具418ca3cf6d874e24127da24c0ba66e9fafecf6e7乾淨，上一輪OR與完整MOV消費已推送。沿用平台規格優先／規格閘門、dosgolem、文件職責與後續回填路由及逆向重製技能，主庫玩法RE閘門不變。

296同次DRAFT＋索引，有限非自然參數探針取得C6 20 FF 07／22050Hz／4096byte DMA ring；第4byte強制拒絕，不當自然成功。公開Creative命令／每channel與TimeConstant、sample duration／block與ring區分審查READY後才實作，未深入driver／ISR／DAC／PIT／DMA逐週期。專用C6回呼、四mode／完整length與8位DMA格式／取樣計數接通，CPU與其他命令不改。

全部CPU／機器層與固定官方EXE全套首次通過，獨立取樣數、原版形狀46440µs首block與92880µs ring重載、IRQ隔離／ack／EOI／IRET、mask／reset／拒絕／來源邊界已驗。兩原版自然排程接受完整C6並實模式333步返回，原版MOV EBX讀成功值0、CMP／JZ進0x2454E7成功分支，完整外層R／段／旗標及50byte封包差異已驗，296才限定CONFORMED。

第一份正式來源診斷誤取尚未設定的呼叫前DMA，改取返回後base／page；舊收據保留而不當C6來源證據，同映像／命令重生兩自然。來源ring確為物理14000h／4096byte／全80h；返回只有21µs、信用926100，小於第一sample門檻，PCMBytes=0。原版保護模式連續PCM／IRQ7仍未知，條件實模式pattern測試不能替代音訊／人耳或玩家路徑驗收。唯讀知識載入的自動核准審查逾時後依工具指示重試一次，成功；一筆文件編輯JS字串錯誤未執行，修正後繼續。沒有退回主機工作負載或另建映像。

兩自然第42,347,639步轉停高位LE0x257662的C1 CA 10，dword ROR立即數10h缺件；IRQ0 started=completed=6173、等待值4579，PNG同已檢視黑圖。255座標／游標、主選單／正常玩家路徑、保護模式音訊／IRQ7、受控亂數與整款remake未完成。下一步為公開ROR計數／旗標契約與有限唯讀初態，審查READY後補窄CPU形式，不猜遊戲欄位用途。

精確命令、輸入／工具／來源與所有有效／參數／診斷更正收據SHA-256集中於296；293／294／295舊停點與索引同次回填。全部39個回填函式、三份舊停點共15項缺證據負例、--check-sb16-c6-spec-backlinks與兩自然獨立時間／封包／完整caller／新停點稽核通過。原版ZIP／patch／EXE雜湊再核對，原版素材及完整終端／記憶體／gzip／PNG留本機，只提交自製來源／測試／有限診斷與文字證據。沿用github隔離分支推送授權，不推本機origin。

來源／輸出UID:GID1000:1000、工具無root-owned／誤建.md目錄及git diff --check通過。本批一次性Go容器已退出移除，moo2名稱無殘留；同Go映像的0da5d78f687c掛載皆屬fd2，保留其他專案工作。未清理其他專案或映像。

## 2026-10-02：dword ROR 全部立即數與原版 MOV 消費

開工主庫813ced925045f3bb756983378af2a0551da18e0d／工具56e1ff979b517740bf056668ee90800bfac27321乾淨；上一輪C6已推送並完成原版返回／caller驗證，屬實際進展。沿用平台規格優先／規格閘門、dosgolem、文件職責及結論回填路由與逆向重製技能，主庫玩法RE閘門不變。

297同次DRAFT＋索引，未改CPU的兩自然完整R／段及公開Intel計數／CF／單位OF與多位OF保留近似審查READY後實作。既有C1 /1由固定08h擴成全部imm8遮罩計數，不加原版位址特例；原來07h負例改由全部計數正例驗，其他拒絕護欄保持。全部CPU與固定官方EXE全套首次通過；八目的／256計數／所有bit與補集／旗標及完整保持、拒絕／截短與既有D1／ROL路徑已驗。

兩自然各兩組ROR的EDX仍0AFF0AFFh，CF清除、模型flags296h，其他完整R／段保持；MOV AX,DX真實消費後完整EAX=00340AFFh／0A0A0AFFh，第二ROR亦完成。獨立單bit整除oracle及自然收據核對後297限定CONFORMED。多位OF未定義保留僅屬工具模型，不稱硬體OF逐值一致。

原版前態命令尾端SHA摘要錯列未建的full-test-297-input.txt而exit1，兩自然執行／gzip／PNG完整；獨立讀取核對後繼續，未把腳本問題寫成CPU或產品失敗。兩次初讀猜錯288與fixture檔名，依實際檔名補讀；沒有退回主機分析或另建映像。

第42,349,111步外層0x2571C9的StepHook內，原版IRQ0呼叫到高位LE0x2520B7的D1 E0，SHL EAX,1缺件；內層完整R／段與flags2已保存，外層EIP不當缺件位址。IRQ0 started6174／completed6173／failed=true，等待值仍4579，PNG同已檢視黑圖。只補公開標準CPU契約，不深入ISR／driver硬體時序；277舊返回樣本不代表所有分支閉合。255／主選單／正常玩家路徑、保護模式PCM／IRQ7、音訊／人耳／受控亂數及整款remake未驗收。

222／288舊計數限制與293／294／295／296停點同步回填，全部40個回填函式、六份舊範圍／停點共24項缺證據負例、--check-ror-dword-immediate-spec-backlinks及兩自然完整ROR／MOV／IRQ0新停點獨立稽核通過。精確命令、輸入／來源／工具與所有收據SHA-256集中於297；原版ZIP／patch／EXE固定雜湊再核對。原始素材與完整終端／記憶體／gzip／PNG留本機，只公開自製來源／測試／有限診斷與文字證據，沿用github隔離分支推送授權，不推本機origin。

來源／輸出UID:GID1000:1000、工具無root-owned／誤建.md目錄與git diff --check通過。docker ps -a依Go映像與moo2名稱分開核對，皆無殘留；本批一次性容器已退出移除，未清理其他專案／映像。

## 2026-10-02：IRQ0 內單位 SHL 與 C1 旗標修正

開工主庫9835225d47eda25f726c87e624051362669d9f62／工具4cdf20e347500a5c996b955831e35ef68ca556e0乾淨，上一輪ROR已推送。沿用平台契約／規格閘門、dosgolem、文件職責與結論回填路由及逆向重製技能；本輪只補標準CPU阻塞，主庫玩法RE閘門不變。

298同次DRAFT＋索引，以未改CPU的兩自然真正IRQ0前態、公開Intel五定義旗標與AF模型審查READY後，補裸D1 /4的八dword暫存器。唯讀StepHook包裝原樣轉送既有hook，不替換CPU／Bus／IRQ橋接或時計、不跳指令；固定九位址與有限記憶體各最多三筆，不深入handler／ISR／driver硬體時序。

第一次完整CPU回歸找到既有C1 E0 01、EAX80000001h的OF漏設，結果2／flags603h，公開契約正確預期2／flagsE03h。D1新測試通過，不更改正確預期；保留失敗收據。299獨立DRAFT／公開CPU反例審查READY後，只補裸C1 /4／/5／/7遮罩計數1 OF，零計數保持、非零AF及多位OF清除模型保持。

同Go1.24.13映像／命令乾淨重跑全部CPU通過，固定官方EXE全套通過。298獨立倍增／整除與PF計數、全部低16位／bit及補集／64初旗標、完整保持與拒絕／既有word／C1／D3等路徑已驗；299三group／八目的／256計數／72值／八初旗標獨立逐步oracle通過。未有MOO2自然OF=1同狀態收據，299完成限公開CPU契約修正，未定義旗標不稱硬體逐值一致。

兩原版自然各七筆完整IRQ0內狀態：EAX0→0、flags46h；EBX1→2、flags2；TEST來源3 AND 8=0，JZ跳過第二組SHL，原版A3／89 1D分別存EAX0／EBX2至DS:00272D40／DS:00272D44。目的八bytes由全0到00000000 02000000，只有EBX有數值突變。其他完整R／段保持；獨立核對真正寫回後298限定CONFORMED，沒有猜欄位用途或用孤立測試取代自然分支。

同outer_step42349111仍在0x2571C9的IRQ0呼叫內，下一缺件移至高位LE0x25179F的13 ED、ADC EBP,EBP。IRQ0 started6174／completed6173／failed=true，等待4579、PNG同已檢視黑圖；沒有把37µs工具時鐘差稱硬體時序對齊或聲稱整個IRQ0返回。255座標／游標、主選單／正常玩家路徑、保護模式PCM／IRQ7、人耳／受控亂數及整款remake未完成。

293–297舊SHL停點與186／298的C1契約／回歸發現同步回填，42個回填函式／28項新缺證據負例、兩個CLI、兩自然完整狀態／分支／寫回與新停點獨立稽核通過。初次收據稽核腳本正規表示式跳脫錯誤，修正後以同一收據重跑通過，未當產品失敗。精確命令、來源／CPU／probe／測試及所有有效／失敗收據SHA-256集中298／299；原始ZIP／patch／417根檔／EXE／MOX.SET再核對一致。原版素材及完整終端／記憶體／gzip／PNG留本機，只提交自製來源／測試／有限診斷及文字證據，沿用github隔離分支推送授權，不推本機origin。

來源／輸出UID:GID1000:1000、工具無root-owned／誤建.md目錄與git diff --check通過。docker ps -a依Go映像與moo2名稱分開核對皆空，本批一次性容器已退出移除，未清理其他專案／映像。下一步只按公開ADC契約與有限唯讀前態／下一消費建立窄CPU規格，不深入IRQ／driver或runtime。

## 2026-10-02：IRQ0 內 ADC 與索引 ADD 真實消費

開工主庫df423d7dd09a9ba202c8051bd40045ed513a4ca0／工具a44c7eaae7f4ac3143a04f183f62ecf91190e124乾淨，上一輪SHL／C1修正已推送並驗真實寫回，屬實際進展。路由命中平台契約／規格閘門、dosgolem、文件職責及結論回填，實際載入入口與上游能力矩陣／分層規格，沿用逆向重製技能。初讀006檔名不符後依實際檔名補讀，未當產品失敗；主庫玩法RE閘門不變。

300同次DRAFT＋索引，以未改CPU的兩自然真正IRQ0前態及公開Intel ADC六定義旗標／別名／保持與拒絕契約審查READY後，補裸13 /r、mod11。四個固定位址各最多三筆唯讀StepHook原樣轉送既有hook，只讀完整R／段／flags／IRQ狀態與八byte來源及原始bytes，不替換CPU／Bus／時計／IRQ橋接、不注入資料或跳指令。

原兩操作數與CF一起用33位總和計算，CF／OF／AF按完整運算、SF／ZF／PF按結果；來源目的相同仍消費舊值。64來源／目的組合、全部byte配對／兩CF、77項bit／補集／符號／繞回／低nibble邊界與64算術初旗標的獨立無號總和／有號範圍／PF計數通過，完整保持、截短／未知前綴／記憶體拒絕及既有ADD／word ADD／SUB／CMP／SBB回歸通過。全部CPU／固定官方EXE全套首次通過。

兩自然各三組完整ADC→索引ADD→MOVSX相同，全部outer_step42349111在同次IRQ0內，不冒稱IRQ內指令序號或總呼叫數。原CF1／0／1令EBP1／0／1、flags2／46h／2；原版ADD以×4索引讀DS:00272D44的dword2或DS:00272D40的dword0，完整ESI為0071E1D2h／0071E1D2h／0071E1D4h、flags6。目的外完整R／段保持，後續MOVSX才將EBP覆寫0；獨立核對真正來源／索引與結果後300限定CONFORMED，不猜欄位用途或深入IRQ0 handler。

同outer_step42349111在外層0x2571C9的IRQ0內轉停高位LE0x24678C，66 83 F7 01、word XOR DI,1缺件。IRQ0 started6174／completed6173／failed=true，完整返回仍未知；等待4579／PNG同已檢視黑圖，工具時計不當硬體wall-clock證據。255／主選單／正常玩家路徑、保護模式PCM／IRQ7、人耳／受控亂數及整款remake未完成，299的原版自然OF=1限制保持。

293–299舊停點與索引同步回填，43個回填函式／七份舊停點28項缺證據負例、--check-adc-dword-register-spec-backlinks CLI及兩自然三組完整資料鏈獨立稽核通過。全部命令／來源／CPU／probe／新測試與前態及有效收據SHA-256集中300；原始ZIP／patch／417根檔／EXE／MOX.SET再核對一致。原版素材及完整終端／記憶體／gzip／PNG留本機，只提交自製來源／測試／有限診斷與文字證據，沿用github隔離分支推送授權，不推本機origin。

來源／輸出UID:GID1000:1000、工具無root-owned／誤建.md目錄及git diff --check通過。docker ps -a依Go映像與moo2名稱分開核對皆空，本批一次性容器已退出移除，未清理其他專案／映像。下一步按公開word XOR立即數的低16位／符號延伸／旗標契約與有限唯讀前態／PUSH消費建立窄CPU規格，不深入IRQ／driver硬體時序或runtime。

## 2026-10-02：word XOR 與原版 PUSH 堆疊消費

開工主庫702b6b39f18312b28603d7a7334feb1143ada2e7／工具752abc607e04b87d830d4a6451af9e51dfc26b78，上一輪ADC索引消費已推送。路由命中平台契約／規格閘門、dosgolem、文件職責與結論回填，沿用已載入入口及逆向重製技能。301同次DRAFT＋索引，以未改CPU的兩自然真正完整IRQ0初態、公開word XOR五定義旗標／AF清除模型及下一PUSH契約審查READY後才實作；主庫玩法RE閘門不變。

只補66 83 /6、mod11的八word目的與全部imm8符號延伸，目的高16位及其他狀態保持，截短／未知前綴／word記憶體拒絕。四固定位址各最多三筆唯讀StepHook保存完整R／段／flags與既有八byte堆疊，原樣轉送既有hook，不替換CPU／Bus／時計／IRQ橋接、不跳指令或注入資料；不追後續CALL目標／IRQ0 handler／ISR／driver硬體時序。

第一次全部CPU的301新指令與PUSH測試通過，舊293仍拒絕已合法word形式、新byte XOR測試誤填保留AF而失敗。依既有286清AF模型修正新增期望，267／293的word XOR舊負例由全部word正例接替，未知拒絕仍保持；CPU未再次修改。保留失敗收據，同映像／命令乾淨重跑全部CPU通過，固定官方EXE全套通過。全部低word／imm8配對、八目的45邊界／256立即數／64初旗標，以逐bit比較／整除／低byte位元計數獨立核對高16位、五旗標及AF模型與完整保持；拒絕／既有指令回歸已驗。

兩自然各一組完整XOR→PUSH EDI→PUSH EAX相同，真正前態與未改CPU收據一致。XOR令完整EDI=1／flags2，下一PUSH EDI令ESP2723FCh、SS:002723FC完整dword從003D6978h變1；PUSH EAX令ESP2723F8h、SS:002723F8從00246752h變00325048h。兩PUSH其餘R／段與flags保持，八bytes由52672400 78693D00到48503200 01000000；完整自然消費獨立核對後301限定CONFORMED。

這次IRQ0成功返回，兩自然第42603292步轉停根CPU高位LE0x256171的66 93、XCHG AX,BX，比前一停點多254181步。IRQ0 active=false／failed=false／started6228／completed6228，等待4634；工具時鐘不稱硬體wall-clock對齊。VBE indexed已有變化，PNG仍同已檢視黑圖。C6返回／來源收據保持，255／主選單／正常玩家路徑、保護模式PCM／IRQ7、人耳／受控亂數及整款remake未驗收；299原版自然OF=1限制保持。

293–300的八份舊停點及索引同步回填，全部44個回填函式／32項新缺證據負例／CLI、兩自然完整XOR／PUSH真實寫入／IRQ0返回／新停點獨立稽核通過。來源／CPU／probe／新測試、前態、失敗及成功收據SHA-256與精確命令集中301；正版ZIP／patch／417根檔／EXE／MOX.SET雜湊再核對一致。初讀猜錯OR測試檔名後依實際檔名補查，未當產品失敗。原版素材及完整終端／記憶體／gzip／PNG留本機，只公開自製來源／測試／有限診斷及文字證據，沿用github隔離分支推送授權，不推本機origin。

來源／輸出UID:GID1000:1000，工具無root-owned／誤建.md目錄與git diff --check通過。本批一次性容器均已退出移除；Go映像清查曾見短暫容器，讀取掛載前已自行移除，歸屬未確認；未停止或移除它，也未清理其他專案／映像。下一步只保存XCHG真正完整前態及下一ROR消費，按公開CPU契約審查窄word暫存器形式；不追helper內部或修改主庫玩法。


## 2026-10-02：word XCHG 與完整 ROR 消費

開工主庫b0b5a4ab86594466673df7f2f6f192cead05f3b8／工具d013029fcddf4da8e8d7650660897223e6fa67ca乾淨，上一輪word XOR／PUSH及原版IRQ0返回已推送，屬實際進展。路由命中平台契約／規格閘門、dosgolem、文件職責與結論回填，實際載入入口及能力矩陣，沿用逆向重製技能；初讀081檔名不符後依實際檔名補讀，沒有當產品失敗或另建映像。主庫玩法RE閘門不變。

302同次DRAFT＋索引，以未改CPU的兩自然真正完整初態及公開Intel交換／兩高16位／全部旗標保持契約審查READY後，只補66 91–97。三固定位址各最多三筆唯讀StepHook原樣轉送既有hook，不替換CPU／Bus／時計／IRQ橋接、不注入資料或跳指令。七目的／全部AX低word與對側同值、補集／45項bit及符號邊界配對／64初算術旗標與32旗標bit、補集，以整除拆原高低word獨立核對保持與拒絕／既有byte／dword XCHG及NOP。全部CPU與固定官方EXE全套首次通過。

兩自然各三組完整XCHG→ROR→RET邊界相同，第一初態與未改CPU收據一致。第一XCHG令完整EAX0A0A0A0Ah／EBX2E0A0A2Eh、flags206h，兩高16位與其他完整R／段保持；下一ROR EBX,8真實消費完整來源，結果2E2E0A0Ah。後兩組完整來源100A0A0Ah與AX0A10h亦驗交換及下一ROR結果10100A0Ah；原值交換與逐次整除循環獨立核對後302限定CONFORMED。多位OF未定義保留只屬297工具模型，RET只記邊界，不追helper／caller內部。

兩自然持續到50M診斷上限，根CPU高位LE0x22FCD2、相同完整R／段／flags246h，沒有step_error或guest_cpu_stop；不把上限當CPU拒絕。IRQ0 active=false／failed=false／started7789／completed7789，等待6195。VBE圖像已變化且兩PNG雜湊相同，已檢視星空／星雲片段，尚未見主選單；C6命令／handled返回與成功caller／來源SHA保持，保護模式連續PCM／IRQ7、人耳未知。工具時鐘不稱硬體wall-clock一致；255／正常玩家路徑／受控亂數、299原版自然OF=1與整款remake仍未驗收。

首次收據稽核誤要求無事件／受控事件unique_sites都17667，實際受控事件17697，完整R／段／圖像與XCHG消費相同。依實際輸入分支覆蓋值修正腳本後，以同一收據重跑通過，沒有修改CPU／重跑原版或當產品失敗。293–301九份舊停點與索引同步回填，45個回填函式／36項新缺證據負例／CLI及兩自然完整消費／IRQ持續返回與上限獨立稽核通過。全部來源／CPU／probe／測試、輸入與有效收據SHA及精確命令集中302；正版ZIP／patch／417根檔／EXE／MOX.SET雜湊再核對一致。

來源／輸出UID:GID1000:1000，工具無root-owned／誤建.md目錄及git diff --check通過；本批一次性容器均已退出移除，Go映像與moo2名稱清查皆空，未清理其他專案或映像。只公開自製來源／測試／有限診斷與文字證據，原版素材及完整終端／記憶體／gzip／PNG留本機，沿用github隔離分支推送授權，不推本機origin。下一步先有限唯讀觀測正常啟動流程的等待／鍵鼠入口，再用原版正常輸入重播；不猜修等待條件，不追helper／runtime／driver／ISR硬體時序或改主庫玩法。


## 2026-10-02：晚期啟動的唯讀平台觀測

開工主庫de386e046750f146e86fd88f8da569f22c9b6d77／工具9e6ee8cea7e40fdf13528fdae6b7959361708aaa乾淨。路由命中平台契約、正常玩家路徑及dosgolem，載入入口，沿用逆向重製技能與文件職責／結論回填契約。主庫玩法RE閘門不變，CPU與平台實作未改；303 DRAFT與索引同次建立，302追加連結。

探針只加四個固定sample、三caller各最多兩筆及terminal的值快照，原樣轉送既有hook，不注入資料／鍵盤或提高50M上限。兩自然各11筆完整CPU／段／旗標、BIOS／音訊時計、DMA／IRQ7／PCM、鍵鼠與24原始bytes。兩次確認BIOSMicros43985659到52095937，音訊VirtualMicros固定1375，DMA8已啟動但block剩2048、completions0／PCMBytes0／credit926100固定。先前實模式DMA16Completions1／PCM16Bytes2／IRQ7Deliveries1保持，不稱從未派送。靜態接線與自然快照確認保護模式裝置時計未推進，未證實它造成原版等待。

所有快照鍵盤未安裝／讀取與入隊零、60／61／64埠讀取零、BDA佇列空；原版DOS AH2509以8:21C4D8安裝保護模式IRQ1，工具正常鍵盤入口尚未接通。受控滑鼠事件回呼完整返回，未注入按鍵，不用BIOS入隊猜補原版IRQ1契約。caller原始0x2A8E54四bytes零值已確認，欄位語意與等待原因仍未知。

獨立稽核兩次完整收據與302一致，只排除新增快照／PNG輸出名／解壓mtime與DOS DTA日期時間四bytes。CPU及三份平台來源雜湊不變、三組XCHG／ROR／完整初終態／IRQ與圖像均保持；兩次50M無CPU拒絕、IRQ0 started7789／completed7789，PNG同已檢視星空片段，主選單未見。唯讀探針已自然編譯驗證，不重跑未變CPU全套。來源／輸入／命令／收據雜湊集中303；觀測完成而平台規格仍DRAFT，255／299自然OF=1／正常玩家路徑／人耳與整款remake仍未驗收。

嘗試以既有moo2-ebiten讀官方本機手冊，pdftotext與三個PDF解析模組皆未安裝；只分類為工具環境缺件，未啟動主機分析、未另建映像或猜按鍵。它不阻塞此次唯讀觀測。原版素材、完整終端／記憶體／gzip／PNG留本機，只推送自製觀測與文字證據。下一步先審查公開DMA／PIC時間與IRQ7呼叫邊界的窄平台契約，READY後實作，不深入driver／ISR／busy-wait或改玩法。

303兩次自然／完整收據／五來源雜湊／原版AH2509核對通過；既有45個回填函式／36個缺證據負例／CLI與新索引、DRAFT、繁體字及UID核對通過。工具無root-owned／誤建.md目錄、git diff --check通過，來源／輸出UID:GID1000:1000；本批一次性容器已退出移除，Go映像及moo2名稱清查皆空，未清理其他專案或映像。沿用github隔離分支推送授權，不推本機origin。


## 2026-10-02：共用裝置時間與保護模式IRQ7邊界

開工主庫4e77399672e4e5988e8081ece62e5572c2e27ba2／工具90b9f4acb3c7e55829973c51a39fe12cb2129df3乾淨。前輪303唯讀觀測／收據已推送，屬實際進展。路由命中平台規格優先、dosgolem、規格閘門與文件職責，載入入口，沿用逆向重製技能；主庫玩法RE閘門不變。初讀猜錯規格入口檔名後按路由實際retro-remake-spec-gated-workflow.md補讀，未當產品失敗。

304同次DRAFT與索引，重核303兩自然完整收據／五來源雜湊及原版C6返回，以Creative取樣率／block／中斷與Intel／Open Watcom模式框架公開契約審查READY後，才接兩CPU模式裝置時間。既有1µs近似／實模式順序保持，把DMA取樣拆成共用advanceDMA，InstallLEBIOSClock每保護步亦前進裝置。CPU及啟動來源不改；IF／PIC只阻擋派送，未建模保護模式IRQ7保留pending／CPU現場及原始三種向量明確停止，不用實模式框架猜轉送，不丟中斷跑過等待。探針只加失敗值快照。

新增測試交錯兩真實CPU指令，按總步數／rate／channels整除獨立核對取樣及兩時鐘、四mode／三rate、40h已含channels、DMA遮罩／reset／來源超界、16位單word與IRQ0巢狀指令。IF／PIC／in-service／IRQ0互斥及首次IRQ7停止的完整CPU／FPU／堆疊保持亦驗。首次機器測試因自製fixture漏設實模式CS=0，讀到重置段外、IP多前進一byte而失敗；核對cpu.Reset與實模式匯流排後只修測試初態，另驗bus.err，平台來源未改。失敗收據留本機，同映像／命令乾淨重跑機器層通過，SHA-256 bff247d74ad4200f631ebd7edcccfebed4f72a47f325b7359396fc2a2d6d7c0e；首輪失敗d54dfd73fd85f02a1da16cffa7406cd4f56651bff389c745fa0432e6f5fd4a14。

固定官方EXE全套首次通過，全部CPU亦通過；full-test-304.txt SHA-256 163b0f2bfa8e8ad2b6efe1f831c7ab35173e1682f82dbf134f4b9fe63c1c79c2。既有45個回填函式／36項缺證據負例／CLI保持。新原版自然收據仍執行中，不預先宣稱DMA首block或等待閉合。Go映像清查見其他FD2 oracle容器，確認掛載另專案後保留，未停止或刪除它。


2026-10-03自然驗證完成：兩次在第42356668步、高位LE0x257FC9首次待派送保護模式IRQ7明確停止，兩時計44032078，C6返回43985659後46419µs。由原信用926100及44100 byte/s獨立整除驗2048 samples、餘數4000；首DSP block完成1／DMA current4800h、count7FFh、block重載2048。首次只有PCM長度，補有限唯讀SHA／prefix及實際IRQ入口16bytes後同命令重生，完整收據只新增兩列／mtime／DTA時間日期四bytes，平台／CPU未改、不重跑未變測試。實際PCM2048個80h雜湊88ed1a04cb43fe65827d1cd9ef6d24a736108730b1ce6315d4d3ca79b6a0d140與已保存原版buffer一致，不當人耳驗收。

absolute IVT0F1201:0682／實模式線性0x12692、DPMI實模式0F與DOS保護模式0F零，入口16bytes保留；只保存定位，不追ISR內部。DSP／PIC pending=true、IRQ7Deliveries仍1，沒有假稱成功派送；IRQ0 started6176／completed6176、failed=false，主選單未見。PNG同較早黑圖，首次IRQ7在星空畫面之前停止；302／303的星空與50M只屬先前未推音訊時計基線。兩自然新gzip ec4abf4f565e6cf1ea3ec1b210e414b459d00a6ac8e80da0307e3c323dcb3de0／d89716bc0d1482670ea1eb5cf1709475ef31bbeff0930cb86af34ef407369fda，來源／失敗與有效收據雜湊／命令集中304。

十一份較早音訊邊界與索引同次回填，全部46個回填函式／80項缺證據負例／兩個CLI及完整自然時間／原始PCM／向量／pending核對通過。新增規格校對抓到兩個混用字後修正文件，同命令重跑通過，不改驗證期望或平台來源。CPU／啟動來源保持，來源／輸出UID:GID1000:1000，工具無root-owned／誤建.md目錄及git diff --check通過。304保持READY，時間到首block已驗，真正IRQ7轉送／返回仍未知；下一步只依公開DOS/4GW與實際IVT補窄派送契約，不跳指令、不修改等待值，不深入driver／ISR／busy-wait或改主庫玩法。完整原版素材、終端／記憶體／gzip／PNG留本機，只公開自製來源、測試與有限文字證據。

本輪測試及兩批自然重播的有界一次性容器均已退出移除，moo2名稱清查空；Go映像清查僅見已確認另一FD2專案的confident_williams，保留未操作。未清理其他專案或映像。沿用github隔離分支推送授權，不推本機origin。

## 2026-10-03：正常事件返回與首個上層邊界

起點2bfb2db0f860d115cb0e96e9d1e5938a89a25c23，330先DRAFT／READY，只加probe唯讀觀察。初次第二組遇到callee後的Jcc被標成caller，沒有採作正式結論；回到DRAFT補RET後首CALL立即停止、以ESP與新return核對，再READY。初次收據留忽略330-initial；同容器同命令乾淨重跑三正常流程。

Docker固定Go1.24.13映像／600s／2GiB／2CPU／128pids／UID1000／network none，原ZIP／patch唯讀重建417檔；go build -p 2 -buildvcs=false -o /tmp/moo2-probe ./workplace/moo2-probe，沿329環境換330輸出，python3 workplace/new-game-330-verify.py PASS。全部3847／4382／6322舊列除既定正規化保持，72PNG逐位元保持，新兩組各16實際步且50M／100M完整32列相同。3 RET／MOV與堆疊／TEST定義flags／非零JNE／CALL獨立核算PASS；CPU／平台／CLI保持，325固定EXE全套PASS有效。

事件DS:2A1228與2A1226均於放開後返回EAX1，20DB69 TEST／JNE跳20DB87；另一組雙RET後20DDF2 CALL209325立即停觀察。不支持未驗短按丟棄修法；NEW GAME命令與正常開局仍未知。下一步只追20DB87非零臂的按鈕判定／指令消費，不加cap或重點，不深入runtime helper。

67回填函式、既有32／49／25／27／27／34／31／36與新增31缺證據負例、--check-event-return-spec-backlinks／--check-bounded-new-game-spec-backlinks PASS。完整來源／收據SHA集中[330](docs/spec/330-moo2-event-return-caller.md)；原ZIP／EXE／MOX.SET與417檔再核對、gofmt／git diff --check／新來源及收據1000:1000、工具root-owned／誤建.md目錄自檢空。原版素材不提交；沿授權推github隔離分支，不推本機origin。主庫RE-first與整款remake／中文化目標保持。

Docker ps -a以主庫與隔離工具鏈兩掛載路徑篩選皆空，本輪無遺留容器，其他專案未清理。

## 2026-10-03：非零事件caller的範圍比較與正常返回

起點34d758498f931d9dc155c4ca93309dd98646328e，331先DRAFT／READY，只加有界caller觀察。第一次96步未保存判定表的來源bytes，回DRAFT補26C480／29BE0E與當步DS:[EAX]窗口，再READY。初次三收據留忽略331-initial；同Docker命令乾淨重跑兩50M與獨立100M正常流程。

固定Go1.24.13映像／600s／2GiB／2CPU／128pids／UID1000／network none，原ZIP／patch唯讀重建417檔，go build -p 2 -buildvcs=false -o /tmp/moo2-probe ./workplace/moo2-probe，沿330正常環境換331輸出。python3 workplace/new-game-331-verify.py PASS：全部3847／4414／6354舊列除既定正規化保持，72PNG逐位元保持。全部96 caller步／五自然CALL返回／原bytes與寫回獨立核算，兩預算完整新列相同。省略callee238步，沒有假裝逐步連續，不深入helper。96上限在index2讀完四word後停，未達caller RET。

自然返回x500／y229，實際表指標298848／count9／55byte stride，index1原範圍10／20／25／35，index2為20／30／35／45。第一筆因x500>25，20DCE7 JLE不跳、E9到20DDAE並將index1加成2；只證實第一筆跳過，其餘命中、NEW GAME指令與正常開局仍未知。下一步只核對index2..8同輪分支、caller返回與目前主選單關係，不改點擊時長或原流程cap。

68回填函式、既有32／49／25／27／27／34／31／36／31與新37缺證據負例、--check-button-branch-spec-backlinks／--check-event-return-spec-backlinks PASS。CPU／平台／CLI保持，325固定EXE全套與329 CLI有效；原ZIP／patch／EXE／MOX.SET／417檔、gofmt／git diff --check、新來源與收據1000:1000、工具root-owned／誤建.md目錄自檢空。正式六私有收據SHA與命令集中[331](docs/spec/331-moo2-button-branch-call-return.md)，原版素材不提交。沿授權推github隔離分支，未推本機origin；主庫玩法RE閘門與整款remake／中文化目標保持。

Docker ps -a以主庫與隔離工具鏈兩掛載路徑篩選皆空，本輪無遺留容器，其他專案未清理。

## 2026-10-03：完整範圍命中及後續正常CALL

起點a48f536a1132731c1b055e4419854642177b1c5e，332先DRAFT／READY，只續331第96步後的原caller。初次於完成IRQ7777→7778處誤停，回DRAFT修正成對完成中斷的觀察分類，再READY；初次收據留332-initial。同Docker命令乾淨重跑兩50M與獨立100M，原輸入、CPU／平台／CLI及正式流程cap未改。

固定Go1.24.13映像／600s／2GiB／2CPU／128pids／UID1000／network none，原ZIP／patch唯讀重建417檔。go build -p 2 -buildvcs=false -o /tmp/moo2-probe ./workplace/moo2-probe；python3 workplace/new-game-332-verify.py PASS。全部3847／4511／6451舊列除既定正規化保持，72PNG逐位元保持、舊331 terminal保持。新313步與341省略callee步兩預算一致；311完整來源、2 MOV僅低word來源已驗、1 IRQ堆疊寫回未重建，限制已明示。

index1..6右界拒絕、index7左界拒絕、index8全畫面0／0／639／479命中x500／y229，實際局部8及DS26C4A6 word8寫回。CALL208FD4自然返回EAX1；CALL209325新return20DDF7仍待返回，8192觀察上限停止，沒有新CPU拒絕。不能稱NEW GAME已觸發。下一步唯讀核對實際註冊表與可見主選單的關係及正常20DDF7返回，不深挖整個callee或猜新輸入。

69回填函式、既有32／49／25／27／27／34／31／36／31／37及新增34缺證據負例、兩CLI PASS；六收據SHA與命令集中[332](docs/spec/332-moo2-button-tail-return.md)。330的SS20h註記修正為實際SS188h並追加勘誤，保留原定位與收據。CPU／startup／provider／matcher逐位元保持、gofmt、原ZIP／patch／EXE／MOX.SET／417檔及新來源／收據擁有權核對通過。工具root-owned／誤建.md目錄自檢空，Docker兩掛載篩選空，其他專案未清理。沿授權推github隔離分支，主庫RE-first與整款remake／中文化目標保持。

## 2026-10-03：原表更換與正常20DDF7返回

起點3580b3e26181ff0978fc7ed0b2c45f8c685f76e3，333先DRAFT／READY，只新增最多16份直接peek快照與既有cap內的原EIP／SS／ESP返回觀察，舊332終態不改。固定Go1.24.13映像／600s／2GiB／2CPU／128pids／UID1000／network none，原ZIP／patch唯讀重建417檔，go build -p 2 -buildvcs=false -o /tmp/moo2-probe ./workplace/moo2-probe；沿332三正常流程換333輸出名各一次，python3 workplace/new-game-333-verify.py PASS。

全部3847／4825／6765舊列及72PNG保持，原CALL開始快照與332實際CALL逐項對接。47990733實際返回20DDF7／SS188／ESP2BDAD8，原回呼／IRQ非活動，開始與返回完整9筆表495bytes保持；50M／100M終態更換同一指標298848的7筆表385bytes且完全相同。兩側各3份新快照唯讀保持。正式終態index2範圍415／217／567／238幾何命中500／229；人工查看PNG強推論對應NEW GAME位置，尚未再次實際點擊，不宣稱指令已觸發。下一步保留舊基線，另立7筆表就緒後的正常press／release情境，不提高100M cap，不深挖callee。

70回填函式、既有負例與新增35缺證據負例、兩CLI PASS，六收據及來源SHA集中[333](docs/spec/333-moo2-menu-table-return.md)。CPU／startup／provider／matcher／CLI保持，325固定EXE全套及329 CLI有效；gofmt、原ZIP／patch／EXE／MOX.SET／417檔及新來源／收據1000:1000通過。工具root-owned／誤建.md目錄自檢空，Docker兩掛載篩選空，本輪沒有遺留容器，其他專案未清理。沿授權推github隔離分支，不推本機origin；主庫RE-first與整款remake／中文化目標保持。

## 2026-10-03：正式選單正常點擊與新字串指令拒絕

起點d6688b01f7a5306bc6d271e06c4a5eb48a1eb430，334先DRAFT／READY，新增明示MENU_READY_CLICK=1與固定50M的原7筆表／callback／IRQ閘門，只經正常InjectMouseEvent送一次press／release；無旗標舊三基線保持。初次腳本輪次替換誤改預期EXE雜湊，輸入檢查停止、未啟動原版；修正腳本後同隔離設定乾淨重跑，不當產品缺陷。

固定Go1.24.13映像／600s／2GiB／2CPU／128pids／UID1000／network none，原ZIP／patch唯讀重建417檔，go build -p 2 -buildvcs=false -o /tmp/moo2-probe ./workplace/moo2-probe；沿333三基線換334名各一次，加一條ready100M情境一次。python3 workplace/new-game-334-verify.py PASS：全部3847／4829／6769無旗標舊列及72PNG保持，ready額外輸入前4780原列保持。50000000 press／50011955 release，相隔31339微秒，callback4／4正常完成；61538983原20DDDB word store實際DS26C4A6 0000→0200，選中第2筆已證實。

後續76658331原1F3640的F2 66 AF被CPU拒絕，最終PNG全黑，未到100M、未進新遊戲設定頁。限定CONFORMED只含輸入／原選擇／舊基線保持，不稱正式開局完成；下一步依[099](docs/spec/099-cpu386-repne-scasb.md)／[292](docs/spec/292-cpu386-repe-scasd.md)與處理器規格補REPNE SCASW，再用同ready情境重生收據。不深挖原函式或提高cap，不代寫結果。

14新增CLI拒絕與有效正對照、71回填函式、既有負例及新增33缺證據負例、兩CLI通過。九私有收據／腳本與原黑屏雜湊集中[334](docs/spec/334-moo2-ready-menu-normal-click.md)。CPU／startup／provider／matcher未改，325固定EXE既有全套有效而未涵蓋新拒絕。原ZIP／patch／EXE／MOX.SET／417檔、gofmt、Git差異及新來源／收據1000:1000核對通過，工具root-owned／誤建.md目錄自檢空。Docker兩掛載篩選空，本輪沒有遺留容器，未清理其他專案。沿授權推github隔離分支，主庫RE-first及完整remake／中文化目標保持。

## 2026-10-03：REPNE SCASW與正常新遊戲設定頁

起點4501b831842f33ee5a0388b3018ae0c240949bc3。命中CPU公開契約／規格閘門／dosgolem路由；335先DRAFT唯讀取得未改CPU初態，證據審查READY後只加F2＋16位AF通用能力，沒有遊戲位址條件。公開Intel契約與獨立寬值oracle驗92416算術組合、多元素／方向／故障與前綴拒絕；CPU窄測試0.266s與固定EXE Go全套通過，cpu386 106.777s，不稱實機硬體逐週期驗證。

同334四原版各一次乾淨重生，不改輸入或100M cap；三舊基線3847／4829／6769列及72PNG保持，ready原入口前5833列保持。76658331原F266AF匹配第6個word，ECX9→3、EDI1F357B→1F3587、flags246h保持；下一MOV讀原SS stack令EAX2→64h。來源與核心獨立核對；100M無新CPU拒絕，原版NEW GAME設定頁PNG人工確認。正常原點擊、選擇與callback4／4保持；ACCEPT／完整開局／remake同狀態仍未知。

命令、來源、私有LOG／PNG與測試收據集中[335](docs/spec/335-cpu386-repne-scasw.md)。292／334與索引同次回填，72函式、335新增26缺證據負例及兩CLI通過。原輸入唯讀、產物1000:1000、工具root-owned／誤建.md目錄自檢空；Docker兩工作區掛載篩選空，沒有本輪遺留容器，未清理其他專案；原版素材不公開。沿既有授權推github隔離分支，主庫玩法RE閘門保持。下一步只保存原設定頁按鈕表與ACCEPT輸入前置，不重開已完成SCASW或renderer考古。

## 2026-10-03：正常ACCEPT與選族頁

起點74f574a78927f6bacdec95ea0519c83078bc71df。命中dosgolem／GUI輸入／規格閘門與文件職責路由；336先DRAFT保存原17筆表三時點，扣新列後8146舊ready列與全部圖保持，935bytes hash與ACCEPT候選範圍足夠後READY。只新增明示SETUP_ACCEPT_CLICK=1、80M原表／畫面／回呼／IRQ前置；全部經InjectMouseEvent，不代寫原選擇或增加cap。

原初態一次600s；正式五流程一次各情境、900s有界容器／固定Go1.24.13／2GiB／2CPU／128pids／UID1000／network none，原ZIP／patch唯讀重建417檔，原版各條100M cap保持。go build -p 2 -buildvcs=false -o /tmp/moo2-probe ./workplace/moo2-probe；python3 workplace/new-game-336-verify.py PASS，四舊基線3847／4829／6769／8146列及102PNG保持，accept額外80M輸入前6145原列保持。80000000 press／80011248 release、差42912微秒，callback6／6完成；80124668原20DDDB word0000→0F00，index15實際選擇已核對。

原版同100M到21595F，無新CPU拒絕，SELECT RACE選族頁PNG人工確認，正常ACCEPT已證實；種族選擇／完整開局與remake同狀態仍未知。已保存90M／100M原16筆表／880bytes供下一正常輸入；不深挖renderer／helper。

15新增CLI拒絕與有效正對照、同binary舊334 14拒絕與正對照、73回填函式、既有負例與新增28缺證據負例、兩CLI通過。CPU／startup／provider／matcher逐位元保持335，335固定EXE Go全套仍有效；只改probe與規格，gofmt及來源／新收據1000:1000通過，工具root-owned／誤建.md目錄自檢空；Docker兩工作區掛載篩選空，沒有本輪遺留容器，未清理其他專案。命令、來源與十三私有收據集中[336](docs/spec/336-moo2-setup-accept-normal-click.md)，原版素材不公開；沿授權推github隔離分支，主庫RE-first保持。

## 2026-10-03：正常第7筆選族與統治者名稱頁

起點7c84f3931953cf3c9ffcbdc0718852e9c700b3ba。命中dosgolem／GUI輸入／規格閘門與文件職責路由；337先DRAFT，直接獨立審查336已保存兩原16筆表／880bytes、90M完整初態與候選唯一index7，不重跑相同初態。證據足夠後READY，只加明示RACE_HUMANS_CLICK=1與固定90M原表／RGB／callback／IRQ前置。人類是測試情境，不改主庫正式預設或typed種族。

六原版由乾淨417根檔／官方EXE各一次重生，固定Go1.24.13 Docker900s／2GiB／2CPU／128pids／UID1000／network none，原ZIP／patch唯讀；go build -p 2 -buildvcs=false -o /tmp/moo2-probe ./workplace/moo2-probe。python3 workplace/new-game-337-verify.py PASS，五舊基線3847／4829／6769／8149／7752列及132PNG保持，humans額外90M輸入前6821原列保持獨立ACCEPT。

90000000 press／90010495 release，差42293微秒，只經InjectMouseEvent；90056672原20DDDB word0000→0700，callback8／8完成。原版同100M到215DEE、無新CPU拒絕，Enter Ruler Name與預設Strader／ACCEPT原PNG人工確認；typed種族、名稱原buffer／確認與完整開局未知。原100M已保存3筆名稱表／165bytes，供下一正常輸入前置；不深入renderer／helper。

16新增CLI拒絕與有效正對照、同binary舊336 15拒絕與正對照、74回填函式與新增27缺證據負例、兩CLI通過。CPU／startup／provider／matcher逐位元保持335，335固定EXE Go全套仍有效；來源／新收據1000:1000與gofmt通過；工具root-owned／誤建.md目錄自檢空、Docker兩工作區掛載篩選空，沒有本輪遺留容器，未清理其他專案。命令、來源與十三私有收據集中[337](docs/spec/337-moo2-race-humans-normal-click.md)。原版素材不公開，沿授權推github隔離分支；主庫RE-first保持，下一步只補較早名稱頁可接受輸入初態與原buffer，不重開已完成選族／ACCEPT／SCASW。

## 2026-10-03：正常統治者名稱確認與旗幟頁

- 起點工具0633c346ce2ca1156ab26c1dbc533ec0e43920e0／主庫5c1b6a4a42d3ff8fc69125d00587cd66277eef96。命中dosgolem、GUI還原、規格閘門與文件職責路由；338先DRAFT。私有唯讀探針首次因少帶IRQ輔助檔而建置失敗，尚未跑原版；完整依賴後100M蒐證一次。Strader加零為8bytes，獨立核算修正長度後直接讀既有收據。95M／97M／100M原165bytes表相同，全部9030原humans列／30PNG保持；95M初態及原候選bytes足夠後READY。
- 初版七情境由乾淨417根檔／官方EXE各一次重生，Docker900s／2GiB／2CPU／128pids／UID1000／network none、Go1.24.13，原ZIP／patch唯讀。go build -p 2 -buildvcs=false -o /tmp/moo2-probe ./workplace/moo2-probe。名稱按下後原mask1，過嚴的mask2B放開前置讓探針未放開；無新CPU拒絕。保存首次來源667e3dd2與未放開收據，不當成功。回到DRAFT核對既有InjectMouseEvent，原mask1允許固定移動兼放開，再重新READY。
- 修訂只改rulerAccept明示分支的一行放開條件，來源核對其餘逐位元相同，CPU／平台不變；六舊基準不重跑。新版只重生ruler與新舊CLI，Docker300s，原版仍100M cap。95000000按下／95015426放開，差47572微秒，callback10／10完成；原版同100M到228E0E，SELECT BANNER COLOR人工確認。共享20DDDB沒有命中，持久名稱writer仍未知。
- python3 workplace/new-game-338-verify.py PASS：六舊情境3847／4829／6769／8149／7752／9030原列與162PNG保持，新輸入前7523原列保持獨立humans，原165bytes表／候選32bytes／核心與RGB吻合。17新CLI拒絕及正對照、同新版binary舊337 16拒絕及正對照、75項規格回填與新338 26負例、337 27負例、兩CLI通過。本輪未重跑Go全套，不外推完整玩法。
- 338限定CONFORMED，337及索引回填；18份私有腳本／收據雜湊見[338-moo2-ruler-name-normal-confirmation](docs/spec/338-moo2-ruler-name-normal-confirmation.md)。CPU／startup／provider／matcher／mouse callback保持335；來源與新收據1000:1000，gofmt／Git差異通過。工具root-owned／誤建.md目錄自檢空；Docker兩工作區掛載篩選空，本輪容器已清理，未動其他專案。
- 下一步唯讀取得較早旗幟頁、原10筆表與正常選色前置；持久名稱writer、旗幟選擇、完整開局、正式RNG與remake同狀態未知，主庫RE-first保持。

## 2026-10-03：旗幟頁正常輸入與原按鍵查詢，選色未推進

- 起點工具0c88d04cb59d2b7bfc716e953cc529c54b5fb6e2／主庫f079ef801ab20d3ab1949ad9ee8d5e9693ee9eea。命中dosgolem、GUI、規格閘門、文件職責與回填路由。339先DRAFT，98M／99M唯讀蒐證保持全部7874原ruler列與30PNG。98M只有一筆初始化表，99M才有完整十筆；READY後固定99M紅色fixture，不改正式預設。
- 首次100M正常press／release與callback12／12完成，仍旗幟頁；保存early來源及負收據。回到DRAFT，僅明示旗標固定120M觀察，同99M輸入與7837原100M前列保持，但仍旗幟頁，沒有新CPU拒絕。不是選色成功。
- 直接解析原收據：原AX3在press到release之間0次輪詢，之後492次只讀到buttons0。重查GUI路由與既有INT33／InjectMouseEvent，重新READY；release新增等待正常callsite24C31B首次確實讀到pressed。不移動按下時點、不重送、沒有代寫原選擇或CPU／平台。
- 最新原版99000000按下、99083819正常查詢讀到BX1／CX276／DX190、99083854首次合法mask1放開，相差245291微秒。120M到228DDC，callback12／12及IRQ28866／28866完成，仍SELECT BANNER COLOR；共享20DDDB未命中。339限定CONFORMED只涵蓋原表、正常查詢與放開；選色消費、下一頁及持久旗色writer未完成，不算主庫玩法完成。
- Docker外層300s／2GiB／2CPU／128pids／UID1000／network none，Go1.24.13，原ZIP／patch唯讀，新鮮417根檔及固定EXE／MOX.SET。新版只重生red一次與新舊CLI，不重跑未受影響六條基準／Go全套。go build -p 2 -buildvcs=false -o /tmp/moo2-probe ./workplace/moo2-probe；日期不是seed。
- python3 workplace/new-game-339-verify.py PASS：7708原列與28既有PNG保持獨立99M輸入前置、完整原表與核心／RGB／pressed輪詢／正常順序吻合。22新CLI拒絕、120M與100M正對照、同binary舊338 17拒絕與正對照；76項規格回填、339 26缺證據負例、338 26負例及兩CLI通過。來源六個有界區塊／五guard逆轉後逐位元保持338；CPU／平台仍335。
- 22份私有收據、來源與核算雜湊見[339-moo2-banner-red-normal-click](docs/spec/339-moo2-banner-red-normal-click.md)。338與索引回填，原素材／PNG／LOG／RAM留忽略workplace。下一步只追首個原pressed查詢後的有界正常GUI消費與返回框架；不再提高cap、移動輸入時點或盲重送，主庫RE-first保持。

來源與22份新收據1000:1000，gofmt／Git差異通過；工具root-owned／誤建.md目錄空，兩工作區掛載篩選沒有執行中或停止容器，本輪已清理。
