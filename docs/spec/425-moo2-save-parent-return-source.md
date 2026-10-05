# 425：原SAVE頁返回與上一層旗標來源

狀態：**CONFORMED，限定來源核對**
日期：2026-10-05

接續[424保存交易與返回前提](424-moo2-save-create-continue.md)。此項只釐清原SAVE頁返回到哪一層，以及呼叫端如何消費退出旗標；沒有新guest、下一正常輸入或remake同狀態驗收。

## 輸入與工具

官方1.31 ORION2.EXE SHA-256 4e11be14217b4aafa1839f333bf5eba037f98b0c44e9e4752c96c464c260419f；IDA Pro9.4的locked-v1 image SHA-256 6f6d59af49d0008c4109a5295b5f374bdc007e2d1ab28cb9de08779584de2780。原patch唯讀，一次性DB在/tmp，UID1000／network none／2GiB／2CPU／128pids／180s，輸出目錄檢查UID/GID1000。

位址分開保存：下列靜態為IDA linear EA；dosgolem_high_le各加F0000h；原MZ／LE file offset及fixup由獨立工具核對。沒有重命名函式、局部欄位或運算元。

## 已證實的來源

- 原415正常SAVE頁入口238113912、runtime16E154，真SS stack首四bytes D1021700即return1702D1。新IDA定位原sub_8012F，802CC CALL7E154的return802D1與實際stack相符。
- 原802C9的LEA EAX,[EBP-0Ch]把呼叫端局部位址傳給子頁；原8013C將該位置初始化0。只保留原運算元，不替它推測命名。
- 原802E6讀word[EBP-0Ch]；802EB為JZ801E1。非零值會略過回圈，沿802F1後的分支與直接call到尾端。這是靜態控制流，不宣稱已觀察實際SAVE後父層值。
- 原4-case table8011F逐bytes與LE fixup核對，case3到802C9／802CC；完整parent146列／146個EA與原LE相同，指令fixup差異0。其他case只保存路由定位，不研究其玩法或callee內部。
- 共用尾端重用413的原7DA0C..7DA12七列：LEAVE、POP EDI／ESI／EDX／ECX／EBX、近RET。子頁由7E50C跳7DA0D，先自行恢復EBP再使用這組POP／RET。不因共用尾端位在另一函式範圍就誤路由。

## 推論與未知

強推論：424已核對SAVE路徑經原指標寫1，與此parent傳入的局部位址銜接，應使parent離開迴圈。實際指標值、child尾端返回、parent局部值及各分支尚未新增動態觀察。

未知：sub_8012F的實際上一層caller、parent共用RET的真SS return、返回後第一個正常輸入點與可操作GUI、正常存讀往返。不能把原16E1FD同頁reader改成另一個猜測位址。

下一最小來源查詢：只取IDA對8012F的直接caller、call前後局部上下文及正常reader路由。證據足夠後另審READY觀察契約，維持固定前置與step上限；唯一PNG檔名及容器內owned timeout也需先驗。沒有新正式玩法或CPU變更，主庫RE-first保持。

## 驗證收據與公開邊界

單次narrow IDA：bash /out/new-game-425-ida-run.sh，session91096 wrapper0、idat1；非空JSON／schema／EXE SHA／5365函式及UID1000檢查通過，不隱藏原退出碼。沿既有Go1.24.13 image／UID1000／network none／2GiB／60s執行python3 workplace/new-game-425-source-verify.py，退出0；原MZ／LE／51363 relocation records、146新列、七列重用尾端及原415真stack核對通過。

本機入口new-game-425-source-plan.json、source-result.json、source-verify.py／tests、moo2-425-ida-save-parent-source.py／json／log／stdout與new-game-425-ida-run.sh／output。source-result保存原輸入及十份證據SHA；入口掛000-index，424同次回鏈。原反組譯、JSON、遊戲檔案與存檔只留本機；公開只提交自撰規格。

目前來源回填：426已驗唯一原104A6 caller、1004AB返回與44項場景分派表；下一正常輸入仍未知，見[426](426-moo2-save-parent-caller-dispatch-source.md)。427已驗原parent entry／真SS1004AB，但原250M未達parent返回／分派，整體DRAFT；418獨立PNG及owned生命週期已驗，見[427結果](427-moo2-save-parent-return-observation.md)。末態支援函式邊界見[428](428-moo2-save-terminal-support-boundary.md)，下一只追parent旗標及保存後玩家consumer。
