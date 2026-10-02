# 316：MOO2 明示硬體 Esc 排程

狀態：**CONFORMED**
日期：2026-10-03
範圍：自製原版探針的輸入時點設定；不改CPU／平台／主庫玩法。

## 基線與證據

工具f1c2fa57675991080e2da3d1f5008b9b49f209bc，固定1.31 EXE SHA-256 4e11be14217b4aafa1839f333bf5eba037f98b0c44e9e4752c96c464c260419f。315兩48M Esc／50M cap正常流程仍在滑入，最後高位LE0x23856E；第21換頁49967220步／62395438µs的面板部分可見，未有完整主選單輸入收據。再次同設定重跑不能增加觀測時間。

[307-moo2-protected-keyboard-irq1.md](307-moo2-protected-keyboard-irq1.md)已READY實作並限定CONFORMED：QueueHardwareScan只排controller01／81，原版IRQ1讀60h、20h EOI、CF返回，沒有宿主代寫遊戲狀態。AttachMachine安裝控制器；m.Keyboard在預設BIOS鏈首次處理才建立，因此舊keyboard_installed=false不表示原版IRQ1向量未安裝。既有AH2509成功保存DS8／EDX21C4D8；不新增BIOS注入、換鍵、直接呼叫handler或調時鐘。

## 型別與契約

- 新明示DOSGOLEM_MOO2_HARDWARE_ESCAPE_STEP為十進位正整數，1≤step<maxSteps；空值保持無新排程。非數字／正負符號／空白／溢位／0／step≥cap拒絕並exit2，在讀EXE前完成驗證。maxSteps原預設8M、明示1..50M契約不改。
- 與舊DOSGOLEM_MOO2_HARDWARE_ESCAPE_AT_48000000=1互斥；兩者同設即拒絕，不默默覆蓋。舊開關仍在48M排01／81；若舊cap不到48M，保持既有「未到排程」行為。兩者皆無時不新增鍵盤輸入。
- 新設定只在執行前決定一個outer_step；到該步排一次01／81到既有QueueHardwareScan，後續IRQ1／IF／PIC／框架／原指令都沿現存模型。不選擇通過結果再改時點。排程收據記錄step／設定來源／maxSteps；舊設定不新增此列，完整既有收據保持。
- 315換頁快照門檻在有設定排程時沿選定step，無設定則保持48M；最多16張，仍只讀成功4F07換頁。既有50M cap與虛擬日期／裝置時計／亂數條件不改，不把輸入時點變更當成相同初態。
- 本輪先固定新46M與既有48M兩个條件，不做搜尋成功時點。兩46M日期流程分別有／無既有受控滑鼠；第三48M／無日期控制流程。另重生48M／日期／無事件基線，逐列與315保持。完整主選單是否展開由實際PNG決定，不預先要求換鍵或加碼讓它成功。

## READY審查

新排程只是307已證入口的有界輸入參數，316不用新的driver或玩法推論。315圖像已證到上限仍有動畫進展，較早Esc作獨立初態能在同cap內觀察更長續行；成功與否都保留，與48M基準分開。已核對現存QueueHardwareScan／refill、原始307讀取／返回、maxSteps以及315只讀門檻，足以READY。

驗收：先建一次probe執行檔，以不存在EXE檢查八類非法設定與衝突都在讀檔前exit2；真正四原版流程，明示46M輸入／IRQ1兩碼與自然原caller、48M前基線保持、三流程的各自日期／cap、原始圖像與狀態。CPU／平台來源不改，314全套仍適用，不為CLI變更重跑硬體語料。正常新路徑遇缺件即記原始bytes／狀態，保持預定輸入不改。

索引入口docs/spec/000-index.md。315部分滑入／下一步排程的現行未知須同次回填；307原48M限定範圍不擴成完整鍵盤或玩家操作。原素材／收據／PNG只留忽略workplace。主庫玩法RE閘門保持。

## 實作與正式驗證

只改workplace/moo2-probe/main.go，SHA-256 57399685a537099ed8871151e9d79d07c4570d94a1f3f498a77efcd0e60efdaa。排程在EXE讀取前驗證；到選定步仍只呼叫QueueHardwareScan。CPU／平台／主庫玩法來源保持，不新增BIOS代寫或handler捷徑，314全套PASS仍適用。

一次go build -p 2 -buildvcs=false -o /tmp/moo2-probe ./workplace/moo2-probe，11個真實CLI負例PASS：0、負數、正號、空白、文字、小數、溢位、等於cap、超過cap、未設cap的8M預設超界，以及與48M舊開關衝突。每個都使用不存在EXE、要求exit2、stdout空、明確錯誤，證明在讀取EXE前拒絕。workplace/moo2-316-escape-config-tests.txt SHA-256 55f6fa2f53c58f507b95dd9aa62a7e935da4050e0649da4d9a3fd01c041f8379。

Go1.24.13固定映像，600s／2GiB／2CPU／128pids／UID1000／network none，原ZIP／patch唯讀，417根檔與官方1.31 EXE乾淨重建。命令沿315，輸出316；前兩日期流程以DOSGOLEM_MOO2_HARDWARE_ESCAPE_STEP=46000000替代舊48M開關，第二含既有受控滑鼠，第三48M無日期控制；最後新增48M／日期／無事件legacy-baseline。四個go run -buildvcs=false ./workplace/moo2-probe /tmp/game/ORION2.EXE --game-dir /tmp/game完成。條件都先固定，沒有失敗後換鍵、挑時點或增加cap。

