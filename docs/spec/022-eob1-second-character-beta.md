# 規格022：EOB1第二角色BETA完成checkpoint

狀態：**CONFORMED**

## 證據與契約

第二角色Human Male Fighter檢視頁點`KEEP (282,180)`後，原版把其肖像放入右上槽並進`Name:`頁；
該頁完整色號SHA-256為`837508573e890228667986bb5932f32dd62732d36c152831984ec504c94bba8b`。

姓名頁依序接受Set 1 `B=30h／E=12h／T=14h／A=1Eh`；每字間隔250,000道指令，Enter確認後回
四槽總覽並顯示ALFA與BETA。第二姓名列安全矩形`(75,100) 70×16`固定為
`26e91595e8209faf10fa52e34ea9d00477ee0fd6c823d3aa1432e623d2a2258c`。

`ToSecondCharacterName`與`ToSecondCharacterBETA`必須走既有正常鏈、KEEP click與硬體掃描碼；
真實資料測試核對姓名列及累計二十二鍵make／break、四十四次port `60h`讀取。

## 限制與停止線

本規格閉合第二角色fixture，不外推第三、第四角色或Play。原版資料／畫面不提交；真實資料與
人眼檢視、四十四次port `60h`讀取與全庫回歸均已通過，本規格升CONFORMED。
