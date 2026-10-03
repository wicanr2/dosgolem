# 332：同輪其餘判定範圍與caller返回

狀態：**CONFORMED**
日期：2026-10-03
範圍：原版probe唯讀續觀察，不改CPU、平台、CLI、輸入與remake玩法。

## 起點

工具a48f536a1132731c1b055e4419854642177b1c5e；官方1.31 EXE SHA-256 4e11be14217b4aafa1839f333bf5eba037f98b0c44e9e4752c96c464c260419f，位址均dosgolem高位LE。沿[331](331-moo2-button-branch-call-return.md)同417根檔／MOX.SET、44M Esc／1996-01-01與47850592按下／47851578放開、50M／100M正常流程。固定日期不是RNG seed。

331保存96 caller步與五自然返回，DS:26C480原表298848、count9、55byte stride。index1因x500>25跳過；index2四word20／30／35／45已讀完。最後47864195於20DCBA MOV寫SS:[EBP-1Ch]，實際下一EIP20DCBD；其餘比較／命中與caller最終返回未取得。

## 唯讀續觀察契約

舊331完整96列及terminal保持。僅當舊觀察seen、samples96、sample_budget且inactive，明示phasePrefix與正常點擊已放開、callback／IRQ0非活動，於首次20DCBD開始332續觀察。保存舊terminal原字串，續觀察另用new_game_button_tail前綴；不把新增列冒充331舊列。最多384個實際caller步、從續觀察起點最多8192外層Step，任一界限只停止觀察，不停原程式。遇到實際caller RET、錯誤／轉向或未驗CALL／RET照331明確停止。

直接peek窗口、核心／FPU／Bus／VBE只讀檢查、CALL target與ESP／new return驗證、自然EIP／SS／ESP返回再續看、callee內部省略且記明步數，完全沿331。窗口為DS:26C480192bytes、DS:29BE0E16bytes、當步DS:[EAX]8bytes、DS:2A121A16bytes、DS:2A11EC4bytes、固定初始SS:EBP-160的320bytes、DS:[EBX]64bytes與當步前後SS:[ESP]4bytes。原位址／bytes／可讀旗標與完整R／六段／EIP／flags均保留；不猜未知欄位語意，不觀察callee或compiler helper。

## 驗收

先READY再改probe。Docker固定Go1.24.13映像、600s／2GiB／2CPU／128pids／UID1000／network none，原ZIP／patch唯讀重建417檔。go build後沿331兩50M與獨立100M環境換332輸出，各只跑一次。扣除332新列後全部331舊列，除既定mtime、DTA四byte、PNG路徑與每次RAM雜湊外保持；72既有PNG逐位元保持。舊331 terminal必須逐字保持，新觀察起點接舊96最後原狀態；新步／CALL resume／實際RET與範圍比較按原bytes、來源與定義flags核算，未驗者明示未知。

若全部index實際比較已取得且能回查，才稱該輪完整範圍結果；沒到、未返回或到預算都保留未知，不能依綠測試推定NEW GAME已激活。不提高原50M／100M流程cap、不改點擊時長、不重點／代寫。CPU／平台／CLI保持，325固定EXE全套與329 CLI仍有效。新規格同次索引000-index；限定CONFORMED後回填331與守衛，原LOG／PNG／RAM留忽略workplace，主庫RE-first與完整remake／中文化目標保持。

READY審查：331真實第96步已給出下一EIP20DCBD，無需新注入或猜入口。先凍結舊331 terminal，再重設純觀察計數，既有原程式仍逐Step執行；新384／8192上限與前綴獨立，不混入舊96收據。只沿已驗peek／自然返回與停止線；完整index結果、RET或預算依實際輸出決定，足以READY。

附帶註記核對：原330 return_selector=188h，段順序CS／DS／ES／FS／GS／SS；330文件舊SS20h誤標已修正並追加勘誤，不改任何原始收據或CPU。

## 已完成IRQ的分類修訂

首輪332於47864432、20DC74 ADD後停止；原始IRQ active／failed前後皆false，started／completed由7777／7777同時到7778／7778，段與ESP保持，EAX298848+EDX1B8=298A00、下一EIP20DC76均吻合。這是完成返回的IRQ與真正caller ADD，首輪觀察器把任何IRQ計數變化判成轉向，並非原程式拒絕。初次收據保留332-initial。

修訂：只把active／failed或started與completed增量不等視為未返回IRQ；增量相同且皆非活動／無錯時可續看。保存原IRQ計數，caller的bytes／R／段／EIP／定義flags仍獨立驗。該步被IRQ覆寫的堆疊原值完整保留但不假裝只有caller指令的寫回，IRQ內部bytes不深挖、不修改平台。各步原視窗保留，實際其他變化明示未驗，不能用一般caller的單指令期望把IRQ寫回當產品缺陷。舊331無IRQ計數變化，原列不改；同容器命令乾淨重跑。

IRQ分類READY審查：原始計數與core已證實該IRQ完成返回；只修觀察器，不改IRQ服務或原CPU Step。正增量仍保存、獨立核對實際caller；IRQ造成的堆疊差異只記原值與未知，不追ISR。active／failed／未成對增量仍停止，384／8192上限與正式輸入保持，足以READY乾淨驗證。

## 正式收據與限定結論

