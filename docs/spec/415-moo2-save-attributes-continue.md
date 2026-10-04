# 415：正常SAVE輸入後的檔案屬性服務續行

狀態：**READY，限定原AH43查詢與存檔入口**
日期：2026-10-05

來源是[412](412-moo2-menu-save-normal-input.md)原成功輸入、[413](413-moo2-save-entry-input-source.md)下一玩家入口及[414](414-moo2-protected-file-attributes.md)限定平台查詢。原1.31 EXE SHA-256 4e11be14217b4aafa1839f333bf5eba037f98b0c44e9e4752c96c464c260419f、原名與IDA EA／runtime＋F0000h／file offset分列保持。這是私有原版觀察，不修改主庫玩法。

## 固定前置與平台變更

固定412 Go SHA-256 6ad43493b3adc0ff46669e1299e755f243f57ef1e8d138cb1137ec4a0f609a67；完整原成功14phase檔new-game-415-source412-prefix.json SHA-256 659439b2d39da39d0b1cc59f2e4219a4781faaa855cf324c505c889b53710af4。原15phase及cpu_stop terminal、389份failed1-412原產物保持，不把已拒絕的INT收據重寫成成功。完整411／409與全部祖先仍須凍結。

新旗標只接受1，要求完整MENU_SAVE_PRESS及所有祖先、state與MAX_STEPS185M，讀EXE前拒絕。關閉時精確反轉至固定412；平台414有開關前均是明示工具版本，不聲稱mode-off可重現舊服務拒絕。新Go禁止新增InjectMouseEvent，沿412同正常SAVE press／selector3／release及原writer，不重擲或更改正式RNG。

runner必須把本輪受版控internal/machine/le_startup.go明示copy進固定archive5a2170cfc18b40e890a21dcdcbb10605f084b57d的建置輸入並核對bytes／hash，否則archive仍缺AH43。不改主機套件、不另建image，不以新檔案提供者或固定CX注入跳過真正路徑查詢。

## 唯讀觀察與停止線

在原412成功14phase全部匹配後才啟用新觀察，凍結整批只略既定RAM hash鍵。原219E75／AX4300呼叫前只讀當次DS:完整EDX，最多260bytes至NUL；保存檔名、真指標、Code16、core／device／RAM與來源。實際檔名未知，不預填SAVE10、LBX或查詢成功。只觀察第一個對應查詢，其他原DOS服務自然運行。

唯一下一CPU.Step到219E77，核對同CS／SS、原SP及實際AX／CX／CF。結果依414真provider查詢比對，錯誤也保存，不能以清CF猜補成成功。成功只支持這個原平台呼叫；後續原case3 CALL802CC→7E154仍要求正常release、原mode3與真SS return1702D1／ESP−4，入口16E154立即停止，不推定存檔頁或檔案保存完成。

新phase最多12，保留412的觀察器、唯一Step與原getter。仍以240M邏輯上限停止，不延長、不改mode或guest RAM。若再次平台拒絕或未到入口，保存當次末態與原bytes；再依具體缺口查來源，不繼續深挖平台helper。

## 資源、驗證與範圍

255CLI保留245前綴，再加8拒絕／2正對照。反轉patch至固定412，公開CPU／probe保持，公開DOS差異只允許414兩檔；獨立核對完整成功14phase與祖先、原LE bytes／fixups、查詢前後非輸出狀態及原provider結果、case3真CALL、來源檔與覆蓋層差異。

412唯一guest雖有完整收據仍outer124，這次只把工具外層wall-clock限改1200s、state capture1150s，加入kill-after15s；虛擬時間、240M、3GiB／2CPU／128pids、GOMEMLIMIT1GiB與所有原輸入保持。使用Go1.24.13既有image、UID/GID1000、network none，原ZIP／patch唯讀；owned PID trap、cgroup及Docker清理保持。不把增加工具等待時間寫成玩法變更或原硬體逐週期一致。

本機忽略入口workplace/new-game-415-source412-prefix.json；後續私有腳本沿new-game-415-generator.py、run.sh、implementation-source-verify.py與verify.py。原EXE／Go／PNG／JSON／LOG不公開。尚無415 Go或guest；原PNG人工與數值驗證分開，正常存讀及remake同狀態未驗。

## 來源與契約審查

原成功14phase、實際DS188h／EDX2BD904／AX4300拒絕、Microsoft契約與414工程驗證已核對。明示新的平台建置輸入，保持邏輯／原輸入與實際provider分支；不重寫412拒絕。原稿hash與new-game-415-ready-review.json保存。READY只授權上述私有觀察器；尚無415 Go或guest。
