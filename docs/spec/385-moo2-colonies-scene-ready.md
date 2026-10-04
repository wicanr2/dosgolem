# 385：殖民地回呼設置與首個輸入入口的有界觀察

狀態：**DRAFT，回呼runtime投影待修正；原返回與首輸入已驗**
日期：2026-10-04

## 來源與範圍

承接[384原210M快照](384-moo2-colonies-job-source-snapshot.md)。先前scene callback0使用錯誤runtime投影，該語意已撤回；本契約只記錄自然返回、原callback寫入與首次C0845輸入CALL，不送人口操作，不改主庫玩法或CPU。使用原200M／210M完整前置，200–210M保持既有guest，210M守衛通過後只有一次10M窗口，最大220M。觀察結果不能靠重啟、加cap或挑結果取得。

官方1.31 ORION2.EXE SHA-256 `4e11be14217b4aafa1839f333bf5eba037f98b0c44e9e4752c96c464c260419f`；工具基線0a6c7f97c75262c1d36098f5d6763918e91e2305。原382 journal SHA-256 `cfffc3f5d60bd04fb3807f256cb4998da32a92cfa2669b3e328e36192d3ffacf`；384原210M完整核心／FPU／clock／callback IRQ／table／palette／PNG／record保持。原資料及收據不公開。

## 已證實的最小原來源

IDA Pro9.4／IDA linear EA，原file offset／file bytes與套fixup bytes分存moo2-385-source-byte-index.json；runtime另標dosgolem_high_le，代碼位移加F0000h。

- sub_BF456為BF456..BF500／43項，BF4FF／C3自然RET；C058A的C07C1／E890ECFFFF呼叫它，下一C07C6映射1B07C6。BF456只有72次sentinel初始化及後續場景呼叫，不深挖共用繪製helper。
- C07D2／B821ED0B00送sub_BED21到EAX，C07E1／E8E4890500呼叫sub_1191CA。1191EA／A340881A00寫dword_1A8840，119225／C3返回C07E6，runtime為2091EA／209225→1B07E6。sub_1191CA 31項，有205 direct callers，匯出32並明示截斷，不宣稱只供本場景。
- C0845／E861690500呼叫sub_1171AB；這是C058A正常輸入迴圈的第一個已定位CALL。定位不證明玩家已點選或人口變更。
- 383已保存的107BC與11926C是原callback欄位的其他直接store候選。只在原指令前後取值，最多32筆callback寫入，保留原來源與raw值，不追共享helper內部。

原READY階段的**強推論**為210M可能仍在場景建立或回呼生命週期中；本輪原返回與首輸入收據已否定此解釋。原欄位零值的語意已撤回；人口正常操作仍**未知**。捕捉從200M開始，避免把210M快照誤當設置必定尚未發生。

## 原READY計畫的私有觀察邊界

DOSGOLEM_MOO2_COLONIES_SCENE_READY只接受1，要求原JOB_SOURCE／upper／row／restore與185M參數及state完整前置。mode off精確回384；mode on最大220M。非法值／缺前置在EXE讀取前拒絕，保留原58CLI，加新mode拒絕及正對照。

200M沿原parent來源。210M取得scene-source210完整frame，除kind與明示run_limit外核對原384／382；舊job-source在210M仍只讀一次，不搬移／改寫原收據。新terminal不混作原210M。

從200M觀察：BF4FF的RET，只有真SS:ESP slot=1B07C6才接受為該原caller返回；C07E1取完整只讀call前置；1191EA、107BC、11926C保留原store的raw before／after與EAX，記錄最多32筆，超限明示截斷；119225只有真slot=1B07E6才當本次setup返回。所有原CPU.Step仍只執行一次，觀察器不代寫guest狀態。

每個RET／store讀code／stack／callback／enable及完整核心／FPU／clock／callback IRQ／VBE／DAC，descriptor-aware peek與RAM前後hash證明讀取只讀。RET核對原C3／真stack／ESP+4與其他核心保持；store核對原A3及目標四bytes，其他RAM保持。實際raw若不符來源預期，保留差異，不以猜值補洞。

