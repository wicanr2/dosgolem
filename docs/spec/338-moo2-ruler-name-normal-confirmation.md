# 338：統治者名稱頁的正常確認

狀態：**CONFORMED（限定正常名稱確認與旗幟頁）**
日期：2026-10-03
範圍：固定 1.31 原版正常選取 Humans 後，保留預設統治者名稱，經正常 ACCEPT 按下／放開確認。這是可重播測試情境，不改正式預設或主庫玩法。原名稱頁與正常輸入前置已取得；正式持久名稱 consumer 仍未知，不提高 100M cap。

## 原始來源與待審查契約

工具起點 0633c346ce2ca1156ab26c1dbc533ec0e43920e0。[337-moo2-race-humans-normal-click](337-moo2-race-humans-normal-click.md) 已證實正常第 7 筆選擇與 Enter Ruler Name、預設 Strader、ACCEPT 畫面。官方 1.31 ORION2.EXE SHA-256 為 4e11be14217b4aafa1839f333bf5eba037f98b0c44e9e4752c96c464c260419f，位址空間為 dosgolem_high_le。

沿 337 的新鮮 417 根檔／MOX.SET、44M Esc／1996-01-01、兩組 NEW GAME、80M 設定 ACCEPT 與 90M Humans 正常輸入。日期不是 seed。既有 workplace/moo2-probe-337-humans.txt.gz SHA-256 為 541d0032fa9711a65fe00f62018cf00bea7a78daed46dc06ce79fe8b5de20e45。100M 終態原表 count3／stride55，完整 165 bytes SHA-256 f3dc28cf153edf625b154c5864b3040cddd52fc9ade2a0cfd2a256b80789d0a7；index1 範圍 273／225／371／253 對照 ACCEPT 為強推論。100M 終態不能當作相同上限內的輸入前置。

先以可丟棄的唯讀探針取得 95M／97M 原狀態、畫面與完整表，限定搜尋原 ASCII 名稱候選，保存原位址及 bytes。名字資源、顯示暫存與正式持久資料不能只因字串相同就當作同一欄位；未追原 producer／consumer 的部分保持未知。原輸入、CPU、平台與 100M 上限不變，唯讀探針的新增列可剝除後核對 337 原行與圖片。

## READY 後的預定驗收

只有證據足以固定正常確認前置後才新增明示旗標，要求完整 RACE_HUMANS_CLICK 固定依賴，只接受值 1。依原 table／DS／RGB、已完成選族、IF／callback／IRQ 核對，在固定時點送一次正常按下；至少 20ms 虛擬時間且回呼完成後首次可送放開。只能經 InjectMouseEvent，不代寫原文字、選擇或 EIP；不符合前置就停止，不延後挑可通過的初態。

限定觀察第一筆原已知 20DDDB 共享選擇 store，完整 bytes／前後 R／段／flags／word 核對。名稱字串的正式寫回需要原 caller／buffer 證據，不以共享 selector store 冒稱已保存名稱。記錄後續原版畫面與新 CPU 錯誤。六條舊情境所有原行與圖片保持；新情境額外輸入前保持獨立 humans，不新增其他事件、不提高 cap。

## 工具、垂直鏈與停止線

原資料 → 唯讀表／緩衝區 → 原正常輸入 → 原選擇與名稱 consumer → 後續 UI。未到達的持久資料／存檔與 remake 同狀態保持未知。名稱確認後阻塞由實際結果另開最小任務，不深挖 renderer／helper，不重開已完成選族／設定 ACCEPT／SCASW。

沿固定 Go1.24.13 映像 sha256:1a6d4452c65dea36aac2e2d606b01b4a029ec90cc1ae53890540ce6173ea77ac；Docker 外層 900s／2GiB／2CPU／128pids／UID1000／network none，原 ZIP／patch 唯讀。CPU／startup／provider／matcher 保持 335，本輪只作唯讀蒐證與 READY 後的正常輸入；不外推整款玩法 parity。主庫 RE-first 保持。

新規格同次加入 000-index；限定驗收後回填 337。原版 LOG／PNG／RAM 留在忽略的 workplace，不公開。輪末核對新來源／收據擁有權與 Docker 清理。

## READY 證據審查與固定輸入

