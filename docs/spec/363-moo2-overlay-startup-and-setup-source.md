# 363：覆蓋層前段開檔與設定頁五個資料窗口

狀態：**CONFORMED，限定雙側只讀窗口與早期開檔；可寫玩家驗收DRAFT**
日期：2026-10-04

沿[362](362-moo2-save-permission-boundary.md)，公開工具4f9c45be2017904ea42d86ef9b7ae692388eee08，主庫088df139965dfc699b56efb4b577d87eae4e0f77。固定ORION2.EXE SHA-256 4e11be14217b4aafa1839f333bf5eba037f98b0c44e9e4752c96c464c260419f；dosgolem_high_le為原位址基準。原ZIP根層417檔／官方1.31 EXE／MOX.SET、1996-01-01與180M參數及既有正常輸入保持，日期不是seed。

玩家阻塞：SAVE10.GAM的3D01被唯讀provider拒絕已定位，但既有overlay試作在80M設定頁完整935bytes guard拒絕。record11–15的+44四byte窗口各增8000h，其餘表及RGB保持；真正資料與前段原因未知。只讀capture不得據此改guard或送輸入，主庫玩法RE-first保持。

## 有界蒐證

兩側各一次。唯讀側沿公開362；overlay側在容器/tmp從固定361 Git與362已雜湊試作重建，不修改公開CPU／DOS／provider。兩側增加同樣可丟棄診斷：
- 前2M的INT21 AH3D／40最多64筆，保存原step／callsite／mode／路徑／R／段／flags及返回。路徑直接讀原descriptor所指Mem，最多260byte／NUL，完整CPU／FPU／VBE與RAM雜湊驗只讀，Handle只呼叫一次。
- 原80M setup_accept_precondition之後、任何ACCEPT press之前，以原DS descriptor讀取record11–15的+44四byte候選值所指最多128byte。保存原值／線性位址／raw bytes／首NUL位置／可讀性／全R／段／flags／FPU及RAM雜湊／VBE保持。窗口不先命名為文字／圖像／函式。此刻診斷停止，明示probe stop，不稱正常開局或180M完成。
- 原正常先前輸入、表hash／RGB／callback／IRQ guard仍原樣；保存原valid結果，不注入資料或調整點擊。分開核對舊唯讀與舊overlay至80M列及已有PNG，mtime／DTA／每輪診斷RAM雜湊依352既有正規化，不稱跨輪全部RAM相同。
- 原417來源bytes全部SHA-256在guest前後比較；overlay state內容／大小／雜湊留本機，只用來追前段實際操作，不散布原素材或存檔。

證據足夠描述前段檔案差異、80M窗口內容與比較邊界才審查下一個READY工具契約。CPU／原DOS規則與公開provider完全保持，不深入fopen／allocator／renderer helper。原保存與完整開局尚未驗。

## 入口

主要原版執行器/home/anr2/cht/dosgolem隔離副本workplace/dosgolem，能力與使用見README.md／CLAUDE.md。本檔同次加入000-index。私有入口workplace/new-game-363-pair-run.sh／moo2-setup-source-363-readonly.go／moo2-setup-source-363-overlay.go。Docker Go1.24.13 image sha256:1a6d4452c65dea36aac2e2d606b01b4a029ec90cc1ae53890540ce6173ea77ac；600s／2GiB／2CPU／128pids／UID1000／network none；原ZIP／patch唯讀，state只在容器/tmp，輸出在忽略workplace。

## 限定CONFORMED：原早期開檔與五窗口

**已證實**：兩側各一次原guest，皆在80M只讀快照後明示診斷停止，exit0不是180M完成，沒有ACCEPT press／release。唯讀側6148列、overlay側6271列至原setup_accept_precondition依352既有mtime／DTA／每輪診斷RAM雜湊正規化保持各自362舊基線；兩側各27PNG逐byte保持，原935bytes guard仍唯讀true／overlay false。核對腳本原先把27張快照誤寫28，修正為8–23及40、50–80M四張與六張post-click的明示檔名集合後讀同guest收據通過；沒有挑選重跑。

每側八個前2M原AH3D開檔、五個80M窗口及一個全狀態快照，診斷readonly true；early Handle各呼叫一次、返回值保持。三個診斷區塊逆轉為各自362來源，公開internal／CPU／DOS服務／provider／probe完全保持4f9c45b，沒有production實作變更。

**已證實，原最早已捕捉差異**：原1192795在dosgolem_high_le:237024執行INT21 AX3D02，DS188:26C3AF的NUL路徑sound.lbx可讀。兩側原R=[3D02 270E26 26C3AF 43 2BDB34 2BDB9C 26C3AF FFFFFFFF]／段=[8 188 188 0 20 188]／flags202h與呼叫前整RAM SHA-256 d89e8bbe89447eaa4834b9b9a9cf102146b5d77d3d4b901545d71aa3e1f84c34相同。
- 唯讀provider無WriteFileProvider，AX5／CF1／flags203h，是拒絕。
- overlay原OpenWrite(name,true)回真正handle5，CF0／flags202h，其他R／段與此次RAM保持；同AX5不代表同結果。
- 後續fonts.lbx開檔原1526485／handle5，overlay1529227／handle6；orioncd.ini原1612596／handle6，overlay1612675／handle7。前段已不是同一整體執行狀態，不要求其step／IRQ／R全部相同，不能把存檔旗標當只影響165M。
- 此前七筆中的SOUND是唯一讀寫mode2，其他七筆mode0；八筆內沒有AH40寫入。這只涵蓋前2M有界觀察，不宣稱2–80M無寫入或完整音訊行為。

