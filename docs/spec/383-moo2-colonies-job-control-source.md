# 383：殖民地職業列的原熱區與輸入來源

狀態：**CONFORMED，限定靜態來源與既有210M收據核對**
日期：2026-10-04

## 範圍與來源

承接[382](382-moo2-colonies-upper-continuation.md)的Sol II正常畫面，只定位三列職業控制的原熱區、型別6輸入與暫存寫入。主庫RE-first保持；本文件不是玩法實作規格，不改Go玩法、CPU或原輸入。正式人口配置、讀寫檔與remake同狀態仍待驗。

官方1.31 ORION2.EXE SHA-256 `4e11be14217b4aafa1839f333bf5eba037f98b0c44e9e4752c96c464c260419f`；工具基線1929523731e5f1af2c1bbb446cdfd89e28401af4，原382 journal SHA-256 `cfffc3f5d60bd04fb3807f256cb4998da32a92cfa2669b3e328e36192d3ffacf`。IDA Pro9.4／IDA linear EA，原執行器另標dosgolem_high_le；原檔定位用file offset，三者不混用。

八份私有查詢沿固定EXE建立一次性IDA資料庫。共用繪圖sub_11B05A有1435項，只匯出前尾及型別6的比較樹切片，不追其他繪圖分支；compiler helper不深挖。完整身分、函式範圍、原運算元、xref及未截斷／截斷標記留在私有JSON。未因相似函式名稱或少量直接xref命名正式欄位。

## bytes與重定位證據

**已證實**：官方EXE內嵌MZ基址26654、LE標頭292E4，沿既有internal/machine/le_machine_test.go的固定來源入口。原兩object的relocation bases為10000h與170000h；365頁、51363筆internal 32-bit fixup由原file bytes逐筆核對。Go1.24.13的既有InspectLEInMZ只作typed讀取，Python3.11再核對原header／object／page／每筆record與套用結果，不修改loader。

4349筆匯出紀錄／3578個原EA全部核對，674筆紀錄含IDA已套fixup的運算元。每筆原file offset／file bytes與IDA relocated bytes另存moo2-383-source-byte-index.json，並列實際fixup record位置、raw record、target object與offset。例：IDA BF80E的`803DC8AA170000`，file offset1330786原值為`803DC8AA000000`；原target object2的170000h基址解釋差異。這是位址空間轉換，不是兩套不同原版指令。

首次驗證錯把IDA全部bytes當file bytes，被上述位置拒絕。新增獨立fixup核對後同來源通過；LE初版工具誤讀外層MZ的e_lfanew，沿已存在的26654入口修正，首失敗保留。沒有改原EXE、IDA定位或guest收據，沒有用正規化刪除差異。

## 原210M熱區與型別6

**已證實，原表**：DS188:298848 count36／bias0／stride55，完整1980bytes SHA-256 `5a6102f64d586c1978a37d8c5b034caf99783c44ba8ca5eb96896780c8b334db`。index1／2／3的矩形依次為310,62..518,92、310,92..518,122、310,122..518,152，原+8h word均6。+20h pointers依次2879DA／2879DC／2879DE；指標指向UI暫存，尚未讀得原值，不當職務數量。

**已證實，靜態**：既有sub_11CEF5從index1按37h stride遞增，signed矩形包含端點，第一個命中即離開，詳見[379](379-moo2-colonies-row-source.md)。330,77先命中1；330,92亦先1，330,93為2；330,122先2，330,123為3。16個端點／重疊／外側樣本以原36表核對。這些是來源模型，未送新滑鼠輸入，不能稱原選取1成功。

**已證實，靜態格式**：sub_115478的115560／66894218只把word存+18h，11557B／6689421C存+1Ch，11561C／66C740080600存kind6，115638／894220才把pointer存+20h。原+18h word310／+1Ch word510，不把+18h的整個dword280136當pointer；高word40保持raw，不猜名稱。不同widget kind不能共享pointer欄位解釋。

