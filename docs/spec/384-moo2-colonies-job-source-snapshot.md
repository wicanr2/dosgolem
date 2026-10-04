# 384：同210M殖民地控制前置的只讀快照

狀態：**CONFORMED，限定原210M只讀快照**
日期：2026-10-04

## 範圍與輸入

承接[383來源](383-moo2-colonies-job-control-source.md)及[382原收據](382-moo2-colonies-upper-continuation.md)。本契約只補正常人口操作前的原暫存／record／callback，不改主庫玩法，不送新輸入，不延長210M預算。原輸入序列與官方1.31 EXE保持；公開只提交自撰契約與索引，原JSON／LOG／PNG／RAM／state／probe留本機忽略workplace。

官方ORION2.EXE SHA-256 `4e11be14217b4aafa1839f333bf5eba037f98b0c44e9e4752c96c464c260419f`。工具基線b331bb640e4932dc59f5e4da694c63530132f4c1。原382 journal SHA-256 `cfffc3f5d60bd04fb3807f256cb4998da32a92cfa2669b3e328e36192d3ffacf`；完整原210M終態及表／code／stack／DAC／RGB／PNG是觀察前置。

## 原定位及等級

**已證實，靜態**：383保存IDA Pro9.4 linear EA原定位及兩種bytes。C0675／6905A877190069010000由原current dword乘169h，C067F／8B1518AB1800讀原pool pointer；C05A8／A1A8771900讀current。B9C3D／B9E94等record消費端保留raw運算元，不預填job名稱。1191EA／A340881A00寫場景callback，1191EF／66C7058CC417000100啟用回呼；原+20h pointers來自210M三個kind6物件。執行器另用dosgolem_high_le，資料DS188的位移加F0000h；file offsets與IDA EA各自標明。其餘UI raw窗口保留383對應operand與界限，不視為已解欄位。

| 原DS188位移 | bytes | 用途／證據界線 |
|---|---:|---|
| 2877A8 | 4 | 原current raw dword，僅按原乘法求record |
| 27AB18 | 4 | 原pool pointer |
| 287888 | 252 | 原三列暫存raw窗口，不猜欄位 |
| 2879D4 | 18 | 原列控制raw窗口，不當人口數量 |
| 26AAC8 | 78 | 原模式／游標raw窗口，不命名每byte |
| 2A8840 | 4 | 原scene callback pointer |
| 26C48C | 2 | 原callback enable word |
| 29BE14 | 2 | 原水平偏移word |
| 三筆表+20h所指位移 | 各2 | 原UI暫存word；先驗指標為2879DA／DC／DE |
| pool+signed current×169h | 361 | 原record完整raw bytes，先驗非負與64-bit加法無越界 |

**強推論**：原callback／mode與此正常畫面的控制相關。**未知**：210M實際pointer值、人口配置與正式存讀語意。取得raw值仍不等於正常操作通過。

## 邊界與狀態轉移

private mode只接受DOSGOLEM_MOO2_COLONIES_JOB_SOURCE=1，要求原upper／row／restore完整前置、原185M參數與state目錄。mode off精確回既有382，mode on仍只跑原210M。缺前置或不合法值在EXE讀取前拒絕；不把210M改作MAX參數。

只在既有finishRestore取得terminal之後捕捉一次。完整terminal所有欄位與固定382終態比較，跨run僅按既有規則排除per-run ram_sha256；本run仍逐byte hash RAM前後相等。實際reason必須step_limit、step210000000。原DS／CS／SS及全部R／flags／FPU、clock、callback IRQ、VBE、DAC journal、raw palette、indexed／RGB／PNG及36表保持。任何守衛失敗即停止，不改前置或重啟求過。

全部來源以既有descriptor-aware peekSourceWindow讀取，不觸發bus／IO，不送輸入、不Step、不寫RAM。原current作int32，必須非負；用uint64算pool+current×361並檢查32-bit上限，原descriptor limit與RAM邊界由peek再驗。無任意typed colony id上限，不用猜測名稱。所有窗口不可讀即拒絕；不寫零值替代。快照保存原offset／hex／可讀旗標、record起點、三個原pointer與word、完整before／after與RAM hash。

## READY與驗收

先核對固定journal／EXE／383原定位、欄位bytes、完整210M前置及本契約索引；通過後才READY及生成private observer。生成器的替換可逆回382；新增mode閘門CLI與正對照，全部原46CLI保持。原guest一次，原輸入及cap不改；同一收據驗證，不追加預算或挑結果。

驗收沿382完整DAC重播、39frames及舊200M／父自然RET、原418來源與state／副本核對，再逐項比較新與382完整journal，僅排除已自證只讀的per-run RAM hash；新增快照的before／after必須全等。原三pointer與raw窗口／record及callback值各自記錄，不推論人口操作。既有Go1.24.13及IDA9.4來源，不重跑無關CPU套件。Docker network none／UID1000，原資料唯讀，容器有資源與逾時限額及owned程序trap。

下一步由實際raw前置回到383原consumer，縮小一次正常人口選取／放置的觀察契約；主庫RE-first保持。固定日期不是RNG seed，完整人口操作／正式存讀／開局／remake同狀態未驗。

