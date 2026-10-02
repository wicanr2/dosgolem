# 324：byte ADD的暫存器與記憶體目的

狀態：**CONFORMED**（僅無前綴00 /r與下列正常消費）
日期：2026-10-03
範圍：通用CPU386的無前綴00 /r；不改MOO2或主庫玩法。

## 原始定位與公開契約

固定官方1.31 ORION2.EXE SHA-256 4e11be14217b4aafa1839f333bf5eba037f98b0c44e9e4752c96c464c260419f。工具e75f5aebed41cccb062609235dd2f0b07ffae371，原ZIP417根檔、MOX.SET、44M Esc／1996-01-01／50M cap與單次正常NEW GAME輸入沿[323-moo2-dos-findfirst-question-pattern.md](323-moo2-dos-findfirst-question-pattern.md)。私有workplace/moo2-probe-323-click.txt.gz SHA-256 62b3344c95ed7215ff3602d4399129ba5e58d923f89585ecb98271d856e78ec3，49442083步在dosgolem高位LE17122B拒絕00 C3 0F BF C2 42 00 1C 06 66 83 FA 08 7D 11 EB。00 C3是ADD BL,AL；後續靜態00 1C 06是ADD DS:[ESI+EAX],BL，323時尚未執行，不從當時bytes推RAM內容或欄位用途。進入完整R0／0／1／0／2BDB50／2BDB84／2BDB68／2BDB68、六段8／188／188／0／20／188、flags247h；錯誤後EIP17122C是fetch，非指令成功。

