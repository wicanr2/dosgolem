# 333：全畫面命中後的原表與正常返回

狀態：**CONFORMED**
日期：2026-10-03
範圍：固定1.31原probe唯讀資料與返回觀察；不改CPU／平台／CLI／正常輸入或remake玩法。

## 已有證據與問題

工具3580b3e26181ff0978fc7ed0b2c45f8c685f76e3；官方1.31 ORION2.EXE SHA-256 4e11be14217b4aafa1839f333bf5eba037f98b0c44e9e4752c96c464c260419f，位址均dosgolem高位LE。沿[332](332-moo2-button-tail-return.md)同417根檔／MOX.SET、44M Esc／1996-01-01、47850592按下／47851578放開、兩50M與獨立100M流程。日期不是RNG seed。

332已證實DS188:26C480指標298848／DS:29BE0E word9／DS:29BE12 dword0與55byte stride，index8範圍0／0／639／479命中x500／y229。47864849於20DDF2正常CALL209325，SS188:ESP2BDAD4存新return20DDF7，原caller返回ESP2BDAD8；8192觀察上限停而原程式正常續跑。第8筆與可見NEW GAME按鈕的關係未知。313步兩次MOV高word未捕捉與一次IRQ堆疊未重建保持限制。

## 唯讀觀察契約

僅332已驗的實際CALL20DDF2→209325且actual_call成功後，獨立設定等待資料，不改舊branchTail／terminal／384或8192界限。保存原DS、SS、caller原EBP／frame base及return EIP／ESP。從實際callee首Step前開始，至原流程50M／100M，不增加guest步數，不記callee指令trace。每步只核對自然EIP／SS／ESP是否三者同時返回20DDF7／188／2BDAD8；有則保存一次正常返回快照並停止新等待。沒有返回則明示仍等待。

最多16份新快照：開始、既有49.5M..50M每100k、60M..100M每10M、不重複50M、一次實際返回及最終終態。只新增new_game_menu_table_前綴；所有332舊列／terminal及72PNG保持。這些是觀察界限，不能當CPU／產品缺陷。

直接沿有descriptor／Limit／RAM邊界的peekSourceWindow，保存：
- 完整R／六段／EIP／flags、FPU、Bus身分、VBE、callback／IRQ原計數與RAM前後SHA-256。
- 固定原DS:26C480192bytes、DS:29BE0E16bytes，從原bytes解出當前table pointer／count／bias。
- count最多16時，從當前pointer保存count*55完整原record bytes，含index0，不命名type／handler或猜結構。count越界或descriptor不足就readable=false並保留原pointer／count，不代寫／截短冒充完整。
- 固定原SS:caller EBP-160的320bytes、固定新return ESP-4的4bytes，以及原20DDF2的16code bytes，全部含可讀旗標。不用當下callee EBP代替原caller frame。
- 新標籤記明kind／outer_step／waiting／實際返回是否匹配。完整執行狀態與RAM前後保持才readonly=true，否則明確失敗；不走CPU Bus、read hook或虛擬時間增加。

## 驗收與停止線

先READY再改probe。固定Go1.24.13映像／600s／2GiB／2CPU／128pids／UID1000／network none，原ZIP／patch唯讀重建417檔。go build後沿332原三流程，只換333輸出名。扣新前綴後全部332舊列除既定mtime／DTA四byte／PNG路徑／每次RAM雜湊保持，72既有PNG逐位元保持。兩預算共同時點的新快照除每次RAM雜湊外保持；表的指標／數量／長度／前8byte及固定frame寫回依332原來源與原bytes交叉核對。不得從新收據自身重複取期望值當玩法驗收。

自然返回有則記實際EIP／SS／ESP與原CALL關係，無則未知；表變更／未變依原bytes，不能單憑全畫面record推定title skip／輸入過早／NEW GAME。若未解正式選單關係，下一步只沿原表實際producer或原caller邊界，優先玩家阻塞，不深入整個209325／renderer／compiler helper。不提高原流程cap、不改點擊時長、不重點。CPU／平台／CLI不變，325固定EXE全套與329 CLI仍有效；主庫RE-first保持。

原版素材／LOG／PNG／RAM留本機忽略workplace，不公開。新規格同次索引000-index，限定CONFORMED後回填332與守衛；來源／收據1000:1000，Docker工作後清查自己容器。

READY審查：332原E8與新return／SS／ESP已獨立驗證，足以設定只讀等待，不需猜原函式用途。原表定位、55stride與固定frame已實測；使用現有直接peek，不經Bus。count≤16／最多16快照有明確界限，未讀完整就記未知；自然返回只匹配原三元組，觀察至既有guest cap。舊332終態不再改寫，CPU平台未改，足以READY。

## 正式收據與限定結論

Docker固定Go1.24.13映像sha256:1a6d4452c65dea36aac2e2d606b01b4a029ec90cc1ae53890540ce6173ea77ac，600s／2GiB／2CPU／128pids／UID1000／network none，原ZIP／patch唯讀重建417檔。go build -p 2 -buildvcs=false -o /tmp/moo2-probe ./workplace/moo2-probe，沿332兩50M與獨立100M環境換333輸出名，各一次；python3 workplace/new-game-333-verify.py PASS。全部3847／4825／6765舊列除既定正規化保持332，72PNG逐位元保持，332 terminal未改。兩預算共同開始與返回快照保持；新快照readonly=true，原Bus、CPU／FPU、VBE、callback／IRQ與RAM前後保持，不宣稱跨次完整RAM一致。

