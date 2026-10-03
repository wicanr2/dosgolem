# 345：宇宙生成迴圈的有界進度與正常返回

狀態：**CONFORMED**，限定範圍見驗收節
日期：2026-10-03
範圍：只觀察DOS1.31目前正常玩家路徑，核對120M內的迴圈計數與正常caller返回。CPU／平台／主庫玩法／亂數／輸入／120M上限保持。

## 原問題與定位

沿[344完整標準SETcc暫存器條件](344-cpu386-setcc-byte-register.md)，主庫33fafba87b14e1bcec47a89df79dc33641bdc6a0、工具d5127adc64a04af79796d933aec73413bbcfd824。官方DOS1.31 ORION2.EXE SHA-256 4e11be14217b4aafa1839f333bf5eba037f98b0c44e9e4752c96c464c260419f，所有原位址dosgolem_high_le；CPU b5c8bd900047e8f018ad66edaf5ae8e1009e9f6d04ec89dea4000c27f3326b30。

已證實：344正式收據workplace/moo2-probe-344-red.txt.gz SHA-256 2724628fa913674f41df2af004c3ec4863b02cfb105695c6864f1bd35f4610a9同120M到17FCE4且無新CPU拒絕，終圖仍「Generating Universe...」。最後32步包含17FCC3..17FD13與EAX／EDX計數；較早114480000..118980000同17FCE4樣本的EAX各不相同。這只證明狀態變化，尚不能證實有限正常返回或宇宙生成完成。

## 最小蒐證契約

先以本機忽略private prototype，對114M／117M／119.9M後首個原17FCC3各臂一次，最多192個原CPU.Step，保存before／after完整R／六段／flags／16指令bytes、固定原frame窗口與stack。窗口保留raw bytes／selector／offset，不先猜欄位名。初遇的原SS:EBP+12 dword按已保存的PUSH／ENTER框架當candidate返回位址；後续由正常RET、實際stack bytes與caller到達核對，不代寫EIP或提前返回。每個group最多一筆正常返回，未返回保留pending。

observer只能讀，沿activationPeek核對CPU／Bus／FPU／VBE身份及完整RAM前後SHA-256。完整RAM SHA-256改在各group的arm／首步／末步及真正RET前後抽樣，其他步只以activationPeek核對CPU／Bus／FPU／VBE，明記ram_checked=false；pure peekSourceWindow只作descriptor範圍檢查與copy，沒有guest寫入。全部原344列／32PNG仍必須保持。每筆最大192步，三group合計最多576步；正常返回觀測只在已臂group後等待相同原frame的真正RET。原版只以同120M情境重生一次，不先延長cap、不重送、不略過迴圈或原helper。prototype三個有界區塊逆轉後source必須逐byte保持344；CPU與平台不改。

核算private原全部344列／全部PNG保持，獨立從指令bytes、R與旗標核對實際計數增量及signed branch條件；數據足以確認正常推進或真阻塞後才READY。若正常原返回已證實，正式probe只保留有界觀測，不擴寫原helper規則；下一個診斷預算需按觀察數據給出明確停止條件，不盲提高cap。

## 驗收與停止線

正式source與private一致時可以重用同一充分原收據，不因private→public再跑相同原流程；差異必須逐byte檢查。舊338／339 CLI、345缺證據／較早344回填／索引護欄必須通過。CPU／平台完全保持344，本輪不重跑無關CPU全套；原固定EXE上輪全套只是既有回歸基準。

生成完成／下一玩家頁／持久姓名旗色writer／typed種族特性／正式RNG與remake同狀態均未知。相同seed數字未控制，固定1996日期不是seed。迴圈計數與正常返回只閉合動態診斷，不計玩法分母，不逐行解helper，也不宣稱宇宙生成完成。

原ZIP 3a28a52f5953ff6d8fc251548940500236752ee19b52b51581e71ec1a3373c2f、patchZIP908d6b7b37ad580039c5d108bab2c64b28f51ba735485287d284d5f5242b98e5唯讀，fresh417根檔／MOX.SET553bytes bfd6855a41760b31156b96114b5b33c88f442ab8f8aae020c1740b3b486a3a80。Go1.24.13 Docker image sha256:1a6d4452c65dea36aac2e2d606b01b4a029ec90cc1ae53890540ce6173ea77ac，network none／2GiB／2CPU／128pids／UID1000／原版外層300s。原LOG／PNG／RAM／private source留本機忽略workplace，不入Git。收尾檢查擁有權／Git與Docker清理。