[Intel 80386原始手冊ADD條目](https://people.freebsd.org/~jhb/386htm/ADD.htm)定義00 /r是ADD r/m8,r8，目的加來源的8位結果並定義CF／PF／AF／ZF／SF／OF；記憶體目的不可寫時例外。此平台語意直接採契約，不追遊戲helper。既有02 byte ADD暫存器、04 AL立即數、80 /0立即數及其他寬度保持；323基線00尚未接，add8與decodeAddress32已存在。

## 型別與READY候選

無前綴00 /r讀取ModR/M：mod3目的為r/m的AL／CL／DL／BL／AH／CH／DH／BH，來源為reg；先保存兩個uint8再發布結果，目的宿主其餘24位保持。mod0..2沿既有32位地址解碼含一般基址／disp8／disp32／SIB／no-base、DS與SS；來源r8不改，目的只寫一byte，鄰接byte不動。地址依既有32bit繞回契約，不放寬selector／limit／Writable／Bus安全。

結果mod256，六旗標獨立依完整和>255、低nibble進位、有號和範圍、結果符號／零／偶parity驗證。ADD不使用輸入CF。其他R／六段／FPU／非算術flags保持。完整讀取與byte目的寫回成功後才發布flags；錯誤不改R／flags／RAM，既有fetch已部分改EIP明示，不稱全指令rollback。66／67／segment／REP／REPNZ／LOCK等未審查prefix維持拒絕。

## 審查與驗收

本次00 opcode同時涵蓋暫存器與記憶體目的，避免拆開相鄰原始兩形狀。八目的／來源及同宿主高低byte別名、全部byte對／兩組初始flags、獨立算術預期、DS／SS／一般ModR/M與SIB／不對齊／地址繞回／段末byte、未知段／唯讀／越界／Bus讀寫錯誤／截短／prefix拒絕及其他ADD形狀回歸須通過。定向測試後，以固定EXE go test -p 2 -buildvcs=false ./... -count=1驗全部CPU／平台；CPU驗收須全PASS。

原版以同44M兩流程正常重生，無點擊完整323基線保持，點擊到舊拒絕前全部既有列保持，只正規化mtime／DTA日期時間四bytes與PNG路徑。新probe在17122B／171231真正CPU.Step保存最多32筆完整前後R／六段／flags／bytes／錯誤；記憶體形狀保存實際DS:offset與五byte窗口及最多三個接續外層步，最多32續行。窗口直接唯讀RAM，須核對selector與limit，不走CPU hook產生額外guest請求。真正ADD與後續MOVSX／INC／CMP／JL只按實際收據下結論，不注入指令、代寫欄位或重點／加cap；遇新拒絕另開窄任務。全部素材／LOG／PNG留忽略workplace，索引docs/spec/000-index.md；323新停點同次回填與startup_probe_131.py守衛，不改先前搜尋／CB結論。主庫玩法RE閘門保持。

READY審查：00 C3的ModR/M為mod3／來源AL／目的BL；00 1C 06的SIB為scale0／indexEAX／baseESI，沿既有DS地址框架。公開Intel契約、既有add8與byte寫入先於flags的OR框架足以實作，不需要新遊戲規則或更多helper RE。前綴及錯誤邊界已明定；正式CONFORMED仍須全部測試和相同輸入的原版收據。

## 正式收據與限定結論

Go1.24.13固定映像sha256:1a6d4452c65dea36aac2e2d606b01b4a029ec90cc1ae53890540ce6173ea77ac，600s／2GiB／2CPU／128pids／UID1000／network none，原ZIP／patch只讀，417根檔乾淨重建。定向go test -p 2 -buildvcs=false ./internal/cpu386 -run 'TestByteADD00|TestByteRegisterANDADDAndAliases|TestByteADDRegisterImmediate' -count=1 -v通過；七個新增主測試覆蓋所有byte對、兩組初始flags、64暫存器目的／來源別名、各ModR/M／SIB／DS／SS、地址繞回與段末byte、Bus及段拒絕、截短與prefix；全部R／六段／FPU／非算術flags和鄰接RAM保持。負例保留原CS取指，避免只因讀錯code而拒絕。

固定EXE DOSGOLEM_MOO2_EXE=/tmp/game/ORION2.EXE go test -p 2 -buildvcs=false ./... -count=1全套PASS，CPU386195.653s／machine6.976s；定向0.903s。原版兩44M Esc／1996-01-01／50M cap／separate DOS與單次正常NEW GAME輸入不改，先go build -p 2 -buildvcs=false -o /tmp/moo2-probe ./workplace/moo2-probe，再/tmp/moo2-probe /tmp/game/ORION2.EXE --game-dir /tmp/game。輸出prefix為moo2-324-baseline-frame／moo2-324-click-frame與moo2-vbe-324-baseline.png／click.png，精確其他環境沿323。無點擊全部323原始列／終圖保持；點擊到舊失敗快照前4089列保持，僅mtime／DTA時間日期四bytes／PNG路徑正規化。第一ADD完整輸入另與舊拒絕核心核對，未刪除真實差異。

首次稽核切點選guest_cpu_stop，錯把此前四個失敗後快照加入「停點前」比較。第一差異正是舊late_startup_platform label=stop，非來源或正常流程退步。切點改到首個失敗後快照前，另核對第一ADD完整R／六段／flags，乾淨重跑通過；CPU／平台／原版來源及正式收據不改。

**已證實，dosgolem高位LE正常原版指令**：
- 49442083，17122B／00 C3到17122D，完整R0／0／1／0／2BDB50／2BDB84／2BDB68／2BDB68與六段保持，AL0＋BL0→BL0，flags247h→246h。六旗標CF0／PF1／AF0／ZF1／SF0／OF0。
- 49442084，17122D／0F BF C2是MOVSX EAX,DX，只EAX0→1，flags246h保持；49442085的171230／42只EDX1→2，flags246h→202h。
- 七次171231／00 1C 06到171234，outer49442086／49442092／49442098／49442104／49442110／49442116／49442122；實際DS188:[ESI+EAX]偏移2BDB69..2BDB6F，來源BL皆0。目的byteD5／D4／D5／D5／D5／D5／D5與各五byte窗口原值保持，六旗標按獨立加法與parity逐筆核對。第一flags202h→282h，第二207h→286h，其餘→282h。這次原版全部是加0，沒有原版非零寫入樣本；非零值由公開契約與完整單元測試驗證，不冒稱另一個遊戲初態。
- 24續行全部error=nil，包含七個171234／66 83 FA 08的CMP DX,8與七個171238／7D 11的JGE。CMP六旗標、完整R／段與RAM保持核對；JGE六次不跳、一次跳，最後DX8時到17124B，非只驗單一分支。未替原版欄位或helper猜名。

兩mouse CB保持191步、started2／completed2、pending0／activefalse。正常流程再前進59052步，在49501135新拒絕：高位LE2130F3／F7 5D D8 8B 45 D8 66 3B 45 E0 0F 8D A8 00 00 00，F7／3記憶體NEG尚未支援；fetch後EIP2130F5不代表NEG成功。完整RFFFFFFFF／498AC0／8／47／2BD9B8／2BD9F4／495230／2BDACC，六段8／188／188／0／20／188，flags286h；時計64507253、IRQ7完成432。依公開ModR/M形狀來源為SS:[EBP-28h]，此時SS188:2BD9CC，來源RAM未另取樣，仍未知。

| 本機忽略收據 | SHA-256 |
| --- | --- |
| workplace/byte-add-324-tests.txt | a48db475b0157766892c6a47f7d333682b4575457fcefa954df4b7968f5a0073 |
| workplace/full-test-324.txt | dd00bcf329a9841fa4810d94a30f90c76245bcee66c0718c6073a6862464656a |
| workplace/moo2-probe-324-baseline.txt.gz | 6130a9fe5bf084b2128c700c9e10fb821d43bd6ee8a474933015e8272d7eb0e1 |
| workplace/moo2-probe-324-click.txt.gz | c9e8b2d3d15c4b8a64fb9ff497513b2a294b67b9a682b5584dc9156d873bf1e3 |

無點擊PNG仍11ec0ed15a4c874d36c94a824af73eb71dc6937dfe3bd568450943861db89927；點擊PNG仍59f76749db5232f97a6b6f969f56f848c2e89e731d7e3fb30b8786982bca4a81，與已實際檢視的323兩圖逐位元相同，仍主選單，設定畫面仍未知。沒有正常開局或remake同狀態收據，不把標準CPU能力當玩法完成。

CPU SHA-256 d3fd7c1125d6ecda2fbc05f021776a0532a820af6babea1f3693dc6608aace4e；probe118203d32391772177dd96e6da2539886618a73bb76f14ec8e161399dc98a985；新增測試10b6297576161f93de3c7f9b9dc6388b02e2fa1c1d985ff2404a22c8923bc58a。DOS startup、read-only provider、問號matcher與323保持。

限定CONFORMED只含無前綴00 /r契約與上述真正ADD／最小消費。未支援prefix、記憶體NEG、正常新遊戲與主庫玩法RE閘門保持；下一步依公開NEG契約先READY、實作、固定EXE全套、同44M原版正常輸入重生，記實際SS來源／目的與最小caller，不追helper內部、不加cap／重點或代寫狀態。

61回填函式、原有32／49負例與324新增25負例及兩CLI通過。私有workplace/byte-add-324-backlink-tests.txt SHA-256 7e134f37db5edeb23ee21a6258a1e5f0655654d2d29663080460d4ed43f1554b；缺原始輸入、窗口、兩方向、正式收據、未知邊界、323勘誤或索引均拒絕。新檔與收據UID:GID1000:1000，工具root-owned／誤建.md目錄自檢空。本輪首次唯讀Docker呼叫的自動核准審查逾時未建立程序，依回報重試一次成功；不是工具或產品失敗。

## 2026-10-03 記憶體NEG後續

記憶體NEG停點已由規格325接通：[325-cpu386-neg-dword-memory.md](325-cpu386-neg-dword-memory.md)。324當時來源未取樣的歷史收據保留；325正常SS取樣三次FFFFFFFFh→1h與九步MOV／CMP／JGE不跳已證，設定畫面仍未知。同44M／50M流程已到上限，沒有新CPU拒絕。現行下一步為相同輸入的後段唯讀進度／畫面觀測，未加cap或重點。
