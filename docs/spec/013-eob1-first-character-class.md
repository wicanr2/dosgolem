# 規格013：EOB1第一角色職業選擇checkpoint

狀態：**CONFORMED**

## 證據與契約

在CONFORMED第一角色`SELECT RACE:`頁直接按Enter，原版接受預設`HUMAN MALE`並進入
`SELECT CLASS:`，列出Fighter、Ranger、Mage、Cleric、Thief及複合職業，右下有Back。

左側角色槽星光持續循環；右側矩形`(138,60) 170×130`完成後色號SHA-256固定為
`731c3a60b6eec89515314cf85aa352bcd4efad241d3aea7bb5106454aee7b890`。因此adapter不等待
全畫面靜止，也不把左側動畫相位納入checkpoint。

`apps/eob1.ToFirstCharacterClass`由冷啟動走既有`ToFirstCharacterRace`，送一個Enter後以有界
預算等待上述安全矩形。真實資料測試核對矩形，以及Escape／Down／三個Enter共五鍵、十次
port `60h`讀取。

## 限制與停止線

本checkpoint只證實預設Human Male進入職業頁，不代表已選定職業或完成其後陣營、屬性、肖像、
姓名步驟。原版資料與畫面只作本機研究；真實資料測試、十次port `60h`讀取與人眼檢視均已
通過，本規格升為CONFORMED。
