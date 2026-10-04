# 387：職業列裝置座標與持按／放開來源

狀態：**CONFORMED，限定原靜態來源及既有首輸入核對**
日期：2026-10-04

## 範圍與來源

承接[386正確回呼前置](386-moo2-colonies-callback-read.md)與[383職業列來源](383-moo2-colonies-job-control-source.md)。本輪只建立原座標到kind6、持按與放開的來源，不改CPU／DOS／主庫玩法，不重跑guest或送人口輸入。原版人口選取／放置及正式存讀仍未知。

官方1.31 ORION2.EXE SHA-256 `4e11be14217b4aafa1839f333bf5eba037f98b0c44e9e4752c96c464c260419f`；工具基線3e2290007d5d0163346b4150c7e8cc625b1e0166。IDA Pro9.4 linear EA、原file offset／file bytes／relocated bytes及dosgolem_high_le分開保存；runtime投影加F0000h，沿原LE來源核對，不能再把2A8840當callback。五個一次性IDA查詢，2562筆rows／2091原EA／514筆fixup差異，原2objects／365pages／51363 records獨立核對，bytes index留私有workplace。

## 已證實的原座標及旗標

sub_1236D1為1236D1..1237F3／74項，runtime8:2136D1與既有386callback target一致。原1236E8保存ECX、1236EB保存EDX；123711比較word_17C51A是否0，非0跳過座標更新；12371F比較word_17C51A+2是否0，非0走替代按鍵分支。兩者均0時，123729 MOVSX取ECX低word，12372D／D1F8作SAR1，12372F／66A3383A1B00存原GUI X；123738／66A3363A1B00存EDX低word為GUI Y，123741存EBX低word到word_1B121A。原1237F2／CB是遠返回，平台回呼dispatcher仍依既有契約處理，不改CPU或推論其餘CB分支完成。

**已證實，靜態**：GUI X在IDA1B3A38／runtimeDS188:2A3A38，Y在IDA1B3A36／runtime2A3A36；旗標在26C51A／26C51C。123ABA／123AD2讀X低word；123AE7／123AFF讀Y低word。1B3A3A是123EA7寫入的區域索引暫存，並非Y，不從無符號名稱猜型別。原123D53把GUI X／Y複製到word_1B121C／1B121E，供123BC1／123BEE的事件來源讀取；不把事件座標與目前座標當同一欄位。

原初始化sub_123491中，12357D設INT33功能7，123598讀width word_17C534，12359E DEC、12359F ADD EAX,EAX，1235A1存上界，得到2×(width−1)。功能8的Y上界由1235D6讀height word_17C538再減1。原1236C1最後清word_17C51A，並非本次首輸入旗標取樣的替代。width／height在runtime26C534／26C538。

## 兩條輸入分支與kind6

**已證實，靜態**：sub_124075於12408D設AX3、1240C5呼叫int386_；一般分支1240D9讀BX低word，1240DF AND3，替代旗標分支另取原17C51E／17C520。原11DB56 CALL後若AX非0，11DB5E跳11E0BF；該分支11E12B／11E133讀目前X／Y，11E160再次AX3，11E168為0跳11E4EB。事件分支11DBF0／11DBF8則讀123BC1／123BEE的已保存座標，不混稱兩條分支均直接讀目前位置。

持按分支受word_17C4E4控制：11E19D比較，非0在11E1A7呼叫sub_113FB9。該函式113FB9..114177／136項，讀目前X／Y、從index1按37h stride及signed含端點矩形掃描，非type14首次命中直接離開。其比較與383／379的來源一致，但它是持按分支的實際選取器，不以原11DC事件掃描代替。原36表index1／2／3仍kind6，shared邊界first-match依原索引順序。

原11E1F1、11E33B在對應原active kind6條件下呼叫1192D1；原11E4EB在AX3為0後檢查共享active表項kind6，11E508再次呼叫1192D1，11E50D清word_17C4A6。實際回呼是否造成職務改變仍未知；不得只看到熱區或callback就宣稱人口操作成功。原118FD4／11B05A／1156E2、+20h pointer及310..510暫存來源仍見383。

## 驗收與未知邊界

先審查固定來源、原opcode／CALL／CB／完整386首輸入及索引，才READY。只讀驗證器核對原bytes index及條件、13個signed X模型、3個range模型、6個人口列端點／內點first-match模型，以及公開程式／既有收據保持。模型只證明來源推導，不是新原版實測。

