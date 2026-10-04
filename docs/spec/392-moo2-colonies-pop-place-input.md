# 392：選取後第二職業列的一次正常放置輸入

狀態：**DRAFT，按下與selector已驗；放開候選拒絕，限定原版動態觀察**
日期：2026-10-04

## 玩家路徑與來源

接續[391 kind7來源](391-moo2-colonies-pop-place.md)，先由相同正常玩家路徑重現完整390至210M，再在下一個原輸入點送出一次裝置660,107按下及放開。只蒐集原型別、選取、放置分支、原record與原畫面；不修改主庫玩法或公開CPU／DOS，不猜職務名稱及配置結果。

官方1.31 ORION2.EXE SHA-256 4e11be14217b4aafa1839f333bf5eba037f98b0c44e9e4752c96c464c260419f，工具基線b17eb21c212deb1f95cef54f1399d7b35efd8bdf。IDA9.4 linear EA與LE file offset／fixup分開保留，dosgolem_high_le程式與資料投影加F0000h。沿Go1.24.13既有Docker，固定日期1996-01-01不是seed。

390固定pop-terminal為8510fe25f71add56aee28ea6621f3475d40fb68470c343a1ad9d140e4e49f996，pop-writes為fe71dc8a45063a6dd28565b40a25705796ca049e2fbec5f825922482e80431f3，restore-journal為6e26225a7ae3ca08fc8796a71bc89cf56587532dc5565407e32a23a005b626b3，private Go為ec043c8fe60c741f99a6ac8ae6a6db2295b84bd4aa306b95ca829331bbf4fa18。三份完整新收據與固定390比較，僅移除每次RAM雜湊鍵；任何其他差異拒絕延伸。

## 已證實來源與候選輸入

391證實210M整表37列／2035bytes／kind7，SHA-256 d1c13ea6764aab2848addf42cadb7166030d2a5421fd0f8533b9b33773842524。前3列310,60..510,88／310,90..510,118／310,120..510,148，pointer2879DA／DC／DE。裝置660,107經signed SAR1成GUI330,107，當次first-hit2。原current4／pool5B2044／record5B25E8，17AABB=1與17A974=4。

原kind7跳過held／release場景回呼，仍經11E50D清active。原11DB87呼叫123C1B，123C33讀cached word_1B1222，123C47近返回。runtime213C1B→20DB8C須從當次真SS stack取得，不以相同EIP替代真返回。390 cached1與device0並存，MouseReadSnapshot388.calls是啟動服務計數，均不能單獨證明新press被消費。

強推論：新press後原getter返回低AX1、新GUI330,107及至少一個新回呼已完成，足以安排安全release。當次是否到BF6ED→B9E94及哪些raw bytes改變均未知，不能寫入預期職務數或固定結果。

## 型別、狀態、停止線

私有POP_PLACE只接受值1並依賴完整POP_WRITES前置；在讀EXE前拒絕缺少前置、錯cap或空state路徑。關閉模式保持原390。原runSteps維持210M直到最後一次原Step及全部舊監看完成，呼叫finishRestore以原210M／step_limit凍結3份完整收據；一致才延伸至最多215M。不提前改run_limit。

新階段首個1B0845須DS188、固定37表與first-hit2、原record／選取狀態、IF與IRQ／滑鼠回呼安全、裝置buttons0、callback8:2136D1／mask2B／pending0／完成19。符合才正常InjectMouseEvent(660,107,1,0,0)，不得改RAM、指派dispatch ID或直接呼叫玩法函式。

觀察原213C1B當次真SS槽及20DB8C近返回，低AX1、當次新GUI座標、裝置buttons1、回呼至少20已完成才標press-consumed。安全IF／IRQ、pending0／inactive、原回呼target且至少20ms虛擬時間才正常release660,107,0。不等待kind6 held場景，不讀calls增量作閘門。

觀察selector203FB9的真SS返回20E1AC、原released-zero／active-clear、1AF6ED及1A9E94真SS入口／返回1AF6F2。記錄實際EAX、raw參數與資料，不假設positive2或負index。放置返回後第一個1B0845可提前停止；否則最多215M。CPU stop／DOS exit／缺來源／事件上限完整保留並分類，不盲增cap。

新階段用固定descriptor188讀四窗口361／32／32／18bytes與當次core／真SS／原code，記錄最多128個Step變更與32個phase。沿既有原Step，不設新Bus／CPU hook；每個快照核對RAM、裝置、core及DAC前後相同。舊390人口／writer／場景store在凍結後停止收集，避免把新階段塞入舊收據。

## 驗證與交付邊界

READY前核對固定390／391來源、原bytes、37表／九個端點及cached反例；READY後生成可逆private Go、來源護欄、CLI及有界run入口。全程network none／UID/GID1000、原ZIP／patch唯讀；900s／2GiB／2CPU／128pids，捕捉state850s並以owned PID trap收尾。

獨立驗證完整390凍結、新裝置press／release、真SS近返回、只讀phase／writer、原PNG／雜湊及record變更重建。正式job語意、跨殖民地、存讀、完整開局、亂數與remake同狀態仍未知；僅實際完成的正常原輸入與分支可CONFORMED。

