# 406：完整404前置後的正常 GAME 輸入與選單入口

狀態：**DRAFT，正常GAME輸入已驗，外層返回觀察契約待修正**
日期：2026-10-04

來源是[405 GAME與選項入口](405-moo2-star-map-game-source.md)。只建立原版正常玩家路徑觀察，不修改主庫Go玩法。固定404 Go SHA-256 f21eaf07fbd6efb6081033a080fdef0d1ac94fb8c93b47e2983f7d939306acc1；404 list-terminal SHA-256 f215afceca9adb63c42a853251c6ed1a8f573edee0ad771ae22e714ceb2925ce。原EXE／IDA與runtime位址基準沿405。

## 前置與正常輸入

保持唯一CPU.Step、原getter、所有正常前置輸入及402／401／399／397凍結。在225305800原星圖1171AB入口按404原順序完成list-terminal，完整逐欄位比對固定404，僅略既定三個RAM雜湊鍵；不符拒絕。保留230M上限，不延長。

新旗標GAME_MENU_PRESS只接受1，要求完整LIST_RETURN及所有原依賴、MAX_STEPS185M與state。讀EXE前拒絕不完整值；關閉精確等同404。新產物嚴格406命名，實際copied build input逐bytes核對，不覆寫既有收據。

只讀descriptor188h的runtime28417C／281976／281830，當次UI word必須6、191976原word0、191830原word34。mode必須current0／previous0。23×55原表須完整等同404、runtime298848／bias0；raw6／kind0與GUI280,13第一命中通過。callback22／22、IF、無IRQ與callback pending／active、buttons0及DS188h須保持。不符拒絕，不能注入ID、mode或代寫RAM。

通過後只正常裝置press560,13,1。沿原113FB9 selector真SS返回20E1AC，ESP+4／EAX6與cached GUI280,13通過後，等callback與IRQ安全、虛擬時間至少20ms、裝置仍同點持按，再正常release560,13,0。20ms是明示測試條件，不宣稱原硬體wall-clock。工具press／release前後CPU與RAM不得直接改寫。

## 原路徑與停止線

保存星圖原8651B輸入返回、86BB6比較、86BC9／86BD2／86BDB writer及86BE4後狀態，均沿唯一原Step自然執行。共享原173D05 RET讀真SS／SP／target，預期1004BC，唯一下一Step須同CS／SS及ESP+4才稱自然返回。

保存原主分派CALL104A6→8012F。到原控件建立8028F時核對CALL bytes與真SS；唯一下一Step須到7D061、ESP−4、同CS／SS且trueSS return170294，再提前停止。終圖／控件表／mode只讀保存，實際畫面人工另判讀。新phase最多28。若未到或CPU異常，保存實際末態，不補玩法／CPU特例，不追renderer或平台helper。

## 驗證與範圍

精確反轉406 patches至固定404，唯一Step／原getter保持，新InjectMouseEvent僅兩個呼叫。215CLI保留205前綴，新增8個拒絕與2個正對照。獨立核對全部新只讀phase、core／device／RAM／原code window與重定位／真SS／UI word及mode，完整404及所有舊前置凍結、原418輸入與SAVE10／MOX副本保持。

數值結果與實際PNG人工檢視分開。固定日期不是seed；正式存讀、完整選單功能與remake同狀態均不在本契約驗證範圍。CPU／DOS公開實作與主庫玩法保持。

本機忽略入口workplace/new-game-406-generator.py、new-game-406-run.sh、new-game-406-source-verify.py、new-game-406-verify.py。沿既有Go1.24.13及IDA9.4 image，UID/GID1000、network none，原ZIP與patch只讀；guest900s／state850s／3GiB／2CPU／128pids及owned PID trap。原EXE／PNG／JSON／LOG／私有Go不公開。

## 來源與契約審查

405原bytes／fixups與分支邊界、固定404完整收據雜湊、當次callback22／22及23表、真RET83D05與8028F控件建立CALL已核對。new-game-406-ready-review.json保存審查結果。只有本機正常輸入觀察可實作；正式存讀、完整選單與主庫玩法仍待各自閘門。

## 首輪前置拒絕

