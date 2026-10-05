# 424：原正常保存越過AH3C後續行

狀態：**READY，限定私有原版存檔交易觀察**
日期：2026-10-05

接續[422原實際檔案請求](422-moo2-save-callee-continue.md)與[423普通檔建立工程驗證](423-moo2-protected-create-file.md)。目標是由原EXE自然建立SAVE1.GAM、寫入、close與返回正常owner，或保存下一個實際拒絕。不增加玩家輸入、不代寫名稱或存檔，不直接呼叫保存函式。

## 固定來源與既有失敗

- 官方1.31 ORION2.EXE SHA-256 4e11be14217b4aafa1839f333bf5eba037f98b0c44e9e4752c96c464c260419f，dosgolem_high_le；原417檔加patch、419受控SAVE10元資料與全部原輸入保持。固定曆法／元資料不當作RNG seed對拍。
- 原422唯一guest及完整500份失敗、15份執行前SHA與原verifier保持，不重跑或改寫原拒絕。其最後兩個error／terminal不會在新平台重生；不得把新成功末態包成原422同末態。
- 完整420的26phase／末態／PNG、四次正常裝置操作、source419九phase／完整失敗PNG與全部祖先仍由新guest重生並獨立比對。只略既定三個RAM hash，原日期／名稱／局部值／CPU／裝置／PNG不略。
- 423四份public source與工程result固定SHA。私有419 metadata provider僅追加與423 public overlay相同的CreateFileProvider／普通檔建立實作；原讀寫、受控mtime與所有既有bytes保持。建置不得以未擴充的419 provider覆蓋新Create能力，也不得移除受控元資料。

## 橋接前置與平台返回

新旗標DOSGOLEM_MOO2_SAVE_CREATE_CONTINUE只接受1，讀EXE前要求完整SAVE_CALLEE_CONTINUE、420及所有祖先、state目錄與舊MAX_STEPS185M。315CLI預定257拒絕／58正對照，原305項前綴保持。旗標關閉的私有Go精確反轉回422；平台423能力保持，不強制製造舊AH3C拒絕。

先凍結完整420，再沿原CPU到238506393、EIP237107h、CD21、AX3C80／CX0／DS188h／完整EDX2BD894h。原422的前六phase包含source-frozen、fopen CALL與下一Step、AH3D before／after、AH3C before；全部原6phase／PNG先逐項核對並獨立保存，不能跳過前置直接執行建立服務。

核對通過後僅沿原CPU.Step。下一Step實際AH3C若成功，核對CF0、AX合法handle、所有非輸出暫存器／segment／RAM／裝置保持及實際overlay建立；若失敗，核對CF1／錯誤碼與實際檔案狀態。不得固定回5或其他成功handle，不改EIP／CPU／RAM／時間／檔案內容，也不重擲。

## 原交易只讀觀察

重用421的55個CALL定位、原fopen／53個fwrite／fclose與true SS返回觀察。附原參數、實際buffer與返回值、DOS請求／返回、VBE frame及PNG；不追標準runtime內部。

- 原fopen CALL與真RET、實際SAVE1.GAM／wb與FILE pointer。
- 53個fwrite CALL與真RET，EAX buffer／EDX size／EBX count／ECX FILE及完整可讀bytes；順序和返回由原EXE決定，不預填成功。
- 實際DOS AH3C／3D／3E／3F／40／42／43／6C，記錄CF與EAX、handle、大小和buffer。不從fwrite次數假定DOS write次數，緩衝寫入由原版決定。
- 原101BCBh fclose及真RET、共用尾端10160Ah近RET到真SS返回16E3F9h，ESP＋4；下一正常16E1FDh CALL→2071ABh／true return16E202後停止。未到不補造返回。
- 讀取平台handle與實際overlay只作觀察，不更改檔案位置或再開可寫檔。正常返回後的原畫面／名稱刷新另以PNG檢視，不由記憶體名稱推定可見成功。

250M後段與1200s外層／1150s owned capture保持；3GiB／2CPU／128pids／UID1000／network none／GOMEMLIMIT1GiB。快照預算在執行前固定512，涵蓋55個CALL各before／after／RETbefore／after及DOS before／after；不宣稱已知DOS緩衝請求數。超過預算或下一平台／CPU拒絕即保存實際結果，不調上限追成功。零新增裝置輸入，沒有正式永久測試seed。

## 驗收與證據限制

READY後才建立私有觀察器。先驗精確反轉、唯一Step／原getter、零新CPU／guest RAM／鍵盤／滑鼠寫入、315CLI、六份逐bytes重生、423平台／metadata來源與獨立verifier，固定所有執行前SHA後跑唯一原guest。

完整保存必須同時證明原filename／mode、實際53個寫入buffer及返回、所有原chunks與實際SAVE1.GAM逐bytes一致、close返回0、owner真返回與下一正常reader。檔案存在、服務成功或工程測試都不足以證明保存完成。原ZIP／patch／base保持唯讀，state來源副本或MOX若有變化照實記錄，不能還原後隱藏。

正常讀回、鍵盤命名、其他slot／版本及remake同狀態另驗。snapshot／數值與人工PNG檢視分開；固定seed與不同PRNG未建立等價條件，本輪沒有亂數規則對拍。主庫RE-first保持，這份規格不修改Go／Ebitengine玩法、CPU解碼或新增DOS能力。

本機前置入口new-game-422-verification-result.json、save-callee-events.json、failed1-422-manifest.json與new-game-423-engineering-result.json；424原稿／READY review保存workplace/，公開入口掛000-index。尚無424 Go或guest。

## 來源與橋接審查

422的六個成功／請求phase與最後兩個拒絕phase已逐項區分；423可寫能力／工程結果／四份source與原metadata provider已核對。原稿與new-game-424-ready-review.json分開固定。只授權私有觀察器及一個有界原guest，來源不足或下一拒絕回RE／spec，不猜補正式玩法。尚無424 Go或guest。