首次C0845保存完整frame／PNG與8個raw窗口、完整361byte record及三pointer，讀前後保持。原callback值、record及控制值當raw事實；不預填job語意，不要求未知值碰巧等於期望。窗口未到入口保留unknown，不能自動延長。

## 驗收、停止線與交接

先核對固定EXE／新IDA／原bytes+fixups及完整384前置、索引，才READY及生成可逆private patches。沿原完整DAC重播、舊frame／200M父RET、原418來源及state／副本驗收；原384完整journal至210M與PNG保持。新收據逐項核對stage順序、原真RET／store、first-input readonly及所有raw值，無新輸入。原guest一次，沿同收據驗證。Go1.24.13／IDA locked-v1，Docker network none／UID1000，原資料唯讀，資源及外層逾時有界，owned程序trap收尾。

主庫RE-first保持；不追renderer／palette／DAC／PIT或一般runtime helper。10M是明示觀察預算，不是原硬體時間契約。固定日期不是RNG seed，人口選取／放置、正式存讀、完整開局與remake同狀態仍未知。

READY審查：固定EXE／新IDA bytes及原384完整210M前置、原RET／store／首輸入CALL、200–220M及32筆界限通過，先於實作。

## 實際結果、地址拒絕與下一步

原guest僅一次，session26846 exit0，終態step_limit220000000／1A5174／513881291µs。原210M完整前置、既有journal／39frames／DAC／PNG、418來源／state及副本保持；沒有新增滑鼠輸入、CPU或主庫玩法改動。8個private patches可逆回384，70CLI為58拒絕及12正對照，來源檢查在原READY階段通過。上述工程成功不代表整份回呼觀察契約CONFORMED。

**已證實，原runtime時序**：BF456於203219421執行1AF4FF／C3，真SS stack返回1B07C6；203219427到1B07E1原CALL，EAX1AED21；203219451於2091EA執行A340882900，下一2091EF；203219467於209225／C3依真stack返回1B07E6。兩次RET的ESP+4、其他核心／FPU與RAM保持核對通過。

**已證實，地址模型被否定**：IDA linear EA dword_1A8840加F0000h是dosgolem_high_le／DS188:298840。原A3運算元也是298840；private watcher卻讀2A8840，偏差10000h。原錯誤觀察窗的四bytes零未變，但它不代表場景回呼零值。store的ram_other_unchanged=false來自錯誤排除範圍，不能推定CPU缺陷。**強推論**：原A3將EAX1AED21寫至298840。**未知**：正確位移的實際讀回與全RAM僅改該四bytes的證明。未捕捉的資料不補值、不重寫原收據。

**已證實，正常輸入入口**：首次1B0845／E861690500在205804505、virtual471004699µs到達，count36，RGB非零681553；完整frame與只讀窗口留scene-input收據。PNG SHA-256 1a0e7551173173b43897be18ccf5295755ca80b222be7dc6dbca0b1d986c8c03與原210M相同。current raw4／pool5B2044／record5B25E8、三word310及其餘正確定位窗口保持384。既有210M預算已涵蓋設置及首次輸入，無需再等待或延長。

獨立驗證session93305 exit0，SAFE EVIDENCE PASS限定上述原返回／首輸入／舊收據保持；ADDRESS MODEL REJECTED明示回呼欄位定位失敗。首次驗證以Python重新序列化Go事件導致key順序與HTML escape雜湊差異；改核對原journal中的事件文字切片後同收據通過。首腳本與輸出保留，沒有guest重跑。生成器第一次定位到三個同名marshal段落，在寫Go前拒絕；限定唯一上層payload後生成。RE verifier初次寫到唯讀mount失敗，改用既有可寫工作樹，屬環境問題。

383與384追加地址勘誤，索引移除目前零callback斷言。本契約由READY回到DRAFT，原private Go及guest收據保留，舊READY審查作歷史紀錄；不再以錯誤定位啟動新guest。下一輪386先依原A3運算元校正298840的只讀觀察與守衛，再在已定位205804505首輸入狀態驗正確讀值；維持原輸入與210M，不猜座標比例、不深入共享renderer。主庫RE-first保持；人口選取／放置、正式存讀、完整開局與remake同狀態仍未驗。
