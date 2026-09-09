# 186 — FD2 自然執行驅動的平台缺口補齊

日期：2026-09-08。各列獨立依 READY → CONFORMED 驗收。
平台前提：[Intel 指令參考](https://www.intel.com/content/www/us/en/developer/articles/technical/intel-sdm.html)；
運算依公開 CPU 契約，不把標準指令再包裝為遊戲語意 RE。
FD2 輸入固定 SHA-256 222b7d067ad4450eb9c5f6e6bce1797d54bb050417ba39ced6067f8039f28c4f。
所有位置為 dosgolem 重定位 LE 線性位址；沿用既有 runtime，不改原版 bytes。

## 目前狀態（2026-09-08，批次146；使用者要求暫停）

| 層次 | 已驗證／尚缺 |
|---|---|
| 平台已驗收子集 | 127–146已補件；r12普通重播越過首攻，完整暫存檔與人物恢復 |
| 最新回歸 | dosfile／machine／cpu386 720項通過、0失敗、0略過，已提供原版測試環境變數 |
| 正常玩家路徑 | 修正後r12：START→117頁→首攻→第2回合；依使用者要求暫停。r11曾到城鎮但模式表受污染，不作最終收據 |
| 畫面勘誤 | 大型FDSHAP讀檔截斷已修，r4自行重生完整草地／道路／HUD；舊黑底不是remake差異 |
| 輸入／檔案界限 | 僅普通BIOS鍵，無PC／RAM／RNG注入；32MiB堆積為明示配置，未驗收完整存讀檔 |
| 時序界限 | 每指令1微秒等既有硬體近似，靜音；不宣稱實機時序或音訊一致 |

本輪收據在 fd2/work/ch01-town-parity-20260908/，為本機原版資料；不得混入公開素材。
本表取代舊批次124的目前狀態；各批次下方歷史停點保留。

選單收據與重跑入口：[標題輸入驗證](../findings/2026-09-08-fd2-title-input-parity.md)。

CONFORMED 僅指本規格明列的平台子集，不代表整套硬體或玩家路徑完成。
唯一機械收據：[平台缺口證據](../evidence/fd2-platform-gaps-20260908.json)。
以下批次按歷史順序保存，舊停點不代表目前仍受阻。


| 切片 | 自然執行證據 | 契約 | 狀態 |
|---|---|---|---|
| OR r32,r32 | after-ss-store-probe.json：37381 / 09 C6 | 09 /r mod3，目的 OR 來源；CF/OF 清除，SF/ZF/PF依結果；AF沿用現有未定義旗標策略清除，不聲稱與硬體AF一致。其他暫存器不變。 | CONFORMED |

測試必須涵蓋非零／零／符號／parity、來源保存，以及真實 FD2 自然越過原阻塞；
每批保存新 stop 與來源雜湊。未到標題／戰場不得宣稱畫面對拍完成。

## 批次2：CONFORMED
gap-01.json 自然抵達 3D331 / A8 01；相鄰原始 code 視窗同時包含
24 FE、03 F8、F7 07 01 00 00 00。只解碼標準指令，不附加遊戲語意：
- A8 ib：TEST AL,imm8，僅更新邏輯旗標。
- 24 ib：AND AL,imm8，保留EAX高24位與其他暫存器。
- 03 /r mod3：ADD目的暫存器與來源，依既有add32更新算術旗標。
- F7 /0 mod00 非SIB／非絕對位址：TEST DS:[base] dword,imm32；
  完整descriptor讀取，僅更新旗標。其他F7既有契約不變。
未列prefix不接受；每項合成邊界值測試與自然原版重跑。

## 批次3：CONFORMED
gap-02：37441 / C6 05 9C 41 05 00 00。支援 C6 /0 mod00 rm5 的 DS 絕對 byte immediate store，先取32位元位址再取imm8，旗標與暫存器不變；descriptor拒絕時不寫入。

## 批次4：CONFORMED
gap-03：3D8A9 / 3B 58 04。把現有兩種特例的 CMP r32,[base+disp8] 擴為無SIB的mod1通用暫存器來源，EBP取SS、其他取DS，有號disp8，僅sub32旗標變化。24 ib 原本已實作，批次2改為沿用既有case，新增測試保留。

## 批次5：CONFORMED
gap-04：3F5D6 / F3 A5，ECX=70。支援 A5 與 F3 A5 的32位元 MOVSD。
依序讀DS:ESI、寫ES:EDI，再依DF調整兩指標±4；REP每次成功遞減ECX，
零count不讀寫；非REP只做一次不改ECX。旗標保持。重疊區依CPU逐次順序，
不能用memmove。每次檢查完整dword，失敗只保留此前完成的迭代；不假稱例外重啟。
16位元／其他prefix不擴充。

## 批次6：CONFORMED
gap-05：3F5DD / 81 C4 6C 01 00 00。81 /0 mod3 的ADD r32,imm32，沿用add32算術旗標，不改其他暫存器；其他group沿用原契約。

## 批次7：CONFORMED
gap-06：3698B / FF 15 58 27 05 00。FF /2 mod00 rm5：讀DS:absolute的32位目標，將下一EIP壓入SS:ESP-4，發布ESP與目標EIP；旗標和其他暫存器不變。來源失敗／stack拒絕不發布跳躍或ESP。

## 批次8：CONFORMED
gap-07：3CC96 / 66 8B 5D 14。16位元 MOV r16,[base+disp8]，無SIB；EBP取SS，其餘DS，disp8有號；目的高16位與旗標不變，缺段／越界不發布。

## 批次9：CONFORMED
gap-08：3CCAE / 0F 8D 68 00 00 00。JGE rel32：SF==OF時將有號位移加到指令尾EIP；否則只前進6 bytes。暫存器與旗標不改，operand16不開放。

## 批次10：CONFORMED — heap 物件與實體頁容量分離
lock-probe.json 與 lockprobe.go 唯讀追蹤：step20111配置40 bytes，回傳68028，
配置器尾端68050；step20157原版0600要求68028+29(hex)=68051。
這不是未知heap破壞：要求尾端與已配置物件在同一4KiB頁。
[DPMI 1.0 §0600，頁118](https://docs.pcjs.org/specs/dpmi/1991_03_12-DPMI_Spec_v10.pdf)
規定跨到部分頁仍鎖整頁；無虛擬記憶體實作可無操作成功。
本工具採更嚴格的實際背書頁驗證：
heap 的next／limit仍依4-byte物件配置、不放寬capacity；只有成功配置時把Mem零填補至4KiB頁尾。
0600原有範圍檢查保留，未背書頁仍拒絕，無FD2特定地址例外。
這是平台規格實作，非原版heap演算法逐位址一致。
驗收：配置地址／容量不變、頁尾可讀寫為零、超容量不增頁、頁內末端加一鎖定通過，
跨入未配置頁拒絕；原版自然重跑。

## 批次11：CONFORMED — Watcom 0100h 轉接
gap-10 自然執行20197步抵達36D98，REGS.AX=0100。
沿用RE060已證實的cdecl／REGS佈局，將0100轉接既有DPMIHost，輸出六個暫存器與CFLAG。
真CPU僅依原ABI發布EAX、ESP、EIP，其他暫存器／旗標保持；不可把REGS順序當成CPU.R順序。
Install可接收啟動器既有host，避免建立兩份配置帳本。
0100之前把DOS游標移到heap背書頁之後；heap偵測外部新增容量後跳過它，
物件容量上限不變，不回收位址。描述子配置必須避開已存在selector。
驗收：0100段號／selector對應實際可寫非重疊memory；交錯heap／DOS配置不破壞舊內容；
原CPU旗標和非回傳暫存器保持；未知功能仍拒絕。未擴充0300等服務。

## 批次12：CONFORMED — LEA 32位元有效位址
gap-11停373D8 / 8D 3C 0E，來源與目的區域已分離，ECX=15118。
LEA依Intel規格計算base+index*scale+disp，不讀memory／descriptor、不改flags；
補齊32位元ModRM/SIB、無base、無index及有號disp8／disp32，32位溢位自然截斷。
發現既有mod3錯誤呼叫sub8，依非法LEA编码改為拒絕，保留此勘誤。
不開放16位元／segment prefix。合成無descriptor、wrap、SIB特例及非法mod3測試。

## 批次13：CONFORMED — REP MOVSB
gap-12停3740F / F3 A4，ECX=2，是15118-byte搬移最後2 bytes。
延伸既有MOVSB以REP逐byte讀DS:ESI寫ES:EDI，DF決定±1，每次成功ECX--。
零count不碰記憶體，失敗保留已成功迭代；其他MOVS契約同批次5。

## 批次14：CONFORMED — word store base+disp8
gap-13停3F71F / 66 89 48 32。MOV r/m16,r16 無前綴段覆寫，mod1非SIB，
EBP使用SS，其餘DS；有號disp8、小端低16位元、不改flags／暫存器。
同185的descriptor與越界檢查，但不擴大185的SS override允許形狀。

## 批次15：CONFORMED — word immediate store
gap-14停3F755 / 66 C7 40 30 00 00。66 C7 /0 mod1非SIB，
依base選DS／SS，有號disp8後讀imm16，寫word、其他狀態保持，其他16位元形狀仍拒絕。

## 批次16：CONFORMED — ADD base+disp8
gap-15停3F785 / 03 45 00。03 /r mod1非SIB讀DS／EBP取SS的dword，
有號disp8，沿用add32 flags，來源memory不改；讀取失敗不改目的。

## 批次17：CONFORMED — PUSHF word
gap-16停3EDFB / 66 9C，後接66 58。PUSHF將低16位flags壓SS:ESP-2，
flags不變；檢查完整word及下溢。原有PUSHFD行為不擴充。
整體回歸發現兩項舊FD2測試把Mem長度等同物件尾端；依批次10改驗4KiB背書頁，
配置回傳地址與正常路徑步數斷言完整保留。

## 批次18：CONFORMED — MOVSX word
gap-17停3EE38 / 0F BF 5B 32。MOVSX r32,word[base+disp8]，mod1非SIB，
DS／EBP取SS，有號disp8；word按int16擴展為32位元，flags不改，讀取失敗不發布。

## 批次18歷史驗收（目前狀態見檔首）
批次1–18各有CPU／機器層測試及原版自然越過對應停點；
[逐批收據](../evidence/fd2-platform-gaps-20260908.json)保存差異與限制。
目前停3EE52／INT31 AX0300，INT66向量由原版自行註冊為6900:0132，
真實DOS配置69000..6CB10。尚未產生遊戲畫面，不是PLAYER-E2。

## 批次19：CONFORMED — DPMI0300實際執行的最小子集
dpmi-real-mode-probe.json：3EE52，AX0300，BL66，CX0，ES:EDI指向50-byte封包；
原版0201自行註冊向量6900:0132。封包EAX0300，其餘通用暫存器0，SS:SP=0。
依[DPMI0300規格](https://www.delorie.com/djgpp/doc/dpmi/api/310300.html)
串接既有cpu.Model80386實模式核心，直接共用已配置DOS區域，從向量實際執行至IRET返回。
支援CX0、BH0、FS/GS0；其他形狀明確拒絕。SS:SP0時提供4KiB專用stack，
非零則驗證既有範圍；flags與返回框架依INT語意，最多200000指令。
資料封包輸入／輸出50 bytes，未完成不得寫回成功封包；保留已執行的memory副作用
並回報失敗位置，不能宣稱transaction rollback。16位元暫存器結果更新低字，EAX高字沿用核心；
核心未支援指令、IO埠、未註冊nested interrupt、HLT或超限一律停止並保存診斷。
本批不宣稱完整386實模式／音效硬體時序；不得默認IO成功。
合成IRET handler驗證真實memory副作用與封包、段位址、FLAGS、PM暫存器保持；
無向量／IO／非零CX失敗，原版自然重跑驗收。

批次19補充審查：既有實模式核心SetFlags一律採8086高四位全1，
不能用於80386；依Intel FLAGS圖，Model80386改保留IOPL／NT（12–14），bit15/5/3清零、bit1固定1；
8086／80186舊行為不改。返回哨兵必須由IRET抵達，不接受RETF偽裝。

## 批次20：CONFORMED — CMP word imm8
gap-19顯示INT66處理程式真正執行110條並IRET返回，接著停3F7B6 / 66 83 78 2E 00。
66 83 /7 mod1非SIB，DS／EBP取SS，word memory與符號擴展imm8相減，
只更新sub16 flags、不改memory；其他16-bit group拒絕。

## 批次21：CONFORMED — word store base
gap-20已實際完成第二次INT66（AX0301，48條實模式指令），返回後停3EE70／66 89 03。
補MOV r/m16,r16 mod00非SIB非absolute的DS:[base]，沿用批次14word store，
不讀額外disp、不改暫存器與flags。實模式診斷保留最多32筆獨立封包以供交叉驗證。

## 批次22：CONFORMED — word stack read
gap-21停43809 / 66 8B 44 24 2A。MOV r16,[ESP+disp8]，SIB24，
讀SS word，保留目的高16位與flags，signed disp8，完整descriptor範圍。
第一次INT66結果已與DOSBox-X交叉比對：EAX0300→1、其餘資料不變；
兩邊輸入FLAGS差AF位（0207／0217）並各自保留，故非完整同狀態／逐byte一致。

## 批次23：CONFORMED — near JBE
gap-22停43672／0F86 rel32。CF或ZF成立則跳到指令尾加signed rel32，
否則前進6 bytes；其他flags與暫存器不變。

## 批次24：CONFORMED — stack MOVSX與base word read
gap-23停4368A／0F BF 04 24，相鄰視窗含66 8B 03。
MOVSX word[ESP] SIB24使用SS；MOV r16,DS:[base]支援mod00非SIB非absolute。
來源字寬、符號擴展／高字保存依各指令既有契約，flags保持。

## 批次25：CONFORMED — 統一word memory有效位址
gap-24停4369C／0F BF 44 24 02，再次落在word來源的SIB／位移形狀。
把無段覆寫的16位元MOV讀／寫與MOVSX word，統一使用Intel 32-bit ModRM/SIB有效位址契約，
涵蓋mod00/01/10、scale、無index／無base、disp8/disp32與32-bit wrap。
base ESP/EBP取SS，其餘（包含無base但index為EBP）取DS；位址計算不讀memory。
各指令自行依word寬度檢查descriptor，MOV保留高字／MOVSX符號擴展、flags不變。
不擴充mod3 MOVSX、address16、segment override；185的SS override窄契約保留。
目的：消除已反覆遇到的同族word位址缺口，不改遊戲語意。
驗收：SIB scale／no-base／no-index、DS與SS分離、disp正負、word邊界、位址wrap。

批次25測試勘誤：舊wrong SIB=25其實是合法EBP+disp32，移除舊拒絕斷言，改由統一SIB測試驗證；absolute word原先未檢查DS，測試補合法descriptor以符合CPU契約。

## 批次26：CONFORMED — dword MOV／CMP共用有效位址
gap-25停436D7／83 7C 04 08 01；相鄰原始視窗同時含8B 54 03 08與89 54 04 08。
將無段覆寫的MOV r32,m32／m32,r32、83 /7 memory（word/dword）也接同一EA32，
CMP立即值仍按int8擴展到運算寬度。完整descriptor讀寫、不改非目的狀態。
這同時修正舊83 EBP+disp8錯用DS：架構規定base EBP應使用SS。
其他算術group及prefix不擴充。驗證SIB base ESP／EBX分離與讀寫往返、word比較。

## 批次27：CONFORMED — LE入口描述子
批次26原版回歸於entry第4步3C9E4停止；此前LoadLE只設EIP／ESP，
DS/SS皆0且沒有descriptor，舊absolute store直接寫Bus掩蓋了此不完整載入狀態。
依規格008既定「extender已建立base-0 code/data」前提，LoadLE應實際建立
非零入口code selector08、data selector10，base0、limitFFFFFFFF；CS指08，DS/ES/SS指10。
08／10只是工具的確定性selector標籤，不冒稱DOS/4GW原版selector編號；
FD2後續握手既有160等映射不變。code不可寫、data可寫，CPU仍嚴格拒絕未知段。
這是補足既定載入器契約，不是遊戲狀態注入。驗收原版入口兩次absolute store
與既有首次INT21完整回歸、未知段單元測試仍拒絕。

## 批次28：CONFORMED — 實模式OPL埠接線
gap-27第三次INT66（AX0304）停6900:28B7，OUT0228=04。
[DOSBox-X OPL映射](https://dosbox-x.com/doxygen/html/adlib_8cpp_source.html)
列明Sound Blaster base+8／+9對應OPL索引／資料（base220），388..38B與220..223為別名。
不再反組譯硬體driver。DPMI實模式bus增加明確可拒絕的IO介面，
接線沿用Machine既有OPL暫存器／計時器狀態；未知埠仍停止。
本次啟用AdLib存在，保留未改MDI.INI及原版driver偵測路徑。
**近似等級：hardware-spec approximation／既有啟動後立即可觀測逾時模型**，
不是實際取樣時間、DAC波形、逐週期或與DOSBox wall-clock一致；
用於有延遲後的偵測狀態契約，不作完整音效／操作時序驗收。
記錄原始port寫入及狀態變化（最多4096筆），以便定位下一個缺口。
驗收標準偵測序列（初0、timer1→C0、reset→0）、alias共用index/state、
未知DSP／PIT埠仍拒絕，原版實際偵測重跑。

## 批次29：CONFORMED — 實模式DOS檔案服務轉接
nested-dos-probe：INT21 AX3D00，DS6900:DX02F0。這是原版驅動開啟檔案，
不是未知中斷向量或硬體timer。DPMI新增可拒絕的實模式DOS callback；
FD2StartupDOS將3D/3E/3F/42/44接同一dosfile.Table與唯讀provider。
實模式DS:DX依段*16+offset轉換，資料上限FFFF；禁止將實模式段值當selector。
保留真CPU非回傳暫存器與CF以外狀態，EAX高字依既有寄存器模型保存。
其他DOS功能仍拒絕。驗收由real-mode開啟／讀取、protected-mode關閉同一handle；
missing file回DOS錯誤、未知功能拒絕，原版driver自然重跑。

## 批次30：CONFORMED — IMUL r32,r32,imm32
gap-29停43B1D／69 C0 D4 06 00 00，EAX8。69 /r mod3取signed32來源與imm32，
計算signed64乘積、截低32至目的；結果不能以signed32表示時CF/OF置1，否則清零。
其餘未定義flags沿用舊值（不宣稱硬體一致），非目的暫存器保存。

## 批次31：CONFORMED — C7立即值寫入共用EA
gap-30停43B87／C7 44 07 04 01 00 00 00。C7 /0 memory支援EA32全部形狀，
imm16／imm32依operand-size取得，再按相同寬度寫入descriptor。
不接受其他group或mod3（既有範圍未要求），flags保持；替換重複C7特殊形狀。

## 批次32：CONFORMED — SAR與IDIV
gap-31停43BAF／C1 FA 1F F7 FE。C1 /7 mod3以5-bit count算術右移，
count0不改flags；非零更新SF/ZF/PF/CF，count1 OF=0，其餘OF/AF未定義沿用清除策略。
F7 /7 mod3：signed EDX:EAX除signed來源，向零截斷，EAX商EDX餘數；
除零／商超signed32範圍則失敗、不發布暫存器，未定義flags保持。

## 批次33：CONFORMED — XCHG memory共用EA
gap-32停3EF43／87 83 94 2B 05 00。87 memory與r16/r32依operand-size交換，
EA32選段與完整寬度；先讀／寫成功後才發布暫存器，flags保持。
只承諾單CPU步進不交錯，不聲稱多核心bus LOCK或Bus任意故障回滾。
mod3未擴充。readonly／descriptor越界拒絕並保留暫存器。

## 批次34：CONFORMED — NOP與OR word immediate
gap-33停43CAF／90，鄰接66 81 CE B0 00。
90無prefix僅前進EIP、其他狀態不變；這是標準NOP，不用來略過未知指令。
66 81 /1 mod3 OR低word與imm16，保留目的高word，CF/OF清除、
SF取bit15，ZF/PF依16-bit結果；AF採既有未定義清除策略。

## 批次35：CONFORMED — byte MOV共用EA
gap-34停4202C／8A 5C 24 1C，相鄰88 9C 02 ...。
無段覆寫的8A／88 memory forms採EA32；一次byte存取，reg8依編碼區分AL..BL／AH..BH。
flags與同一32-bit暫存器其他24位保存，descriptor界限以1 byte檢查。

## 批次36：CONFORMED — INC／DEC dword memory
gap-35停4207C／FF 86 A8 01 00 00。FF /0與/1 memory使用EA32，
32-bit加／減1，保留CF，其餘算術flags依結果；先完整寫入成功才發布flags。
不擴充16-bit FF或其他group。readonly／越界不改原資料和flags。

## 批次37：CONFORMED — TEST word immediate
gap-36停43DDB／66 F7 C7 03 00。66 F7 /0 mod3取低word與imm16做AND，
只設word邏輯flags，暫存器不變。CF/OF=0、SFbit15、ZF/PF依結果；
AF採既有未定義清除策略。

## 批次 38：DOS/4GW BIOS 資料區（CONFORMED）

DOSBox-X 輔助實驗位於 workplace/fd2-input-parity-20260907/dosbox-bios-selector-20260908：
原版 0158:001F4EB3 載入 DS=0040，SELINFO 顯示基底 00000400、
界限 00000FFF、可寫資料描述子；0040:0063 四位元組為 D4 03 29 30。
此處對應 dosgolem LE 線性 0003EEB3，輸入沿用本規格固定 SHA-256。
這是輔助平台證據，不是遊戲畫面的正式同狀態對拍。

顯式安裝 BIOS 資料區配置，沿用 Machine.initBDA，不修改通用 LE 載入器。
拒絕已有非零資料的 0400–04FF 或既有 0040 描述子，避免覆蓋程式內容。
驗收：0040:0063 讀得 302903D4、初始化外記憶體不變、衝突拒絕無副作用，
再由原版入口自然重跑。其餘 BIOS 欄位沿用既有平台預設，未宣稱逐欄原版一致。

## 批次 39：顯示回掃等待指令與輸入埠（CONFORMED）

批次 38 原版重跑停於 LE 0003EEC1，E3 12 EC A9 08 00 00 00，
DOSBox 同位置亦為 JECXZ、IN AL,DX、TEST EAX,8。
依 Intel 指令契約補 E3 rel8（完整 ECX、flags 不變）、
EC（DX 低 16 位元埠、只更新 AL、拒絕未處理輸入）及 A9 imm32。
目前只接受無 prefix 形狀，其他仍拒絕。
平台輸入增加 03DA，沿用 Machine.In8 的確定性回掃狀態與屬性控制器 flip-flop；
此為 hardware-spec approximation，不代表 wall-clock 或逐掃描線時間一致。
驗收包含 ECX=00010000、不改 EAX 高位與 flags、埠拒絕無暫存器副作用、
TEST 的零與非零結果，以及自然原版重跑。

## 批次 40：回掃等待 LOOP（CONFORMED）

批次 39 原版於 LE 0003EED3 遇 E2 EE；ECX=3。
依 Intel LOOP rel8 契約減少完整 ECX，非零跳轉、零落下，flags 不變；
零初值繞回 FFFFFFFF。只接受無 prefix。驗收零、1、2、00010000 與原版重跑。

## 批次 41：有號近距離 JL（CONFORMED）

批次 40 原版 LE 00043DF4 的 0F 8C B6 FE FF FF 尚未支援。
依 Intel JL rel32，以 SF 與 OF 不同為跳轉條件，符號位移加在下一指令；
不改暫存器或 flags。驗收 SF/OF 四組與 ZF 無關性，然後自然重跑。

## 批次 42：無號 DIV 暫存器（CONFORMED）

批次 41 於 LE 0003F090 遇 F7 F3，EDX:EAX=00000000:000F4240、EBX=120。
依 Intel F7 /6 mod3：無號 64 位元被除數除以 32 位元暫存器，商 EAX、餘數 EDX。
除數零或商超過 FFFFFFFF 時拒絕且不發布暫存器；未定義 flags 保持既有值。
驗收 1000000/120、高半非零、除以零、商溢位，再自然重跑。

## 批次 43：無號 MUL 暫存器（CONFORMED）

批次 42 LE 0003E8B5 遇 F7 E1，EAX=8333、ECX=10000。
依 Intel F7 /4 mod3：EAX 乘以來源無號 32 位元，64 位元乘積寫 EDX:EAX；
高半非零則 CF/OF=1，否則清零，其餘未定義 flags 保持。
驗收零、最大值與高半非零，原版自然重跑。

## 批次 44：OR word 與符號擴展 imm8（CONFORMED）

批次 43 LE 0003D824 遇 66 83 CE 03。83 /1 mod3 的 imm8 符號擴展為 word，
與目的低 word 做 OR，高 word 保留；依既有 setLogicFlags16 設定旗標。
負立即值與正值皆測，prefix 只允許 66，然後自然重跑。

## 批次 45：Sound Blaster DSP 重設握手（CONFORMED）

批次 44 第 56819 步的 DPMI0300 已進入第二個原版驅動 7300:016A，
停於 7300:0666 的 OUT 0226,01。這不是 0300 再度缺失，而是其下硬體埠未實作。
採 Creative《Sound Blaster Series Hardware Programming Guide》2-2 至 2-5：
https://www.ardent-tool.com/sound/Sound_Blaster_HW_Programming_Guide_1st.pdf
重設埠 2x6 寫 1 再寫 0，讀取狀態 2xE bit7 顯示佇列，2xA 取出 AA。
2xC bit7=0 表示可寫；本批尚不接受 DSP 命令。未知命令必須拒絕。
只建 0220 基底，重設清除舊回覆；重複寫 0 不可重生 AA。
重設等待採立即完成的 hardware-spec approximation，不宣稱微秒、DMA 或音訊完成。
驗收握手順序、消費後狀態、重複重設與未知命令拒絕，再原版自然重跑。

## 批次 46：DSP 重設埠讀取（CONFORMED）

批次 45 同一原版 handler 再前進兩步，7300:066A 讀 0226。
DOSBox-X read_sb 的 DSP_RESET 分支回 FF：
https://dosbox-x.com/doxygen/html/sblaster_8cpp_source.html （來源行 2901–2902）。
採此相容契約，讀取不消費回覆或改 reset 狀態，不推論其微秒延遲。
測試讀取重設埠後 AA 與 ready 狀態仍存在，再自然重跑。

## 批次 47：DSP 版本查詢（CONFORMED）

批次 46 重設成功後，原版 7300:062C 送 E1 至 022C。
依 Creative 手冊 6-29，E1 回兩位元組，依序為主、副版號。
本次輔助 DOSBox 使用預設 SB16；沿用 DOSBox-X SBT_16 的 4.05 配置
（同上 sblaster.cpp 1950–1958），不代表 4.xx 全命令均已實作。
E1 清舊回覆再排入 04 05；reset 期間拒絕命令，其餘未知命令繼續拒絕。
驗收順序、舊回覆清除與 reset 期間拒絕，然後自然重跑。

## 批次 48：SB16 mixer 硬體配置查詢（CONFORMED）

批次 47 7300:08CE 遇 OUT 0224,80，為 mixer 索引埠。
Creative 手冊 2-7 定義 80 的 IRQ 與 81 的 DMA 位元。
本隔離平台明示採 SB16／0220／IRQ7／DMA1／HDMA5，參考 DOSBox-X
dosbox-x.reference.conf 的常用配置；此為平台設定，不從原版未知語意猜出。
0224 選索引並可讀回，0225 僅允許讀 80=04、81=22；
未列索引讀取與資料寫入拒絕，尚無 DMA 傳輸或 IRQ 排程。
驗收索引切換、配置讀取與未知索引拒絕，再自然重跑。

## 批次 49：PIC 遮罩暫存器（CONFORMED）

批次 48 7300:0372 讀 A1。既有 Machine 未實作 PIC 埠，不能將其預設 FF
誤稱為控制器狀態。DOSBox-X 輔助重跑以 INB 21／INB A1 量得 F8／2C，
收據在 workplace/fd2-input-parity-20260907/dosbox-pic-20260908。
先前用 IN 的失敗記錄另存，不列有效量測。封包00至15對應原版實模式呼叫；
本次只取已測啟動配置，不宣稱其他硬體組合有相同遮罩。

依 8259 IMR 的讀寫契約（DOSBox-X pic.cpp 的 imr/set_imr 亦交叉支持）：
https://dosbox-x.com/doxygen/html/pic_8cpp_source.html
LE 平台明示初始化 21=F8、A1=2C，讀取回目前值，寫入更換該控制器遮罩。
本批只有遮罩資料埠；命令埠、IRQ 請求／派送與 EOI 尚未實作並拒絕。
驗收雙控制器獨立保存、全遮／解除與未知埠拒絕，再原版自然重跑。

## 批次 50：8237A 第一控制器的程式設定介面（CONFORMED）

批次49原版7300:0220寫000A=05，遮罩DMA通道1。
採Intel 8237A資料表（1993-09，231466-005）的Register Description與Software Commands：
https://www.pcjs.org/documents/datasheets/intel/INTEL_8237A_DMA.pdf
只補第一控制器的設定介面：00–07位址／計數低高位共用flip-flop、
0A單通道遮罩、0B模式、0C清flip-flop、0D主清除、0E清遮罩、0F全遮罩。
位址／計數寫入同時更新base/current；尚未設定的位元組讀取拒絕，避免猜初值。
reset清flip-flop、全遮罩，不清未被規格指定清除的位址／計數。
本批不實作DMA請求、傳輸、page latch或第二控制器；未知埠仍拒絕。
驗收共用phase、低高位讀回、通道隔離、遮罩與reset，再自然重跑。

## 批次 51：PC/AT 第一 DMA 控制器 page latch（CONFORMED）

批次50原版7300:028F寫83=07。PC/AT通道1 page latch為83，
其餘通道0/2/3為87/81/82。DOSBox-X dma.cpp 200–203、252–255
同時支持讀寫對應：https://dosbox-x.com/doxygen/html/dma_8cpp_source.html 。
寫入與讀回8-bit page，不改8237共用flip-flop。未設定page仍拒絕讀取；
本批尚無記憶體傳輸。驗收通道對應、phase保持及原版重跑。

## 批次 52：DSP 時間常數設定（CONFORMED）

批次51原版7300:062C送40至022C。Creative手冊3-4與DSP命令40h定義：
40後下一byte為時間常數；單聲道取樣率為1000000/(256-TC) Hz。
本批保存TC與是否已設定，參數byte即使為E1也不得誤當命令；
reset清未完成命令與設定有效性。未設定不猜取樣率。
這是hardware-spec approximation的參數基礎，尚未啟動PCM或計時。
驗收參數解析、重設中止及原版自然重跑。

## 批次 53：單次 8-bit DMA 完成與 IRQ7（CONFORMED）

批次52在DSP命令14h停下。唯讀觀測確認原版自行把IVT 0F設為7300:06F3，
DMA通道1位址07359E、計數3（4 bytes）、模式48、已解除遮罩；TC=D3。
採Creative手冊單次8-bit DMA命令14h（low/high的length-1）與8237A契約：
每個樣本讀目前DMA位址、位址16-bit遞增、計數遞減；到terminal count遮罩通道；
DSP區塊完成才提出IRQ7，PIC遮罩與CPU IF控制派送，走原版IVT及CPU.Interrupt。
讀22E確認DSP IRQ，PIC20的非特定EOI清除in-service；未知PIC命令拒絕。

只支援已初始化的通道1、模式48、單次memory-to-device、區塊不超過DMA計數。
reset取消未完成DSP傳輸；不支援auto-init、第二控制器或其他DMA模式。
虛擬時間明示採每道實模式指令1微秒的執行器近似；每樣本間隔256-TC微秒，
本例4*(256-211)=180微秒。只在實模式CPU步進時推進，不能宣稱整機時鐘、
人耳音訊、逐週期或遊戲操作時序已一致。此為hardware-spec approximation；
不深挖原版DAC/PIT/ISR時序。PCM讀取有界保存為觀測，尚無音訊輸出器。

驗收：完成前不得IRQ；正確長度、位址和計數；遮罩或IF關閉仍保留待處理；
解除後僅派送一次並走真正IVT；未初始化／錯誤模式拒絕；原版入口重跑。

## 批次 54：PIC 的 IRR／ISR 查詢（CONFORMED）

批次53已完成一次4-byte DMA並派送IRQ7；原版7300:03F3寫20=0B，
需要OCW3選擇ISR。依8259及DOSBox-X pic.cpp 243–245／325–332，
0A選IRR、0B選ISR，讀命令埠回所選暫存器，不清旗標。
本平台目前只生成主控制器IRQ7；從控制器無已生成請求。
接受20/A0的0A/0B查詢選擇，保持兩者独立；EOI沿用已支援的主控制器非特定20。
驗收待處理與in-service區別、讀不清除、EOI後清除，再原版重跑。

## 批次 55：SB16 mixer 中斷來源（CONFORMED）

批次54補充觀測確認MixerIndex=82、DSPIRQPending=true；原版7300:06E3讀225。
Creative手冊2-6定義82 bit0=8-bit DMA IRQ、bit1=16-bit、bit2=MIDI。
本平台僅產生已實作的8-bit事件，回bit0；讀82不清除，讀22E才確認並清除DSP IRQ。
驗收讀取兩次保持、22E確認後清除、沒有IRQ為零，再原版重跑。

## 批次 56：DSP 區塊長度設定（CONFORMED）

批次55原版7300:062C送48；Creative手冊6-16定義後續low/high為區塊bytes-1。
僅保存BlockSize與有效性，不立即啟動DMA；命令参数共用有界兩byte狀態，
不能把參數當成另一命令。reset取消未完整設定；未知後續啟動命令仍拒絕。
驗收1與65536 bytes、參數順序、設定本身不觸發DMA，再原版重跑。

## 批次 57：8-bit auto-init DMA 與重設後取樣率（CONFORMED）

批次56原版送1C，DMA模式58、base/count已重設；DSP reset後並未重送TC。
Creative手冊6-8定義1C依48設定的區塊長度重複輸出，每區塊完成通知一次；
8237A auto-init在TC重載base/current，不遮罩通道。只開通通道1、模式58。

重設取樣率以前標未知並拒絕啟動；此缺口採DOSBox-X DSP_Reset的22050 Hz
平台預設（sblaster.cpp來源行1218）。這是模擬器配置近似，不是原版硬體實測。
TC設定仍為1000000/(256-TC)；以有理數樣本累積保留餘數，避免每樣本取整漂移。
一微秒／實模式指令的近似與停止線不變，沒有擴張到逐週期音訊逆向。

DMA資料環長度與DSP區塊長度分開計數；尚未確認上一DSP IRQ時不重造邊緣，
資料傳輸持續。reset停止傳輸。未支援的模式／命令仍拒絕。
驗收兩個區塊的資料重載、遮罩不變、IRQ確認後再派送，以及22050 Hz下
10000微秒累積220個樣本而非每樣本取整；原版自然重跑。

## 批次 58：SBB 暫存器（CONFORMED）

批次57原版數位音效偵測已以IRET返回（AX=8405），完成13區塊與13次IRQ7。
後續LE 00040C29遇1B C2。依Intel SBB r32,r/m32的mod3契約，
目的減來源再減進入時CF；無prefix。完整32-bit暫存器，CF依33-bit借位，
AF/OF依原始來源與結果，其他結果旗標照既有策略。驗收借位與符號溢位邊界，
再由原版入口自然重跑；不把8405回傳宣稱為DOSBox完整同狀態對拍。

## 批次 59：無號近距離 JA（CONFORMED）

批次58 LE 00040403 遇0F 87 8E 00 00 00。依Intel JA rel32，
僅當CF=0且ZF=0跳轉，不改暫存器與flags；32-bit有號位移加於下一指令。
驗收CF/ZF四組、正負位移與自然重跑。

## 批次 60：CS 段覆寫的間接近跳躍（CONFORMED）

批次59停於LE 00040409：2E FF 24 85 34 03 04 00。
依Intel JMP r/m32與段覆寫契約，FF /4從暫存器或完整32-bit有效位址讀目標，
CS覆寫只開放此記憶體跳躍形狀；讀取仍檢查descriptor範圍。
成功只改EIP，不推堆疊、不改旗標與通用暫存器。其他CS指令仍拒絕。
驗收CS與DS不同基底、SIB、越界拒絕、暫存器跳躍與原版自然重跑。

## 批次 61：帶索引的 byte 即值比較（CONFORMED）

批次60在LE 0004049F遇80 3C 19 00，即CMP byte [ECX+EBX],0。
依Intel 80 /7，對無段覆寫的記憶體運算元採既有完整EA32解碼，
唯讀來源、以sub8更新算術旗標，暫存器與記憶體不變。
驗收SIB、預設SS、無號借位與相等，越界失敗後旗標不变，再自然重跑。

## 批次 62：雙運算元暫存器 IMUL（CONFORMED）

批次61停LE 00040620，0F AF D0。依Intel IMUL r32,r/m32之mod3，
兩個有號32-bit來源相乘，目的取低32-bit；若不能以有號32-bit表示則CF/OF置位，
否則清除。其餘未定義旗標沿用既有保留策略，不宣稱硬體一致。
驗收正負乘積、零、最小負數與截斷溢位，並原版自然重跑。

## 批次 63：dword 記憶體即值比較（CONFORMED）

批次62停LE 0004067C：81 7C 24 20 00 08 00 00。
依Intel 81 /7，無prefix的r/m32記憶體採EA32讀取，再與imm32作sub32旗標比較，
不寫記憶體或暫存器。驗收SS堆疊與SIB、正負界限及越界拒絕，再自然重跑。

## 批次 64：固定一位的算術右移（CONFORMED）

批次63停LE 000406D7：D1 F8。依Intel SAR r32,1，mod3／7，
保留符號右移一位，CF取原bit0，OF清除，SF/ZF/PF依結果；
AF採既有未定義策略。驗收正數、負數、零及最小負數，再自然重跑。

## 批次 65：記憶體來源的 SUB（CONFORMED）

批次64停LE 000406AF：2B 46 10，即SUB EAX,[ESI+10h]。
依Intel 2B /r，32-bit目的暫存器減記憶體來源，採EA32與預設DS/SS，
只更新目的與sub32旗標。來源越界不發布結果。驗收借位、溢位、SS及自然重跑。

## 批次 66：CDQ 符號延伸（CONFORMED）

批次65停LE 000406B2：99。依Intel CDQ，EAX符號延伸到EDX:EAX：
EAX最高位0則EDX=0，否則EDX=FFFFFFFF；EAX與全部旗標保持。
本批只開放無prefix的32-bit形狀。驗收兩側符號界限與自然重跑。

## 批次 67：DSP 4.xx 喇叭旗標（CONFORMED）

批次66回到原版實模式7300:062C，DSP命令D1尚未支援。
Creative原廠手冊6-25至6-28明定：4.xx的D1/D3只改内部喇叭旗標，
不改實際輸出訊號；D8讀回FF為開、00為關。採同一旗標介面完整讀写，
不暫停DMA。重設清旗標；命令參數仍優先於新命令解析。
来源：https://www.ardent-tool.com/sound/Sound_Blaster_HW_Programming_Guide_1st.pdf 。
本平台採即時旗標設定的硬體近似，不新增112/220ms busy-wait考古。
驗收開關讀回、重設、與DMA回呼獨立，原版自然重跑。

## 批次 68：通用記憶體來源 IMUL（CONFORMED）

批次67原版音效啟用AX0305已返回AX1，接續LE 00040D1E遇0F AF 4D 40。
沿Intel雙運算元IMUL與批次62旗標契約，把既有單一堆疊形狀改為EA32，
目的由ModRM.reg選取，預設DS/SS；讀取失敗不改目的或旗標。
驗收不同目的、EBP預設SS、正負乘積、跨界拒絕與自然重跑。

## 批次 69：非分頁 DPMI 的線性鎖定契約（CONFORMED）

批次68停Watcom int386轉接：0600鎖定76000+1001超過目前backing。
查證DPMI 1.0的0600註記：不支援虛擬記憶體的host忽略此呼叫並清CF，
不配置記憶體，也不要求逐物件區間。來源：
https://www.delorie.com/djgpp/doc/dpmi/api/310600.html 。

目前DPMIHost本就採非分頁0600成功且記錄Locks，Watcom轉接卻另以Mem長度
拒絕，兩條同服務路徑不一致。修正轉接0600沿用同一host與REGS欄位映射。
這是既有非分頁平台契約的統一，不擴張heap、不偽造可讀bytes；
後續CPU／DMA讀寫仍檢查實際backing。沒有host的轉接需明確拒絕。
本批不引入可分頁host、鎖定計數或換頁機制。

驗收兩條入口鎖定紀錄相同、跨目前backing不增加Mem、REGS CFLAG清除、
未知服務仍拒絕、真實原版入口重跑。批次10的頁backing仍有效，
但其「0600必須按Mem界限拒絕」已由公開非分頁契約否定；歷史收據不刪除。

## 批次 70：dword 記憶體 TEST 即值（CONFORMED）

批次69停LE 00049584：F7 43 1C 20 00 00 00。
依Intel F7 /0無prefix，EA32記憶體讀取後與imm32作AND，
只依結果更新邏輯旗標，不改資料。驗收位元命中／未命中、SIB預設SS、
來源越界不改旗標，原版自然重跑。既有word形狀不變。

## 批次 71：索引記憶體的間接 CALL（CONFORMED）

批次70停LE 000495C5：FF 14 85 38 76 04 00，即CALL [EAX*4+47638h]。
依Intel FF /2 r/m32，無prefix的記憶體CALL採EA32讀目標，成功把指令尾EIP
推入SS:ESP-4後發布新ESP/EIP。目標在壓棧前讀取，含ESP來源的位址取舊ESP。
來源或堆疊越界失敗，不發布新ESP或跳躍。驗收SIB目標、ESP來源順序、
返回位址與flags保持，再原版自然重跑。

## 批次 72：EAX 即值 XOR（CONFORMED）

批次71停LE 00047A3D：35 00 80 00 00。依Intel XOR EAX,imm32，
目的與32-bit即值互斥或，清CF/OF，SF/ZF/PF依結果；AF沿既有未定義策略。
無prefix，其他暫存器不變。驗收符號位、零、上16-bit保留正確與自然重跑。

## 批次 73：以舊 ESP 定址的記憶體 PUSH（CONFORMED）

批次72已自然執行77269步，停LE 000111DD：FF 74 24 14。
依Intel FF /6，先依舊ESP及EA32讀取r/m32，再寫入SS:ESP-4並更新ESP。
來源／目的越界拒絕，flags不變；只開無prefix記憶體形狀。
驗收ESP來源順序、SS與其他基底、目的失敗不更新ESP及自然重跑。

## 批次 74：記憶體目的的 ADD／SUB（CONFORMED）

批次73停LE 0003775A：01 03，下一指令29 43 04。
依Intel 01 /r與29 /r，r/m32目的加／減r32来源。無prefix記憶體採EA32，
先完整讀取及確認寫入，再發布算術旗標；拒絕時記憶體與旗標保持。
驗收DS/SS、SIB、無號進借位與有號溢位、唯讀拒絕，原版自然重跑。

## 批次 75：F2 MOVS 的相容重複前綴（CONFORMED）

批次74停LE 0003CBEF的F2 A5，後續同一程式視窗有F2 A4。
F2套MOVS不可僅以REPNZ名稱推論ZF條件。DOSBox-X 2026.07.02輔助量測
0158:001F2BEF→001F2BF1：ECX 2→0、ESI 20D742→20D74A、
EDI 19F3A8→19F3B0；8bytes確實由來源複製，後8bytes目的不變。
原始收據在dosbox-f2-movs-20260908，是真正DOSBox輸出，非dosgolem收據。

相容契約：F2 A5/A4與F3相同，依ECX重複，不看ZF；沿既有DF、
逐元素讀寫與失敗停止，不宣稱所有x86世代的未文件化prefix均如此。
DOSBox-X prefix_none.h亦將F2設定為重複前綴：
https://raw.githubusercontent.com/joncampbell123/dosbox-x/master/src/cpu/core_normal/prefix_none.h 。
本批以原版實際相容行為為據，非全CPU架構保證。

驗收兩種寬度、ZF兩值、DF雙向、零count不存取，再由dosgolem原入口自行重跑。

## 批次 76：通用記憶體來源 CMP（CONFORMED）

批次75停LE 00037103：3B 03。依Intel 3B /r無prefix32-bit比較，
目的暫存器減r/m32來源，只發布旗標。以EA32取代既有多個位址特例，
不新增word或段覆寫。驗收基底／SIB／SS、借位溢位、來源拒絕與自然重跑。

## 批次 77：記憶體來源無號 DIV（CONFORMED）

批次76停LE 0003723C：F7 75 18。依Intel F7 /6，
EDX:EAX除以r/m32，商存EAX、餘數存EDX；旗標未定義沿既有保留策略。
來源透過EA32與段界限取得。除零／商超32-bit／來源失敗均不發布結果。
驗收SS來源、正常商餘與錯誤原子性，再原版自然重跑。

## 批次 78：word 即值 AND（CONFORMED）

批次77停LE 0003712E：66 81 E7 00 FE。依Intel AND r16,imm16，
只更新目的低16-bit，保留高16-bit，採16-bit邏輯旗標。
既有同群組OR保持相同行為，改共用既有setLogicFlags16以避免寬度分岐。
驗收高位保留、符號與零旗標、自然重跑。

## 批次 79：近堆配置／釋放成對轉接（CONFORMED）

前置：[IDA及原版軌跡證據](../re/2026-09-08-fd2-nfree-bridge.md)。
批次78之3D33B越界是合成物件未具原版header卻進入原生釋放，
不是新CPU缺件。保留既有_nmalloc入口；同一heap新增已證實37426的_nfree入口，
按cdecl讀ESP+4，維持callee保存暫存器，EIP取返回、ESP只加4；
本平台保留其他暫存器及旗標（caller-saved不宣稱位元一致），清原版byte_5419C。

配置記錄每個live物件的4-byte對齊範圍；釋放NULL無作用，只有精確live起始
位址可釋放；未知／內部指標／重複釋放明確拒絕，不觸碰原生header。
已釋放範圍可按位址first-fit重用並分割；相鄰空閒範圍合併，
重用仍清零且不覆蓋活物件／DPMI區域；最大容量保持既有界限。
釋放本身不改物件bytes、不縮小backing。未登錄入口維持正常CPU解碼。

驗收正常配置釋放再配置、分割合併、NULL、非法／重複釋放、cdecl與副作用、
DPMI不重疊及自然入口越過原版釋放路徑。這是runtime適配，不宣稱原生heap
位址或內部管理器完全一致。

## 批次 80：Watcom 的 DPMI DOS 記憶體釋放轉接（CONFORMED）

批次79配置／釋放成對後，位址重用使原版在第56991步經Watcom int386
呼叫DPMI0101；host本已支援此服務，轉接尚未開通。
這是配置路徑改變，不以較少步數冒称向前，也不保證舊指標布局不變。
沿既有DPMIHost0101契約：DX指定selector，成功刪DOS block與descriptor，
不回收位址；無效selector回CF與8022。Watcom依既有REGS映射回填，
只有0100才需要調整dosBrk避開heap。未知其他功能仍拒絕。
驗收有效釋放、失敗碼、caller暫存器保持與原版自然重跑。

## 批次 81：保留 VGA／ROM 區域的配置邊界（CONFORMED）

批次80的唯讀int386-80觀測確認INT10、AX0013，準備320×200顯示。
既有近堆從LE尾端向上配置、DOS配置上限1MiB，兩者都可能侵入A0000顯示區；
不能只開畫面後清VRAM，否則會破壞已配置物件。
公開VGA記憶體映射A0000–BFFFF見：
https://www.scs.stanford.edu/10wi-cs140/pintos/specs/freevga/vga/vgamem.htm 。

本PC配置明確不開UMB：DOS可配置區止於640KiB；DPMI線性配置仍從1MiB以上。
FD2近堆也從max(LE尾端,1MiB)開始，原有1MiB容量不變。
共用Mem只是backing，不代表所有區域可配置；DOS游標不再因高位近堆而跳高。
DPMI線性配置若碰到其他配置增長的Mem，先跨過已背書尾端；
近堆遇外部增長也只向上移，不得把高位起點拉回傳統記憶體。
通用小型heap測試夾具仍可用原構造器，FD2安裝入口才選擇高位配置。

這是平台位址配置的修正，配置位址及步數可改變，不宣稱DOSBox位址一致。
驗收640KiB邊界、high heap／DOS／DPMI交錯不重疊、容量與自然入口回歸。
本批尚未接INT10或產生畫面。

## 批次 82：VGA 13h 與 Watcom INT10 受限接線（CONFORMED）

前置批次81記憶體分區回歸657筆通過事件。原版INT10 AX0013需要顯示模式。
新增明示LEVideo裝置並接Watcom int386：只接受AX0013，
設定BDA模式13與40文字欄、清A0000–AFFFF，320×200色號由同一Mem讀回。
未安裝裝置、未知BIOS功能或backing不足均拒絕；不先擴充任意BIOS回傳。

沿既有Machine DAC的3C8索引／3C9三分量6-bit寫入與6→8-bit色彩展開。
CPU port輸出接同一平台，以未知埠拒絕；尚未支援的VGA寄存器另按停點補齊。
BIOS預設DAC目前未量測，因此保存為未知；只有遊戲寫過的調色盤可作顏色證據。
裝置能擷取色號不等於已產生遊戲畫面；全黑初始化圖不得標為標題或PLAYER-E2。

驗收模式記錄、清畫面不碰heap、DAC索引遞增／6-bit遮罩、未知功能拒絕、
共用REGS回填及自然原版重跑。此為mode13平坦顯示子集，尚非全部VGA。

## 批次 83：PIT 通道0的模式3設定（CONFORMED）

批次82首次把保護模式OUT交給嚴格平台後，第2951步3E86E輸出43=36。
此前LoadLE預設只記錄OUT即成功，因此1–81收據只能證明列出的CPU／實模式
子集；不能證明保護模式硬體輸出已被實作。這項限制明確追加，不能把
新嚴格路由提前停止寫成遊戲倒退。82顯示接線仍待原版真正抵達。

Intel 8254資料表231164-004的控制字36：通道0、先低後高、mode3、binary；
後續兩次40寫入形成16-bit重載，0代表65536。重寫控制字清未完成的組字狀態。
來源：https://www.scs.stanford.edu/10wi-cs140/pintos/specs/8254.pdf 。

只補這個實際設定子集，不假稱已推進PIT計數或派送IRQ0；未知讀埠／模式拒絕。
後續玩家時序需另接機器虛擬時鐘，以公開規格近似處理，遵守PIT硬體考古停止線。
驗收低高位順序、0重載、重設中斷設定、未知形狀拒絕，嚴格原版重跑。

## 批次 84：記憶體來源的三運算元 IMUL（CONFORMED）

批次83嚴格原版重跑86620步，已實際設定mode13，尚全黑且未寫DAC。
停LE 0003C936：69 10 6D 4E C6 41。依Intel69 /r r32,r/m32,imm32，
EA32唯讀來源乘有號imm32，目的取低32-bit，CF/OF指示有號截斷；
其他旗標沿既有未定義保留策略。先完成EA與即值解碼再發布結果。
驗收正負乘積、截斷、SS來源、跨界拒絕及原版自然重跑。

## 批次 85：反向編碼的暫存器 XOR（CONFORMED）

批次84停LE 0004E893：33 C0。依Intel33 /r mod3，
目的ModRM.reg與来源r/m互斥或；清CF/OF、SF/ZF/PF依結果。
無prefix，記憶體形狀仍拒絕。驗收不同來源與零結果、原版自然重跑。

## 批次 86：16 位元累加器讀取、加法及旋轉（CONFORMED）

gap-85 在 LE 4E895 的實際視窗依序為 66 A1 B8 27 06 00、
66 05 14 90、三次 66 D1 C0。按 Intel 標準指令契約：
A1 仍取32位元位址，透過DS完整讀16位元，保留EAX高半部及旗標；
05 更新AX與16位元算術旗標，高半部不變；
D1 /0 mod3 的ROL r16,1循環左移，CF取原bit15，OF取結果bit15 XOR CF，
其他旗標及高半部保持。未列段覆寫與重複前綴拒絕。
不將該原版函式推論為特定遊戲或亂數語意。
驗收DS基底／界限、進位／溢位／半進位、旋轉雙邊界及原入口重跑。

## 批次 87：MOVZX byte 的通用記憶體來源（CONFORMED）

gap-86自然停LE25982的0F B6 05。依Intel MOVZX r32,r/m8，
以既有EA32計算DS／SS、SIB與位移，完整讀byte後零擴展到目的暫存器；
不改來源或旗標。以通用來源取代舊modrm特例，不擴充prefix。
驗收絕對位址、SS基底、零擴展、界限拒絕與原版重跑。

## 批次 88：byte 來源比較（CONFORMED）

gap-87自然停LE49645的3A 0A。依Intel CMP r8,r/m8，
用8-bit減法只更新旗標；暫存器含AH等高byte映射，記憶體採EA32及段界限。
無prefix，失敗不發布旗標。驗收高byte、DS來源、有號溢位／借位與原入口重跑。

## 批次 89：統一32位元近距離條件跳躍（CONFORMED）

gap-88停LE44120的0F83（JAE）。用Intel0F80–8F同族16種条件，
取有號rel32，成立才相對指令尾跳轉；暫存器與旗標不變。
依序為OF、非OF、CF、非CF、ZF、非ZF、CF或ZF、兩者皆無、
SF、非SF、PF、非PF、SF異於OF、SF等於OF、ZF或SF異於OF、
非ZF且SF等於OF。16-bit不開放。
以五個條件旗標的全部32組值驗證16種跳躍及負位移，取代散落白名單。

## 批次 90：C6 的通用記憶體 byte 寫入（CONFORMED）

gap-89停LE1F8C3的C6 44 24 58 00。依Intel C6 /0 r/m8,imm8，
記憶體目的以EA32取代舊特例；先完整取EA與即值，再按段寫入權限寫一byte。
旗標／暫存器保持，未列prefix及非記憶體仍拒絕。
驗收ESP加位移、SS基底、相鄰byte保存與唯讀拒絕，再原版重跑。

## 批次 91：DX 指定的 byte 埠輸出（CONFORMED）

gap-90停LE3779E的EE，DX=03C8，AL=00。
依Intel OUT DX,AL取DX低16位元及AL低8位元，交既有PortOut；
未安装或拒絕時明確錯誤，不改暫存器與旗標。沿E6的無prefix限制。
驗收截斷、一次輸出、拒絕及原版自然重跑。03C8已由既有DAC路由支援。

## 批次 92：LODSW／LODSD 字串載入（CONFORMED）

gap-91已寫入768次DAC分量，停LE4E646的66 AD。
依Intel AD與66 AD：DS:ESI讀dword／word至EAX／AX，
word保留高16位；成功後ESI依DF增減4／2，旗標及ECX保持。
無重複前綴或段覆寫。逐元素完整界限拒絕，不在失敗時發布目的或指標。
驗收兩個寬度、雙向DF、段基底與跨界拒絕，再由原版入口重跑。

## 批次 93：同一解碼視窗的 byte 位移與 word 運算（CONFORMED）

gap-92 LE4E696視窗含D0 E1、C0 E9 02、66 2B D9與66 0B DB。
依Intel標準契約補D0／C0的暫存器SHL/SHR，count取1／imm8低5位，
零count保留全部；非零更新SF/ZF/PF，CF取最後移出bit；
count1的OF為SHL結果符號 XOR CF／SHR原符號，其他count的OF未定義保留。
count達寬度時CF未定義，採逐次移位結果，不宣稱未定義旗標與硬體一致。
word SUB／OR只開mod3，保留高半部，依16位元算術／邏輯旗標。
FE INC、AC及F3 AA已有實作，沿用並由原版路徑消費。
驗收count0/1/2/8/32、byte高位映射、進位溢位及原版重跑。

## 批次 94：word 記憶體 DEC 與原生畫面快照（CONFORMED）

gap-93已有320個非零色號，停LE4E6ED的66 FF 0D B6 27 06 00。
依Intel FF /1 r/m16，本批只開EA32記憶體DEC：完整讀寫兩byte，
成功後更新16-bit減法旗標但保留CF；拒絕時不更動旗標或記憶體。
測試有號溢位、零、CF保存、唯讀拒絕，再原版自然重跑。

bootprobe新增選用PNG輸出，直接以自身VGA色號及DAC快照編碼，
保留320×200原生尺寸、PNG雜湊與分類，不覆寫既有輸出。
這是停點觀測，不因非零色號自動聲稱遊戲畫面完整或玩家路徑已驗收。

批次94第一次自然重跑滿500000步，沒有未支援指令，63510個非零色號、
15186次DAC分量寫入；PNG人工檢視為淡入中的漢堂標誌。
此為局部開場可見證據，未與DOSBox同狀態比較，不提升整體PLAYER-E2。
探針允許明示1至20000000步的有界延長，不注入或跳過原版狀態。

## 批次 95：PIT 驅動的預設 BIOS 時鐘（CONFORMED）

DOSBox輔助收據dosbox-bios-clock-20260908：
0158:001CDADD→001CDADF（dosgolem LE17ADD→17ADF），
0040:006C由65 B1 15 00增為66 B1 15 00，EAX由0變1、EDX保持1；
相同等待迴圈因此退出。這不是完整同狀態或wall-clock量測。
dosgolem gap-94-5m仍讀零，只有PIT設定而無時鐘消費端，原因已確認。

公開契約：DOSBox-X timer.h的IBM PIT輸入1193182Hz；
bios.cpp INT8_Handler每IRQ0增加0046C的dword，達1800B0回零並增加00470，
預設INT1C無作用。來源：
https://dosbox-x.com/doxygen/html/timer_8h_source.html
https://dosbox-x.com/doxygen/html/bios_8cpp_source.html

新增明示、可選LE BIOS clock平台裝置，從CPU StepHook及既有實模式步進共用：
每指令1微秒是hardware-spec approximation，非實機速度或逐週期相等。
用整數累積1193182／1000000時基，PIT重載控制週期；預設65536，
重新完成設定時重設相位。屏蔽或IF清除期間IRQ0只保留一個pending edge，
開放後派送一次預設BIOS服務，不追補遺失中斷。
預設BIOS以平台服務實作，不偽造遊戲ISR；若INT08或INT1C已安裝非零向量，
立即拒絕尚未支援的客製handler，不默默略過。服務保留CPU暫存器及旗標。
這僅支援無客製IRQ0／INT1C的BIOS時鐘，非完整PIC優先序或全音訊排程。
既有DSP仍僅於實模式推進；此限制保留，不宣稱聲音時序已閉合。

驗收分數週期、重載、IF及mask、pending合併、午夜、未知handler拒絕，
原版入口重跑必須自然越過等待，不可改EIP或直接寫入遊戲等待結果。

## 批次 96：MOVZX word 的通用記憶體來源（CONFORMED）

gap-95的BIOS clock派送548次，原版自然越過等待及標誌淡出，
第2750708步停LE2051B的0F B7 44 24 04。
依Intel MOVZX r32,r/m16，保留暫存器來源，記憶體改用EA32，
完整讀兩byte再零擴展；旗標不變，越界不發布目的。
驗收ESP來源／SS基底、零擴展與跨界拒絕，再原版自然重跑。

## 批次 97：word 即值位移（CONFORMED）

gap-96停LE36CB5的66 C1 E0 02。按Intel C1 /4、/5、/7的r16暫存器
SHL／SHR／SAR，count低5位、零不變；高半部保存。
非零更新SF/ZF/PF，CF採最後移出bit；count1 OF依左移符號異或CF、
右移原符號或SAR清除。其他count OF未定義保留，AF沿現有清除策略；
count>=16的未定義CF不宣稱硬體一致。無其他prefix或記憶體來源。
驗收雙向與有號、count0/1/2/16/32、高半部及自然原版重跑。

## 批次 98：下一段解碼迴圈的 word 運算與 STOSW（CONFORMED）

gap-97 LE36BE0視窗包含66 3B DA、66 03 D9、D1 E9、
F3 66 AB、66 43。依Intel契約補word暫存器CMP／ADD／INC，
高半部保存，INC保留CF；ADD16共用既有已驗證累加器算法。
補D1 /5暫存器SHR1（CF原bit0、OF原bit31）。
AB增加word寬度及F3重複，ES:EDI逐元素寫2byte，
成功後EDI依DF±2、ECX遞減；零count不寫，越界保留此前完成迭代。
F2 AB尚不開放。驗收算術旗標、INC CF、高位、STOS雙向／零／部分失敗，
原版自行重跑；不把標準解碼指令命名為未證實的遊戲規則。

## 批次 99：重複 MOVSW（CONFORMED）

gap-98停LE36C70的F3 66 A5。依Intel MOVSW，
DS:ESI至ES:EDI逐word複製，成功後兩指標依DF±2，
REP依ECX消費、零count不存取，旗標保持；部分失敗只保留完成迭代。
沿既有MOVSD及DOSBox已量測F2 MOVS相容重複契約，增加word寬度，
不擴充段覆寫。驗收正反向、重疊逐次語意、跨界及原版自然重跑。

## 批次 100：SB16 直接輸出取樣率（CONFORMED）

gap-99-20m第5674595步，DPMI0300實模式停67FF:062C，
OUT022C值41。此為Creative DSP4.xx標準41h，後續先高byte再低byte取Hz，
手冊第6-15頁允許5000–45000。完整接收才更新有理取樣率Hz/1，
TimeConstantKnown清除；半筆輸入不發布、無效率拒絕，reset取消pending。
來源：https://www.ardent-tool.com/sound/Sound_Blaster_HW_Programming_Guide_1st.pdf 。
不反組譯DAC／PIT硬體driver。驗收高低順序、上下界、拒絕與原版重跑。

## 批次 101：byte 記憶體 INC（CONFORMED）

gap-100第5694028步停LE1FC04的FE /0記憶體形狀；
原版實模式0401音效呼叫已返回AX1。依Intel INC r/m8，
EA32讀一byte，加1寫回，成功更新8-bit算術旗標並保留CF。
沿既有暫存器/0，不開DEC或未知prefix。
驗收SS堆疊位移、符號溢位、CF及唯讀拒絕，自然重跑。

批次101自然執行20000000步無未支援指令，動畫持續推進；
為觀測完整開場，探針上限提高至明示100000000步，外層仍120秒逾時。
DMA/IRQ7完成數仍13，只是初始化數量；保護模式音效排程尚缺，
不可因動畫前進宣稱音畫同步完成。

## 批次 102：ADD dword 的通用記憶體來源（CONFORMED）

gap-101-100m自然執行51394659步停LE16899的03 54 82 06。
依Intel ADD r32,r/m32，以共用EA32取代舊stack／base特例；
完整讀來源後才發布結果及32-bit算術旗標。word分支沿批次98。
驗收SIB比例與位移、DS／SS、溢位及跨界拒絕，自然原版重跑。

## 批次 103：同一資料處理視窗的 ADD／ROL／XOR（CONFORMED）

gap-102 LE4DBEA視窗為66 81 C2 14 90、66 C1 C2 03、32 C2。
依Intel補81 /0 word暫存器ADD，共用add16；C1 /0 word ROL，
本子集只接受遮罩後0–15，零count保留，其餘循環旋轉並CF取結果bit0，
count1 OF為結果bit15 XOR CF，多位OF未定義保留；高半部及其餘旗標保存。
32 /r mod3 byte XOR透過reg8含高byte映射，依8-bit邏輯旗標。
不推論這個函式的遊戲或亂數語意。驗收寬度、CF／OF、高byte及自然重跑。

## 批次 104：標題選單的 BIOS 增強鍵盤讀取（CONFORMED）

gap-103-service自然停Watcom int386：中斷16h、REGS.AX=1013h；
gap-103.png人工檢視已是FLAME DRAGON2、START／LOAD／CONTINUE標題選單。
依DOSBox-X bios_keyboard.cpp INT16 AH10：從BIOS按鍵佇列取scan／ASCII字，
空佇列等待且不能偽造按鍵；特殊低byte F0且scan非零時低byte清零。
來源：https://dosbox-x.com/doxygen/html/bios__keyboard_8cpp_source.html 。

新增明示LEBIOSKeyboard，使用BDA041A/041C head/tail、041E–043D的標準
16-word循環緩衝（保留一槽，容量15）；驗證偶數位置、範圍與寫入界限。
Enqueue接收已解碼BIOS按鍵事件，滿時拒絕，不覆寫未讀按鍵。
這是BIOS層輸入，尚非8042原始scan、IRQ1或長按重複／釋放手感驗收。
Watcom只開AH10，其他功能明確拒絕；阻塞時保留EIP、ESP、REGS、暫存器，
讓主機時鐘仍可推進；有鍵才依既有cdecl返回AX，其他REGS與旗標沿保留策略。
探針可在等待邊界送出明示的方向／Enter／Esc按鍵，記錄順序；
這是平台輸入自動化，不是修改遊戲狀態。無鍵時停止探針並記為等待輸入，
不是錯誤或遊戲完成。驗收佇列順序／滿／回繞／無效BDA、阻塞不發布、
真實標題選單方向鍵與確認鍵路徑。

## 批次 105：byte 無號乘法（CONFORMED）

gap-104-start消費down／up／enter三個BIOS按鍵，
第54919969步停LE4DC0C的F6 E4。依Intel MUL r/m8，
本批只開mod3：原AL乘來源byte（含原AH），16-bit結果存AX，
EAX高半部不變；AH非零時CF/OF置位，否則清除，其餘未定義旗標保留。
驗收AH來源先讀、零／255邊界、CF／OF與原版正常按鍵路徑重跑。

## 批次 106：IMUL 的有號8位元即值（CONFORMED）

gap-105正常START路徑停LE10A5B的6B 05 E3 3B 05 00 06。
Intel6B /r r32,r/m32,imm8沿既有69乘法，差異只在取一byte並符號擴展；
來源EA32、目的低32-bit、CF/OF指示有號截斷，其他旗標保留。
驗收負即值、溢位、記憶體來源及自然按鍵路徑重跑。

## 批次 107：TEST byte 的通用記憶體來源（CONFORMED）

gap-106正常START路徑停LE10CF7的F6 44 07 06 40。
Intel F6 /0 TEST r/m8,imm8，改以EA32統一既有記憶體特例；
完整讀來源及即值，只更新邏輯旗標，不改來源，越界不發布。
暫存器與MUL分支保持。驗收SIB／SS、位元旗標與拒絕，自然重跑。

## 批次 108：byte 至 word 的零擴展（CONFORMED）

gap-107停LE10E87的66 0F B6 08。Intel MOVZX r16,r/m8，
來源沿既有暫存器／EA32 byte讀取，目的只替換低16-bit，
高16-bit與旗標保持。零擴展只清目的bit8–15，不得清整個32-bit目的。
驗收高byte暫存器、記憶體、高半部與原版正常START路徑。

## 批次 109：OR byte 的通用記憶體目的（CONFORMED）

gap-108停LE146CD的80 08 80。Intel80 /1 r/m8,imm8，
記憶體以EA32完整讀寫一byte，寫成功才更新邏輯旗標；來源拒絕或唯讀不發布。
以此取代舊單一SIB OR特例，無新增prefix。
驗收DS目的、相鄰byte、CF/OF清除與唯讀拒絕，自然重跑。

## 批次 110：word 暫存器 CMP 與 dword 記憶體 XOR（CONFORMED）

gap-109 LE3CE09視窗有66 83 FF 02及33 05 E8 37 05 00。
Intel83 /7 word mod3以有號imm8擴至16-bit比較，僅更新sub16旗標；
33 /r dword來源擴為EA32記憶體，成功讀取後XOR到目的、邏輯旗標。
驗收負即值、高位不變、DS記憶體與拒絕，自然START路徑重跑。

## 批次 111：byte 符號擴展與檔案呼叫觀測（CONFORMED）

gap-110停LE46329的66 0F BE 80 7C 37 05 00。
依Intel MOVSX r16/r32,r/m8，來源沿reg8／EA32，依目的寬度符號擴展，
word保留高半部，旗標不變；越界不發布。驗收負值、零、高byte及記憶體。
此位置可能是檔案存取錯誤處理，尚不能推論是哪個遊戲檔案；
探針新增唯讀INT21開檔參數／結果觀測，再定位平台檔案能力。

## 批次 112：FD2.TMP 的隔離可寫覆蓋層（CONFORMED）

gap-111唯讀INT21觀測確定最後呼叫為AX3D01、FD2.TMP，
平台回AX5、CF=1；隨後46329／46331是錯誤處理中的CPU停點，
不應繼續沿錯誤分支深挖來取代檔案能力修正。

依DOS3D開啟模式0讀、1寫、2讀寫及40寫入契約，新增可選WriteFileProvider；
唯讀模式預設保持。DirectoryOverlayFiles使用獨立os.Root：
讀取先查覆蓋層再查唯讀原版；第一次開寫複製既有原檔到覆蓋層，開寫不截斷。
大小寫不敏感、只接受單一檔名，拒絕路徑與非regular覆蓋檔；
覆蓋根不能與原版根相同，原版mount仍唯讀。缺檔回2，無權限回5；
未開放建檔／刪除／改名，不偽造成功。

DOS40以BX handle、CX低16位長度及DS:EDX緩衝寫入，完整驗證輸入後才寫；
正常返回AX實際bytes、清CF，無效handle回6，拒絕回5。
零長度依目前檔案位置截斷／延伸；開啟權限仍由底層file模式強制。
參考DOSBox-X原始碼及Go1.24 os.Root：
https://dosbox-x.com/doxygen/html/dos_8cpp_source.html
https://pkg.go.dev/os#Root.OpenFile

探針以明示-state目錄開啟，每批原版重跑使用全新state目錄。
驗收原檔bytes不变、第一次複製、後續讀取覆蓋內容、唯寫不可讀、
唯讀不可寫、拒絕路徑／symlink／同根、寫入與截斷、原版正常START重跑。

批次112契約勘誤：舊openReadOnly把供應器所有error都映射2，
造成明確fs.ErrPermission也被誤報缺檔；現在保留fs.ErrNotExist→2，
其餘拒絕→5。既有unsafe-path測試預期同步為5，仍驗證CF及不配置handle，
missing-file仍驗證2，未放寬存取。第一次回歸因此失敗，修正預期後乾淨重跑。

## 批次 113：byte 記憶體 XOR（CONFORMED）

gap-112已成功以3D01開啟FD2.TMP並在覆蓋層截為零長度；
原始207360 bytes、SHA256 af7b9687d133c52563dd570bfbcb69c1fb968e396e0a3911d82de29ea3f20183保持。
停LE11F0F的80 35 40 3A 05 00 01。Intel80 /6記憶體XOR，
沿OR的EA32與先寫後旗標契約，只把OR運算改為XOR；
相鄰byte與其他狀態保持，唯讀拒絕。驗收抵消／符號位與原版進場重跑。

## 批次 114：byte 暫存器 OR 與 DEC（CONFORMED）

gap-113 LE4DF07視窗包含0A FF與FE CB。
Intel0A /r mod3以reg8映射OR來源與目的，含高byte，依byte邏輯旗標；
FE /1 mod3的DEC減1但保留CF，其餘算術旗標依sub8。
2A及字串複製已有支援，不重做。驗收高byte、零、符號溢位、CF保存，
原版自然START與乾淨覆蓋層重跑。

## 批次 115：王宮開場的堆疊保存與 word 來源（CONFORMED）

gap-114 正常 START 路徑第57303809步停 LE4DFCD：60；
後續視窗含66 2B 05 00 00 06 00及A0 02 00 06 00。
依 Intel PUSHAD／POPAD 契約成對補60／61（僅32位元）：
PUSHAD依EAX、ECX、EDX、EBX、原ESP、EBP、ESI、EDI壓棧，
POPAD反向讀取但略過保存ESP槽，以目前ESP加32。旗標保持。
完整堆疊descriptor範圍先驗證，POPAD先讀齊再發布所有暫存器；
Bus寫入錯誤仍可能保留已寫byte，不宣稱具可重啟CPU例外。
SUB word增加EA32來源，完整讀取後更新低半部與sub16旗標；
A0讀DS:moffs32一byte至AL，EAX高24位及旗標不變。
驗收堆疊順序／原ESP／略過槽／越界與唯讀、word來源及AL保留，再正常重跑。

## 批次 116：IDIV 的通用記憶體來源（CONFORMED）

gap-115正常START第61101467步停LE2CA79的F7 7C 24 20。
Intel F7 /7 IDIV r/m32：以EA32讀有號除數，EDX:EAX解為有號64位元；
商向零截斷，餘數同被除數符號；商存EAX、餘數存EDX。
零除及超出int32商範圍拒絕，含MinInt64/-1；讀齊及驗證後才發布，
未定義旗標保留。取代既有mod3專用分支，驗收SS來源、負值、溢位及原版重跑。

## 批次 117：byte 記憶體 DEC（CONFORMED）

gap-116正常START第61742990步停LE13949的FE 48 01。
Intel FE /1 DEC r/m8，沿已驗證INC的EA32與先寫後旗標契約，
減1並依sub8更新旗標、保留CF。拒絕不發布。
驗收符號溢位、CF及唯讀拒絕，再原版自然重跑。

## 批次 118：word 符號擴展的暫存器來源（CONFORMED）

gap-117正常START第65718640步停LE12E7D的0F BF C3。
Intel MOVSX r32,r/m16：增加mod3取來源低word，再符號擴展至32位元；
旗標及非目的暫存器不變，目的與來源相同也先取原值。
既有EA32記憶體來源不變。驗收8000／FFFF／7FFF／0及alias，再正常原版重跑。

## 批次 119：word 暫存器 DEC（CONFORMED）

gap-118正常START第75423804步停LE4E9B6的66 4A。
依Intel DEC r16，以sub16更新低word、保留高16位及CF；
其餘算術旗標依16位元結果。驗收零下溢、8000有號溢位、
高位與CF保存，再正常原版重跑。

## 批次 120：word 暫存器 XOR（CONFORMED）

gap-119正常START第75515292步停LE4E8FA的66 33 C0。
Intel XOR r16,r16：本批增加mod3低word互斥或，高16位保留；
CF/OF清除，SF/ZF/PF依16位元結果，AF沿未定義清除策略。
既有dword來源不變，word記憶體尚不開放。
同一視窗LODSB／CMP AL與byte DEC已支援，不重做。
驗收相同暫存器歸零、符號位、高位及原版重跑。

## 批次 121：byte 暫存器即值 SUB（CONFORMED）

gap-120第75515302步停LE4E927的80 EC C1，同gap-119後續視窗。
Intel80 /5 mod3：來源reg8含AH，減imm8後只寫指定byte，
依sub8更新算術旗標。驗收AH減法、相鄰AL／高word保存及借位，再自然重跑。

## 批次 122：byte 交換與 word 單位移（CONFORMED）

gap-121第75576955步停LE4EAC4的86 C4，同視窗有66 D1 E0。
Intel86 /r mod3 XCHG r8,r8，先取兩個原byte再互換，旗標保存。
word D1 /0、/4、/5、/7以count1沿既有C1的ROL／SHL／SHR／SAR邏輯，
不讀imm8，保留高word；count1的CF／OF採Intel定義。
驗收AL/AH同暫存器交换、高word／旗標、word位移及指令長度，再自然重跑。

## 批次 123：累加器符號擴展（CONFORMED）

gap-122第76170037步停LE16D0D的98。
Intel CWDE將AX符號擴展至EAX；同族66 98 CBW將AL擴至AX、
EAX高16位保持。旗標不變，不開其他prefix。
驗收負值／正值邊界、兩種寬度及原版自然重跑。

## 批次 124：輪詢期間的排程輸入探針（CONFORMED）

gap-123已執行100000000步無unsupported，唯讀dialogue-trace最後2000步證實
原版在16CF3→10620與16D00嘴型等待迴圈。
既有FD2證據 docs/data/ida/fd2_ch29_input_cleanup_ida.txt
（相同固定EXE雜湊、IDA9.4 LE linear）已證10620直接比較041A／041C，
不應重新反組譯，也不能等AH10阻塞才提供所有輸入。
DOSBox本輪第一頁及Enter後第二頁皆實際可達；第一頁輔助圖另作完整像素比較。

平台BIOS佇列已具備共用BDA，缺件是探針只在AH10等待邊界排輸入。
新增明示-timed-keys「已完成指令數:按鍵」序列，在下一指令前送既有Enqueue；
嚴格遞增、有界、名稱合法、最多100筆，記錄實際送達步數。
未到時點不送、佇列滿拒絕，不直接更改EIP、遊戲欄位或呼叫consumer。
此為BIOS層輸入自動化，仍非8042／IRQ1／長按與實體wall-clock。
驗收正常START後第80000000步Enter，原版自行讀取並轉至第二頁。

批次124實測：第80000000步Enter由原版讀取，Reads由3至4，第二頁
父王對話整張64000像素與DOSBox輔助圖零差異。
七次排程Enter也都由原版讀取，至100000000步未遇unsupported。
為繼續走完開场而非反覆停在預算末端，探針明示上限改300000000；
外層180秒、2CPU／3GiB仍有界。這不是加速或改寫遊戲等待。


## 批次 125：完整對話等待邊界收據（CONFORMED）

沿批次124與 FD2 的 fd2_dialogue_cause_20260908.json：固定 EXE SHA-256、
IDA9.4 LE 0x16C57 進入等待、0x16CF3 輪詢輸入已證實。新增明示
-dialogue-enters N 與 -dialogue-frames 目錄，只在每次 16C57 呼叫第一次
到16CF3時擷取完整原生PNG，再用既有 BIOS Enqueue 送 Enter。每次呼叫只送
一次，到第N次後的下一等待頁停止；0..100有界，與其他輸入合計最多100。
不改遊戲記憶體、EIP、亂數或等待條件，不跳過任何指令；目錄必須預先存在、
檔案拒絕覆寫，保存 step、頁序、原生像素與PNG雜湊。所有per-binary位址
只留在 apps/fd2。原有未啟用此選項的探針行为保持。這是等待邊界 BIOS
輸入自動化，非實體鍵盤 wall-clock 或全機器同狀態聲明。
驗收從正常 START 重生多頁原版收據，與 remake 正常玩家頁面比對；
若遇尚缺 CPU/DOS能力，保存確切停點，不偽造下一頁。


## 批次 126：AND byte 的 SIB 位址（CONFORMED）

批次125由正常 START 自行擷取38頁後，145768680步停 LE136C7：
80 64 24 54 7F（AND byte [ESP+54h],7Fh）。現有80 /4已支援其他記憶體
形狀與邏輯旗標，但未接通用EA32的SIB。沿既有80 /1 OR、/6 XOR
通用來源／先寫後旗標契約增加/4，以原byte AND imm8，ESP預設SS。
不改旗標規則、不開新prefix，唯讀或越界拒絕且不發布新旗標。
驗收SS而非DS、相鄰byte／暫存器保存、零與符號旗標、唯讀拒絕，
完整cpu386回歸後由正常 START 重跑。來源為批次125固定EXE停點與現有
AND／EA32已驗證規格，不猜測遊戲高階玩法語意。

批次125／126驗收：CPU與machine完整套件通過。125自行擷取38頁後停缺指令，
126修正後以全新狀態重跑，自行擷取65頁，300000000步預算停止，無unsupported。
此預算停點不構成新CPU缺口。逐頁PNG雜湊已回讀核對，收據見
`docs/evidence/fd2-dialogue-prefix-20260908.json`。未啟用原版狀態注入。


## 批次127：開場交接的明示堆積容量（READY）

依據 `docs/evidence/fd2-opening-heap-capacity-20260908.json` 的同指令重現，
固定1 MiB近堆於519860940步無法提供153216 bytes的連續區塊。
live=887856，最大free=110240；原版未檢查空指標，後續REP MOVSD覆蓋程式碼。
這是容量／碎片限制，不是新CPU opcode缺口；不更改MOVSD或原版函式。

新增明示容量的安裝API，範圍1..64 MiB以內的非零bytes，保留原API的1 MiB
相容預設。FD2 bootprobe新增 `-heap-mib`，預設32、範圍1..64，收據記錄
實際bytes；32 MiB是本機驗證環境預算，不冒稱原版配置器或實體RAM大小一致。
近堆仍按需背書、不侵占既有DPMI區、不修改free／cdecl／失敗傳回0語意。
步數上限擴至20億、對話與總按鍵上限512，既有低預設步數不變，以支援
超過100頁的正常開場；外層容器仍需有界。

驗收：保留1 MiB重現、較大容量通過相同請求；容量邊界拒絕；machine回歸；
再由dosgolem自身普通START超越0x329D8交接，不得以DOSBox圖片替代。


## 批次128：SUB 累加器立即數（READY）

批次127的32 MiB正式bootprobe已越過舊記憶體破壞點，於519981966步
停在固定FD2.EXE的LE 0x32AA9，原始bytes `2D B0 0A 00 00`。
這是x86既有整數算術契約的SUB EAX,0xAB0；比照現有05 ADD與3D CMP的
立即數寬度，沿用sub32／sub16的CF/PF/AF/ZF/SF/OF運算，不重寫旗標公式。
66前綴只改AX並保留高16位；段覆寫與REP維持拒絕。測試包含遊戲立即數、
借位、帶符號溢位、零結果、16位高半保存與非算術旗標保存。
來源：fd2/work/ch01-town-parity-20260908/oracle-fixed-run/receipt.json，
同檔保留EXE SHA-256與dosgolem LE位址空間。CPU386與machine整套回歸後
再由正式bootprobe重生，不把支援一個指令宣稱成第一關已完成。


## 批次129：段暫存器寫入EBP區域變數（READY）

批次128正式收據已到117次對話輸入；629120845步停於固定FD2.EXE LE
0x3E2EA，bytes `8C 5D FC`，即MOV word [EBP-4],DS。
沿用8C既有段編碼與word寬度，新增mod=01/rm=EBP且無段覆寫的disp8
形狀，由decodeAddress32選SS，再由writeSegment16驗證描述子範圍。
有無66均只寫2 bytes；所有旗標、相鄰記憶體與來源段保持不變。
測試驗證非零SS基址、負disp8、兩種前綴寬度及越界拒絕。
來源：fd2/work/ch01-town-parity-20260908/oracle-sub-run/receipt.json；
EXE雜湊與dosgolem LE位址空間同收據。不得將這個printf路徑補件
宣稱成全段原版比較已完成。


## 批次130：ADD dword 記憶體立即數（READY）

2026-09-08 正式 dosgolem 收據 oracle-segment-run/receipt.json 在
629120877 步、固定 FD2.EXE LE 0x3E4CC 停於 `83 00 04`。
來源執行檔 SHA-256 為 222b7d067ad4450eb9c5f6e6bce1797d54bb050417ba39ced6067f8039f28c4f。
新增 83 /0 無 REP／段覆寫、32 位元記憶體形狀；沿用 decodeAddress32
與 add32，立即數符號擴充，記憶體存取沿用描述子邊界檢查。
寫入失敗時恢復旗標；測試正負立即數、溢位、DS 基址、SS 負位移及越界。
這是平台標準指令補件，不將 printf 消费端推測升格為遊戲規則。
CPU386／machine 全套通過後由正式 bootprobe 重生原版收據。


## 批次131：CS 覆寫的 byte 讀取（READY）

2026-09-08 oracle-add-run/receipt.json，固定 FD2.EXE 雜湊同批次130，
dosgolem LE 0x46AEE、629120913 步：`2E 8A 82 A4 6A 04 00`。
新增 CS: MOV r8,r/m8；不支援 register 來源的無效 CS 前綴，不新增寫入。
沿用 decodeAddress32，但讀取描述子明確選 CS。測試非零 CS base、
disp32、保留 EAX 高位、旗標與越界拒絕；其他未列入的 CS 指令維持拒絕。


## 批次132：ES byte 記憶體 CMP（READY）

oracle-cs-run/receipt.json 的 629120963 步停於 LE 0x3E15A，
`26 80 3B 00`，固定 EXE 雜湊同批次130。
將既有 80 /7 記憶體 CMP 延伸至 ES 覆寫，沿用 decodeAddress32、
readSegment8 與 sub8，不寫記憶體。測試非零 ES 基址、零值 ZF、
負差 CF／SF、描述子越界及原記憶體不變。未列入的前綴維持拒絕。


## 批次133：EBP 區域 word 載入段暫存器（READY）

oracle-es-run/receipt.json：629120983 步、固定 FD2.EXE LE 0x3E67B，
`8E 45 FC`。這是批次129保存後的 word 還原形狀。
8E mod01/rmEBP 無覆寫：decodeAddress32 以 SS 讀16位 selector，
沿用 canLoadSegment 驗證再發布目的段；有無66都讀2 bytes。
越界或無效selector不得改變目的段／旗標，測試包含這兩種拒絕。


## 批次134：ES byte MOVZX（READY）

oracle-segload-run/receipt.json：629121096 步、LE 0x3DF9E，
`26 0F B6 06`；固定 EXE 雜湊同批次130。
只開放 ES 覆寫的 0F B6 記憶體來源，目的為32位元，沿用地址解碼與
ES 描述子讀取；其他 ES 0F 一律拒絕。測試零擴充、ES非零基址、
旗標不變、越界拒絕及未知 extended opcode 拒絕。


## 批次135：SUB byte 暫存器減記憶體（READY）

本輪 oracle-live/receipt.json 經 START 與普通 right/down/down/enter，
709519202 步停於固定 FD2.EXE LE 0x4E188：`2A 0C 06`。
2A 的記憶體來源沿用 decodeAddress32／readSegment8，含 SIB；旗標由 sub8
計算，來源記憶體不變。測試實際 SIB、借位、DS 基址、上位元保存與越界。


## 批次136：CLC／STC（READY）

oracle-live-r2/receipt.json：709519213 步、LE 0x4E1A2，
`F8 C3 F9 C3`；原版選人 byte 減法之後的 carry 回傳。
依 x86 契約，F8 只清 CF、F9 只設 CF，其他旗標與所有暫存器不變。
不接受此最小支援以外的前綴；測試兩種初值與連續 clear/set。


## 批次137：SUB word 暫存器符號立即數（READY）

oracle-live-r3/receipt.json：709519504 步、LE 0x4E160，
`66 83 EF 07`。新增83 /5的16位元暫存器形狀；imm8符號擴充，
只改低16位，沿用sub16旗標。測試正負立即數、借位、帶符號溢位及高位保存。


## 批次138：FD2 保護模式 AH=3F 的完整 ECX 長度（READY）

2026-09-08 file-io.jsonl 的唯讀 INT21 觀察：固定 FD2.EXE LE
返回點0x3D9A4，FDSHAP.DAT兩次讀取 ECX=96768／143872，卻因現行
uint16截短只返回31232／12800。來源DS:EDX為1886696／1825102。
沒有DOS錯誤；DOSBox同段庭院有完整草地，而dosgolem依序缺塊／黑底。
固定EXE雜湊同批次130。這是FD2保護模式服務橋接，普通real-mode DOS不改。

FD2StartupDOS.readFile 使用完整ECX，成功以完整EAX回傳bytes read；
短讀仍回實際長度，EOF回0。拒絕大於64 MiB或超出DS可讀寫界限的目的區間，
拒絕必須在消耗檔案位置前；64 MiB是執行器資源界限，不宣稱原版實機上限。
其他AH與寫入寬度不因這個切片改動。測試兩個實際長度、讀尾及目的範圍拒絕；
全套machine／CPU回歸後重生同一117頁與首關底圖。

## 批次 139：移動後 81／83 記憶體減法（READY）

2026-09-08，修正大型讀取後從 START 正常選騎士、移動至 (5,18)，
dosgolem 在 LE 線性 0x17587 停於 `81 2C 24 E8 08 00 00`，
後續直接 bytes 包含 `83 6C 24 04 06`。來源為固定 FD2.EXE，
SHA-256 222b7d067ad4450eb9c5f6e6bce1797d54bb050417ba39ced6067f8039f28c4f；
收據 `fd2/work/ch01-town-parity-20260908/oracle-live-r4/receipt.json`。
只補 32 位元 `81 /5` 與 `83 /5` 記憶體目的端，沿用完整 ModRM／SIB
與 DS／SS 選擇、sub32 旗標，83 立即數有號延伸。拒絕前綴、越界與不可寫
不得改變記憶體及旗標。測試覆蓋 ESP、EBP、一般 DS、借位、溢位、負立即數及拒絕。

## 批次 140：移動框 81 /0 記憶體加法（READY）

同一固定 FD2.EXE 的 r5 普通 START→騎士移動在 LE 線性 0x17598
遇 `81 44 24 0C E8 08 00 00`，前一批 SUB 已越過。收據位於
fd2/work/ch01-town-parity-20260908/oracle-live-r5/receipt.json。
SHA-256 222b7d067ad4450eb9c5f6e6bce1797d54bb050417ba39ced6067f8039f28c4f。
補同一 81 的 /0 dword 記憶體加法，共用位址解碼與 add32，保持無效寫入原子性。
回歸覆蓋 ESP+disp8、進位、溢位與實際 2280 立即數；不擴大其他 opcode。

## 批次 141：指令選單 D3 暫存器位移（READY）

固定 FD2.EXE 的 r6 正常選騎士及移動後，在 LE 線性 0x1C2BA
遇 `D3 FA`（SAR EDX,CL），收據 oracle-live-r6/receipt.json。
SHA-256 222b7d067ad4450eb9c5f6e6bce1797d54bb050417ba39ced6067f8039f28c4f。
補 D3 暫存器目的端 /4、/5、/7 的 32 位元位移，count=CL&31；0 不改值／旗標，
1 依位移種類更新 OF，多位的未定義 OF 保留。禁止未具收據的記憶體／16 位元形式。
回歸覆蓋正負值、0／1／31／32／33 計數、CL 自身為目的端時的先讀後寫及 prefix 拒絕。

## 批次 142：攻擊動畫 byte 暫存器 AND／ADD（READY）

固定 FD2.EXE，r7 正常 START→騎士移動→選敵攻擊後，LE 線性 0x4DE85
停於 `22 C4 02 C2 F3 AA 0A FF`；即 AND AL,AH、ADD AL,DL。
SHA-256 222b7d067ad4450eb9c5f6e6bce1797d54bb050417ba39ced6067f8039f28c4f。
補 22／02 的 mod3 byte 暫存器運算，來源先讀再寫，保留目的高位與其他暫存器；
AND 使用邏輯旗標、ADD 使用 add8。未觀察記憶體及 prefix 形式仍拒絕。
驗收含 AH／AL 別名、借位以外的進位與溢位、零值以及來源不變。

## 批次 143：回合切換 word MUL（READY）

固定 FD2.EXE，r8 正常首攻結算→空地 END→YES 後，LE 線性 0x4E828
停於 `66 F7 E3`；MUL BX 的結果為 DX:AX，保留 EAX／EDX 高16位。
來源 SHA-256 222b7d067ad4450eb9c5f6e6bce1797d54bb050417ba39ced6067f8039f28c4f。
只補 F7 /4 mod3 的16位元形式；乘前讀完整来源，CF／OF 在高半部非0時設定，
其餘未定義旗標沿用既有 MUL 策略保留。回歸含0、65535²、來源DX別名與高位保存。

## 批次 144：回合切換 word 記憶體 ADD（READY）

固定 FD2.EXE，r9 正常 START→首攻→END／YES，LE 線性 0x1E5B5
停於 66 01 02（ADD word [EDX],AX）。來源 SHA-256
222b7d067ad4450eb9c5f6e6bce1797d54bb050417ba39ced6067f8039f28c4f。
只補 01 的 16 位元目的寬度，沿既有 EA32、DS／SS 與 add16 旗標；
暫存器形式保存高16位，記憶體形式只寫兩個位元組。寫入成功後才發布旗標，
唯讀／越界拒絕不改來源與目的。回歸實際形狀、SS、進位、溢位、相鄰位元組與拒絕。

## 批次 145：關檔後重用 DOS 檔案代號（READY）

固定 FD2.EXE SHA-256 `222b7d067ad4450eb9c5f6e6bce1797d54bb050417ba39ced6067f8039f28c4f`，IDA Pro 9.4 DOS LE 線性位址。
`sub_3D095` 在 `0x3D0A1` 呼叫 `__IOMode` (`0x46352`)，
其比較 handle 與 `dword_53790`，超出上限回 0，寫入 consumer 隨即回 -1，不送 INT21h。
普通 START 的只讀觀測在 step 172591123 取得 argument=150、limit=20、return=0x3D0A6；
同一原版暫存檔在攻擊返回後要求讀 207360 bytes，實際回 0。
舊 Table 每次 open 都增加代號、close 永不重用，超出原版固定模式表並造成寫入被拒。
原始定位、指令與未知項目保存在 FD2 本輪 `ida/io-mode.json`；執行紀錄為
`oracle-io-mode-live/file-io.jsonl`。這是已證實的寫入前阻塞，尚未宣稱黑塊完全修復。

READY：保留一般診斷用 NewTable 的不重用政策；新增明示重用模式，FD2 DOS
前端採最低可用代號，保留 0..4，不覆蓋仍開啟檔案。關閉後、再次配置前的舊代號仍無效。
不得修改原版的模式表上限、記憶體、程式碼或跳過寫入閘門。測試反覆開關、存活檔隔離、
空洞重用與原 API 相容；再普通 START 重生暫存檔寫入／首攻返回圖片。

## 批次 146：保護模式完整 ECX 寫入長度（READY）

批次 145 的正式建構器與零值前端均採代號重用後，719 項回歸通過。
`oracle-reuse-r2/file-io.jsonl` 自普通 START 觀察到 `FD2.TMP` 的 handle=6、
`__IOMode` limit=20，已實際越過原先寫入前閘門；`0x3D124` 的 INT21h
ECX=207360，但舊 writeFile 截成16位元，只寫10752 bytes並回成功。
IDA 9.4 `sub_3D095` 的原始指令與 caller `fwrite` 位於本輪 io-mode.json／io-writer.json，
仍使用固定 FD2.EXE 雜湊與 DOS LE 線性位址。

READY：保護模式 AH40 檔案及主控台寫入依完整 ECX／EAX 計數，
上限64MiB且先拒絕位址溢位；檔案寫入在完整來源可讀後才更動覆蓋檔。
ECX=0保留既有截斷語意；原版目錄仍唯讀，16位元 DOS 前端不變。
驗收207360-byte往返、65536-byte不得誤截斷、來源尾端越界不寫入、唯讀拒絕。
