# 391：選取後 kind7 職業列與按下／放開來源

狀態：**CONFORMED，限定 RE 來源**
日期：2026-10-04

## 範圍與固定來源

接續[390原選取寫入](390-moo2-colonies-pop-writes.md)。本輪核對選取後的原控件型別與按下／放開分派，阻止沿用kind6的錯誤放開條件。沒有新的guest、裝置輸入、Go實作或主庫玩法變更，正式跨列放置仍未驗。

官方1.31 ORION2.EXE SHA-256為4e11be14217b4aafa1839f333bf5eba037f98b0c44e9e4752c96c464c260419f。工具基線574f8e60998bb74c1a5add54e0cd5c362386c9ff。390完整pop-terminal為8510fe25f71add56aee28ea6621f3475d40fb68470c343a1ad9d140e4e49f996，pop-writes為fe71dc8a45063a6dd28565b40a25705796ca049e2fbec5f825922482e80431f3，restore-journal為6e26225a7ae3ca08fc8796a71bc89cf56587532dc5565407e32a23a005b626b3。

IDA Pro9.4 linear EA與原file offset／LE fixups分開保留。dosgolem_high_le程式及資料投影加F0000h，cached word_1B1222的runtime位移為2A1222。固定日期不是seed。

## 已證實的原控件表

390原205804505首輸入是36列／1980bytes，前3個職業列是kind6。原210M的表在298848，bias0，已成37列／2035bytes，SHA-256 d1c13ea6764aab2848addf42cadb7166030d2a5421fd0f8533b9b33773842524。

| index | kind | signed含端點矩形 | +20h raw pointer |
|---|---|---|---|
| 1 | 7 | 310,60..510,88 | 2879DA |
| 2 | 7 | 310,90..510,118 | 2879DC |
| 3 | 7 | 310,120..510,148 | 2879DE |

390 pop-terminal與scene-source210的整表相同。原first-match模型沿387：裝置660,107經signed SAR1成GUI330,107，當次37表first-hit2，kind7。原390 record current4／pool5B2044／record5B25E8及17A974=4／17AABB=1保持。不能把原先36表kind6或舊矩形用於此階段。

## 已證實的原 kind7 控制流

原sub_11CEF5的11E1EC與11E334只在kind==6呼叫1192D1；kind7不走11E1F1或11E33B的held場景回呼。387原11E503／11E506同樣只在kind6於放開呼叫1192D1，kind7略過11E508，仍到11E50D清共享active。

kind7在11E554讀取控件+8的word，經11E569／11E570及11E57B／11E582落入11E69D。沒有kind1／2的pointer開關寫入，也不進kind4的118FD4呼叫。11E6AB起以原var_30分流：值1在11E6D6選擇正index；值2在11E6E2選擇負index；11E706／11E70C返回。這是靜態分支證據，當次返回值仍須正常輸入實測。

11DB87呼叫原sub_123C1B，11DB8C保存結果至var_30。該getter在123C33以MOV AX讀word_1B1222，123C47近返回。這是原cached事件值，與裝置當前buttons分開。390末態cached word為1，裝置buttons為0；不能只讀cached值就宣稱新press已被消費。

原sub_11B05A對kind7在11B0BE／11B0C3進11B0DC，跳11C2E8返回；不走kind6的1156E2繪圖分支。本輪只保存型別分流，不研究renderer或平台helper內部。

## calls 欄位與 DRAFT 勘誤

初稿沿用36表／kind6及等待held場景返回的放開條件。READY審查被當次37表拒絕，尚未生成私有觀察器或送新輸入。初稿與拒絕輸出保存在忽略的new-game-391-first-draft.txt、new-game-391-ready-review.first.py及new-game-391-ready-review-tests.first.txt，不能作目前契約。

既有MouseReadSnapshot388的calls原樣取s.calls。le_startup.go的FD2StartupDOS.Calls及switch s.calls／s.calls++證實它是啟動服務計數，不是AX3查詢次數。本輪附加語意，原388／390收據與公開API不改，不以calls增加作為滑鼠消費閘門。

## 驗證、推論與下一步

兩份窄IDA9.4匯出共238列／238個EA／27筆原file bytes與IDA重定位差異；原MZ／LE、2object／365page及51363fixup records獨立核對。kind6／kind7靜態分派、原36／37表形狀、kind7第一命中、cached getter及服務計數來源分別驗證，不宣稱正常放置完成。

強推論：kind7正常點選應以新press的實際原按鍵getter返回、安全裝置回呼與至少20ms條件安排release，不能等待kind6 held場景。下一步392先保留完整390至210M，再有界捕捉第一個原1B0845；當次37表／kind7／first-hit2、17AABB=1／17A974=4及安全裝置全匹配才送660,107。新press後觀察runtime213C1B依真SS返回20DB8C，低AX1與當次新座標、callback完成均符合後，才安全放開。原型別與cached值來源已證實，當次輸入／BF6ED→B9E94／record及原畫面仍須驗。

重生入口為workplace/new-game-391-ida-run.sh、new-game-391-button-ida-run.sh、new-game-391-byte-verify.py及new-game-391-source-verify.py。沿Go1.24.13及IDA9.4 locked-v1 Docker、UID/GID1000、network none、patch唯讀；IDA120s／2GiB／2CPU／128pids，來源核對90s／2GiB／1CPU。原EXE／JSON／PNG／LOG／RAM／state／IDA與private Go留本機忽略，公開只提交自撰證據。

主庫RE-first保持；正式職務、放置、跨殖民地、存讀、完整開局、RNG與remake同狀態未知。本輪完成聲明限於此來源，沒有新的玩法實作。

READY來源審查：固定390原36／37表、9個kind7含端點命中與y92反例、型別分派與getter bytes、calls實作及公開CPU保持通過；沒有新guest、輸入或實作。

## CONFORMED 限定來源結果

兩份IDA／238列原bytes、390整表與cached／device反例、九個熱區含端點正對照及y92型別轉換反例通過。READY先於來源結果收斂；無新guest、輸入、CLI旗標或Go實作。正式放置未驗。

## 392／393 正常 kind7 放開回填

不可變鍵：DOS／官方1.31 ORION2.EXE／IDA linear EA BF6ED、B9E94、1237D9及1237DB；EXE SHA-256 4e11be14217b4aafa1839f333bf5eba037f98b0c44e9e4752c96c464c260419f。runtime投影加F0000h。原kind7按下／selector由[392](392-moo2-colonies-pop-place-input.md)補驗；當次getter至20DB8C候選未觀測，不能作消費閘門。

[393](393-moo2-colonies-pop-release.md)以selector真SS返回EAX2、新GUI及callback20安全release，原BF6ED→B9E94與BF6F2真返回、下一正常1B0845及50個raw writer已驗。原MOV SS／MOV ESP中間Step的堆疊不可讀屬被動unknown，真near仍要求至少4-byte合法槽。較早kind6來源及391靜態getter仍有效，不能外推該getter候選到此kind7正常路徑。正式job欄位／跨殖民地／存讀及remake同狀態仍未知。
