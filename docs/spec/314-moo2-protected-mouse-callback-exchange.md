# 314：MOO2 保護模式滑鼠回呼交換

狀態：**CONFORMED**
日期：2026-10-03
範圍：MOO2 明示平台的 INT33／AX0014h；不改主庫玩法。

## 原始定位與公開契約

工具基線0cc36241a3523df865d9b8f336d70c124bc7093c，固定官方1.31 ORION2.EXE SHA-256 4e11be14217b4aafa1839f333bf5eba037f98b0c44e9e4752c96c464c260419f。313兩個1996-01-01明示零時、48M正常Esc／50M上限流程，第49564005步在dosgolem高位LE0x24C31B的CD 33拒絕AX0014h。完整R為14／1／2136D1／0／2BDA88／2A0000／0／0，六段8／0／8／0／20／188，flags6h。下一0x24C31D是C3，尚未執行；返回地址須由原始SS:ESP讀取，不猜caller。

[Microsoft Mouse Programmer’s Reference 1989，印刷頁107](https://www.bitsavers.org/pdf/microsoft/mouse/Microsoft_Mouse_Programmers_Reference_1989.pdf)說明Function20替換遮罩與回呼並回傳舊值，供呼叫者恢復。[DOSBox-X來源case0x14，行1408–1422](https://dosbox-x.com/doxygen/html/mouse_8cpp_source.html)交叉驗證CX／ES:DX交換。[Watcom11.0c Programmer’s Guide，印刷頁129–130](https://openwatcom.org/ftp/archive/11.0c/docs/cprogguide.pdf)確認DOS/4GW的0Ch使用CX／ES:EDX；此資料未直接證明14h的32位元擴充。原始14h傳入ES0008:EDX002136D1，既有[255-moo2-protected-mouse-callback.md](255-moo2-protected-mouse-callback.md)已保存完整32位元目標。314採同一32位元交換模型，標為platform-spec approximation，不宣稱原版extender逐值exact。

## 契約

- 只在已安裝MOO2派送器的正確CPU接受。輸入低CX遮罩限007Fh；非零遮罩須已登錄、base=0、入口可讀的ES:完整EDX。未安裝、錯誤CPU、未知位元或無效目標拒絕，R／段／flags／FPU／RAM及舊註冊、佇列不改。
- 先保存舊遮罩與遠指標，再用255的register驗證並註冊；成功回傳低CX舊遮罩，保留ECX高word，ES舊selector、完整EDX舊offset。EAX與其餘R／段、flags／FPU／RAM保持。不可先改輸出再驗證新指標。
- 低CX=0沿255解除規則，清除註冊與待派送事件，解除指標可無效；仍回傳舊值。非零交換保留已排事件的舊目標，後續事件用新目標，活動回呼不重入且原框架不改。這些佇列／解除／活動規則沿既有受控模型，沒有原版驅動時序收據，明示為platform-spec approximation。
- 提供唯讀MouseCallbackTarget查詢selector／完整offset，不修改執行狀態。正常觀測限最前三次0014h及每次最多五條後續外層指令，讀R／六段／flags／SS窗口與callback狀態，不跳指令、改資料／亂數或增加50M上限。

## 驗收

合成服務交換／交換回舊值、非零高word、完整EDX、無註冊舊值0、解除、拒絕不突變、FIFO舊新目標、活動回呼框架與恢復、一般FD2與未安裝拒絕。既有callback與固定EXE全套回歸，兩原版同日期／Esc流程自然越過0014h、真正C3及caller消費、第三不設定日期拒絕保持。分開有／無既有受控滑鼠；不冒稱新註冊後自然事件、完整255座標／游標、主選單／正常玩家路徑或remake對拍已完成。

## READY審查

313固定輸入／真正0014h拒絕、公開交換語意、255現存目標儲存／register驗證及唯讀觀測構成最小充分證據。根因是INT33分支缺14h，非CPU CD33或C3故障。32位元擴充與佇列時序維持明示近似；正常收據再驗舊值與caller。不追滑鼠驅動／中斷橋接內部。完成後只回填同固定EXE＋高位LE0x24C31B＋AX0014h，不能因其他INT33共用stub而重開0Ch或敏感度。

原始素材與完整收據只留忽略workplace。索引入口docs/spec/000-index.md。

## 實作與目標測試

INT33新增14h分支；exchange先保存舊mask／target，再沿register驗證，成功才回傳低CX與ES:完整EDX。既有0Ch／reset／callback派送不改。唯讀目標API只輸出selector與offset；探針只讀最前三次交換與五步caller。原版沒有新事件注入或執行捷徑。

四個新測試與六個既有回歸PASS，命令go test -p 2 -buildvcs=false ./internal/machine -run 'TestLEMouse(Exchange|Callback|Position)' -count=1 -v，180s／2GiB／2CPU／128pids／UID1000／network none，Go1.24.13固定映像。驗證0008:00012000與0018:00023001真正互換、非零ECX高word、完整核心／非零FPU／RAM保存、六類無效目標／遮罩與錯誤CPU不突變、解除及空舊註冊、已排舊／新事件與活動框架保存；CPU真正執行兩回呼及CB返回。

internal/machine/le_mouse_callback.go SHA-256 044817ffbb3a22e1446bf729e1dd37a20505d7269ed6883df9c513e027a3b2bb；le_startup.go 88c44d3b2bd292fb7eb4d572a17e1ca1108ad9b6e1ca564a654244cfd90bfce5；le_mouse_callback_exchange_test.go 282b8e2b8e1a30e71822c8143fefa792afaca864bd404b10071c4fa542d6694d；workplace/moo2-probe/main.go 874aadc18fd5eeb59b407ed1d7133912ad63cea249701a0545ca5221babed963。目標收據workplace/moo2-314-mouse-exchange-tests.txt e9baa0f79237394f0d5e217bb71d15033a713cf629ea6bfd98e53eed20fbd6f1。

正式驗證沿313的600s Docker命令，輸出313改314。固定EXE全套go test -p 2 -buildvcs=false ./... -count=1，乾淨417原ZIP根檔加官方1.31 EXE；兩epoch1996-01-01／48M正常Esc／50M cap／SEPARATE_DOS=1，第二組含既有受控事件，第三不設日期。原始輸入唯讀，正式來源凍結，收據審查前保持READY。

## 正式原版收據

固定EXE全套PASS，workplace/full-test-314.txt SHA-256 12df9650c1fa427bce3f0d1e73a9aeda2f81ac905a47187ac0c029218df186fb。三正常流程由同一600s命令完成，來源與固定輸入保持，沒有失敗測試需調參數。

已證實，dosgolem高位LE兩設定日期流程各有24次真正AX0014h成功。每次完整輸入／輸出R與flags6h驗返回低CX是交換前mask，其餘R保持，最後新mask2Bh。唯讀前三次完整段／目標樣本皆ES:EDX=0008:002136D1，舊／新mask依次1→1、1→2B、2B→1；前兩次返回CX1，第三返回CX2Bh。非零ECX高word與不同目標的交換由合成測試補足，不冒稱原版這三次使用不同位址。

前三次下一原版C3都正常執行：第49564006／49564388／49602023步，0x24C31D→0x24C1AE，ESP002BDA88→002BDA8C，完整R除ESP外、六段與flags6h保持。原始SS0188:002BDA88的32byte窗口為AEC12400C4382A00880100001C3A2A0088010000C4382A005C4A2300BCDA2B00，首dword真正返回地址0024C1AE。每組後四步依序執行1E／57／89 E5／8B 7D 08，建立框架並載入EDI002A38C4；三組15步兩排程相同。這只確認返回與後續框架／參數讀取，未擷取之後返回值寫回，不深入平台包裝helper或替它補遊戲欄位語意。

所有313交換停點前的完整前綴除mtime／DTA四bytes外逐列保持，第三未設定日期的完整原始313流程同樣保持0x240A32拒絕，沒有任何交換觀測。不默認日期，不固定正式遊戲亂數。兩初態的既有受控回呼0／1分開驗，三次交換時pending0、activefalse、started／completed保持0／0或1／1。新mask後沒有新增事件驗收，255完整座標／游標保持未知。

兩設定流程沒有step_error或guest_cpu_stop，均step_limit=50000000 eip=0x23856E；這是有界觀測終止，不是未支援指令。時計62461366、IRQ0 started8383／completed8383／failedfalse，VBE Bank7／StartY512／BankSets815／Writes9385664／DisplaySets21，indexed SHA-256 4aec1cb98a44d2f9bdae79b9722f6a8b0821c162ca9c5a819a634b6d20d733eb。兩末尾窗口逐列相同，完整終態R／六段／flags與IRQ7快照亦已保存並核對，勘誤見315。受控滑鼠另使DOS allocator selector及unique_sites不同，這是已明示的不同初態，不抹除差異作同狀態聲明。

兩PNG SHA-256 8f7791ae57649991fbf9bf3a86fdacab602e792f39a9b3d57ae691484a754d47，逐位元相同。已實際檢視原版標題背景，右側主選單面板正在滑入、文字僅部分可見，尚未完整展開，正常點擊／完整主選單未驗。第三圖保持黑色過場1610444d26adb3135e7e933dd44912044bb636728af59c70e614945278d3c622。

| 本機忽略收據 | SHA-256 |
| --- | --- |
| workplace/moo2-probe-314-full-game.txt.gz | 41923df89d7657f6ceb015cb740c6d426e09d0892ab4968a59f414cb53141b30 |
| workplace/moo2-probe-314-mouse-event.txt.gz | a912b61514f85eb348271958666486ee991b80fc5a6b331a5acbb6ababd4dfe9 |
| workplace/moo2-probe-314-unconfigured.txt.gz | 053d2d831055b737673985b6ddf48ea50e1bf7dd2646b0c56b94de3fe83eedbb |

## 回填與限定結論

AX0014h交換停點已由規格 314 接通。309／310／311／312／313同一固定EXE＋高位LE0x24C31B＋AX0014h同次追加勘誤與本檔連結，保留舊拒絕及其歷史範圍。護欄apps/moo2/tools/startup_probe_131.py --check-mouse-exchange-spec-backlinks驗定位、公開近似、正常交換／返回、三收據與五份回填，缺項即拒絕。

限定CONFORMED只包含受控交換服務、上述正常原版返回與24次R／flags觀測。32位元橋接／佇列時序仍是platform-spec approximation，未擷取寫回保持未知，終態已保存。303整體觀測DRAFT、255完整游標READY、299自然OF=1、AH2Ch／RNG、人耳、完整主選單／玩家路徑與remake同狀態未完成，主庫玩法RE閘門保持。下一步在50M內補主選單滑入的有界唯讀階段觀測，先辨識正常等待與輸入條件，不提高上限、猜欄位或跳過動畫。

同次56回填函式、31新增缺定位／狀態／收據／舊標記／連結負例與CLI通過。首輪文件護欄因遠指標文字未含完整冒號形式拒絕，只補正ES:EDX=0008:002136D1並乾淨重跑，程式與正式收據不改。全部檔案及收據UID:GID1000:1000，工具root-owned／異形.md目錄檢查空。20:20:54 UTC本輪有界容器均已退出移除，其他Go容器掛載均屬hr專案，保留未操作；未清理映像。

314終態缺快照斷言已由規格 315 勘誤，見[315-moo2-menu-slide-observation.md](315-moo2-menu-slide-observation.md)。同一兩原始gzip實際含label=terminal與step_limit_registers；完整R／六段／flags206h及IRQ7 started388／completed388已核對，只有既有受控mouse_started／mouse_completed0／1不同。原始收據保持，錯誤來自前輪只搜尋limit／step_limit標籤，漏讀terminal，不是缺觀測或程式故障。
