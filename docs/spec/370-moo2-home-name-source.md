# 370：母星命名來源與正常輸入前置觀察

狀態：**CONFORMED，限定只讀來源觀察**
日期：2026-10-04

## 來源與範圍

沿[369](369-cpu386-and-byte-register-source.md)同可寫180M正常輸入。官方1.31 ORION2.EXE SHA-256 4e11be14217b4aafa1839f333bf5eba037f98b0c44e9e4752c96c464c260419f，工具eca6a803aba5176b87f27c5defd1146ff121c226；位址空間dosgolem_high_le。原369收據59254d0a66659a475cf28d20be1d32ec4fcd131c424cd1ea9745fa225446b631。原170M／180M的table298848、count3、stride55已證實；index1矩形227,246–324,273、+24指向261AC2；index2矩形165,200–236,226、+24指向28439D。候選與標籤在此流程的真實bytes尚未取，不從Sol圖像推定正式名稱writer。

只補兩個正常時點的唯讀觀察，不送新的滑鼠或鍵盤輸入，不改cap／state／核心／正式玩法。裸20 CPU保持369版本，其他公開internal與probe保持。來源→完整表／候選→後續正常ACCEPT契約；本規格不聲稱點擊或持久化已完成。

## DRAFT取樣契約

170000000在同原checkpoint之後、原CPU.Step之前取樣；180000000在原cap取樣之後取樣。使用原globals26C480的pointer與header29BE0E的count／bias，限制count3／bias0，完整165byte表可讀。index1+24取16byte原始窗口，index2+24取32byte候選，原指標保留，不賦予型別或正式欄位名。另取當前code16／SS:ESP stack16、完整R／六段／flags／FPU bits、VBE／RGB、虛擬時間、target／callback／IRQ及整RAM前後SHA-256。

沿既有activationPeek核對核心／FPU／VBE保持，額外核對整RAM及callback／IRQ前後完整狀態保持。兩次均記錄可讀性與實際bytes；不可讀明示拒絕，不猜字串或fallback指標。觀察兩筆，沒有native重啟、重送或重擲。現有四CLI拒絕與合法缺EXE正對照保持。

## 驗收與工具

剝除新function與兩call後逐byte等於369私有probe。公開internal／probe逐byte保持eca6a80。原369全部共通原列按352既有mtime／DTA與每輪RAMhash正規化保持，39PNG逐byte保持；兩筆額外診斷除外。實際cap180M、無CPU停止、原418來源前後保持；state副本及終態／大小／UID GID保存，與369比較。沒有新CPU碼，沿369固定EXE全套收據，另做本觀察建置。

沿Go1.24.13 image sha256:1a6d4452c65dea36aac2e2d606b01b4a029ec90cc1ae53890540ce6173ea77ac，600s／2GiB／2CPU／128pids／UID1000／network none，原ZIP與patch唯讀，owned背景監測有界且trap清理。既有workplace/new-game-370-run.sh與new-game-370-verify.py為私有入口；公開本規格同次掛000-index，收據連主庫docs/re/dosgolem-moo2-intake-20260930.md，不公開原LOG／PNG／RAM／state。

主庫玩法RE-first保持，正式名稱writer／旗色持久語意／讀檔／完整開局／RNG／remake同狀態未知。下一步依真實候選與標籤、CPU／FPU／target／clock／IRQ審查獨立正常ACCEPT契約，另經READY，不直接套舊ruler或banner guard。

## READY審查

new-game-370-ready-review.py直接核對369原170M／180M完整表、指標／矩形、IF1、callback12／12及IRQ非活動非failed，限定只讀兩取樣通過。候選及index1窗口bytes仍未知，READY只授權取樣，沒有ACCEPT輸入或正式玩法變更。

## CONFORMED限定結果

狀態：**CONFORMED，限定母星候選來源與兩正常時點只讀觀察**。未送母星ACCEPT，正常確認及正式名稱writer仍未知。