原EXE／IDA／JSON／LOG／PNG／RAM／state及private Go只留忽略的workplace；公開僅自撰規格、索引及證據回填。主庫RE-first保持。

READY來源審查：固定390完整收據與private Go、391 kind7／getter來源及BF6ED／B9E94 bytes已錨定。未生成新Go或guest。候選輸入完成條件與未知輸出分開，限有界動態蒐證。

## 原回呼堆疊與第一輪觀察器勘誤

第一輪完整390凍結通過，210272551送出正常660,107 press；210272553進原callback2136D1，SS158／ESPFF8。觀察器要求16-byte stack而拒絕，沒有原CPU缺陷證據，所有產物按failed-392及manifest保存。工具le_mouse_callback.go既有4096-byte私有堆疊，descriptor limitFFF，初始SPFF8及8-byte far-return frame；原callback合法，16-byte peek越界。

修正版只在新392階段使用獨立core getter，依descriptor及RAM邊界讀最多16-byte有效prefix，code至少1-byte，stack至少4-byte；保存CodeBytes／StackBytes，剩餘padding不當原bytes。正常近返回仍要求當次真SS槽及SP+4；讀不到4-byte拒絕。原390 getter及全部凍結收據保持。DRAFT下先做可丟棄的實際getter測試，證實舊拒絕、callback尾端8-byte與4-byte有效prefix、越界拒絕及只讀，才回READY。同容器／同命令乾淨重跑一次，不盲改wall-clock或cap。

修正版READY：抽取實際prefix getter，callback尾端8-byte、near槽4-byte、正常16-byte與descriptor／RAM拒絕及只讀通過；原390 getter不改。第一次guest的175份失敗產物與正常press／callback來源保留。

## 第二輪 MOV SS／MOV ESP 交界

第二輪210280976／277，原callback先執行1237D9的8ED2（MOV SS,DX），下一指令1237DB的89C4（MOV ESP,EAX）尚未執行。此時SS158／ESP2A1E2B暫時不在descriptor內，EAXFD0。兩個原bytes已在387獨立核對；先改SS再改ESP是原相鄰指令，不能要求每個中間Step都有可讀堆疊。第二輪產物按failed2-392及manifest保存，無CPU缺陷證據。

修正版被動core保留原寄存器及CodeBytes／StackBytes，SS不可讀時StackBytes0、padding不冒充原bytes；原Step照常執行。真near槽另以placeNearSlot強制至少4-byte並拒絕不可讀。真返回、輸入安全、record與原code護欄保持。記錄最多16個不同EIP的暫時不可讀stack樣本及增量writer收據。先驗證實際getter的8-byte／4-byte／16-byte、被動unknown0、嚴格near拒絕及只讀，再回READY。

前兩次原重播到210M約需10分鐘，外層執行預算改900s以涵蓋新增5M觀察及建置；guest指令上限仍215M，沒有調整虛擬時序、亂數或重擲。state捕捉同有界850s，沿相同owned PID trap、UID／memory／cpus。

第二次修正版READY：被動unknown0與strict near拒絕、有效8／4／16-byte及只讀通過；387原MOV SS／MOV ESP bytes與實際暫存器反例已錨定。原390／CPU不變，兩次失敗產物保留。

## 215M實際結果與放開候選拒絕

第三次guest正常到215M／step_limit，原390三份完整收據保持，無CPU stop。210272551正常660,107 press，210282004進selector203FB9，210282377依真SS返回20E1AC，EAX2、新GUI330,107、callback20／20，距press41,697µs。原getter至20DB8C的候選未觀測，getter_entry為null；consumed／released／placement_returned均false，不能作正常放置CONFORMED。

5個phase與22個Step變更逐項核對。原A4EB8／A4EC2僅交替將17AAB9寫2／0，record四窗口末值保持，原record無差異；這是實際held窗口，不新增population配置語意。原code／近返回／只讀／PNG及SAVE10／MOX副本保持。正式配置仍未知。

下一步393以已驗selector真返回作候選press消費證據，核對同一步EAX2、新座標及callback20、安全IF／IRQ及至少20ms，再正常release660,107,0。完整392前4個phase及390凍結必須保持，原getter不是此路徑的消費閘門。此候選不猜規則或改RAM；正式放置輸出仍按實測驗收。

## 392／393 正常 kind7 放開回填

不可變鍵：DOS／官方1.31 ORION2.EXE／IDA linear EA BF6ED、B9E94、1237D9及1237DB；EXE SHA-256 4e11be14217b4aafa1839f333bf5eba037f98b0c44e9e4752c96c464c260419f。runtime投影加F0000h。原kind7按下／selector由[392](392-moo2-colonies-pop-place-input.md)補驗；當次getter至20DB8C候選未觀測，不能作消費閘門。

[393](393-moo2-colonies-pop-release.md)以selector真SS返回EAX2、新GUI及callback20安全release，原BF6ED→B9E94與BF6F2真返回、下一正常1B0845及50個raw writer已驗。原MOV SS／MOV ESP中間Step的堆疊不可讀屬被動unknown，真near仍要求至少4-byte合法槽。較早kind6來源及391靜態getter仍有效，不能外推該getter候選到此kind7正常路徑。正式job欄位／跨殖民地／存讀及remake同狀態仍未知。
