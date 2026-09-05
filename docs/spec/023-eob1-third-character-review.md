# 規格023：EOB1第三角色檢視checkpoint

狀態：**CONFORMED**

## 證據與契約

在ALFA、BETA完成後點第三槽`(49,142)`，原版進入第三角色`SELECT RACE`頁；右側安全區
`(130,55) 180×140`色號SHA-256為
`b566dd781770ae6af1cd569cc84d82e37a38fe82b81405da0c979722f6e0041c`。

依序按四次Enter後，原版顯示`HUMAN MALE／FIGHTER／LAWFUL GOOD`，抵達含
`REROLL／MODIFY／FACES／KEEP`的檢視頁；同安全區SHA-256依序為：

- 職業：`31954c4eabf3fe236734a89b17447907ac3c54448a962b4ffd0432a29e2777eb`
- 陣營：`b701f7d8e0738aaf110f702f34a53d3ab9f0c01b60fd0f7e514bdd773a7bafbc`
- 屬性：`9e2cbd7c72e68d21971726b39df27ec139e1550373f76abc90f366f879fa8793`
- 檢視：`832af588ffc848efb145567ac80698b71481fee9da43b2e63892139712998948`

真實資料測試必須由冷啟動走既有ALFA、BETA正常鏈，核對檢視安全區與累計二十六鍵
make／break、五十二次port `60h`讀取。五個階段均另以原版畫面作人眼核對。

## 限制與停止線

本規格只閉合第三角色到檢視頁的fixture，不外推姓名、第四角色或Play。原版資料與畫面不提交。
