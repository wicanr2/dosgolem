# 規格014：EOB1第一角色陣營選擇checkpoint

狀態：**CONFORMED**

## 證據與契約

在CONFORMED第一角色`SELECT CLASS:`頁直接按Enter，原版進入
`SELECT ALIGNMENT:`，列出Lawful／Neutral／Chaotic與Good／Neutral／Evil九種組合。

左側角色槽星光持續循環；右側矩形`(138,60) 170×130`完成後色號SHA-256固定為
`aade7402dce32dc8a71b9ca7d1ed057836487dc67b27e0fc482cef512da62b40`。adapter只以此安全矩形
定位，不要求全畫面靜止。

`apps/eob1.ToFirstCharacterAlignment`由冷啟動走既有`ToFirstCharacterClass`，送一個Enter後以
有界預算等待安全矩形。真實資料測試核對雜湊，以及Escape／Down／四個Enter共六鍵、十二次
port `60h`讀取。

## 限制與停止線

本checkpoint只證實職業頁進入陣營頁，不把清單順序推定為已選職業；不代表選定陣營或完成屬性、肖像、姓名。原版資料
與畫面只作本機研究；真實資料測試、十二次port `60h`讀取及人眼檢視均已通過，本規格升為
CONFORMED。

## 2026-09-07 勘誤

後續屬性頁實際顯示職業為`MAGE`，推翻原先僅由清單第一項推定的「預設Fighter」。保留頁面
轉移與掃描碼證據；角色選項身分只能由後續可見consumer或直接狀態證據確認。