READY審查：固定382完整210M與383原bytes／fixup／三pointer通過；初版對115988間接指令誤要求relocation已訂正，首失敗保留。尚無新guest。

## 實際結果與界線

原guest僅一次，session6185 exit0；真正guest為step_limit210000000，不是遊戲完成。末態228DF6／483821442µs、完整382核心／FPU／clock／callback IRQ／36表／DAC／PNG保持，全部journal及舊PNG、原418來源／state／副本通過。58CLI、五private patch逆回382與公開internal／CPU／DOS／原probe保持；獨立快照before=after及RAM前後hash相等。

**已證實，原raw值**：DS188:2877A8原dword為4，pool為5B2044，完整361byte record在5B25E8，SHA-256 6fff16d8ffc032a510cf5ef745b4d5c862010078ea57a71a48537b450e111da0。原三pointer2879DA／DC／DE均word310／3601，不當人口數量。DS188:2A8840原scene callback為0，26C48C原enable word1，29BE14水平偏移0；所有窗口可讀，enable與完整原globals重疊核對。原UI選取13與current raw4各自保存，不混為typed colony id。

**未知**：當前callback為何是0、正式人口配置與安全輸入前置。C07C1／E890ECFFFF先呼叫sub_BF456；C07D2／B821ED0B00才將sub_BED21送EAX，C07E1呼叫sub_1191CA，1191EA寫callback。這條靜態先後已證實，但未觀察本次設置／還原時序。210M可見畫面不證明已進入可操作的穩定狀態，不把0當CPU缺陷，也不猜值補洞。

驗證腳本首次在exec來源檢查後，b被覆蓋為整數，導致既有日誌迭代TypeError；首腳本與輸出保留first-check。只隔離來源檢查namespace，同一收據完成全套驗收，沒有重跑guest或改原收據。READY初版對115988間接WORD指令誤要求relocation已另保留及訂正。

下一步只核對原sub_C058A的C07C1→sub_BF456返回邊界與C07D2／C07E1的callback設置，保留210M原callback0作前置。來源足夠後才建立一次有界等待／安全輸入契約；不直接加cap、送人口輸入或深入共享renderer。主庫RE-first保持。

## 385 回呼runtime地址勘誤

見[385實際結果](385-moo2-colonies-scene-ready.md)。原IDA linear EA 1A8840加F0000h後為dosgolem_high_le／DS188:298840；原runtime 2091EA的A340882900亦直接指定298840。384觀察器誤用2A8840，偏差10000h。舊收據在2A8840讀到四bytes零的事實保持，但「scene callback為0」的欄位語意撤回；正確298840讀值與實際store全RAM差異尚未取得，不當CPU缺陷。

385原guest僅一次；原BF456返回203219421、原callback A3寫入203219451、setup返回203219467、首正常輸入CALL205804505已觀察。首次輸入PNG與原210M相同。先前仍需等待場景設置的推論被此次原指令與時序否定。current／pool／361byte record、三pointer、enable／水平偏移原定位保持。本節追加訂正，舊正文與收據不改；385回呼觀察契約回到DRAFT，下一輪先修正只讀地址，不送人口輸入。

## 386 正確回呼讀值回填

不可變定位為官方1.31 ORION2.EXE／IDA linear EA dword_1A8840、1191EA／A340881A00；dosgolem_high_le runtime2091EA／A340882900及DS188:298840。見[386限定驗收](386-moo2-colonies-callback-read.md)。386沿同輸入與原210M取得原203219451寫入前55451700、寫入後21ED1A00／raw1AED21，全RAM僅實際四bytes改變已驗；首正常輸入205804505的完整原385前置守衛通過，第9只讀窗298840仍21ED1A00。

正確callback值與寫入範圍由未知／強推論升為**已證實，限定此次原A3及首輸入**。這不是原384在210M已取過正確值，也不外推全部callback生命週期。舊2A8840零raw維持未分類，原錯誤385觀察器與其DRAFT狀態保留；沒有改舊正文／Go／收據，也不當CPU缺陷。人口正常操作／正式存讀／remake同狀態仍未知。

## 392／393 正常 kind7 放開回填

不可變鍵：DOS／官方1.31 ORION2.EXE／IDA linear EA BF6ED、B9E94、1237D9及1237DB；EXE SHA-256 4e11be14217b4aafa1839f333bf5eba037f98b0c44e9e4752c96c464c260419f。runtime投影加F0000h。原kind7按下／selector由[392](392-moo2-colonies-pop-place-input.md)補驗；當次getter至20DB8C候選未觀測，不能作消費閘門。

[393](393-moo2-colonies-pop-release.md)以selector真SS返回EAX2、新GUI及callback20安全release，原BF6ED→B9E94與BF6F2真返回、下一正常1B0845及50個raw writer已驗。原MOV SS／MOV ESP中間Step的堆疊不可讀屬被動unknown，真near仍要求至少4-byte合法槽。較早kind6來源及391靜態getter仍有效，不能外推該getter候選到此kind7正常路徑。正式job欄位／跨殖民地／存讀及remake同狀態仍未知。
