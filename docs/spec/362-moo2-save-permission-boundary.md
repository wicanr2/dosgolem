# 362：正常開局的DOS存檔權限邊界

狀態：**CONFORMED，限定唯讀存檔拒絕診斷；覆蓋層正常玩家驗收DRAFT**
日期：2026-10-04

## 玩家阻塞與輸入

沿[361](361-cpu386-ror-dword-memory-imm8.md)，工具5a2170cfc18b40e890a21dcdcbb10605f084b57d；固定DOS1.31 ORION2.EXE SHA-256 4e11be14217b4aafa1839f333bf5eba037f98b0c44e9e4752c96c464c260419f。同180M正常輸入沒有CPU拒絕，終圖旗色選單顯示Error saving game／Permission denied。真正DOS呼叫／檔名／mode／errno未知，工具唯讀政策與錯誤的關係目前是強推論。主庫玩法RE-first保持。

## DRAFT診斷與停止線

一次性可丟棄probe副本只在universe180、160M至180M範圍觀察INT21檔案服務，最多128筆，保存dosgolem_high_le原callsite／step／function／mode／R／段／flags與返回。開檔及路徑服務讀DS:EDX最多260byte的NUL字串，依原descriptor與Mem範圍直接只讀，不經Bus或修改guest。比對讀取前後完整RAM雜湊／CPU／FPU原bits與VBE狀態；服務原Handle只呼叫一次，返回原handled值。未修改CPU、DOS服務、檔案提供者、輸入、calendar或cap，38PNG與361正規化原列全部保持才可採證。日期1996不是seed。

證據足以描述真正失敗契約後才READY。既有machine.OpenDirectoryOverlayFiles(basePath,statePath)支援現存檔案的隔離copy-on-write；目前沒有建立新檔案介面，不能假設足以解本次阻塞。原ZIP／patch唯讀、base與state分離，state只在容器內新目錄，原遊戲素材／存檔／圖與私有log不散布。不增加cap／代寫存檔／注入成功／重送輸入，不深入原版fopen或runtime helper。若真正需要新DOS能力，回到公開DOS契約與窄spec，不猜補。

## 工具與入口

主要執行器/home/anr2/cht/dosgolem隔離副本workplace/dosgolem，用法與能力見README.md／CLAUDE.md。Go1.24.13 image sha256:1a6d4452c65dea36aac2e2d606b01b4a029ec90cc1ae53890540ce6173ea77ac；600s／2GiB／2CPU／128pids／UID1000／network none；原ZIP與patch唯讀。workplace/new-game-362-input-run.sh及moo2-save-diagnostic-362.go為可丟棄診斷；本檔同次加入000-index。

### 覆蓋層的啟動相容邊界

已證實，工具源碼：le_find_question.go的firstQuestionDOSName要求ListReadOnlyNames；DirectoryReadOnlyFiles已有此介面，DirectoryOverlayFiles尚無。因此不可只換provider後宣稱存檔修法有效。覆蓋層目前只copy既有檔案、沒有建檔／改名，若本次只需現存SAVE10.GAM，可依原唯讀base清單提供既有名稱；仍須審查metadata、原問號搜尋與來源不變測試。對任意外來state檔名、新建檔案或全面DOS搜尋不外推。既有覆蓋層三項測試0.037s通過，只證明現有工程契約。

