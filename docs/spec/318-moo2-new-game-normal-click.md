# 318：NEW GAME的正常滑鼠點擊

狀態：**CONFORMED**
日期：2026-10-03
範圍：既有滑鼠API的單次原版輸入實驗；不改CPU／平台／玩法。

## 證據

固定EXE SHA-256 4e11be14217b4aafa1839f333bf5eba037f98b0c44e9e4752c96c464c260419f。基線cbc63f6加317唯讀觀測。[317-moo2-menu-display40-observation.md](317-moo2-menu-display40-observation.md)第49882420步全見六按鈕，NEW GAME內部畫面500／229，剩117580步。255原版回呼已證CX右移一位、DX原值的座標消費，故正常API座標1000／229；未宣稱熱區已證。

[255-moo2-protected-mouse-callback.md](255-moo2-protected-mouse-callback.md)既有InjectMouseEvent以按鍵轉換／位置變化決定旗標，排FIFO、IF1及無IRQ0活動才派送，真正原版CB與框架返回。主選單已註冊mask2Bh／8:2136D1，可收移動與左按下，不收單獨左放開；放開伴隨兩CX單位正常移動可產生5h事件並沿移動位送到同一回呼。不直接改遊戲RAM。

## 契約與READY審查

明示DOSGOLEM_MOO2_NEW_GAME_CLICK_AFTER_DISPLAY40=1才啟用。執行前固定46M Esc、1996-01-01、50M cap，禁止與既有早期受控滑鼠開關並用，避免混合初態。未啟用時全部317觀測與輸入保持。

在已成功第40換頁後的首個IF1、無活動／待派送回呼、target8:2136D1且mask含移動與左按下的外層邊界，呼叫InjectMouseEvent(1000,229,1,0,0)一次。記錄步數／時計／mask／target／全部R／段／flags。條件未到不強行派送。callback完成後，距按下時計至少20000µs、IF1且無活動／pending時，呼叫InjectMouseEvent(1002,229,0,0,0)一次。沒有連續排兩事件、代寫按鍵或跳動畫。

只在此模式擷取至多256次實際活動callback步的原始bytes、R／段／flags與真實返回，包括CB前後，不為完整driver或玩法反組譯。固定50M cap，若未放開或未轉畫面，照實記未知；不換時點重試。正常流程新拒絕保留bytes／狀態，再以公開CPU／平台規格另開窄任務。

已核對317圖、255座標與既有事件／IF／回呼機制。這是授權玩家路徑的可逆實驗，不改規則；足以READY。驗收先一個新增模式正常原版流程，核對按下前317全部既有列保持、兩真正原版回呼及自然consumer／畫面，與無點擊終態分開。314CPU全套仍適用於不改來源。原素材／PNG／完整LOG留忽略workplace。索引docs/spec/000-index.md，317同次回填。

## 正式收據與限定結論

按鍵寫後已由規格 319 取樣，見[319-moo2-new-game-button-consumer.md](319-moo2-new-game-button-consumer.md)。兩CB後word0100／0000已證，讀取覆蓋未有正對照，consumer與設定畫面仍未知。以下原始318收據保持。

Go1.24.13固定映像，600s／2GiB／2CPU／128pids／UID1000／network none，417根檔與官方EXE乾淨重建。一次go build -p 2 -buildvcs=false -o /tmp/moo2-probe ./workplace/moo2-probe。固定46M Esc／1996-01-01／50M cap，加點擊開關執行/tmp/moo2-probe /tmp/game/ORION2.EXE --game-dir /tmp/game；另重生模式關閉基線。來源SHA-256 79c2430efbb3cdadce214c2e9cc73d147c519bf22fc60725c8544cd593da7bf7；CPU／平台保持314。

已證實：按下outer_step49882421／時計63906833µs，caller高位LE228CA9；回呼8:2136D1，AX3／BX1／CX3E8／DXE5。第49882522步原始2137F2的CB返回228CA9，R4F／0／0／0／2BDB40／2BDB68／0／7、六段8／188／188／0／20／188、flags246h恢復。

放開49883408／63939183µs，相隔32350µs，caller2354D1；入口AX5／BX0／CX3EA／DXE5。49883496同一CB返回2354D1，R18／18／0／0／2BDB18／2BDB48／3C9462／A3E82、六段及flags207h恢復。191筆實際callback步全error=nil；終態pressed=true／released=true／mask1／pending0／activefalse／started2／completed2。

原始21372D的D1 F8確將1000／1002轉500／501；21372F／213738／213741的66 A3目的依序2A3A38／2A3A36／2A121A。只核對原始指令與暫存器，尚無每次寫後快照；不猜主選單消費了按下。

50M到2385AF，R361944／0／96／1／2BDA2C／2BDA84／4FF258／361B40、六段8／188／188／0／20／188、flags202h，無未支援指令，時計64282188。終圖0c45ba73ddfe850693520f5aee093c1c188ab118df9c51570521cfe3fc0ddad9已檢視，主選單與NEW GAME上的游標可見，未見設定畫面。mask1只是目前註冊狀態，不證按鈕已啟動。

按下前全部317原始列，除mtime／DTA四bytes／PNG路徑外保持；模式關閉全部317列與終圖亦保持。兩CB完整R／六段／flags恢復核對通過。私有workplace/moo2-probe-318-click.txt.gz SHA-256 f49d8ab7c7cc6c88c7229d0dfac0d23e01d4d02da4e286086793cba217ab744a；workplace/moo2-probe-318-baseline.txt.gz f3f80f78409e490f1c4a0a0d1ffca00194eab0b0b614d7a61d1f436e3573ab24。

限定CONFORMED只包含正常輸入、兩原版CB與caller恢復。NEW GAME成功轉移仍未知。下一步只讀追2A121A／2A121B的按鍵consumer，確認按住期間是否讀取；不反覆換座標／點擊或增加cap。
