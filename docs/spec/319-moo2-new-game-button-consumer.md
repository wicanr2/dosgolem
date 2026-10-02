# 319：NEW GAME按鍵消費的唯讀觀測

狀態：**CONFORMED**
日期：2026-10-03
範圍：318相同初態／輸入的按鍵讀取與寫後觀測。

## 證據與契約

固定官方1.31 EXE SHA-256 4e11be14217b4aafa1839f333bf5eba037f98b0c44e9e4752c96c464c260419f。[318-moo2-new-game-normal-click.md](318-moo2-new-game-normal-click.md)兩正常CB返回已證，原始高位LE213741的66 A3 1A 12 2A 00是word目的2A121A，但未擷取寫後byte或主選單讀取；50M終圖仍選單，不推定按鈕已啟動。

沿318固定46M Esc、1996-01-01、50M、display40後按下、至少20ms後移動放開，沒有新設定或新輸入。按下時才包裝CPU既有SegmentRead8與SegmentRead16，原函式非空時只呼叫一次，保留value／ok及其他observer。普通flat讀取的委派返回false，CPU才讀Bus，故不能以委派ok判斷有無讀取。只在實際CPU.Step期間、非活動mouse callback、有效descriptor且8／16位元請求涵蓋2A121A..2A121B時標記；Step後至多輸出32筆請求、原始指令前後EIP／bytes、R／六段／flags、請求時原始word及error。error=nil才算實際指令成功。保持Bus身分，不包裝或改寫它。

既有startup會覆蓋段掛勾，故在正常按下時才安裝，沿已存在OR觀測鏈，避免初始化覆蓋。32位元直接Bus讀取不在此觀測涵蓋範圍，不因零筆8／16位元請求宣稱沒有其他consumer。原欄位位於一般RAM，快照只讀m.Mem兩byte，不返回它作為CPU的值。

另於每個正常CB返回後新增buttons_word原始兩byte收據行；不新增讀寫遊戲狀態。模式關閉不新增收據。原始欄位保持地址，附語意等級已證實僅限回呼word目的；consumer用途待實測。至多32筆不代表完整consumer台帳，零筆只代表此有界續行未觀測到，不推論不存在讀取端。

## 審查與驗收

已核對318原始66 A3 bytes、startup掛勾覆寫、CPU readSegment8／readSegment16的Bus fallback與既有OR鏈。只轉交委派一次、只讀descriptor與m.Mem，沒有改CPU／平台或輸入。修正觀測契約後審查READY。驗收重生同318點擊流程；除新增read與buttons_word行外，全部318原始列逐列保持，兩CB／191筆、終圖保持。實際寫後及按住期間讀取結果照實保存。314全套保持適用。索引docs/spec/000-index.md，318未知同次回填，原始資料與PNG／LOG留忽略workplace。

## 正式收據與觀測邊界

59回填函式、32缺定位／原始收據／正對照未知／狀態／回填負例與--check-menu-click-spec-backlinks通過。三新規格及所有修改來源／收據1000:1000；工具root-owned與異形.md目錄空。21:35:20 UTC本輪有界容器已退出移除，唯一剩餘Go容器掛載為fd2，保留未操作。沒有修改CPU／平台，沒有發布素材或宣稱remake完成。

Go1.24.13既有固定映像，600s／2GiB／2CPU／128pids／UID1000／network none，417根檔乾淨重建，與318完全相同輸入／初態。go build -p 2 -buildvcs=false -o /tmp/moo2-probe ./workplace/moo2-probe後執行原版，沒有新輸入或提高cap。

第一輪SegmentRead8觀測在startup前安裝，且把委派ok=false當成未讀。核對le_startup.go與cpu.go才確認startup覆蓋及一般RAM fallback，該輪不可用來判斷有無consumer。原始控制收據保留workplace/moo2-probe-319-segment8-control.txt.gz，SHA-256 eb850aa6e18dc4faf47134368dd4289962a2b9153cef24cf1ff940c5a31ab617；來源8ebc20fbda2a569bec011e07f640cf470e8110eef8230f416aed804a41f94d58。這是觀測工具缺口，原版CPU／正常輸入沒有失敗。

修正契約並審查READY後，同映像／同命令／同輸入乾淨重跑。來源SHA-256 2cab77b5f3ddcb4a6dcc024e4a0b07462ee63a8be660ab662d71fb46ee4f6a10。只新增兩寫後列：
- outer_step49882522 completed1 buttons_word=0100。
- outer_step49883496 completed2 buttons_word=0000。

全部318原始列除既定mtime／DTA四bytes／PNG路徑外逐列保持，包括191筆真正callback步、兩次CB完整核心恢復、50M終態2385AF與時計64282188。終圖逐位元保持0c45ba73ddfe850693520f5aee093c1c188ab118df9c51570521cfe3fc0ddad9。私有workplace/moo2-probe-319-click.txt.gz SHA-256 597266974b32440f35818cc672464573202bb997ebcf3dd29a7d2fd65bf8ac9e。

已證實只限兩個原版CB返回後的原始word與觀測不突變。new_game_button_read零筆，讀取覆蓋尚未有真實正對照，consumer仍未知。不能推論按住期間沒有任何讀取，更不能宣稱主選單未消費按鍵。32位元直接Bus／其他segment路徑與觸發正常遊戲設定均未驗。限定CONFORMED只涵蓋寫後／基線保持；讀取端下一步先加真正請求數及路徑正對照，不先換座標、輸入時點或提高cap。
