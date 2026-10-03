# 372：隱含AL的byte IMUL來源

狀態：**CONFORMED，限定工具CPU與正常續行**
日期：2026-10-04

## 原阻塞與來源

[371](371-moo2-home-name-normal-accept.md)正常母星Sol確認後，原174213914在dosgolem_high_le:1749C0 bytes F6 EC A2 06 1F 28 00 E9 33 F3 FF FF拒絕F6 /5。官方1.31 ORION2.EXE SHA-256 4e11be14217b4aafa1839f333bf5eba037f98b0c44e9e4752c96c464c260419f；工具c39af543efa47387b1fd96f86038d08f940f42c6，CPU a39e5b9f74e026fc1c2514d733808e94920b6789d1b68ef8eb8960dc72df5350。原R=[FF01 1A5 2 8 2BD488 2BD4E0 171C80 2BD4E0]、段=[8 188 188 0 20 188]、flags246h，FPU127F／status0／depth0／八stack bits0。拒絕只取opcode／ModRM，after EIP1749C2；不代表乘法已執行。

[Intel 80386 IMUL](https://www.ardent-tool.com/CPU/docs/Intel/386/manuals/prref386/IMUL.htm)定義F6 /5將AL與r/m8有號相乘，16位結果放AX。結果不能以低byte符號延伸表示時CF／OF置1，其餘置0；[附錄C](https://www.ardent-tool.com/CPU/docs/Intel/386/manuals/prref386/appc.htm)標SF／ZF／AF／PF未定義。本工具沿既有乘法保留未定義旗標，不稱硬體逐值一致。EC為AH來源，舊AL01、AHFF，即1乘負1；ISA預期AXFFFF、CF／OF0，原native結果尚未取得。

## 預定契約與READY閘門

只新增裸F6 /5的八register byte別名與memory來源，沿reg8、decodeAddress32及readSegment8；支援32位ModRM／SIB、DS／SS、signed位移、段末與繞回。先捕捉AL及來源再發布AX，涵蓋AH與AL重疊。只改EAX低16及CF／OF，EAX高16、其餘R／段／FPU／RAM保持。memory來源可唯讀，乘法不呼叫任何memory寫入。

讀取、位址解碼或前綴拒絕保持R／段／flags／FPU／RAM；EIP可按既有工具模型消耗bytes，不稱硬體exception rollback。66／segment／67／F0／F2／F3仍拒絕，不擴張其他F6 group或既有prefix範圍。核心新增分支逆轉後必須逐byte等於c39af54的CPU。

工程oracle以有號範圍及重複加法建立全部256×256產品，以little-endian視圖發布AX，以[-128,127]判斷CF／OF。八register來源×全部byte pair×兩flags初態、memory全部pair、ModRM／SIB及DS SS、wrap／段末、讀拒絕／未知段／越界、每byte截短及prefix拒絕均比較完整核心與RAM，FPU用既有非零fixture。工程原consumer以明示合成記憶體驗F6EC→A2→E9，不冒充原native。

審查原371原列、固定CPU hash與Intel契約後轉READY，才改CPU／測試。窄測及固定官方EXE乾淨Go全套通過後，只跑一次相同371正常輸入，180M cap、1996日期、可寫state與原418檔RO保持。不改GUI／輸入／核心或RAM，不重送／挑結果。固定日期不是RNG seed，主庫玩法RE-first維持。

## 原版有界consumer與驗收

私有probe只增加首個1749C0起三步observer，保存完整前後R／段／flags／FPU bits、code16／stack16及DS188:281F06的byte。原IMUL應到1749C2；原A2應從1749C2到1749C7並按原AL寫281F06；原E9從1749C7到173CFF由原disp獨立核算。每筆observer只讀，首末步RAM保持；A2可改唯一目標byte，變化由全RAM比較保存。原舊byte由同native取得，不預填。

與371比較拒絕前共通原列及38PNG，只排新增observer及舊拒絕終態；沿既有mtime／DTA／每輪RAM hash正規化契約。完整開局、正式存讀語意、typed名稱／旗色、其他CPU支援與remake同狀態未驗。新CPU拒絕或cap均如實保存，probe exit0不算guest成功。

## 工具、入口與權利

Go1.24.13 image sha256:1a6d4452c65dea36aac2e2d606b01b4a029ec90cc1ae53890540ce6173ea77ac，Docker network none／600s／2GiB／2CPU／128pids／UID1000，原ZIP與patch唯讀。公開自製CPU、獨立測試、本規格、000-index與371回填；原LOG／PNG／RAM／state及probe保持私有。

既有workplace/入口依序為new-game-372-ready-review.py、new-game-372-full-run.sh、new-game-372-run.sh、new-game-372-verify.py。主庫紀錄沿docs/re/dosgolem-moo2-intake-20260930.md，不新增交付目錄。

## READY審查

原371拒絕bytes、174213914完整核心／FPU與CPU hash直接核對。獨立核算AL01乘AH負1為AXFFFF、CF／OF0；原E9 signed disp返回173CFF。Intel80386指令與附錄C已核對，undefined flags保留僅工具模型。new-game-372-ready-review.py通過，規格轉READY後才開始工具CPU與自製測試；原native結果仍未知。

## 工程回歸判準修正

窄測通過。首次乾淨Go全套只有289舊NEG拒絕fixture把F6E8當未知group而失敗；新READY支援該IMUL AL，移除唯一過期拒絕樣本，原F6 /1、/2、/7及memory NEG護欄保持。372全byte正測接續其合法形狀，回填289後用相同容器及命令乾淨重跑。這是測試判準更新，未重新執行原guest。首次失敗留workplace/full-test-372-first-failure.txt。

## CONFORMED限定結果

狀態：**CONFORMED，限定裸F6 /5工具能力、原IMUL／A2／E9與相同正常輸入180M續行**。完整開局、正式存讀語意及remake同狀態未驗。

已證實原174213914、dosgolem_high_le:1749C0 F6EC用舊AL01與AHFF得到AXFFFF，EIP1749C2，R只有EAX低16改變，CF／OF0、工具flags246h保持。SF／ZF／AF／PF保留僅既有未定義旗標模型。原174213915在1749C2執行A2 06 1F 28 00，DS188:281F06真實舊byte01→FF，EIP1749C7；全RAM比較只有此一byte改變，完整R／段／flags／FPU保持。原174213916在1749C7執行E9 33 F3 FF FF，signed disp按原bytes核算返回173CFF，完整核心／RAM保持。三observer只讀、error nil，callback14／14與IRQ45735／45735完成且inactive，首末步全RAM保持。目標用途不因byte寫入而命名；工程fixture的舊55另標合成，原01由同native取得。

只新增F6 /5分支，逆轉後逐byte等於c39af54的CPU；CPU SHA-256 1d8a4d8252372c97d8e873ba13c3ab3670796527dcd6d74de52e8cbf226e068c。窄測0.212s通過，八來源×全部byte pair×兩flags初態共1048576組register fixtures，AL重疊按真正舊值核算；memory131072組與唯讀、全ModRM／SIB、DS SS、signed位移／wrap／段末及失敗發布皆通過。獨立oracle用重複加法、[-128,127]範圍與little-endian lane，完整nonzero FPU及RAM／禁止memory寫入檢查。新窄測SHA-256 08e22ec33fba14abed76eb3f4c86f07ebd1e9cfcc54d66dd9103a94c15ad2dc4；289拒絕測試只移除F6E8過期樣本，SHA-256 6248ee23037b32db47c8d68baec8bf84731886cad7d95102c3540123e68fd042，其餘公開internal與probe保持c39af54。

固定官方EXE乾淨Go全套通過，CPU38657.925s／machine1.699s；首次只有289過期拒絕判準，失敗收據保留，修正後相同容器與命令重跑。未掛8088外部資料，不外推該驗收。全套後測試檔只把兩處中文與／舊字改為繁體，識別字及斷言保持；窄測與full覆蓋同實作。原guest只跑一次，三私有區段逆轉後等於371，輸入、日期、cap保持。371拒絕前11645共通原列按既有mtime／DTA及每輪RAMhash正規化保持，38PNG逐byte保持；本輪39PNG。私有驗證首次及增加cap／完整表檢查皆通過，未重啟或挑結果。

實際step_limit=180000000／EIP2176C5／unique_sites54239，無guest_cpu_stop／step_error／dos_exit。180M虛擬414027099µs，R=[35017C 70 39C17C 70 2BD478 2BD4C0 39C17C 35017C]、段=[8 188 188 0 20 188]、flags202h、FPU127F／status0／depth0／八stack bits0，IF1；callback14／14與IRQ47425／47425完成且inactive。VBE bank9／startY512／sets2927／writes55120744／display93。終圖已親看正常星圖、Sol／3500.0、底部COLONIES／PLANETS／FLEETS／ZOOM／LEADERS／RACES／INFO與TURN；命名視窗已消失，這些控制尚未另送輸入驗收。

既有dumpSetupTable入口在cap保存23物件／stride55／1265bytes，DS188:298848，table SHA-256 4392f446efbdd96acafbfba8ee39cc0e8ac67a2388119df5bfbfef14a89ab15a、globals 6ff76fc0f447a300d6468cb76bc2acc884bd9b0e544c01f0ad101d76ee6d1dfe、header 35d7cde9f525f64e4d64ea3bdaa7bf3ee2770440c7ecb8cbe2f3fc954aff8cda；只讀且完整前後核心／RAM保持。沒有放寬舊count≤16 observer，後者仍table_readable=false；完整表由另一既有count≤64入口真正取得，不把舊觀察器拒絕當產品缺陷。終PNG SHA-256 beb773bf0623f56bc9b2be697c6e0e472abebd8fa4c319f15e6074291bb7a832，RGB 9b433167360cbfa77c0422b5ba6848db3efba59cd7d168738e0f62d7dd03200f。

SAVE10.GAM208000bytes／0cf71fc5368861758da5a5dffbd791f8b711a48cfd295572eb9a21d27dea0f4d、MOX.SET553bytes／de8554b3284c9f9065efe6e4f2dfa71ab0c7e732e75eaea0d0ecf465ab86409f、sound.lbx4250888bytes保持；有界同guest副本與終態及371一致，UID／GID1000，原418來源前後保持。六CLI拒絕及合法mode on／off缺EXE正對照保持。原418來源唯讀、state目錄可寫，程序已終止且相關容器清理。

## 跨規格回填與下一步

| 不可變鍵 | 語意與等級 | 受影響規格 | 回填 |
|---|---|---|---|
| 官方1.31／dosgolem_high_le:1749C0／F6EC／174213914 | 原IMUL→A2單byte store→E9返回已證實；未定義flags僅工具模型 | 367、369、370、371 | 新CPU拒絕已解，保留原371失敗歷史與其它點擊上下文 |
| 同映像／DS188:298848／count23／stride55／180M | 既有dumpSetupTable取得完整1265bytes只讀表，已證實 | 371 | 解出缺少完整表的觀察限制，不放寬舊guard |
| 裸F6 /5 register與memory來源／F6E8 | 工具新增支援，已證實工程覆蓋 | 289 | 移除唯一過期拒絕fixture，NEG邊界保持 |

367／369／370／371同次追加372回填，289補工程拒絕契約。其它CPU／輸入規格與唯讀source不由本輪外推。完整native收據及CPU逆轉／其它public檔保持／所有回填由workplace/new-game-372-verify.py檢查；主庫沿既有研究入口保存收據SHA-256。

下一步以本180M正常星圖與實際23物件，核對首個COLONIES控制的來源、矩形、callback與安全輸入前置；先做有界只讀觀察，再依READY送正常裝置輸入。不直接注入核心或RAM，不為renderer／runtime helper深挖。主庫玩法RE-first保持；正式存讀語意、typed名稱／旗色持久writer、完整開局與remake同狀態未驗。1996固定日期不是seed。
