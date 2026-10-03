# 337：原選族頁的人類正常選擇

狀態：**CONFORMED**，限定第7筆選擇／名稱頁
日期：2026-10-03
範圍：固定1.31正常SELECT RACE中的人類測試情境，一次正常按下／放開及最小原選擇消費。不改正式遊戲預設種族或主庫玩法；不提高100M cap、不代寫原選擇。名稱輸入與完整開局未驗。

## 原來源與待審查契約

工具7c84f3931953cf3c9ffcbdc0718852e9c700b3ba；官方1.31 ORION2.EXE SHA-256 4e11be14217b4aafa1839f333bf5eba037f98b0c44e9e4752c96c464c260419f，位址均dosgolem高位LE。沿[336-moo2-setup-accept-normal-click](336-moo2-setup-accept-normal-click.md)的新鮮417根檔／MOX.SET、44M Esc／1996-01-01、兩組NEW GAME原點擊與80M正常ACCEPT。日期不是seed，人類只是可重播輸入fixture，不改玩家種族選擇功能。

現有workplace/moo2-probe-336-accept.txt.gz SHA-256 a8c64e4519ac73c78578e3d953257c6cd8d19b5be1345899f15cab0c0b86a252，已保存90M／100M完整R／六段／flags／FPU、VBE、原16筆表與callback／IRQ。先獨立解碼兩份表與原range，再審查READY；不重跑已完成ACCEPT，不深入renderer／helper。已證實原SELECT RACE畫面，按鈕名稱與index關係在正常消費前只作強推論。

## READY後的預定驗收

新增明示DOSGOLEM_MOO2_RACE_HUMANS_CLICK=1，只接受值1與完整SETUP_ACCEPT_CLICK固定依賴。原90M前置先核對原table／header／DS／RGB、已完成ACCEPT、IF／callback／IRQ，固定時點不符就停止，不重擲或延後挑可通過的初態。正常press／release只經InjectMouseEvent，至少20ms虛擬時間且回呼完成後首次可送放開，原輸入／CPU／平台保持；原資料決定座標與完整核對值，審查後寫入READY。

最多保存第一筆既有20DDDB原store的完整初態／原bytes／word寫回，沒有命中就明示未知，不代寫結果或推定全部元件相同。新CPU錯誤與後續畫面按實際結果保存。五舊情境全部行與圖片保持；新情境額外輸入前保持336獨立ACCEPT，同100M cap、不增加其他事件。來源與收據不等於remake同狀態或完整開局。

## 工具與權利

沿336固定Go1.24.13映像sha256:1a6d4452c65dea36aac2e2d606b01b4a029ec90cc1ae53890540ce6173ea77ac；Docker900s／2GiB／2CPU／128pids／UID1000／network none，原ZIP／patch唯讀。CPU／startup／provider／matcher保持335，335固定EXE Go全套仍有效；本輪只加正常輸入與觀測。新規格同次索引000-index，限定CONFORMED後回填336與檢查器。原版素材／LOG／PNG／RAM留忽略workplace，不公開；擁有權／Docker收尾。主庫RE-first保持。

## 原輸入證據審查與READY契約

兩份正式336原表count16／bias0／pointer298848／stride55，完整880bytes逐位元保持，SHA-256 f03515b12cb289bfcfe49b46d5cf8619f8f4e300ccfd1bd4ca43107f1c313ade。原index7 offset2989C9完整55bytes為5F014A01D901760101000300D8E1290000000000000000008A0F2600000000002A3828000000000000000000E4364F0000000000002800，四原word351／330／473／374；對照實際336 PNG強推論為Humans。其他欄位與指標用途仍未知，不命名成玩法欄。候選412／352與413／352只落此第7筆。

90M原EIP238576／R=[369321 7A 2D 0 2BD9F0 2BDA4C 502022 369321]／段=[8 188 188 0 20 188]／flags206h、IF已開，FPU control127F／status0／depth0，VBE Active true／StartY512／DisplaySets47。callback mask2Bh／pending0／active=false／6／6，IRQ19929／19929非活動／非failed；原header與表可讀、整RAM前後保持。RGB SHA-256 9d8c0a1acb3b96200296f789bb13c6877067f6165ec608a7e019506036832dfc。

