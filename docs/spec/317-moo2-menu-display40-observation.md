# 317：完整主選單第40換頁的唯讀觀測

狀態：**CONFORMED**
日期：2026-10-03
範圍：原版探針觀測，不改輸入、CPU、平台或主庫玩法。

## 證據與契約

工具基線cbc63f6ad19f17ca8eda81c1ab44f6852fb383e9。固定1.31 ORION2.EXE SHA-256 4e11be14217b4aafa1839f333bf5eba037f98b0c44e9e4752c96c464c260419f。[316-moo2-configured-hardware-escape-schedule.md](316-moo2-configured-hardware-escape-schedule.md)兩46M Esc流程終態DisplaySets40、PNG六按鈕可見，但只有換頁8..23的逐張時點，不能假設第40頁早已停穩或可點。

既有DOSGOLEM_MOO2_VBE_FRAME_PREFIX非空時，成功4F07且沿既有keyboardStep門檻，保留最多16張既有快照，另擷取DisplaySets恰40的一張。沿315原始同一段PNG編碼、R／六段／flags／FPU逐位元／VBE不突變檢查與outer_step／時計／雜湊收據。相同頁不重複；沒有prefix仍不擷取。沒有新環境變數或輸入。

## 審查與驗收

已核對VBEIndexed／VBERGB只返回副本、VBEState回傳值，以及既有315唯讀框架。新增條件只擴觀測的已見頁號，不代表動畫完成，不直接注入點擊。足以READY。

驗收一個真正原版46M Esc／1996-01-01／50M cap正常流程。全部316-full-game既有列，除檔案mtime／DTA四bytes／PNG路徑外逐列保持；只允許新增一個display40只讀收據。PNG需實際檢視，記錄步數與剩餘預算，不推測熱區或跳動畫。CPU／平台來源保持，314全套仍適用。原始素材、PNG、完整終端留忽略workplace。入口docs/spec/000-index.md。

## 正式收據

後續正常滑鼠輸入已由[318-moo2-new-game-normal-click.md](318-moo2-new-game-normal-click.md)限定驗證；兩CB完成，設定畫面仍未知。下述原始唯讀收據保持。

Go1.24.13既有映像、600s／2GiB／2CPU／128pids／UID1000／network none，417根檔與官方EXE從唯讀ZIP乾淨重建。go run -p 2 -buildvcs=false ./workplace/moo2-probe /tmp/game/ORION2.EXE --game-dir /tmp/game，沿316-full-game固定條件，輸出317。

第40換頁outer_step49882420、dosgolem高位LE callsite228CA7、時計63906833µs。VBE Bank4／StartY0／BankSets846／Writes15212532／DisplaySets40；R4F／0／0／0／2BDB40／2BDB68／0／7，六段8／188／188／0／20／188、flags246h，readonly=true。PNG dc938ac71e2a617ec9a6c2d75029f689de6aaf985530a030713195e0c566b15d已實際檢視，六按鈕全見，NEW GAME文字約畫面500／229。此座標只是正常點擊的視覺來源，尚未證熱區或動畫停穩。

剩餘117580外層步；全部316既有列除既定mtime／DTA四bytes／PNG路徑外保持，只增一張第40頁。17張PNG雜湊與唯讀標記通過；終圖87fabf21f7be22d264f83390bdb4d39f09098516191c3a17c51815452856645e與316逐位元保持。來源SHA-256 670be537e43be204c1fe377d7e4d170ac9df29f5d6bb7440703a8f6c5063f831；本機workplace/moo2-probe-317-full-game.txt.gz 35e2604c0d223e7170e6774dee33c67828992269d75ab41bb0c7607c26c1d0b7。限定CONFORMED只含此唯讀時點觀測，下一步由獨立READY輸入規格驗證正常點擊，50M上限保持。
