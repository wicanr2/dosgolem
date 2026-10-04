# 407：原選單控件建立與真正的 mode0 輸入來源

狀態：**CONFORMED，限定原RE來源**
日期：2026-10-05

接續[405 GAME與選單入口](405-moo2-star-map-game-source.md)及[406正常GAME觀察](406-moo2-star-map-game-input.md)。固定官方1.31 EXE SHA-256 4e11be14217b4aafa1839f333bf5eba037f98b0c44e9e4752c96c464c260419f。IDA9.4 linear EA、runtime＝EA+F0000h及原file offset分列。兩份窄查詢515列／451個EA／121列原bytes與重定位bytes不同；2object／365page／51363fixup records獨立核對。只看玩家選單的控件建立、分派及正常輸入，平台／renderer helper內部不追。

## 7D061是控件建立邊界

已證實：sub_7D061有536指令，本輪只取18個控件與條件窗口、31個直接call邊界及共享尾段；沒有完整追其helper。7D120 CALL1151B0後由7D12C把DX寫入dword_194038指向物件的+28h；7D1AF CALL1151B0、7D1BB寫+2Ah。其他控件保留原offset與call，不猜尚未實測的ID。7D88C leave與7D88D..7D890 pop之後，7D891原C3返回。原8028F CALL7D061的trueSS return是runtime170294。

405與406原稿把7D061稱為「正常輸入入口」，證據不足，現改稱選單控件建立入口。406私有事件名menu-input-call／menu-input-entry及key menu_input_reached只作位址導覽，實際僅證明8028F→7D061；不代表原1171AB reader已到。這項分類修正不改執行中probe或CPU／DOS，也不重跑原guest。

## mode0的真正常規輸入

完整sub_7DD41 7DD41..7E00F的174指令保持原名、bytes與xref。原802AE CALL7DD41是case0；7DD77 CALL1171AB、7DD7C保存原回傳選取值。下一觀察應在runtime16DD77記錄CALL，唯一下一Step到2071AB、真SS return16DD7C、ESP−4及同CS／SS，才稱第一個原選單正常輸入。這個實際邊界、畫面與控件表仍待新有界觀察；不能以406的7D061 entry代替。

7DD89用選取值比較物件+32h；7DEC8比較+30h，7DED5將191830設1。7DEE3比較+28h，7DF04將191830設2；7DF12比較+2Ah，且7DF18的1919E4＝0時，7DF29將191830設3。正常讀檔／存檔的branch意義由[395](395-moo2-player-return-options-source.md)與405的mode2／3原case CALL錨定；正式輸入、完整存讀與持久內容尚未驗。191830在星圖首輸入原值34、GAME分支寫0，與選單內2／3不同時點；保留raw值，不把跨畫面共用word當成永久型別。

## 下一個最小觀察

[409](409-moo2-game-outer-frame-continue.md)最初經READY授權：凍結406已有七phase與正常GAME press／release，按[408](408-moo2-star-map-frame-source.md)的outer slot分開作用域，再只讀續行到原mode8 writer、控件建立與真RET、case0的7DD41入口與7DD77→1171AB第一正常輸入。沒有新點擊，不預設新控件ID或把原表形狀等同星圖。到達後另存原PNG／完整表與實際綁定物件；存讀按鈕再各自建立契約。主庫RE-first與玩法保持。

本機忽略入口workplace/new-game-407-ida-run.sh、new-game-407-mode0-ida-run.sh、new-game-407-byte-verify.py、new-game-407-source-verify.py；原JSON／byte index／LOG及私有腳本不公開。沿Go1.24.13／Python3.11及IDA9.4 locked-v1，UID/GID1000、network none、patch唯讀。IDA120s／2GiB／1CPU／128pids、bytes90s／2GiB／1CPU。兩殼層exit0／idat_exit1、非空JSON／5365函式與固定EXE雜湊通過。407沒有新guest、裝置輸入或Go修改。

首次source verifier把7D1BB的DX誤寫為BX bytes，原IDA rows及原file bytes早已一致；保存failed1-407兩份產物與manifest，依原6689502A修正驗證器後重核同一批收據。沒有重跑IDA或guest。

409動態回填：完整406七phase與先前正常玩家前置保持，227146859原first-input返回EAX6、原mode8與1004BC outer真RET、8012F入口已驗。230M尚未到控件建立或真正正常reader，409仍DRAFT；見[409](409-moo2-game-outer-frame-continue.md)。末態最小來源由[410](410-moo2-menu-frontier-source.md)保存，下一私有只讀續行依[411](411-moo2-game-frontier-continue.md) READY，不新增裝置輸入。原版正常存讀與remake同狀態仍未驗。

### 411原正常GAME輸入補驗

[411](411-moo2-game-frontier-continue.md)保持409完整230M末態及15phase後，自然驗出helper trueSS target16EE5E與ESP＋4；原8028F控件建立、7D891真RET、802AE case0及7DD77 CALL均由唯一下一Step驗證。236253170到2071AB、trueSS return16DD7C，實際11表及原GAME面板可見。本篇舊收據與證據作用域保持；409舊完整契約仍DRAFT，正常存讀與remake同狀態仍未知。SAVE GAME的當次binding及啟用條件見[412](412-moo2-menu-save-normal-input.md)。
