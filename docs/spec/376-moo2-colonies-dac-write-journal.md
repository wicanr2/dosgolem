# 376：正常COLONIES轉頁窗口的DAC寫入序列

狀態：**CONFORMED，限定原DAC序列與歸零邊界**
日期：2026-10-04

## 原阻塞與證據

[375](375-moo2-colonies-color-source.md)同正常COLONIES輸入與185M，一次取得完整20表，原DAC768bytes全0、maskFF、12723非0像素索引逐pixel映黑。色盤何時歸零、是否轉頁中間態與後續恢復未知，殖民地正常畫面／操作未驗。官方1.31 ORION2.EXE SHA-256 4e11be14217b4aafa1839f333bf5eba037f98b0c44e9e4752c96c464c260419f，工具03dcee257142e790661be653e9ba6102136d162a，CPU保持1d8a4d8252372c97d8e873ba13c3ab3670796527dcd6d74de52e8cbf226e068c，位址空間dosgolem_high_le。

既有internal/machine/le_opl_ports.go:Out8()只把3C6／3C8／3C9交給device.Out8()再record；record的Log只保留最早4096筆，Writes map持續累加。internal/machine/machine.go:Out8()先在device.PortLog追加實際埠／值／device Steps，再按既有DAC契約寫入mask、index／phase或6bit色值。device.PortLog不是LEOPLPorts.Log，不用早期4096筆假稱晚期證據；device Steps也不當正常outer step。這些是工具來源已證實，不是硬體逐週期聲明。

主庫玩法RE-first保持。本輪沿375相同正常輸入、state與185M，只取180M至185M現有DAC寫入與最小觀察邊界。不增加cap／輸入，不改DAC或ports／CPU，不反組譯driver、PIT或renderer helper。

## 只讀契約

私有package-machine getter直接讀同一VBE device的PortLog長度、rawDAC768／mask／index／phase及從cursor起的複本。cursor必須0≤cursor≤length，每次增量≤4096，越界不讀slice。getter不呼叫IO、Step、Install或Restore，不改device、RAM或任何公開internal；只在/tmp/source-overlay隔離編譯。

正常180000000、完整來源前置之後且COLONIES press之前保存初始DAC／mask／index／phase、journal cursor、完整原核心／FPU／clock、VBE、callback target／state與IRQ。其後在每個outer loop起點讀已發生的增量，終185M再讀最後增量；事件保存原device journal index／port／value／device Step，過濾3C6／3C8／3C9，仍核對未過濾序列長度單調。觀察點是各loop邊界，不把目前EIP稱為每個內部write的執行位址，也不把同group多write猜成逐write virtual time。

每個含DAC事件的group保存observation_step、當前完整R／段／flags／FPU bits／clock／VBE／callback與IRQ、rawDAC hash與非0數、mask／index／phase、ports三埠累計Writes，以及有序原事件。group最多262144筆、DAC事件最多262144筆；不超限記錄，超限或source不一致即拒絕觀察驗收，不能略過事件稱通過。初始與cap完整rawDAC保存；group只保存hash／count，避免大量原資料重複。

每次增量getter前後以完整核心／FPU／clock／VBE／callback與IRQ、device State、ports Reads／Writes map與Log長度、journal長度／DAC／mask／index／phase核對只讀；長度getter只讀slice長度，逐loop不複製map或RAM。初始及終cap另核對整RAMhash。純讀欄位的函式來源與可逆patch另審查，不把事件自然造成的DAC變化算成observer寫入。

## READY與驗收

先核對375原185M核心／header20／rawDAC全0／maskFF／histogram、無CPU停止、39PNG及來源hash；審查完整journal的append與唯讀slice來源，再轉READY才寫私有observer。私有probe新patch逆轉精確等於375，所有公開internal／CPU／DOS／probe保持03dcee2。沿372固定官方EXE全套，另建置private probe，沒有新CPU行為，不重跑無關CPU測試。

原guest一次。375全部共通原列、39frames、黑終圖與兩375快照保持，只新增一筆私有journal收據摘要；375色彩快照RAMhash逐輪正規化但保持前後相等與只讀，原bytes與色彩hash不省略；13CLI拒絕與三正對照保持，原418來源前後hash、state與同guest副本／UID GID1000保持。固定日期不是seed。

獨立驗證從初始rawDAC／mask／index／phase，按所有有序3C6／3C8／3C9值重播，用除法與餘數實作6bit寫入／相位／index wrap；每group核對rawDAC hash／非0數、mask／index／phase及三埠Writes差。不可只比較終黑。由重播找第一個全零原write與最後write，連到其group原核心／clock；如果180M已全零，只稱初態已零，不虛構新變化。group之間沒有palette寫入時不能宣稱轉頁完成。

工具觀察輸出workplace/new-game-376-dac-journal.json、new-game-376-ready-review.py、new-game-376-run.sh、new-game-376-verify.py；原DAC／事件／LOG／PNG／state及private getter／probe不入Git。公開本規格／000-index與375追加回填，收據連主庫既有docs/re/dosgolem-moo2-intake-20260930.md。

