# 334：正式選單就緒後的正常新遊戲點擊

狀態：**CONFORMED**
日期：2026-10-03
範圍：固定1.31原probe新增明示玩家驗證情境，不改CPU／平台／remake玩法。

## 原證據

工具d6688b01f7a5306bc6d271e06c4a5eb48a1eb430，[333](333-moo2-menu-table-return.md)已證實原50M與100M表pointer298848／count7／bias0／stride55、完整385bytes SHA-256 776c6e6e61a5b17529ff383cae79a194edc17c7cf0b9a11f3dc8fc8841339183，第2筆原word415／217／567／238。500／229幾何命中第2筆，原PNG顯示位置對應NEW GAME為強推論。舊單次短按實際消費的是更早9筆表全畫面index8，不是新增情境。

原官方1.31 ORION2.EXE SHA-256 4e11be14217b4aafa1839f333bf5eba037f98b0c44e9e4752c96c464c260419f，位址均dosgolem高位LE。原417根檔／MOX.SET、44M Esc／1996-01-01與47850592按下／47851578放開仍保存；日期不是RNG seed。

## 新情境契約

只有新環境DOSGOLEM_MOO2_MENU_READY_CLICK=1啟用；空值關閉，其餘非空值exit2。啟用要求舊NEW_GAME_CLICK_AFTER_DISPLAY40=1、44M Esc、100M cap、1996-01-01、SEPARATE_DOS=1與非空VBE_FRAME_PREFIX。無旗標三條基線行為不變。

在50M既有checkpoint保存後、同一步CPU.Step前，以直接peek核對DS188:26C480 pointer298848／DS:29BE0E count7／bias0、完整385bytes，以及第2筆word415／217／567／238；正常舊press／release已完成、callback mask／target=2Bh／8:2136D1、pending0／非active、IRQ非active／非failed、IF=1且BIOSClock存在。任一不符則明確停止新情境並留錯，不改時點或反覆試點。保存原表、R／六段／EIP／flags與callback／IRQ，直接peek前後RAM／核心保持。

只經原services.InjectMouseEvent送一次x1000／y229／buttons1／delta0,0；固定guest正常座標轉為500／229。不代寫原記憶體、EIP、caller索引或註冊表。新press時間執行前固定為50M。等本次press callback完成、pending0／非active、IF=1與虛擬時間至少20ms後，沿舊契約送一次x1002／y229／buttons0／delta0,0，利用正常移動放開事件；不增加其他點擊。保存兩次正常輸入的時間與driver狀態，原guest最多100M，不調CPU時序。

額外只觀察新press後首個實際20DDDB、66A3A6C42600 word store：保存原R／六段／EIP／flags／16code bytes與DS:26C4A6 word前後、callback／IRQ原計數，依原指令驗store來源及寫回，不把後來正確結果補成預期index2。這個樣本只是原caller選擇消費，不替代玩家可見畫面。保存新情境既有60M..100M與最終PNG／終態；無新CPU拒絕且新遊戲設定畫面實際出現才稱這一玩家步驟完成，未出現則未知／阻塞並依實際指令或畫面追查。

## 驗收與停止線

先READY再實作。原三流程重播只換334輸出名，全部333舊列除既定mtime／DTA四byte／PNG路徑／每次RAM雜湊保持，72PNG逐位元保持。第四獨立ready100M與333獨立100M在額外50M輸入前完全保持，不比較新輸入後不同流程為同狀態。新ready條件表bytes逐位元對接333正式終態，兩正常輸入／callback完成與首store按原bytes核算；新畫面人工確認。

新增參數負例覆蓋非1值、缺舊旗標／44M／100M／日期／SEPARATE_DOS／frame prefix，拒絕必須在讀EXE前。固定Go1.24.13 Docker映像／600s／2GiB／2CPU／128pids／UID1000／network none，原ZIP／patch唯讀重建417檔，不修改原版。

