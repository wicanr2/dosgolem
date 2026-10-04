# 378：CPU386 SUB AL,imm8與原2C17 consumer

狀態：**READY**
日期：2026-10-04

## RE來源與缺口

[377](377-moo2-colonies-restore-continuation.md)相同正常COLONIES輸入、185M前置與明示195M上限，原188532362在dosgolem_high_le:1F455D／2C17停止。原16bytes2C173C080F87ED0000000FB6C02EFF24，工具抓opcode後EIP1F455E；原SUB尚未執行。官方1.31 ORION2.EXE SHA-256 4e11be14217b4aafa1839f333bf5eba037f98b0c44e9e4752c96c464c260419f，工具168b91b9a8cb08a36f9517ae031a98239f841161，原CPU SHA-256 1d8a4d8252372c97d8e873ba13c3ab3670796527dcd6d74de52e8cbf226e068c。

原停止R=[1A 0 2BD800 D 2BAFC8 2BD8A0 29E1D8 2BD348]、段=[8 188 188 0 20 188]、flags206h／FPU127F／status0／depth0／八stack bits0、clock430866468µs，callback16／16與IRQ49858／49858已返回。原source為未支援decoder停態；naked2C無派送且fallback只回error，fetch8成功只後移EIP，不能把原停止當指令成功。

internal/cpu386/cpu.go已有sub8()更新CF／PF／AF／ZF／SF／OF並保留其它flags，reg8／setReg8處理AL。搜尋到的case0x2c屬D9 x87的ModRM，不是SUB入口。現行2D與80 /5不代替2C，新功能只補naked2C，主庫玩法RE-first保持。

## ISA與規則

