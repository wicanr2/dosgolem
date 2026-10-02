# 323：保護模式DOS問號首次搜尋

狀態：**CONFORMED**（僅下列問號搜尋與此次正常caller）
日期：2026-10-03
範圍：CX0／ASCII 8.3／'?'／根目錄或單一目前目錄前綴的FindFirst。不改CPU或主庫玩法。

## 原始證據與公開平台契約

固定1.31 EXE SHA-256 4e11be14217b4aafa1839f333bf5eba037f98b0c44e9e4752c96c464c260419f。[322-moo2-earlier-escape-new-game-continuation.md](322-moo2-earlier-escape-new-game-continuation.md)真實單次正常點擊在47995790步，高位LE229A59／CD21拒絕AH4Eh，CX0，DS188:261692字串save?.gam，DTA188:295828。此前save10.gam成功。原正版ZIP根層417檔實際只有SAVE10.GAM／208000bytes；不能把'?不支援'包成缺檔，也不捏造資料。

[219-protected-dos-findfirst-exact.md](219-protected-dos-findfirst-exact.md)與[261-moo2-dos-findfirst-current-directory.md](261-moo2-dos-findfirst-current-directory.md)已有同服務精確檔名／DTA／錯誤返回。公開主來源：[DOSBox-X WildFileCmp](https://raw.githubusercontent.com/joncampbell123/dosbox-x/master/src/dos/drives.cpp)按8與3位置比較，'?'可略過該位置；[SetupSearch／SetResult](https://raw.githubusercontent.com/joncampbell123/dosbox-x/master/src/dos/dos_classes.cpp)分搜尋樣式與成功結果；[DOS_FindFirst](https://raw.githubusercontent.com/joncampbell123/dosbox-x/master/src/dos/dos_files.cpp)依實際drive搜尋。只採已知平台契約，不追Watcom檔案helper內部，不複製來源控制流。

## 型別與READY契約

只在moo2Profile延伸現存AH4Eh／CX0，仍由有界ASCIIZ與43byte DTA有效性先驗證。問號樣式以替代字元驗證合法ASCII 8.3形狀，另保留大寫原pattern的8／3欄位；每個'?'匹配同一位置的任意字元或短名剩餘空位，其他位置必須相同。SAVE?.GAM含SAVE.GAM／SAVE1.GAM，排除SAVE10.GAM；多字元問號與副檔名問號同規則。'*'、LFN、磁碟／父子路徑、重複前綴、其他屬性與FindNext維持未支援。

DirectoryReadOnlyFiles新增可選ListReadOnlyNames，透過既有os.Root.FS的根目錄讀取，只排除目錄、不搬運素材；候選仍經exactDOSName驗證並由原OpenRead取得metadata，os.Root防穿越及symlink逃逸不放寬。其他非空provider無列舉介面即拒絕，不假裝空目錄。nil provider沿219明示空目錄契約。多候選依大寫8.3名稱穩定排序選首個，列舉順序是platform-spec approximation，不稱原FAT精確順序。

DTA搜尋標頭保存含'?'的pattern；成功結果保存實際候選的名稱／size／UTC DOS日期與時間，沿219 EAX低16清零／CF清除；缺檔沿既有EAX12／CF置位並保留之前結果區。任何不支援輸入、越界、provider錯誤或metadata不良都在DTA寫回前拒絕，其他R／段／旗標保持。缺檔高EAX與保留區是沿219近似，不新增原版wildcard逐byte相同聲明。

## 審查與驗收

原始讀取字串、真正資料集合與公開匹配契約已足夠READY，不需更多遊戲RE。整合測試以真實暫存根檔驗存在／缺檔、問號空位／單位置／副檔名、SAVE10排除、穩定首結果、搜尋header與實際結果區、metadata與鄰接哨兵、目前目錄、舊exact保持、非MOO2／屬性／無列舉provider／錯誤／路徑拒絕與symlink逃逸。

固定EXE全套go test -p 2 -buildvcs=false ./... -count=1須通過。兩44M正常原版無點擊／點擊重生，原版save?.gam處理及實際CF／EAX／DTA／少量caller續行驗證，禁止素材代換或猜測無檔。probe只於真實問號搜尋記before／after核心與DTA及最多12外層caller步，不直接呼叫guest helper。遇新拒絕留原始bytes／狀態並另開窄任務。原始LOG／PNG／素材留忽略workplace。索引docs/spec/000-index.md，219／261／322邊界同次回填，主庫玩法RE閘門保持。

## 正式收據與限定結論

Go1.24.13，映像sha256:1a6d4452c65dea36aac2e2d606b01b4a029ec90cc1ae53890540ce6173ea77ac。600s／2GiB／2CPU／128pids／UID1000／network none；原ZIP／patch唯讀、417根檔乾淨重建、EXE不改。定向測試先因測試包裝型別編譯錯誤退出，修正後同映像／同命令乾淨重跑通過，屬測試程式問題。存在／缺檔、SAVE10排除、空位與副檔名問號、實際結果名稱／metadata／鄰接哨兵、12項未授權輸入拒絕、連結逃逸與已關閉根目錄、nil診斷空目錄、舊精確／目前目錄查詢均通過。

固定EXE的DOSGOLEM_MOO2_EXE=/tmp/game/ORION2.EXE go test -p 2 -buildvcs=false ./... -count=1完整測試通過，CPU386194.255s、machine6.660s；私有workplace/full-test-323.txt SHA-256 a5b1ee8b15de471a587768f875ee5cb1e2761684e66c7d5a5e361dd5e5b53e8a。定向workplace/find-question-323-tests.txt SHA-256 bf27d5772627c0e0e970faf0637809d01056372f175c18a7d0af10a920065080。沒有以單元測試升格原版玩法。

go build -p 2 -buildvcs=false -o /tmp/moo2-probe ./workplace/moo2-probe後，沿322兩44M Esc／1996-01-01／50M cap／separate DOS流程；點擊只加DOSGOLEM_MOO2_NEW_GAME_CLICK_AFTER_DISPLAY40=1。無點擊全部322原始列與PNG保持，只正規化mtime／DTA日期時間四bytes／PNG路徑；點擊到原始save?.gam FindFirst前的全部前綴保持。按下47850592、放開47851578、兩CB返回47850693／47851666、191步與完整核心保持。

**已證實，dosgolem高位LE正常caller**：47995790步，229A59／CD21真正AH4Eh搜尋save?.gam，原ZIP集合只有SAVE10.GAM，實際列舉後無匹配。R264E92／0／261692／295828／2BDB60／2BDB78／FFFFFFFF／2BDC2C，只EAX→12h；六段8／188／188／0／20／188保持，flags246h→247h。DTA188:295828的前12bytes由02534156453130000047414D變為02534156453F00000047414D，保存SAVE?樣式；+0Ch..+2Ah保留之前SAVE10.GAM結果與metadata，不把舊結果誤稱匹配新樣式。

12個caller步47995791..47995802全部error=nil；原版229A5B呼叫24000C，原始73 0E因CF=1不跳，24000E／25 FF FF 00 00保留AX12h並處理錯誤。後續僅保存有界原始核心與bytes，不追檔案helper內部。

正常點擊後再前進1446293步，49442083新拒絕為**dosgolem高位LE**17122B／00 C3 0F BF C2 42 00 1C 06 66 83 FA 08 7D 11 EB，opcode00尚未支援；錯誤後EIP17122C。完整R0／0／1／0／2BDB50／2BDB84／2BDB68／2BDB68，六段8／188／188／0／20／188，flags247h，時計64287801；IRQ7完成427、兩正常mouse回呼完成2且無pending。這是新CPU形狀，不重開已接通的FindFirst或CB。

| 本機忽略收據 | SHA-256 |
| --- | --- |
| workplace/moo2-probe-323-baseline.txt.gz | 966a964554a3fe5ca744f792baf492d1c3dc4e03a902c952174059ac55da1d46 |
| workplace/moo2-probe-323-click.txt.gz | 62b3344c95ed7215ff3602d4399129ba5e58d923f89585ecb98271d856e78ec3 |
| workplace/moo2-vbe-323-baseline.png | 11ec0ed15a4c874d36c94a824af73eb71dc6937dfe3bd568450943861db89927 |
| workplace/moo2-vbe-323-click.png | 59f76749db5232f97a6b6f969f56f848c2e89e731d7e3fb30b8786982bca4a81 |

已實際檢視點擊終圖仍為主選單；設定畫面仍未知。沒有正常開局／remake同狀態收據。CPU SHA-256 538abd53a40d65cc07b33cbeaf272d3541cd6dd4622c4fa17a5d8244a0521a77保持；startup dbdbba06c9616547ad7beaec251f4aceb8a9b07d44ed6ffab2e53911ec7e972c、readonly provider d73b13ae9b0ebc188e5180ff3e8a123d5a7eee2807687bd4f033eace2aeb814a、question matcher 4dad56af8754c488a5cd11b3a923a835b467c4f0574c11c6a2ffdda47ce2fafc、probe feb3b80caa1b220aa6ee3c47856bebb70dda40d1b9a47b45d03186b955ffdbc4。

限定CONFORMED只含公開問號匹配、實際檔案集合、既有DTA與正常caller的此次處理。列舉排序／UTC時間／保留區仍是明示platform-spec approximation，不稱原FAT或DOSBox-X完整DTA逐位元同狀態。219／261／322回填323入口，startup_probe_131.py --check-menu-event-find-spec-backlinks核對320–323與較早邊界。下一步依公開CPU契約補00 C3的byte ADD，先READY再實作，沿原44M單次正常輸入重生，不加cap、重點或代寫原版狀態；主庫玩法RE閘門保持。

60個回填函式、原有32負例與320–323新增49負例及兩CLI通過。私有workplace/menu-event-find-323-backlink-tests.txt SHA-256 284f35860025eabeff19d5c68e01b025adc0225b31aac30558b6e49c18f88350。索引正對照先驗已收錄的316，再驗320–323與較早連結；移除原始定位／收據／限定狀態／索引或回填均拒絕。

byte ADD停點已由規格324接通，見[324-cpu386-add-byte-register-memory.md](324-cpu386-add-byte-register-memory.md)。真正一暫存器／七記憶體ADD與JGE兩方向已驗；新記憶體NEG拒絕另列，原搜尋與CB限定收據保留。