新增唯讀初態收據 workplace/moo2-probe-338-input.txt.gz SHA-256 47172f944786e4d134091485ce73ae64d7df622fb75470fa62ddf7a660526e9e；私有可丟棄探針 workplace/moo2-name-338-input.go SHA-256 9aa51fcddc4ffdc44896424f3bd50757c2cf49b52390dbc90550dd130faf812a；獨立核算 workplace/new-game-338-input-verify.py SHA-256 e29103c693b3536da56b9081c75e8edf297776982fb217a4d0688ecdbb730544，收據 workplace/new-game-338-input-tests.txt SHA-256 b26cb2cb901c0930689c1c17596550442dd41b70e1a78ac822471ab955949e6b，PASS。剝除 23 新唯讀列後，全部 9030 原 337 humans 列與 30 張 PNG 保持，100M 終态與原圖逐位元保持。第一次只有建置失敗，因漏 IRQ 輔助檔；尚未執行原版。補齊依賴後蒐證一次。驗證腳本的 Strader 加零長度修正為 8，直接核對既有收據，不重跑原版。

**已證實**：95M／97M／100M 原名稱頁表完整165bytes相同，SHA-256 f3dc28cf153edf625b154c5864b3040cddd52fc9ade2a0cfd2a256b80789d0a7。原 DS188／base0／limitFFFFFFFF，DS188:26C480 pointer298848、DS:29BE0E count3／bias0／stride55。index1原位址29887F，完整55bytes：
`1101E1007301FD000700000000000000000000000000000000000000000000000000000000000000000000000000000000000000002800`
其範圍273／225／371／253；320／239與321／239只命中index1。95M原PNG人工確認Enter Ruler Name／Strader／ACCEPT，PNG SHA-256 e1c739f5aeaf47a6cfdca4749b14509ec2592b65974fa6669e9347c491a52a36，RGB SHA-256 2646a7ef25939869ede60aa35bcd97d649b906fd2cc287d08deb6c2d8058cc12。按鈕與index語意在消費前保持強推論。

**已證實，raw 候選**：index2原位址2988B6的+24 dword為28439D，該原位址32bytes為
`5374726164657200000000000000000000000000000000000000000000000000`。
95M／97M另見29871C、2BD9BC、2BDAAC、506994字串；100M的29871C已不相同。EDI28439D與ESI2843A5亦由原暫存器取得。表指標、原bytes與名稱顯示相互對應，作「文字編輯候選」為強推論；正式持久player／save名稱producer仍未知。

**固定95M初態**：原EIP215D9E，R=[A0 178 2C0864 7 2BD94C 2BD970 2843A5 28439D]、段=[8 188 188 0 20 188]、flags202h、IF開、FPU127F／status0／depth0。VBE Active=true／Bank4／StartY0／DisplaySets48，callback mask2B／target8:2136D1／pending0／非活動／started=completed=8；IRQ非活動／非failed／started=completed=21395。完整RAM65818624bytes SHA-256 6272fad6da4146af26f7513cba625ec2ad2a040d75cdf31cc2b332cffd7a463c與核心／FPU／VBE／callback／IRQ的唯讀不突變核對通過。97M只作原表與buffer交叉核對，不是重擲或輸入備援。

READY後明示 DOSGOLEM_MOO2_RULER_NAME_ACCEPT_CLICK=1，只接受值1與完整RACE_HUMANS_CLICK依賴；固定95000000按下 x640／y239／buttons1，原320／239。原表165bytes／header／DS、32bytes候選／RGB與完整R／六段／EIP／flags、callback8／8、IRQ21395／21395先核對，不符即停止。正常回呼完成且至少20ms後首次可送放開 x642／y239／buttons0，原321／239；不輸入文字或代寫結果。

此READY足夠驗證原名稱頁正常確認與後續畫面；未證實的持久名稱writer不阻塞局部正常按鈕驗收，仍不得稱為typed名稱／存檔鏈已閉合。第一筆20DDDB若未命中，保留未知，不因下一頁出現而捏造store或名稱writer。原版各情境只重生一次；六舊基準保持後，限定CONFORMED只套用實際結果。

## 338 首次確認的契約勘誤

首次正常確認收據 workplace/moo2-probe-338-ruler-press-only.txt.gz SHA-256 1c71292dca36c0046c12b09bf1ebba9704492eaa55d72873fb2acf4d515fda44，探針來源 SHA-256 667e3dd2a6a56e59d610ad40f4bd94e418ddaf6ee89faf2b8d611807ebe83e83。固定95000000、167823195微秒正常按下已送出，原版回呼9／9完成。原版同100M無新CPU拒絕，但mask由2B改為1；因舊放開前置仍要求2B，未送放開，原store未命中，仍是Enter Ruler Name。首次PNG SHA-256 6d063dee7fd1b212ea6615a64aee566103c2e879fc5f0ab2d9cb759330584bd8，RGB SHA-256 e5b2bb70f61172adf70b9bdd222fbab9408353adf3f3fbe61de71687589b372d。這是輸入探針前置過嚴，不能稱為原版名稱確認成功或CPU錯誤。

