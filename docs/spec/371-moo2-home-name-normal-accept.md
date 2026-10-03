# 371：母星Sol的正常ACCEPT確認

狀態：**CONFORMED，限定正常母星輸入與新CPU拒絕**
日期：2026-10-04

## 原來源與範圍

沿[370](370-moo2-home-name-source.md)已證實170M命名前置。官方1.31 ORION2.EXE SHA-256 4e11be14217b4aafa1839f333bf5eba037f98b0c44e9e4752c96c464c260419f，工具46ae96f788d4242da161d833add26cdedd9f4c32，位址空間dosgolem_high_le。原370收據0c650a8610291fb3d5c141fce339f8c24850a7dd0c70f0b173febb566116fed2；候選DS188:298848 index2+24→28439D的32bytes為Sol補零，SHA-256 6919c6ec1f4751149ac2653bf5decc4881f64d5b997fb2cac85cf8f4c175f30d。index1+24→261AC2只保留原16byte窗口，不當ACCEPT標籤。

只允許隔離fixture保留預設Sol，一次正常母星ACCEPT與受控release，沿相同可寫180M、1996日期及全部既有正常輸入。公開CPU／DOS／provider／probe、主庫玩法RE-first保持。正式名稱writer／保存後讀取／完整開局／RNG與remake同狀態未驗。

## DRAFT前置與輸入

DOSGOLEM_MOO2_HOME_NAME_ACCEPT_CLICK=1只接受值1與完整可寫bannerProfile、game-dir、universe180／maxSteps180000000及非空save-state路徑，現有空state閘門保持；無效值／缺依賴在讀EXE前拒絕。關閉模式保留370兩只讀取樣，不新增輸入。

固定170000000在370原取樣之後核對完整核心：EIP235948／R=[0 18 0 0 2BD3C4 2BD3F8 2A206E 34F062]／段=[8 188 188 0 20 188]／flags293h／IF1，FPU127F／status0／depth0／八stack bits0，虛擬386324835µs，VBE bank9／startY512／sets2793／writes51487286／display93。完整globals／header／165byte表／32byte候選／index1原窗口／RGB按370雜湊核對；target8:2136D1、mask2B／pending0／inactive／callback12／12、IRQ44492／44492 inactive／notfailed。不可讀／不符即停，不選其他時點或放寬原欄位。

index1原矩形227,246–324,273；275,260與276,260唯一命中，畫面ACCEPT親看，按鈕consumer語意仍待正常輸入驗證。一次InjectMouseEvent x550,y260,buttons1，沿既有x÷2平台契約，delta0/0，不代寫核心／候選／RAM。待原正常INT33 AX3返回pressed BX1／CX550／DX260且callback完成至少一次、虛擬至少20000µs後，首次IF1／target相同、pending0／inactive、IRQ inactive／notfailed且started=completed、mask2B或1可送放開x552,y260,buttons0。不skip指令、重送或改cap。callback未返回或原pressed未出現時保持觀察並明示未release，不猜補。

## 原consumer與終點觀察

只讀記錄首個原INT33 AX3 pressed返回，保持原callsite與完整R／段／flags，定位不預定24C31B。homePressed後原20DDDB共享store若命中，保存首筆code16、DS188:26C4A6 word與32byte候選前後；未知正式名稱writer不由共享store推定。原214104 RET在pressed poll後最多三筆，保存真實SS:ESP stack16、完整R／段／flags／FPU bits與原Step error，獨立按原stack核算返回與ESP+4，不預定caller或AX。store／RET未命中就明示未知，不延伸追helper。

370原170M前綴與38PNG保持，新增按下前置與observer分開。啟用模式180M不套370 count3候選guard，改沿既有終態取實際count≤64完整表、VBE／RGB／PNG與核心／callback／IRQ。原guest一次，原CPU停止／失敗／新玩家畫面／cap按真實記錄，exit0不代表完成。state副本／終態bytes／SHA-256／UID GID核對，原418來源前後保持；沒有新CPU碼，沿369固定EXE全套收據，另建置本隔離probe。

## 閘門、入口與權利

READY前只審查原收據與可丟棄設計，未知正常點擊結果不得寫入測試真值。自製規格同次掛000-index；私有workplace/moo2-home-name-accept-371.go／new-game-371-run.sh／new-game-371-verify.py入口，收據連主庫既有docs/re/dosgolem-moo2-intake-20260930.md。原LOG／PNG／RAM／state不入Git。

沿Go1.24.13 image sha256:1a6d4452c65dea36aac2e2d606b01b4a029ec90cc1ae53890540ce6173ea77ac；600s／2GiB／2CPU／128pids／UID1000／network none，原ZIP／patch唯讀，背景state監測由run有界擁有及trap清理。主庫玩法未改；正常母星名稱確認與後續UI以實測限定，不稱整段開局或完整parity。

## READY審查

new-game-371-ready-review.py直接核對370原170M核心／FPU／clock／VBE／完整表候選與原target／callback／IRQ，唯一熱區及原裝置x÷2映射吻合。READY只授權一次隔離正常輸入與有界consumer觀察，不預定caller、store或後續畫面。原GUI結果及持久writer待native，主庫玩法保持。

## CONFORMED限定結果

狀態：**CONFORMED，限定正常母星ACCEPT／共享選取store／星圖與新CPU拒絕**。完整開局、正式名稱writer及存讀語意仍未驗。

