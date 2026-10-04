# 403：原殖民地列表 RETURN 綁定與狀態恢復來源

狀態：**CONFORMED，限定原RE來源與402既有控件表**
日期：2026-10-04

接續[402正常列表返回](402-moo2-return-parent-continue.md)。官方1.31 ORION2.EXE SHA-256 4e11be14217b4aafa1839f333bf5eba037f98b0c44e9e4752c96c464c260419f；IDA9.4 linear EA、runtime＝EA+F0000h及原file offset分列。兩窄查詢238列／209EA／70fixup差異、原2object／365page／51363fixup records獨立核對；完整sub_C4343 C4343..C4562的121指令保留原名、bytes、xref。

## 原producer與consumer

已證實：sub_C2E4D的C2F10 CALL1151B0，C2F15把回傳AX寫入word_17B0EE；呼叫前C2F0B的EAX＝213h，C2EEE的EDX＝1BDh。C2F25 CALL114C72後，C2F2A把AX寫入word_17B0F0。只保留建立控件的呼叫與回傳邊界，不追美術或平台helper。

C4343接收原選取值指標；C43DD與17B0EE比較，符合則到C43EF，否則C43E6再比較17B0F0。C4405／C440B比較191A08／191A10；兩者相同且17A7BD非零時，C441D把191A08設19h，其餘走C4428，從1985AC讀回原值，C442E／C4434寫回191A08／191A10。C443D符合17B0F0時，C444A／C4450把17B0EE寫回原選取值指標。C4562的C478B比較17B0EE，不符合回C4725輸入迴圈；符合則走原清理與C47C0返回。C45B1是1985AC的保存writer，條件沿[400](400-moo2-return-parent-source.md)保存，不猜當次值。

## 證據邊界與下一輸入

402原20×55byte表的raw3／kind0矩形531,445..615,470及九個first-hit案例已驗；原PNG右下RETURN與該矩形一致。403來源輪以強推論標示raw3為當次RETURN候選，當時未直接讀取17B0EE／17B0F0／1985AC；404已補驗當次3／4／0及真SS selector返回3。source producer本身的證據範圍保持。

[404正常列表RETURN輸入](404-moo2-colonies-list-return-input.md)已凍結完整402末態，當次UI word及表核對通過後正常press／release，並自然回到星圖及下一輸入。word_17B0F0＝4只讀已驗，鍵盤替代入口未實際按下。options／存讀與remake同狀態仍未驗；主庫RE-first及玩法保持。

本機忽略入口workplace/new-game-403-ida-run.sh、new-game-403-producer-ida-run.sh、new-game-403-byte-verify.py、new-game-403-source-verify.py。Go1.24.13／Python3.11／IDA9.4 locked-v1，UID/GID1000、network none、patch唯讀；IDA120s／2GiB／2CPU／128pids，bytes90s／2GiB／1CPU。兩殼層exit0／idat_exit1、非空JSON／5365函式與原hash通過。沒有新guest或裝置輸入；EXE／JSON／LOG與私有工具不公開。

解決回鏈：[404](404-moo2-colonies-list-return-input.md)的當次UI word、原mode恢復writer與自然返回收據；原source rows及byte index不覆寫。
