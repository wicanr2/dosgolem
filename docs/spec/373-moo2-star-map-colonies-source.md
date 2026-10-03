# 373：正常星圖COLONIES控制的來源前置

狀態：**CONFORMED，限定只讀來源前置**
日期：2026-10-04

## 原來源與範圍

[372](372-cpu386-imul-byte-source.md)正常母星確認後已達180M，官方1.31 ORION2.EXE SHA-256 4e11be14217b4aafa1839f333bf5eba037f98b0c44e9e4752c96c464c260419f，工具e57e9e063b1713b087423a78bef1349237c3d4b0，CPU 1d8a4d8252372c97d8e873ba13c3ab3670796527dcd6d74de52e8cbf226e068c，位址空間dosgolem_high_le。原180M／EIP2176C5、R=[35017C 70 39C17C 70 2BD478 2BD4C0 39C17C 35017C]、段=[8 188 188 0 20 188]、flags202h／IF1、FPU127F／status0／depth0／八stack bits0，虛擬414027099µs；callback14／14、IRQ47425／47425完成且inactive。

原完整23物件表DS188:298848／stride55／1265bytes，SHA-256 4392f446efbdd96acafbfba8ee39cc0e8ac67a2388119df5bfbfef14a89ab15a。原圖正常星圖、底部COLONIES已親看，PNG beb773bf0623f56bc9b2be697c6e0e472abebd8fa4c319f15e6074291bb7a832。以原前8byte四signed word矩形核算，邏輯點43,450只命中index10矩形17,434–79,471，與COLONIES對應為強推論，正常consumer未驗。不送COLONIES輸入，不改cap、state、公開CPU／DOS／probe或主庫玩法。

## DRAFT只讀契約

僅在相同正常guest的180M既有checkpoint／dumpSetupTable後取一次快照，限定pointer298848、count23、bias0及完整表hash；首次不匹配即拒絕，不挑選時點。完整R／段／flags／FPU bits、code16／SS:ESP stack16、VBE／RGB、clock、callback target／完整state與IRQ及全RAM前後hash保存，沿activationPeek保護原狀態。

23物件中的index1–5各取原+24指標的32byte窗口，不當正式label型別；index10取+24→261716的32byte、+32→2801A9的16byte、+44→3ED894的16byte，原指標與bytes均保留。+40實際為0，不能誤用作圖片位址；+44窗口用途仍未知，不解碼renderer或猜正式欄位。只對固定非零且原可讀窗口取樣，總八窗口、224bytes有界，不掃記憶體。取樣不寫核心、RAM或裝置輸入，不呼叫INT33服務以觀察。

## READY與驗收

審查直接核對372原表／唯一命中矩形、完整核心／FPU／clock／cap、原終PNG與工具CPU hash，DRAFT轉READY後才加私有observer。function及單一cap call剝除後逐byte等於372。原372全部共通原列依既有mtime／DTA／每輪RAMhash正規化保持，39PNG及final逐byte保持；僅新增一筆star_map_source_snapshot。全公開internal／probe保持e57e9e0，沒有新CPU行為，沿372固定官方EXE全套收據，不重跑無關測試。

只跑一次相同正常guest至180M，所有舊輸入／CLI六拒絕二正對照、1996日期與可寫state保持。原418檔RO前後hash、SAVE10.GAM／MOX.SET／sound.lbx終態與同guest有界副本、UID GID核對。日期不是RNG seed。原畫面／來源→唯一矩形／原窗口／callback前置為本輪鏈；COLONIES正常點擊、正式存讀語意及remake同狀態仍未知。

## 入口與權利

Go1.24.13 image sha256:1a6d4452c65dea36aac2e2d606b01b4a029ec90cc1ae53890540ce6173ea77ac；Docker network none／600s／2GiB／2CPU／128pids／UID1000，原ZIP／patch唯讀，有界owned監測550s與trap收尾。沿workplace/new-game-373-ready-review.py、new-game-373-run.sh、new-game-373-verify.py；公開本規格與000-index及372回填，原probe／LOG／PNG／RAM／state不入Git，收據掛主庫既有docs/re/dosgolem-moo2-intake-20260930.md。

下一步以實際窗口及完整callback前置建立正常COLONIES一次press／pressed查詢／首安全release與原consumer契約，另經READY。不套用母星命名165byteguard，不代寫原選取或typed語意；主庫玩法RE-first保持。

## READY審查

原372完整23表、唯一index10矩形、+24／+32／+40／+44原值及完整核心／FPU／180M／終PNG／CPU hash直接審查通過。八窗口實際為六個32byte與兩個16byte，共224bytes，修正計數後轉READY。只讀窗口及callback target尚未原native，不送新輸入。

## CONFORMED限定結果

