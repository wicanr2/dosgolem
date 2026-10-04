# 400：原RETURN分派20的父入口與輸入邊界

狀態：**CONFORMED，限定原RE來源，實際父入口未直接取樣**
日期：2026-10-04

接續[399原分派碼](399-moo2-return-mode-trace.md)。官方1.31 ORION2.EXE SHA-256 4e11be14217b4aafa1839f333bf5eba037f98b0c44e9e4752c96c464c260419f；IDA9.4 linear EA、runtime+F0000h及原file offset分開保存。兩次窄查詢404列／229EA／113fixup差異，原2object／365page／51363fixup records獨立核對通過。殼層exit0、idat_exit1、非空JSON／5365函式／固定SHA／UID1000均保存。本來源輪無新guest、Go修改或裝置輸入。

## 已證實的原owner與輸入路徑

原sub_C4562為C4562..C47C1共135指令。C4582讀word_191A08，C458F與word_191A10比較；條件成立才於C45B1保存191A10到word_1985AC。C45B7／C45BC及C45C6..C45D0建立場景的呼叫邊界。C46D6取原sub_C3D34指標，C46DB呼叫1191CA，C46E0直接呼叫C3D34。本篇不猜控件名稱或深入renderer。

C4725呼叫12C2A0，C472A呼叫原1171AB；C4732保存EAX，C4735呼叫114177，C4740把局部選取值交C4343。原C478B與word_17B0EE比較，C4792可回C4725；C4747及C4792分支、原清理與C47C0 RET均保留。這是原靜態輸入／返回邊界，不代表399已到達該輸入。

原C4343的C4405..C4434含mode寫入邊界：按原條件把191A08設19h，或從word_1985AC回寫191A08／191A10。不直接派送選取值，也不據此猜RETURN／options熱區。

## 原末態篩選邊界與未知

B4EF6的原C541C caller位於sub_C53C9 C53C9..C5426共35指令；其direct caller為C54D6。此處只保留caller定位及呼叫前原參數，不深入篩選helper。C3947／C3996及C4343的direct callers保留原EA索引；沒有來源證明它們直接呼叫C53C9，不補虛構call chain。

399原分派點1006A7的EAX20與398完整44項表支持父C4562的強推論；399沒有直接取樣C4562入口，原215M仍為B5051。真SS返回鏈、實際父入口、下一正常輸入、畫面切換與控件表變化仍未知。正常存讀與remake同狀態未驗。

## 重生與下一閘門

本機忽略入口workplace/new-game-400-ida-run.sh、new-game-400-flow-ida-run.sh、new-game-400-byte-verify.py、new-game-400-source-verify.py。Go1.24.13與IDA9.4 locked-v1既有image，UID/GID1000、network none、patch唯讀；IDA120s／2GiB／2CPU／128pids，bytes90s／2GiB／1CPU。原JSON／EXE／LOG及private scripts不公開。

下一401先建立只讀READY契約，保持完整399／397與原215M；只捕捉原1050C／C4562實際入口、B4EF6的真SS caller及C472A／1171AB邊界。不得新增press／release、延長cap或代寫guest RAM。主庫RE-first保持。

解決回鏈：[398](398-moo2-return-mode-source.md)來源raw20→1050C CALL C4562；[399](399-moo2-return-mode-trace.md)只讀原分派碼與原末態；[397](397-moo2-colony-return-deferred-release.md)自然RETURN收據不變。