### 蒐證環境修正

首次private單檔建置缺既有IRQ原型，原版未啟動；沿341入口加入兩個原型檔後原版才啟動。第二次外層300s逾時exit124，容器已結束，未產生終態LOG，不當作CPU或玩法失敗。576筆觀測各需完整RAM前後核對，增加診斷成本；保存已產生的partial PNG為345-attempt1，原版步數仍120M，外層改450s。新命令將raw LOG寫忽略workplace並以EXIT trap保存gzip，避免環境中止丟失既有觀測；CPU／輸入／觀測規則保持。

### 真因與prototype修訂

450s再次環境逾時，保存495筆完整觀測至moo2-probe-345-attempt2.txt.gz；全部8444可比原344列保持，39次INC／44次CMP／44次JL／51次MOVSX／41次MOVZX數學核算通過，第三group只111步，不當576或120M完成。12次65,818,624-byte SHA-256實測1.029894684s，逐步2304次約3m17.739779328s，足以解釋額外成本；不是CPU拒絕。

原code17FC82開始56 57 C8 14 00 00，框架先PUSH ESI／EDI再ENTER；17FD1B..17FD20為C9 5F 5E C2 14 00，返回位址在EBP+12而非+4。原三frame的+4=2BDA74是資料位址候選，+12=17F037才是code候選；原+4 prototype未捕捉RET的負結果保留，不冒充已驗。新v2改+12並以真正RET／raw stack驗證。移動cold觀測buffers到有界state，避免原120M步未臂時重複分配；RAM雜湊按上節明示抽樣，576筆核心與raw窗口照常保存，正常輸入／CPU／120M保持。外層回300s，相同固定工具鏈乾淨重跑。

## READY 證據審查

v2正式完整收據核算通過：全部8503原344列／32PNG保持，三組各192步共576，45次INC／51次CMP／51次JL／60次MOVSX／48次MOVZX以數學獨立核算。114058778與117106151原17FD1E C21400真正返回17F037，原stack37F01700／ESP2BD9F8→2BDA10／其他R六段flags246h保持；第三組在120M仍pending，不猜返回。首／末步完整RAM各group抽樣，其他步明示ram_checked=false，CPU／Bus／FPU／VBE每筆核對。純reader不寫guest。三prototype區塊逆轉保持344，CPU／input／cap保持，允許把v2相同source升為正式有界診斷，不再重生相同原流程。

## 限定驗收：有界迴圈計數與兩個正常返回

345限定CONFORMED，只閉合三組各192步的有界原計數／分支與其中兩個真正RET，正式writer、生成完成／完整開局與remake同狀態未驗。CPU／平台完全保持344，主庫RE-first保持；這不是新的玩法規格。

**已證實，原計數與分支**：三group於114030168／117001983／119946572首遇17FCC3，各192連續原CPU.Step共576。SS188／EBP2BD9EC固定raw窗口可讀，signed AX比較的EBP-28界值分別18／66／87，BX比較DI15、DX比較由EBP+28讀出的8。45次INC、51次CMP、51次JL、60次MOVSX、48次MOVZX由原bytes／完整R／raw字段獨立數學核算；CMP以16bit有號差與溢位範圍推導旗標，JL採SF／OF四列字面真值，未呼叫CPU helper。其餘資料用途未知，不把這些數字稱為宇宙生成百分比。

**已證實，原框架與兩個返回**：17FC82 bytes56 57 C8 14 00 00先PUSH ESI／EDI再ENTER；17FD1B..17FD20 bytesC9 5F 5E C2 14 00。原EBP+4是2BDA74，不能當原code返回。EBP+12的37F01700才指17F037。114058778與117106151的原17FD1E C2 14 00確實返回17F037，SS188:ESP2BD9F8的實際四bytes37F01700，ESP2BD9F8→2BDA10共24bytes即pop4+imm20。其他完整R／六段／flags246h保持，readonly／valid真、error nil。兩次從觀測起點到返回各28,610／104,168步；這兩次呼叫已完成，不是永遠停在同一次呼叫。第三group直到120M仍waiting=[false false true]，未捕捉RET，不宣稱第三例或所有呼叫已完成。

