# 416：原存檔頁第一正常輸入續行

狀態：**CONFORMED，限定原控件建立與第一reader**
日期：2026-10-05

接續[415原真SAVE入口](415-moo2-save-attributes-continue.md)，來源沿[413](413-moo2-save-entry-input-source.md)。官方1.31 EXE SHA-256 4e11be14217b4aafa1839f333bf5eba037f98b0c44e9e4752c96c464c260419f；IDA Pro9.4 linear EA、dosgolem_high_le runtime＝EA＋F0000h、file offset分列。本篇只授權私有原版唯讀觀察，不改主庫玩法或解除RE-first。

## 固定前置與範圍

固定415 Go SHA-256 63237795ea000ded0675c7886738f1196db1877cabdc9cc1961eb95a91eaadfb。原238113912入口16E154、完整18個SAVE及4個屬性phase與各自terminal必須逐欄凍結，僅略既定三個RAM hash鍵；411／409／406及全部祖先保持。保留415原SAVE1.GAM缺檔AX2／CF1、正常release、mode3 writer與真SS return1702D1，不重寫412原拒絕或原失敗產物。

新旗標只接受1，要求完整SAVE_ATTRIBUTES_CONTINUE及全部祖先、state、MAX_STEPS185M，讀EXE前拒絕。關閉須精確反轉至415私有Go及其原入口停止契約。265CLI保留255前綴，再加8拒絕／2正對照；不新增InjectMouseEvent、CPU.Step、getter或任何guest暫存器／RAM寫入。公開CPU／probe保持，DOS差異只允許已審查的414兩檔。

在415原停止點先呼叫原finisher並保存完整terminal，核對原18／4phase後才停止舊觀察器、解除工具停止旗標，沿原CPU.Step續行。工具旗標不得改變guest狀態。原415公開平台建置輸入hash 0d1860f0c22c7583e25865cfa5697dc54b11061efb0a38b8e1feceb85f40b90d保持，runner必須明示copy到固定archive。

## 已證實來源與動態握手

413單次IDA來源225列／135EA／26fixup差異核對，原sub_7E154名稱與260指令定位保持。原7E1E6 CALL7D061，runtime16E1E6→16D061及真SS return16E1EB；建構函式原RET7D891已由407核對。只在本次實際builder入口SP／CS／SS與真槽符合時觀察C3，不把另一次builder框架預填過來。

觀察每個CALL前及唯一下一Step、ESP−4／真SS返回槽；RET前及唯一下一Step、ESP＋4／同CS／SS及實際目標。原7E1F2條件另只讀保存，不猜局部欄位名稱或強制走零分支。若自然走非零7E4AC，保存原bytes與末態，不能以測試專用條件補洞。

原7E1FD CALL1171AB，runtime16E1FD→2071AB、真SS return16E202，是第一正常輸入候選。要求本次builder已自然返回、原正常release保持，實際device buttons0。唯一下一Step到2071AB即停止，保存完整當次控件表／raw records、core／RAM／device及原PNG。控件count、表內容、slot名稱與可見畫面未知，從原guest讀取，不預填。到reader只證明原頁面路徑，不推定slot選取或檔案保存完成。

## 停止線、資源與驗證

仍以240M邏輯上限停止，不延長；新phase最多24。若平台拒絕、CPU不支援、原分支未到或觀察器失敗，保存精確原因與原bytes，分類後再開最小問題。保持唯一Step及全部舊收據，不深挖平台helper。

沿Go1.24.13既有image、UID/GID1000、network none、原ZIP／patch唯讀；3GiB／2CPU／128pids、GOMEMLIMIT1GiB、外層1200s／kill-after15s、state1150s與owned PID trap／cgroup收據保持。前置建置／265CLI／重生不啟動guest，正式原續行只跑一次；不為改名、PNG或驗證腳本重跑。

獨立重建LE bytes／fixups，核對完整415 terminal／18＋4phase及全部祖先、本次真CALL／RET、正常release與實際控件表。418原輸入保持，覆蓋層差異另存。原PNG人工與數值分開，不能以入口或單元測試替代存讀驗收。正式存讀與remake同狀態未驗，固定日期不是seed。

