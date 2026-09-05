# 008 — EOB1 原版觀察 adapter

狀態：**CONFORMED**（Westwood標誌checkpoint）  
日期：2026-09-07

## 1. 目的與範圍

`oracle`是作品中立的執行／觀察契約；`apps/eob1`只保存EOB1正常啟動器的導航知識。
本切片只提供`START1.EXE → VGA → 無音效 → 使用滑鼠 → Westwood標誌穩定幀`，不宣稱
片頭、標題、建角、防拷、存檔或遊戲內狀態已支援。

## 2. 證據

固定輸入：`START1.EXE` SHA-256
`3274f986770203b891f0c4f2ae18c457e602f54be7955d983427fa7ce548fa96`，依序鍵入`4`、`4`、`Y`。
dosgolem的5,000,000步畫面與DOSBox-X 2026.07.02正常路徑的27個穩定幀逐像素相同；RGBA
SHA-256皆為`f49fee3af72c50fad10fb0708436e6733b2cfbe068dcfc423f1a4efc0065806e`。

載入進度的可觀察路標是首次開啟`EOBDATA4.PAK`；在此之後等待畫面連續1,000,000道指令
不變，得到穩定標誌幀。單靠固定總指令數不是checkpoint契約。

## 3. 契約

- `ToWestwoodLogo(*oracle.Oracle) error`排入`44Y`，以有界預算等待`EOBDATA4.PAK`及畫面穩定。
- 任一程式退出、CPU錯誤、預算耗盡或路標未出現均回錯，不輸出成功收據。
- `apps/eob1/cmd/logo`只讀玩家提供的EXE／資料根，寫入指定輸出目錄：
  `indexed.bin`（64,000 bytes）、`palette.rgb`（768 bytes）、`frame.png`及`receipt.json`。
- JSON只記錄輸入檔basename與SHA-256、步數、artifact雜湊、開檔清單及未實作服務；不得記錄
  私有絕對路徑，也不得嵌入原版bytes。
- 輸出是本機研究artifact，不得由dosgolem或EOB專案提交／公開散布。

## 4. 驗證與停止線

- 純測試固定palette flatten與SHA-256收據欄位。
- 真實資料測試由`EOB1_ORACLE_EXE`／`EOB1_ORACLE_ROOT`選配；存在時必須到達checkpoint，並核對
  indexed、palette與RGBA雜湊。缺少私有資料只能明確skip，不能偽造pass。
- 同一frame已由DOSBox-X獨立比對，通過後本切片升CONFORMED；下一狀態另立窄規格。

## 5. 2026-09-07 驗證收據

- 真實資料checkpoint在2,240,174道指令達成；色號SHA-256為`b4c544…d0e4`，palette為
  `b075ca…6fd`，PNG為`1dd0ac…bd7`，與既有DOSBox-X零像素差frame一致。
- CLI實際輸出64,000-byte色號、768-byte palette、PNG及JSON；測試回讀檔案大小、必要欄位並
  拒絕私有`exe`／`root`路徑出現在收據。
- 帶真實EOB1環境變數的dosgolem全庫`go test ./... -count=1`與`go vet ./...`通過。

## 6. 有效存檔法術書中止／關閉擴充

- `ToSavedGameProtectionBookClosed`沿用有效`EOBDATA.SAV`的正常標題載入、CAMP祈禱、休息及
  TENMIYANA聖徽開書路徑，不注入法術或玩家狀態。
- 開書左側`176×168`色號SHA-256為`1f985614…cf92`；底部`ABORT SPELL`是原版正式中止控制。
  點擊安全點`(102,172)`後一次關閉法術書並返回探索，左側`176×168`為`b1dd7f80…645b`，
  地城視窗`176×100`為`689c6d86…00a`。
- 目標模式的Escape、Z及Enter都會被鍵盤處理常式消費但不改畫面；adapter不得以這些鍵假裝
  完成原版中止。正常交易累計port `60h`讀取32次。
- 真實資料測試`TestToSavedGameProtectionBookClosedRealData`固定上述安全矩形與輸入次數；原始
  存檔唯讀，動態肖像不納入永久全畫面雜湊。

## 7. LEVEL1 CAMP Game Options生命週期

- `ToLevel1CampGameOptions`由冷啟動正常建立ALFA／BETA／GAMMA／DELTA四人、按PLAY進LEVEL1，
  再點原版CAMP按鈕；不使用存檔、direct-entry或預製隊伍。
- CAMP根選單以五次Down及Enter選取第六列Game Options。全畫面色號SHA-256為
  `2207602e…d912`，port `60h`累計96次；原版畫面可見Load Game、Save Game、Drop Character、
  Quit Game與Exit。
- `ToLevel1CampGameOptionsExit`先按Escape回CAMP根選單，再按Escape回LEVEL1探索；最終地城
  `176×120`為`2ef2c024…abbc`，port `60h`累計100次。
- 可丟棄相鄰實驗在文字列附近點`(80,111)`與`(80,102)`都直接關閉CAMP，沒有開啟Game Options；
  因此adapter不猜測原版滑鼠命中矩形，只把已證實的滑鼠開CAMP＋鍵盤子選單路徑列為契約。