**已證實，觀察範圍保持**：全部8503原344列／32PNG逐位元保持，原SETG／MOV、SETLE實際SS write、TEST三步與全部原正常press／release均保持。兩份private／正式三有界observer逆轉後逐byte保持344；正式source逐byte等於已驗v2，Go程式、Bus／hooks／原input／calendar／120M cap保持。DRAFT→READY後才複製相同v2到正式probe，不只為public檔名再跑同一充分原流程。

完整RAM SHA-256在各group arm／首步／末步／真正RET前後抽樣；576步中只有六個首／末步ram_checked=true，其餘570步明示ram_checked=false，每筆activationPeek仍核對R／六段／EIP／flags／Bus／FPU bitpattern與VBE。peekSourceWindow只查descriptor範圍並copy原Mem，不寫guest。沒有把抽樣稱為每一步全RAM雜湊或整段完整RAM對拍；原來源逆轉與完整舊列／PNG另驗。觀測buffers移至有界state，未臂時不反覆分配，不影響guest資料。

**環境負結果與真因**：單檔private初建置缺IRQ原型，原版未啟動；修正沿341的三檔build入口。之後外層300s與450s各exit124，未取得完整終態，不當CPU／玩法失敗。450s保存495原步，8444可比舊列保持，獨立算術核算通過；partial仍未到120M。原錯誤EBP+4候選及未捕捉RET的負結果不重寫。65,818,624-byte RAM做12次SHA-256實測1.029894684s，逐步2304次約3m17.739779328s；修訂為明示抽樣並修正PUSH／ENTER框架後，相同工具chain／原輸入／120M於原300s容器乾淨重跑成功。觀測紀錄另持續寫入忽略workplace，保留逾時partial，沒用更大的遊戲步數掩蓋問題。

終態仍同120M／17FCE4／unique_sites37433、R=[48 8 1 5 2BD9D0 2BD9EC 2BDA74 F]／flags297h、無新CPU拒絕。原PNG逐位元保持344「Generating Universe...」，SHA-256 d0e387dc900c9955d82dc9c6b21a533d581ac383ba36abca9ce2de28b290f13a、RGB3eeb511abe9d33ff110ce8e478b7d2c5073d36d6775a56622e64651ca8c63fce；沿上一輪同hash人工圖判讀，不把同圖當新玩家頁。共享20DDDB仍未命中，probe exit0不當完整開局通過。

實際命令：以cp workplace/moo2-universe-345-v2.go workplace/moo2-probe/probe345_input.go與trap清理，go build -p 2 -buildvcs=false -o /tmp/moo2-probe workplace/moo2-probe/probe345_input.go workplace/moo2-probe/irq1_prototype.go workplace/moo2-probe/irq7_prototype.go；正式go build -p 2 -buildvcs=false -o /tmp/moo2-probe ./workplace/moo2-probe。python3 workplace/new-game-345-partial-verify.py／new-game-345-verify.py／new-game-345-source-verify.py PASS。同binary舊338 CLI17無效值與正對照、339 CLI22無效值與100M／120M正對照通過。CPU／平台未改，未重跑無關Go全套；固定EXE上輪全套b7bcf138095b48d20b4a8ca1f43b8f60531a80ae59c6243463f3de52d6e38554只是既有回歸，不冒稱本輪新跑。

private v2／正式source SHA-256 03bb2adb3414b8801f35ea25398a1cffc239befd4994f454050d1074ee659c35；原proto67783bbbccffd894218f5ded77ecf7f1d3ff1607b7260bf1308e2f889ec931df與partial證據保留。正式完整workplace/moo2-probe-345-red.txt.gz SHA-256 c5ffbfda48e4ff7354452ace99bded92c4ccb31f4522e4cdc8534a34d84a4e44。所有原LOG／PNG／frame／private source留本機忽略workplace，不入Git。

### 回填帳與下一步

