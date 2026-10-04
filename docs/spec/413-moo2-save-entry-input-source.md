# 413：原存檔子頁入口與第一正常輸入來源

狀態：**CONFORMED，限定原來源**
日期：2026-10-05

接續[412正常SAVE GAME](412-moo2-menu-save-normal-input.md)。官方1.31 ORION2.EXE SHA-256 4e11be14217b4aafa1839f333bf5eba037f98b0c44e9e4752c96c464c260419f；IDA Pro9.4 linear EA、runtime＝EA＋F0000h、原file offset與LE重定位bytes分列。單次窄來源查詢225列／135EA／26個重定位差異，原2object／365page／51363fixup records獨立核對。

## 已證實的玩家邊界

原7E154保留名稱sub_7E154，共260指令；只取入口、24個直接CALL周邊與最小收尾定位，不翻譯完整控制流。原7E159 ENTER80h、7E15D push eax、7E15E減EBP82h均核對。原7E1DA／7E1DD分別取局部欄位地址，7E1E6 CALL7D061建立控件，返回7E1EB。原物件名與未定型欄位保持，不用局部var名稱推定玩家語意。

原7E1F2比較局部word是否0；非零走7E4AC，零到7E1FD CALL1171AB，返回7E202把EAX放入EBX。runtime CALL16E1FD→2071AB、真SS return16E202是子頁第一正常輸入候選。原來源的當次條件、25項控件表及caller已由[416](416-moo2-save-page-input-continue.md)補驗；畫面尚未刷新，不能以靜態或動態CALL聲稱存檔頁已可操作。

24個直接CALL中只有7E1FD呼叫1171AB。strcmp_、sprintf_及其他標準helper只作跨越定位，不研究內部，不計入玩法分母。7D061已有[407](407-moo2-menu-control-input-source.md)原建構與RET7D891證據；動態返回必須按當次SP及真SS槽分開，不把另一次builder的返回值預填到這次框架。

## 驗證與下一邊界

本機忽略入口workplace/new-game-413-ida-run.sh、moo2-413-ida-save-entry-input-source.py、new-game-413-byte-verify.py及new-game-413-source-verify.py；原JSON／LOG／byte index不公開。IDA殼層exit0、idat_exit1保留，非空JSON／5365函式／固定EXE hash及UID1000核對。原LE重建、225列file bytes／IDA重定位bytes與24CALL、入口及正常reader來源全部通過。沒有新CPU／DOS或Go玩法改動，本篇沒有新guest。

沿IDA9.4 locked-v1與Go1.24.13既有image、UID/GID1000、network none、原patch唯讀。IDA120s／2GiB／2CPU／128pids，bytes90s／2GiB／1CPU／128pids。私有腳本與資料均在既有workplace，原檔與私有輸出不入Git。

416已保持原正常SAVE輸入及415完整末態，只讀核對原7E1FD第一輸入與25項控件；原PNG仍是GAME，第一顯示更新由[417](417-moo2-save-page-display-continue.md)補驗。來源CONFORMED不代表存檔頁可見、正式保存／讀取或remake同狀態已驗。

## 後續輸入分派來源

416附帶一次窄IDA查詢，只取reader返回後48指令及同玩家函式的分支目標，不讀callee內部。62列／59EA／12fixup差異已用原LE／file bytes獨立核對。原7E202將EAX複製到EBX、7E204檢查AX；7E226與7E240分別比對原dword_194038物件的+38h／+7Ch加2×signed DX的word，7E29D限定十列迴圈。原7E2AC另比對物件+232h的word。上述bytes、原位址、operands與比較已證實；當次物件內容、控件名稱、slot語意及正式保存仍未知，不能從offset或局部var名猜補。

本機忽略入口new-game-416-ida-run.sh、new-game-416-byte-verify.py、new-game-416-dispatch-source-verify.py及dispatch-source-result.json，原JSON／byte index不公開。來源結果首次寫入誤用唯讀掛載，失敗收據已保存；改用UID1000可寫容器後通過，沒有重跑IDA或原guest。416正常存檔頁續行見[工具契約](416-moo2-save-page-input-continue.md)；來源只支持上述比較，不宣稱已完成存檔操作。