原int33事件契約回查 internal/machine/le_mouse_callback.go 的 InjectMouseEvent：位置改變設flags1，左鍵放開設flags4，flags&mask非零才排回呼，但一般按鍵／位置狀態仍更新。這是既有平台契約，沒有修改其程式。原版mask1支持位置事件，固定放開x642相對原x640會產生flags5，5&1=1，能經同一正常入口完成放開與回呼。mask1不阻止實際裝置按鍵放開；先前把註冊遮罩當作裝置輸入禁令錯誤。

規格已回到DRAFT審查。修訂READY契約只改本明示情境的放開可用條件：按下前仍要求mask2B；按下已送出且回呼完成後，放開允許同target的原mask1或2B，保留IF、IRQ、pending、至少20ms與第一次可送時放開的條件。座標、日期、95M初態、其他事件、CPU／平台與100M cap不變。若原mask不在1／2B，仍停止等待並明示未放開，不猜測其他遮罩。原版控制流對mask1的寫入端仍未定位，不阻塞已有正常輸入契約。

六條舊情境由首次來源667e3dd2各重生一次。修訂只位於rulerAccept旗標分支，剝除338新增區塊後其餘探針完整來源必須逐位元保持337 HEAD；不重跑六條已完成基準。新版重跑CLI與新ruler情境，原版結果以新收據核對，不覆寫或冒充首次未放開的收據。

## 限定驗收與目前邊界

限定驗收：原名稱頁／原候選bytes／正常名稱ACCEPT／旗幟頁／六舊基線保持。原版正常確認後已到SELECT BANNER COLOR；共享20DDDB未命中，持久名稱writer仍未知。沒有代寫名稱、EIP或原選擇；未改CPU／平台，不外推完整開局或remake同狀態。

首次7情境由乾淨417根檔各重生一次，Docker外層900s／2GiB／2CPU／128pids／UID1000／network none；原版各條仍100M cap。初版只按下未放開，來源667e3dd2與收據已保存，前置過嚴分類見上方勘誤。修訂只位於rulerAccept明示分支，唯讀來源審查證實除此一行外逐位元相同；六舊情境不會進此分支，其收據維持有效。新版僅重跑新ruler與CLI一次，Docker外層300s，同樣乾淨417根檔／官方EXE與100M cap；沒有重擲種子、延後輸入時點或新增其他事件。go build -p 2 -buildvcs=false -o /tmp/moo2-probe ./workplace/moo2-probe，Go1.24.13映像與原ZIP／patch唯讀保持。

python3 workplace/new-game-338-verify.py PASS：六情境全部3847／4829／6769／8149／7752／9030原337列除既定正規化保持，全部162PNG逐位元保持；新ruler固定95M額外輸入前全部7523原列保持獨立humans。原165bytes表／核心／候選32bytes／RGB前置與私有95M獨立收據吻合，不把正常輸入後不同狀態稱為同狀態對拍。

**已證實，正常按下／放開**：press固定95000000、virtual_micros167823195、x640／y239／buttons1、mask2B；release實際95015426、virtual_micros167870767、x642／y239／buttons0、原mask1，差47572微秒。原座標320／239與321／239只命中名稱頁index1；回呼9／9完成且至少20ms後首次可送放開，後續callback10／10完成、pending0／非活動，mask回到2B。此正常輸入已由原版消費；沒有命中首筆共享20DDDB，不能捏造index1寫回或正式名稱store。

**已證實，原旗幟頁**：同100M cap到原228E0E，無新CPU拒絕。終態R=[37 EE AF 4B 2BD9B8 2BDA14 2C0B20 2C0390]／段=[8 188 188 0 20 188]／flags212h，callback10／10與IRQ22854／22854完成且非活動／非failed。640×480原PNG人工確認SELECT BANNER COLOR與八種旗幟。PNG workplace/moo2-vbe-338-ruler.png SHA-256 96efbd1ce6538c27b019cc6713fe7397d6d0fde7a63c7e01cb82023f75614c67；RGB SHA-256 8c2c573c08ebf4bce0b4e166d4268fb6fcf90f35dd27fbbb2de614b26f187f15。

原100M旗幟表count10／pointer298848／bias0／stride55，完整550bytes SHA-256 978afb91aaed9e0cb37672a66352e2ae515b382d5b6f472da9238e09fb650dfb、readonly=true。index1..8範圍對照八色為強推論，index9原5000／5000矩形用途未知。只保存作下一正常選擇來源；100M終態不能當相同cap內的輸入前置，尚未選色。

