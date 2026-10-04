# 396：原殖民地 RETURN 正常輸入

狀態：**DRAFT，即時放開候選已拒絕；正常按下及selector真返回已驗**
日期：2026-10-04

接續[395原RETURN與選單來源](395-moo2-player-return-options-source.md)。本契約只授權私有原版觀察器：原正常選單至人口配置後，按右下RETURN並驗證原自然返回；主庫玩法與公開CPU／DOS保持，RE-first未開。

## 固定前置與證據

官方1.31 ORION2.EXE SHA-256 `4e11be14217b4aafa1839f333bf5eba037f98b0c44e9e4752c96c464c260419f`、Go1.24.13及既有IDA9.4。IDA linear EA與runtime+F0000h投影分開。393 private Go `dbff2283b2e1332f46a01f53135f3acb2de66ad2bded9a5e50b8af8e2f14bacb`；place-terminal `a92979e8b9b9badf8e8b95cbb3d472d18726eca3442c116f05a28306b0664ef0`。

已證實：393原212590909／1B0845、36表／bias0、RETURN index18／kind0、556,459..628,478、裝置buttons0、callback20／20及安全IRQ。395原BF0CD存word_17AAD1，C0897比較及C0960→BAD99..BAD9F自然清理／RET；當次raw control值需新收據核對。強推論：一次正常裝置1180,468 press可命中RETURN18；能否依原selector真返回、安全放開及自然RETURN閉合，由實際結果驗收。

## 私有模式與轉移

COLONY_RETURN只接受值1，依賴完整POP_PLACE_RELEASE及其所有既有模式、原MAX_STEPS185M與非空state；讀EXE前拒絕缺項／錯cap。mode off保持393。唯一原Step不變，不插指令、不跳PC、不直接派送ID、不寫RAM。

先重播至393原212590909，保存完整15phase／50writer／terminal結果，與固定393原結果比對，只忽略每次RAM雜湊鍵。通過才凍結舊collector，保持原215M上限，啟用RETURN觀察。不得以縮減前置或只比較PNG取代完整原結果。

press前再驗當次1B0845／DS188、原控件表／RETURN first-match18、kind0、word_17AAD1 raw18、C9有效殖民地、C8普通模式、八槽及0／6／2啟用職務、buttons0／callback20／安全IF與IRQ。正常InjectMouseEvent(1180,468,1,0,0)，裝置注入前後RAM／core相同。

以原203FB9當次真SS槽讀取返回20E1AC，ESP+4及相同SS，EAX18、新GUI590,468、完成的新callback、至少20ms及安全IF／IRQ作press消費條件；成立才一次正常release1180,468,0。未成立時維持held至215M，不補寫原cached值或反覆press。

觀察原released-zero、active清除、C08CA及C0960，於BAD9F讀當次真SS near槽，只有已到C0960才接受該共享RET。依原caller限制返回1004EF或1EE0E3，ESP+4且SS相同才證實原C058A返回。後續第一個2071AB正常輸入入口另存frame與其真SS caller；若未達就215M終止。不得先稱其為星圖、COLONIES列表或存讀完成。

每個只讀phase保存core／frame／GUI cache／原四範圍及新增26AAC8的16-byte原flags；新phase最多24，增量保存。被動stack不可讀沿既有unknown0；真正near槽至少4-byte，不能以padding當真值。

## 驗證與垂直鏈

來源審查先於生成private Go；patch可精確反轉回固定393原Go。CLI沿142筆完整前置加RETURN依賴拒絕與mode off正對照。檢查唯一原Step、僅新正常press／release兩個呼叫、既有readonly getter不改、完整393結果守衛先於新輸入、所有舊收據與公開CPU／DOS保持。

一次正常原guest後獨立核對完整393前置、原按下／放開、trueSS返回／flags／caller、每phase PNG與readonly、原SAVE10／MOX副本及418原輸入保持。不是remake對拍；若未返回或環境／觀察器拒絕，保留產物與原STOP，不以改cap求通過。

沿既有Go image，900s／2GiB／2CPU／128pids、state850s、owned PID trap、network none與UID/GID1000。原ZIP／patch唯讀，暫存game與state在容器tmp；新private Go／EXE／JSON／PNG／LOG只留本機忽略。重生入口workplace/new-game-396-generator.py、new-game-396-run.sh與new-game-396-verify.py。

正常存讀仍未知；原RETURN若返回COLONIES列表，下一步先驗列表RETURN再進options，不靠direct-entry。raw mode／typed名稱、跨殖民地、完整開局／亂數及remake同狀態不在本輪完成聲明。

READY來源審查：固定393完整末態／Go、395原436列／4與43-entry跳表、九個RETURN18邊界及type0來源通過；raw flags及實際正常返回由新原收據驗，不預填結果。

### 首次重播的環境拒絕與修正

首次原guest已到212590909，完整15phase／50writer的固定393結果在外部核對相同，尚未送RETURN。overlay_probe_exit=137；原程式沒有guest_cpu_stop、step_error或RETURN事件。兩份各48036591-byte terminal於新增守衛同時展開並複製成通用JSON物件，增加觀察器記憶體。此退出不能當作CPU缺陷或RETURN失敗，失敗產物與雜湊保留於workplace/failed1-396-manifest.json。

READY修正只把新增完整393守衛改成逐token串流SHA-256，檢查原來源原始檔SHA，僅略過ram_sha256／ram_before_sha256／ram_after_sha256三種鍵，其餘鍵與型別、值、結構順序完整比對。來源Go及既有getter不改，兩份原JSON由json.Marshal排序鍵，另驗串流結果等價於既有正規化及對非雜湊突變會拒絕。保持900s／2GiB與215M；同一容器入口乾淨重跑，不提高上限。

### 原結果與停止線

串流修正後session31572 exit0，完整393結果通過，212590909正常RETURN按下；212596669原selector入口，212597711依真SS返回20E1AC／EAX18，GUI590,468、callback21／21及安全IRQ。當時原虛擬時間489262873，距press489255516僅7357µs，低於既定20ms，故即時放開候選拒絕。原215M維持held，沒有正常release、C058A返回或下一輸入點完成，不歸為原CPU缺陷。

六phase／PNG、原四範圍與flags、418原輸入及SAVE10／MOX副本保持通過。獨立驗證初稿把實際place-return-事件鍵誤寫return-；修正純讀取腳本後核對相同產物，沒有再跑guest。396完整RETURN仍DRAFT。後續[397](397-moo2-colony-return-deferred-release.md)只驗已辨識後第一個符合原安全／20ms條件的正常放開，不能從本篇推定自然返回完成。

### 397正常RETURN補證

[397正常RETURN](397-moo2-colony-return-deferred-release.md)保持本篇原前五phase，第一次符合安全及20ms條件才正常放開；原真SS返回0x1004ef已驗；畫面切換與下一輸入未驗。本篇即時放開候選仍DRAFT，不以後續候選成功改寫原held結果。正常存讀及remake同狀態仍未知。
