# 328：後段目的 RAM 與 VBE 發布監測

狀態：**CONFORMED**（僅後段CPU Bus觀測與VBE對帳）
日期：2026-10-03
範圍：固定正常1.31單次NEW GAME的後段實際CPU Bus觀測，不改CPU／平台或主庫玩法。

## 證據與問題

工具b6fc71c0ba5a426a7c496fc3a5bd342cca1515ec；官方EXE SHA-256 4e11be14217b4aafa1839f333bf5eba037f98b0c44e9e4752c96c464c260419f，原417根檔／MOX.SET與44M Esc／1996-01-01／50M cap沿[327](327-moo2-post-click-source-consumer.md)。兩次真正213336寫DS188:499300h／499303h原值FDh，尚未確認讀回或畫面發布。

既有internal/machine/moo2_vbe_video.go以moo2VBEMode0101固定參數建立A0000h起64KiB視窗；LEMachine.Read8／Write8只將active視窗存取送至獨立VBE framebuffer，其餘送m.Mem。VBEIndexed另回目前StartY的顯存副本。原版RAM寫入不是VBE發布。平台契約沿[265](265-moo2-vbe-window-control.md)與[266](266-moo2-vbe-display-start.md)，不重做硬體RE。

CPU的fetch／一般read16／write16／write32均循Bus.Read8／Write8；SegmentRead8／16先走既有hook，普通RAM回false而回Bus。internal production沒有Bus型別斷言依賴。讀取觀測只能證明監測區間的CPU Bus路徑；平台服務直接讀RAM與原版較早或較晚取用不在覆蓋內。

## 觀測契約

沿既有phasePrefix與正常點擊，恰在49500000單步前、已正常按下放開時，包裝現有m.CPU.Bus一次。包裝只有Read8／Write8，原始Bus每次真正請求恰呼叫一次，原返回值與error完整保留；不自行重讀、不吞錯、不走合成guest讀請求、不更改CPU／平台或任何hook。包裝只存自身計數／少量樣本，舊Bus保留於欄位；基線未點擊不安裝。

保存當時DS原始selector／descriptor.Base，以uint64檢查DS offsets499300h／499303h與來源3DDD67h..3DDD71h在段limit與RAM範圍，再轉實際linear監測。offset與linear分開列，不能假定base=0。不可讀／超界就明示並停止安裝，不猜欄位用途。

監測區間49500000至原50M上限。每次CPU.Step前保存outer_step與input_eip；統計真實Bus總讀寫、errors、兩目的byte成功讀／寫次數、來源11byte區成功讀次數、A0000h..AFFFFh成功寫次數。每目的前四次讀／寫、來源前四次讀保留原請求linear／value／error和原始EIP，其餘只計數；條件與上限執行前固定，不挑數據。終態保存開始／結束VBEState與實際監測Step數／Bus身分保持。

正對照為真正來源MOV與目的Write8，其值／outer_step應與327舊列吻合。VBE寫次數與Writes差額對帳；零只限區間與已觀測路徑，不說完整軟體永不發布。目的值相同仍是實際寫入，不說值改變。沒有讀回或發布時，不追完整renderer／helper、不加cap／重點／代寫。

## 審查與驗收

只修改workplace/moo2-probe/main.go與證據守衛。CPU／平台來源逐位元保持，325固定EXE全套沿用。go build後從原417輸入跑未點擊／點擊兩正常流程，剔除新post_click_publish_*列，沿327明示正規化mtime／DTA四byte／PNG路徑與各次RAM雜湊，全部327舊列與六PNG／兩終PNG保持，每點RAM前後相同與readonly=true仍逐筆檢查。

核對包裝原Bus單呼叫與錯誤透傳；實際計數／正對照／來源／目的值／區間界線／VBE前後對帳通過才限定CONFORMED。跨次完整RAM一致、未被Bus覆蓋服務、80h後續、整個畫面發布／正常新遊戲與remake同狀態保持未知。新規格同次掛入000-index，成功後回填327最小發布待辦與守衛，不改主庫RE-first閘門。

下一步由實際結果決定最小玩家阻塞；不以沒有CPU拒絕推原版與remake完成。

READY審查：production CPU無Bus型別斷言或較寬Bus快速路徑；一般RAM來源MOV與目的MOV確實循Read8／Write8，既有hook的普通RAM回false。包裝只轉呼叫與保存自身計數，不改m.Mem／CPU／平台；監測只在真正CPU.Step內啟用，探針快照不計入。兩目的與來源窗口先由實際DS descriptor驗線性範圍。固定模式與265提供視窗位置；VBEState差額是獨立服務對帳。舊327逐列／六PNG保持與正對照足以驗限定觀測，零讀回不可升格完整renderer結論。

## 正式收據與限定結論