**已證實，正常CALL的外層返回邊界**：47864850開始時EIP209325，完整R／段／flags、原globals／header／frame／新return bytes／code與332最後CALL逐項對接。47990733自然返回20DDF7／SS188／ESP2BDAD8，R=[3CE038 1DF 2100E5 E5 2BDAD8 2BDB40 0 7]、flags293h；callback active=false、started／completed2／2，IRQ active=false、started／completed7816／7816，原return bytes F7DD2000。起始CALL47864849與返回觀察相隔125884外層Step；沒有記錄或驗證callee內部逐指令。332原8192只停觀察，不能當函式不返回；332歷史收據保留，未知已回填。

**已證實，原表更換**：開始與返回皆DS188:26C480 pointer298848、count9、bias0、55byte stride，完整495bytes逐位元相同，SHA-256 d04abf3b20ccb6058d571a8092aa113242ddd8fd3ddefc42fc79d6cfff2a3a08。index1..8前8bytes與331／332實際消費來源逐項一致。index0全55bytes零；index1..6範圍10／20／25／35、20／30／35／45、30／40／45／55、40／50／55／65、50／60／65／75、60／70／75／85；index7四界5000，index8全畫面0／0／639／479。index8完整record的+8..+9此時為0700，僅追加此時來源，未把332先前兩個MOV缺高word收據改稱完整原來源。

50M／100M終態皆相同pointer298848、count7、bias0；完整385bytes逐位元相同，SHA-256 776c6e6e61a5b17529ff383cae79a194edc17c7cf0b9a11f3dc8fc8841339183。新觀察開始／返回／終態各3份，waiting=false，沒有新CPU拒絕；正常返回後停止週期等待快照，終態保存當前資料。原caller frame之後可被其他正常呼叫覆用，50M／100M原return槽不同不冒充仍在原CALL。

| 終態index | 原四word範圍：left／top／right／bottom |
| --- | --- |
| 0 | 0／0／0／0 |
| 1 | 415／172／567／193 |
| 2 | 415／217／567／238 |
| 3 | 415／240／567／260 |
| 4 | 415／262／567／283 |
| 5 | 415／285／567／306 |
| 6 | 5000／5000／5000／5000 |

**強推論，僅顯示關係**：實際50M主選單PNG人工查看，index2範圍415／217／567／238對應可見NEW GAME位置；同500／229幾何上只命中終態index2。這不是原版重新點擊或NEW GAME指令已執行的證據，正式type／handler／更換producer與精確更換時點仍未知。已證實先前單次點擊在其實際消費時選中9筆表的全畫面index8，不能把舊click基線稱正式新遊戲按鈕已點擊成功。

| 本機忽略收據／腳本 | SHA-256 |
| --- | --- |
| workplace/moo2-probe-333-baseline.txt.gz | 0157ca8aaf9998e13068830b69624850fbd77fe51ee64dafe9e66aca01b3161e |
| workplace/moo2-probe-333-click.txt.gz | f268d7b9285eb35b86246d465d6968d82e01c86eb5d2aea3ff24867a6d9ff723 |
| workplace/moo2-probe-333-extended.txt.gz | 690cd85a374102025fd8092950ad11d66ddb5ccc62a8305cd3f85e5e4a931c82 |
| workplace/new-game-333-verify.py | db1dc3e53c34a40105f34a8549c2a73caf44232e95de858d9e022e4ac192ccaa |
| workplace/new-game-333-parity-tests.txt | 3ab912b9a2e8cb2b80511007dc4853875617e6388e713fc88083f7fbab83f16d |

probe SHA-256 fbd038f68ad029c1427cebf5baa852df0c2d90649554c38a95f10964f7ca0b57；CPU 1448f29f24dad35e83575189055dc15dd7ac9db2b3529c6d450f03baf91c97b1。CPU／startup／provider／matcher／CLI保持，325固定EXE全套及329 CLI有效。原版素材／LOG／PNG／RAM留忽略workplace，不公開。

限定CONFORMED只含此原表／來源／正常返回與舊收據保持。**正常開局／NEW GAME指令仍未知**。下一步保留44M與單次短按舊基線，另立正式選單7筆表就緒後的一次正常點擊情境：先確認實際7筆表與第2筆範圍，再正常press／release與原caller消費，不提高100M cap。這是新增驗證情境，不改正式遊戲或用代寫狀態補洞；不深入整個209325／renderer，主庫RE-first保持。

70回填函式、既有32／49／25／27／27／34／31／36／31／37／34及新增35缺證據負例、--check-menu-table-spec-backlinks／--check-button-tail-spec-backlinks PASS。workplace/new-game-333-backlink-tests.txt SHA-256 b1eec641549e6ed249dcaf60a008abd3d1bb7b0d0e68d4332cba2808590282da。原ZIP／patch／EXE／MOX.SET／417檔、CPU／平台來源保持、gofmt／Git差異／新來源及收據1000:1000核對通過，工具root-owned／誤建.md目錄自檢空。有界一次性容器均已退出移除，沒有本輪遺留；未清理其他專案或映像，沿授權推github隔離分支。
