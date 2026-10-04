# 388：首輸入座標、按鍵與裝置範圍只讀前置

狀態：**CONFORMED，限定原首輸入只讀觀察**
日期：2026-10-04

## 範圍與固定來源

承接[387條件座標與持按來源](387-moo2-colonies-input-coordinates.md)及[386正確回呼](386-moo2-colonies-callback-read.md)。原人口操作尚未送；本契約只在原205804505首輸入補讀旗標／寬高／目前及事件座標／按鍵／持按選取閘門與裝置範圍，不改CPU／DOS行為或主庫玩法，不送新輸入，不延長210M。

官方1.31 ORION2.EXE SHA-256 `4e11be14217b4aafa1839f333bf5eba037f98b0c44e9e4752c96c464c260419f`；工具基線1d0d128c52a7de357a41a19b8bcbb2513bc70e46。原386首輸入JSON SHA-256 `aa6728601569f07b6ed5022d0a7fa5c1f5291fa3e18592e40a7e84415c5c715f`，原386 private Go SHA-256 `3a050603540f9da42345ca0726b089952177e20cedfeadfd4884e33c92532e92`。IDA Pro9.4 linear EA及原file bytes／fixups見387 index；dosgolem_high_le資料投影加F0000h，原DS188:298840已讀回21ED1A00，不再使用錯誤2A8840回呼語意。

## 已證實來源與raw窗口

387已證實原callback1236D1只有word_17C51A與+2均0時，ECX低wordsigned SAR1存GUI X，EDX低word存Y；原1237F2／CB遠返回。123491初始化X range為2×(width−1)、Y=height−1。123ABA／123AE7讀目前座標，123D53把座標複製到1B121C／1B121E；持按113FB9受17C4E4控制。這些是來源，不能猜本次值。

| dosgolem_high_le／DS188位移 | bytes | 來源及解釋邊界 |
|---|---:|---|
| 26C51A | 4 | 原17C51A及+2旗標words |
| 26C534 | 2 | 原width word，來源123598 |
| 26C538 | 2 | 原height word，來源1235D6 |
| 2A3A34 | 10 | 原座標及區域raw；Y在+2、X在+4，+6不當Y |
| 2A121A | 16 | 原按鍵／事件座標及相鄰raw；1B121A／1C／1E、1220／1222等各自保存 |
| 26C48E | 26 | 原active／共享選取相關raw，保留未命名bytes |
| 26C4E4 | 2 | 持按選取閘門word，來源11E19D |
| 298840 | 4 | 原正確場景callback，沿386讀回守衛 |

實際raw值必須成功讀取；不可讀、界限超出或完整前置漂移立即拒絕，不填零或重新跑求過。pointer／record與三word仍沿386原9窗保持，不命名正式job數量。

## 私有觀察行為與裝置快照

新DOSGOLEM_MOO2_COLONIES_MOUSE_SOURCE只接受1，要求原CALLBACK_READ／SCENE／JOB／upper／row／restore與185M參數及state完整前置。mode off精確逆回386；mode on固定原210M與原輸入。保留原82CLI，增加新mode拒絕及正對照，全部在EXE讀取前測試。

先經既有完整385首輸入守衛，再以固定386首輸入JSON核對完整frame，僅按既有跨run規則排除ram_sha256；核對正確callback及原9窗來源後，才在same step取新8窗。透過既有activationPeek及descriptor-aware peekSourceWindow讀取，不Step、不IO、不InjectMouseEvent、不代寫guest RAM。

裝置range與目前x／y／buttons保存在MOO2StartupDOS所嵌入的FD2StartupDOS私有欄位。原internal/machine/le_startup.go的mouseCoordinateRange含set／minimum／maximum，INT33功能7／8設定range；InjectMouseEvent會先依該範圍裁切。只在私有overlay加入MouseReadSnapshot388純取值方法，保存range設定旗標與signed界限、目前uint16座標／按鍵、query enabled及calls。方法無Handle、無CPU／bus／IO／queue寫入，不成為公開platform API。以取樣前後device map、完整core／FPU／clock／callback IRQ／VBE／DAC及RAM hash全等證明只讀。

新收據獨立保存完整source_terminal、原8窗offset／size／hex／readable、descriptor、device_before／after及完整state_before／after。原386scene-input及9窗收據不改；不把新資料塞回舊收據，不新增任意typed欄位上限或預設660,77。

## 審查、驗收與停止線