只改probe的Bus觀測，CPU／平台來源保持327，325固定EXE全套仍有效。Go1.24.13固定映像sha256:1a6d4452c65dea36aac2e2d606b01b4a029ec90cc1ae53890540ce6173ea77ac；600s／2GiB／2CPU／128pids／UID1000／network none。唯讀原ZIP／patch重建417根檔，go build -p 2 -buildvcs=false -o /tmp/moo2-probe ./workplace/moo2-probe，再沿327完整兩正常流程，只換328輸出名。

獨立python3 workplace/post-click-publish-328-verify.py PASS：無點擊全部3847列、點擊全部4369列，除327既定mtime／DTA四byte／PNG路徑／每次RAM雜湊正規化外保持，六快照RAM前後相同與readonly=true；六後段PNG／兩終PNG逐位元保持，沒有新CPU拒絕。觀測包裝在49500000安裝，當時DS188 descriptor.Base=0、Limit=FFFFFFFFh，兩目標linear確為499300h／499303h，來源linear3DDD67h..3DDD71h共11byte。普通RAM與VBE服務沒有變更，Bus原呼叫／返回值／錯誤完整透傳，包裝身分終態bus_matches=true。

**已證實，固定正常單次點擊的有界原版CPU Bus**：

- 監測恰500000個實際CPU.Step，5775248次讀請求、686978次寫請求、errors=0。
- 兩目標target_reads=[0 0]、target_writes=[3 5]；來源11byte區source_reads=154、VBE視窗vbe_writes=0。
- 開始與結束VBEState完全相同：Bank4／StartY0／BankSets797／Writes16194454／DisplaySets42。
- 源前四筆49500047／49500078／49500098／49500129皆21334D，讀3DDD67h／68h／69h／6Ah，值02h／82h／02h／02h。前三筆與327真正MOV前後R／來源窗口逐項吻合；第四筆只有真實Bus收據，不稱獨立完整核心驗證。
- 兩目標前四筆寫入保留。499300h三筆於49500071／49575709／49910819，前兩筆213336寫FDh，第三筆2176A1寫00h。499303h前四筆於49500122／49557015／49575760／49614173，皆213336，值FDh／FDh／FDh／D5h；第五次只計數、不猜值與用途。最早兩個327目的窗口／AL正對照完全吻合。
- 不以後來D5h／00h寫入推新畫面或素材錯誤。原327兩筆中心原本就是FDh的歷史結論保持；後段後續寫值與該歷史取樣分開。

VBE寫次數與Writes差額0獨立對帳通過，終圖仍PNG59f76749db5232f97a6b6f969f56f848c2e89e731d7e3fb30b8786982bca4a81，設定畫面仍未知。零讀回只限此後段CPU Bus；服務直接RAM讀取、較早／較晚取用與完整renderer不在覆蓋範圍，不稱目的資料永不被消費或整款遊戲掛起。包裝真正一般來源／目的正對照有結果，零讀回不能歸因掛勾未工作。

| 本機忽略收據／腳本 | SHA-256 |
| --- | --- |
| workplace/moo2-probe-328-baseline.txt.gz | 5fecae395d6bda9def947025d4571974abc5121adea2708ad94cd213f51958d9 |
| workplace/moo2-probe-328-click.txt.gz | 09ae500cc236aba3c661aa967720b7fe861ffc30fbb9ca83c2de188ca8633878 |
| workplace/post-click-publish-328-verify.py | 90c8908f6cc691c6c464a2a9d2124b2ce2a071c6ca722a3cf2b654aee38b38f8 |
| workplace/post-click-publish-328-parity-tests.txt | 332849a1cbf7bb63338fbb9b0283a319eea05d088e6def28fd5f0182cfefd818 |

probe SHA-256 5590a5cad4663dcb91df9124648f04b2a70a6d4a25b9799d0082df262a76f0b5；CPU仍1448f29f24dad35e83575189055dc15dd7ac9db2b3529c6d450f03baf91c97b1，startup／provider／matcher沿327逐位元保持。固定官方EXE與來源ZIP／patch／MOX.SET不改。

限定CONFORMED為此區間CPU Bus觀測／原呼叫透傳／三源MOV與兩目的MOV正對照／VBE計數對帳與既有列保持。完整RAM跨次一致、80h後續、服務直接RAM路徑、正常設定畫面與remake同狀態仍未知。既定50M收據已到上限且沒有新CPU拒絕，未有足夠證據把問題路由成CPU／素材／renderer故障；獨立100M有界續行已由規格329接通，見[329](329-moo2-bounded-new-game-continuation.md)；50M兩基線與前綴保持，較晚Bus讀回／VBE寫入與致謝文字已見，仍主選單。下一步追最小NEW GAME激活，不繼續加預算或追整個renderer。

65回填函式、原有32／49／25／27／27／34與328新增31缺證據負例、兩CLI PASS。workplace/post-click-publish-328-backlink-tests.txt SHA-256 aab8c33aa44a17a15d66de7d707d38eff4f1da3eef72a4abd203c2fbd123d9b7。CPU／平台與VBE服務來源逐位元保持，gofmt通過，所有新來源／本機收據1000:1000，工具root-owned／誤建.md目錄自檢空。
