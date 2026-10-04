# 417：原存檔頁第一個顯示更新

狀態：**CONFORMED，限定第一原顯示更新與頁面可見**
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

本機忽略入口沿new-game-417-generator.py、run.sh、implementation-source-verify.py及verify.py，新Go沿moo2-colony-return-417.go；原EXE／Go／PNG／JSON／LOG不公開。原檔保存、slot選取、正常存讀及remake同狀態未驗。417私有Go與唯一原guest已驗，限定結果見下節。

## 來源與範圍審查

416完整末態、原VBE107與未刷新的實際PNG、413原分派CALL／分支及既有VBE狀態API已核對。顯示語意保持未知；READY授權取樣，未預判頁面結果。原稿hash與固定來源hash分別保存draft-review／ready-review；主庫玩法閘門保持，417私有Go與唯一原guest已驗，限定結果見下節。

## 唯讀欄位補充審查

為避免再次只取得控件表而缺少ID映射，本次在新417快照附上當次DS:原dword_194038的pointer及0x234原bytes。範圍只涵蓋413已證實消費的+38h／+7Ch兩組十個word及+232h的word；原offset、完整bytes、pointer與可讀性保持，控件名稱與欄位語意仍未知。不得以預填ID或自訂結構欄位取代原資料。每次追加讀取核對core／RAM／device前後不變。

此補充只擴充私有唯讀收據，原417-ready-review.json保持不覆寫；審查另存new-game-417-readonly-bindings-review.json。每Step沿原getter保留前一個core與VBE狀態，首次顯示更新可核對唯一Step與原Code16，不新增CPU.Step或原輸入。

## 實作前置驗證

10個可反轉patch、唯一Step／原getter、零新裝置輸入、275CLI含225拒絕／50正對照及六份逐bytes重生通過。414平台建置輸入、完整416來源hash與補充唯讀範圍保持；原續行收據完成後才判定CONFORMED。


## 原版續行與限定驗證

唯一原session74131，outer／probe均exit0。完整416 terminal／9phase與25項表、415與全部祖先保持。238143403原207261 C3的真SS槽16E202，238143404唯一下一Step自然返回、ESP＋4，實際EAX0；沒有預填返回值。238143422／455／187171分別觀測原16E498／16E49D／16E4A2 CALL，不把觀測當callee語意。

238251948第一原顯示更新，上一Step DisplaySets107、當次108，末態EIP228CA9，8個phase全部只讀；240M上限不延長。人工檢視實際PNG可見九個empty slot、Auto Save、SAVE／CANCEL，原GAME標題與星圖保持。PNG SHA-256 ee59ac6fdce06a1a7e391ea30607abc2ceef5789355e178f0a23c07d81ebfb67。數值與圖像驗證分開，沒有為畫面重跑。

當次原pointer4516E0、0x234原bytes已保存，+38h十個word為1..10、+7Ch為11..20、+232h為21；25項／1375bytes表hash仍為0400259e495af6e8bf475e1d5c63e55118459ca94f0bfc9a716a3b959c44b1e1。兩組ID與比較consumer已證實；矩形及畫面可建立位置關聯，正式選格／編輯語意尚待原正常輸入確認，不用自訂名稱覆蓋raw offset。

275CLI含225拒絕／50正對照、10個反轉patch、六份逐bytes重生、原LE bytes／fixups及所有完整來源保持。原418輸入、SAVE10／MOX與覆蓋層保持，cgroup峰值2012610560bytes，oom／oom_kill增量0。收據沿new-game-417-verification-result.json、visual-review.json及conformance-review.json，原EXE／Go／JSON／PNG／state仍本機忽略。

CONFORMED只限本頁首次原顯示及實際可見。原first reader返回0是當次結果，後續選格、命名、正式保存／讀取及remake同狀態仍未驗。下一步先核對當次第一空格kind11與兩組ID的正常輸入分派、選格／文字編輯／SAVE的consumer，形成來源審查後才決定下一個有界原輸入。主庫玩法RE-first保持。

## 418 輸入來源回填

[418](418-moo2-save-slot-name-input-source.md)錨定原名稱／ID／writer與保存CALL；[419](419-moo2-save-first-slot-submit.md)原守衛失敗保持，由[420](420-moo2-save-release-guard-correction.md)取代。420已驗第一格選取與SAVE到原callee入口，本417的首次顯示契約與原收據不變；正式存讀與鍵盤命名仍未驗。
