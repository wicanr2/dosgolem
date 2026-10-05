# 428：原保存後末態的圖像支援候選邊界

狀態：**CONFORMED，限定靜態邊界與原bytes；函式語意為強推論**
日期：2026-10-05

接續[427未達父層返回的結果](427-moo2-save-parent-return-observation.md)。兩次原250M上限未到parent返回，重新核對compiler／runtime分流與停止線後，只查實際末態函式邊界、入口及直接caller。沒有新guest、CPU修正或步數調整。

## 輸入與驗證

官方1.31 ORION2.EXE SHA-256 4e11be14217b4aafa1839f333bf5eba037f98b0c44e9e4752c96c464c260419f。IDA Pro9.4／locked-v1；IDA linear EA與dosgolem_high_le分開，runtime=IDA+F0000h，原MZ／LE檔案偏移另存。原始名稱與定位保持，沒有工具內改名。

單次一次性IDA查詢只匯出129列／111個EA，原MZ／LE、51363fixup records獨立核對，指令fixup差異0。IDA最小JSON確認5365函式、輸入SHA與UID1000；wrapper退出0、idat退出1，以實際非空JSON及原bytes驗證為準。首輪獨立verifier因漏掛/patch缺檔，在原LE讀取前失敗；同腳本改用既有正確掛載重跑退出0，屬驗證環境問題。

## 已證實邊界

| 實際末態 | IDA函式邊界 | 靜態直接caller |
|---|---|---|
| 427 runtime238B82 | sub_1489F4，1489F4..148C33 | 12A34B、12A7E7 |
| 424 runtime238611 | sub_148605，148605..14861D | 1487C0、148BE3、148BF9 |

sub_148605由原LODSB讀來源，非零byte才寫目的；來源為0仍推進目的與剩餘數。1487D0的caller將目的每列加280h，即640bytes。兩個函式的IDA library flag皆未設，不能把它們批次稱為C runtime。

**強推論**：這是帶透明byte的圖像／掃描線支援鏈。**未知**：此次末態的實際呼叫鏈、parent局部退出旗標、保存後第一個玩家consumer，以及parent未返回的原因。靜態caller清單不能充當此次runtime caller。沒有CPU拒絕事件，也沒有證據支持將末態位置改寫成CPU缺陷。

## 停止線與下一步

保留上述最小支援邊界後停止，不逐行翻譯圖像helper。下一只核對既有425的parent局部退出旗標與保存callee後的玩家consumer；如需新只讀觀察，先由這些來源審查READY，保持原前置與250M上限。正常輸入、GUI可操作、存讀往返及remake同狀態仍未驗；主庫RE-first保持。

本機入口new-game-428-source-result.json與source-verify.py／tests-with-patch.txt、source-plan.json、moo2-428-ida-terminal-boundary.py／JSON／log／stdout及ida-run.sh／output。427原1080份產物已由failed4-427-manifest.json凍結，24份v2啟動SHA保持。公開僅提交自撰文件，000-index與427同輪掛回。