**未知，阻塞送人口輸入**：首正常輸入205804505的26C51A／26C51C、width／height、目前及事件座標、按鍵暫存與17C4E4的實際raw值，以及持按／放開真正到達的分支。386原前置只讀了record／UI控制／callback，未讀這些新窗口，不補值或假設旗標0。660,77→GUI330,77只在上述原座標分支和裝置range條件成立時可作候選，不直接送入。

下一步388沿同原輸入與210M，在已固定205804505首輸入完整frame及正確callback守衛後，取上述raw窗口、descriptor／裝置range與目前按鍵。先取得可比較前置，再建立一次正常人口列press／持按消費／安全release契約。停止於玩家路徑所需證據，不深挖cursor繪圖／鍵盤替代實作／平台helper。主庫RE-first保持，固定日期不是seed，正式人口變更／存讀／完整開局／remake同狀態未驗。

原EXE／JSON／LOG／PNG／RAM／state與私有腳本不公開。沿Go1.24.13及IDA9.4 locked-v1 Docker，原patch唯讀、UID1000／network none，IDA120s／2GiB／2CPU／128pids，驗證30s／512MiB／1CPU／64pids，有界一次性容器收尾。

READY審查：固定五份IDA／原LE bytes、原條件SAR1與Y／CB、持按選取及放開回呼、完整386首輸入與未知旗標邊界通過；先於只讀模型實作。

## CONFORMED限定結果與停止線

五份IDA schema1／5365函式及固定EXE核對，全部2562筆rows／2091個原EA、514筆fixup差異與原LE51363 records通過。sub_1171AB54項、callback1236D1 74項、目前座標選取器113FB9 136項均完整；11CEF5有1550項，只保留原頭尾及三個生命週期、三個press片段，不稱整函式已解。caller最多32並保留截斷標記，直接xref不代表所有間接讀寫。

原13個signed X、3個range與6個kind6命中模型通過；含奇數、負word、三列共享邊界的首命中。這些只是來源推導，660,77→330,77在條件成立時模型先1；沒有送新的guest輸入。完整386首輸入／36表／正確callback與舊385／386收據及公開internal／CPU／DOS／原probe保持。

READY先於只讀模型；387 SOURCE PASS限原靜態來源與既有收據。379／383／386按不可變定位追加回填，索引同步。RE verifier首次誤在唯讀mount建檔，沒有寫出檔案；改用既有可寫工作樹後同來源通過，分類為環境問題。五個IDA外層均exit0，idat exit1用有效非空JSON、原hash與UID核對，不把exit1單獨當成功或失敗。

**未知，仍阻塞人口輸入**：當次首輸入的旗標／寬高／座標／事件／按鍵與持按分支原raw條件。下一步388沿同輸入與210M完整首輸入守衛補讀，不重跑模型求碰巧符合條件，不猜旗標0或動態job語意。主庫玩法閘門保持，人口變更／正式存讀／完整開局／remake同狀態未驗。

## 388 原首輸入動態前置回填

見[388只讀raw及裝置範圍](388-moo2-colonies-mouse-source.md)。不可變鍵為官方1.31／IDA linear EA word_17C51A／17C51C、17C534／17C538、1B3A38／1B3A36、1B121A及17C4E4；dosgolem_high_le投影見388原8窗。原205804505的8窗與裝置range已由388只讀補驗，完整前後狀態及既有210M結果保持。

較早正文只描述當時收據，保留其未知邊界；當次205804505的上述raw前置由388補驗。原持按／放開真正消費及人口變更仍未知。下一步389固定本次完整首輸入、原旗標及range，建立一次正常職業列press觀察契約；候選裝置660,77按原signed SAR1為GUI330,77，原36表先命中kind6 index1。先追原持按選取與1192D1／場景回呼，依實際消費點安全release；不把候選命中當人口變更或預設職業語意。

## 389 正常職業列輸入回填

見[389正常press與release](389-moo2-colonies-pop-press.md)。不可變鍵為官方1.31／IDA linear EA 11E1A7→113FB9、1192F3／1A8840、BED21、11E508／11E50D；dosgolem_high_le投影分開見389時序。原205804505完整388前置後，正常660,77按下，原選取器返回1，場景依真SS返回後安全放開；零按鍵、kind6場景回呼與共享active清除已驗。原8-byte record差異只在206658147之後到210M形成，正式職務與放置語意未驗。較早未知是當時收據邊界，389補驗限定此正常輸入；不改舊正文或receipt。

下一步390沿同輸入及210M，追查206658147之後原C086E→C02F9與B9C3D／B9E94的最小正式寫入鏈，定位這8個record差異與可放置狀態；取得證據才訂一次跨職業列放置。不假設8,000k→4,000k已完成換職或刪除人口，不盲增cap或深挖renderer／平台helper。
