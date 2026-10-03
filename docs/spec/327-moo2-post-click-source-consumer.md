# 327：NEW GAME後段實際來源與最小消費

狀態：**CONFORMED**（僅六組實際來源／分支與原值目的寫回，其他限制見收據）
日期：2026-10-03
範圍：固定1.31正常單次點擊的唯讀資料流，不改CPU／平台／主庫玩法或輸入。

## 基線與問題

工具ca437d1b17135d511b52e7fb40591b7ea12176c4，官方EXE SHA-256 4e11be14217b4aafa1839f333bf5eba037f98b0c44e9e4752c96c464c260419f。原417根檔／MOX.SET、44M Esc／1996-01-01／50M cap／separate DOS與正常單次NEW GAME沿[326](326-moo2-post-click-progress-observation.md)。私有workplace/moo2-probe-326-click.txt.gz SHA-256 fa3d71704a13552e22ef0b04b5700afc9264934ed709a1c8134b5bb8b76782dc。

326已證49500000至50M六取樣畫面與VBE計數相同、R／堆疊／RAM變化；設定畫面未知。原版末尾32步真正來源為高位LE213345／8B 15 74 BE 29 00讀DS:29BE74 dword、21334B／01 D0、21334D／8A 00讀DS:[EAX] byte、21334F／88 45 F8寫SS:[EBP-8]，再到2132E0..2132EA的讀取／CMP／JE與2132FA的JLE。另見213336／88 02寫DS:[EDX] byte。來源指標與目的窗口未另取樣；ESI零窗口不是這組來源，不推素材故障或完整renderer語意。

## 唯讀契約

既有VBE_FRAME_PREFIX非空、正常NEW GAME已按下及放開、outer_step≥49500000時觀測。僅在213345、觀測閒置時以直接唯讀描述子與RAM看DS:29BE74四byte及預期base+EAX來源byte，用uint32位址繞回與既有段／RAM界限。來源byte預定分成<80h／=80h／>80h三類，各只取前兩次閒置命中，共最多六組；不可讀另只取首筆並明示。條件執行前固定，不挑通過的結果、不重播換輸入或seed。

每組記開始213345真正CPU.Step與最多32個實際續行。完成第一次213336寫回或下一次即將回到213345時停止該組；若先遇cap／原版拒絕，以當時已執行資料與未知收尾。續行可能少於32步，不注入靜態指令或代跑helper。

保存每步完整前後R／六段／flags／EIP與16指令byte、錯誤、真正DS:29BE74四byte、首個base／index／source_offset與五byte來源窗口、首個SS:[EBP-40h]起80byte堆疊窗口。窗口只讀RAM、先驗selector／limit／linear／underflow界限，不走CPU hook、不寫guest。在真正213336前後另記DS:[EDX]五byte目的窗口及offset，核對只改byte與鄰接保持；其他步目的窗口明示未取，不猜輸出位址。相同來源窗口出現alias時按真正位元組記錄，不預設永遠不變。

分類用執行前真正RAM，之後必須與原始MOV實際結果核對，不能只信預讀。MOV不改flags；ADD／CMP用獨立較寬加減與六旗標核對，JE／JLE以真正flags與去向核對。XOR的未定義AF只保留工具清除近似，不稱原版硬體AF exact。來源／條件／目的寫回有充分證據才分別標已證實；未到的類型／路徑保持未知。輸出RAM與VBE畫面發布分開，不把實際RAM寫回當設定畫面已顯示。

## 審查與驗收

只改workplace/moo2-probe/main.go唯讀觀測與證據守衛，CPU／平台與326逐位元保持、325固定EXE全套PASS沿用。go build後以同326兩流程乾淨原417根檔輸入重生。只移除新增post_click_source_consumer／post_click_source_totals列，僅mtime／DTA日期時間四byte／PNG路徑正規化；兩流程全部326原始列、六張快照與終圖保持，包含完整前後核心、計數、IRQ、CB與VBE。

