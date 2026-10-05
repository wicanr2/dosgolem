# 418：原存檔格、名稱與SAVE輸入來源

狀態：**CONFORMED，限定原玩家輸入來源**
日期：2026-10-05

接續[417](417-moo2-save-page-display-continue.md)當次可見存檔頁。只建立原版資料流證據，不修改主庫Go／Ebitengine玩法或解除RE-first。

## 輸入與來源範圍

官方1.31 ORION2.EXE SHA-256 4e11be14217b4aafa1839f333bf5eba037f98b0c44e9e4752c96c464c260419f。IDA Pro9.4 linear EA、dosgolem_high_le runtime＝EA＋F0000h與file offset分列，原LE fixups獨立核對。一次窄IDA匯出原sub_7E154的260個指令，只涵蓋正常SAVE頁的選格、名稱與SAVE分派；不追callee、strcmp／strcpy／sprintf或其他runtime內部。

通用selector、kind11按下、文字consumer與release只重用moo2-379-ida-list-input-first-query.json的已保存原始rows。合計915列／891個EA／151筆原檔與IDA重定位差異逐bytes通過，沒有新guest。

## 已證實：控件與原分派

417當次原物件pointer4516E0，原dword_194038的runtime定位284038。+38h十個word為1..10、+7Ch為11..20、+232h為21、+6Eh為22、+70h為24。25項表與實際PNG保持原417收據，不使用猜測ID。

第一格title行的原控件1是kind11，矩形170,49..370,61；第二行80..92，直到第十行328..340。十個control+18h pointer依序為2816BE＋37×row，對應原byte_1916BE的37-byte record。control+2Ah raw1E0000h進入原文字長度consumer；寬度限制另有原繪圖量測，不能只靠此word宣稱所有字元均可接受。

原113FB9從index1起依inclusive矩形找first-hit，排除kind14；13個含端點／交界案例通過。裝置400,54沿既有X右移1契約成GUI200,54，first-hit1。SAVE候選裝置430,373成GUI215,373，first-hit21；CANCEL在GUI349,373對應22。名稱行與下方資訊行交界不混用，GUI200,62命中11而非1。

原7E1B2／7E1E0將[EBP+6Ah]初始化FFFF。7E226比reader低word與第一組ID，DX為zero-based loop index；7E23A將DX寫原[EBP+6Ah]。點擊已選第一組ID可設原[EBP+76h]及[EBP+7Ah]；另一組由7E240比對，另處理文字狀態。各欄位保留原offset，名稱只是本段資料流說明。

## 已證實：文字與SAVE consumer

原11E2C0／11E365辨識kind11，11E3A6設word_17C4CA＝1，11E3B2寫原word[dword_17C4CE+2]為控件ID；不走kind6的scene callback。原11D9B2將ASCII低byte寫byte_1A871C[index]，11D9BC寫後續NUL。原硬體IRQ1字母接受、初始名稱與實際輸入尚未測得，來源consumer不等於鍵盤路徑已驗。

原7E2AC比+232h的ID21；7E2B5..7E2C9接受原selected值0..9，亦有byte_191BF1的fallback。7E312使用37-byte stride，7E322..7E3EF區分空名稱／預設名稱／編輯buffer與自動生成名稱。只記錄原分支與bytes，不預填名稱或強制分支。7E3F4 CALL sub_1160B，傳入signed selected；runtime16E3F4→10160B，真return16E3F9。callee內部及實際檔案保存未驗。

原+6Eh的ID22，或負名稱ID在1..10範圍內，可走7E461關閉child；+70h的ID24為另一外層動作。正式玩家按鍵語意仍須實測，不把負ID直接命名為某個鍵。

## 收據與限制

本機忽略入口moo2-418-ida-save-slot-name-owner.py／json、new-game-418-ida-run.sh、moo2-418-source-reused-input.json、new-game-418-byte-verify.py、new-game-418-source-verify.py及new-game-418-source-result.json。原檔、IDA JSON與派生bytes不公開，公開只含自撰結論與原始定位。

IDA wrapper exit0，idat實際exit1與非空JSON／5365函式／固定hash均記錄；以原EXE獨立bytes核對通過，不將idat退出碼改寫為0。沒有新原版guest或原輸入。418只限來源CONFORMED，正常選格／命名／正式存讀與remake同狀態均未驗。

下一個私有工具契約見[419](419-moo2-save-first-slot-submit.md)：以第一格與SAVE的正常滑鼠操作驗證資料流，止於原保存callee入口；不直接派送ID、不代寫名稱或執行未查明的檔案寫入。

## 語意等級

已證實的是原控件bytes／ID、比較與writer、37-byte pointer關係及7E3F4 CALL／參數資料流。「保存入口」是依SAVE頁分派建立的強推論用途名稱，421已補sub_1160B的原檔案與資料來源；實際檔案writer結果與成功回饋仍未知。名稱不取代原函式名與位址，正常輸入到此CALL已由420補驗，見下節。

## 正常輸入回鏈

[419](419-moo2-save-first-slot-submit.md)原失敗保持；[420](420-moo2-save-release-guard-correction.md)修正額外守衛後，420已驗第一格選取與SAVE到原callee入口。16E23A selected writer與16E3F4→10160B／參數0已由正常輸入實測，callee檔案來源由421補驗；正式存讀與鍵盤命名仍未知。

## 421檔案交易來源回填

421已驗原存檔檔名、wb開啟、53個fwrite來源與close／共用返回尾端，見[421](421-moo2-save-callee-file-source.md)。已證實原CALL與資料來源，實際檔案請求／寫入、close／返回、成功GUI與正式讀回仍未驗。原有收據與限定驗收不變；下一有界原版觀察契約見[422](422-moo2-save-callee-continue.md)。