**已證實，靜態輸入**：sub_11B05A的11B0AC／11B0B3比較kind6，11B0B8 JBE到11C2C2；原狀態值為1才於11C2CF呼叫sub_1156E2。這是原比較樹，首次switch查詢為空不表示沒有分派。sub_1156E2為1156E2..115994／211項；+2Ch word0走水平座標，原X與偏移求比例，最後受+18h／+1Ch上下界限制。115982讀+20h pointer，115988／668902間接寫word。對本三列與明示bias0，8個水平邊界模型核對得到310..510；原暫存讀寫與按住／放開的實際時序尚未取樣。

## 場景與正式人口的界線

**已證實，靜態**：sub_C058A的C07D2傳sub_BED21、C07E1呼叫sub_1191CA；1191EA將callback寫dword_1A8840並啟用word_17C48C，sub_1192D1在1192F3間接呼叫它。型別6的持按主輸入分支在11E1F1／11E33B／11E508使用此回呼。原210M的場景callback值尚未讀取，執行本分支仍是強推論，不能只由靜態設定宣稱已觸發。

C058A在C086E呼叫sub_C02F9分派場景操作。三列控制由sub_BCB07／BCB4B／BCBA0／BCBE6取址1979D4／1979DA／1979E0，再經B4EF6及115478建立欄位；直接xref少不等於沒有間接寫入。sub_BF627遍歷三列並依word_1979E0及byte_17AABB分支，連到B4EF6的mode3／4與sub_B9C3D／B9E94。**強推論**：這是選取人口與後續放置的來源鏈；原mode、暫存值、實際callback及人口變更未驗，不先訂點選或拖曳的正常操作契約。

**已證實，最小分類**：sub_BC928只歸零word_17AAB9；sub_B9CE3是population packed值及共用分類比較，沒有正式人口store；sub_BB1FA重設游標相關狀態。三者不命名人口job writer。sub_B9C3D於B9CAF對原pool+169h×colony+4×slot+0Dh作AND FDh；B9E94有相對的bitmap及record寫入，也包含殖民者轉移分支。保留原bits／運算元與caller，只當正式人口觀察候選，不把整函式外推成當前同殖民地換職成功；不深挖無關轉移分支。

## 驗收與下一閘門

獨立入口：workplace/new-game-383-source-verify.py。原8份IDA schema／5365函式、每筆file offset、原與重定位bytes、完整210M核心／FPU／clock／callback IRQ、36表及PNG通過；16熱區邊界與8水平模型只證明來源推導。公開internal／CPU／DOS／原probe保持1929523，383沒有guest重跑、沒有新輸入、沒有cap變更。382正文保留並追加回鏈，索引同步。

IDA沿locked-v1 image、120s／2GiB／2CPU／128pids／UID1000／network none，patch唯讀、tmp DB一次性、輸出既有workplace。Go與Python沿既有Go1.24.13 image，資料唯讀，private輸出UID1000。原EXE／JSON／LOG／bytes index／PNG／state留本機忽略目錄，公開只保存自撰來源文件與回鏈。

下一步先建立384只讀觀察契約，維持同輸入與210M；補讀kind6的三個原pointer值、原current colony／pool、完整361byte record、相關UI暫存與場景callback。以原210M完整核心／畫面作守衛，不送新輸入，不延長預算，不預填正式job語意。取得可回播前置後才訂一次正常人口選取／放置契約。固定日期不是RNG seed，人口操作、正式存讀、完整開局與remake同狀態未知。

## 384 原前置快照回填

[384](384-moo2-colonies-job-source-snapshot.md)沿同輸入及210M完成只讀來源。DS188:2879DA／DC／DE原pointer words均310；current raw4、pool5B2044、361byte record5B25E8已取得，未命名正式job語意。原scene callback2A8840=0而enable26C48C=1均可讀；靜態C07D2／C07E1設置鏈不代表本次已觸發或已進入穩定輸入。完整382終態／journal／PNG／state保持，人口操作未驗。下一步核對C07C1→sub_BF456返回與callback設置時序，不直接由職業列熱區送輸入。