原表與可接受輸入初態足夠，337轉READY：固定90000000、核對已保存原880bytes hash／header／DS／RGB、ACCEPT原store已見／press與release已完成、callback6／6、IF與IRQ，任何不符即停止。press x824／y352／buttons1，原映射412／352；正常至少20000µs且回呼完成後首次可送release x826／y352／buttons0，原映射413／352。不猜固定Step差，不延後重試，不代寫原RAM／EIP／種族。新增旗標RACE_HUMANS_CLICK僅值1與完整SETUP_ACCEPT_CLICK固定依賴，無效值與缺依賴在讀EXE前拒絕；模式關閉保持五舊情境。

實際首筆20DDDB原66A3A6C42600只保存來源、寫回與完整核心，不預定index7必經這個consumer；若未命中則明示未知。新正常輸入後的畫面／CPU錯誤據實保存，100M cap不改。Humans語意由正常消費與原後續玩家畫面再核對，不把來源幾何當已選族。若出現名稱／確認介面，先保存，未READY不自行送下一事件。

python3 workplace/new-game-337-input-verify.py PASS：兩原880bytes／完整90M核心／回呼與IRQ／兩候選唯一index7；腳本SHA-256 3d3db6ccaf6bf48ab934f148bf31ec8f2d166c8568811bb5d032996163b36c57，收據workplace/new-game-337-input-tests.txt SHA-256 4ac2d0b56904d9fba8323c5ceb184ed9e5e0d5fb40abf1ecfc9415f2b02abf2a。使用現有正式原來源，不額外重跑相同初態；無新CPU或玩法推測。

## 正式驗收與目前邊界

限定驗收：原16筆選族表／正常第7筆選擇／統治者名稱頁／五舊基線保持。人類候選正常選擇已消費；此處不宣稱typed種族特性、名稱寫回、完整開局或remake同狀態已驗收。

六原版由乾淨417根檔／官方EXE各一次重生。固定Go1.24.13 Docker900s／2GiB／2CPU／128pids／UID1000／network none，原ZIP／patch唯讀。go build -p 2 -buildvcs=false -o /tmp/moo2-probe ./workplace/moo2-probe；沿336五基線換337名，加RACE_HUMANS_CLICK=1獨立正常輸入情境，原版每條仍100M cap。初態直接採336既有正式收據，不重跑相同初態；未改CPU／平台或其他事件。

python3 workplace/new-game-337-verify.py PASS：五舊情境全部3847／4829／6769／8149／7752原336列除既定正規化保持，全部132PNG逐位元保持；新humans固定90M額外輸入前全部6821原列保持獨立ACCEPT。完整原表／R／段／EIP／flags與RGB前置對接；正常輸入後不同狀態不冒充同狀態對拍。

**已證實，正常第7筆選擇**：press固定90000000、virtual_micros154693274、x824／y352／buttons1，映射412／352；release實際90010495、virtual_micros154735567、x826／y352／buttons0，映射413／352，差42293微秒，正常callback7／7後首次可送時放開。兩點只命中原index7，全部只經InjectMouseEvent，不代寫原種族。

90056672原高位LE20DDDB實際66A3A6C42600，EAX7、DS188:26C4A6 word0000→0700，下一EIP20DDE1；R=[7 502004 181 298848 2BDA2C 2BDA94 D 1A]／段=[8 188 188 0 20 188]／flags297h保持，callback8／8與IRQ19946／19946完成、皆非活動／非failed、error=nil。原store獨立核算，正常選族表第7筆已實際選中；typed種族／trait producer不在本輪驗收。

**已證實，統治者名稱頁**：同100M cap到原高位LE215DEE，無新CPU拒絕。終態R=[6 178 DDE0 7 2BD94C 2BD970 2843A5 28439D]／段=[8 188 188 0 20 188]／flags206h、callback8／8完成。640×480原PNG人工確認Enter Ruler Name、預設Strader與ACCEPT，末尾底線視為畫面字形，不直接當作輸入buffer字元。PNG SHA-256 7f1725d8669cacd9758350420bb60e4dcf6f01c8139ad422edc1e49b3577fc61；RGB SHA-256 3e264c7fbd003a8e24cfea2e4fc0e019d9ee096a760c467634b44a6ef37a9fc9。

