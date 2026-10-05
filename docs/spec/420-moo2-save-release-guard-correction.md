# 420：原第一存檔格放開守衛修正

目前證據：424已驗原SAVE1.GAM的53次寫入共208000bytes、close0與callee真返回；正常玩家返回及讀回未驗，詳見[424原保存續行結果](424-moo2-save-create-continue.md)。本文件的來源或原失敗驗收範圍保持；424整體回DRAFT，截圖留存、返回路徑與容器逾時待修。

狀態：**CONFORMED，限定私有原版正常選格與SAVE到callee入口**
日期：2026-10-05

[419](419-moo2-save-first-slot-submit.md)唯一原guest已到238295561、原selector真RET到20E1AC／EAX1。工具額外要求mask2Bh，原版此時mask1，因此正常release沒有派送。這是私有觀察器偏離419 READY契約的守衛錯誤，沒有CPU拒絕證據。419現由本規格接替；原失敗、完整九phase及一個正常press保持。

## 已證實來源

官方1.31 EXE SHA-256 4e11be14217b4aafa1839f333bf5eba037f98b0c44e9e4752c96c464c260419f，位址空間dosgolem_high_le。完整417八phase／terminal含日期與PNG保持。238285407 CALL16E1FD、238285408真SS到2071AB，正常400,54,1 press；238295560原204176 C3、238295561真SS return20E1AC、ESP＋4及實際EAX1已證實。

原mask1、callback25／25 idle／pending0、target8:2136D1、buttons1與原25項表／物件ID保持，全部九phase只讀。已有415正常SAVE GAME release-before在236264659同樣mask1、buttons1，後續reader／mode3／存檔入口已驗。不得將2Bh當成所有放開的必要條件。來源釘選、失敗manifest與未知保存於本機new-game-420-ready-review.json；原419資料不覆寫。

## 修正契約

新旗標DOSGOLEM_MOO2_SAVE_RELEASE_GUARD_CORRECTION只接受1，要求完整419及所有祖先、空state與舊MAX_STEPS185M。295CLI保持285前綴，新增8拒絕／2正對照。旗標關閉可逐bytes反轉回419，包含其原停止結果。唯一CPU.Step、原getter與公開CPU／414平台保持。

新guest必須自然重播419完整失敗。只有完全相同的238295561守衛前置、完整九phase／terminal與CPU／RAM／device只讀檢查通過，才能啟用修正。原419中止點以獨立source419收據保存；不能只比EIP或從存檔入口直接開始。只略既定三個RAM hash，日期、PNG、控件、原碼與全部輸入仍比較。

由窄wrapper捕捉當次實際額外守衛錯誤，其他panic照常拋出。附加原terminal只讀快照以凍結九phase與完整失敗末態，另存new-game-420-source419-save-submit-events／terminal.json及對應PNG。續行時僅移除私有collector的失敗release-before／terminal，已另存的九phase不動；再用原API執行當次尚未派送的release，不修改guest狀態。放開只額外接受mask1，仍要求原selector已消費、idle／pending0、真target、buttons1及原first-hit；按下仍依原守衛。正常輸入、座標、虛擬時間方法、原名稱處理及245M停止線沿419，不調延遲、不派送ID或代寫RAM。

第一reader實際結果、原選格writer與kind11焦點、SAVE正常press／release及原16E3F4 CALL→10160B／真return16E3F9與EAX0仍須實測。到callee入口就停，正式檔案寫入／讀取與鍵盤命名未知。這是工具守衛修正，主庫RE-first保持。

## 有界驗證

沿既有Go1.24.13 image、3GiB／2CPU／128pids／UID1000／network none／GOMEMLIMIT1GiB、1200s外層與1150s owned state capture。原ZIP／patch唯讀，沿419受控檔案日期輸入。先建置、CLI、六份逐bytes重生及獨立驗證器，再跑唯一420 guest；原419不重跑。所有失敗照實保存。

本機入口預定new-game-420-generator.py／run.sh／implementation-source-verify.py／verify.py、moo2-colony-return-420.go及new-game-420-ready-review.json。原版與私有Go／JSON／PNG留本機。來源充分只允許實作，不宣稱正常release、保存入口或remake對拍已通過。


## 當次原版驗證結果

唯一420 session64443，outer／probe exit0；獨立verifier session61489 exit0。原419完整失敗九phase／terminal、PNG45262e及完整417八phase／terminal與所有祖先保持。295CLI含241拒絕／54正對照、五個反轉patch、六份重生及公開CPU／414平台釘選通過。原419沒有重跑或覆寫。

238285408正常400,54,1 press；238295561原selector真RET／EAX1後，同一步mask1、callback25／25 idle／pending0正常release。兩個操作的CPU／RAM立即保持，僅裝置狀態由原API改變。238311500原207261 C3、238311501真SS返回16E202／ESP＋4／EAX1；238311517原16E23A寫word [EBP+6Ah]，238311518原值FFFF變0，第一格已由原writer定案。

238473198下一原reader入口時，selected0、editor active1／focus1由實際raw確認；正常430,373,1 SAVE press。238484120原selector真RET／EAX21，mask1正常release；238503075原reader真SS返回16E202／ESP＋4／EAX21。原16E3F4 CALL後，238505421真SS到10160B、ESP−4／return16E3F9、實際EAX0，立即停止，callee尚未執行。

26個新phase全部只讀，四次正常裝置操作，零新鍵盤或CPU.Step／guest RAM寫入。原首37-byte名稱record已生成Strader, Human, 1 colony；editor buffer仍為原empty-slot字串。這是當次記憶體結果，不推廣為所有名稱模板。實際PNG a6a210241532bd87f46b58fd33ecf13665b4dbe8f4bfa4aeedf78c0f5c2d836a人工可見SAVE頁與SAVE游標，首列仍顯示empty slot，沒有保存成功回饋。

cgroup峰值1679069184bytes、OOM增量0；418原輸入、SAVE10／MOX保持，overlay無新內容差異，22筆受控日期與原兩群一致。平台硬體時間仍為既有近似，固定calendar不是整段PRNG seed對拍。原419失敗保存，Docker專案容器已清理。

本CONFORMED只限正常第一格與SAVE到原callee入口。原CALL／參數與selected writer已證實；421已補原檔案與資料來源，實際寫入／close／返回、成功回饋、正式保存／讀取與鍵盤命名仍未知，remake同狀態未驗。下一步依422 READY作有界原版觀察，不追C runtime／OS wrapper內部。

## 421檔案交易來源回填

421已驗原存檔檔名、wb開啟、53個fwrite來源與close／共用返回尾端，見[421](421-moo2-save-callee-file-source.md)。已證實原CALL與資料來源，實際檔案請求／寫入、close／返回、成功GUI與正式讀回仍未驗。原有收據與限定驗收不變；下一有界原版觀察契約見[422](422-moo2-save-callee-continue.md)。

422當次動態結論：422已驗SAVE1.GAM／wb實際buffer與AH3C平台拒絕，正式存檔未完成。見[422原實際請求](422-moo2-save-callee-continue.md)；原fopen未返回、53個fwrite來源尚未成為實際寫入，後續平台補缺依[423](423-moo2-protected-create-file.md)。
