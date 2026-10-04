# 398：原RETURN後的分派碼與槽位篩選邊界

狀態：**CONFORMED，限定原RE來源與397既有末態**
日期：2026-10-04

接續[397原正常放開](397-moo2-colony-return-deferred-release.md)。官方1.31 ORION2.EXE SHA-256 4e11be14217b4aafa1839f333bf5eba037f98b0c44e9e4752c96c464c260419f；IDA9.4 linear EA與runtime+F0000h投影、原file offset及LE重定位bytes分列。兩次窄查詢320列／298EA／84fixup差異、原2object／365page／51363fixup records獨立核對通過；殼層0／idat_exit1均保留，非空JSON／5365函式／固定SHA及UID1000通過。本來源輪沒有新guest或Go修改。

## 原分派與恢復來源

已證實：main__0的10365呼叫151指令sub_1049B。1049F清ESI，10687測ESI；ESI=0時以word_191A08索引原跳表。case1在104EA呼叫C058A，104EF設byte_191F19=1，104F6跳10687。case0在104B7呼叫86188，case8在104A6呼叫8012F，case39在104D9呼叫8B956；不得直接派送raw碼。

已證實：C058A正常清理後，C0925讀word_191A08並與word_191A10比較。不同走C0934／C093A，把191A10寫回191A08；相同走C0942／C0948／C094E，從word_1979E8同時寫回兩者。較早C070F..C0724在兩碼不同時保存191A10到1979E8。8B956原入口8B960先將word_191A08清0。本篇只證原讀寫與分派流程，399已補驗當次raw碼1→20與last byte20→1；raw20來源指向C4562，實際父入口仍未直接取樣。

## 原215M末態邊界

已證實：397已從C058A真SS返回runtime1004EF，但215M末態runtime1A5051對應IDA B5051，位於sub_B4EF6的槽位篩選迴圈。B5044取元素+0Ch、B5047取低四位，B504D／B5051取stack局部篩選值，再於B5059比較。不把該處稱為輸入等待，不深挖332指令helper；只保存玩家阻塞點的迴圈邊界及direct caller索引。

397當次控件table298848／count18／stride55，VBE仍StartY0／DisplaySets100，原末態PNG仍殖民地。此事實不能證明再次進入C058A，也不能證明已回COLONIES列表或星圖。1171AB及1192D1原入口／尾端與caller只保留輸入追查邊界，不由未再看到2071AB推定CPU故障。

## 解決回鏈與下一驗證

[397](397-moo2-colony-return-deferred-release.md)原215M結果保持；本篇補明1A5051是篩選邊界，返回目的地待[399原分派狀態只讀追蹤](399-moo2-return-mode-trace.md)。私有重生入口workplace/new-game-398-ida-run.sh、new-game-398-mode-ida-run.sh及new-game-398-byte-verify.py；原JSON／LOG／EXE／Go／PNG只留本機忽略。主庫RE-first／公開CPU與DOS保持，正常存讀與remake同狀態未驗。

## 完整跳表與395範圍勘誤

原103EB..1049B共有44個dword，與1069E的AX≤2Bh相符；398從原EXE及44筆LE fixup逐項重建，舊395前43項完全相同，末項raw43指向106AF。395的43項是擷取前綴，不是完整原表；新本機收據moo2-398-full-dispatch-table.json及new-game-398-table-tests.txt保存file offset／raw bytes／原relocation與投影。

解決回鏈：固定上述EXE SHA＋IDA投影103EB..1049B，語意為44項完整分派表；[395](395-moo2-player-return-options-source.md)必保留「398完整44項」標記，來源395／397已驗分支不受影響。入口workplace/new-game-398-table-verify.py；不重跑IDA、guest或修改原快照。

## 399原分派碼回填

[399](399-moo2-return-mode-trace.md)已驗當次原writer C093A與104EF，raw分派20及完整397保持。來源raw20在1050C呼叫C4562；父入口只作強推論，實際入口與下一正常輸入待驗，窄來源見[400](400-moo2-return-parent-source.md)。B5051只保留篩選邊界，不稱輸入等待。
