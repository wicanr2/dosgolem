# 397：原 RETURN 辨識後的正常放開

狀態：**CONFORMED，限定原正常RETURN放開與自然返回**
日期：2026-10-04

接續[396](396-moo2-colony-return-input.md)，只授權私有原版觀察器。官方1.31 EXE SHA-256 4e11be14217b4aafa1839f333bf5eba037f98b0c44e9e4752c96c464c260419f，Go1.24.13；runtime投影與IDA linear EA分列。396 Go SHA-256 ed75f3fb65ff48372ec781ce8149a5b53cdbb577bea6359f24de4df417e6ada5；原return-events SHA-256 e095ced17ff9cc56c7222715ab6addaceb0130172825873d4bee567b77ff2c5e。

## 已證實前置

396完整393／正常1180,468 press／RETURN18 selector真SS返回、EAX18及GUI590,468、新callback21已驗。212597711安全IF／IRQ通過，但只經7357µs，20ms即時放開候選拒絕。末態215M持續held，不證正常RETURN失敗。主庫RE-first及公開CPU／DOS保持，沒有Go玩法實作。

## 私有契約

新COLONY_RETURN_DEFERRED只接受1且要求完整COLONY_RETURN及其所有前置、原MAX_STEPS185M與非空state；缺項／錯cap讀EXE前拒絕。mode off保留396候選。

完整393結果比對先於新press不變。原396前五phase逐欄位匹配，僅略三RAM雜湊鍵，確認完整原按下、selector真返回與7357µs拒絕後才啟用延後放開。不得在原press之前、未讀真SS或只看座標就放開。

每個原Step仍正常執行。在已確認原selector返回後，首次虛擬elapsed至少20ms且IF／IRQ安全、callback21／21無pending／active、原GUI590,468及RETURN18 flags保持、裝置1180,468／buttons1時，保存core／RAM／四範圍／flags並正常InjectMouseEvent release1180,468,0。只送一次release，不猜PC或改RAM，來源prefix通過只代表測試前置，不是正式規則。

其餘觀察沿396：released-zero／activeclear、C08CA／C0960及共用BAD9F真near槽；只有到C0960才接受RET，原caller只接受1004EF／1EE0E3且ESP+4／相同SS。下一2071AB真正常輸入另存frame及caller，不能預設父畫面名稱。保持原215M，未安全就持續held至cap，不補寫GUI cache或變更硬體時序。

新觀察最多24phase，每項readonly／core／frame／device及每PNG核對。固定日期不是seed；SAVE10／MOX及418原輸入保持。公開CPU／DOS、舊getter與唯一Step保持。397 Go必可精確反轉至固定396 Go，CLI154完整prefix及新增拒絕／mode-off測試、來源prefix與設備注入及真near返回獨立核對。

## 工具、證據與界線

workplace/new-game-397-generator.py、new-game-397-run.sh及new-game-397-verify.py為本機忽略重生入口；沿既有Go image，900s／state850s／2GiB／2CPU／128pids，UID/GID1000、network none、owned PID trap，原ZIP／patch唯讀。396原Go／JSON／PNG／失敗239份產物均保持，不覆寫。

READY審查以396原結果、原callback／IRQ／物理裝置與7357µs實測為依據；新release只是正常測試輸入，原自然返回結果仍未知。正常存讀、跨殖民地與remake同狀態未驗，主庫RE閘門保持。

### READY改名守衛修正

首次原guest session38119在原選族畫面因私有改名將35039662誤改35039762而被來源守衛拒絕，沒有送RETURN，也沒有原CPU停止證據。failed1-397-manifest.json保存全部該次產物。原Go修正僅限new-game-396-及moo2-396-檿名的前綴替換，新增來源檢查比對整份基底其餘bytes與固定數值保持；再由精確patch反轉至原396。保持原資源、輸入、步數與唯一Step，不削弱選族守衛。

## 原版驗收

固定396前五phase與完整393結果保持，正常release的實際elapsed 47384µs。原released-zero／active清除及C08CA／C0960後，BAD9F的C3真near槽返回0x1004ef，ESP+4且SS相同；原215M末態1A5051仍顯示殖民地畫面，2071AB下一輸入未觀測。16phase／PNG及全部readonly／裝置注入核對通過，沒有guest_cpu_stop或step_error；原215M、418輸入及SAVE10／MOX副本保持。

原timeline：

| 事件 | 原步數 | runtime EIP |
| --- | ---: | --- |
| place-return-press-before | 212590909 | 0x1b0845 |
| place-return-press-after | 212590909 | 0x1b0845 |
| place-return-selector-entry | 212596669 | 0x203fb9 |
| place-return-selector-return | 212597711 | 0x20e1ac |
| place-return-selector-release-guard-rejected | 212597711 | 0x20e1ac |
| place-return-press-consumed | 212606147 | 0x1aceab |
| place-return-release-before | 212606147 | 0x1aceab |
| place-return-release-after | 212606147 | 0x1aceab |
| place-return-gate-20E4EB | 213068102 | 0x20e4eb |
| place-return-gate-20E50D | 213068111 | 0x20e50d |
| place-return-gate-20E516 | 213068112 | 0x20e516 |
| place-return-gate-1B08CA | 213102567 | 0x1b08ca |
| place-return-gate-1B0960 | 213103582 | 0x1b0960 |
| place-return-near-before | 213103589 | 0x1aad9f |
| place-return-colony-returned | 213103590 | 0x1004ef |
| place-return-terminal | 215000000 | 0x1a5051 |

397收據只證明正常裝置放開及原C058A函式自然返回；畫面切換與下一正常輸入當時未驗，已由402補驗。正常存讀及remake同狀態仍未驗。public CPU／DOS及主庫Go玩法保持。初次397 CLI缺檔正對照誤假設退出值1，而原panic為2，原guest前拒絕；保留new-game-397-cli-rejected.txt，修正腳本後同入口重跑。398已核對1004EF控制流，1A5051是槽位篩選邊界。399只讀驗當次分派碼20及完整397保持；400補C4562父入口來源，實際父入口由401補驗，下一輸入及列表畫面已由402補驗。不直接派送ID或寫RAM。

原返回後分派證據回鏈：[398](398-moo2-return-mode-source.md)、[399](399-moo2-return-mode-trace.md)、[400](400-moo2-return-parent-source.md)。原215M及上述timeline保持，未新增正常存讀或remake parity聲明。

後續回鏈：[401](401-moo2-return-parent-trace.md)實際C4562 entry及RET1Ch真SS通過；[402](402-moo2-return-parent-continue.md)保留本篇正常RETURN及完整215M前置後另追父層輸入。本篇16phase／原215M不覆寫。

## 402正常父層輸入與畫面回填

[402](402-moo2-return-parent-continue.md)保持完整215M原前置，於222329889真SS進入C4562的1171AB父層輸入並提前停止；原PNG已可見COLONIES列表、Sol II的6工人／2科學家。原正常RETURN→分派20→C4562→列表輸入鏈與畫面返回已驗。新20控件的RETURN／options後續操作、正常存讀及remake同狀態仍待驗，本篇原來源與收據不覆寫。
