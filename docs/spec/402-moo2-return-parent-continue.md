# 402：保留原215M前置後的有界父層輸入續跑

狀態：**CONFORMED，限定原完整前置後的正常父層輸入與畫面返回**
日期：2026-10-04

接續[401原父入口與自然返回](401-moo2-return-parent-trace.md)及[400原父層輸入來源](400-moo2-return-parent-source.md)。原1.31 ORION2.EXE SHA-256 4e11be14217b4aafa1839f333bf5eba037f98b0c44e9e4752c96c464c260419f。401 Go SHA-256 1593aa58a873d8c2b3a2d5b961d7115e449f9b1b2bb97deab8546b76376763fa；401 parent-terminal SHA-256 5b340590cc6a2d3d0d11125ae770db4dc6bd5a1aa71d1d8a68d00ee8f6033f99。工具Go1.24.13／IDA9.4／Python3.11，原EA為IDA linear EA，runtime=EA+F0000h、原file offset及LE fixup分列。

## 證據與範圍

已證實：原分派20自然CALL C4562；215M內B4EF6已12次entry／10次自然RET1Ch，末態仍有兩層active frame。原C472A CALL1171AB，下一C472F為返回位置，來源逐原EXE核對。原215M是觀察截點，不能從該點推定初始化不會返回。

只授權原正常路徑的只讀續跑，不改產品規則、Go玩法或任何原版輸入。以401固定原215M完整前置為比較門：到215M原CPU狀態時，依原順序凍結return／mode／parent收據，完整401 parent與完整399／397逐欄位保持，僅略既定三個RAM雜湊鍵。前置不符即拒絕，不進入延長窗口。

續跑有界上限230M，增加15M觀察窗口；在原C472A CALL後實際進入1171AB即提前停止。若未到達，保留實際末態與未知，不繼續延長本契約。固定前置後只停止舊RETURN／mode／parent只讀監看，原CPU.Step／計時／裝置與guest RAM保持；不存在新增press／release或mode代寫。mode off為精確401。

## 觀察與失敗模式

COLONY_RETURN_PARENT_CONTINUE僅接受1，要求原PARENT_TRACE／MODE_TRACE及完整RETURN_DEFERRED依賴、MAX_STEPS185M與state；錯值在讀EXE前拒絕。初始cap與原所有前置不改，只在215M最後原Step成功後準備續跑，下一iteration先凍結並核對完整前置。凍結前不取新續跑phase，避免增加原B5051計數。

固定前置後捕捉runtime1B45C1／1B472A／2071AB。每phase沿既有core／frame／四raw範圍／mode七byte／DAC／device及PNG，前後原狀態保持，新phase最多8。C472A保存原SP／SS／CS；1171AB entry須由唯一原CALL Step到達，SP減4、同CS／SS、真返回槽1B472F，才稱正常父層輸入已到。

CPU stop／DOS exit保存實際點；原碼出現未支援指令時，以其原bytes與最小同狀態測試另開CPU修正，不猜補規則。正常輸入entry、畫面切換、控件表及原campaign/save狀態分開驗證；原日期不是seed，未驗事項保持未知。

## 重生與驗收

本機忽略入口workplace/new-game-402-generator.py、new-game-402-run.sh、new-game-402-source-verify.py、new-game-402-verify.py。Go patch精確反轉固定401，改名限明確檿名／產物前綴，不更換數字或原getter。195CLI保留185前綴，再做拒絕／正對照；實際建置Go input與輸出名稱需檢查401原產物不覆寫。

一次原guest後獨立從固定EXE／原LE重定位重建每個實際code window，核對完整401／399／397凍結、真CALL與source bytes、所有只讀phase／PNG及原418檔。早停收據記錄實際步數與230M上限，SAVE10／MOX副本保持。畫面只依實際原PNG判讀；不得用正常input到達冒稱完整remake同狀態。

沿既有image、UID/GID1000、network none、原ZIP／patch唯讀；原guest900s／state850s／2GiB／2CPU／128pids及owned PID trap。原EXE／JSON／LOG／PNG／private Go不公開；公開CPU／DOS與主庫RE-first保持。

## 原402正常返回驗證

原guest session23489 exit0，195CLI含161拒絕／34正對照及精確反轉固定401通過。215M完整401 parent／399 mode／397 return收據保持；固定前置後有界續跑，222329888於runtime1B472A原CALL，222329889自然到2071AB並提前停止，少於230M上限。原SP減4、同CS8／SS188、真返回槽1B472F以及原EXE／LE重定位code window逐bytes核對通過。4個新phase全只讀，原418輸入與SAVE10／MOX副本保持；無新增玩家輸入或Go玩法／CPU／DOS修改。

末態VBE StartY0／DisplaySets102，控件表runtime pointer0x298848／count20／stride55。原PNG SHA-256 299868824b0ea14741bbcd781a8aa353c8344ef29bd5997f50f5be6d211de8c2經實際圖像檢視：已回COLONIES列表，Sol II列農夫欄空、6個工人、2個科學家；上方欄名與下方排序／RETURN按鈕可辨讀。這是同一原guest內的畫面返回與正常父層input entry，驗證範圍不涵蓋remake逐像素或同狀態驗收。原215M PNG與舊收據不覆寫。

initializer-returned監看點未取樣，不猜C2259何時返回，也不把B4EF6呼叫鏈補成C53C9靜態caller。已證原C4562進入與C472A／1171AB輸入，403已保存新控件20表的RETURN producer／handler，404已補驗實際ID3、正常點擊與星圖返回；options／存讀仍待後續。正常存讀及remake同狀態未驗，固定日期不是seed，主庫RE-first保持。

本機視覺收據workplace/new-game-402-visual-review.json記錄原PNG hash、實際scope與人工檢視結果；new-game-402-verify.py獨立核對數值／原code／真SS／完整前置，新視覺判讀不偽裝成該程式自動通過。402 source／verifier與native一次通過，沒有新的guest失敗或重跑。

## 20項原控件表的只讀核對

本機入口workplace/new-game-402-table-verify.py使用同一402末態，不重跑guest、不新增裝置輸入。runtime data pointer0x298848、bias0、20×55＝1100原bytes，九個含端點及外側first-hit案例通過。已證實：raw index3、kind0矩形531,445..615,470；GUI590,468對應裝置1180,468，外側first-hit為19。原index18矩形現為378,34..510,64，不沿用395的RETURN18。

強推論：raw3的矩形與實際原PNG右下RETURN一致，是下一正常點擊候選；尚未實際按下，不將幾何解碼當成原選取返回值。400的C4343只保存mode恢復writer與17B0EE／17B0F0來源，403已保存原handler與producer，404已補驗當次UI word、正常press／release與星圖返回；options及存讀另驗。兩個私有table收據由本篇索引。

後續解決回鏈：[403控件綁定來源](403-moo2-list-return-binding-source.md)、[404正常列表RETURN與星圖返回](404-moo2-colonies-list-return-input.md)。本篇原222329889末態與230M上限保持。
