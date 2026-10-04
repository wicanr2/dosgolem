# 409：完整406七phase後的外層框架只讀續行

狀態：**DRAFT，外層返回已驗；完整正常選單契約尚未到達**
日期：2026-10-05

來源是[408框架與作用域](408-moo2-star-map-frame-source.md)、[407正常選單reader](407-moo2-menu-control-input-source.md)，沿[406](406-moo2-star-map-game-input.md)的正常輸入。固定406 Go SHA-256 36496a3f9ca1ba8a3f126e8ee2f234454be5fdb4403bf11dc48a89f6017141ff，failed3原七phase SHA-256 21b367cb55a1e8508dbeb2092ca50df75d5e261b4717cc730619482916745156。原EXE／IDA與runtime基準沿408。這是私有原版觀察契約，主庫RE-first與玩法保持。

## 前置凍結與作用域

新旗標GAME_MENU_OUTER_CONTINUE只接受1，要求完整GAME_MENU_PRESS與舊依賴、MAX_STEPS185M與state；讀EXE前拒絕。關閉精確等同固定406，包括原caller guard拒絕，不宣稱舊契約已完成。

在226846736的原8763F phase寫入後，完整逐欄位比對failed3原七phase，僅略既定三個RAM雜湊鍵；保持404／402／401／399／397完整凍結。通過才停止原406的後續觀察器與caller預設，guest仍沿唯一CPU.Step自然續行。舊正常裝置press／release全部保持，新增InjectMouseEvent呼叫數為0。不改原RAM、core、mode或派送ID。

首輸入trueSS RA必須17651B，ESP＋4與EBP＋82h−6CCh一致；由原首輸入EBP算outer RET slot2BDBE0。在凍結點只讀固定descriptor188h的該slot dword及指向的原Code16，保存實際值與來源；只要求可讀、可映到原LE code，不把1004BC當成已知。讀取必須保持core／device／RAM與原virtual time。不符就停止，不能猜值。

## 只讀續行與停止線

記錄已拒絕較低框架的173D05：保存真SS／SP／target及下一原Step的同CS／SS、ESP+4與實際target。按ESP區分outer slot，不能把較低框架的return叫外層返回。原first-input返回17651B另要求ESP＝首輸入ESP＋4與同CS／SS。記錄當次EAX與GAME比較／mode writer，數值依原實際結果，不代寫8。

outer173D05只在ESP＝已算出的2BDBE0時保存，當次target須與凍結點讀到的dword一致；唯一下一Step同CS／SS與ESP+4才稱outer返回。原main104A6→8012F CALL、8028F→7D061控件建立CALL、7D891真RET→170294與case0的802AE→7DD41都保存真SS／SP及逐Step握手。helper的其他scope只記實際定位，不猜其語意。

到原7DD77 CALL1171AB時保存CALL；唯一下一Step到2071AB、ESP−4、同CS／SS及真return16DD7C通過後，立即停止。原mode、PNG／完整控件表與實際物件綁定另存。若230M未到、CPU拒絕或候選frame不符，保存真正末態、原bytes與未知，不補CPU或規則特例。上限沿230M，不延長；新只讀phase最多40。

## 驗證與範圍

精確反轉409 patches至固定406，唯一Step與原getter保持；新裝置呼叫數0。225CLI保留215前綴，再加8個拒絕與2個正對照。獨立核對全部新只讀phase、core／device／RAM／Code16與LE fixups、真正SS與scope、完整failed3七phase及所有舊凍結。原418檔與SAVE10／MOX保持。

原PNG人工檢視與數值結果分開。正常選單reader、控件建立、正式存讀與remake同狀態分開報告；固定日期不是seed。只讀追蹤不取代正常保存／讀取玩家路徑。

本機忽略入口workplace/new-game-409-generator.py、new-game-409-run.sh、new-game-409-source-verify.py、new-game-409-verify.py；原資料與私有Go／JSON／PNG／LOG不公開。沿既有Go1.24.13／IDA9.4，UID/GID1000、network none、原ZIP／patch唯讀。guest900s／state850s／3GiB／2CPU／128pids、GOMEMLIMIT＝1GiB與cgroup memory.events收據保持；owned PID trap與Docker清理必須收尾。409私有Go與唯一原guest已執行；完整選單契約未驗，實際收據見下節。

## 來源與只讀契約審查

408原prologue與框架算式、407的真mode0 reader、406七phase的正常輸入及所有舊完整凍結已獨立核對。新ready-review保存固定Go與原七phase雜湊，actual target只讀、不猜caller、不新增裝置輸入、既有230M與資源上限保持。原READY只授權這個私有觀察器。執行後仍未到真正選單reader，回DRAFT；正常存讀未驗。

## 實際觀察與未完成範圍

原session79227殼層exit0，230M正常上限停止；沒有panic、CPU stop或step error。225CLI含185拒絕／40正對照、完整215前綴、7個精確可反轉patch、唯一Step與原getter、零新裝置呼叫通過。完整406七phase與404／402／401／399／397保持；15個新phase的core／device／RAM／原Code16、LE fixups及PNG雜湊獨立核對。

已證實：226846742較低173D05的真SS target174BC9，唯一下一Step的ESP＋4通過。227146859回原17651B，ESP2BD4FC與EAX6。原176BC9寫191830＝0、176BD2寫191A08＝8、176BDB寫191A10＝0，逐phase記錄實際值；沒有observer代寫。227148164外層173D05的ESP2BDBE0、當次實際target1004BC，227148165同CS／SS、ESP＋4通過。227148175原104A6 CALL8012F，下一Step到17012F、trueSS return1004AB與ESP−4通過。

230000000末態runtime21F7C1，尚未到8028F控件建立、7D891返回、case0或7DD77正常reader。實際原PNG人工檢視仍為星圖，沒有GAME選單；與數值verifier分開保存。原418檔、SAVE10／MOX保持；cgroup峰值1479024640bytes、oom／oom_kill增量0。六份生成器產物在容器暫存區逐bytes重生一致，沒有為畫面重跑guest。

本篇完整選單契約仍DRAFT，不以外層返回替代完成。[410](410-moo2-menu-frontier-source.md)只保存真正末態的最小原helper定位；[411](411-moo2-game-frontier-continue.md)另建完整230M凍結後的只讀續行。正式存讀與remake同狀態未驗。