## 385 回呼runtime地址勘誤

見[385實際結果](385-moo2-colonies-scene-ready.md)。原IDA linear EA 1A8840加F0000h後為dosgolem_high_le／DS188:298840；原runtime 2091EA的A340882900亦直接指定298840。384觀察器誤用2A8840，偏差10000h。舊收據在2A8840讀到四bytes零的事實保持，但「scene callback為0」的欄位語意撤回；正確298840讀值與實際store全RAM差異尚未取得，不當CPU缺陷。

385原guest僅一次；原BF456返回203219421、原callback A3寫入203219451、setup返回203219467、首正常輸入CALL205804505已觀察。首次輸入PNG與原210M相同。先前仍需等待場景設置的推論被此次原指令與時序否定。current／pool／361byte record、三pointer、enable／水平偏移原定位保持。本節追加訂正，舊正文與收據不改；385回呼觀察契約回到DRAFT，下一輪先修正只讀地址，不送人口輸入。

## 386 正確回呼讀值回填

不可變定位為官方1.31 ORION2.EXE／IDA linear EA dword_1A8840、1191EA／A340881A00；dosgolem_high_le runtime2091EA／A340882900及DS188:298840。見[386限定驗收](386-moo2-colonies-callback-read.md)。386沿同輸入與原210M取得原203219451寫入前55451700、寫入後21ED1A00／raw1AED21，全RAM僅實際四bytes改變已驗；首正常輸入205804505的完整原385前置守衛通過，第9只讀窗298840仍21ED1A00。

正確callback值與寫入範圍由未知／強推論升為**已證實，限定此次原A3及首輸入**。這不是原384在210M已取過正確值，也不外推全部callback生命週期。舊2A8840零raw維持未分類，原錯誤385觀察器與其DRAFT狀態保留；沒有改舊正文／Go／收據，也不當CPU缺陷。人口正常操作／正式存讀／remake同狀態仍未知。

## 387 職業列座標來源回填

見[387條件座標與持按／放開來源](387-moo2-colonies-input-coordinates.md)。不可變定位為官方1.31／IDA linear EA sub_1236D1的123729／12372D／12372F與123738、1237F2／CB；runtime callback8:2136D1及DS188:2A3A38／2A3A36。原回呼只有在word_17C51A及+2皆0時，將ECX低word作signed SAR1成GUI X，Y保存EDX低word；初始化range為2×(width−1)及height−1。原2:1列表來源未被推翻，這輪補齊其旗標及range條件，不改原列表輸入／收據。

持按實際選取器是sub_113FB9，與原事件來源的11DC掃描分開；原11E160再次AX3，為0後到11E4EB，kind6在11E508呼叫1192D1並於11E50D清共享選取。條件控制流已證實，人口操作是否觸發仍未知；不能只用原pressed poll當作職業列已消費。386完整首輸入沒有取得本輪新旗標／座標／range，下一步388先補只讀前置，不送人口輸入。

## 388 原首輸入動態前置回填

見[388只讀raw及裝置範圍](388-moo2-colonies-mouse-source.md)。不可變鍵為官方1.31／IDA linear EA word_17C51A／17C51C、17C534／17C538、1B3A38／1B3A36、1B121A及17C4E4；dosgolem_high_le投影見388原8窗。原205804505的8窗與裝置range已由388只讀補驗，完整前後狀態及既有210M結果保持。

較早正文只描述當時收據，保留其未知邊界；當次205804505的上述raw前置由388補驗。原持按／放開真正消費及人口變更仍未知。下一步389固定本次完整首輸入、原旗標及range，建立一次正常職業列press觀察契約；候選裝置660,77按原signed SAR1為GUI330,77，原36表先命中kind6 index1。先追原持按選取與1192D1／場景回呼，依實際消費點安全release；不把候選命中當人口變更或預設職業語意。