本機忽略入口沿workplace/new-game-416-generator.py、run.sh、implementation-source-verify.py、verify.py命名；新Go沿moo2-colony-return-416.go，原EXE／Go／PNG／JSON／LOG不公開。來源及完整415末態已審查，見下節；416私有Go與唯一原guest已驗；結果見下節。

## 來源與契約審查

413原builder／reader CALL及返回位置、407原RET、415完整18＋4phase與真SAVE入口已核對。原稿hash、固定來源hash與審查分別保存new-game-416-draft-review.json及new-game-416-ready-review.json。READY只授權上述私有唯讀觀察器；未知控件內容與分支由原guest取樣，不解除主庫RE-first，416私有Go與唯一原guest已驗；結果見下節。

## 實作前置驗證

12個可反轉patch、唯一Step／原getter、零新裝置輸入、265CLI含217拒絕／48正對照及六份逐bytes重生通過。414平台建置輸入及原415來源hash保持。實際原續行收據完成後才判定CONFORMED。

## 原版續行與限定驗證

唯一原session56044，outer／probe均exit0。完整415兩個terminal與18＋4phase及全部祖先保持；原238113912入口只解除工具停止旗標，沒有guest狀態寫入。238114944原16E1E6 CALL、238114945真SS到16D061，ESP−4／return16E1EB；238142546原16D891 C3、238142547唯一下一Step自然返回16E1EB，ESP＋4與同CS／SS通過。238142548原局部word分支自然走零路徑，沒有代寫條件。

238142550原16E1FD CALL、238142551真SS到2071AB，ESP−4／return16E202通過，入口立即停止。9個新phase全只讀，唯一Step／原getter及零新裝置輸入；265CLI含217拒絕／48正對照、12patch及六份重生通過。原LE bytes／fixups、全部前置與真握手由獨立驗證器核對。

當次25×55bytes控件，pointer298848／bias0，表hash 0400259e495af6e8bf475e1d5c63e55118459ca94f0bfc9a716a3b959c44b1e1。含十個kind11矩形、十個kind7矩形及兩個底部kind0矩形；實際欄位語意、物件兩組ID與+232h尚未動態映射，不預填SAVE或RETURN名稱。

原PNG人工仍可見GAME面板及SAVE游標，hash與415相同；控件更新沒有證明存檔頁已刷新。CONFORMED只限控件建構與第一reader，不宣稱頁面可見、slot選取、檔案保存或remake同狀態。418輸入及SAVE10／MOX保持，覆蓋層無新差異，cgroup峰值1725698048bytes、oom／oom_kill增量0。後續須沿正常reader返回與原外層迴圈觀察刷新，禁止直接入口或假畫面。

附帶最小IDA分派來源62列／59EA／12fixup差異，見[413回填](413-moo2-save-entry-input-source.md)。最初來源結果誤用唯讀掛載，失敗收據保存後只修輸出掛載，沒有重跑原guest或IDA。初次規格審核判為跨越主庫RE-first；核對玩法限制、工具READY規範及活表下一步後，同操作獲准，正式玩法閘門保持。

本機忽略入口new-game-416-verify.py、verification-result.json、visual-review.json及conformance-review.json；原檔、Go、JSON／PNG／LOG不公開。

| 本機收據 | SHA-256 |
| --- | --- |
| moo2-colony-return-416.go | d49ea3616cee52bf17638089f866c956e2f06b5387cfae691b10b6c6552205e9 |
| new-game-416-save-page-events.json | 5b039dacf6d3045206f490ded8224634572a4a0f95bb813d61d055d442a8482d |
| new-game-416-save-page-terminal.json | bfb51eaa961768b594d3379e216519842ad70fc798161bcff987928aaeabc599 |
| new-game-416-verification-result.json | a6ca744d153442ddea68545ec9fd2b77edb78a864f886f704f3eb30c9195915c |
| new-game-416-visual-review.json | 0d1d811874c9ca19d1a2b0eca9c240b00fb0a3c446514b75c9b5c5ecad6a89bb |
