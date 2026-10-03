# 368：可寫旗色頁正常紅旗輸入與後續開局觀察

狀態：**CONFORMED，限定正常紅旗輸入、原寫檔與新CPU拒絕**
日期：2026-10-04

## 範圍與來源

沿[367-moo2-overlay-ruler-accept](367-moo2-overlay-ruler-accept.md)原99M旗色初態，建立隔離正常紅旗輸入profile，保存原正常選擇／後續UI／存檔結果與新能力缺口。固定紅旗只是重播fixture，未改主庫玩法、正式預設或typed資料。主庫RE-first保持。

原官方1.31 ORION2.EXE SHA-256 4e11be14217b4aafa1839f333bf5eba037f98b0c44e9e4752c96c464c260419f，原ZIP417根檔加官方EXE共418檔，MOX.SET、1996-01-01、44M Esc、兩組NEW GAME、80M設定ACCEPT、90M Humans與95000065預設名稱ACCEPT保持。日期不是seed。原位址dosgolem_high_le，工具起點a649d0b9d035eec8d8f57235b7e54a30cd7848bd。

367原收據SHA-256 4e241f99d868c28cde3107d884b68136769d8dd9bb158021edbc2f9547d34725，99M完整550byte表／globals192bytes／header16bytes、CPU／FPU／VBE／RGB／callback與IRQ已取。原640×480旗色頁親看，PNG96efbd1ce6538c27b019cc6713fe7397d6d0fde7a63c7e01cb82023f75614c67與339唯讀原旗色圖相同；唯讀339完整CPU／表不同，不取代本輪初態。

## 預定正常輸入與拒絕契約

私有DOSGOLEM_MOO2_OVERLAY_BANNER_RED_PROFILE=1只接受值1與完整367可寫名稱profile、BANNER_RED_CLICK及既有180M game-dir空state依賴。無效值／缺依賴在讀EXE前拒絕。旗標關閉保持367在99M diagnostic stop，原339唯讀guard逐byte保留；啟用時在原99M同一執行期分支先保存367原快照，再以獨立完整真實profile審查，不改IF、不skip原指令、不延後挑初態。

固定99M EIP23857C、R=[347B20 0 4B 1 2BD9B8 2BDA14 6AE528 347B6C]、段=[8 188 188 0 20 188]、flags206h，FPU127F／status0／depth0／八stack bits0。VBE Active／Bank4／StartY0／BankSets1724／Writes35039662／DisplaySets48，虛擬185561342µs，RGB8c2c573c08ebf4bce0b4e166d4268fb6fcf90f35dd27fbbb2de614b26f187f15。原target8:2136D1、mask2B／pending0／非活動／callback10／10、IRQ非活動非failed／23503／23503。原globals／header／完整550byte表以實際hash核對、pointer298848／count10／bias0／stride55。任何不符即停，不只替換舊guard的hash。

index1矩形96,144–179,242，映射原x138／y190與139／190唯一命中；按下raw x276／y190／buttons1，放開x278／y190／buttons0。紅旗對應index1仍為強推論，正常消費前不稱正式旗色。

沿[339-moo2-banner-red-normal-click](339-moo2-banner-red-normal-click.md)、[340-moo2-banner-pressed-consumer](340-moo2-banner-pressed-consumer.md)、[341-moo2-banner-after-gui-return](341-moo2-banner-after-gui-return.md)已有正常輸入契約：一次固定press；保留既有原24C31B的INT33 AX3 pressed查詢、214104自然RET20DB5B／20E165返回AX1、原stack top與完整核心只ESP+4、段／flags保持。首個匹配caller不符即拒絕，不挑後續返回。完成兩個原GUI返回且callback完成一次、至少20000µs、同target／IF／pending／IRQ／mask1或2B安全條件下首次合法release。既有觀察與放開閘門保持，不寫CPU／RAM／選擇／EIP／持久資料。

## 後續原版觀察與驗收

只移除明示profile的99M停止，沿既有180M解析參數與有界loop自然執行；不新增診斷上限或輸入。保存既有100M…180M checkpoints／正常UI／原20DDDB共享store首筆、原DOS開檔／存檔與實際state清單。新CPU拒絕／DOS錯誤或guest提早退出據實記錄，exit0不等於180M或完整開局完成。原版是否真正選色／生成／存檔仍待native結果，不從回呼或頁面推定持久writer。

367在99M停止前的所有共通列依352既有mtime／DTA／每輪只讀RAMhash正規化保持，30PNG逐byte保持；新增profile採獨立診斷列，不改原339 precondition列。三個私有變更可逆為367，公開CPU／DOS／provider／probe與全部internal保持。四CLI拒絕與合法缺EXE正對照；原418檔前後SHA-256保持，state保存實際bytes／SHA-256／UID／GID，是否多出SAVE10.GAM依收據，不猜內容語意。

原資料→完整表→正常press／原poll及caller→共享選擇／後續UI→state，逐段只聲明實際到達範圍。typed名稱／旗色、完整規則與地圖／RNG／音訊／remake同狀態未驗；不深挖renderer／allocator／runtime helper。Go1.24.13映像sha256:1a6d4452c65dea36aac2e2d606b01b4a029ec90cc1ae53890540ce6173ea77ac、600s／2GiB／2CPU／128pids／UID1000／network none，原ZIP／patch唯讀。私有workplace/moo2-banner-red-368.go、new-game-368-run.sh；本規格同次掛000-index，收據掛主庫既有研究入口。原LOG／PNG／RAM／state與素材不入Git。

## READY審查

