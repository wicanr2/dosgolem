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