python3 workplace/new-game-338-cli-verify.py PASS：17無效值／缺依賴在讀EXE前exit2，有效正對照通過閘門後缺EXE明確失敗；同新版binary舊337 CLI的16拒絕與正對照保持。新probe SHA-256 1cc5f74e48b63017b4dc68cfe714b2bec674e7adba0c47d8f119f095583f58c5；CPU SHA-256 967de02753e0dce276413fe67a085e6df16789b52c948999d30ed26c3bb4e6a4，CPU／startup／provider／matcher／mouse callback逐位元保持335。沒有重跑Go全套；335固定EXE全套不能外推整款玩法。

| 本機忽略收據／腳本 | SHA-256 |
| --- | --- |
| workplace/moo2-probe-338-input.txt.gz | 47172f944786e4d134091485ce73ae64d7df622fb75470fa62ddf7a660526e9e |
| workplace/new-game-338-input-verify.py | e29103c693b3536da56b9081c75e8edf297776982fb217a4d0688ecdbb730544 |
| workplace/new-game-338-input-tests.txt | b26cb2cb901c0930689c1c17596550442dd41b70e1a78ac822471ab955949e6b |
| workplace/moo2-probe-338-baseline.txt.gz | 94e4e8e18e6cb21fca2a263bf1431380eb5ea7d6db7f5e83b5bf1619390a8c49 |
| workplace/moo2-probe-338-click.txt.gz | 5afe2ab6f88283b926beaca6352e14eb87f581fcf4179082a488c55c0b5ada6b |
| workplace/moo2-probe-338-extended.txt.gz | 4991ba7afda8c7343e7d450d5292a8f44e54ef3196d72876cd38ff85f47191f5 |
| workplace/moo2-probe-338-ready.txt.gz | 260c7e5a372ffa46e1cb8cbc0c073596eacd8252605765c46a3f453b10ba93b9 |
| workplace/moo2-probe-338-accept.txt.gz | cd4ac24f40c5e6ab27afd6e9aa2604fce6ae95b636e710d385dc4b831642db8a |
| workplace/moo2-probe-338-humans.txt.gz | cd9b796834045d35ddab5afce9e40d791d0836c23e4cef6f454b566930ebe91c |
| workplace/moo2-probe-338-ruler-press-only.txt.gz | 1c71292dca36c0046c12b09bf1ebba9704492eaa55d72873fb2acf4d515fda44 |
| workplace/moo2-probe-338-ruler.txt.gz | 21e94e53b1a4f43b3a6fe736490412ea291bb2c3796c922c1940a6dbd2a5dcbb |
| workplace/new-game-338-verify.py | c507578eda62b34522c882e0f1a97951c10f5adb384e00fc06a96b7e24e52300 |
| workplace/new-game-338-parity-tests.txt | 79a5f7d0444a38afad151dc09544a1aea21339fb5ef4be8f7a36b6a5e6c531e6 |
| workplace/new-game-338-cli-verify.py | 872069a6fcdfe31ae5779494ff879d529d84c656a8889f4a361f2d722aaa1e79 |
| workplace/new-game-338-cli-tests.txt | c5d03a8a5858cdeb915036ef21e2a8f0f3627b6007bfbe3e54c3745fb624b6b1 |
| workplace/moo2-name-338-input.go | 9aa51fcddc4ffdc44896424f3bd50757c2cf49b52390dbc90550dd130faf812a |
| workplace/moo2-name-338-press-only.go | 667e3dd2a6a56e59d610ad40f4bd94e418ddaf6ee89faf2b8d611807ebe83e83 |

三份原RAM的65818624bytes各自唯讀保持；95M SHA-256 6272fad6da4146af26f7513cba625ec2ad2a040d75cdf31cc2b332cffd7a463c，97M SHA-256 6e1f1ea729118153c95e716b1be96dc072d2208942d7d42510872fcc4c561cad，100M SHA-256 5de3d135522bb1a1b08d701b884b4547f287a57aba57bb10e6500a1cbf6ad673。這些是初態蒐證收據，各次RAM不冒充跨次逐位元相同。

下一步唯讀取得本ruler情境較早旗幟頁、原10筆表與正常選色前置，再依新READY規格送一次可重播色彩fixture。預設名稱正常確認與下一旗幟頁已驗；持久名稱writer、旗幟選擇、完整開局、正式RNG及remake同狀態未知，主庫RE-first保持。不追renderer／helper，不重開已完成名稱放開或SCASW。原版LOG／PNG／RAM留忽略workplace，不公開。

75項規格回填正對照、新338 26個缺證據負例及337 27個負例、兩個CLI通過；私有workplace/new-game-338-backlink-tests.txt SHA-256 7ceda5b0175a4d41a30c89cccde10c9d1ff6940241d4d8c41460c7459cf8e042。
