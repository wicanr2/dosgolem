# 408：原星圖框架與共用收尾段的作用域

狀態：**CONFORMED，限定原prologue與既有406收據**
日期：2026-10-05

接續[406正常GAME輸入](406-moo2-star-map-game-input.md)。固定官方1.31 EXE SHA-256 4e11be14217b4aafa1839f333bf5eba037f98b0c44e9e4752c96c464c260419f。IDA9.4 linear EA、runtime＝EA+F0000h、file offset分列。一次窄查詢66列／57個EA／15列原bytes與重定位bytes不同；原2object／365page／51363fixup records核對。只找入口框架、原輸入CALL與共用尾段，沒有新guest或Go。

## 框架來源與計算

已證實：sub_86188先push EBX／ECX／EDX／ESI／EDI，8618D ENTER6CCh，86191 SUB EBP,82h。原86516 CALL1171AB；原87638 LEA ESP,[EBP+82h]、8763E POP EBP、8763F JMP83D00，再由83D00..83D04的五個pop到83D05 C3。原main104B7 CALL86188只證明靜態直接caller，當次outer return value須另讀，不從xref預填。

406完整404末態的1171AB entry有trueSS return17651B，ESP2BD4F8／EBP2BDB46。ENTER／EBP rebasing的獨立算式：

| 定位 | 算式 | 既有原值 |
| --- | --- | --- |
| CALL返回後的outer local base | 首輸入ESP＋4＝首輸入EBP＋82h−6CCh | 2BD4FC |
| 外層epilog的saved registers起點 | 首輸入EBP＋82h＋4 | 2BDBCC |
| 五個pop之後的outer near RET slot | 首輸入EBP＋82h＋4＋20 | 2BDBE0 |

這些是框架定位，不是存檔欄位或新的玩法規則。406取到226846736的8763F，但當次ESP是2BD378，與outer saved registers起點2BDBCC不同。只用EIP命中共用尾段會套錯框架。已證實原ESP異於outer算式的位置；強推論為另一個活動框架。為何進入該位置仍未知；當次返回值由409補讀為174BC9，不猜間接caller或重入機制。406原input return17651B與mode8 writer未取樣，不稱外層退出已驗。

## 下一個最小觀察

[409](409-moo2-game-outer-frame-continue.md)先凍結406七個原phase，保持已送的press／release，之後只讀續行。從原first-input EBP算outer slot並讀實際return dword；對173D05按ESP與CS／SS分開作用域，保存已拒絕的較低框架之真RET與實際target，不預設1004BC。真正outer return須同slot、同CS／SS、唯一下一Step與ESP+4。原17651B input return另按首輸入ESP+4辨識。

選單後續依[407](407-moo2-menu-control-input-source.md)：7D061是控件建立，7D891是其真RET；case0的7DD77→1171AB才是正常選單reader。主庫RE-first、CPU／DOS與原輸入保持；沒有間接平台helper深挖。

本機忽略入口workplace/new-game-408-ida-run.sh、new-game-408-byte-verify.py、new-game-408-source-verify.py。原JSON／LOG／byte index及私有腳本不公開。IDA9.4 locked-v1、Go1.24.13／Python3.11，UID/GID1000、network none、patch唯讀；IDA120s／2GiB／2CPU／128pids、bytes90s／2GiB／1CPU。殼層exit0／idat_exit1、非空JSON／5365函式與固定原hash通過。

來源回填：[405](405-moo2-star-map-game-source.md)保留原8763F／83D05定位，但實際outer scope改依本篇框架；[406](406-moo2-star-map-game-input.md)仍DRAFT，正常GUI選取6及放開已驗，outer return與選單尚未驗。

409動態回填：完整406七phase與先前正常玩家前置保持，227146859原first-input返回EAX6、原mode8與1004BC outer真RET、8012F入口已驗。230M尚未到控件建立或真正正常reader，409仍DRAFT；見[409](409-moo2-game-outer-frame-continue.md)。末態最小來源由[410](410-moo2-menu-frontier-source.md)保存，下一私有只讀續行依[411](411-moo2-game-frontier-continue.md) READY，不新增裝置輸入。原版正常存讀與remake同狀態仍未驗。