[Intel SDM Vol.2B](https://www.intel.com/content/dam/www/public/us/en/documents/manuals/64-ia-32-architectures-software-developer-vol-2b-manual.pdf) SUB條目4-654至4-655頁列2C ib／SUB AL,imm8，目的AL寫入8bit減法結果，更新六算術flags。此為公開ISA契約，不稱遊戲原硬體時序或逐週期證實。

typed input為原AL／一byte imm8／原EFlags。8bit結果按256回繞；CF用unsigned借位，AF用低nibble借位，OF用有號8bit差值超出-128..127，ZF／SF／PF按結果。保留EAX高24bit、其它R、段、FPU與非算術flags；不讀寫資料RAM，不呼叫port或平台。立即數成功fetch後才更新AL／flags；opcode或imm fetch失敗不發布結果，EIP按既有fetch8已消費byte前進，error仍指原start與opcode。

本規格只支援naked2C，66／segment／repeat／LOCK及未知prefix維持工具能力拒絕，不宣稱這些全是ISA非法。既有其它SUB入口與prefix契約保持，不藉本任務擴張一般CPU範圍。

## READY與驗收

先核對377完整CPU停止／原code與stack／baseline185M／原來源hash及CPU source，審查SUB helper與fallback，再轉READY才實作。先用單一原2C17測試證明舊core拒絕，保存RED結果，才加派送並跑新測試。原AL1Ah減17h的預期為03h／flags206h，此時只是ISA導出預期；自然consumer完成才升原版已證實。

獨立驗證256×256×三種EAX高24bit×兩種初flags，共393216組；用寬差值／有號界限／nibble借位與bits計數oracle，不呼叫production sub8，不用相同xor／shift旗標公式。另用邊界值遍歷64種初算術flags，核對完整R／段／FPU／非算術flags、只讀code及零Bus write。截斷／Bus imm fetch拒絕、原指令位址與最後byte／EIP wrap、工具不支援prefix均核對。原2C17→3C08→0F87→0FB6C0四步另用完整R與獨立flags核算；不命名後續jump table語意。

CPU source有變更，重跑完整internal/cpu386與internal/machine，明設固定官方EXE避免MOO2測試skip；其它缺原素材的外部oracle仍按既有skip，不能宣稱全遊戲或其它CPU分支完成。公開新CPU／自製測試／本規格／000-index及377回填，原資料與私有native收據不進Git。

## 原正常路徑

保持377全部入口、185M前置與一次195M上限，18CLI拒絕4正對照、39frames與baseline黑PNG。private probe只在原188532362／1F455D匹配後加四步只讀consumer，保存before／after完整R／段／flags／FPU bits、code16／SS:ESP stack16、callback IRQ／clock及RAM效果；不注入原R／RAM／選取結果，不增加輸入或cap。

比較377所有共通原列至真正2C入口，baseline完整DAC序列與185M來源保持；原新指令／四consumer及後續新邊界另驗，不拿不同末態或terminal totals稱同狀態。首色彩恢復與PNG仍沿377獨立序列／PNG映色驗證。新CPU若先遇其它拒絕，保存實際邊界，不重啟guest或假稱195M成功。原guest一次，state及同guest副本按實際核對，不預填存檔內容相等。

Docker沿Go1.24.13 image sha256:1a6d4452c65dea36aac2e2d606b01b4a029ec90cc1ae53890540ce6173ea77ac，network none／600s／2GiB／2CPU／128pids／UID1000，原ZIP／patch唯讀、owned監測550s／trap收尾。private入口workplace/new-game-378-ready-review.py、new-game-378-cpu-tests.sh、new-game-378-run.sh、new-game-378-verify.py；原probe／journal／PNG／LOG／RAM／state不入Git，收據連主庫既有docs/re/dosgolem-moo2-intake-20260930.md。

下一步由原四consumer與正常畫面／實際CPU邊界決定，不再擴cap，不深入DAC／PIT／driver或renderer helper。主庫玩法RE-first保持，列表正常操作／正式存讀／完整開局／RNG與remake同狀態未驗；固定日期不是seed。

## READY審查

377原停止完整來源與CPU hash、naked2C缺派送／D9 ModRM區別、sub8及fetch fallback核對通過。Intel SDM SUB之8bit結果與六旗標契約、獨立393216組／64flags邊界／失敗保留與原四consumer、同195M窗口審查通過，轉READY後才寫測試與入口。

## CONFORMED限定結果

狀態：**CONFORMED，限定naked2C、原四步consumer及同195M窗口**。正式CPU只新增2C入口，既有sub8與其它opcode保持；不是整個CPU或remake玩法驗收。

舊core單一原2C17測試先RED，明示opcode尚未支援。新入口通過393216組、64種初算術flags的5184組邊界、原完整核心四步、fetch截斷與拒絕、工具prefix邊界及EIP wrap。固定官方EXE重跑internal/cpu386與internal/machine全套通過，沒有MOO2 skip。首次測試wrapper誤加Bus不存在的Write16／Write32，編譯在測試前失敗；讀回Read8／Write8契約後移除，首失敗另存。真正RED與後續綠測試分開，不把編譯失敗當CPU證據。

原188532362在dosgolem_high_le:1F455D自然執行2C17，AL1Ah→03h／flags206h；下一3C08產生293h，0F87ED000000未跳、EIP1F4567，0FB6C0後EAX03h／EIP1F456A。四步其它R／段／FPU／SS:ESP stack與RAM保持，callback16／16、IRQ49858／49858均已返回且inactive／非failed。原R與RAM未注入，沒有追加玩家輸入；這四步結果已證實，不猜jump table用途。

原377共通12393列至2C入口保持，完整185M DAC baseline、39frames及黑PNG保持；377已有12300事件前綴保持。相同一次195M上限實際到step_limit195000000／EIP22C8BA，無CPU stop或DOS exit。全部24601 DAC事件獨立重播，三埠3C6／3C8／3C9計25／6144／18432筆，終DAC／mask／index／phase及ports累計一致。

首非0DAC write為sequence292693／原189322149／432332347µs／dosgolem_high_le:222CCE／3C9=04h。首恢復快照僅一個DAC值非0，當時RGB仍全黑；不能把這個write當可見畫面恢復。195M終DAC588個非0、indexed296428非0、RGB759775非0，獨立PNG解碼與palette histogram映色一致。終PNG SHA-256 d3c775f1b8594e8313c27b5bd9e363dfb6e3210ca05f0f17136b36409b7a67ba，親看殖民地列表顯示Sol II與人口圖示；不稱列操作、轉入殖民地或人口調整已驗。185M、首非0與終PNG分開保留。

20物件表DS188:298848／stride55／1100bytes，終SHA-256 3c6bd2afa22a4ac6500ce62df53ea9bfe88dd4713423922949d23f179820471c，與377停止表相同；185M至377的10個raw byte變化仍語意未知。既有collector另自然取得第三個214104 RET，原189574450按SS188:2BD920實際stack返回20DB5B，AX0、flags246h與完整其它核心／FPU保持，這次共享word為0000，不把早先選取10宣稱持久不變，也不猜重設writer。

五私有patch逆轉精確回377，CPU單一2C新增區段逆轉精確回168b91b，其它公開internal與原probe保持。18CLI拒絕／4正對照、原418來源與state、同guest副本及UID GID1000保持，原guest一次。完整readonly取樣前後狀態與PNG／原序列核對通過，原LOG／PNG／journal／RAM／state不入Git。

| 不可變鍵 | 新語意與等級 | 舊規格 | 回填 |
|---|---|---|---|
| 官方1.31／同195M上限／1F455D 2C17 | SUB與四consumer自然完成，195M可見殖民地列表，已證實於本收據 | 377 | 追加恢復結果，保留舊缺opcode與黑圖歷史 |

[377](377-moo2-colonies-restore-continuation.md)正文保留並追加378，索引與backlink同次核對。CPU／原平台／時序範圍保持，Docker容器收尾；收據SHA-256與實際命令連主庫既有研究入口。

下一步以195M可見Sol II與同時取得的20物件表，核對正常列表行的熱區／原選取來源及callback前置，再建立一次正常press／原AX3 poll／安全release的限定驗證。不增加cap或猜欄位，不深挖DAC／PIT／renderer helper。主庫玩法RE-first保持；列表操作／正式存讀／完整開局／RNG與remake同狀態未驗，固定日期不是seed。

## 379原列表行來源回填

[379](379-moo2-colonies-row-source.md)重用本195M完整來源與終PNG，固定官方EXE另建一次性IDA9.4資料庫。原索引遞增、signed含端點矩形與首命中即離開已核對；Sol II名稱區logical43,48／44,48幾何命中13與19，原順序先13，type7非14／11。預期index13為原靜態規則導出，實際行點擊與word13未驗。本輪沒有新guest、cap、輸入或CPU／state改動；378四consumer與195M恢復正文及收據保留。

## 380正常Sol II行選取回填

[380](380-moo2-colonies-row-click.md)保持本完整195M前置／39frames／可見列表PNG與24601 DAC前綴，再一次正常行press／poll／release；原20DDDB在195226311把shared word寫0→13，正常選取已證實。新200M固定窗口末圖為黑，11輪新降色與count1／55零bytes另保存，殖民地畫面未驗。本四consumer與195M恢復正文及收據保持，不能把新末態當本195M變更。