獨立核對開始global指標→ADD地址→MOV真正byte→SS局部寫回→CMP／JE／JLE與實際目的寫回；所有取樣碼只改probe自身狀態，不能產生額外guest讀請求。來源與目的範圍、flags、完整R／段與原始定位都必須可回查；未觀測項保持未知，遇缺口另開窄任務，不追整個helper或renderer、不加cap／重點／先調輸入，不寫新的玩法規則。

新規格同次掛入docs/spec/000-index.md；成功後回填326真正來源待辦與守衛，保留歷史六時點與50M收據。原版LOG／PNG／RAM留本機忽略workplace，不公開原素材。主庫RE-first閘門、255游標、303整體DRAFT、299自然OF=1、AH2Ch／RNG／人耳與正常開局／remake同狀態未知保持。

READY審查：326真正尾跡已定位global MOV、ADD、byte MOV、SS局部store與CMP／JE／JLE、目的byte store，足以界定最小輸入／消費而不命名整個helper。直接描述子／RAM窗口與最多六正常類型／一不可讀組不走CPU hook，條件與預算執行前固定；精確原始opcode與完整兩流程基線驗收防止預讀冒充實際消費。只觀測不改平台，325全套沿用；正式CONFORMED仍須來源、分支與必要目的RAM收據，不把byte分類當正式玩法。

## 驗證範圍審查與正式收據

審查勘誤：初次獨立比較在六筆post_click_progress的完整RAM雜湊不同而停止；各次前後雜湊仍相同，R／段／flags／時計／窗口／VBE／全部其他列保持。未保存跨次完整RAM差異，不能確認差異來源，也不宣稱跨次全部RAM相同。驗收改為明示正規化每次RAM雜湊，逐筆檢查ram_before_sha256=ram_after_sha256與readonly=true；這只驗證既有六快照不突變。新來源觀測的copy方向／指標取值／探針局部狀態另經程式審查，沒有guest寫入或額外Bus／hook請求。此修正不改原版輸入、CPU或平台，不把失敗寫成產品缺陷。

Go1.24.13映像sha256:1a6d4452c65dea36aac2e2d606b01b4a029ec90cc1ae53890540ce6173ea77ac，600s／2GiB／2CPU／128pids／UID1000／network none。原ZIP／patch唯讀、乾淨417根檔與官方EXE沿326，go build -p 2 -buildvcs=false -o /tmp/moo2-probe ./workplace/moo2-probe，再/tmp/moo2-probe /tmp/game/ORION2.EXE --game-dir /tmp/game。兩流程環境沿326完整值，只換327輸出名；44M Esc／1996-01-01／50M cap／separate DOS、同47850592按下與47851578放開不改。固定日期不是RNG seed。

獨立python3 workplace/post-click-source-327-verify.py PASS：無點擊全部3847列、點擊全部4208列，除mtime／DTA日期時間四byte／PNG路徑／每次RAM雜湊外保持326，六點各次前後RAM相同與readonly=true。兩終PNG及六張後段PNG逐位元保持326；沒有新CPU拒絕。CPU／startup／provider／matcher與325逐位元保持，325固定EXE全套PASS仍有效。本輪160個真正CPU.Step保存完整連續R／六段／flags／堆疊；其中90步以獨立位址／MOV／算術／六旗標／分支／窗口預期核對，剩70步只保存原始續行，不稱全部160步已獨立解碼。XOR／AND的AF沿工具清除近似，不宣稱硬體未定義旗標exact。

**已證實，固定正常單次點擊的有界原版**：六組DS188:29BE74原始D4A33D00，指標3DA3D4h；真正213345→21334B→21334D讀取各自base+index，AL結果與預讀來源byte相同，21334F寫SS:[EBP-8]。

| 組 | 開始外層步 | 原始index | DS來源offset | byte | 真正步數 | 停止 |
| --- | --- | --- | --- | --- | --- | --- |
| 1 | 49500045 | 3993h | 3DDD67h | 02h | 27 | 213336目的寫回 |
| 2 | 49500076 | 3994h | 3DDD68h | 82h | 20 | 返回213345前 |
| 3 | 49500096 | 3995h | 3DDD69h | 02h | 27 | 213336目的寫回 |
| 4 | 49500158 | 3997h | 3DDD6Bh | 80h | 33 | 預定budget，2132DB→2132DD |
| 5 | 49500249 | 399Ah | 3DDD6Eh | 82h | 20 | 返回213345前 |
| 6 | 49500331 | 399Dh | 3DDD71h | 80h | 33 | 預定budget，2132DB→2132DD |

