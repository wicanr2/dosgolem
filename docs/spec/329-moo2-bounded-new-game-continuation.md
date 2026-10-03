# 329：正常單次NEW GAME的獨立100M有界續行

狀態：**CONFORMED**（僅獨立100M預算／50M保持與唯讀觀測）
日期：2026-10-03
範圍：原版探針預算與唯讀觀測，不修改CPU／平台或remake玩法。

## 已有證據與目的

工具963a57228f429b8e028570f9d4c1c9cfddf16837；固定官方1.31 EXE SHA-256 4e11be14217b4aafa1839f333bf5eba037f98b0c44e9e4752c96c464c260419f。原417根檔／MOX.SET、44M Esc／1996-01-01／50M cap與單次正常NEW GAME沿[328](328-moo2-post-click-publish-monitor.md)。既有50M後段CPU仍正常執行、沒有新拒絕，RAM有真正來源／寫回但VBE提交差額0；不能由探針上限推CPU、素材或整個renderer故障。

本輪保留原50M正式收據，另外從原入口開始跑最多100M真正Step，讓正常輸入後有更多執行預算。這是獨立有界觀測，不改遊戲規則、時間模型、亂數、初態或正式玩家輸入，不恢復主庫玩法RE閘門，也不稱更晚終態與50M同狀態。

## CLI契約

既有DOSGOLEM_MOO2_MAX_STEPS預設8M不變，明示範圍1..100000000。正常NEW GAME只接受既有44M／46M Esc+50M cap，或新增44M Esc+100M cap；其餘點擊預算／Esc組合仍拒絕。1996-01-01、無早期滑鼠注入與正常按下放開限制保持。100M不追加任何點擊、重試、狀態注入或seed；固定日期不是RNG seed。

只調probe的整數上限、NEW GAME預算閘門與錯誤文案；不改CPU／DPMI／PIT／IRQ／VBE／DOS服務。保持先檢查CLI再讀原EXE。CLI負例覆蓋零／負值／非數字／100000001／click51M／click99M／46M Esc+100M／錯日期／早期滑鼠／無硬體Esc；原50M兩組與新100M正常從真實EXE驗正例。

## 唯讀觀測與驗收

既有328的50M兩正常流程重跑，全部舊列與六後段PNG／兩終圖按既定正規化保持；原50M收據檔名保留不覆寫。再另外正常100M點擊流程，只有cap與輸出名不同，精確比較同50M前綴：原輸入、IRQs、CB、事件、搜尋、來源／分支、VBE與50M核心必須保持。不把不同terminal計數混比，不猜續行必然成功。

只在100M模式與既有phasePrefix非空、正常點擊已放開時，於50M、60M、70M、80M、90M、100M真正單步前唯讀抓圖與核心快照，最多六筆；沒到不抓。不改舊六點觀測。保存完整R／六段／EIP／flags／FPU、VBEState與時計，索引／RGB／PNG與RAM前後SHA，先後所有快照讀前後狀態相同與readonly=true才寫通過。100M cap終態若無拒絕另抓，提前原版拒絕按既有step_error與終圖收尾，不補指令或再加上限。原版LOG／PNG／RAM留本機忽略workplace。

CPU／平台來源與328逐位元保持，325固定EXE全套仍有效。go build後以唯讀ZIP／patch乾淨重建417輸入，50M未點擊／點擊與100M點擊三流程各只跑一次，預定600s／2GiB／2CPU／128pids／UID1000／network none。只做最小充分比較與實際查看原版圖，未到的點與正常開局／remake同狀態保持未知。實際CPU／平台拒絕才開下一窄切片，不深挖完整helper。

新規格同次索引000-index，成功後回填328續行入口與守衛。CONFORMED只限新預算閘門、舊50M保持與有界觀測，不等於完整新遊戲／整款remake與中文化完成。

READY審查：maxSteps只控制探針迴圈與CLI，不送入CPU／時計／原版狀態；新100M只放行44M Esc與既定單次輸入。50M獨立模式程式分支保持，增加快照只在100M生效。較長流程於50M保存核心、VBE／RAM與Bus計數快照，對既有50M前綴與終態核對；不同長度terminal統計不得逐列混比。PNG／RAM與FPU讀前後核對沿326既有契約，最多六點，提前拒絕不補步。正式規則與素材不改，足以READY實作；結果不預定為成功開局。

## 正式收據與限定結論

Go1.24.13固定映像sha256:1a6d4452c65dea36aac2e2d606b01b4a029ec90cc1ae53890540ce6173ea77ac；600s／2GiB／2CPU／128pids／UID1000／network none，原ZIP／patch唯讀重建417根檔。go build -p 2 -buildvcs=false -o /tmp/moo2-probe ./workplace/moo2-probe後，沿328兩組50M正式環境換329輸出名；另加同44M Esc／1996-01-01／單次正常點擊與MAX_STEPS=100000000，輸出329-extended。CPU／平台來源保持，325固定EXE全套PASS沿用。