## 真正正常輸入與畫面

兩46M流程皆hardware_keyboard_schedule step=46000000 source=explicit_environment max_steps=50000000。排隊前全部列除新設定行／mtime／DTA四bytes外，與315對應初態的原始前綴逐列保持。

第46000000步，dosgolem高位LE0x257BC3，排01／81。初始R68／FFFF767E／7159A4／381765／2BDB38／8／70DF34／71BAB0，六段8／188／188／0／20／188，flags202h。01與81由原版8:21C4D8，各97／77步到8:21C573的CF，實際讀60h及20h EOI各一次，started2／completed2，沒有直接呼叫。正常外層下一EIP257BC8、再257BCA，真正原caller繼續，raw_2a42ac首兩bytes仍1B01，沒有宿主代寫。只記可回查的原始核心與返回，不替此caller補用途名稱。

兩皆到50M上限0x2385AC，無step_error／guest_cpu_stop。完整R361BC4／0／95／0／2BDB10／2BDB68／4FF3A0／361DD8，六段8／188／188／0／20／188，flags246h，時計64282188，IRQ0 started8401／completed8401／failedfalse，IRQ7 started427／completed427。裝置irq7_deliveries428另含16位傳輸。兩核心／時計／IRQ／裝置一致，有／無既有受控回呼1／0與allocator selector差異保留，不叫相同初態。

兩VBE Bank2／StartY0／BankSets848／Writes15212758／DisplaySets40，indexed SHA-256 a9cc1f1a678f5ca7a9e995796d38b62b70172ba666e87e454c7f9256e18567a9。兩PNG逐位元相同87fabf21f7be22d264f83390bdb4d39f09098516191c3a17c51815452856645e，已實際檢視主選單六個按鈕CONTINUE／LOAD GAME／NEW GAME／MULTI PLAYER／HALL OF FAME／QUIT GAME文字完整可見；正常點擊與動畫是否完全停穩尚未驗，不冒稱已進入新遊戲。

兩46M各16張換頁快照8..23；既有48M十四張與無日期一張，共47張全部readonly=true、PNG SHA核對。兩46M序列相同；第8..21的索引／RGB／PNG與315對應頁號保持，步數／時計與VBE寫入次數依不同排程各自記錄，不混為同狀態。新第23換頁48135144步／58230840µs尚在滑入；完整按鈕是50M終圖所見，未擷取第24..40逐張畫面，不猜哪一張最早可正常點擊。

新48M legacy-baseline每一原始列，除mtime／DTA四bytes／PNG路徑外完整保持315全流程，包括十四張phase、完整核心／所有輸入／IRQ／時計與VBE。無日期控制亦逐列保持315的0x240A32拒絕；沒有新設定行或默認日期。兩基線終圖分別維持8f7791ae57649991fbf9bf3a86fdacab602e792f39a9b3d57ae691484a754d47與1610444d26adb3135e7e933dd44912044bb636728af59c70e614945278d3c622。

| 本機忽略原始收據 | SHA-256 |
| --- | --- |
| workplace/moo2-probe-316-full-game.txt.gz | 06d0fe6d2cb2e4bfa676ab67a514f5165a33e4cfc6d458e83975976626108209 |
| workplace/moo2-probe-316-mouse-event.txt.gz | c820da5c2d3e2f11be4d1fe09932d2880210a7d3946c13b8a6183b7ba78d337a |
| workplace/moo2-probe-316-unconfigured.txt.gz | e4cef3b724b933508b331a6746c3ec11bbfc7464d126a30f14b76801329363a3 |
| workplace/moo2-probe-316-legacy-baseline.txt.gz | 32306c41cd16d801f2dd99cca5c03b7584e1d2406eabf1793d8e0c6412d83a60 |

## 限定結論與回填

明示Esc排程已由規格 316 接通。315保留48M滑入的原始收據，追加46M不同初態的正常輸入與六個按鈕文字已見收據，不把原48M覆寫成完整主選單。apps/moo2/tools/startup_probe_131.py --check-escape-schedule-spec-backlinks核對預定排程／真正IRQ1／完整終態／四收據與315回填。

限定CONFORMED只包含CLI閘門、新預定排程與上述原版正常返回及可見六按鈕。307仍只涵蓋Esc，255完整游標、303整體DRAFT、299自然OF=1、AH2Ch／RNG、人耳、主選單實際操作／正常玩家路徑與remake同狀態未完成，主庫玩法RE閘門保持。下一步從已見的NEW GAME按鈕定單次滑鼠正常輸入契約，先核對第40換頁的實際步數／可點條件及座標來源，READY後驗原版回呼／自然畫面轉移；不猜熱區或代寫遊戲狀態。

同次58回填函式、26新增缺定位／真正返回／終態／四收據／315回填負例與CLI通過。所有受驗來源與原始收據雜湊保持，47張快照和修改檔1000:1000，工具root-owned／異形.md目錄空。21:00:17 UTC本輪有界容器已退出移除；剩餘Go容器掛載皆屬hr，保留未操作。沒有清理其他專案或映像，沒有失敗的產品測試。
