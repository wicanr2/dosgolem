# 326：NEW GAME後段唯讀進度觀測

狀態：**CONFORMED**（僅六時點唯讀觀測與完整325基線保持）
日期：2026-10-03
範圍：固定1.31正常單次NEW GAME後的有界觀測，不改CPU／平台／玩法或輸入。

## 基線

工具31ca939e73203bafa2f55d7d23ae6115e9dc54c2；官方EXE SHA-256 4e11be14217b4aafa1839f333bf5eba037f98b0c44e9e4752c96c464c260419f。原417根檔／MOX.SET、44M Esc／1996-01-01／50M cap／separate DOS與單次正常點擊沿[325](325-cpu386-neg-dword-memory.md)。325三次NEG／九步消費已驗；點擊到50M、高位LE21334F，無新CPU拒絕，設定畫面未知。私有workplace/moo2-probe-325-click.txt.gz SHA-256 5c269c7fbd6360ad5248da763e2f57f302ea3b837e48a41d1896cb73a16a1d7b。

既有325收據已見：點擊最後兩次成功換頁為DisplaySets41／42；50M時DisplaySets42、Writes16194454、BankSets797。未點擊最後DisplaySets54、Writes19500622、BankSets831。這些是不同正常輸入流程，不當作同初態畫面parity；不能由不同換頁數推新遊戲完成、掛起或CPU錯誤。

## 唯讀契約

只在既有DOSGOLEM_MOO2_VBE_FRAME_PREFIX非空、正常NEW GAME已按下並放開時擷取。外層單步前固定49500000／49600000／49700000／49800000／49900000及上限終態50000000，最多六張，不增加環境參數。未到取樣點就不擷取，較短cap安全；錯誤依原探針停止，不加步數／重點／種子或代寫資料。

每筆保存外層步數、dosgolem高位LE EIP／16bytes、完整R／六段／flags、IRQ時計、VBEState、索引／RGB／PNG／完整RAM SHA-256。直接唯讀SS:[EBP-38h]起64byte窗口與DS:[ESI]16byte窗口，先驗描述子、limit、RAM邊界，不呼叫CPU讀hook；不可讀明示，不猜用途。堆疊偏移與未知byte保留原始定位，不以欄位名替代。

僅在49500000之後、真正CPU.Step前計數實際EIP，每100000步區段保存前十個熱門位址及次數，次數降序／位址升序，結果決定性；最後區段到有界上限。首次取樣尚無區段指令，計數零。這是有界觀測，不深入任何helper控制流。計數只改探針自身map，不改原seen、CPU、IRQ或服務。

沿315／317的VBEIndexed／VBERGB副本與PNG寫法，檔名使用既有prefix加-post-click-<步數>.png，留本機忽略workplace、不建新目錄。讀取前後核對八R／六段／EIP／flags、完整FPU、VBEState與完整RAM雜湊相同，否則失敗；不稱整機時序或亂數完全對齊。

## 審查與驗收

只改workplace/moo2-probe/main.go觀測與證據守衛；CPU／平台來源逐位元保持，325固定EXE全套PASS沿用。新probe正常go build編譯後，以同325兩流程乾淨原417檔輸入重生。移除新增post_click_progress列，僅正規化mtime／DTA日期時間四byte／PNG路徑，兩流程全部325原始列與終圖須保持，不僅比較末尾；原NEG／CB／事件／問號搜尋也保持。

六張PNG與每筆索引／RGB／PNG／RAM雜湊、唯讀標记、區段計數核對。至少首／末畫面實際檢視，量測逐點差異與VBE換頁／Writes，區分原版畫面可見轉移、內部狀態變化及未知。單一首末圖與指令計數不證明loop正確或新遊戲必定完成；來源／目的欄位和後續玩家路徑沒有足夠證據仍標未知。發現實際最小阻塞再開窄任務，不先改Esc／cap或推測性規則。

新規格同次索引docs/spec/000-index.md，325後段觀測待辦成功後回填此入口，歷史50M收據保持。主庫RE-first閘門、255完整游標、303整體DRAFT、299自然OF=1、AH2Ch／RNG／人耳與正常開局／remake同狀態未知保持。

READY審查：既有VBEIndexed以append建立索引副本、VBERGB沿副本與Palette值產生RGB、VBEState只回值。直接RAM窗口不走Bus或CPU hook；map計數與PNG編碼不修改guest。六時點、固定輸入、有限區段／前十排序與完整基線比較足以界定純觀測。CPU／平台與325完全保持，原全套仍有效；正式CONFORMED另需六張與完整兩流程不突變收據，未知玩家功能不升格。

## 正式收據與限定結論

只改原版探針唯讀觀測，CPU／startup／provider／問號matcher逐位元保持325，325固定EXE全套PASS仍有效。probe SHA-256 034e6a2c85a3cbbb93d9c7b762d3ba5aea070ec2fc126f82fef30bc913841512；CPU仍1448f29f24dad35e83575189055dc15dd7ac9db2b3529c6d450f03baf91c97b1，來源與正式測試雜湊見[325](325-cpu386-neg-dword-memory.md)。

