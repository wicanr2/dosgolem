# 375：185M新物件表與原色彩狀態只讀

狀態：**CONFORMED，限定185M完整表與色彩來源只讀**
日期：2026-10-04

## 原阻塞與範圍

[374](374-moo2-star-map-colonies-click.md)正常COLONIES座標press／poll／release已使原20DDDB shared word寫0→10，固定185M無CPU拒絕，終圖全黑；RGB921600bytes全0、indexed307200bytes非全0，header count20／stride55／pointer298848。官方1.31 ORION2.EXE SHA-256 4e11be14217b4aafa1839f333bf5eba037f98b0c44e9e4752c96c464c260419f，工具20fd1507c45a053860b0ad50ad9f9f86049566ad，CPU保持1d8a4d8252372c97d8e873ba13c3ab3670796527dcd6d74de52e8cbf226e068c，位址空間dosgolem_high_le。

原185M／EIP223A71、R=[0 0 0 0 2BD720 2BD980 0 2BDC2C]、段=[8 188 188 0 20 188]、flags206h／IF1、FPU127F／status0／depth0／八stack bits0，虛擬424485517µs，callback16／16與IRQ48857／48857完成且inactive／非failed。VBE bank4／startY0／sets2941／writes55575758／display94。黑畫面原因未知，殖民地列表未驗。

本輪保持同185M、全部正常輸入與state，只補目前20物件完整1100byte表、當前code16／SS:ESP stack16、索引使用集合／RGB對應與DAC／像素遮罩只讀值。不增加cap、不送新輸入，不深入renderer／driver／PIT或硬體逐週期。公開CPU／DOS／probe與主庫玩法保持。

## DRAFT取樣與既有API

既有dumpSetupTable限制count≤64，只在COLONIES私有模式固定185M加一次取樣例外，保留舊count≤16 observer與其它checkpoint條件。原globals192／header16與1100byte新表由原DS188窗口取得；固定pointer298848、count20、bias0，第一次不匹配即拒絕，不從舊menuFrame推定目前frame。

私有cap observer只執行一次，沿activationPeek保存完整R／段／flags／FPU bits、code16與目前SS188:ESP2BD720 stack16、clock／VBE／callback target／完整state及IRQ／全RAM前後hash。VBEIndexed()取得固定可見頁307200bytes，VBERGB()取得921600bytes，建立256bin計數與每個用到索引的RGB樣本，逐pixel驗RGB與索引映射一致。映射只作工具觀察，不稱原硬體時序一致。

internal/machine/moo2_vbe_video.go:VBERGB()直接讀vbeVideo.ports.device.Palette()；internal/machine/machine.go:Palette()以device.DAC與dacMask建立RGB，原碼已審查為純讀取。私有package-machine適配檔只返回同一VBE device的DAC768bytes、Palette256×3與mask，不呼叫In8／Out8、不安裝Video、不修改device或guest。只在/tmp/source-overlay/編譯隔離probe，不進公開internal；原sourceoverlay以既有362檔案覆蓋層與現在public CPU建立。

保存rawDAC／mapped palette／mask前後陣列、ports讀寫map與Log長度、device State及clock保持，再對照所有307200pixel。rawDAC非零數、masked palette非零數、使用到的palette非零數與pixel RGB非零數分開，不用全黑猜是哪一層。完整新表內容與原rawDAC在native前未知，取樣契約只固定範圍，不預填結果。

## READY與驗收

直接核對374終態完整核心／FPU／clock／VBE／header20、無CPU停止及黑PNG／indexed與RGB hash；確認getter純讀取與3個既有source hash後轉READY。新private function／call／setup cap條件逆轉後精確等於374，私有適配檔僅讀取已審查的欄位，所有public internal／CPU／DOS／probe保持20fd150。沿372固定官方EXE Go全套，沒有新CPU行為，不重跑無關測試，另建置新probe。

374全部共通原列、39frames及black final PNG保持，僅多185M setup_table_snapshot與一筆colonies_color_source_snapshot；沿既有mtime／DTA／每輪RAMhash正規化，IMUL ram_effect仍保持changed_bytes與hash是否相等。13CLI拒絕三正對照、原418來源前後hash、state終態與同guest有界副本／UID GID均核對。原guest只跑一次，不重擲或挑選，1996日期不是seed。

Go1.24.13 image sha256:1a6d4452c65dea36aac2e2d606b01b4a029ec90cc1ae53890540ce6173ea77ac，Docker network none／600s／2GiB／2CPU／128pids／UID1000，原ZIP／patch唯讀，owned監測550s／trap收尾。私有入口workplace/new-game-375-ready-review.py、moo2-palette-snapshot-375-prototype.txt、new-game-375-run.sh、new-game-375-verify.py；公開本規格／000-index與374回填，原probe／LOG／PNG／RAM／state不入Git，收據連主庫既有docs/re/dosgolem-moo2-intake-20260930.md。

下一步以實際20表與色彩狀態判定玩家畫面所需的最小正常轉頁邊界。未知原因不寫成產品缺陷，不深挖DAC driver或以更大cap代替證據；主庫玩法RE-first保持，殖民地內容／存讀／完整開局與remake同狀態仍未驗。

## READY審查

