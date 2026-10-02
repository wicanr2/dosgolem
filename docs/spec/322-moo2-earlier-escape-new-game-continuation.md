# 322：44M Esc初態的正常NEW GAME續行

狀態：**CONFORMED**
日期：2026-10-03
範圍：單次原版正常輸入實驗的獨立初態，不改CPU／平台／玩法。

## 證據與契約

固定1.31 EXE SHA-256 4e11be14217b4aafa1839f333bf5eba037f98b0c44e9e4752c96c464c260419f。[321-moo2-menu-mouse-event-consumer.md](321-moo2-menu-mouse-event-consumer.md)46M Esc／50M下，原版在放開後正常取用按下事件與500／229座標並清除待處理欄位；兩CB、七真正讀取與續行已證，高層設定轉移未知。[317-moo2-menu-display40-observation.md](317-moo2-menu-display40-observation.md)第40換頁49882420，僅餘117580步，不增加cap。

預先固定新44M Esc／1996-01-01／50M cap，其他輸入與單次display40後按下／至少20ms移動放開保持。探針點擊guard接受已有明示44M或46M；48M／其他步數仍拒絕，日期／cap／禁止早期滑鼠混用保持。沒有新環境變數、無宿主代寫、沒有調時計或永久鎖seed。44M與46M不同初態分開，不冒稱same-state；AH2Ch／RNG仍保留既有近似。

驗兩個預定44M正常原版流程，無點擊與點擊。先建probe一次，真實CLI檢查44／46可越過guard讀不存在EXE、其他排程與混合輸入拒絕。兩44M共享同417根檔／MOX.SET／日期與原始EXE，點擊前完整前綴需逐列保持，只許設定來源／mtime／DTA四bytes／PNG路徑差異。第40頁時點與PNG實際核對，44M正常IRQ1及兩CB／真正事件consumer／自然畫面或新拒絕照實記錄。

## READY審查與驗收

307／316已證正常controller排程，321已證此次正常滑鼠事件不會隨放開遺失。較早Esc只提供更多有界續行，不換座標或延長按住。足以READY。CPU／平台來源未改，314全套仍適用；46Mguard依原實際輸入條件保持，不重跑其整段以測單一條件擴充。固定兩新流程，不搜尋成功時點，遇拒絕保留原始bytes／狀態並另開窄規格。公開入口docs/spec/000-index.md，321下一步同次回填；原始素材／PNG／完整LOG留忽略workplace，完整remake與正常玩家路徑未完成。

## 正式收據與限定結論

固定Go1.24.13映像、600s／2GiB／2CPU／128pids／UID1000／network none，417根檔乾淨重建，先go build -p 2 -buildvcs=false -o /tmp/moo2-probe ./workplace/moo2-probe。八真實CLI檢查通過：44／46越過guard讀不存在EXE，其餘45／48、錯日期／cap及兩早期事件拒絕。私有workplace/moo2-322-click-config-tests.txt SHA-256 2ba1a70a1be8ad69187a4dce32429233c9fd3ffab24b52cd80880e858380f0c0。來源d4c0e20b8dca806d53d066d11954612fd0d85dda85e66d915e47f1caebda578a。

兩44M原版正常controller01／81，44000000步高位LE257B81，真正8:21C4D8各97／77步CF返回，原caller自然到257B84／257B8B。兩第40換頁47850591／59121761µs，PNG dc938ac71e2a617ec9a6c2d75029f689de6aaf985530a030713195e0c566b15d與46M同圖，不是同時計／核心。

點擊前兩44M完整前綴逐列保持，只正規化PNG路徑。47850592按下、47851578放開，相隔32370µs，原版兩CB於47850693／47851666返回、191步，寫後word0100／0000及事件window保留，十筆正常事件讀取。無點擊到50M高位LE238573、R35F8C4／2／A3／0／2BDB10／2BDB68／4FD29A／35FAAC、六段8／188／188／0／20／188、flags206h，無拒絕，終圖11ec0ed15a4c874d36c94a824af73eb71dc6937dfe3bd568450943861db89927。

點擊新拒絕47995790步，高位LE229A59的CD21、下一EIP229A5B；R264E92／0／261692／295828／2BDB60／2BDB78／FFFFFFFF／2BDC2C，六段8／188／188／0／20／188、flags246h、時計59620564。實際AH4Eh／CX0／DS188:261692的ASCIIZ save?.gam，DTA188:295828；之前save10.gam成功，DTA已帶SAVE10.GAM結果。新拒絕是萬用字元未支援，不能泛稱所有INT21或AH4Eh未接線。終圖488479b046d2360a601f35745caf85ab2942503f9cde642140a270fc5d342a19已檢視仍主選單。

私有workplace/moo2-probe-322-baseline.txt.gz SHA-256 417992b4c93c9092cd4366bea6ab43a5b53eeabf898c1d20f07697b4e56b7d6e；click.txt.gz 3285597c857f30c6e542cef70ee0a8568e4725e05f2de0c8d7ed90bcbc9ff7fb。原ZIP417根檔實際只有SAVE10.GAM／208000bytes，沒有捏造空存檔。下一步依公開DOS '?' 8.3契約實際搜尋此唯讀檔案集合，不把不支援當缺檔、不追Watcom檔案helper內部。

限定CONFORMED只包含新guard、預定初態／正常輸入與上述真正新拒絕，尚未見設定畫面，不能宣稱高層NEW GAME已完成。44M／46M收據獨立保留，50M上限及主庫玩法RE閘門保持。

後續問號搜尋由[323-moo2-dos-findfirst-question-pattern.md](323-moo2-dos-findfirst-question-pattern.md)延伸。原始限定收據保留；其他萬用字元、FindNext與完整DTA保留區不新增對齊聲明。
