# 規格032：EOB1 LEVEL1 CAMP根選單生命週期

狀態：**CONFORMED**

## 證據與契約

由規格028的新隊伍正常LEVEL1入口，點原版DOS CAMP按鈕定義`(289,177) 31×21`的中心
安全點`(304,187)`：

- 左側地城視窗被`Camp:`根選單覆蓋；七列依序是Rest Party、Memorize Spells、
  Pray for Spells、Scribe Scrolls、Preferences、Game Options、Exit；
- 右側四人隊伍、底部方向控制／羅盤／訊息框與CAMP按鈕保持可見；
- 第一列以紅字選中；完整320×200色號SHA-256為
  `a9f5ef56e878a83df3767854dba801c32f28d475232fb8dd5bd40b0565b402f7`；
- 此時鍵盤port `60h`仍84次，證明進場只使用滑鼠callback。

由根選單送Set 1 `Down=50h`後：

- 紅色選擇移至第二列Memorize Spells，第一列恢復白字；
- 完整色號SHA-256為`67c50f4e57e89ad2ed046c4b10a9717ee8a3349f16cdfbc9b2342937c29db5a0`；
- port `60h`為86次，恰增加一組make／break。

再送`Escape=01h`後，左側恢復原本LEVEL1入口場景，176×120視窗SHA-256為
`2ef2c0240070bce02b59735c5266fc6163eee170ea8c135982a469f04bb2abbc`，port `60h`為88次。
這證明選擇與返回由CAMP input owner處理，返回時恢復父層場景，不是另建地城狀態。

## 原版來源與實作邊界

ScummVM EOB參考來源`resource/staticres_eob.cpp::initButtonData`保存CAMP矩形；
`gui/gui_eob.cpp::clickedCamp`保存備份左側場景、執行CAMP、返回後恢復／重畫與重新啟用timer的
owner流程。玩家可見結果另由同版本DOS bytes以dosgolem正常重播證實。

本規格只固定根選單presentation、Down selection與Escape restore；不外推七個child panel的
交易、休息時間、記憶／祈禱法術或存讀檔語意。原版截圖只供本機人工檢視，不提交或散布。
