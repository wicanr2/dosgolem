# 335：REPNE SCASW與正式選單後續

狀態：**CONFORMED**，限定CPU契約／原掃描／正常設定頁
日期：2026-10-03
範圍：cpu386新增F2 66 AF／66 F2 AF的16位比較、32位地址／計數；既有F3 AF與SCASB保持。裸AF／66 AF／F2 AF／F3 66 AF／67／段覆寫及其他前綴不擴張。

## 玩家阻塞與原始定位

工具4501b831842f33ee5a0388b3018ae0c240949bc3；官方1.31 ORION2.EXE SHA-256 4e11be14217b4aafa1839f333bf5eba037f98b0c44e9e4752c96c464c260419f，位址均dosgolem高位LE。沿[334-moo2-ready-menu-normal-click](334-moo2-ready-menu-normal-click.md)新鮮417根檔／MOX.SET、44M Esc／1996-01-01、兩次正常press／release與100M cap情境。76658331於1F3640的F2 66 AF拒絕；EAX2／ECX9／flags246h已見，完整初態已在下節唯讀取得；此處記錄334歷史停點與黑屏，335設定頁驗收見末節。正常開局未知。日期不是RNG seed。

## 公開CPU契約

[原廠80386 SCAS](https://www.ardent-tool.com/CPU/docs/Intel/386/manuals/prref386/SCAS.htm)：AX−ES:[EDI]，只更新CF／PF／AF／ZF／SF／OF，依DF令EDI±2，不改AX或資料。[Intel SDM 2B](https://www.intel.com/content/dam/www/public/us/en/documents/manuals/64-ia-32-architectures-software-developer-vol-2b-manual.pdf)印刷頁4-549..4-551：REPNE於比較後ZF=1或ECX=0停止，初始ZF不限制第一次比較；故障恢復指令前EFLAGS，保留成功元素的ECX／EDI。80386 REP鏡像偽碼條件顛倒，採SDM及同頁文字，不依錯偽碼。沿[099-cpu386-repne-scasb](099-cpu386-repne-scasb.md)與[292-cpu386-repe-scasd](292-cpu386-repe-scasd.md)。

## 最小實作契約

只允許repne=true／operand16=true／無段覆寫的AF；F2早期閘門只為該形狀放行。每次讀完整little-endian ES word，沿既有sub16計算六旗標；成功比較才EDI依DF加減2並32位繞回、ECX−1。ECX=0不讀資料、不改R／flags；初始ZF=0／1都先比較。其餘R／六段／FPU／記憶體／非算術旗標保持，EAX高16也不參與比較或被改寫。不是依遊戲位址特例。

讀取失敗恢復原EFLAGS、保留已成功元素ECX／EDI；解碼EIP可能前進、工具Error與單次Step內無IRQ模型沿292，不聲稱硬體exception重啟或逐週期時序。重複／衝突前綴、截短與未知selector／段末／bus錯誤仍明確拒絕。

## 唯讀初態與驗收

DRAFT階段只做可丟棄／有界probe：新ready情境首個1F3640及下一MOV1F3643，共最多兩實際步，保存完整R／六段／EIP／flags／16code bytes，固定原ES:EDI最多256word及原SS:EBP-4四bytes、callback／IRQ與readonly；不改CPU／輸入／資料，原first count9／DF0範圍直接RAM peek。其他count或DF的未保存來源明示不可讀，不猜。CPU未改時先保存初態，再審查READY。

READY後增加獨立16位寬值減法／有號範圍／低nibble借位／同位旗標oracle，測不同初始六旗標與DF兩向、零／1／2／9／17／大計數、first／middle／last命中與不命中、ES與DS分離、非對齊／段末完整word／EDI32位繞回／完整ECX高word、未知段／bus逐byte失敗／成功進度與旗標恢復、前綴／截短拒絕。既有292與SCASB保持；沒有可用386實機語料時明記是公開契約推導的獨立測試，不稱實機硬體對拍。

CPU／固定EXE Go全套通過，再重跑原三基線與同ready情境，不提高100M cap、不增加輸入。新掃描由已保存原ES words獨立找首次AX匹配／計數耗盡，核算六旗標／EDI／ECX與下一MOV原bytes consumer。舊無旗標全部3847／4829／6769列與72PNG保持；新ready至335原入口前保持334相同輸入；其後新CPU錯誤／正常UI按實際結果保留，不推定設定頁成功。原版helper只取一條後續MOV消費，不深入函式。

原版素材／LOG／PNG／RAM留忽略workplace；自製CPU測試不含原版素材。新規格同次索引000-index，限定CONFORMED後回填334／292與守衛。Go1.24.13固定Docker／600s／2GiB／2CPU／128pids／UID1000／network none，輸入唯讀。主庫RE-first與完整remake／中文化目標保持；Docker／擁有權收尾。

## 未改CPU的完整初態與READY審查

未改CPU SHA-256 1448f29f24dad35e83575189055dc15dd7ac9db2b3529c6d450f03baf91c97b1，單次同ready情境只加最多兩步的診斷，扣新repne_scasw前綴後全部334 ready舊列除既定正規化保持。初態收據workplace/moo2-probe-335-input.txt.gz SHA-256 c78e848f2577e04bbe46ed34968a9e3476d034197a4f0c860f609532af57c955。

76658331原高位LE1F3640完整R=[2 9 D6 2BDA40 2BDA0C 2BDA14 78 1F357B]，段=[8 188 188 0 20 188]、flags246h／DF0。ES188:1F357B完整18bytes 03080208010800080300020001000000E936，word依序0803h／0802h／0801h／0800h／3／2／1／0／36E9h，直接peek前後相同、readonly=true。原SS188:EBP2BDA14-4即2BDA10四bytes64000000，下一1F3643的8B45FC；callback4／4、IRQ15961／15961皆非活動。來源用途未知，只核對此CPU消費。

公開契約與未改CPU原來源足夠，335可READY：AX2在第6個word匹配，推導ECX3／EDI1F3587／flags246h，其餘R／段／來源保持，EIP1F3643；下一MOV讀原SS2BDA10的64h到EAX、EIP1F3646，其餘核心與旗標保持。這是規格預期，尚未當作驗收結果。指令主體沒有遊戲位址條件；read16完整成功才更新元素進度，故障恢復原flags。沒找到已裝的386實機語料，測試明示為公開契約導出的獨立規則驗證，另以固定原版自然流程核對。原helper只取這一MOV消費，不挖內部。

## 正式驗收與目前邊界

限定驗收：REPNE SCASW契約／原掃描與單筆MOV／正常設定頁。335只補通用CPU能力，不改主庫玩法、原版輸入或100M cap，不把設定頁當完整開局。

**已證實，原掃描與最小消費**：同ready情境76658331原1F3640 F2 66 AF實際成功，完整初態與未改CPU收據逐項吻合。AX2第6個word匹配，ECX9→3、EDI1F357B→1F3587、flags246h保持、下一EIP1F3643；其餘R／六段與ES來源18bytes保持。76658332下一8B45FC讀原SS188:2BDA10四bytes64000000，EAX2→64h、下一EIP1F3646，其餘核心／flags與來源保持。callback4／4、IRQ15961／15961皆非活動，readonly=true、error=nil。原來源SHA-256 8d541fc3f57f9ea2e104600161cf1973e124d7970ee34c6aea92329a2a9f8221，來源用途未知，不深挖helper。

**已證實，正常設定頁**：原本61538983第2筆原store、全部正常press／release保持334；新ready到100000000原高位LE228DE8，無新CPU拒絕。終態R=[3 1DF 163 9F 2BDB40 2BDB9C 2C0DEC 2C0660]、段=[8 188 188 0 20 188]、flags246h，callback4／4完成、pending0／active=false。實際640×480 PNG人工確認NEW GAME設定頁，Difficulty Tutor／Galaxy Size Medium／Galaxy Age Average／5 Players／Tech Level Average、CANCEL與ACCEPT已顯示；只確認畫面，不推定選項或ACCEPT已經操作。PNG SHA-256 1507dfb323dd4614fee5eb1523ab60c5652c7eb2a7fa6c53182772b5bdc64908，RGB SHA-256 3bbe6c339cfbc74e84b3210bccfdc4574073b4d308ef9b90a1720cc5c74e623e。原版PNG留本機忽略workplace，不公開素材。

python3 workplace/new-game-335-verify.py PASS：無旗標全部3847／4829／6769舊列除既定正規化保持334、全部72PNG逐位元保持；ready掃描入口前全部5833原列保持334，未改CPU完整初態對接、掃描結果與下一MOV獨立核算，正常原輸入／選擇保持。初始ZF=1仍進行第一比較，確認本次ECX3，不以零步掃描放過。

自製獨立測試涵蓋92416個單元素算術／旗標／方向／前綴組合及多元素、零／高ECX、32位EDI繞回、ES／DS分離、非對齊／段末、逐byte讀取故障、進度與原flags恢復、未知／截短／重複前綴拒絕及單筆MOV消費。go test -p 2 -buildvcs=false ./internal/cpu386 -run 'TestREPNESCASW|TestREPESCASD|TestREPNESCASB' -count=1 -v PASS，cpu386 0.266s；DOSGOLEM_MOO2_EXE=/tmp/game/ORION2.EXE go test -p 2 -buildvcs=false ./... -count=1 PASS，cpu386 106.777s。公開契約推導的獨立規則驗證，不稱386實機語料或硬體逐週期對拍；原版同初態消費另有上述收據。

CPU SHA-256 967de02753e0dce276413fe67a085e6df16789b52c948999d30ed26c3bb4e6a4；自製repne_scasw_test.go SHA-256 f8e512f4d57cc18e1eb3933f448f7c5493f3e4a2b224c3da72d00d8eecfac97d；probe SHA-256 63d47d60e38e0d2489aadda95aef334f3d66005c72cde96dd893f5d71f272979。startup／provider／matcher／輸入CLI保持334，所有新來源與收據1000:1000。

四原版由新鮮417根檔／官方EXE各一次重生；固定Go1.24.13映像sha256:1a6d4452c65dea36aac2e2d606b01b4a029ec90cc1ae53890540ce6173ea77ac，Docker600s／2GiB／2CPU／128pids／UID1000／network none，原ZIP／patch唯讀。go build -p 2 -buildvcs=false -o /tmp/moo2-probe ./workplace/moo2-probe；沿334三基線與ready情境換335輸出名，沒有新增輸入或提高cap。

| 本機忽略收據／腳本 | SHA-256 |
| --- | --- |
| workplace/moo2-probe-335-input.txt.gz | c78e848f2577e04bbe46ed34968a9e3476d034197a4f0c860f609532af57c955 |
| workplace/moo2-probe-335-baseline.txt.gz | 62289373b26d4a2289fd23b6f6d7df8efb80779d3d037389767f381e1655a638 |
| workplace/moo2-probe-335-click.txt.gz | 15a308f1ad491c5c99f98f49d024c25d61d37e96a4ef5e6015fca5eeb584b8a3 |
| workplace/moo2-probe-335-extended.txt.gz | cb2a91372a22e6bcdf3786aa6060c3e27dd06890a97cd6553d2454037a65a7bf |
| workplace/moo2-probe-335-ready.txt.gz | 00d1c848f02c7d9fdf29fd92b7f4010a96e35f092eeb8b3f630b6e9701959233 |
| workplace/new-game-335-verify.py | dda63b17dfa67491bc140cc5f809c0c9b364656a8745c967000b4d30c9c7133f |
| workplace/moo2-335-cpu-narrow-tests.txt | d87bd1466a51b5a245fdff3125b4d2a688ba2b6e7aa7f1e508fceaebb46955a3 |
| workplace/full-test-335.txt | 2b8c03eb757e9a021b70885047357e77b4c931d23eec7f8f1376f56feae10e52 |

72回填函式、既有缺證據負例及335新增26負例、兩CLI通過。私有parity收據workplace/new-game-335-parity-tests.txt SHA-256 9df66abe1d8bcf40295dd82919dc612be41da300a747478919d21dfb002b9b4a；backlink收據workplace/new-game-335-backlink-tests.txt SHA-256 440e7f914cfc6c8fc5ba298db0e299344f759bea8f6a864ae70d4b4736aa597e。

**未知與下一步**：設定選項、ACCEPT消費、選族與完整正常開局、正式RNG、remake同狀態與整款中文化仍未驗。下一步只保存此原設定頁的按鈕表與ACCEPT正常輸入前置，按原資料核對狀態；不追原掃描helper或renderer內部。335完成的CPU能力不重列待辦。主庫RE-first保持。

輪末工具root-owned／誤建.md目錄自檢空，Docker兩工作區掛載篩選空，沒有本輪遺留容器。原ZIP／patch／EXE／MOX.SET與417根檔由每次fresh輸入檢查通過；startup／provider／matcher逐位元保持334，gofmt與Git差異檢查通過。

## 2026-10-03 正常玩家路徑回填

正常ACCEPT與選族頁已由規格336接通，見[336](336-moo2-setup-accept-normal-click.md)。原17筆表／正常index15 store已核對，四舊基線與102PNG保持；SELECT RACE畫面已確認。本文原ACCEPT未知屬先前範圍，完整開局、種族選擇、RNG與remake同狀態仍未知。