| 不可變鍵 | 已證實語意 | 證據 | 較早規格 | 必須回填 |
| --- | --- | --- | --- | --- |
| DOS1.31／ORION2.EXE4e11be14…／dosgolem_high_le:17FCC3／17FD1E→17F037／SS188:2BD9F8 | 原計數前進，兩次RET imm20正常返回；第三例pending | 576步／兩RET與8503列／32PNG保持 | 340／341／342／343／344 | 原17FCC3迴圈進度與兩個正常返回已由規格345驗證 |

下一步保留120M原基準，建立獨立固定160M的明示診斷分支，用同正常輸入續行；先定120M同狀態收據與160M有界終態，再DRAFT→READY。160M是一次性觀測預算，不保證生成完成，未驗默認不開放。若仍同頁，先找生成producer／狀態變化，不連續盲提高cap或重送。正式writer、typed種族特性、生成完成／完整開局、RNG、人耳與remake同狀態未知，整款remake／中文化目標仍活躍。

82項規格回填正對照、345新增27缺證據／抽樣／pending／狀態／索引負例與其餘四份較早回填8負例，全部舊負例通過。來源／新收據1000:1000；兩次逾時60partial PNG逐位元保持相應344基準，manifest另保存。

### 本機忽略證據索引

| 收據／核算 | SHA-256 |
| --- | --- |
| workplace/moo2-universe-345.go | 67783bbbccffd894218f5ded77ecf7f1d3ff1607b7260bf1308e2f889ec931df |
| workplace/moo2-universe-345-v2.go | 03bb2adb3414b8801f35ea25398a1cffc239befd4994f454050d1074ee659c35 |
| workplace/moo2-probe-345-attempt2.txt.gz | f6535c62ddea3697415ed3f400e6d29a32a392dc514899f3d368a4ec4958be74 |
| workplace/moo2-probe-345-attempt2-raw.txt | 167e1cacc2710615ca1c2d9099f5565315a41b8c58399309ba2f49e8ebd28f98 |
| workplace/new-game-345-partial-verify.py | 9c12e2d2683f167edb5e657b9d7821a6a68a9dd2e698d6c937cd9eb3bf9c2b65 |
| workplace/new-game-345-partial-tests.txt | a626ff35a5249b429135bb76a3d3e57e10826327530a51481efc7211624a7441 |
| workplace/new-game-345-sha-cost.txt | 2cf6031e6533b102f049772b28efbecccb3cd45ba889a4f7382abba7c4d29178 |
| workplace/moo2-probe-345-red.txt.gz | c5ffbfda48e4ff7354452ace99bded92c4ccb31f4522e4cdc8534a34d84a4e44 |
| workplace/moo2-vbe-345-red.png | d0e387dc900c9955d82dc9c6b21a533d581ac383ba36abca9ce2de28b290f13a |
| workplace/new-game-345-verify.py | c6be67c4a1109f512c070b6166b6733b787944d3e03be6b2c4f930c2228cd180 |
| workplace/new-game-345-tests.txt | 14c2d70635217f3266ad9b811d624e18c805abf7f02f92726635378c173ff096 |
| workplace/new-game-345-source-verify.py | e0087de1adc8490943013c87b1cbca49f193b683e6ed14853909a8023960ee50 |
| workplace/new-game-345-source-tests.txt | d01abf123dd62e684115e636babd2b513ba71b341891a38059ef7e66363e2419 |
| workplace/new-game-345-backlink-verify.py | 35a5c69cffab41f10921538d1c46e1e4b54219b4b75474bdd04524c0d639bdd9 |
| workplace/new-game-345-backlink-tests.txt | 489053fd9832911efae521e38f859c892c1e70d14ae5ce5736854f57a93cce2a |
| workplace/new-game-345-old338-cli-tests.txt | c5d03a8a5858cdeb915036ef21e2a8f0f3627b6007bfbe3e54c3745fb624b6b1 |
| workplace/new-game-345-old339-cli-tests.txt | e4461c40e971daf4de8338567b57b68f0945355ed595b7c5c09620a2590be774 |
| workplace/new-game-345-partial-frames.json | f9f98a7be5291b00d87552a7391808cce66baa3f3e4b3f4db738b7c1408454b7 |
