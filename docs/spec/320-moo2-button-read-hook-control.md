# 320：按鍵讀取掛勾的真正請求正對照

狀態：**CONFORMED**
日期：2026-10-03
範圍：319既有探針的唯讀覆蓋檢查，不改輸入／CPU／平台。

## 證據與契約

工具062670051203076ff688d36a390f46dd8a7883c6。固定1.31 EXE SHA-256 4e11be14217b4aafa1839f333bf5eba037f98b0c44e9e4752c96c464c260419f。[319-moo2-new-game-button-consumer.md](319-moo2-new-game-button-consumer.md)兩CB後word0100／0000已證，但目標讀取零筆且無真實正對照。不能從零筆判斷沒有consumer。

沿319固定46M Esc／1996-01-01／50M cap、display40後單次按下與至少20ms後移動放開。只在明示點擊模式、實際CPU.Step中統計現存SegmentRead8／16真正請求，分原版mouse callback與非callback。保持每個委派至多一次、原value／ok、Bus身分與OR hook鏈。各寬度最多擷取兩個非callback有效RAM請求的selector／offset／linear／原始byte、真正指令前後EIP／R／段／flags／error，合計最多四筆。沒有測試指令注入或假裝正常請求。

正常按下後記錄新掛勾的reflect函式code位址，收尾只輸出8／16位元code是否仍匹配，不輸出宿主位址。此比較只證code位址，不是完整closure身分或完整Bus讀取覆蓋。統計與控制樣本都只讀狀態，不把取樣byte作為CPU返回值。原版decoder後續未走委派的32位元或直接Bus仍是能力邊界。

## READY審查與驗收

已核對cpu.go readSegment8／16在委派false時才走既有Bus、319安裝時點與OR鏈；統計不影響控制流。足以READY。重生一個同319正常原版流程，除新增正對照／統計收據外，全部319原始列及終圖逐位元保持，含兩CB／191步與word寫後。有有效正常樣本才宣稱該委派路徑正對照；零請求或code不匹配照實記工具缺口，不換輸入或提高cap。CPU／平台來源不改，314全套仍適用。原素材／完整LOG／PNG留忽略workplace，索引docs/spec/000-index.md；319未知同次回填。

## 正式收據與限定結論

轉換後的事件讀取已由[321-moo2-menu-mouse-event-consumer.md](321-moo2-menu-mouse-event-consumer.md)補證：正常七讀／原版清除／28續行，放開後仍讀按下時事件與座標；高層NEW GAME設定轉移仍未知。以下320正對照收據保持。

Go1.24.13固定映像，600s／2GiB／2CPU／128pids／UID1000／network none，417根檔乾淨重建。go build -p 2 -buildvcs=false -o /tmp/moo2-probe ./workplace/moo2-probe後，以319完全相同條件執行/tmp/moo2-probe /tmp/game/ORION2.EXE --game-dir /tmp/game。CPU／平台來源保持314。

真正統計normal8=103537／callback8=59／normal16=11288／callback16=15，positives8=2／positives16=2，target_samples=0，hook_code_matches=true/true。normal欄位只表示無活動mouse callback，也包含平台框架讀取，不泛稱全部為原版玩家consumer。

四樣本中49882421的8:2136D1／byte1E是派送器validTarget讀取，非原版客體指令；49882523的原始228CA9／61 POPAD，讀SS188:2BDB40／byte07並自然到228CAA。兩真正word原版指令為：
- 49882543，高位LE234AC5的66 8B 1D 38 3A 2A 00，讀188:2A3A38／F401，EBX0→1F4，下一234ACC。
- 49882546，高位LE234AD5的66 8B 1D 36 3A 2A 00，讀188:2A3A36／E500，EBX0→E5，下一234ADC。

全部error=nil、完整R／六段／flags在原始收據；已證實原版在按下後讀取畫面500／229座標。只新增四正對照／一統計列，全部319原始列與終圖保持。來源SHA-256 f15e65085139a99bb25f2656159d51d7fcd7b4c17942cdb5785982a8638d6643；私有workplace/moo2-probe-320-click.txt.gz 88d0b595b250003fd11e4c1f311a23abac74cbe2605f80ebd7f4078cbb6b298c。終圖仍0c45ba73ddfe850693520f5aee093c1c188ab118df9c51570521cfe3fc0ddad9。

CPU readSegment32也先嘗試SegmentRead8；一般RAM委派false後直接Bus。因此normal8包括dword取值首byte嘗試，不能用計數分出完整實際byte／dword流量。沒有獨立Bus觀測仍保留覆蓋邊界，不推論所有按鍵consumer不存在。

已保存的原版callback本身實際在213D61／213D71讀2A121A，之後213D7C寫2A1222、213DA2／213DAB寫word1到2A1228／2A1220、213DBA／213DC6寫座標到2A121C／2A121E。319刻意排除活動callback，故零筆不代表按鍵值沒有被原版消費。下一步追這些真正產生的事件欄位與2A11EC更新的讀取端，保留原始地址與未知用途，不再只查瞬時buttons word或換輸入。
