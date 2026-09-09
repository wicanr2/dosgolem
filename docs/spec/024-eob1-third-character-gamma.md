# 規格024：EOB1第三角色GAMMA完成checkpoint

狀態：**CONFORMED**

## 證據與契約

第三角色檢視頁點`KEEP (282,180)`後，原版姓名頁完整色號SHA-256為
`95118f4657f2096ec0a4fa0ac8ddfafd0ae162c3351b2d7d82ecb5856190b06f`。

姓名頁依序接受Set 1 `G=22h／A=1Eh／M=32h／M=32h／A=1Eh`；每字間隔250,000道指令，
Enter確認後返回四槽總覽。第三姓名列安全矩形`(5,160) 70×16`固定為
`84db489bbd3ecd5cc1050df46382eb3468083941f2fcf2584907c85af15bdef1`。

`ToThirdCharacterName`與`ToThirdCharacterGAMMA`必須走既有正常鏈、KEEP click與硬體掃描碼；
真實資料測試核對姓名列及累計三十二鍵make／break、六十四次port `60h`讀取。

## 限制與停止線

本規格閉合第三角色fixture，不外推第四角色或Play。原版資料／畫面不提交；真實資料、人眼檢視與
全庫回歸通過後才維持CONFORMED。