DOS開檔與錯誤碼契約參考：[RBIL AH3D](https://www.delorie.com/djgpp/doc/rbinter/id/90/27.html)、[RBIL錯誤碼](https://www.delorie.com/djgpp/doc/rbinter/it/80/16.html)。AX05／CF1表示access denied，仍待實際原呼叫確認。

## READY：真正拒絕契約與隔離state

READY於2026-10-04。未改CPU／DOS／provider的原收據f05a445160916c9da6b70ad85d24c18733b7a6d7c6ac1a21921d7e6b9d5669b3，128有界診斷，361全部14498列依既有mtime／DTA／每輪RAM診斷雜湊正規化保持，38PNG逐byte保持。各次唯讀診斷自身RAM前後雜湊與CPU／FPU／VBE保持；服務造成的讀檔RAM改變不算診斷寫入。第一次直接全列比較因重新解壓檔案mtime不同失敗，依352既有正規化讀同收據通過，guest未重跑挑選。不能稱跨輪全部RAM逐byte一致。

已證實：原165025480在dosgolem_high_le:237024的INT21 AX3D01，DS188:2BDB68 ASCIZ SAVE10.GAM，原R=[3D01 270E74 2BDB68 42 2BD6AC 2BDB1E 2BDB68 FFFFFFFF]／段=[8 188 188 0 20 188]／flags202h。handled true，AX0005／CF1／flags203h，其他R／段與全部RAM保持，路徑可讀NUL終止。此處是openReadOnly中WriteFileProvider型別檢查拒絕，provider沒有寫介面；AX5成功handle與AX5／CF1錯誤須分清。本次拒絕不是AH40寫入，也不需要AH3C新建。ZIP根層有SAVE10.GAM，既有overlay足以開寫。失敗早於361的168496272 ROR，後段ROR在錯誤路徑仍是合法CPU契約，不當生成成功證據。

READY只接工具：可選DOSGOLEM_MOO2_SAVE_STATE_DIR只接受完整universe180／game-dir情境的既有空state目錄，實驗在容器/tmp新目錄，沿既有OpenDirectoryOverlayFiles，不建立／代寫game檔、不改open或write DOS規則。未設定時仍唯讀。原base及state不可同根；state啟動非空即拒絕，來源仍唯讀。DirectoryOverlayFiles新增ListReadOnlyNames委派base，因API只copy既有檔、不建檔／改名，既有名單不增；fresh state／原SAVE10 casefold、覆蓋metadata、問號搜尋與來源保持做測試。任意外來state或新檔案目錄的列舉不列範圍。

保持CPU／原DOS服務／其他測試逐byte與舊8M，既有overlay與問號搜尋測試、固定原EXE乾淨Go全套必須通過。正式同180M原正常輸入，首個save open之前共通狀態／frames保持，AX3D01須清CF回真handle；後續真寫入／seek／截斷／close及state bytes／來源全檔雜湊記錄，再人工檢視UI。沒有新stop才稱通過限定存檔契約；若新CPU或DOS拒絕仍記限制，完整開局及remake同狀態不外推。固定1996不是seed，state mtime／FAT時間為平台模型，不能用它證明原硬體時鐘或RNG相同。

## 限定CONFORMED與覆蓋層玩家驗收回到DRAFT

已證實：唯讀128筆診斷、14498正規化舊列與38PNG保持，原237024 SAVE10.GAM唯讀拒絕已由規格362定位。正式公開probe只留兩個診斷區塊，移除即逐byte回361；CPU／DOS服務／provider／所有既有測試保持。原165025480的3D01／SAVE10.GAM／AX5／CF1是已證實的工具拒絕，不是AH40失敗。後段存檔內容與原正常成功writer未驗。

可寫試作沿既有overlay，加base名稱委派與新空state控制；未改CPU／DOS服務。窄測0.053s、既有overlay0.037s、固定原EXE乾淨Go全套CPU386130.397s／machine1.643s通過。第一次窄測把11byte的SAVE10.GAM加NUL欄位預期誤寫12byte，修正測試長度後乾淨重跑；逆轉來源腳本的空行差異同樣修正，皆是驗證腳本問題。

正式overlay試跑exit1，在80M setup_accept_precondition valid=false停止，panic「設定頁ACCEPT點擊的原表或輸入條件不符」，未到165M存檔。沒有送ACCEPT press、沒有PNG終圖，不稱開寫成功或完成玩家路徑。原設定頁RGB3bbe6c339cfbc74e84b3210bccfdc4574073b4d308ef9b90a1720cc5c74e623e、globals／header與幾何相同，但935bytes表七個byte不同。差異全在record11–15的offset+44四byte窗口：從offset+44取四byte窗口為004AFA3C／004B0D14／004AE634／004B2084／004B3294，各增8000h，差異byte在+45及兩筆的+46；其他欄位逐byte保持。這些值為heap地址的解釋是強推論，指向內容與實際消費尚未取，不宣稱已證實標籤指標。第1177共通列開始改變：唯讀1260000 EIP24659F，overlay1410000 EIP238291，早於正常點擊。兩側IRQ／EIP不同，不能把新profile當舊same-state。

本次profile改變前段正常流程，原336的完整表hash閘門正確拒絕；不能刪guard／調點擊時刻／補特例猜過。可寫profile回DRAFT，試作只保存在忽略workplace/moo2-save-overlay-362.go與overlay-files-362-prototype.txt／moo2-overlay-enumeration-362-prototype.txt；公開production不接state旗標／overlay列舉新方法，原provider保持唯讀。原失敗收據不覆寫、不挑選成功重跑。實驗容器exit後state未保留，state bytes／來源全檔收據未完成，不宣稱已驗來源不變的原版正式寫入。新state五拒絕／空目錄正對照及舊8M1693列／PNG／68舊＋32新CLI皆僅驗當輪試作，單元測試不能取代玩家驗收。

下一步最小蒐證：有界保存初段開檔mode／路徑及結果，再於80M直接讀record11–15的+44四byte窗口所指資料，兩側各最多五個128byte／NUL視窗，沿原描述符及完整唯讀CPU／RAM／FPU／VBE檢查。比較真正輸入初態與欄位消費，證據足夠才另立READY正常輸入契約。這是工具驗證修正，主庫玩法RE-first保持，不提高180M、不注入資料／代寫存檔或重送。

### 實際命令與限制

Docker Go1.24.13、固定輸入與資源如入口節。兩條原guest各一次，沒有失敗後重擲或挑選。
```text
bash workplace/new-game-362-input-run.sh
python3 workplace/new-game-362-input-verify.py
  128唯讀診斷／14498正規化原列／38PNG PASS
go test -p 2 -buildvcs=false ./internal/machine -run 'TestMOO2Overlay|TestOverlayWritesPreserveSource|TestDOSOverlay|TestMOO2FindFirstQuestion' -count=1
  試作窄測0.053s PASS
bash workplace/new-game-362-full-run.sh
  試作固定原EXE全套 PASS
bash workplace/new-game-362-formal-run.sh
  試作80M完整表前置不符，exit1，FAIL
python3 workplace/new-game-362-source-verify.py
  試作四probe區塊與base列舉方法可逆轉 PASS
bash workplace/new-game-362-off-run.sh
  試作8M／CLI／五state拒絕及空state正對照 PASS
```

完成聲明只限唯讀診斷與工具拒絕定位。公開源碼不是可寫試作；正式存檔／完整開局／RNG／remake同狀態仍未驗。文件回填鏈與守衛不把DRAFT覆蓋層列為成功。

99項規格回填與新362的29＋32缺證據負例、全部較早負例通過。--check-save-permission-diagnostic-spec-backlinks只驗唯讀拒絕及可寫DRAFT限制。公開probe SHA-256 63b0182fc7311d4e0760dc29906419a1480ea3fa12faf1e58067f61775104bb1；可寫試作f1f2be8d3a2a15cb0fa0d7013b64f8ed97d9798f5b9a2b6d85b69f570602043a；失敗原收據5bc986900a6f8fcd2a5089f93dfbd83a7540a9bb4fac729832b8054770155a62。兩者分開，不把試作與公開程式混稱。唯讀與可寫收據的原檔／腳本雜湊保存於MOO2研究入口docs/re/dosgolem-moo2-intake-20260930.md。歷史formal-run使用當輪試作；未來明示重生用workplace/new-game-362-prototype-rerun.sh，在容器/tmp從固定361 Git輸入與本機已雜湊試作另建隔離source，不覆寫原失敗收據；本輪未執行此重生入口。

## 363前段差異與資料窗口回填

原SOUND3D02與五個+44窗口前128bytes已由規格363核對；可寫玩家路徑仍DRAFT，見[363](363-moo2-overlay-startup-and-setup-source.md)。原1192795／237024的SOUND.LBX讀寫開檔，兩側同原初態但唯讀AX5 CF1／overlay真handle5 CF0。80M五候選值各增8000h，指向的前128byte及RGB相同；兩側各27PNG／各自舊列保持，原418來源檔未變，state僅有內容未變的sound.lbx副本。之前「所指內容未取」已限定解出開頭128bytes，完整物件／角色／原指標消費仍未知。下一步依363另立DRAFT可寫profile正常ACCEPT前置，不忽略位址／調時刻／改舊336唯讀guard。完整存檔／音訊／RNG與remake同狀態未驗。

## 364可寫ACCEPT限定驗收回填

可寫profile正常ACCEPT與90M選族頁已由[364](364-moo2-overlay-setup-accept.md)驗證。原336唯讀guard保持，獨立profile核對原overlay完整935byte表／五窗口後一次press／release；真正20DDDB選擇store word0000→0F00，原SELECT RACE畫面與既有原圖相同。原418來源檔保持，state仍僅sound.lbx，不稱存檔成功。下一步依新90M完整880bytes表另立Humans輸入規格，完整可寫玩家路徑／RNG／remake同狀態仍未驗。