各類只取前兩組、總groups=[2 2 2 0]、active=false／remaining=0，無不可讀樣本。CMP EAX,80h後JE四次不跳至2132F0、兩次跳至213354；JLE兩次跳至21330B、兩次不跳至2132FC，皆按真正flags獨立核對。82h經兩次AND變成2h，再213306／01 45 C0使SS:[EBP-40h]的局部dword加2；不替局部值命名玩法欄位。80h分支在33步預定上限結束，後續發布沒有驗證，不延長budget追整個helper。

兩個02h樣本真正213336／88 02將AL=FDh寫DS188:499300h及499303h。五byte窗口分別FDFDFD00FD→FDFDFD00FD、00FDFDFD00→00FDFDFD00；中心原本就是FDh，寫入與鄰接保持通過，不能稱目的值發生改變。213330／8A 80 7B BE 29 00的映射表來源未另取樣，僅保存實際EAX／AL續行，不由02h猜完整素材／renderer語意。來源窗口與目的窗口都不是VBE發布成功的證據。

六張後段畫面仍PNG 59f76749db5232f97a6b6f969f56f848c2e89e731d7e3fb30b8786982bca4a81，DisplaySets42／Writes16194454保持，設定畫面仍未知。末態與CB／事件／搜尋／NEG保持326，原版正常開局與remake同狀態未完成。

| 本機忽略收據／腳本 | SHA-256 |
| --- | --- |
| workplace/moo2-probe-327-baseline.txt.gz | c781b232058e3ca2c157a14c685ee44c12db851c00c6ed97d7f92344e9377d45 |
| workplace/moo2-probe-327-click.txt.gz | 6e46e29082ceff5cdcab14668c113b89ac8e43f0ba19f7ef2e562e613db564a5 |
| workplace/post-click-source-327-verify.py | f994ed0fabea7ad5eca2e51acc088c3988443670952be3258ec3f6990ab89d77 |
| workplace/post-click-source-327-parity-tests.txt | 15fef4b3ed0dd796418a71a1cc2c52bb1826373392ff665f7c8bd27516957664 |

probe SHA-256 73ff02f6b4885207c029c99efa1c1920053e03b3fb2cbafc53db032ca2aef3d2；CPU仍1448f29f24dad35e83575189055dc15dd7ac9db2b3529c6d450f03baf91c97b1，startup dbdbba06c9616547ad7beaec251f4aceb8a9b07d44ed6ffab2e53911ec7e972c，provider d73b13ae9b0ebc188e5180ff3e8a123d5a7eee2807687bd4f033eace2aeb814a，matcher 4dad56af8754c488a5cd11b3a923a835b467c4f0574c11c6a2ffdda47ce2fafc。

限定CONFORMED只含六組來源／分支、90步獨立核對、兩次原值寫回與已列觀測保持。完整RAM跨次一致未知、70步未獨立解碼、80h後續與映射表未驗；不追完整helper／renderer。下一步只沿既有同輸入50M流程，定位目的RAM的最小畫面發布契約與是否已被消費，先查現有VBE服務與正常trace，未發現實際玩家阻塞不擴大RE。禁止把RAM寫回稱新遊戲設定完成，主庫玩法RE閘門與AH2Ch／RNG／人耳未知保持。

64回填函式、原有32／49／25／27／27與327新增34缺證據負例、兩CLI PASS。workplace/post-click-source-327-backlink-tests.txt SHA-256 fc4f283990fae78884bce4c74056d1ed572d660118fb46b5581ae1c7c62b748a。新來源與本機收據UID:GID1000:1000，工具root-owned／誤建.md目錄自檢空；CPU／平台來源保持，不清理其他專案。