直接核對367同guest99M原收據，workplace/new-game-368-ready-review.py通過。完整原CPU／FPU／VBE／RGB／callback／IRQ、虛擬185561342µs與唯一index1吻合；globals SHA-256 9dc776e75d99d4bfd31d5ed28c58c01c72854c990c472adc97edc345bdcfe883，header SHA-256 0943dfdc49b8bd9a5899e8155bd7b98553f3a0dfa5402eeb37ad6a41261711d9，550byte表SHA-256 644348154fe8afe4dcb68ec333c8351f84fc729d55babb6a551567f2f71af947。既有339–341正常poll／兩個原RET與放開閘門直接核對source，沿用其完整框架與首匹配失敗即停契約，不從新初態猜持久旗色。READY只允許隔離fixture；原正常選色結果／生成與存檔待實測。初版審查腳本誤用診斷名稱banner_gate_return，依原source改為banner_red_selection_gate_return後重讀同367收據通過；尚未啟動368guest，不是產品缺陷。

## CONFORMED限定結果

狀態：**CONFORMED，限定正常紅旗裝置輸入、原GUI返回、成功寫檔與新CPU拒絕**。原旗色持久語意／正式讀檔／整段開局尚未完成。

已證實99M獨立完整profile通過，原339唯讀guard false保持。一次press99000000／185561342µs／x276／190／buttons1。原99103163在24C31B的INT33 AX3返回BX1／CX276／DX190；99103343原214104 RET20DB5B返回AX1，99103698同原RET20E165返回AX1。兩者原SS188:ESP2BD998、stack top分別5BDB2000／65E12000，完整R只ESP+4、六段／flags202h保持，readable／readonly／valid true、error nil。99103699首次合法mask1 release／185890422µs／x278／190／buttons0，差329080µs、callback11／11完成，不代寫CPU／選擇／RAM。不重送、不重跑guest。

已證實160M原Placing home worlds畫面親看。原165058686、dosgolem_high_le:237024的INT21 3D01對SAVE10.GAM返回handle9／CF0，165058731在237093的40／CX0正常truncate。13筆40包含truncate；後續238310各筆完整EAX都等於請求ECX，數量合計208000bytes，原3E正常close、CF0。另兩次MOX.SET 3D01各handle9／CF0、truncate及40寫553bytes，再正常close。99筆DOS診斷皆handled、CF0，未發生原存檔權限錯誤。本輪只證實原寫檔到隔離state，不證正式讀取或內容語意。

state實際SAVE10.GAM208000bytes／SHA-256 0cf71fc5368861758da5a5dffbd791f8b711a48cfd295572eb9a21d27dea0f4d，MOX.SET553bytes／de8554b3284c9f9065efe6e4f2dfa71ab0c7e732e75eaea0d0ecf465ab86409f，sound.lbx4250888bytes仍與367及原ZIP相同。三檔UID／GID1000，原418來源guest前後SHA-256保持。同guest存活時以Docker exec另存變動state的本機副本，來源state只讀；原guest結束後最後副本hash／bytes與終態清單逐份一致。中途copy-on-write原副本、truncate0與部分寫入快照不當成最終存檔；不公開任何state bytes。精確monitor命令等價存於workplace/new-game-368-state-capture.py，400s有界且在原bounded容器內，不留下背景程序。

已證實原165113094在169E49 bytes20 D0 59 C3 53 51拒絕opcode20，EAX18900／EDX601／EBX0F／ECX0／flags216h，段=[8 188 188 0 20 188]。ModRM D0為register AL destination／DL source，AND byte能力缺口；當輪CPU保持，未猜補結果。probe shell exit0但guest_cpu_stop／step_error確實存在，未達180M、不稱完整開局。終圖黑底游標已親看，原共享20DDDB旗色store未命中。由正常poll／兩caller到原生成與寫檔的流程已驗，紅旗正式持久欄位仍未知。

驗證：367前99M的7846共通原列依352既有mtime／DTA／每輪只讀RAMhash正規化、30PNG逐byte保持；四CLI拒絕與合法缺EXE正對照通過。三私有變更可逆為367，公開internal／CPU／DOS／provider／probe保持。原guest只執行一次。初次READY審查的診斷名稱誤拼在guest前修正；原結果未重擲。

## 跨規格回填台帳

| 不可變鍵 | 語意與等級 | 舊規格 | 回填 |
|---|---|---|---|
| 官方1.31／dosgolem_high_le:24C31B、214104→20DB5B／20E165 | 可寫旗色正常poll與兩原GUI返回後release，已證實 | 339、341、367 | 獨立368接通，唯讀舊收據保持；正式旗色writer未知 |
| 官方1.31／dosgolem_high_le:237024／3D01／SAVE10.GAM、237093／40、238310／40 | 原正常truncate／208000bytes／close到隔離state，已證實 | 362 | 362唯讀CF1仍有效；368可寫CF0及實際副本限定解出 |
| 官方1.31／dosgolem_high_le:169E49／20D0 | 新AND byte拒絕邊界，已證實拒絕；指令結果未知 | 本規格 | 另立CPU規格，先READY再補支援 |

339／341／362／367同次回填，私有驗證核對原定位／bytes、舊文件與回填標記缺項即失敗。主庫玩法RE-first保持，原LOG／PNG／RAM／state只留忽略workplace。

## 命令與下一步

```text
python3 workplace/new-game-368-ready-review.py
bash workplace/new-game-368-run.sh
python3 workplace/new-game-368-verify.py
```

均於既有Docker通過；監測同次native，不重跑輸入。下一步以原165113094／169E49的20D0與完整核心另立byte AND規格，補正常register-source consumer、CPU窄測與原前綴保持後沿相同輸入續行。不新增點擊或改cap；正式讀檔／完整開局／RNG與remake同狀態未驗。