已證實170000000完整370前置通過，一次press／386324835µs／x550,y260／buttons1，callback12／12與IRQ44492／44492已返回。原170015047、dosgolem_high_le:24C31B正常INT33 AX3查詢返回BX1／CX550／DX260，完整段與flags12h保持。170015082／386372270µs首次符合安全條件release x552,y260／buttons0，持按47435µs；mask1、IF1、pending0／inactive、callback13／13與IRQ44497／44497已返回。沒有重送、代寫核心或RAM。

原170024038在214104 C3依真實SS188:ESP2BD364 stack返回20DB5B，ESP+4、AX0及flags246h保持；沒有把它寫成AX1。170024419原20DDDB六byte66A3A6C42600真正寫DS188:26C4A6 word0000→0100，下一EIP20DDE1，完整R=[1 1 37 8 2BD368 2BD3D0 2843A1 28439D]／六段／flags297h／FPU保持，32byteSol候選保持，callback14／14與IRQ44498／44498非活動／非failed。已驗正常點擊選取index1，正式星名持久writer仍未知。

後續兩個有界原RET樣本171940309返回20DB5B、172067022返回174742，均按真實stack核算ESP+4，AX0／flags246h／段／FPU及候選保持。這些樣本只記原返回，caller語意不推定。正常consumer共首store一筆與poll後RET三筆，皆只讀快照、error nil。

終圖已親看母星命名視窗消失、正常星圖顯示Sol與3500.0。來源表／候選→正常press／poll／release→共享選取store→後續星圖的垂直鏈已驗；不是正式存檔讀取或完整開局對拍。

### 新CPU與觀察界限

原174213914在dosgolem_high_le:1749C0 bytesF6 EC A2 06 1F 28 00拒絕F6 ModRM EC。拒絕後EIP1749C2，R=[FF01 1A5 2 8 2BD488 2BD4E0 171C80 2BD4E0]、段=[8 188 188 0 20 188]、flags246h，FPU127F／status0／depth0／八stack bits0。原CPU只fetch opcode與ModRM，尚未執行IMUL結果。依[Intel80386 IMUL](https://www.ardent-tool.com/CPU/docs/Intel/386/manuals/prref386/IMUL.htm)，F6 /5為有號byte隱含AL乘法，EC為AH來源；ISA辨識已證實，原結果待補工具後native，未在本輪猜補。CPU保持，probe exit0仍含guest_cpu_stop／step_error，沒有step_limit或dos_exit，不稱180M或完整開局。

終態header count23／stride55，既有new_game_menu_table_snapshot限制count≤16，故table_readable=false／records空。這是觀察器界限，不是已證實資產越界或表格讀取失敗；未擷取完整23物件，不放寬舊guard。VBE bank9／startY512／sets2837／writes52762744／display93，原虛擬397790869µs；正常輸入terminal pressed／released／polled／store_seen true、三returns、callback14／14／pending0／inactive。

### 驗證

370額外點擊前11435共通原列按既有mtime／DTA及每輪只讀RAMhash正規化保持，38PNG逐byte保持，完整170M來源／核心／候選／表／RGB／clock與callback IRQ精確對接。六私有patch逆轉為370；全部公開internal／CPU／DOS／probe保持46ae96f。六CLI拒絕及合法開啟／關閉缺EXE正對照通過。原guest一次，沒有重啟或挑結果；原418來源前後保持。state副本／終態與370一致，SAVE10.GAM208000bytes、MOX.SET553bytes、sound.lbx4250888bytes，三檔UID GID1000，監測與guest均terminal，容器清理。沒有新CPU碼，沿369固定EXE的全套收據，另建置本隔離probe。

## 跨規格回填與下一步

| 不可變鍵 | 語意與等級 | 舊規格 | 回填 |
|---|---|---|---|
| 官方1.31／dosgolem_high_le／DS188:298848 index2+24→28439D／170M | 母星Sol候選到正常ACCEPT後星圖，已證實 | 369、370 | 本371正常press／poll／release已驗，候選不等於持久欄位 |
| 同映像／dosgolem_high_le:20DDDB／66A3A6C42600／DS188:26C4A6 | 正常母星確認共享word0000→0100，已證實 | 367 | 新母星上下文補回填；367統治者確認與正式名稱writer未知保持 |
| 同映像／dosgolem_high_le:1749C0／F6EC | 新byte IMUL能力拒絕，已證實拒絕；原指令結果未知 | 本規格 | 下一獨立READY CPU切片，不改本輪CPU |

369／370／367同次追加回填。338／339／340／341／365／366／368既有其它點擊上下文與唯讀原收據不受本母星輸入影響，不把新的store當舊same-state。私有驗證檢查不可變定位與三份回填，缺項即失敗。

公開本規格／索引及三份回填，私有probe／LOG／PNG／RAM／state不入Git，收據掛主庫既有研究入口。下一步以本174213914／1749C0 F6EC及完整核心、ISA契約建立byte IMUL READY規格、補CPU獨立測試與原正常consumer，再沿相同輸入續行，維持180M。不深挖renderer／runtime helper；23物件全表需取時另以有界只讀入口保存。主庫玩法RE-first保持，正式名稱與旗色持久語意／讀檔／完整開局／seed與remake同狀態未驗。