## 389 正常職業列輸入回填

見[389正常press與release](389-moo2-colonies-pop-press.md)。不可變鍵為官方1.31／IDA linear EA 11E1A7→113FB9、1192F3／1A8840、BED21、11E508／11E50D；dosgolem_high_le投影分開見389時序。原205804505完整388前置後，正常660,77按下，原選取器返回1，場景依真SS返回後安全放開；零按鍵、kind6場景回呼與共享active清除已驗。原8-byte record差異只在206658147之後到210M形成，正式職務與放置語意未驗。較早未知是當時收據邊界，389補驗限定此正常輸入；不改舊正文或receipt。

下一步390沿同輸入及210M，追查206658147之後原C086E→C02F9與B9C3D／B9E94的最小正式寫入鏈，定位這8個record差異與可放置狀態；取得證據才訂一次跨職業列放置。不假設8,000k→4,000k已完成換職或刪除人口，不盲增cap或深挖renderer／平台helper。

## 390 原選取寫入回填

見[390限定原寫入鏈](390-moo2-colonies-pop-writes.md)。不可變鍵為官方1.31／IDA linear EA C086E→C02F9／BF627／B9C3D／B9CAF，runtime及file offset各自見390索引。修正版原210M／step_limit保持389全部15事件、完整日誌與DAC journal、末態EIP1A5042／482658319µs及原PNG。206659890到C086E、206659891到C02F9、206660054到C0337、206660055到BF627。206697288原BF681將17AABB由0設1；206697307原B9C81把17A974由FFFF設4。原B9CAF於206697426／545／664／783依序將record+0Dh／11h／15h／19h的bit1清除，02→00。BF6ED及B9E94未到達，限定此次走選取分支。

69個實際Step變更重建四個監看範圍終值；8個首末record差異均定位。+E7h的word由原DE727在206700878寫0；+C8h先由原E19C6在206704020寫FE70h，再由E1CD9在206704659加到FEB9h，後續重算保持FEB9h；+0Bh由E1E64在206704718將FF改02。另有+EFh／F2h／FCh／104h等先清除再重建的中間值，首末比較不會顯示，均保留實際byte變更。record+0Ah原08保持；原+0B／C8／E7正式名稱與職務數量仍未定型。

391先捕捉選取後下一個原輸入點1B0845，核對原17AABB=1、17A974=4及第二列signed熱區；條件成立才以正常裝置660,107一次按下及安全放開，驗證BF6ED→B9E94與四槽是否恢復。不得直接改bit／派送ID，不把210M中途renderer末態當可按輸入點。 原較早正文與receipt保持，不將局部選取升格為配置完成。

## 391 選取後 kind7 來源回填

見[391 kind7與放開來源](391-moo2-colonies-pop-place.md)。不可變鍵為官方1.31／IDA linear EA 11CEF5的11E1EC／11E334／11E503、11E582／11E69D及123C1B／1B1222。原390的210M控件表已成37列，前三職業列kind7，第二列signed矩形310,90..510,118；原SAR1下裝置660,107仍first-hit2。kind7不呼叫kind6 held場景，不能沿用等待held場景返回的release守衛。123C1B讀cached word_1B1222，須與當次新裝置／座標／callback完成分開核對；既有snapshot的calls為s.calls啟動服務計數，不能當AX3 poll次數。舊36表kind6仍是205804505原首輸入階段，不改其receipt或歷史正文。391沒有新guest或輸入，正式放置未驗。

下一步392保留原390至210M後，有界捕捉原1B0845及當次37表／kind7，正常660,107 press；觀察213C1B依真SS返回20DB8C的低AX1及新座標、callback完成、安全IRQ與至少20ms條件，再release。實際selector／BF6ED→B9E94及record／原畫面另驗，不使用calls增加或kind6場景回呼作放開閘門。
