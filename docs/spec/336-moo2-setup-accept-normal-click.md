# 336：原設定頁按鈕表與ACCEPT正常輸入

狀態：**CONFORMED**，限定原表／正常ACCEPT／選族頁
日期：2026-10-03
範圍：固定1.31 ready情境的原設定頁表、正常ACCEPT輸入及最小選擇消費。不改CPU／主庫玩法，不提高100M cap，不改舊正常事件或既有觀測行。

## 原始來源與玩家問題

工具74f574a78927f6bacdec95ea0519c83078bc71df；官方1.31 ORION2.EXE SHA-256 4e11be14217b4aafa1839f333bf5eba037f98b0c44e9e4752c96c464c260419f，位址均dosgolem高位LE。沿[335-cpu386-repne-scasw](335-cpu386-repne-scasw.md)的44M Esc／1996-01-01、新鮮417根檔／MOX.SET、舊正常NEW GAME點擊與MENU_READY_CLICK=1。335於80M／90M／100M同一設定頁PNG，RGB SHA-256 3bbe6c339cfbc74e84b3210bccfdc4574073b4d308ef9b90a1720cc5c74e623e。日期不是seed。

已證實：100M原表指標298848／count17／bias0，舊333觀測器限制count≤16因此records空，不是原資料無表。DRAFT起點的未知是完整17筆／ACCEPT index與範圍、80M輸入前置、正常消費與後續畫面；下節原來源與正式收據已解出本輪範圍。不得由PNG猜index或代寫選擇。

## DRAFT唯讀蒐證

在ready情境原80M／90M／100M三時點，另加setup_table_snapshot前綴。保存完整R／六段／EIP／flags／FPU、VBE、原DS:26C480 globals192bytes、DS:29BE0E header16bytes、由原pointer與count讀完整55byte stride表，count上限64；以原描述符／RAM bounds拒絕越界。保存原事件／選擇與callback／IRQ、整RAM前後hash與readonly。原333限制與舊列保持，不將舊caller frame當新設定頁frame。最多三快照，不改輸入／CPU。

先同ready重跑一次，扣新增前綴後完整335舊列／圖片保持，再獨立解碼17個原範圍。對照實際PNG，只為ACCEPT定位；區域名稱在實際正常消費前標強推論。三時點資料足夠後審查READY，再增獨立明示正常ACCEPT情境。

## READY後的預定驗收

新旗標只接受明示1與完整ready固定依賴，無效值／缺依賴在讀EXE前拒絕。正常press只經InjectMouseEvent，先核對已保存原table／header／狀態、IF、callback／IRQ及VBE；release沿正常至少20ms虛擬時間和回呼完成閘門。固定一次press／release，不代寫EIP／RAM／選項。原store只保存一筆與最小消費；實作地址與輸入值由原證據審查後寫入READY，不預猜。

保持四舊基線完整行與圖片，再獨立新情境至同100M。無新CPU拒絕與後續畫面以實際結果保存，不預定成功。設定頁消費不冒充完整開局、RNG同狀態或remake parity。原LOG／PNG／RAM留忽略workplace，公開只提交自製工具與證據定位。主庫RE-first保持。

## 工具鏈與入口

沿335固定Go1.24.13映像sha256:1a6d4452c65dea36aac2e2d606b01b4a029ec90cc1ae53890540ce6173ea77ac；Docker600s／2GiB／2CPU／128pids／UID1000／network none，原ZIP／patch唯讀。新規格同次掛[000-index](000-index.md)，產物擁有權與Docker收尾核對；限定CONFORMED後回填335與檢查器。

## 完整原表與READY審查

唯讀情境扣三新列後，全部8146原335 ready列除既定正規化保持，所有舊ready快照與終圖逐位元保持。80M／90M／100M原DS188:26C480 pointer298848、header11000000000000000000000000000000、count17／bias0／stride55，完整935bytes三份逐位元保持，SHA-256 2a18a0213dcb1d539de3175c8356b8c8b886c1d61b38f63795b3fa1859a3f52f。原表以實際17筆擷取，未修改舊333 count≤16觀測器。

原index15 offset298B81的55bytes以原bytes保存：B10188010F029E0100000300D8E129000000000000000000B89428000000000000000000000000000000000094324B0000000000002800。四原word為433／392／527／414，對照原設定頁PNG強推論為ACCEPT；index14為115／391／209／412對應CANCEL，其他欄保留未知，不由byte名稱猜行為。正常候選480／400及481／400均在index15內，其他有效矩形不包含此點。

