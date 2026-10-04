# 401：原RETURN父入口與篩選自然返回的只讀追蹤

狀態：**CONFORMED，限定只讀原父入口與RET1Ch自然返回**
日期：2026-10-04

接續[400父入口來源](400-moo2-return-parent-source.md)及[399原只讀結果](399-moo2-return-mode-trace.md)。官方1.31 ORION2.EXE SHA-256 4e11be14217b4aafa1839f333bf5eba037f98b0c44e9e4752c96c464c260419f。399 Go SHA-256 2256ab0cd3ba70900b773612ec2cd57bbad0d1ffd98c597df75bd63d4f24c642；399 mode-terminal SHA-256 b7d1b942581e56f5de8dcc01e7f71776305d094acdd6895120e0189e261764fe。沿Go1.24.13／Python3.11／IDA9.4，原EA為IDA linear EA，runtime=EA+F0000h，原file offset與LE fixup分列。

## 來源與證據層級

已證實：399原B4EF6入口的真SS讀取窗口有16-byte，前4-byte為runtime1B2C5F，原ESP／SS完整保留。新增窄IDA43列／43EA／11fixup差異逐原EXE核對；原C2C5A CALLB4EF6，下一C2C5F跳C2C74，C2C74增加SI，C2C75與3比較、C2C79可回C2B96。此caller位於sub_C2B72 C2B72..C2C84共88指令，direct caller為C2E0A。C2259入口與C45BC caller只保存初始化邊界，不補未證實呼叫鏈。

398原B53B3為C2 1C 00，近返回清28-byte參數。398原1050C CALLC4562、400原C472A CALL1171AB及C4740 CALLC4343構成候選父入口與輸入邊界。強推論：399分派20選C4562。契約建立時，實際父入口、篩選呼叫是否自然返回、原cap內下一輸入及畫面切換未知；本篇與402分別補驗下列結果。

## 只讀實作範圍

保持完整399及原397自然RETURN、唯一原CPU.Step、既有getter、全部正常press／release、原215M cap與418輸入。首次Go patch只改new-game-399-／moo2-399-檿名；不得全字串更換數字。新增COLONY_RETURN_PARENT_TRACE僅接受1，要求原MODE_TRACE及完整RETURN_DEFERRED依賴、MAX_STEPS185M與非空state；讀EXE前拒絕錯參數。mode off等同399。

只在原C058A已真返回後監看runtime10050C／1B4562／1B2259／1B2B72／1B2C5A／1B2C5F／1B472A／2071AB／1B4732／1B4740。捕捉first-hit與既有core／frame／device／四raw範圍／原mode七byte及PNG，前後CPU／RAM／DAC／裝置與讀取值保持。父phase最多32，core與getter不改。

在runtime1A4EF6只讀真SS近返回槽，保存entry step／SP／SS／target與巢狀frame，深度上限64。在原1A53B3核對C2 1C 00、真SS／SP／slot；唯一原Step後須到保存target、同SS且ESP增加32。記錄全部entry／near-before／returned的純觀察計數，前3次保存完整phase及PNG；B5051只計首3次完整phase與總命中數，不追篩選或renderer內部。未回到原CALL後時保留active frame，不能稱停死或完成返回。觀察器不停止guest或注入輸入。

## 驗證與收據

沿原175CLI前綴，再加拒絕／正對照。精確反轉Go patch至固定399，既有getter／唯一Step／裝置輸入呼叫數及公開CPU／DOS保持；mode off來源保持。一次原guest後獨立核對完整399 mode-terminal／events及完整397 return-terminal，僅略既定三個RAM雜湊鍵，不能略其他欄位。

每新phase核對core／frame／只讀前後／四raw範圍／mode七byte／PNG與適用的原source code bytes。entry與近返回依真SS、精確Step及ESP32核對，獨立由原native日誌重建計數與active frame；實際父入口、輸入是否達到、原畫面／控件表均分開報告。SAVE10／MOX副本保持；固定日期不是seed，不宣稱正常存讀或remake同狀態。

本機忽略入口workplace/new-game-401-ida-run.sh、new-game-401-byte-verify.py、new-game-401-generator.py、new-game-401-run.sh、new-game-401-verify.py。沿既有image，原ZIP／patch唯讀、UID/GID1000、network none。原guest900s／state850s／2GiB／2CPU／128pids及owned PID trap；IDA120s／2GiB／2CPU／128pids，bytes90s／2GiB／1CPU。原EXE／JSON／LOG／PNG／private Go不公開；主庫RE-first與玩法保持。

停止線：證據足以定位正常父入口、自然返回與下一輸入後即停止。不深挖renderer、DAC／PIT或平台helper，不為通過延長cap。無CPU異常證據時不把未到下一輸入稱為CB指令缺陷。

## 原401驗證結果

修正版原guest session93429 exit0；source及185CLI含153拒絕／32正對照通過。獨立原EXE／LE重定位逐16-byte code window、真SS及完整399 mode／397 return-terminal比對通過。19個新phase全部只讀，原215M、418輸入、SAVE10／MOX及PNG保持。

已證實：213103600原1050C CALL，213103601自然到C4562；原SP減4、同CS／SS、真返回槽為runtime100511。原C2259於213130688、C2B72於214695178、C2C5A於214695240取樣。B4EF6共有12次entry、10次RET1Ch真返回，11,823次B5051命中；每個原RET的唯一下一Step、同CS8／SS188及ESP增加32均通過。caller返回target分別為runtime1B2C5F與遞迴1A4F44。末態有ID11／12兩層active frame，沒有pending RET；不能把B5051末態稱為停死或輸入等待。401限定215M內未到C472A／1171AB，原畫面切換當時未驗；402已補驗父層輸入與列表畫面，正常存讀仍未驗。

首次runner仍編譯399 Go，parent_value參數在原guest前拒絕；3份failed1-401產物及manifest保存。修正明確Go檿名並加入exact private build input守衛後，185CLI通過。執行中另發現native日誌／state manifest沿用399輸出名稱；保留原399 gzip及manifest，目前實際401 trace完結後保存為401、還原原399 gzip／raw及原manifest，沒有重跑guest。實際執行腳本另存new-game-401-executed-run.sh，clean runner名稱已修正。這些是runner接線問題，不作原CPU缺陷。獨立verifier在執行前核對受版控cpu.go枚舉，ESP=4／SegCS=0／SegSS=5。

[402有界續跑](402-moo2-return-parent-continue.md)已保持原401完整215M前置，補驗原父層首次正常輸入；不新增玩家輸入或代寫RAM。主庫RE-first保持。

## 402正常父層輸入與畫面回填

[402](402-moo2-return-parent-continue.md)保持完整215M原前置，於222329889真SS進入C4562的1171AB父層輸入並提前停止；原PNG已可見COLONIES列表、Sol II的6工人／2科學家。原正常RETURN→分派20→C4562→列表輸入鏈與畫面返回已驗。新20控件的RETURN已由[404](404-moo2-colonies-list-return-input.md)正常點擊回到星圖；options、正常存讀及remake同狀態仍待驗，本篇原來源與收據不覆寫。
