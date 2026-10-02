# 321：原版滑鼠事件欄位的主選單消費

狀態：**CONFORMED**
日期：2026-10-03
範圍：同正常輸入的原版事件資料流唯讀觀測。

## 證據

固定1.31 EXE SHA-256 4e11be14217b4aafa1839f333bf5eba037f98b0c44e9e4752c96c464c260419f。[320-moo2-button-read-hook-control.md](320-moo2-button-read-hook-control.md)已證兩正常原版指令讀座標500／229，掛勾真正請求與原始基線保持。已保存callback在213D61／213D71讀瞬時2A121A，213D7C寫2A1222、213DA2／213DAB寫word1到2A1228／2A1220、213DBA／213DC6寫座標到2A121C／2A121E，隨後更新2A11EC。正常讀取端不能只查瞬時2A121A；事件用途仍保留原址與推論等級。

## 型別與契約

維持320完全相同46M Esc／日期／50M／一次按下與放開，不新增輸入。現存8／16位元委派的目標範圍擴至2A121A..2A1229與2A11EC..2A11EF，仍排除活動callback、至多32筆。每筆記真正請求selector／offset／linear／width及實際CPU.Step指令前後完整R／六段／flags／原bytes／error，原始兩個watch window前後快照。各正常CB後另記兩window原始byte，不以宿主填值。

每次目標命中後，最多追接續四個正常外層步的原始指令／核心／window；全程最多32筆consumer續行，避免展開完整helper。IRQ或callback若介入，保留原始狀態，不補用途。既有讀取正對照與統計保持、不改Bus身分或委派返回；CPU／平台與主庫玩法不改。

## READY審查與驗收

所有窄目標來自已保存callback的原始指令，不從猜測文名或假欄位名選取。現存observer鏈已由320真實MOV正對照。足以READY。正常重生同320流程，只允許新目標read／consumer／事件window列；全部320其他列與終圖保持。初次事件寫後／真正讀取／分支與自然畫面照實核對，未見設定畫面不稱完成。CPU／平台來源保持，314全套仍適用。素材／完整LOG／PNG只留忽略workplace，索引docs/spec/000-index.md；320下一步同次回填。

## 正式收據與限定結論

更多有界續行已由[322-moo2-earlier-escape-new-game-continuation.md](322-moo2-earlier-escape-new-game-continuation.md)取得：獨立44M初態的單次點擊觸發save?.gam搜尋缺口，兩CB與事件保持；高層設定畫面仍未知。原46M收據保留如下。

沿320命令／固定Go1.24.13映像、600s／2GiB／2CPU／128pids／UID1000／network none，417根檔乾淨重建；同46M Esc／日期／50M及正常單次滑鼠輸入。來源SHA-256 cc42dde3fb59b788351981c6457e10444b1df6ac6e7e18bf03019f34df3a0ea1；私有workplace/moo2-probe-321-click.txt.gz cf8392ef4e849c3ec268a6742fe46ee6d843850e56298d3afffafeaa7bf7e30e。

兩CB後原始window：
- 49882522：2A121A..2A1229=0100F401E50001000100010000000100，2A11EC..2A11EF=01000000。
- 49883496：0000F401E50001000100010000000100，02000000。

七真正請求皆released=true，回呼已完成後客體正常執行，高位LE：
- 49895675，213C60／66 A1 28 12 2A 00讀2A1228=1到AX；49895677的213C69／66 C7 05 28 12 2A 00 00 00清為0，寫後window核對。
- 49895713，213C33讀2A1222=1。
- 49895750／49895784，213BD9／213C06讀2A121C=500／2A121E=229。
- 49895819／49895889，213EC5／A1 22 12 2A 00取dword10001，再SAR16取1並CMP／JG；SegmentRead8首byte嘗試被記為width1，不表示客體只讀byte。
- 49896663，213A7C讀2A1226=1，下一66 A3寫26C518的原始consumer亦保存。

7讀／7前後window／28續行／2CB window皆通過，完整R／六段／flags、bytes與錯誤保留原始收據。已證正常程式在放開後仍取用按下時座標與事件，未丟棄短按下；欄位用途只限上述讀寫契約，不補整個UI規則。

全部320其他原始列及終圖保持，僅新增事件列及目標計數0→7。normal8=103537／normal16=11288／callback8=59／callback16=15、四正對照與兩掛勾code匹配保持。50M仍2385AF／時計64282188，終圖0c45ba73ddfe850693520f5aee093c1c188ab118df9c51570521cfe3fc0ddad9保持。CPU／平台來源未改，314全套PASS適用。

限定CONFORMED只含真正事件寫入／原版讀取、清除與上述續行。高層NEW GAME選擇及設定畫面仍未知，不以七個helper讀取冒稱完整正常玩家路徑。下一步以獨立明示44M Esc初態取得同50M cap下更多正常續行，46M歷史基準保持；不延長按住、反覆點擊或代寫欄位。
