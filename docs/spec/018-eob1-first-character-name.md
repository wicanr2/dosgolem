# 規格018：EOB1第一角色姓名輸入checkpoint

狀態：**CONFORMED**

## 證據與契約

在CONFORMED第一角色檢視頁，`KEEP`按鈕的原版320×200中心安全點為`(282,180)`。以`Click`送出
移動、左鍵按下與放開callback後，原版接受目前Elf Male Mage，將肖像放入左上角色槽，右側顯示
`Name:`及閃爍輸入游標。

姓名頁完成後整張色號畫面穩定，SHA-256為
`ae05e3fff97f306ec714c79603509ec45a4778f152d8ccca2943faef23c47a89`。`apps/eob1.ToFirstCharacterName`
由冷啟動走既有檢視頁、點擊KEEP並以有界預算等待該畫面；Click無回應或checkpoint不到均回錯。

## 驗收與停止線

- 真實資料測試核對全畫面雜湊及原版三次`AX=000Ch`callback註冊。
- 人眼檢視確認左上肖像與右側`Name:`輸入頁。
- 本checkpoint不代表姓名字元輸入、確認角色或其餘三名角色完成；下一切片須先證實原版姓名的
  掃描碼／字元consumer。真實資料、callback合成測試與全庫回歸均已通過，本規格升CONFORMED。