正常開局、種族設定、正式PRNG、remake同狀態與整款遊戲完成仍需後續垂直驗證。新指令錯誤先依原bytes與硬體語料走CPU規格；無錯但玩家流程不前進先沿實際選擇／新UI邊界，不深入renderer／runtime helper，不提高100M cap。CPU／平台未變時325固定EXE全套仍有效。主庫RE-first保持。原版素材／LOG／PNG／RAM留忽略workplace，新規格同次索引000-index，限定CONFORMED後回填333，Docker與擁有權收尾。

READY審查：333已取得50M完整7筆表及第2筆幾何來源，足以指定固定50M後一次正常press／release；原InjectMouseEvent與20ms放開契約沿已驗舊輸入，不需猜driver。增加明示旗標及原表／callback／IRQ條件，不改三條舊基線，尚未知道NEW GAME結果就保留驗收條件。最小store只核對原bytes，不重建callee。四流程／新增參數拒絕與原cap均有明確邊界，足以READY。

## 正式收據與限定結論

限定驗收：正常輸入／第2筆原store／舊基線保持。新遊戲設定畫面未完成，原CPU的新F2 SCASW拒絕是下一修正點，不把本規格標題當新遊戲已完成。

Docker固定Go1.24.13映像sha256:1a6d4452c65dea36aac2e2d606b01b4a029ec90cc1ae53890540ce6173ea77ac，600s／2GiB／2CPU／128pids／UID1000／network none，原ZIP／patch唯讀重建417檔。初次腳本更新輸出輪次時誤改預期EXE雜湊，輸入檢查就停止、未啟動原版；修正腳本後同容器設定乾淨重跑四流程，不當產品缺陷。go build -p 2 -buildvcs=false -o /tmp/moo2-probe ./workplace/moo2-probe；前三條沿333環境換334輸出名各一次，第四條沿獨立100M加DOSGOLEM_MOO2_MENU_READY_CLICK=1，各一次。python3 workplace/new-game-334-verify.py PASS：無旗標全部3847／4829／6769舊列除既定正規化保持333，無旗標全部72PNG逐位元保持；ready額外50M輸入前全部4780原列保持333獨立100M。新輸入後不冒充相同狀態對拍。

**已證實，原正式範圍與正常額外輸入**：50000000前置DS188:26C480原指標298848／DS:29BE0E count7／bias0／stride55、完整385bytes逐位元對接333終態，SHA-256 776c6e6e61a5b17529ff383cae79a194edc17c7cf0b9a11f3dc8fc8841339183，index2範圍415／217／567／238。R／段／EIP21334F／flags216h與333同50M checkpoint吻合；直接peek readonly=true。callback mask2Bh／target8:2136D1、pending0／active=false、started／completed2／2，IRQ8415／8415非活動／非failed，才送一次正常press。

額外press固定50000000、virtual_micros65660599、x1000／y229／buttons1；release實際50011955、virtual_micros65691938、x1002／buttons0，差31339微秒，沿正常20ms契約首次可送時放開，未以舊986 Step推定相同虛擬時長。正常callback3／3後放開，終態4／4、pending0／active=false。兩正常事件只經InjectMouseEvent，不代寫原RAM／EIP或索引。

61538983於20DDDB執行66A3A6C42600，原EAX2、DS188:26C4A6 word0000→0200，下一EIP20DDE1；R／六段／flags297h保持，原store來源與寫回獨立核算。callback4／4、IRQ11675／11675皆非活動，error=nil。正式選單第2筆被實際選中已證實，未只用幾何交集當輸入成功。

**新CPU阻塞已證實**：76658331於原高位LE1F3640、bytes F2 66 AF 8B 45 FC 01 F0 48 2E FF 24 8D 8B 35 1F拒絕；錯誤「F2 prefix 只支援 SCASB／MOVSB／MOVSD」。原起始EAX2／ESP2BDA0C、錯誤後EIP1F3643／ECX9／flags246h、DS／ES／SS188，protected IRQ15961／15961完成。指令是REPNE SCASW，CPU尚未支援；未跑到100M，不能把cap當新情境已通過。StartY512／DisplaySets43的實際最終640×480 PNG人工查看仍全黑；新遊戲設定頁與完整正常開局未知，不能宣稱NEW GAME後續已完成。

