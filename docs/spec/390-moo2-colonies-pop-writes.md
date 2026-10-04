# 390：放開職業列後的原記錄寫入

狀態：**CONFORMED，限定原版只讀寫入觀察**
日期：2026-10-04

## 範圍與來源

接續[389正常職業列輸入](389-moo2-colonies-pop-press.md)。保持相同前置、660,77按下及安全放開、210000000步上限。沒有新輸入，不改主庫玩法、CPU或DOS服務。只定位206658147共享active清除後至210M的361byte記錄差異及選取暫存。

官方1.31 ORION2.EXE SHA-256為4e11be14217b4aafa1839f333bf5eba037f98b0c44e9e4752c96c464c260419f。工具基線d2c3519528cc475d4c891990e72449a0698fd5bd。389私有Go SHA-256為4395a63d157555ad918007ab816d92fccdf5d872693033253dc672d4227bff4d；終態收據為3af68d6bdac519dbb2e2107b0af35c71b9b245b6ee2f87e470ddaa93b15bcfab。IDA Pro9.4 linear EA沿383原來源與byte索引，原檔偏移及LE fixups各自保留；dosgolem_high_le程式與資料投影加F0000h。

## 已證實來源與未知

原IDA C086E呼叫C02F9；C0337呼叫BF627。BF627檢查17AABB，0分支在BF681設1、BF688呼叫B9C3D；1分支在BF6ED呼叫B9E94。B9C3D的B9C81保存SI至17A974；B9CAF以AND FDh清pool+169h×colony+4×slot+0Dh的bit1。這些是靜態分支，不預設本次一定執行。

389首14事件的record保持，終態有8個byte差異。正式職務與跨列放置未知。不得從Pop8,000k→4,000k推定人口刪除或配置完成。

## 私有觀察契約

新DOSGOLEM_MOO2_COLONIES_POP_WRITES只接受1並要求POP_PRESS及原完整前置。mode off可精確逆回389。原runSteps、唯一CPU.Step及既有裝置輸入保持。

從原active-clear-after206658147啟動，驗證DS188 descriptor及current4／pool5B2044／record5B25E8。每個原CPU.Step前後直接只讀已驗RAM範圍：record361bytes、26A970起32bytes、26AAB0起32bytes、2879D4起18bytes。前兩個暫存範圍包含原17A974與17AABB投影；不猜欄位名稱。

保存每次實際byte變更的前後值、來源EIP／code／registers／segments／flags、原Step錯誤及變更後完整只讀frame。另取原C086E、C02F9、C0337、BF627、BF681、BF688、B9C3D、B9CAF、BF6ED及B9E94首次到達。變更最多128，來源到達最多10；超出即拒絕。觀察器不用Bus包裝、不額外Step、不寫guest RAM，也不直接呼叫玩法。

取樣前後完整RAM及狀態相同，PNG各綁雜湊。若來源未到達或新CPU停止，保存有界結果，不延長、不重新送輸入或改seed。固定日期不是seed。

## 驗證及停止線

來源審查後READY才生成私有觀察器。獨立驗證所有389事件、210M末態、完整日誌及DAC journal保持；只沿389既有每次RAM／檔案mtime／DTA時間欄位正規化，不遮新record或選取值。以變更鏈重建每個監看範圍終值，證明8個record差異來自實際原Step；將來源指令對383 byte索引或必要的窄IDA匯出。來源缺口先保留unknown，不猜正式語意。

Go1.24.13既有Docker，network none、UID/GID1000、原ZIP與patch唯讀；native600s／2GiB／2CPU／128pids，owned PID trap，驗證90s／2GiB／1CPU。原EXE／JSON／PNG／RAM／LOG／state及private Go留本機忽略，公開只提交自撰文件。重生入口為workplace/new-game-390-run.sh，獨立驗證為workplace/new-game-390-verify.py。

本輪完成只表示這一次正常選取的原寫入鏈已定位。正式放置、存檔語意、完整開局、RNG與remake同狀態另驗；主庫RE-first保持。不深入renderer、DAC、PIT或平台helper。