Go1.24.13 image sha256:1a6d4452c65dea36aac2e2d606b01b4a029ec90cc1ae53890540ce6173ea77ac，Docker network none／600s／2GiB／2CPU／128pids／UID1000，原ZIP／patch唯讀，owned監測550s／trap收尾。依實際寫入序列決定下一個玩家可見邊界；不盲目延長cap求過，不深入DAC／PIT／driver。列表內容／正常操作／正式存讀／完整開局／RNG與remake同狀態仍未驗。

## READY審查

375終態、原DAC與核心／callback IRQ及三source hash已核對；完整device journal append與早期LE Log區別已審查。cursor／增量界限、純讀來源、獨立序列重播、原共通列／frames保持與固定185M契約通過，轉READY後才建立私有適配與observer。

## CONFORMED限定結果

狀態：**CONFORMED，限定正常180M至185M實際DAC寫入序列與歸零邊界**。殖民地畫面與正常操作未驗。

原guest一次，相同375正常輸入與固定185M，沒有CPU拒絕。375全部12255共通原列／39frames／black final及兩只讀快照保持；375色彩快照只將逐輪RAMhash正規化，前後相等／只讀與其餘原bytes、色彩hash仍核對。公開internal／CPU／DOS／probe保持03dcee2。四私有patch逆轉精確等於375，13CLI拒絕及三正對照、原418來源／state／同guest副本／UID GID1000通過；沿372固定官方EXE全套，沒有新CPU行為，不重跑無關測試。

180M press前原DAC有592個非0值，rawDAC SHA-256 ddf4dd57bc57ded0c9deddf28069189396b21f3c855841debfe98a1f482d5292，maskFF。完整journal過濾共有11275筆DAC事件，3C6／3C8／3C9分別11／2816／8448筆；11275個group各一筆。每筆保留原device journal index／port／value／device Step，獨立除法／餘數重播初始DAC／mask／index／phase，全部group hash／非0數／三埠累計Writes與終態核對。原sequence不是outer step，group目前EIP不當每個內部write的原指令位址。

首次DAC觀察邊界182482821／419218224µs、目前EIP222C9F，開始第一輪maskFF。共有11輪，每輪寫maskFF、index0..255與768個色值，全部色值相對初始及前次單調不增。每輪終非0數依序592／583／583／583／583／583／571／554／545／496／0。這是原寫入數值已證實；轉頁呼叫語意與正常列表仍未知。

首次rawDAC全0與最後DAC事件同為原device sequence290512、observation_step182566943／419464025µs，port3C9 value0；當前EIP222D1C、R=[0 1 7003C9 64 2BD99C 2BDBD0 2A3758 2BDC2C]、段=[8 188 188 0 20 188]、flags6h，FPU127F／status0／depth0／八stack bits0。target8:2136D1／mask2B／pending0／inactive、callback16／16與IRQ48162／48162已返回且非failed；VBE bank9／startY512／sets2935／writes55268558／display93。這是寫入後loop觀察邊界，不冒稱埠指令的逐週期時間。

182566943之後直到185M沒有新增DAC write；終rawDAC全0／maskFF與375一致。原185M仍EIP223A71／flags206h／clock424485517µs，完整核心／FPU／VBE／callback16／16／IRQ48857／48857及20表保持。原降色到零已證實，後續色盤恢復與轉頁完成未知；不把黑圖稱列表驗收或產品缺陷。

每次增量getter前後完整核心／FPU／clock／VBE／callback與IRQ／device／ports map及早期Log／rawDAC／mask／index／phase／完整journal保持，初始與終cap另核對全RAM只讀。長度getter純讀slice length；不呼叫IO、安裝、Restore或代寫模型。最大增量4096／總事件與group262144界限未超出。

## 回填與下一步

| 不可變鍵 | 新語意與等級 | 舊規格 | 回填 |
|---|---|---|---|
| 官方1.31／180M至185M／同VBE device PortLog／3C6 3C8 3C9 | 11輪色值單調不增、首次全零與末DAC寫入182566943已證實；185M前無恢復 | 375、374 | 追加降色來源與恢復未知，不改舊輸入／黑終圖歷史 |

375與374正文保留並追加376，索引及backlink由私有驗證核對。原序列／rawDAC／LOG／PNG／state／probe與getter維持本機，私有收據SHA-256與實際Docker入口見主庫既有研究檔。原guest一次，固定日期不是seed；相關Docker容器清理。

下一步以376已證實的11輪單調降色與182566943歸零為來源，先審查新的轉頁觀察契約：保留185M完整前置，限定追加一次10M窗口至195M，追首個恢復非0色值的DAC寫入並保存原核心／clock與可見頁；未恢復時記錄實際邊界，不以加碼重跑求過。不得代寫palette、增加玩家輸入或深入DAC／PIT／driver及renderer helper。主庫玩法RE-first保持，列表內容／正常操作／正式存讀／完整開局／RNG與remake同狀態未驗。