新ready PNG SHA-256 1610444d26adb3135e7e933dd44912044bb636728af59c70e614945278d3c622，RGB SHA-256 0b150fd32588b1daca5569992ebe559c0102c837306b1af4c44d35128ec58366，indexed SHA-256 7818f5542a0404157573be6cffc0e0c8e68ce3c0f5d17d07ccdd9313fb700baf。本機文件名workplace/moo2-vbe-334-ready.png，不公開原版素材。

python3 workplace/new-game-334-cli-verify.py PASS：14無效值／缺依賴在讀EXE前exit2、有效情境正對照越過參數閘門後缺原EXE明確失敗。無旗標舊CLI保持，新增情境只接受明示1及固定依賴。

| 本機忽略收據／腳本 | SHA-256 |
| --- | --- |
| workplace/moo2-probe-334-baseline.txt.gz | 89af01a886da66931c694b5de618b1fa9f16e6adb9cae4fac11d7ca9402104bf |
| workplace/moo2-probe-334-click.txt.gz | 4b8eaa813db115852c5f965e7fc7f482fbcb1d76e45bdc55523ae7c67e5916f0 |
| workplace/moo2-probe-334-extended.txt.gz | ac175972abd192de3cf816eb1bf427b77a42c3cf32cd95419cbe81e4c2b09738 |
| workplace/moo2-probe-334-ready.txt.gz | 23c850a0b1f444da6975a300cd890f9d4022a5223db01a2b764617d1df21baa1 |
| workplace/new-game-334-verify.py | 02b8a7e1451cdbf7d608048f61bf8bce900645e809f0f6802faffc446318d229 |
| workplace/new-game-334-parity-tests.txt | 807639d9171e392e4c92623a2623dcb3a3447b5119a2bbbbda7720c0d8c08f6a |
| workplace/new-game-334-cli-verify.py | 10771963a67d238c3528a1cba08f10bcb383c4a8123b5b74794e8428822ab8fe |
| workplace/new-game-334-cli-tests.txt | 5b7e3ab2e595ba8396c74593efeb62daca5ef46729048bcf0625809d4ebd6ac2 |

probe SHA-256 f58f52154c18389dea984d83d746980327529fd5d0150896d34734433efef8d3；CPU 1448f29f24dad35e83575189055dc15dd7ac9db2b3529c6d450f03baf91c97b1。CPU／startup／provider／matcher未改，325固定EXE既有全套仍有效，但不能涵蓋此新拒絕；原版LOG／PNG／RAM留忽略workplace。

下一步只補CPU的F2／66／AF字串指令契約，以硬體語料／公開處理器規格核對ECX／EDI／DF／ZF與定義旗標，再由同一ready情境重生原版收據；不深挖1F3640函式內部，不代寫結果、不增加輸入或提高100M cap。主庫RE-first、正式RNG與remake同狀態保持未知，整款remake／中文化未完成。

71回填函式、既有32／49／25／27／27／34／31／36／31／37／34／35與新增33缺證據負例、--check-ready-menu-spec-backlinks／--check-menu-table-spec-backlinks PASS。workplace/new-game-334-backlink-tests.txt SHA-256 b1e9b6ea218e367510eadf5f1c1a323bd66361af655b8d001c19d4d94d857dc2。原ZIP／patch／EXE／MOX.SET／417檔、CPU／平台來源保持、gofmt／Git差異與新來源／收據1000:1000核對通過，工具root-owned／誤建.md目錄自檢空。有界重播、CLI與驗證容器均已退出移除；一次進度讀取時原容器已自動移除，原PTY隨後回報完成，未重啟或重點。未清理其他專案或映像。沿授權推github隔離分支，原版素材不公開。

## 2026-10-03 後續回填

REPNE SCASW已由規格335接通，見[335](335-cpu386-repne-scasw.md)。同ready情境原掃描與下一MOV已核對；原版正常進入NEW GAME設定頁，100M無新CPU拒絕。本文先前拒絕與黑屏屬歷史收據，保留定位；ACCEPT與完整開局仍未知，不擴張本文驗收。