原374完整185M／header20／核心／FPU／callback／IRQ／黑PNG與3個純讀source hash已核對。getter直接讀同一VBE device的DAC／Palette／mask，不呼叫IO或安裝Video；Palette既有實作已審查，未知raw結果保留。固定同185M／新20表／當前stack code／256bin索引 RGB契約審查通過，轉READY後才實作私有observer。

## CONFORMED限定結果

狀態：**CONFORMED，限定185M完整新表與原色彩狀態只讀**。殖民地畫面與正常操作尚未驗。

同374正常座標輸入與固定185M重播一次。實際step_limit=185000000／EIP223A71／unique_sites55872，完整R／段／flags206h／FPU127F與八stack bits0／clock424485517µs及VBE保持。callback target8:2136D1、mask2B／pending0／inactive、16／16與IRQ48857／48857完成且非failed。沒有guest_cpu_stop／step_error／dos_exit，也沒有新輸入或延長cap。

新185M完整表由DS188:298848取得count20／bias0／stride55，共1100bytes，SHA-256 d308de8fbcf9736ce8b4edb64e7c93b3e0cad2bfb8d7024da0776fef36235387；globals192／header16與原374終態一致。只保存原bytes，未命名新物件語意。當前EIP223A71的code16=C1E0028B8014392A000345A88A0025FF，SS188:ESP2BD720的stack16全0；不使用較早menuFrame解釋目前frame。

同一VBE device的原DAC768bytes全0，SHA-256 ef115a0e0c15cdc41958ca46b5b14b456115f4baec5e3ca68599d2a8f435e3b8；dacMask=FF，mapped palette768bytes亦全0且同hash。獨立逐bit除法與算術RGB擴展核對既有Palette()，沒有以同一AND／shift程式當oracle。256bin histogram總307200，index0有294477，其餘12723非0，使用索引集合與每個索引RGB樣本已核對；每個pixel皆符合palette映射。indexed SHA-256 4d46c5beedd237ddba268a74a01d8c33a4a0323fb52213a2e5d5b6ea4d688d43，RGB SHA-256 0b150fd32588b1daca5569992ebe559c0102c837306b1af4c44d35128ec58366，black PNG保持1610444d26adb3135e7e933dd44912044bb636728af59c70e614945278d3c622。

已證實目前RGB全黑由原DAC全0映出，遮罩沒有把索引限制成0。色盤何時歸零、是否正常轉頁中間態、是否有後續恢復仍未知，不推定列表正常開啟或產品缺陷。

取樣前後完整R／段／flags／FPU bits、clock全值、VBE、callback target／state、IRQ、device State、ports Reads／Writes map及Log長度4096保持；原DAC／mapped palette／mask保持，整RAM前後SHA-256一致。新table與color source兩observer只讀、valid true。未呼叫IO或InstallLEVideo，未代寫原RAM、色盤或結果。

## 驗證、回填與下一步

374全部12253共通原列、39frames及black final保持，僅新增185M完整setup_table_snapshot與一筆colonies_color_source_snapshot。既有mtime／DTA／每輪RAMhash比較契約保持，IMUL ram_effect仍比較changed_bytes與hash相等關係。三私有patch逆轉後精確等於374；全部公開internal／CPU／DOS／probe保持20fd150，getter僅在/tmp隔離編譯。13CLI拒絕與三正對照保持，沒有新CPU行為，沿372固定官方EXE全套，不重跑無關測試。

原418來源SHA-256保持，state與374一致；SAVE10.GAM208000bytes／0cf71fc5368861758da5a5dffbd791f8b711a48cfd295572eb9a21d27dea0f4d、MOX.SET553bytes／de8554b3284c9f9065efe6e4f2dfa71ab0c7e732e75eaea0d0ecf465ab86409f及sound.lbx4250888bytes保持。同guest有界副本與終態／UID GID1000核對。原guest一次，固定日期不是seed。Docker命令與私有收據連主庫既有研究入口，相關容器清理。

| 不可變鍵 | 新語意與等級 | 舊規格 | 回填 |
|---|---|---|---|
| 官方1.31／185M／DS188:298848 count20 stride55；VBEIndexed／同device DAC | 完整1100byte表與DAC全0／maskFF／非0索引12723映為黑已證實；轉頁原因未知 | 374 | 追加只讀新結果，保留374原收據與輸入歷史 |

下一步保持相同輸入與185M，審查internal/machine/machine.go的DAC ports既有寫入入口，再以有界私有只讀觀察保存180M起到185M的DAC寫入總數、首個全零與最近寫入邊界及其原核心／clock。只觀察已存在的寫入，不添加IO、palette修補、輸入或盲目擴cap；先由來源判斷正常轉頁邊界，不深入DAC／PIT／driver或renderer helper。主庫玩法RE-first保持，列表內容／正常操作／正式存讀語意／完整開局／RNG與remake同狀態未驗。

## 376正常轉頁窗口DAC序列回填

[376](376-moo2-colonies-dac-write-journal.md)保持375正常輸入與185M，原DAC初592非0，完整11275事件獨立重播通過。11輪均maskFF／index0..255／768色值，全部色值單調不增；首次全0與末DAC事件同為device sequence290512、loop觀察182566943／419464025µs、目前EIP222D1C。到185M無新增DAC write；降色來源已證實，恢復與列表正常畫面仍未知。375全部12255共通原列／39frames／黑終圖與兩只讀快照保持。這是loop邊界，未推定埠指令逐週期時間，舊正文與收據保持。