原100M名稱表count3／pointer298848／stride55，完整165bytes SHA-256 f3dc28cf153edf625b154c5864b3040cddd52fc9ade2a0cfd2a256b80789d0a7、readonly=true，IRQ22901／22901完成。index1範圍273／225／371／253對照ACCEPT為強推論；輸入前置與名稱原buffer／正常確認尚未驗，不從100M終態直接送輸入或代寫文字。

python3 workplace/new-game-337-cli-verify.py PASS：16無效值／缺依賴在讀EXE前exit2、有效正對照越過參數閘門後缺EXE明確失敗；同新binary舊336 CLI 15拒絕與正對照保持。CPU／startup／provider／matcher逐位元保持335，335固定EXE Go全套仍有效，不外推完整玩法parity。新probe SHA-256 6d13d37c144e35cea19a2decc1b623b4d24023d07cee341a25d4ff5f5ba93712；CPU SHA-256 967de02753e0dce276413fe67a085e6df16789b52c948999d30ed26c3bb4e6a4。

| 本機忽略收據／腳本 | SHA-256 |
| --- | --- |
| workplace/new-game-337-input-verify.py | 3d3db6ccaf6bf48ab934f148bf31ec8f2d166c8568811bb5d032996163b36c57 |
| workplace/new-game-337-input-tests.txt | 4ac2d0b56904d9fba8323c5ceb184ed9e5e0d5fb40abf1ecfc9415f2b02abf2a |
| workplace/moo2-probe-337-baseline.txt.gz | 2b86481ae22b7c4cf675ac169890a33b63b574dad642c150a0ffd7b3a0b40b3e |
| workplace/moo2-probe-337-click.txt.gz | 32e007ef3550b70775faa8a89362e0c963bc2ec542b9cf88818b1ebfb3cc2c45 |
| workplace/moo2-probe-337-extended.txt.gz | b6e5177086d2477db952c78774cbc72e92e93c3a7a72102273ff8984682deaba |
| workplace/moo2-probe-337-ready.txt.gz | ca2aa612913c9ef5d90666576039ddccc28c541fc2aa7e610b048bb12de0861b |
| workplace/moo2-probe-337-accept.txt.gz | 5a0ad464cc663be9701bad5314f4c82308a310c19e9b8bb2bba0dcbce23f5c40 |
| workplace/moo2-probe-337-humans.txt.gz | 541d0032fa9711a65fe00f62018cf00bea7a78daed46dc06ce79fe8b5de20e45 |
| workplace/new-game-337-verify.py | 95ff52a16ae185bd989a74e4bff962758d08884680eebe7d4dfb7c462450c134 |
| workplace/new-game-337-parity-tests.txt | 9b41ace6e35830796bade6341a10688f7800bbd37522b5172e124826709ae30f |
| workplace/new-game-337-cli-verify.py | 5c12da132ba19c3139c0014f6513b6d4755d4e1c74310e4e0e8e095c32663dd6 |
| workplace/new-game-337-cli-tests.txt | 22d72c914180b92bec7ae33148f59702fe53d1d839221c0d51dc25aef271fce1 |

74回填函式、既有缺證據負例及新增27負例、兩CLI通過。私有backlink收據workplace/new-game-337-backlink-tests.txt SHA-256 9b9c07085f859ad64350ee7ea8fc919cab4d0dee989eebbe5d6b5df4cd52844f。

下一步只在同humans情境較早正常時點唯讀保存名稱頁與165bytes原表、名稱buffer及ACCEPT可接受輸入前置，再依新READY規格正常確認。此輪未輸入／確認名稱，不深挖renderer或原helper，不重開ACCEPT／SCASW。正式RNG、完整開局、typed種族狀態與remake同狀態未知，主庫RE-first保持。原版LOG／PNG／RAM留忽略workplace，不公開素材。

## 2026-10-03 名稱確認後續回填

正常名稱確認與後續畫面已由規格338接通，見[338-moo2-ruler-name-normal-confirmation](338-moo2-ruler-name-normal-confirmation.md)。原mask1放開後callback10／10完成，SELECT BANNER COLOR已見，無新CPU拒絕；共享20DDDB未命中，持久名稱writer仍未知。337原收據／來源與限定驗收保留，不外推完整開局或remake同狀態。
