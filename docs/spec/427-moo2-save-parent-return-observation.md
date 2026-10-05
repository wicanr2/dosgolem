# 427：原保存父層返回與主分派只讀觀察

狀態：**DRAFT，原250M未達父層返回與分派；檔案交易、獨立PNG及有界生命週期已驗**
日期：2026-10-05

來源為[424原保存交易](424-moo2-save-create-continue.md)、[425父層旗標](425-moo2-save-parent-return-source.md)與[426直接caller及分派](426-moo2-save-parent-caller-dispatch-source.md)。固定官方1.31 EXE SHA-256 4e11be14217b4aafa1839f333bf5eba037f98b0c44e9e4752c96c464c260419f，dosgolem_high_le；IDA static各減F0000h，原file offset另保存。只授權私有只讀觀察器，公開CPU、平台與主庫玩法不變。

## 範圍及固定前置

重用424私有Go與423同一建立能力／受控mtime。完整420的26phase／末態／全部祖先、422前六phase／專用PNG及舊拒絕保持；424完整失敗625份及16份執行前SHA不改寫、不重跑。沒有新滑鼠／鍵盤、RAM／CPU寄存器或檔案內容注入，也不替原EXE製造成功。

新旗標DOSGOLEM_MOO2_SAVE_PARENT_RETURN_CONTINUE只接受1，執行前要求SAVE_CREATE_CONTINUE=1、完整前置與state，MAX_STEPS仍185M以進入舊前置；真正run bound250M、snapshot cap512。旗標關閉保留原424規則，新產物一律另存427；精確反轉patch驗證原424 bytes。

父層原entry17012F與真SS return1004AB，只記錄core／實際step，無新的裝置輸入。完整source422凍結後沿原保存交易，父層entry若不符或未觀察即拒絕，不猜補stack。先核對原53次buffer／close／保存callee返回，再觀察父層16DA12 C3及同entry SS/SP／真slot1004AB，下一唯一Step必須真返回1004AB、ESP+4。

父層返回後觀察第一個1006A7分派JMP：讀實際R0、原44項CS table1003EB與完整原target表，讀原word281A08；核對R0<44、table相符與下一唯一Step EIP為原表target後停止。目標未到、step error或250M上限就保存實際末態，不提升上限。這是父層返回及場景分派收據；第一正常reader、可操作GUI和正式存讀仍另驗。

## PNG保留修正

保持原phase kind及prefix對拍欄位。每個callee phase的原生PNG產出後立即複製到按ordinal／step／kind唯一命名的427檔案，O_EXCL拒絕碰撞，核對frame SHA並另寫artifact manifest。每個事件皆有獨立實體PNG，不能只留hash或用舊圖補齊。source420／422專用PNG照常原生重生；424缺失48事件保持未知，不補造。

## 容器及時間邊界

沿既有Go1.24.13 image、UID1000／network none／3GiB／2CPU／128pids／GOMEMLIMIT1GiB。容器內GNU coreutils9.1 timeout接管guest：TERM1150s、KILL-after5s；完整腳本另owned1300s，外層1450s只保留解壓／建置／after-capture與控制面餘裕。原步數250M不加長。timeout124／137與probe0分開保存，不把逾時當成功。trap清理guest wrapper與capture，有界等待後完成資源／state／base manifest。

已讀容器timeout --version／--help，保存new-game-427-timeout-version.txt及help.txt；標準工具契約以這份已實測help為來源，不深挖遊戲runtime。先用普通完成、TERM逾時、忽略TERM的KILL及外部TERM案例驗證程序群組／capture收尾，再跑原guest。native開始前固定腳本、來源、CLI、工程測試及獨立verifier SHA。

## 驗收與限制

READY後建立私有Go、run／capture／generator。精確反轉、唯一CPU.Step、原getter與注入數保持，CLI原315項前綴加新守衛案例、隔離重生、PNG碰撞工程案例、owned timeout工程案例通過後才授權唯一有界guest。獨立原LE／CALL／RET／table verifier在guest前固定；逐bytes檔案交易與每phase PNG均驗，實際GUI人工另看。

