# 381：200M 末態的堆疊框架與直接呼叫來源

狀態：**CONFORMED，限定同200M只讀框架與直接呼叫定位**
日期：2026-10-04

## 範圍與來源

[380](380-moo2-colonies-row-click.md)已驗正常 Sol II 行選取13，200M 仍為黑圖。本項只核對該固定末態的返回槽與上層呼叫來源，不改原輸入、200M 預算、CPU、DOS 或主庫玩法，不深挖繪圖及 DAC／PIT helper。

原版為官方1.31 ORION2.EXE，SHA-256 `4e11be14217b4aafa1839f333bf5eba037f98b0c44e9e4752c96c464c260419f`。工具基線 `5d3f5b80373c4872e1401366d3b5dd482577252a`。原380完整收據 `new-game-380-restore-journal.json` SHA-256 `009108c483e0c490a322d213f56938ab319547f17805c299af3047c47b5b4cab`。固定日期不是亂數 seed。

IDA Pro9.4使用既有 locked-v1 image `6f6d59af49d0008c4109a5295b5f374bdc007e2d1ab28cb9de08779584de2780`。原ZIP唯讀、一次性DB在/tmp、network none、120s、2GiB、2CPU、128pids、UID/GID1000。IDA linear EA與dosgolem_high_le分列；本固定LE指令映射加F0000h，以原bytes及file offset驗證，不能移用至其他版本。

## 已證實的靜態來源

- IDA linear EA `sub_1338C9`，1338C9..133BAE，214項；原末態dosgolem_high_le:223A23對應133A23，bytes0345A8、file offset1806455。
- 序言1338D3..1338D9依序push ECX／ESI／EDI／EBP、mov EBP,ESP、sub ESP,260h。原末態EBP2BDB68−ESP2BD908恰為260h。因此ESP首值21A6F3在區域空間，不是本函式的返回槽。
- 尾端133BA7..133BAD為mov ESP,EBP，再pop EBP／EDI／ESI／ECX及RET。靜態返回槽為SS:EBP+10h，即原末態2BDB78；其中原值尚未讀取，不宣稱自然RET已發生。
- 21A6F3確實映射到IDA 12A6F3的指令邊界，但前一call在12A6EE指向sub_14852C，非sub_1338C9。不能因值像程式位址就認成目前caller。
- 七個直接CALL均保留原bytes、xref及file offset。IDA call EA為A50E1、12EAE7、133417、133494、1336AD、1336F0、133DBB；其原返回定位加5後再加F0000h。1336AD／1336F0沒有IDA函式邊界，保留unknown，不補造函式。

只用序言、尾端、末態與直接call邊界定位，不為helper添加推測名稱。上層玩家語意及殖民地畫面仍未知。

## 有界觀察契約

沿380 private probe重播相同一次正常流程到原200M，所有裝置輸入與cap不變。只在rowClick且terminal快照讀取32bytes原SS:EBP；不得重新挑時點、提前跳出、增加cap或寫guest RAM／核心。原完整200M核心、FPU、clock與ESP／EBP來源不符即拒絕。

讀取置於既有activationPeek與前後完整DAC／核心／RAM只讀保護內。新增frame_selector／frame_offset／frame_readable／frame及return_slot_offset，只記錄原值。原200M終點、原state及PNG與380完整收據分別比對；只忽略已有證據的每輪RAMhash，不略過核心、裝置、表與像素差異。

private patches逆轉後逐byte等於380，公開internal及原probe保持基線。原34CLI保持；mode off不取新框架。所有private原資料及收據留在忽略workplace，公開只保存本證據契約、索引與380回填。若原返回槽未命中七個直接call，只記未知並回到RE，不擴充候選求過。

## READY 與驗收

獨立核對固定原EXE、IDA schema／5365函式、原code file offsets、序言／尾端、七個E8相對目標與380完整末態後才READY及修改private probe。原guest只跑一次；本輪同200M重播是為取得缺少的32bytes，不能當新的畫面驗收。

實際命令入口為workplace/new-game-381-ida-run.sh、new-game-381-ida-boundaries-run.sh、new-game-381-run.sh與new-game-381-verify.py。Go1.24.13沿既有image；native限制600s／2GiB／2CPU／128pids、來源唯讀、PID監測550s與trap。驗證使用90s／1536MiB／1CPU／128pids。原例外、收據雜湊、清理及未知邊界連主庫既有docs/re/dosgolem-moo2-intake-20260930.md。

主庫RE-first保持；人口調整、正式存讀、完整開局、RNG及remake同狀態仍未驗。

READY審查：固定原EXE file offsets、原4push／4pop RET、七個直接E8及380同200M完整來源已獨立通過；才修改private觀察器。

## CONFORMED 限定結果

狀態：**CONFORMED，限定同200M只讀框架與直接呼叫定位**。

同一正常輸入重播至原step_limit200000000，EIP223A23。新frame以外的完整final、35876 DAC事件及13032原紀錄均與380保持；所有既有PNG逐byte保持，原418來源、state及同guest副本保持。讀取32bytes置於既有前後核心／FPU／DAC／RAM只讀保護內，沒有結果注入或cap變更。

**已證實，原框架值**：SS188:EBP2BDB68的32bytes為
`9CDB2B0000002B00576AE730000000001C342200750000000100000000000000`。
原返回槽SS188:2BDB78讀得22341C，原保存EBP為2BDB9C。ESP2BD908首值21A6F3是區域空間中的值，不是本函式返回槽。

**已證實，靜態直接CALL**：IDA linear EA133417／file offset1804907／E8AD040000指向sub_1338C9；下一定位13341C對應dosgolem_high_le22341C。該CALL屬sub_133237，133237..1334BB、184項。返回槽與此CALL吻合；目前呼叫鏈為強推論，未觀察本次自然RET，不稱轉頁已完成。caller133425對byte_1B2358的store也僅為靜態下游，不預填其原執行結果或typed欄位。

兩個private patches逆轉精確回380，公開internal／CPU／DOS／原probe不改。原34CLI逐byte保持。第一輪私有建置因guard使用了不可見的ports變數而中止，原guest尚未啟動；改用既有只讀base的virtual_micros後，同命令乾淨重跑，原guest只有一次。兩次IDA查詢的引用上限與缺少函式邊界例外保留first-query，修成明示bounded輸出及unknown；不改原EXE或正式.i64。

| 不可變鍵 | 新結論與等級 | 舊規格 | 回填 |
|---|---|---|---|
| 官方1.31／200M／dosgolem_high_le223A23／SS188:2BD908 | ESP首值不是本函式返回槽；原slot2BDB78為22341C已證實，當前caller鏈強推論 | 380 | 追加381原框架回填，保留原末態 |
| 同EXE／其它步數的SS188:2BDB78 | 相同stack offset在不同frame生命週期使用，不能套用200M解釋 | 322、323、348、349、352 | 原證據不受影響，未重寫 |

下一步只查sub_133237直接上層的CALL／返回邊界與正常畫面建立入口，使用已讀原框架定位；來源充分後才決定一次有界畫面完成觀察。不延伸palette計算、renderer或DAC／PIT helper，不盲增cap。

200M仍是黑圖／count1空表，殖民地正常畫面未驗。主庫RE-first保持；人口調整、正式存讀、完整開局、RNG及remake同狀態仍未驗。