80M原初態EIP22F1FB／R=[2BD88C 1 D6 67 2BD8D8 2BD91C 1B0 1F3587]／段=[8 188 188 0 20 188]／flags213h；VBE Active true／StartY0／DisplaySets44與原設定頁RGB吻合，callback mask2Bh／4／4／pending0／active=false，IRQ16965／16965、非活動／非failed。直接peek與整RAM前後保持。原事件bytes0000F401E50000000100010001000000，原選擇word仍2。

輸入初態與區域證據足夠，336轉READY：新增DOSGOLEM_MOO2_SETUP_ACCEPT_CLICK=1，僅接受值1與完整ready情境固定依賴，舊模式不啟用。固定80000000先核對原935bytes hash／header／DS／ready原選擇已見／callback4／4／IRQ／IF／RGB；不符就停止、不延後挑可通過時點。press x960／y400／buttons1，原callback座標縮放至480／400；至少20000µs且正常回呼完成後首次可送release x962／y400／buttons0，映射481／400，不猜固定Step差。只經InjectMouseEvent，各一次。保留四舊情境；新情境不提高100M cap。

原20DDDB的66A3A6C42600是既有選擇store定位；新情境首個該原store只保存完整初態／原bytes與兩byte寫回，沒有命中就明示未知，不代寫index15或假裝ACCEPT成功。新畫面與CPU停止按實際結果驗收，最多一筆store，不追callee。這份READY只足以實作正常輸入與觀察，不預定ACCEPT結果或玩法。

初態收據workplace/moo2-probe-336-input.txt.gz SHA-256 ec8f0867e944a0323813316b3805e21c1e220c654b0f6651826b210cb6d043fb；唯讀probe SHA-256 717080e391f566869fc41b54b4ff4154c0e484884089b47a38ae6dc858ee7f56；自製核算腳本workplace/new-game-336-input-verify.py SHA-256 e22d9aa59b4b5cc855b0777f6dc5537ee05f8cf83137c9255b8167d6e8b402ff。CPU／startup／provider／matcher保持335，原素材不公開。

## 正式驗收與目前邊界

限定驗收：完整17筆原表／正常ACCEPT／選族頁／四舊基線保持。設定頁的ACCEPT正常消費已證實，完整正常開局、種族選擇與remake同狀態仍未知。

正式五原版由乾淨417根檔／官方EXE重生，各一次。Docker固定Go1.24.13映像與UID1000／network none／2GiB／2CPU／128pids；初態蒐證600s，五情境正式重跑外層900s，只調整容器有界逾時、原版每條仍100M cap。go build -p 2 -buildvcs=false -o /tmp/moo2-probe ./workplace/moo2-probe；沿335四基線換336名，加SETUP_ACCEPT_CLICK=1獨立情境，不改舊原事件，不提高100M cap。

python3 workplace/new-game-336-verify.py PASS：四舊情境全部3847／4829／6769／8146原335列除既定正規化保持，ready只扣三setup_table_snapshot新列，全部102PNG逐位元保持。accept固定80M額外輸入前全部6145原列保持336獨立ready，新正常輸入後不同狀態不冒充同狀態對拍。新前置935bytes與未加ACCEPT原80M snapshot逐位元保持、完整R／六段／EIP／flags與RGB對接。

**已證實，正常ACCEPT選擇**：press固定80000000、virtual_micros125567232、x960／y400／buttons1；release實際80011248、virtual_micros125610144、x962／y400／buttons0，差42912微秒，正常回呼5／5後首次可送時放開。座標480／400與481／400只落原index15，沒有代寫原選擇。正常callback6／6完成、pending0／active=false。

80124668於原高位LE20DDDB實際66A3A6C42600，EAXFh、DS188:26C4A6 word0000→0F00，下一EIP20DDE1；完整R=[F 0 339 0 2BDB14 2BDB7C 0 2B0001]／段=[8 188 188 0 20 188]／flags297h均保持，IRQ17000／17000與callback6／6皆非活動／非failed，error=nil。正常原store與畫面轉移共同證實ACCEPT，不只用幾何交集當成功。

