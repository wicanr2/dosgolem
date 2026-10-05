# 421：原存檔交易的檔案與資料來源

目前證據：424已驗原SAVE1.GAM的53次寫入共208000bytes、close0與callee真返回；正常玩家返回及讀回未驗，詳見[424原保存續行結果](424-moo2-save-create-continue.md)。本文件的來源或原失敗驗收範圍保持；424整體回DRAFT，截圖留存、返回路徑與容器逾時待修。

狀態：**CONFORMED，限定原版檔案／資料來源核對**
日期：2026-10-05

接續[420 正常選格與SAVE入口](420-moo2-save-release-guard-correction.md)。原正常玩家輸入已到dosgolem_high_le 10160Bh、EAX0、真SS返回槽16E3F9h；本項只查原sub_1160B的玩家交易。沒有新guest，不修改主庫玩法或公開CPU／DOS／probe。

## 固定輸入與位址

官方MOO2 1.31的ORION2.EXE，SHA-256 4e11be14217b4aafa1839f333bf5eba037f98b0c44e9e4752c96c464c260419f。IDA Pro 9.4，image 6f6d59af49d0008c4109a5295b5f374bdc007e2d1ab28cb9de08779584de2780；Go1.24.13／Python3.11作獨立檔案與LE重定位核對。

三套位址分開記錄：

- IDA linear EA：sub_1160B，1160Bh至11BE4h，380個指令項。
- dosgolem_high_le：同一code object的IDA EA加F0000h；原入口10160Bh。
- file offset：由原MZ26654h、LE292E4h、2object／365page映射重建。51363筆原fixup逐record核對，IDA重定位bytes與原file bytes分欄。

私有證據在既有workplace/：moo2-421-ida-save-callee.json、moo2-421-ida-save-callee-tail.json、moo2-421-source-byte-index.json與new-game-421-source-result.json。原碼、bytes、原PNG與JSON本機忽略。

## 已證實的遊戲呼叫端

直接caller為IDA12759h、13866h、7DE7Eh、7E3F4h、80773h、827CAh。只有7E3F4h有本輪420正常輸入收據，不將其他caller的模式推定相同。

| 原始IDA定位 | 保留的操作與來源 | 等級 |
|---|---|---|
| 11610h／11614h／11615h | ENTER498h、保存EAX、EBP減82h | 已證實bytes |
| 1163Dh／1164Fh／11650h | 取原saved signed word，增加1，拼接SAVE | 已證實來源 |
| 11673h／1167Bh／11696h | 拼接.GAM；原EDX配置wb；EAX配置EBP+4Ah | 已證實來源 |
| 116A0h | CALL fopen_，原target IDA12685Dh；runtime1016A0h | 已證實CALL；尚未實測參數與結果 |
| 116A5h至116ABh | EAX保存到EDI／ESI、TEST0，非零走116E9h | 已證實分支 |
| 116ADh至116E4h | 建立Error saving game.訊息，經原caller回共用尾端 | 已證實來源；錯誤GUI未實測 |
| 116E9h／116F0h／116F5h／11704h | local+72h配置E0000000，EBX1／EDX4，再到第一fwrite CALL | 配置與CALL已證實；中間sub_8F855的保留行為待動態核對 |
| 1170Eh／11713h／11727h／1172Ch／11731h | 第二fwrite配置37bytes；原byte_1916BE加37乘slot，slot>=9使用9 | 已證實來源；本輪slot0的實際writer未執行 |
| 11BCBh | CALL fclose_，原target IDA12697Ah | 已證實CALL；close成功未知 |
| 11BDFh→115FEh | 共用尾端恢復ESP／EBP／暫存器 | IDA來源已證實；原返回尚未觀測 |

原SAVE、.GAM、wb與錯誤字串同時核對IDA引用bytes、獨立LE映射與原EXE。IDA170664h／170669h／17084Ch／17084Fh分別投影runtime260664h／260669h／26084Ch／26084Fh。對目前slot0，來源導出SAVE1.GAM；實際fopen buffer仍待觀察。

共有53個fwrite CALL位址。這證明序列化呼叫端存在，不能直接證明53次成功寫入。原buffer名稱與運算元保留，不將未知欄位猜成玩家語意。檔案buffering、短寫、flush與作業系統服務內部不屬本項RE。

共用尾端被IDA歸在sub_10E2F。只取115FEh附近13列，不分析該函式其他玩法。IDA最後匯出列11609h為POP EBX；下一個file byte在offset617566為C3，作為原近RET的直接x86位元組證據。這筆不冒充IDA匯出列。

## 驗證與差異

兩次窄IDA查詢共526列／447個EA／64處fixup差異；原LE與file bytes獨立核對。新輸入釘選與420完整末態保持。首次腳本換行字串SyntaxError沒有新JSON或guest，四份失敗按failed1-421前綴與manifest保存；修正語法後同image／UID／命令重跑。兩次有效查詢wrapper0，idat實際exit1、非空JSON／5365函式與原SHA核對，退出碼照實保存。

本項CONFORMED只能表示來源核對通過。SAVE1.GAM是否真正建立、寫入範圍與完整檔案、fclose／返回、成功GUI、正常讀檔、鍵盤命名與remake同狀態仍未驗。主庫RE-first保持。[422 有界存檔續行](422-moo2-save-callee-continue.md)只準備私有原版觀察，不授權猜補平台行為。

422當次動態結論：422已驗SAVE1.GAM／wb實際buffer與AH3C平台拒絕，正式存檔未完成。見[422原實際請求](422-moo2-save-callee-continue.md)；原fopen未返回、53個fwrite來源尚未成為實際寫入，後續平台補缺依[423](423-moo2-protected-create-file.md)。
