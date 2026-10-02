# 325：32 位記憶體 NEG

狀態：**CONFORMED**（僅無前綴F7 /3記憶體目的與下列正常消費）
日期：2026-10-03
範圍：通用 CPU386 無前綴 F7 /3 記憶體目的；不改主庫玩法。

## 原始定位與平台契約

固定官方 1.31 ORION2.EXE SHA-256 4e11be14217b4aafa1839f333bf5eba037f98b0c44e9e4752c96c464c260419f，工具 f31793166415705c75a81339808372da24cded44。原 ZIP 417 根檔、MOX.SET、44M Esc／1996-01-01／50M cap、separate DOS 與單次正常 NEW GAME 輸入沿[324](324-cpu386-add-byte-register-memory.md)。私有 workplace/moo2-probe-324-click.txt.gz SHA-256 c9e8b2d3d15c4b8a64fb9ff497513b2a294b67b9a682b5584dc9156d873bf1e3。

49501135 步在 dosgolem 高位 LE 2130F3 拒絕 F7 5D D8 8B 45 D8 66 3B 45 E0 0F 8D A8 00 00 00。完整 R FFFFFFFF／498AC0／8／47／2BD9B8／2BD9F4／495230／2BDACC，六段 8／188／188／0／20／188，flags286h。F7 /3、mod1、EBP 與 disp8 D8 解碼為 SS:[EBP-28h]，此時 SS188:2BD9CC。324 未取樣來源 RAM，不由 EAX 或靜態後續 bytes 猜值。拒絕後 EIP2130F5 是部分 fetch，非成功。

