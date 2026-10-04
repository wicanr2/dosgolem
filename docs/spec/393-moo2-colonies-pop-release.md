# 393：以原 kind7 selector 真返回安全放開

狀態：**CONFORMED，限定正常原版放開／配置分支與 raw 變更**
日期：2026-10-04

接續[392正常按下與拒絕的getter候選](392-moo2-colonies-pop-place-input.md)。392原第三次215M無CPU stop，正常press與selector真返回已驗；候選getter至20DB8C未觀測，未release，22個變更只屬held窗口。393以當次selector真返回、新GUI及完成的裝置回呼作為press消費證據，送一次正常release，不猜job數值或直接寫RAM。

官方1.31 ORION2.EXE SHA-256 4e11be14217b4aafa1839f333bf5eba037f98b0c44e9e4752c96c464c260419f；沿Go1.24.13，固定日期不是seed。IDA9.4 linear EA與LE file offset／fixup分開，dosgolem_high_le程式及資料投影加F0000h。公開CPU／DOS及主庫玩法保持，RE-first未開。

## 不可變前置與證據等級

已證實：392 private Go 6edc3987af0e5210c26cf3790848be5f8173a29d59e6b1b38b06f46b212afa40；place-terminal 028fc6f967f968ccfc0079109b38de140760077c237d23a112abe8bf288cd197；place-events 83ab1eb5a1ef2cbeb044bce833c3b0fb5122e3c11805171e0fd9d553897bf3b4；result 96773422729722a14ea751e7d38b97f9511315bf6376ebfa570d2b2a52bd3a48。先保持完整390至210M及392前4個phase，僅移除每次RAM雜湊鍵，不替換PNG或原core。

392在210272551按下660,107，210282004原selector203FB9入口，210282377依當次真SS返回20E1AC，ESP+4、EAX2、GUI330,107、callback20／20；virtual483552903距press483511206為41,697µs。當次mask1、pending0／inactive、IRQ56170／56170、IF有效、裝置buttons1，原record／選取狀態保持。kind7略過kind6 held場景與release回呼由391來源證實。

強推論：此真返回足以證實新press已用於第二列first-match，且已超過20ms。當次安全裝置release後是否到BF6ED→B9E94、原record／UI怎樣變化仍未知；依實際結果驗收，不以getter等待或完整骰序為閘門。

## 私有模式、轉移、邊界

POP_PLACE_RELEASE只接受值1，依賴POP_PLACE及完整392前置，讀EXE前拒絕缺項／錯cap／空state。關閉模式保持392；維持原210M凍結才延伸最多215M。新mode只改press-consumed及release候選，callback4KiB尾端與MOV SS／MOV ESP的被動unknown、strict near槽4-byte及原390 getter保持。

當次selector真返回要先比對392完整前4個phase，包含原PNG、新座標、回呼與四範圍。符合才新增press-consumed phase；同一個安全返回點正常InjectMouseEvent(660,107,0,0,0)，核對前後guest RAM／core／raw相同。禁用舊getter作新mode的消費閘門，calls仍只是啟動計數。

觀察released-zero／active-clear、BF6ED與B9E94當次真SS返回1AF6F2；原配置返回後首個1B0845可提前停止，否則215M。每個原Step最多128 writer、32 phase、16個不同EIP的unknown stack樣本；增量保存writer。禁止猜填正式job欄位、直接dispatch／RAM注入、盲增指令cap或深挖renderer／平台helper。

## 驗證、垂直鏈與權利

READY先於私有實作：固定392原結果／前4phase及390凍結、391 type7來源、strict near／readonly測試已錨定。之後生成可逆Go及來源／CLI護欄，沿原正常選單至人口輸入重播。獨立核對完整前置、兩次正常裝置輸入、當次近返回、原Step變更重建、PNG、source bytes與實際SAVE10／MOX副本。原save資料來源不等於已驗正常存讀；正式職務欄位、跨殖民地、完整開局、亂數及remake同狀態仍未知。

Docker原ZIP／patch唯讀、UID/GID1000、network none、900s／2GiB／2CPU／128pids，state850s及owned PID trap。重生入口workplace/new-game-393-generator.py、new-game-393-run.sh、new-game-393-source-verify.py及new-game-393-verify.py；新writer缺來源才用既有窄IDA入口，原LE／bytes獨立核對後停止。

原EXE／JSON／LOG／PNG／RAM／state／IDA及private Go只留本機忽略；公開僅自撰規格、索引及證據回填。正常原輸入與正式配置聲明限於實際通過的範圍。

READY來源審查：392完整原前4phase、selector真SS／SP+4／EAX2／callback20／新GUI、安全IRQ及41,697µs均核對；舊getter候選拒絕與raw held結果保持，沒有先寫新Go。

## CONFORMED 原版結果

完整390凍結與392前4phase保持。210272551正常press，210282377以selector真SS返回、EAX2、新GUI330,107與完成callback20安全release，兩側裝置輸入均不直接改guest RAM。210283103到released-zero，210283112／113清active，跳過kind6 release回呼。210285765到BF6ED，210285766到B9E94，210305813依當次真SS／ESP+4返回BF6F2。原212590909回到下一個1B0845，提前於215M停止。

15個phase／50個實際Step變更均只讀，完整重建四窗口末值，65個新frame及PNG雜湊逐項通過。原record13個首末差異：+08h 00→06、+0Bh 02→01、四槽+0Ch／10h／14h／18h 00→80及+0Dh／11h／15h／19h 00→02，另+E9h 06→0C、+EDh 0F→12、+F9h F4→F1。+0Ah原08保持；不把這些raw欄位直接稱人口總數或正式job enum。17AABB回0、17A974回FFFF，正常控件表回36列。

五個新writer以IDA9.4各八個鄰近指令補證，83列／53EA／8筆LE重定位差異，原MZ／LE／2object／365page／51363fixup records獨立核對；所有50個runtime writer的原bytes及+F0000h投影通過。sub_BA5DA只保存實際BA6E8／BA6EF寫入鄰近上下文，沒有翻譯整個函式或深入renderer／平台helper。IDA殼層exit0、idat_exit1，非空JSON、原檔hash、schema及獨立原bytes均通過；不以殼層exit覆蓋內層狀態。

固定日期不是seed；原418輸入與實際SAVE10／MOX副本保持，沒有正常存讀或跨殖民地實測。主庫RE-first保持，正式職務位元語意及remake同狀態未知。正常放開／配置分支已驗，不外推整款遊戲。


## 394 職務位元與產出讀取端回填

見[394人口槽位職務與產出讀取端](394-moo2-pop-slot-consumers.md)。官方1.31原BA6DF／BA6E5／BA6E8寫第7、8位，BA6EF設第9位；DE393依第9位篩選，DE39F／DE3A6抽出職務，三caller送0／1／2。不可變390／393正常收據證實八槽保持、四個原農夫改派工人，產出分布由4／2／2經選取暫停的0／2／2到0／6／2。較早正文的正式job未知只屬當時證據邊界；394補證限定此次同殖民地路徑。job3／跨殖民地／正常存讀與remake同狀態仍未驗，不改原receipt或歷史正文。

### 397正常RETURN補證

[397正常RETURN](397-moo2-colony-return-deferred-release.md)完整保持本篇15phase／50writer及212590909配置末態，再正常按下右下RETURN18；396的7357µs即時放開候選被拒絕，397首次安全20ms後放開，原trueSS返回0x1004ef已驗；2071AB及畫面切換未驗。本篇人口配置結果不變，正常存讀與remake同狀態未驗。