狀態：**CONFORMED，限定180M星圖來源與正常輸入前置只讀**。COLONIES未點擊，畫面對應仍是強推論。

原180M只有一筆star_map_source_snapshot，全部八窗口224bytes可讀，完整R／六段／flags202h／FPU bits、VBE／RGB及虛擬414027099µs與372對接，原code16／SS:ESP stack16已取。target8:2136D1、mask2B、pending0／inactive、callback14／14與IRQ47425／47425完成且非failed，IF1。快照前後核心／FPU／clock／VBE／callback／IRQ／全RAM保持，valid及readonly true，colonies_press_sent=false。

原表DS188:298848 count23／stride55及完整1265bytes保持372。邏輯43,450只命中index10、矩形17,434–79,471，和畫面COLONIES的對應仍待正常pressed查詢及選取consumer。index10+24的DS188:261716首byte00，32byte SHA-256 487a51bcf042e9ed41e13586fbf0e823b3fa9bc77dae18296d9d34c95739de53；+32的DS188:2801A9共16byte全0，SHA-256 374708fff7719dd5979ec875d56cd2286f6d3cf7ec317a3b25632aab28ec37bb；+44的DS188:3ED894原16byte SHA-256 c36adafdfcf829c80935633c82a6fe2bf9d24469ce4ae188bc6881dfe21dade2。這些原offset與bytes保持，正式欄位型別未知，不將+44命名為圖片、不從+24猜按鈕標籤。

| 原物件／+24 offset | 首個NUL前原bytes所讀文字 | 32byte SHA-256 |
|---|---|---|
| index1／26A104 | EINSTEIN | be26e63162efb08ed239860cc7ce5a62faf2bb746da8adf397f21f681dd9cd28 |
| index2／26A10D | MOOLA | 57bdf0c978734f78a5bfd2c953ebb623fd7cad1a22dc1eb363b7cdc02639942a |
| index3／26A113 | MENLO | 7be9b04ad2b4a0e02c99b552321865d9e8c6417aa318b31237e70688ec668e57 |
| index4／26A119 | ISEEALL | 57f59bad4d6929358fdb24e7fadc34ab115d8c3b33d3d6e723d5a828b210756a |
| index5／26A12B | SCORE | 47356d6868e57aeadca7cbab9bf847a18b7cb6edd1eccf472f8046344302a07e |

以上僅原字串已證實，物件用途不推定。index1–5的矩形全為-1，沒有把它們當底部COLONIES熱區。沒有再追字串handler、renderer或runtime helper。

372全部12047共通原列按既有mtime／DTA／每輪RAMhash正規化保持，39PNG及final逐byte保持，僅新增一筆只讀snapshot。IMUL ram_effect保留 changed_bytes 與before／after hash相等關係，再正規化每輪雜湊，A2唯一byte效果不被抹去。兩私有區段逆轉後精確等於372，所有公開internal／CPU／DOS／probe保持e57e9e0；沿372固定官方EXE全套，不重跑無關CPU測試。六CLI拒絕與合法mode on／off缺EXE兩正對照保持。

原guest一次，cap180M／EIP2176C5／unique_sites54239，無guest_cpu_stop／step_error／dos_exit。所有正常輸入／日期保持，原418來源前後SHA-256保持。state終態與372一致，SAVE10.GAM208000bytes／0cf71fc5368861758da5a5dffbd791f8b711a48cfd295572eb9a21d27dea0f4d、MOX.SET553bytes／de8554b3284c9f9065efe6e4f2dfa71ab0c7e732e75eaea0d0ecf465ab86409f、sound.lbx4250888bytes；有界同guest副本與終態一致，UID／GID1000，相關容器清理。私有驗證首次通過，沒有重啟或挑結果。

## 回填與下一步

| 不可變鍵 | 語意與等級 | 受影響規格 | 回填 |
|---|---|---|---|
| 官方1.31／DS188:298848／count23／index10／180M | 原三個窗口及callback前置已證實，畫面COLONIES對應強推論 | 372 | 原星圖控制未知分解為已取來源與未送正常輸入，保留舊native |

372同次追加回填，索引與原輸入／source沿前述入口；私有驗證檢查舊正文保持與新引用。其它點擊上下文不由新窗口外推。

下一步建立獨立COLONIES正常一次press、原AX3查詢、首次安全release與原選取consumer契約。邏輯點43,450以既有2:1橫向裝置尺度對應physical x86,y450，release仍在index10內；預定點仍須READY審查，不直接寫原選取word。新輸入需要有界後續觀察窗口，另明示預算與模式，不修改本373的180M收據或加cap挑結果。主庫玩法RE-first保持，正式存讀語意／完整開局／RNG與remake同狀態未驗。
