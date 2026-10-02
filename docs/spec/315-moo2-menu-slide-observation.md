# 315：MOO2 主選單滑入階段觀測

狀態：**CONFORMED**
日期：2026-10-03
範圍：固定1.31原版正常48M Esc後的唯讀換頁取樣；不改平台／玩法或增加輸入。

## 基線與勘誤

工具9a2c7a21b0901ac8acc1f4387729ce25072b5fd9，固定EXE SHA-256 4e11be14217b4aafa1839f333bf5eba037f98b0c44e9e4752c96c464c260419f。314兩正式流程均到50M，高位LE0x23856E，最末畫面主選單面板部分滑入。

314所寫「沒有完整R／六段／IRQ7終態快照」錯誤。原始同一gzip實際有late_startup_platform label=terminal、irq7_passdown_state label=terminal與step_limit_registers。兩完整R為3530C4／0／F3／1／2BDB10／2BDB68／4F6F42／353316，六段8／188／188／0／20／188，flags206h；IRQ7 started388／completed388，DMACompletions388，剩1742、credit371200、current4132h、count0ECDh。裝置irq7_deliveries389含先前16位傳輸，與保護模式完成數不混用。時計62461366、IRQ0完成8383，mouse mask2Bh／pending0／activefalse，started／completed有／無1／0。此勘誤來自既有原始收據，不需重跑來補不存在的缺口。

## 唯讀契約

- 只在明示DOSGOLEM_MOO2_VBE_FRAME_PREFIX時擷取；在outer_step≥48000000、真正成功INT10 AX4F07h換頁後，VBE active、DisplaySets新值才擷取，最多16張。原始VBE hook及服務返回值保持。
- 取樣以m.VBEIndexed／VBERGB的複製資料，不寫RAM／CPU／裝置／時間；每張記錄高位LEcallsite、outer_step、虛擬時計、頁面狀態、完整R／六段／flags、索引／RGB／PNG SHA及readonly核對。CPU、段、EIP、flags、完整FPU與VBEState在讀取前後必須相同，否則失敗。
- 檔名沿既有忽略workplace使用指定prefix＋固定DisplaySets編號，不建新目錄，不提交原版畫面。沒有設定prefix時完全不擷取。原48M Esc與50M cap保持，沒有新受控滑鼠／鍵盤事件。
- 正常兩日期流程與第三不設定日期，用同一乾淨417原檔／官方EXE重生；除新增唯讀phase列、PNG輸出名稱及mtime／DTA四bytes外，全部314原始列與核心、IRQ／時計／輸入／VBE逐列保持。兩初態受控事件與allocator selector差異保留，不能逐項刪成相同初態。

## READY審查

314真實21次換頁與部分面板、兩上限完整核心已核對；根因尚未確定，不能當作掛起或缺指令。現存VBE快照API已確認複製framebuffer，色盤讀取不改裝置，PNG只是本機證據輸出。純觀測契約已足以READY；不新增玩法規格、重做VBE時序或深入renderer helper。CPU／平台來源不改，314全套PASS仍有效；新probe由正常go run編譯，三正式流程驗不突變。未觀測到完整主選單或正常輸入前，只判定畫面階段，不宣稱正常玩家路徑或remake對拍。

索引入口docs/spec/000-index.md，較早勘誤回填314同一固定EXE＋高位LE0x23856E＋50M終態；歷史原始收據保持。

## 實作與正式收據

只有workplace/moo2-probe/main.go增加明示換頁快照，SHA-256 546c234534a8f82e3a5e37de82e95fc46d220af0e428184ac2eafbaf64fc530d。CPU／平台來源與314逐位元保持；314固定EXE全套PASS沿用，不重跑CPU語料。正常go run實際編譯新探針與三流程成功，沒有新的玩法或平台實作。

Go1.24.13映像sha256:1a6d4452c65dea36aac2e2d606b01b4a029ec90cc1ae53890540ce6173ea77ac，600s／2GiB／2CPU／128pids／UID1000／network none，原ZIP／patch唯讀。命令沿314去掉已通過且來源未改的全套測試，輸出315；三go run各明示DOSGOLEM_MOO2_VBE_FRAME_PREFIX=/src/workplace/moo2-315-<條件>-frame，其他日期／48M Esc／50M cap／SEPARATE_DOS／第二受控滑鼠不變。所有條件先固定，未試多個輸入挑成功。