Go1.24.13映像sha256:1a6d4452c65dea36aac2e2d606b01b4a029ec90cc1ae53890540ce6173ea77ac，600s／2GiB／2CPU／128pids／UID1000／network none。唯讀原ZIP／patch乾淨重建417根檔後，go build -p 2 -buildvcs=false -o /tmp/moo2-probe ./workplace/moo2-probe，再/tmp/moo2-probe /tmp/game/ORION2.EXE --game-dir /tmp/game。兩流程沿325全部環境，只換輸出326；完整環境為DOSGOLEM_MOO2_CALENDAR_EPOCH=1996-01-01、DOSGOLEM_MOO2_HARDWARE_ESCAPE_STEP=44000000、DOSGOLEM_MOO2_MAX_STEPS=50000000、DOSGOLEM_MOO2_SEPARATE_DOS=1，VBE_FRAME_PREFIX=/src/workplace/moo2-326-baseline-frame／click-frame、VBE_PNG=/src/workplace/moo2-vbe-326-baseline.png／click.png均帶DOSGOLEM_MOO2_前綴。點擊流程另加DOSGOLEM_MOO2_NEW_GAME_CLICK_AFTER_DISPLAY40=1。

獨立python3 workplace/post-click-326-verify.py通過：無點擊全部3847列、點擊全部4202列與兩終PNG保持325，僅mtime／DTA日期時間四byte／PNG路徑正規化。只新增六筆post_click_progress；每筆readonly=true、前後完整RAM SHA相同、64byte堆疊與16byte ESI窗口可讀。PNG逐張CRC、640×480 RGB與記錄SHA核對；每個後續區段100000真正Step、前十地址的降序次數／升序地址排序通過。初次區段零，未點擊沒有新列或新圖。

**已證實，固定正常單次點擊的有界原版**：

| 外層步數 | 高位LE EIP | ESI | 虛擬µs | 區段unique_sites |
| --- | --- | --- | --- | --- |
| 49500000 | 2132B0 | 495230 | 64506044 | 0 |
| 49600000 | 22F1F9 | 495230 | 64831498 | 494 |
| 49700000 | 2131AD | 495230 | 65113460 | 618 |
| 49800000 | 21333E | 4952B0 | 65295775 | 623 |
| 49900000 | 2132BF | 4952B0 | 65478284 | 467 |
| 50000000 | 21334F | 4953AA | 65660599 | 1326 |

六時點Bank4／StartY0／BankSets797／Writes16194454／DisplaySets42完全相同；索引SHA b6fb8d68a422788d78e693eca398cab0e54c46127bc49de42c10d18c3aae9cb3、RGB efce8f0dd2eb63e07b25b6933808b2e18d6cc606419a867f292a8b37b44e05c6、PNG 59f76749db5232f97a6b6f969f56f848c2e89e731d7e3fb30b8786982bca4a81各相同。五組相鄰與首末不同像素皆0；首末兩圖已實際檢視，主選單NEW GAME與游標仍見，設定畫面仍未知。

六個不同RAM雜湊與R／堆疊值變化是已證實狀態差異，不證明內部運算正確、最終必然轉畫面或產品掛起。所有區段前四熱門位址2132E0／2132E2／2132E5／2132EA，各區段次數2860／2021／2573／2816／2566；實際末尾32步已含2132E0／31 C0、2132E2／8A 45 F8、2132E5／3D 80 00 00 00、2132EA／0F 84 64 00 00 00。這些分別是XOR EAX,EAX、讀SS:[EBP-8] byte、CMP EAX,80h與JE，實際形狀來自真正尾跡，不以自訂欄位名取代定位。

末尾另見213345／8B 15 74 BE 29 00讀DS:29BE74的dword，21334B／01 D0、21334D／8A 00、21334F／88 45 F8，最後EAX3DD341／AL42h；先前49999979的DS:[EAX] byte為84h、進JE未跳與JLE未跳／低7bit4，後續完整R、來源指標／目的RAM未另取樣，仍未知。ESI窗口六次為零只描述DS:[ESI]取樣，不代表這組真正DS:[EAX]指令的來源，不推字型或素材故障。

| 本機忽略收據／腳本 | SHA-256 |
| --- | --- |
| workplace/moo2-probe-326-baseline.txt.gz | d37cf5c32298e69e38648ac9d70045d6501e188d56f1e7bb3fa2efb6a1877612 |
| workplace/moo2-probe-326-click.txt.gz | fa3d71704a13552e22ef0b04b5700afc9264934ed709a1c8134b5bb8b76782dc |
| workplace/post-click-326-parity-tests.txt | d9e9c2b7ab2386e6ae4d49b02b2070ebbd8742599a33a1424a71f5386f9da6e0 |
| workplace/post-click-326-verify.py | c0c40c21aa0ef315f64463cd9925a9cd7e56ec2383e1424caf4dc4aadf1f36c7 |

限定CONFORMED只含六時點唯讀觀測與兩流程不突變。下一步依末尾實際指令做最小原始讀取／分支觀測：DS:29BE74指標→DS:[EAX] byte→SS:[EBP-8]→CMP／JE／JLE，必要時核對真正目的RAM寫回。來源與目的按實際地址保存，不命名helper、不追完整renderer，不改輸入、50M cap或主庫玩法RE閘門。AH2Ch／RNG、人耳、正常開局與remake同狀態未知保持。

63回填函式、原有32／49／25／27與326新增27缺證據負例、兩CLI PASS。workplace/post-click-326-backlink-tests.txt SHA-256 434fc2536d4b96a9e68e762dc36f81b712dcf0e51ac95d1b776dd01632c94f62。固定原ZIP／patch／417根檔／EXE／MOX.SET再次核對PASS。新來源／六PNG／收據UID:GID1000:1000，工具root-owned／誤建.md目錄自檢空；本輪一次性工作均已退出，未清理其他專案。