[Intel 80386 原始手冊 NEG](https://www.ardent-tool.com/CPU/docs/Intel/386/manuals/prref386/NEG.htm)定義 F7 /3 的記憶體或暫存器目的作 0-value，CF 只在來源零時清除；不可寫或段越界有例外。[附錄 C](https://www.ardent-tool.com/CPU/docs/Intel/386/manuals/prref386/appc.htm)明列 NEG 修改全部六算術旗標，包含 NEG 條目旗標段未另列的 AF。採平台契約，不追原版 helper 控制流。

## 型別與 READY 候選

無前綴 F7 /3 mod0..2 使用既有 decodeAddress32、readSegment32、writeSegment32，涵蓋一般 ModR/M、SIB、無基址、正負 disp8／disp32、DS／SS 與既有 32 位位址繞回。來源／目的為同一 uint32 四 byte 小端序；結果為 0-value mod2^32。保留暫存器 F7 /3 與既有 ESP 形狀，不放寬其他群組或前綴。

CF=來源非零；AF=低 nibble 非零；OF=來源80000000h；SF／ZF／PF 依結果，PF 只看低 byte 的偶數個1。不使用輸入 CF。八 R、六段、FPU、非算術 flags 與鄰接 RAM 保持，flags 只在全部目的寫回成功後發布。

段界限／未知 selector／唯讀／位址溢位／來源任一 byte 讀錯時不寫 RAM 或 flags。沿既有 sequential Bus 寫入近似：目的第 n byte Bus 寫失敗可保留前 n 個已寫 byte，但 flags／R 不發布；不稱完整 CPU 例外或原子 rollback。fetch 錯誤已部分改 EIP。66／67／段覆寫／REP／REPNZ／LOCK 的未審查 NEG 形狀繼續拒絕。

## 審查與驗收

新增獨立預期測試：零、1、全部低 word、符號極值、每一 bit 與補數、64 初始算術 flags；全部 ModR/M／SIB／DS／SS、地址繞回、不對齊與段末 dword；任一來源讀失敗、任一目的寫失敗及部分寫前綴、唯讀／段界限／未知段／溢位、截短／前綴／未審查群組拒絕，以及暫存器與 ESP 回歸。既有「wrong SIB」F7 5C 25 disp8 是合法 EBP 形狀，325 明示納入並以真正未審查 LOCK 負例替換，180 同次勘誤。固定 EXE 全部 go test -p 2 -buildvcs=false ./... -count=1 須 PASS。

原版兩流程沿同 44M 輸入乾淨重生，不加 cap 或重點。無點擊全部基線保持，點擊到舊首個失敗後快照前全部324列保持，只正規化mtime／DTA日期時間四byte／PNG路徑；第一 NEG 完整 R／段／flags 另與舊拒絕核對。

probe 只在真正外層 CPU.Step 記 2130F3 最多8筆 NEG 與各3步最小消費，續行總數最多32。保存完整 R／段／flags／EIP／16指令byte與SS:offset；直接唯讀 RAM 的16byte窗口由 EBP-2Ch 起，包含 NEG 來源四byte及 EBP-20h 比較 word。須先核對描述子、limit、RAM界限，不呼叫 CPU讀取hook；窗口前後與後續消費核對，不代寫原版狀態。原始 MOV／CMP／JGE 只依實際收據判斷，未到的分支保持未知。來源／窗口／所有 flags／最小caller有正式收據才限定 CONFORMED。

新規格掛入 docs/spec/000-index.md。成功後回填084／180／324與守衛；原始未取樣的歷史事實保留，現行後續連結325。所有原版 LOG／PNG／RAM 與素材留本機忽略 workplace；主庫 RE-first 閘門、日曆非 RNG seed、AH2Ch 時間近似與正常開局／remake 同狀態未知保持。

READY 審查：F7 5D D8 的 reg=3、mod=1、rm=5，D8符號延伸為-40；SS188、EBP2BD9F4得到偏移2BD9CC。公開 NEG 與附錄C足以界定結果及六旗標；既有通用地址、描述子、逐byte Bus讀寫與sub32可沿用。完整寫回前不發布flags，晚期Bus部分寫已明示為工具近似。擴充所有無前綴32位記憶體形狀在相同平台範圍內，不需新玩法、架構或資料格式決定。原版未知RAM與未執行分支仍待真正正常收據，READY不稱對拍通過。

## 正式收據與限定結論

固定 Go1.24.13 映像 sha256:1a6d4452c65dea36aac2e2d606b01b4a029ec90cc1ae53890540ce6173ea77ac，600s／2GiB／2CPU／128pids／UID1000／network none，原 ZIP／patch唯讀，417根檔乾淨重建。定向 go test -p 2 -buildvcs=false ./internal/cpu386 -run 'TestNegDword|TestNegStackDisp8Dword|TestNegRegister32|TestNegByte' -count=1 -v PASS，CPU3860.491s。六新增主測試涵蓋上述值域、六旗標、全部地址、每byte失敗、前綴／截短／未審查群組及暫存器／ESP回歸，完整R／六段／FPU／非算術flags和鄰接RAM保持。

DOSGOLEM_MOO2_EXE=/tmp/game/ORION2.EXE go test -p 2 -buildvcs=false ./... -count=1 全套 PASS，CPU386143.926s／machine3.148s。probe先 go build -p 2 -buildvcs=false -o /tmp/moo2-probe ./workplace/moo2-probe，再 /tmp/moo2-probe /tmp/game/ORION2.EXE --game-dir /tmp/game。兩流程環境為 DOSGOLEM_MOO2_CALENDAR_EPOCH=1996-01-01、DOSGOLEM_MOO2_HARDWARE_ESCAPE_STEP=44000000、DOSGOLEM_MOO2_MAX_STEPS=50000000、DOSGOLEM_MOO2_SEPARATE_DOS=1；輸出DOSGOLEM_MOO2_VBE_FRAME_PREFIX=/src/workplace/moo2-325-baseline-frame／click-frame、DOSGOLEM_MOO2_VBE_PNG=/src/workplace/moo2-vbe-325-baseline.png／click.png；點擊流程另加 DOSGOLEM_MOO2_NEW_GAME_CLICK_AFTER_DISPLAY40=1。cap、Esc、epoch、檔案與按下／放開輸入不改。

無點擊全部324原始列與終圖保持；點擊舊失敗後首個快照前4123列保持，僅正規化mtime／DTA日期時間四byte／PNG路徑。第一NEG完整R、段、flags與舊拒絕另核對。新probe只直接讀RAM，不新增guest服務、CPU讀hook或改輸入。

**已證實，dosgolem高位LE正常原版執行**：

- 三次2130F3／F7 5D D8：outer49501135／49520331／49576598，皆到2130F6。真正SS188:2BD9CC來源FFFFFFFFh，結果1h，四byte由FF FF FF FF變01 00 00 00；flags286h→213h，CF1／PF0／AF1／ZF0／SF0／OF0。八R與六段保持。
- 窗口在SS188:2BD9C8，16byte由08000000FFFFFFFF0D00000008000000到08000000010000000D00000008000000；只有NEG目的四byte改變。比較來源SS188:2BD9D4真正word為0008h。
- 九續行全部error=nil。每次真正MOV EAX,SS:[EBP-28h]在2130F6／8B 45 D8把EAX由FFFFFFFFh變1，flags213h保持；CMP AX,SS:[EBP-20h]在2130F9／66 3B 45 E0比較0001h與0008h，flags213h→297h，R／段／窗口保持。2130FD／0F 8D A8 00 00 00的JGE三次不跳到213103，flags297h保持。跳轉方向原版未發生，仍未知，不推另一個遊戲初態。
- 兩mouse CB仍191步、started2／completed2、pending0／activefalse；問號搜尋、byte ADD與全部舊停點前列保持。點擊流程從舊停點續行498865個外層步到50M cap，未出現guest_cpu_stop或step_error。
- 點擊50M時高位LE21334F／88 45 F8 EB 8C FF 45 E8 0F BF 45 D0 01 45 CC E9，完整R3DD302／0／3DA3D4／42／2BD948／2BD998／4953AA／2BDACC，六段8／188／188／0／20／188，flags216h；26937個unique_sites。這是有界流程的終點，沒有新CPU拒絕。未點擊仍238573／24693 sites。

| 本機忽略收據 | SHA-256 |
| --- | --- |
| workplace/neg-dword-325-tests.txt | a48cd9c18945224c5610961b38058ade0f272b3016471b7d57e6cec6ceab8520 |
| workplace/full-test-325.txt | f98aa39690cc448441483c927be79f48d73dc943970819136b4d7e8ace851d03 |
| workplace/moo2-probe-325-baseline.txt.gz | 42adfe85b088d25bb3a4e3856d7f8c2187f001253759b9f194c4e123572c552f |
| workplace/moo2-probe-325-click.txt.gz | 5c269c7fbd6360ad5248da763e2f57f302ea3b837e48a41d1896cb73a16a1d7b |

CPU SHA-256 1448f29f24dad35e83575189055dc15dd7ac9db2b3529c6d450f03baf91c97b1；probe2f58792f30d6fea061861baacef317f55a0eb503b5a0d1acbec0fca131eb8844；新增測試ab4330e73511963f050c82d086384cb0095658e9ed9fe200070d314bb42aa436。startup、read-only provider、問號matcher與324逐位元保持。

兩PNG與已實際檢視的323／324逐位元保持：無點擊11ec0ed15a4c874d36c94a824af73eb71dc6937dfe3bd568450943861db89927，點擊59f76749db5232f97a6b6f969f56f848c2e89e731d7e3fb30b8786982bca4a81。仍主選單，設定畫面仍未知。正常開局、remake同狀態、AH2Ch／RNG、人耳與完整玩家路徑未完成；固定epoch不等於固定亂數seed。

限定CONFORMED只含無前綴32位記憶體NEG及以上真正寫回／最小消費；晚期Bus部分寫與fetch EIP近似保持，未放寬prefix或主庫RE-first閘門。下一步在相同50M上限與單次正常輸入下，增加唯讀、有界的後段進度／畫面觀測，判定選單之後的可見轉移與最小阻塞；保留原始地址與未知，不追helper內部、不加cap、重點、代寫狀態或提前調整輸入。

62回填函式、原有32／49／25與325新增27缺證據負例、三CLI全部PASS。workplace/neg-dword-325-backlink-tests.txt SHA-256 02e589623a7b22418ead00bb74525b1622d6d8c1ab3c26cd5497f3559ffe8628。獨立正常來源／寫回／六旗標／9續行、舊4123列與兩終圖驗證可在容器內重跑python3 workplace/neg-dword-325-verify.py；本機忽略腳本SHA-256 f13094e9dfc2c75bd089a60b6cbb4f35f459eaf1840a2979a78be53695ea0e83，收據workplace/neg-dword-325-parity-tests.txt SHA-256 b7bd5bca62c91c69c0d784d57e7e1b7177f6acb0bb429c1a287ef28a4e645a84。新檔與收據UID:GID1000:1000，root-owned／誤建.md目錄自檢空；一次性工作容器已退出。

## 2026-10-03 後段唯讀進度

後段唯讀進度觀測已由規格326接通：[326-moo2-post-click-progress-observation.md](326-moo2-post-click-progress-observation.md)。六時點VBE換頁／寫入與可見像素保持，R／堆疊／RAM持續改變；原始325全部列與終圖仍保持。現行下一步為末尾真正DS:29BE74→DS:[EAX]→SS:[EBP-8]與CMP／JE／JLE的最小來源／分支核對。ESI窗口不當作實際來源，不改輸入或cap，設定畫面仍未知。