兩設定流程各14張DisplaySets8..21，第三不設定日期1張第8頁，共29張；全部readonly=true，PNG SHA與收據逐張核對。兩14張索引／RGB／PNG與完整R／段／flags／時計／VBE逐列相同。移除新增menu_slide_phase列，只對mtime／DTA四bytes／終圖檔名做既有正規化後，三流程每一既有314列均逐列相同，包含完整終態與所有輸入／回呼／音訊／IRQ／時計，而非只比較末圖。兩設定末圖仍8f7791ae57649991fbf9bf3a86fdacab602e792f39a9b3d57ae691484a754d47，第三仍1610444d26adb3135e7e933dd44912044bb636728af59c70e614945278d3c622。

| 真正換頁 | 外層步數 | 虛擬µs | 已檢視的畫面 |
| --- | --- | --- | --- |
| 8 | 48222699 | 57866266 | 黑色，與第9頁RGB相同，索引不同 |
| 10 | 49512086 | 60909981 | 標題背景，與第11頁RGB／索引相同 |
| 16 | 49756873 | 61670846 | 右側面板邊緣可見 |
| 21 | 49967220 | 62395438 | 右側面板已部分滑入，完整主選單仍未見 |

兩組先前0x23856E不是未支援停點；當時仍在執行普通指令。逐幀測量以第10張為背景，Go標準image/png解碼，固定像素區域x=[500,640)、y=[110,350)，逐點RGBA不同即計數。第11張0點；12／16／17／18／19／20／21張變動區最左x分別622／621／619／608／596／583／571，變動點數407／1669／3852／6193／8718／11726／14364。這是畫面量測，不給遊戲欄位或動畫公式命名；支持「到50M時動畫仍有進展」的強推論，不證明之後必定完成或正常點擊已可用。

前三個代表畫面10／16／21已實際檢視。第21次換頁PNG SHA-256 0e5f213ed365098b07e2fe92bc5d8259fc6a13081b413d51c3edb1bc0293db4f，與最末PNG不同，取樣點為成功換頁後、後續游標／畫面修改前；不把兩個不同執行時點混為同狀態。

| 本機忽略原始收據 | SHA-256 |
| --- | --- |
| workplace/moo2-probe-315-full-game.txt.gz | 91ff5b147572fc6a7dbe706c3e19f832fdd3ab61b8c7ed6a0e2c29a9cc6ba779 |
| workplace/moo2-probe-315-mouse-event.txt.gz | 6d53ab8d999c22f89814fa2793316a59ffada212f6fe14639cfc904e270f9926 |
| workplace/moo2-probe-315-unconfigured.txt.gz | 4895e5e54aa52d12d0333f1fc1cec86bfeb5121957dbadfc0316ee9962f7a701 |

輔助Go圖像量測首輪未限制Go平行建置，64pids下asm fork拒絕，屬分析環境。明示GOMAXPROCS=2／go run -p 2後，沿同一Go映像、120s／1GiB／1CPU／64pids／唯讀工作樹乾淨重跑通過；沒有改PNG或產品來源來放行。此量測不作原版規則證據。

## 限定結論與下一步

315只對唯讀階段觀測及314終態勘誤限定CONFORMED，不新增玩法規則。314終態缺快照斷言已由規格 315 勘誤，較早314保留原始收據及錯誤成因索引。apps/moo2/tools/startup_probe_131.py --check-menu-slide-spec-backlinks檢查原始定位／完整終態／階段收據與314回填。

既定48M Esc／50M cap只能證明面板部分滑入；繼續同一上限重跑不會取得新進度。下一步為明示較早Esc的獨立正常輸入排程，先定READY契約，再用既有硬體鍵盤路徑驗證，與48M基準分開，不提高50M上限或代寫資料／跳動畫。AH2Ch／RNG、255完整游標、299自然OF=1、303整體觀測、完整主選單／玩家路徑、remake同狀態與主庫玩法RE閘門保持。

57回填函式、21新增缺定位／終態／收據／狀態／314勘誤負例與CLI通過。新來源及29張原版快照1000:1000，工具root-owned／異形.md目錄檢查空，原始ZIP／patch／417根檔／EXE／MOX.SET再次核對保持。20:38:16 UTC Go工具映像的專案相關執行中與停止容器清查空，本輪所有有界容器已退出移除，未清理其他專案或映像。

明示Esc排程已由規格 316 接通，見[316-moo2-configured-hardware-escape-schedule.md](316-moo2-configured-hardware-escape-schedule.md)。不同46M初態兩組到50M時六個主選單按鈕文字完整可見；48M每一原始列及PNG仍保持本檔基線，不能把此處的48M部分滑入改寫成同狀態完整選單。新遊戲點擊、動畫停穩與正常玩家路徑未驗。