已證實170M／180M原index2+24指向DS188:28439D，32byte為Sol加NUL及補零，SHA-256 6919c6ec1f4751149ac2653bf5decc4881f64d5b997fb2cac85cf8f4c175f30d。兩時點候選、165byte表與16byteheader保持；候選是輸入緩衝區，不由它推定持久星名欄位。170M原PNG親看Enter Home Star Name／Sol／ACCEPT，SHA-256 410c764d6bc1e928af3100cae79431bc03e152d83330ebd94b29c6fa0523c862，RGB677ff4d0508dd2470b05ab122639815b8382cdc8f2146f83f0e821536c3ad16e。index1原矩形227,246–324,273對應畫面ACCEPT為強推論，尚需正常點擊consumer驗證。

index1+24的DS188:261AC2原16byte窗口首byte00，並含BUFFER0，SHA-256 2c9d54f6d98794161cf52a7d394dc84cca138e8c6793f82b56577d7e8c7c7699。它不是直接ACCEPT字串；診斷label_*只供導覽，標籤語意未知，不當欄位證據。維持原指標與bytes，不深挖renderer或helper來猜按鈕物件。

原170M：EIP235948、R=[0 18 0 0 2BD3C4 2BD3F8 2A206E 34F062]、段=[8 188 188 0 20 188]、flags293h／IF1，FPU127F／status0／depth0／八stack bits0，虛擬386324835µs。VBE bank9／startY512／sets2793／writes51487286／display93；callback target8:2136D1、mask2B／pending0／inactive／12／12，IRQ44492／44492、inactive／notfailed。完整核心／code16／SS:ESP stack16／RGB／表與所有可讀性已取，observer前後RAM及callback／IRQ保持。

原180M仍EIP235AA3，完整核心及終圖逐值／逐byte等於369，callback12／12／IRQ47499／47499完成。globals只有26C4C6由02→01，完整192byte SHA-256在170M為3e2f27c3b1dfa465b3f915b3a6a14427ebd8eba8ecc18023d89513014896c3b1、180M為69a0c2f011924ac7d4f6960ddb98df646f158d49b48f0f43ed72c92cfa0956b9。該byte語意未知；兩時點各與369相同原時點逐值保持。初版驗證誤要求跨時點globals相同，在全14282共通列及39PNG已保持後於該斷言失敗；按原兩時點差異改為各自核對，不重跑guest。

驗證：369全14282共通原列按既有mtime／DTA與每輪RAMhash正規化保持，39frames及final PNG逐byte保持；僅新增兩筆home_name_source_snapshot。兩筆所有核心／FPU／clock／VBE／表／callback／IRQ各對接369原時點；三私有patch逆轉為369，全部公開internal／probe保持eca6a80。原guest一次，真正step_limit180000000／EIP235AA3／unique_sites53798，無CPU停止／step_error／dos_exit。四CLI拒絕及合法缺EXE正對照保持，原418來源前後保持。state最終副本／bytes／SHA-256／UID GID與369一致，監測及原guest均terminal，容器清理完成。沒有新CPU行為，不重跑無關全套，沿369固定EXE的CPU與machine收據。

## 跨規格回填與下一步

| 不可變鍵 | 語意與等級 | 舊規格 | 回填 |
|---|---|---|---|
| 官方1.31／dosgolem_high_le／DS188:298848 index2+24→28439D／170M及180M | 同正常命名緩衝區Sol，已證實；持久writer未知 | 369 | 本370補候選bytes與170M完整ready，未送確認 |
| 同映像／DS188:298848 index1+24→261AC2 | 原始窗口已證實；直接ACCEPT標籤說法否定，物件語意未知 | 本規格 | label_*只供導覽，不以它預定點擊結果 |

369同次追加回填，私有驗證依原候選指標及記錄offset核對，缺項即失敗。公開本規格／索引／369回填，19份私有來源與收據雜湊掛主庫研究入口，原LOG／PNG／RAM／state不進Git。主庫玩法RE-first保持。

實際Docker入口：
```text
python3 workplace/new-game-370-ready-review.py
bash workplace/new-game-370-run.sh
python3 workplace/new-game-370-verify.py
```

下一步依已證實170M原核心、完整165byte表／候選／RGB與target／clock／IRQ，審查母星預設Sol的正常ACCEPT press與受控release。275,260與276,260唯一落index1；正常裝置x550／552、y260沿既有x÷2映射。consumer及共享store是否命中待實測，不代寫核心或RAM。維持180M及既有輸入，正式名稱／旗色持久語意、正式讀檔、完整開局、seed與remake同狀態仍未驗。