**已證實，80M可讀資料**：表仍pointer298848／count17／stride55，RGB仍3bbe6c339cfbc74e84b3210bccfdc4574073b4d308ef9b90a1720cc5c74e623e。原／overlay full-table SHA-256為2a18a0213dcb1d539de3175c8356b8c8b886c1d61b38f63795b3fa1859a3f52f／49374b4c6dfd2d1d8231cfc137e1b5b7d86c7ec49f6fdf3be7417480d0da948b。五窗口由原DS188 descriptor直接讀，source_linear與raw_value相同、各128byte可讀、首NUL均在offset1；單純NUL切字會截斷此二進位資料。raw值均增8000h，128byte逐byte相同：

| record index | +44 raw原值 | overlay值 | 128byte SHA-256 |
|---|---|---|---|
| 11 | 004AFA3C | 004B7A3C | 341126d6d5f317600d89f387b750f90cbe195c7e83958004090ffde1ec5a7e4f |
| 12 | 004B0D14 | 004B8D14 | e2b48afd82459643c45a8bea94291387186477c5431351035a6ecd5ca428a9f8 |
| 13 | 004AE634 | 004B6634 | 6ab2a1c7f01f7912ef08b9127444b862b751ea944016504f37161c08eff4d8fc |
| 14 | 004B2084 | 004BA084 | 6ce77ceefe8e64b2998a582e73b9551fb70a924c2811bc56c42e87d33b622e0d |
| 15 | 004B3294 | 004BB294 | c4ee738ae1b11e3ed4125bf1d3d653739af8627cc1b81aa9a7e8ef53adfddabc |

只證明候選位址可讀與開頭128bytes相同；完整物件長度／消費端／角色仍未知，不稱已證實標籤文字或圖像指標。畫面與prefix相同不等於整物件逐byte相同。

80M唯讀EIP22F1FB／R=[2BD88C 1 D6 67 2BD8D8 2BD91C 1B0 1F3587]／flags213h，overlayEIP2349D5／R=[0 C 3DC66D 29BE7C 2BD870 2BD8D4 3DC66D 366276]／flags246h。兩側段同[8 188 188 0 20 188]、FPU127F／status0／depth0／八stack bits0。各自快照RAM前後hash與CPU／FPU／VBE保持，不聲稱兩側RAM相同。

**已證實，來源與state**：兩側原418檔guest前後bytes逐檔SHA-256完全保持，417 ZIP根檔另加官方DOS EXE。state只有sound.lbx 4250888bytes／SHA-256 3f0354ac5c1b13a3c5c4fd098c2cbc22af37b71e74fc95e582782c22024c449d／UID及GID1000，與原ZIP SOUND.LBX及本機保留副本完全相同。這是copy-on-write開啟現存sound資料；未有SAVE10.GAM state，不稱存檔成功。

**視覺抽查，2026-10-04**：親看dosgolem原overlay 80M快照，640×480 NEW GAME設定頁可辨；Tutor／Medium／Average／5 Players／Average，Tactical Combat未勾、Random Events及Antaran Attacks勾選。ACCEPT與CANCEL完整，既有ACCEPT熱區433,392–527,414包含480,400。這只驗當前設定頁，未送ACCEPT，不代表已開局。

### 後續契約與未知

原source只讀限制在啟動時就拒絕SOUND3D02，是前段行為差異的已證實入口。額外32KiB配置導致五值搬移為強推論，未追allocator，不稱其內部已解。正式音訊cue／人耳、完整資產讀取與指標消費、RNG、存檔writer／內容、完整開局與remake同狀態仍未驗。

下一步立獨立DRAFT可寫profile的正常ACCEPT輸入：沿既有overlay、同80M／原hotspot與callback條件，以本輪已捕捉的完整overlay935bytes hash及五個窗口／RGB檢查，不從未證實欄位猜規則。原336唯讀guard保持；不能直接忽略指標差異、套任意+8000h或調時刻挑過。來源、表、畫面與完整前置證據審查足夠才READY；一次正常press／release後保存真正選擇store及90M選族表，後續輸入仍須獨立證據，不外推完整存檔。

### 實際命令

```text
bash workplace/new-game-363-pair-run.sh
python3 workplace/new-game-363-pair-verify.py
  6148／6271各自舊列、各27PNG／八開檔及五128byte窗口／原guard保持 PASS
  SOUND3D02同原初態CF差異／原418檔與state來源保持 PASS
```

原版收據readonly bebf7c93f2a802cc1c6241985eafe0b8955cbfc034fa76f39ef679b0f91dfe6e，overlay72f5cc29637de198914526676953f53c99fb333c20f5dafa449d921fe7c11811，state manifest e35322cba18956bcd245e4091932c822f9460271deda20ddda8b7722aa1375bc。原PNG／LOG／128byte資產窗口／sound副本全留忽略workplace，公開只交自製spec／索引／守衛及雜湊。完整私有收據帳掛到MOO2研究入口docs/re/dosgolem-moo2-intake-20260930.md；主庫玩法RE-first保持。

100項規格回填、新363的37＋34缺證據負例及全部較早負例通過。公開守衛入口python3 apps/moo2/tools/startup_probe_131.py --check-overlay-setup-source-spec-backlinks。只驗雙側只讀證據，不把可寫profile或完整存檔升為完成。

## 364可寫ACCEPT限定驗收回填

可寫profile正常ACCEPT與90M選族頁已由[364](364-moo2-overlay-setup-accept.md)驗證。原336唯讀guard保持，獨立profile核對原overlay完整935byte表／五窗口後一次press／release；真正20DDDB選擇store word0000→0F00，原SELECT RACE畫面與既有原圖相同。原418來源檔保持，state仍僅sound.lbx，不稱存檔成功。下一步依新90M完整880bytes表另立Humans輸入規格，完整可寫玩家路徑／RNG／remake同狀態仍未驗。