14 CLI負例PASS，皆exit2且在EXE讀取前拒絕；三個真實EXE正常流程驗正例。獨立python3 workplace/new-game-329-verify.py PASS：50M未點擊全部3847列、點擊全部4382列除既定mtime／DTA四byte／PNG路徑／每次RAM雜湊外保持328，六後段PNG／兩終PNG逐位元保持。100M同50M前綴4335列保持，只另正規化明示max_steps預算；50M checkpoint核心／VBE／Bus／CB與原50M終態吻合。不同長度terminal計數不混比。

**已證實，固定1.31原入口、同單次正常輸入的獨立100M流程**：按下47850592／放開47851578與虛擬時間保持，沒有追加輸入；六點全部readonly=true、RAM前後相同、Bus身分保持、完整FPU控制127Fh／status0／depth0／八槽0。兩CB已開始／完成2、callback_samples191維持，沒有新CPU拒絕；最終step_limit=100000000、高位LE213321／flags297h／unique_sites27018。正常新遊戲仍未知。

| 外層步 | 高位LE EIP | 虛擬µs | BankSets | Writes | DisplaySets | Bus VBE累計 |
| --- | --- | --- | --- | --- | --- | --- |
| 50000000 | 21334F | 65660599 | 797 | 16194454 | 42 | 0 |
| 60000000 | 213311 | 83672515 | 797 | 16194454 | 42 | 0 |
| 70000000 | 2131BC | 102038657 | 802 | 16207094 | 42 | 12640 |
| 80000000 | 213239 | 120423628 | 807 | 16220318 | 42 | 25864 |
| 90000000 | 21332A | 138808801 | 812 | 16233406 | 42 | 38952 |
| 100000000 | 213321 | 157193966 | 817 | 16246494 | 42 | 52040 |

終態Bus監測50500000 Step、讀444340260／寫54307571／errors0；target_reads=[2 2]、target_writes=[446 423]、source_reads19481，VBE52040與Writes差額52040吻合。328的50M區間零讀回／零提交只限當時，較晚已發生讀取和顯存寫入，不能延伸成永不發布。Bank4／StartY0維持，不必換頁才能修改目前顯示區。

六PNG經CRC與640×480RGB解碼／記錄SHA核對。相鄰不同像素0／186／408／484／891，變化都在x66..273、y414..422。50M與100M兩圖已實際檢視，主選單六按鈕與NEW GAME游標仍在，下方由空區變成Game Design／Steve Barcia致謝。最終PNG0ff69fc4f60431f01fb2dcadfbc7ee97b5378e87f0bdda3ff7eeff8fe2d4e363，畫面確有變化，設定畫面仍未知。這些不是完整renderer正確或按鍵指令已被激活的證據。

| 本機忽略收據／腳本 | SHA-256 |
| --- | --- |
| workplace/moo2-probe-329-baseline.txt.gz | 553cca0bcd9ba0b44bb2284877345efa1f0c6f25bb85354ee64bb1514f150bc4 |
| workplace/moo2-probe-329-click.txt.gz | 07aa81ac86c5e142fc11c99530907424241aadd320800c9a855625a67ab78f64 |
| workplace/moo2-probe-329-extended.txt.gz | c7dc2bb37a5292f588d3027cc7d99ac1e682d618bf8259dfc067bb6f533793c5 |
| workplace/new-game-329-cli-tests.txt | 365aaaa3a75ae05c12a430aa58310fd86ee8897e9444cc9bf931af4a95021989 |
| workplace/new-game-329-verify.py | 1e10a48db678caf3ed6a40ba4c6021fc6f8012729928a0c65dfc853ae240902c |
| workplace/new-game-329-parity-tests.txt | 8c041b02e2ac4594e2c3db9d0e47ee706bc3badd65d3c71d458e155796b97b98 |

probe SHA-256 83154a870ee955744d847c26eb25ba94404eb718ec4ffcdeec5984e7a27a89bf；CPU仍1448f29f24dad35e83575189055dc15dd7ac9db2b3529c6d450f03baf91c97b1，startup／provider／matcher／VBE來源與328逐位元保持。原版素材／LOG／PNG／RAM留本機忽略，不公開。

限定CONFORMED只含預算閘門、50M兩基線／100M前綴保持與六點唯讀觀測。已見較晚Bus讀回、VBE寫入和致謝文字變化，沒有新的CPU拒絕，下一步改沿實際按下／放開追最小NEW GAME指令激活與原事件讀取時間，先確認它是否真正離開主選單；不繼續增加預算、不追整個renderer或由綠測試猜玩法。主庫RE-first、255完整游標、303整體DRAFT、299自然OF=1、AH2Ch／RNG／人耳與remake同狀態仍未知。

66回填函式、原有32／49／25／27／27／34／31與329新增36缺證據負例、兩CLI PASS。workplace/new-game-329-backlink-tests.txt SHA-256 f2538841315d8d53db30eb0aaee340e72eb741bc32b5fe39c9b05eb1d9c6bb9d。CPU／平台來源逐位元保持，gofmt通過，所有新來源／收據與PNG1000:1000，工具root-owned／誤建.md目錄自檢空。
