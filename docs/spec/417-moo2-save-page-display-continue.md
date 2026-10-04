# 417：原存檔頁第一個顯示更新

狀態：**READY，限定唯讀原顯示續行**
日期：2026-10-05

接續[416](416-moo2-save-page-input-continue.md)完整第一reader末態，並沿[413](413-moo2-save-entry-input-source.md)的原輸入分派與外層CALL。只建立dosgolem私有原版觀察器，不改主庫玩法、不解除RE-first。

## 已證實與未知

已證實：官方1.31 EXE SHA-256 4e11be14217b4aafa1839f333bf5eba037f98b0c44e9e4752c96c464c260419f，IDA Pro9.4 linear EA、dosgolem_high_le runtime＝EA＋F0000h、file offset分列。416原238142551在2071AB，真SS return16E202、25項控件與9個只讀phase保持；VBE DisplaySets107、原PNG仍是GAME面板。控件建立不等於頁面已刷新。

已證實的原CALL／分支：7E202消費reader結果；7E492可回7E1F2，7E498→124D41、7E49D→7F206、7E4A2→1077D，7E4A7回7E1F2。只保留原定位與bytes，三個callee的具體繪圖語意未知，不深入helper，不宣稱某CALL必然完成畫面。

未知：第一reader是否先返回、其實際結果、原顯示何時更新、存檔頁是否可見，以及控件兩組ID／名稱。由原guest取樣，不強制返回0、不派送選單值，也不新增mouse move／press或鍵盤輸入。

## 固定前置與觀察契約

固定416 Go及完整page terminal／9phase，各hash見本機new-game-417-ready-review.json。完整415的18＋4phase、兩個terminal、411／409及所有祖先保持；只略既定三個RAM hash鍵。關閉新旗標須精確反轉至416原停止契約。

新旗標只接受1，要求完整SAVE_PAGE_INPUT_CONTINUE及全部祖先、state、MAX_STEPS185M；讀EXE前拒絕。275CLI保留265前綴，新增8拒絕／2正對照。保持唯一Step／原getter與公開414平台輸入，零新InjectMouseEvent或guest暫存器／RAM寫入。凍結完整416末態後才解除工具停止旗標，沿原Step續行。

首要邊界是原VBE DisplaySets首次大於當次107。只讀CPU／RAM／device、VBE、完整控件表及原PNG，在原Step後捕捉並停止；不預填下一個值、檔名或像素。若自然發生本次reader真RET，保存C3真SS槽16E202／同CS及SS／ESP＋4與唯一下一Step，記錄實際EAX；若進入上述外層CALL或下一reader，只記錄原定位，不要求它們先於顯示更新。所有phase最多24。

## 有界驗證與聲明

保持240M邏輯上限、Go1.24.13既有image、UID/GID1000、network none、原ZIP／patch唯讀，3GiB／2CPU／128pids、GOMEMLIMIT1GiB、外層1200s／kill-after15s、state1150s與owned PID trap／cgroup／Docker清理。前置建置不啟動guest，正式續行只跑一次。

獨立核對完整前置、原LE bytes／fixups、原顯示更新、非輸出狀態及原418輸入／覆蓋層。數值與實際PNG人工分開；僅有DisplaySets變化不能宣稱存檔頁已正確可見。PNG仍是GAME或不完整時保存差異，不用改名、假圖或重跑挑結果。若原服務／CPU拒絕或240M內無更新，保存實際末態，不延長或猜補。

本機忽略入口沿new-game-417-generator.py、run.sh、implementation-source-verify.py及verify.py，新Go沿moo2-colony-return-417.go；原EXE／Go／PNG／JSON／LOG不公開。原檔保存、slot選取、正常存讀及remake同狀態未驗。本稿尚無417 Go或guest。

## 來源與範圍審查

416完整末態、原VBE107與未刷新的實際PNG、413原分派CALL／分支及既有VBE狀態API已核對。顯示語意保持未知；READY授權取樣，未預判頁面結果。原稿hash與固定來源hash分別保存draft-review／ready-review；主庫玩法閘門保持，尚無417 Go或guest。
