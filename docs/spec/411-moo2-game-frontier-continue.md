# 411：完整409末態後的 GAME 原版只讀續行

狀態：**READY，限定完整409末態後的只讀續行**
日期：2026-10-05

來源是[410最小末態邊界](410-moo2-menu-frontier-source.md)、[409](409-moo2-game-outer-frame-continue.md)與[407正常reader](407-moo2-menu-control-input-source.md)。這是私有原版觀察契約，主庫RE-first與玩法保持。

## 固定前置與新作用域

固定409 Go SHA-256 0f06450a25d71fdeebdccb9ba0b13695d3d9ff21c93d4c036d06d785daa9fa24；完整15phase SHA-256 a12e2882d1b2a429bf2f75ee5c9da7269b6cb6fdaa86d264335d5f0b7dba3e74；完整230M terminal SHA-256 84af7158ef52528a32c1279e5287bbe3197864540121ab8d643e1cde82479f19。原七phase及404／402／401／399／397完整凍結保持。

新旗標GAME_MENU_FRONTIER_CONTINUE只接受1，要求完整GAME_MENU_OUTER_CONTINUE、GAME_MENU_PRESS與所有舊依賴、MAX_STEPS185M與state；讀EXE前拒絕。關閉精確等同固定409，仍在230M停止且正常reader未驗，不稱409完整契約已完成。

在230000000的原末態、執行任何新CPU.Step前，重生完整409 terminal與15phase並逐欄比對，只略既定三個RAM雜湊鍵。通過後才開新有界觀察到240M。409原230M收據保持，不覆寫或把它寫成已到選單；新上限是續行的工作上限，不能推論240M必到reader，不通過時不延長。

## 當次最小返回契約

原230M EIP21F7C1、EBP2BD934／ESP2BD888、CS8／SS188h必須匹配。依410原prologue與epilog算當次RET slot＝EBP＋20＝2BD948，只讀該dword及指向原Code16，保存實際值；要求可讀且在原LE code範圍，不預填16EE5E或其他caller。觀察不改core／device／RAM與virtual time。

原12F7E5 RET只在同CS／SS及ESP＝當次slot時辨識。保存真SS／SP／actual target，唯一下一Step同CS／SS與ESP＋4才稱自然返回。其他frame或共用helper的命中只作原定位，不猜語意。保留原getter與唯一CPU.Step，不補CPU特例，不深入byte-copy、圖像格式、driver或runtime內部。

之後沿409已知契約觀察原8028F→7D061、7D891真RET→170294、802AE→7DD41及7DD77→1171AB。CALL與RET皆以實際SP、同CS／SS與唯一下一Step驗證。原7DD77 CALL的下一Step到2071AB、ESP−4與真SS return16DD7C通過後立即停止。原mode、實際控件表、物件綁定及PNG另存，不假定新表仍有23項。

新InjectMouseEvent呼叫數0，不改原RAM、core、mode、派送ID或正式RNG。新phase最多40，舊15phase另存。若240M未到、CPU拒絕或frame握手不符，保存真正末態與原bytes並保留未知；不把失敗寫成產品缺陷。

## 獨立驗證與範圍

235CLI保留完整225前綴，再加8個拒絕與2個正對照。精確反轉patches至固定409，唯一Step與原getter保持、零新裝置呼叫；獨立核對原LE bytes／fixups、全部新phase唯讀、完整舊末態與所有前置、trueSS及SP握手。原418檔、SAVE10／MOX保持。

原PNG人工檢視與數值驗證分開；正常選單reader、正式存讀與remake同狀態分開，不把固定日期當seed。完整409末態凍結與下一段原自然返回只支持本次原版觀察，不替代正常保存／讀取或remake驗收。

本機忽略入口workplace/new-game-411-generator.py、new-game-411-run.sh、new-game-411-source-verify.py、new-game-411-verify.py。原EXE／Go／PNG／JSON／LOG不公開。沿既有Go1.24.13、UID/GID1000、network none、原ZIP／patch唯讀；guest900s／state850s／3GiB／2CPU／128pids、GOMEMLIMIT1GiB、cgroup memory.events、owned PID trap及Docker清理保持。尚無411 Go或guest。

## 來源與只讀審查

409固定Go、完整15phase與230M末態雜湊、410原prologue／RET框架、407真正reader與原225CLI均已核對。new-game-411-ready-review.json保存審查；actual helper target仍未知，不預填caller。READY只授權此私有觀察器；尚無411 Go或guest。