先審查387原bytes及投影、386完整首輸入／正確callback、裝置欄位來源與純只讀方法，才READY及生成可逆private patches。原CPU.Step及原玩家輸入仍唯一，private machine helper只讀而不取代任何正式函式。原guest一次，同一收據驗證，保持210M，不重啟或延長。

沿386完整210M journal、四事件／真RET／A3／全RAM四bytes、完整首輸入／PNG／DAC／原39frames／418來源／state與副本；新原9窗保持、新8窗及device state只讀核對。原旗標、range或座標不符預期也照實保存，不把未知結果改成錯誤或改條件求綠。CONFORMED只限只讀原前置，不能外推人口操作成功。

下一步按實際旗標／range／座標／按鍵及閘門，縮小一次正常人口列press／持按消費／安全release契約；未取得資料前不直接送人口輸入。主庫RE-first保持，不深挖renderer／DAC／PIT／cursor或鍵盤替代內部；固定日期不是seed，人口變更／正式存讀／完整開局／remake同狀態未知。原EXE／JSON／PNG／LOG／RAM／state及private Go維持本機忽略目錄，公開只提交自撰文件與回鏈。

READY審查：new-game-388-ready-review.py通過原387 bytes、386完整首輸入／正確callback及裝置欄位來源；審查先於private實作。

## CONFORMED限定結果

已證實，限定原首輸入只讀前置。原205804505／1B0845首輸入：DS188:26C51A／26C51C為0／0，width／height為640／480。目前GUI X／Y=43／48，保存事件X／Y=43／48，按鍵word=0，持按閘門26C4E4=1，共享active word26C4A6=0。裝置x／y／buttons=86／48／0，X range=0..1278，Y range=0..479，range設定旗標=True／True。 原8窗均可讀，descriptor／device before=after，完整core／FPU／clock／callback IRQ／VBE／DAC及RAM保持；原386的9窗、record／pointer words及210M完整收據保持。沒有新人口輸入，正式人口變更／存讀／完整開局／RNG及remake同狀態未知。

| DS188位移 | bytes | raw hex |
|---|---:|---|
| 26C51A | 4 | 00000000 |
| 26C534 | 2 | 8002 |
| 26C538 | 2 | E001 |
| 2A3A34 | 10 | 000030002B0000000100 |
| 2A121A | 16 | 00002B00300000000100010001000000 |
| 26C48E | 26 | 0000FFFF0100FFFF000001000000000000007F02DF0100000000 |
| 26C4E4 | 2 | 0100 |
| 298840 | 4 | 21ED1A00 |

裝置快照透過純私有取值取得，兩側calls及range完全一致。94CLI含原82逐項保持、78拒絕及16正對照。原guest一次／210M，來源與獨立驗證均通過；沒有CPU／DOS／公開platform API或主庫玩法變動。

下一步389固定本次完整首輸入、原旗標及range，建立一次正常職業列press觀察契約；候選裝置660,77按原signed SAR1為GUI330,77，原36表先命中kind6 index1。先追原持按選取與1192D1／場景回呼，依實際消費點安全release；不把候選命中當人口變更或預設職業語意。

固定輸入與影像：官方1.31 EXE雜湊及Go1.24.13 image沿上方來源；新mouse-source JSON SHA-256 650876e32fe41394fe85ae2b7dc871ccc7de551a72153fe16d2f0fee6beee49d。PNG沿原1a0e7551173173b43897be18ccf5295755ca80b222be7dc6dbca0b1d986c8c03。私有收據入口為本機workplace/new-game-388-mouse-source.json及new-game-388-verify.py，需沿new-game-388-run.sh在唯讀原ZIP／patch、UID1000／network none的有界Docker重生；本契約不支持未控制完整開局或正式人口對拍。

## 389 正常職業列輸入回填

見[389正常press與release](389-moo2-colonies-pop-press.md)。不可變鍵為官方1.31／IDA linear EA 11E1A7→113FB9、1192F3／1A8840、BED21、11E508／11E50D；dosgolem_high_le投影分開見389時序。原205804505完整388前置後，正常660,77按下，原選取器返回1，場景依真SS返回後安全放開；零按鍵、kind6場景回呼與共享active清除已驗。原8-byte record差異只在206658147之後到210M形成，正式職務與放置語意未驗。較早未知是當時收據邊界，389補驗限定此正常輸入；不改舊正文或receipt。

下一步390沿同輸入及210M，追查206658147之後原C086E→C02F9與B9C3D／B9E94的最小正式寫入鏈，定位這8個record差異與可放置狀態；取得證據才訂一次跨職業列放置。不假設8,000k→4,000k已完成換職或刪除人口，不盲增cap或深挖renderer／平台helper。
