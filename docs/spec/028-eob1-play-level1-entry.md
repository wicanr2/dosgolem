# 規格028：EOB1四人隊伍PLAY至LEVEL1入口

狀態：**CONFORMED**

## 證據與契約

第四角色DELTA的姓名安全矩形會在處理該鍵盤事件的callback尚未返回前成立。若立即再送PLAY滑鼠
callback，原版CPU會形成巢狀callback，之後在`133E:1E21–1E58`腳本解譯迴圈以遭污染的segment
取得全零資料，永久停在`Entering game. Please wait.`。這是oracle輸入checkpoint邊界，不是
EOB1缺腳本。

相鄰實驗證實：DELTA checkpoint後先執行1,000,000道指令讓事件收尾，再以PLAY中心`(65,190)`、
hover 1,000,000、hold 200,000、settle 1,000,000點擊，原版會從PAK載入LEVEL1資料並完成第一格地城、
四人HUD及CAMP。第一人稱視窗安全矩形`(0,0) 176×120`色號SHA-256為
`2ef2c0240070bce02b59735c5266fc6163eee170ea8c135982a469f04bb2abbc`；記憶體可見
`LEVEL1.MAZ`及kobold資產名稱。`LEVEL1.MAZ`不是獨立open事件，不得作`Opened`條件。

`ToLevel1Entrance`必須由冷啟動走完整四人建角，不得注入角色、位置或直接載入關卡；PLAY後依
實驗的1,000指令粒度跑20,000批初始化，再以30,000,000道指令上限等待第一格安全矩形。真實資料
測試核對視窗與累計八十四次port `60h`讀取。

## 限制與停止線

本規格閉合原版正常新隊伍進入LEVEL1第一格，不外推移動、戰鬥、施法或存檔。原版資料與畫面不
提交；本checkpoint只作後續同狀態對拍oracle。