**已證實，SELECT RACE選族頁**：同100M cap到原高位LE21595F，無新CPU拒絕。終態R=[7F 1A8 341DF4 8A 2BDA7C 2BDA9C D 1A]／段=[8 188 188 0 20 188]／flags212h。原640×480 PNG人工確認SELECT RACE與種族／Custom按鈕；滑鼠留在480附近不能當已選任何種族。PNG SHA-256 7aec4ca6aad1f948560e184695bd3b415b1145ad9778e536da14a60e11d61861；RGB SHA-256 9d8c0a1acb3b96200296f789bb13c6877067f6165ec608a7e019506036832dfc。90M／100M原選族表count16／pointer298848／bias0，完整880bytes保持，SHA-256 f03515b12cb289bfcfe49b46d5cf8619f8f4e300ccfd1bd4ca43107f1c313ade，留作下一正常輸入來源，不推定完整欄位語意。

python3 workplace/new-game-336-cli-verify.py PASS：15無效值／缺依賴在讀EXE前exit2、有效正對照越過參數閘門後缺原EXE明確失敗；同一新binary舊334 CLI 14拒絕與有效正對照保持。CPU／startup／provider／matcher逐位元保持335，335固定EXE Go全套仍有效，這不擴張為整款玩法parity。新probe SHA-256 cfcc89585eb163e67c3043202501f957708b4818985ddbd6d4b1f2138c635b19；CPU SHA-256 967de02753e0dce276413fe67a085e6df16789b52c948999d30ed26c3bb4e6a4。

| 本機忽略收據／腳本 | SHA-256 |
| --- | --- |
| workplace/moo2-probe-336-input.txt.gz | ec8f0867e944a0323813316b3805e21c1e220c654b0f6651826b210cb6d043fb |
| workplace/new-game-336-input-verify.py | e22d9aa59b4b5cc855b0777f6dc5537ee05f8cf83137c9255b8167d6e8b402ff |
| workplace/new-game-336-input-tests.txt | 7802819d83ad8c61b7c49d63b7bed05f58e20ed0b011408855331cb5241770f3 |
| workplace/moo2-probe-336-baseline.txt.gz | bde1882211515e81d17abd98f9c2a49fa8cbe3f11939c80cd88680da69f3fedb |
| workplace/moo2-probe-336-click.txt.gz | 29445900c269635d1c47877d42eed2f3d4f8cc4effb7c0b318d33c4a280a57f1 |
| workplace/moo2-probe-336-extended.txt.gz | 08cbf2b2994d353033ec84c22df2f9e0a03e21227e7a817ae0447b915583c0ff |
| workplace/moo2-probe-336-ready.txt.gz | e3f22c215074a9d3911971a9bdadc76414e569162c868f224ffd23f6d0abbbfc |
| workplace/moo2-probe-336-accept.txt.gz | a8c64e4519ac73c78578e3d953257c6cd8d19b5be1345899f15cab0c0b86a252 |
| workplace/new-game-336-verify.py | da6336609bb79a5887b383a2505ed4f7f4f4cfa927aa59b9a46f924d9b5d7083 |
| workplace/new-game-336-parity-tests.txt | a7488a255580640aef0af2a8b67d1c9572ff7f81e6c9bc0007350e48eee4db0d |
| workplace/new-game-336-cli-verify.py | f63589c59f6a91749537d75453af4bf8da6883776e02849ae006d6c9f4bb678b |
| workplace/new-game-336-cli-tests.txt | 503b4534840a44fe81a235244c8ea4e2393610ed8458b2b23f2719db66efc313 |

73回填函式、既有缺證據負例及新增28負例、兩CLI通過。私有backlink收據workplace/new-game-336-backlink-tests.txt SHA-256 4d79d60bd98d9b0eddf69c3a4a855e10077477f90632cf1252ae78f240d42e74。

下一步只獨立解碼已保存90M／100M的原16筆選族表與正常種族輸入前置，再依新READY規格送一次正常選擇；不重開已完成ACCEPT或SCASW，不深入renderer／原helper。正式RNG、完整開局、remake同狀態與中文化保持未知，主庫RE-first保持。原版LOG／PNG／RAM留忽略workplace，不公開素材。

## 2026-10-03 正常選族回填

正常第7筆選族與名稱頁已由規格337接通，見[337](337-moo2-race-humans-normal-click.md)。人類候選第7筆原store已核對，正常進入Enter Ruler Name；五舊基線與132PNG保持。本文先前種族選擇未知屬先前範圍，typed種族／trait、名稱確認、完整開局與remake同狀態仍未知。
