# 規格015：EOB1第一角色屬性／肖像checkpoint

狀態：**CONFORMED**

## 證據與契約

在CONFORMED第一角色`SELECT ALIGNMENT:`頁按Enter，原版進入屬性／肖像頁。畫面可見四個肖像、
左右肖像切換按鈕，並明確顯示`ELF MALE`、`MAGE`及STR／INT／WIS／DEX／CON／CHA六項屬性。
這項下游consumer證據推翻前兩份規格僅由清單順序猜測的Human Male／Fighter身分。

右側矩形`(130,55) 180×140`完成後色號SHA-256固定為
`d57c5b9740bf8708984ab7da91f950755a1255d06f0bcc4555500d294f95d42e`；左側角色槽星光循環不納入。
`apps/eob1.ToFirstCharacterStats`由冷啟動走既有陣營頁，送Enter後以有界預算等待安全矩形。
真實資料測試核對雜湊及七鍵make／break共十四次port `60h`讀取。

## 限制與停止線

本checkpoint證實此輸入序列得到Elf Male Mage與可見屬性／肖像頁；目前沒有可見陣營consumer，
不猜測已選陣營。它也不代表屬性重擲、肖像切換、姓名或角色完成。真實資料測試與人眼檢視
及十四次port `60h`讀取均通過，本規格升為CONFORMED。