READY審查：固定389來源、15事件及原dispatch／兩分支／AND寫入bytes通過；私有實作尚未生成。

## 只讀監看守衛勘誤

首輪native的觀察器在active清除後以「390缺固定DS」panic。該守衛把每步目前DS等於188列為必要條件，無法容許原程式切換段暫存器。這是觀察器拒絕，尚無原CPU錯誤證據；第一次沒有寫入鏈完成收據，不列CONFORMED。失敗Go、日誌、圖片、來源／CLI、實際state副本均按failed-390前綴保留，入口failed-390-manifest.json。

修正為監看固定selector188的descriptor與同一組原RAM位址，獨立於目前DS；descriptor每步核對，越界仍拒絕。原code及stack依當步CS／SS讀取。先用抽出的實際getter驗證DS切換不影響監看值及越界拒絕，重新審查後READY，才同命令／同輸入／210M乾淨重跑一次。此例外僅修正觀察器，不改CPU、guest輸入或執行上限；原失敗保持。

修正版READY審查：抽取的實際getter以三種DS切換重現舊拒絕，新getter保持四個窗口與RAM，只讀與descriptor／RAM越界拒絕通過。原來源／輸入／210M、公開CPU不變。

## CONFORMED限定寫入結果

**已證實，限定此正常選取路徑**：修正版原210M／step_limit保持389全部15事件、完整日誌與DAC journal、末態EIP1A5042／482658319µs及原PNG。206659890到C086E、206659891到C02F9、206660054到C0337、206660055到BF627。206697288原BF681將17AABB由0設1；206697307原B9C81把17A974由FFFF設4。原B9CAF於206697426／545／664／783依序將record+0Dh／11h／15h／19h的bit1清除，02→00。BF6ED及B9E94未到達，限定此次走選取分支。

**已證實，原bytes與實際writer**：69個實際Step變更重建四個監看範圍終值；8個首末record差異均定位。+E7h的word由原DE727在206700878寫0；+C8h先由原E19C6在206704020寫FE70h，再由E1CD9在206704659加到FEB9h，後續重算保持FEB9h；+0Bh由E1E64在206704718將FF改02。另有+EFh／F2h／FCh／104h等先清除再重建的中間值，首末比較不會顯示，均保留實際byte變更。record+0Ah原08保持；原+0B／C8／E7正式名稱與職務數量仍未定型。

新增一次窄IDA9.4查詢，16個未索引實際writer定位，233列／212個EA／14筆重定位差異；原MZ／LE、2object／365page／51363fixup records獨立核對。保留原始函式名、EA、file offset及bytes，runtime投影分開；__STOSB／__STOSD只保存實際清除writer與呼叫邊界，不追平台helper。

118CLI含98拒絕及20正對照，原106保持。原418輸入、SAVE10／MOX實際副本hash／size／UID1000保持；79份新增只讀frame與PNG綁定，初態／8來源到達／69Step變更／終態，完整RAM／device／狀態取樣前後相同。所有實際writer均有原bytes索引，沒有未知writer。

首輪觀察器以目前DS必須188誤拒，session94008 exit1，原失敗88份產物按failed-390前綴及manifest保留；無原CPU缺陷證據。改監看固定descriptor188，實際getter隔離測試重現舊拒絕並證實三種DS切換、只讀及越界拒絕；DRAFT→修正版READY後同命令／同輸入／210M乾淨重跑。修正版session42905 exit0；獨立驗證session84523 exit0，同一批收據，無第三次guest。

391先捕捉選取後下一個原輸入點1B0845，核對原17AABB=1、17A974=4及第二列signed熱區；條件成立才以正常裝置660,107一次按下及安全放開，驗證BF6ED→B9E94與四槽是否恢復。不得直接改bit／派送ID，不把210M中途renderer末態當可按輸入點。 正式放置、人口配置、存檔語意及remake同狀態未驗；主庫RE-first保持。
