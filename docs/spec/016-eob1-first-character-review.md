# 規格016：EOB1第一角色檢視操作checkpoint

狀態：**CONFORMED**

## 證據與契約

在CONFORMED第一角色屬性／肖像頁按Enter，原版進入角色檢視操作頁。畫面保留Elf Male Mage、
六項屬性與肖像，右下明確出現`REROLL`、`MODIFY`、`FACES`、`KEEP`四個按鈕。這證實上一頁不是
最終確認或姓名輸入。

右側矩形`(130,55) 180×140`完成後色號SHA-256固定為
`1f1bbba36dc25a84f6ce2ebb4f01a60dc64d2b7ddc63bb9ffa3d25d8e47c164f`；左側角色槽循環效果排除。
`apps/eob1.ToFirstCharacterReview`由冷啟動走既有屬性頁，送Enter並以有界預算等待安全矩形。
真實資料測試核對雜湊及八鍵make／break共十六次port `60h`讀取。

## 限制與停止線

本checkpoint只證實檢視操作頁，不猜測目前鍵盤焦點，也不代表Reroll、Modify、Faces或Keep已操作。
下一切片須先證實鍵盤如何選到Keep，再進姓名頁。真實資料測試、十六次port `60h`讀取與人眼
檢視均已通過，本規格升為CONFORMED。
