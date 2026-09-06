# 規格030：EOB1 LEVEL1首次場景拾取

狀態：**CONFORMED**

## 證據與契約

由規格028的正常LEVEL1入口，使用原版DOS場景按鈕定義的左近區域
`(0,102) 88×18`，在中心安全點`(44,110)`送出一次左鍵：

- 第一人稱安全矩形`(0,0) 176×120`由入口SHA-256
  `2ef2c0240070bce02b59735c5266fc6163eee170ea8c135982a469f04bb2abbc`
  變為`1d123d4fe9a5a1001446f2d180601e8713fa2dd02f00caac4c9f406f31b59375`；
- 完整320×200色號SHA-256為
  `e282a5980cb8ce9d5fee0dc64b6d66ad399cabf3e3bae6edb1cde64e6618909f`；
- 人工檢視可見左側地面石塊消失、游標持有石塊，底部訊息為`ROCK TAKEN.`；
- 鍵盤port `60h`讀取仍為84次，證明拾取沒有混入額外鍵盤捷徑。

`ToLevel1FirstPickup`必須從冷啟動完成四人建角並按PLAY，不能載入存檔、注入物品、
直接修改queue或跳入LEVEL1。點擊採零額外hover、200,000道指令hold及1,000,000道
指令settle，避免地城游標重設吃掉玩家輸入。

## 來源定位

ScummVM EOB參考來源`resource/staticres_eob.cpp::initButtonData`保存DOS左近按鈕
`{x:0,y:102,w:88,h:18,arg:0}`；`gui/gui_eob.cpp::clickedSceneDropPickupItem`
保存空手時從目前block／position移除物品、設為hand並以flag 8執行level script的consumer。
本規格的玩家可見結果另由同版本原版bytes實際重播證實，不把參考來源單獨當成parity。

## 限制與停止線

本規格只閉合LEVEL1入口第一件石塊的正常滑鼠拾取，不外推物品ID、queue順序、放下、
裝備、投擲或其他場景按鈕。原版截圖只供本機人工檢視，不提交或散布。