2026-10-05原session7418殼層exit1／probe exit2，完整404／402／401／399／397保持，在225305800只讀press-before後拒絕，沒有送GAME press。原28417C／281976／281830三word為6／0／34；191830＝0是86BC9執行後的值，不是星圖前置。原條件猜定0已撤回，現依原實測34設固定guard；不改寫RAM，也不把34命名為選單tab。首次Go、READY審查、generator／patches／run、CLI／source驗證、原日誌、所有新凍結與PNG及state產物保留於failed1-406前綴，manifest記錄雜湊。這是觀察契約錯誤，沒有CPU缺陷證據。修正版須再審查READY並同命令乾淨重播，新的正常輸入與選單仍待驗。

修正版審查：原失敗318份產物／雜湊、完整404及較早四份凍結、原PNG／23表與Code16／fixups、只讀6／0／34、沒有GAME輸入已由new-game-406-prepress-verify.py獨立核對。新ready-review revision2固定原34，86BC9自然寫0仍待正常輸入；原CPU與DOS不改。

## 選單邊界分類補證

[407](407-moo2-menu-control-input-source.md)已證實7D061建立控件，真正mode0輸入在7DD77 CALL1171AB。406的私有事件menu-input-call／entry、key menu_input_reached及reason menu_input_entry保留原定位名稱，僅證明8028F→7D061控件建立入口；不宣稱真正選單輸入或操作已驗。這次分類修正不改執行中的Go或原guest。

## 第二輪環境中斷

原session61174殼層exit1，runner記錄probe exit137／Killed，最後週期trace210420000。沒有panic、step_error或新GAME輸入；較晚的406檔仍為首輪殘留，不能當第二輪收據。failed2-406前綴與manifest保存所有現有產物，逐檔指明原名；原418檔與SAVE10／MOX仍保持。退出碼證實SIGKILL，容器已自動刪除，原OOM旗標未知，不宣稱確診。記憶體壓力列強推論。下一輪仍沿同image／UID／輸入與230M邏輯上限，資源改為3GiB，Go1.24官方runtime的GOMEMLIMIT＝1GiB設定主工具heap軟上限，新增只讀cgroup memory.events收據；不改guest RAM、CPU、時鐘或玩法。重新審查READY後同命令重播，先清除已完整保存的406生成輸出以免殘留混入。

第三輪READY審查：兩次失敗產物與manifest雜湊均保持；固定原34與來源writer0、完整404hash、既有230M上限、唯一Step／兩個新增正常裝置呼叫保持。只變工具容器資源與主工具GC設定；資源收據在guest前後讀memory.events／memory.peak，若OOM增加則拒絕環境驗收。未驗正常GAME或選單輸入不提早改CONFORMED。

## 第三輪正常輸入與框架拒絕

session10879殼層exit1／probe exit2。完整404／402／401／399／397保持；225305800正常press560,13,1，225315011原113FB9 entry、225315546真SS返回20E1AC／EAX6／GUI280,13，225323360安全release、elapsed49572µs。七個新phase全只讀；裝置press／release前後CPU與RAM保持。226846736取到原8763F JMP83D00，但當次ESP2BD378／EBP2BD3FC與首輸入ESP2BD4F8／EBP2BDB46不同；之後83D05的caller guard拒絕。原actual target沒有保存，未知保持未知。原17651B、GAME mode writer與8012F／7D061未取樣，不宣稱星圖自然返回或選單入口已驗。failed3-406原產物與manifest保存；沒有第四次guest。

3GiB與GOMEMLIMIT＝1GiB下，cgroup峰值2119880704bytes，guest前後oom／oom_kill增量均0；環境驗收通過。第二輪SIGKILL的原OOM旗標仍未知，不回填成確診。原418檔與SAVE10／MOX保持。六份生成器產物在容器暫存區逐bytes重生一致，沒有額外guest。下一步先查真正的外層框架與原出口，再建新的READY只讀續行觀察，不添加RAM寫入、ID注入或新的點擊。

作用域來源已由[408](408-moo2-star-map-frame-source.md)核對，下一步[409](409-moo2-game-outer-frame-continue.md)凍結本篇七個原phase、無新輸入，只讀真outer slot及正常選單reader。本篇完整選單觀察契約仍DRAFT，不把正常press／selector／release的成功寫成完整通過。

409動態回填：完整406七phase與先前正常玩家前置保持，227146859原first-input返回EAX6、原mode8與1004BC outer真RET、8012F入口已驗。230M尚未到控件建立或真正正常reader，409仍DRAFT；見[409](409-moo2-game-outer-frame-continue.md)。末態最小來源由[410](410-moo2-menu-frontier-source.md)保存，下一私有只讀續行依[411](411-moo2-game-frontier-continue.md) READY，不新增裝置輸入。原版正常存讀與remake同狀態仍未驗。