Docker固定Go1.24.13映像sha256:1a6d4452c65dea36aac2e2d606b01b4a029ec90cc1ae53890540ce6173ea77ac，600s／2GiB／2CPU／128pids／UID1000／network none；原ZIP／patch唯讀重建417檔。go build -p 2 -buildvcs=false -o /tmp/moo2-probe ./workplace/moo2-probe，沿331兩50M與獨立100M環境換332輸出名，同命令乾淨重跑。python3 workplace/new-game-332-verify.py PASS：全部3847／4511／6451舊列除既定正規化保持331，72PNG逐位元保持，舊331 terminal逐字保持；兩預算的完整新313列相同。既定mtime／DTA四byte／PNG路徑／每次RAM雜湊正規化不代表跨執行完整RAM一致；每次快照readonly保持。

**已證實，僅固定1.31單次原輸入的範圍消費**：47864196..47864849保存313實際caller步、341省略callee步。313步的控制／R／六段／EIP／定義flags／數學核算PASS；311完整來源、2 MOV僅低word來源已驗、1 IRQ堆疊寫回未重建。20DD2D與20DDCB的8B4006讀DS:[EAX+6] dword，8byte窗口只含低word479，高word在窗口之外；實際EAX701DFh與SAR16得到7可核算，但高word來源未捕捉，不稱完整typed record已證實。47864432完成IRQ7777→7778的堆疊原bytes保留，未重建IRQ內部寫回，不稱313步全記憶體精確驗收。

DS188:26C480原dword298848、DS:29BE0E word9、DS:29BE12 dword0、55 byte stride。index1..6右界25／35／45／55／65／75皆小於x500，實際跳過；index7左界5000大於x500，20DCD1 JL到20DCE9跳過，未走其餘邊界。index8原範圍0／0／639／479，DS:298A00八bytes000000007F02DF01，x500／y229通過四比較。47864490的20DD3B保存局部index8到初始SS188:EBP2BDB40-14h；47864503的20DDDB、66A3A6C42600保存word8到DS:26C4A6。完整八項決策可回查，僅證實命中全畫面項8，不能稱NEW GAME命令已觸發。

47864507於20DDED E8 CALL208FD4，EAX8／EDX500／EBX229，新return20DDF2，ESP2BDAD8→2BDAD4；實際47864849正常返回EIP／SS188／ESP2BDAD8吻合，EAX1，callee省略341步。同一步20DDF2實際CALL209325，新return20DDF7，堆疊bytes／ESP-4吻合。新觀察terminal為samples313／max_samples384／max_outer_steps8192、waiting=true、return_selector188／return_esp2BDAD8／return_eip20DDF7、outer_budget；未達caller RET。觀察停止後原程式仍正常跑完50M／100M，沒有新CPU拒絕，主選單與credits保持。8192是觀察上限，不是原程式失敗或新的CPU缺口。

初次237步於完成IRQ處停止是觀察器分類錯誤，332-initial收據保留；修訂只接受已完成返回的成對IRQ增量，active／failed／不成對仍停止。CPU／平台／CLI未改，325固定EXE全套與329 CLI仍有效。SS段順序為CS／DS／ES／FS／GS／SS，330原SS188h的舊SS20h註記已追加勘誤；不改原始定位、bytes或歷史收據。

| 本機忽略收據／腳本 | SHA-256 |
| --- | --- |
| workplace/moo2-probe-332-baseline.txt.gz | d8d129deb83dcf71adf8cd46772e22206cbacf61be7f3723600d6e2de5bb2a55 |
| workplace/moo2-probe-332-click.txt.gz | f38048de3d96cc1db43b68f092ebd55fb0cf3443af57ca30a11436cb68d4e501 |
| workplace/moo2-probe-332-extended.txt.gz | cfb76ca5490e2dfa89cd74404f2c9a33bd48969fe4a4dcf49875273b3dbdc509 |
| workplace/new-game-332-verify.py | dd9ade15018780b0284232a058eec81678cf17446e1acb9979b2c19d2a3dde08 |
| workplace/new-game-332-parity-tests.txt | 9744908cee05cf75cf9e788cde86f96cadb1baff6a2cbb933fd48e20a50ab418 |

probe SHA-256 32f37ac91a6758e6794f30882a5184228836b5ad84317058b51828a3b336a2a4；CPU 1448f29f24dad35e83575189055dc15dd7ac9db2b3529c6d450f03baf91c97b1。原版素材／LOG／PNG／RAM留本機忽略，不公開。

限定CONFORMED只含上述範圍消費、來源限制、自然返回與舊收據保持。**正常開局／NEW GAME指令仍未知**。下一步以唯讀原表及正常返回核對全畫面index8與目前可見主選單／正式按鈕的關係，保存實際註冊表與待返回20DDF7；未有證據前不猜title skip／輸入過早或指令意義。不提高原流程cap、不改點擊時長、不重點、不深挖209325或整個renderer。主庫RE-first及255完整游標、303整體DRAFT、299自然OF=1、AH2Ch／RNG／人耳／remake同狀態保持未知。

69回填函式、既有32／49／25／27／27／34／31／36／31／37與新增34缺證據負例、--check-button-tail-spec-backlinks／--check-button-branch-spec-backlinks PASS。workplace/new-game-332-backlink-tests.txt SHA-256 66e47bb4a8605e2bcf02e2886f899bb446fab9de363f33bf5d7ffeb31ecff658。原ZIP／patch／EXE／MOX.SET／417檔、CPU／平台來源保持、gofmt／Git差異及新來源／收據1000:1000核對通過，工具root-owned／誤建.md目錄自檢空。一次性容器均已退出移除；未清理其他專案或映像。沿授權推github隔離分支，不推本機origin。