parent或dispatch未到則回RE／spec，數值／snapshot不能外推下一正常input、正式讀回或remake parity。固定日期／mtime不是RNG等價seed；沒有亂數玩法對拍。源碼、PNG、save／JSON留本機。入口000-index與424／425／426同次回鏈；主庫RE-first保持。


## READY後實作與啟動

私有觀察器由原424經11個精確反轉patch產出；唯一CPU.Step／原getter／滑鼠及鍵盤注入數保持，沒有新CPU／guest RAM寫入。保存callee各phase立即按ordinal／step／kind存獨立PNG並以O_EXCL拒絕碰撞，原phase欄位／source420與422前置保持。父層entry、真RET及原44-target table只讀觀察已建置。

325CLI含265拒絕／60正對照，原315項前綴保持。五份產物在隔離/tmp逐bytes重生；實際PNG archive closure用兩張不同原生圖片驗同kind留存與碰撞拒絕。GNU timeout普通完成0、TERM逾時124、忽略TERM的KILL、外部TERM均確認所有子程序停止；原state capture也完成最後副本。限定工程CONFORMED；原版另確認在owned時間邊界內自然完成，未觸發timeout信號。

第一次生成器trap引號SyntaxError與PNG測試harness全域換行替換失敗均保存failed1／failed2-427，修正同image／命令重跑，沒有原guest。啟動前發現native會重跑工程測試並改寫耗時收據；原24份啟動輸入及READY快照保存failed3-427-manifest，native改為只驗證既有工程證據，重新前置／重生後固定one-guest-ready-v2。前述舊快照在原guest前SUPERSEDED，不是第二個guest。

獨立new-game-427-verify.py在guest前固定，直接從原LE重建code／fixup／44項表，不使用生成器target map；驗真CALL／SS RET、全部phase各自原生PNG與有序buffer／file。24份v2執行前SHA固定後，唯一原session23415、容器moo2-save-427-20261005／f36706eefe17自然到原250M上限，外層及owned腳本退出0，耗時938.771s；容器已移除。父層返回與分派均未到，整體回DRAFT。

本機新增入口new-game-427-one-guest-ready-v2.json、prelaunch-correction-review.json、generator.py／tests、patches.json、run.sh、implementation-source-verify.py／tests、regeneration.py／result、tooling-tests.py／txt／engineering-result、verify.py、launch.json與owned-lifecycle-result.json。原v1與424原16份輸入／完整失敗保持；實際結果以verification-result、conformance-review及owned-lifecycle收據為準。

## 唯一原版結果及未通過範圍

獨立verifier退出0，只證明已觀察範圍。53個原fwrite call site全部實際return=count，有序buffer合成208000bytes，逐bytes等於SAVE1.GAM，SHA-256 2e587bf7437efcf73ffe0928276cd36ea9f848da63249fb2c6a64c3db349b375。原fclose真return0、保存callee近RET返回16E3F9已驗；原base418檔保持。MOX.SET553bytes變更照實保留。這是檔案交易，正式存讀往返仍未驗。

418只讀phase各有獨立原生PNG，418個不同路徑全部核對。完整420／祖先與422前六phase／專用PNG保持。424缺失的48張不補造。427最後PNG與424最後PNG相同，畫面可見星圖、GAME及SAVE按鈕，slot列表消失；不能外推正常輸入或成功回饋。

原父層entry17012F在227148176實際命中，真SS return1004AB與426來源相符。原250M末態EIP238B82、虛擬時間576422528µs。parent_returned=false、dispatcher_reached=false、next_input_reached=false；不得稱正常玩家返回CONFORMED。cgroup峰值1844297728bytes、OOM增量0。owned腳本在938.771s完成，低於1150s guest／1300s整批邊界；原版未送TERM／KILL，信號處理能力只由四個工程案例證明。

failed4-427-manifest.json凍結1080份本次實際產物，原24份v2輸入SHA保持；conformance-review保存限定成功與整體未通過。新[428末態邊界來源](428-moo2-save-terminal-support-boundary.md)只核對linked圖像支援候選與caller，沒有重跑guest或提高250M。下一窄任務是原parent局部退出旗標與保存callee後的玩家consumer；不深入掃描線helper，也不以延長上限追成功。主庫RE-first保持。
