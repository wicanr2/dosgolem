# 399：原RETURN後分派狀態的只讀追蹤

狀態：**CONFORMED，限定原分派碼只讀追蹤與完整397保持**
日期：2026-10-04

接續[398原來源](398-moo2-return-mode-source.md)。只授權私有原版觀察器，不增加玩家輸入或cap。官方1.31固定EXE SHA沿398。397 Go SHA-256 6ef58e19e94671a0a4281cc9147e6e9ec9d9bf78816631f52621e7e0fe201f94；397原return-terminal SHA-256 d6ecbf67eb19ec92c36a4ff288f6faf11c02c26d1b8de64f11c2f78f14cf73f6。

## 固定前置及觀察

保留完整397正常流程、154加11共165CLI prefix、唯一原Step／既有getter／正常press與release；原215M cap不改，mode off保持397。新COLONY_RETURN_MODE_TRACE只接受1且要求完整RETURN_DEFERRED及既有依賴／MAX_STEPS185M／非空state，讀EXE前拒絕缺項／錯cap。Go patch只改明確產物檿名，整份其他原基底bytes保持，不使用全字串數字改名。

只讀DS188四個原始定位：IDA191A08→runtime281A08 word、191A10→281A10 word、1979E8→2879E8 word、191F19→281F19 byte。使用既有activationPeek與descriptor邊界，不更換Bus身分或寫RAM。每phase保存原byte／位址基準、core／frame／device／四既有範圍及PNG；前後CPU／RAM／讀取值須保持。新phase最多24。

只在正常RETURN已放開後觀察C0925／C093A／C0942／C0948／C094E原恢復分支；每個實際MOV writer另捕捉唯一原Step後的改變。原C058A真返回後再觀察1004EF／1004F6／100687／1006A7，以及case1 C058A、case0 86188、case8 8012F、case39 8B956的入口。B4EF6及當次B5051只保存首個原邊界，不深入helper或注入mode。監看寫入末訪byte_191F19的原104EF並核對其唯一Step。

末態完整397的16phase／terminal及215M boundary逐欄位比對，僅略三個RAM雜湊鍵；新只讀觀察不得改原裝置、流程或畫面結果。原418輸入及SAVE10／MOX副本保持。固定日期不是seed，不宣稱remake同狀態或正常存讀。

## 來源與驗證閘門

實作可精確反轉至固定397 Go，首次patch僅new-game-397-及moo2-397-產物前綴，並保持35039662等全部原數值。新模式CLI拒絕／正對照接在完整165 prefix後，公開CPU／DOS與主庫Go玩法保持。一次原guest後獨立核對原末態、每只讀phase與PNG、原MOV code bytes／raw field差異及實際分支，不依EAX、檔名或舊文字猜目的地。

本機忽略入口workplace/new-game-399-generator.py、new-game-399-run.sh、new-game-399-verify.py。沿既有Go1.24.13，900s／state850s／2GiB／2CPU／128pids、UID/GID1000、network none、owned PID trap，原ZIP／patch唯讀。原397／396及239／45份失敗產物保持，不另建image或在主機跑工作負載。

READY來源審查：398原320列／mode兩恢復分支／caller及397真返回／table18／215M末態通過。當次mode原寫入由下列收據補證；父入口尚未直接取樣。原畫面切換、下一輸入與正常存讀仍待後續正常玩家路徑驗證。主庫RE-first保持。

## 原399重播及獨立核對

原guest session76282 exit0，175CLI含145拒絕／30正對照、完整165舊prefix及精確反轉固定397通過。215M cap、正常press／release及完整397的16phase／terminal逐欄位保持，僅略既定三個RAM雜湊鍵；11個新phase全部只讀，PNG與原MOV code bytes／唯一Step差異核對通過。原418輸入及SAVE10／MOX副本保持。

| 事件 | 原步數 | runtime EIP | 原raw四欄位，十進位 |
| --- | ---: | --- | --- |
| place-return-state-gate-1B0925 | 213103510 | 0x1b0925 | 1 / 20 / 20 / 20 |
| place-return-state-gate-1B093A | 213103514 | 0x1b093a | 1 / 20 / 20 / 20 |
| place-return-state-writer-1B093A-after | 213103515 | 0x1b0940 | 20 / 20 / 20 / 20 |
| place-return-state-gate-1004EF | 213103590 | 0x1004ef | 20 / 20 / 20 / 20 |
| place-return-state-writer-1004EF-after | 213103591 | 0x1004f6 | 20 / 20 / 20 / 1 |
| place-return-state-gate-1004F6 | 213103591 | 0x1004f6 | 20 / 20 / 20 / 1 |
| place-return-state-gate-100687 | 213103592 | 0x100687 | 20 / 20 / 20 / 1 |
| place-return-state-gate-1006A7 | 213103599 | 0x1006a7 | 20 / 20 / 20 / 1 |
| place-return-state-gate-1A4EF6 | 214695241 | 0x1a4ef6 | 20 / 20 / 20 / 1 |
| place-return-state-gate-1A5051 | 214695335 | 0x1a5051 | 20 / 20 / 20 / 1 |
| place-return-state-terminal | 215000000 | 0x1a5051 | 20 / 20 / 20 / 1 |

已證實：原C093A於213103514→515把word_191A08從1恢復20；原104EF於213103590→591把byte_191F19從20設1。四欄位末值為20／20／20／1。原分派runtime1006A7在213103599的EAX為20；398原完整44項表中raw20指向1050C，該處CALL C4562。

強推論：下一父分支為C4562。399未監看raw20的實際入口，因此沒有直接入口收據，不把來源推導寫成原函式已觀測。原case0／8／39及C058A重入入口均未取樣；未取樣不等於證實未執行。原215M仍在1A5051，PNG SHA-256 741bb4ca62f46653dc9c65d0b3d6639b93cbf9c8b71758e3864988105de89ee1與397相同。399當時未驗畫面切換與下一輸入；402已補驗。

[400父入口來源](400-moo2-return-parent-source.md)補原C4562的輸入／返回邊界。401已補原1050C／C4562實際entry與12次B4EF6呼叫／10次RET1Ch自然返回；原215M保持。下一正常輸入已由402保持完整215M前置後有界續跑補驗，不新增輸入或改RAM。正常存讀及remake同狀態仍未知，主庫RE-first保持。

## 401實際父入口回填

[401](401-moo2-return-parent-trace.md)於213103600原CALL後，213103601自然進入C4562，真SS／CS與SP−4、返回槽100511通過。399當時未取樣的entry已由401解決，399原收據與完整397保持。下一正常輸入與畫面切換已由[402](402-moo2-return-parent-continue.md)補驗，不把entry等同於完整畫面。

## 402正常父層輸入與畫面回填

[402](402-moo2-return-parent-continue.md)保持完整215M原前置，於222329889真SS進入C4562的1171AB父層輸入並提前停止；原PNG已可見COLONIES列表、Sol II的6工人／2科學家。原正常RETURN→分派20→C4562→列表輸入鏈與畫面返回已驗。新20控件的RETURN已由[404](404-moo2-colonies-list-return-input.md)正常點擊回到星圖；options、正常存讀及remake同狀態仍待驗，本篇原來源與收據不覆寫。
