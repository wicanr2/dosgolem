# 310：MOO2 保護模式 DOS AH2Ah 日期服務

狀態：**CONFORMED**
日期：2026-10-03
範圍：DOS平台日期契約與明示的可重播啟動輸入，platform-spec approximation；主庫玩法RE閘門不變。

## 原版停點與來源

工具基線f6bf96a976fb31e19b19545ae438b3abcb2006fa，固定官方1.31 ORION2.EXE SHA-256 4e11be14217b4aafa1839f333bf5eba037f98b0c44e9e4752c96c464c260419f；417原檔／ZIP／patch／MOX.SET553bytes與Go1.24.13 linux/amd64、全部有效收據沿[309-sb16-pause-resume-dma8.md](309-sb16-pause-resume-dma8.md)。

已證實：兩48M正常Esc排程第48796894步高位LE0x240A32的CD 21拒絕；完整R為2B2AA8 0 0 2BDBA8 2BDB88 2BDBDC 2B2151 274090、六段8 188 188 0 20 188、flags216h，AH2Ah。後16bytes為CD 21 66 81 E9 6C 07 88 C5 C1 E1 10 66 89 D1 89。下一EIP240A34不是成功返回。兩時計58553364，IRQ0完成8022／IRQ7完成303。未解日曆語意前不猜年份、weekday或控制玩家資料。

公開來源：[Microsoft MS-DOS Version 3.3 Programmer’s Reference，Get Date Function 2AH](https://www.pcjs.org/documents/books/mspl13/msdos/dosref33/)：CX年1980–2099、DH月1–12、DL日1–31、AL星期0星期日至6星期六。這是作業系統目前日期，來源須由外部初始狀態明示。保護模式高16位與未指定暫存器／flags保持是本平台近似，不冒稱DOS/4GW硬體exact。

## 候選契約

- MOO2StartupDOS.SetCalendarEpoch(year,month,day)是明示外部設定，Gregorian有效日期1980-01-01至2099-12-31；只在AttachMachine成功且BIOS／device兩時計為0時設定。無設定、clock缺件／不同步、無效日期、非MOO2、已執行／重複設定或超出日期範圍拒絕，失敗保持全部客體核心與記憶體。
- epoch為virtualMicros0的零時日期；AH2A讀取epoch＋floor(Micros/86400000000)日，重複查詢不自行加日／改時鐘，不讀主機time.Now、不從檔案mtime／ZIP或原版遊戲欄位猜日期。有效服務只改AL、CX／DX低16位，其餘R／六段／EIP／flags／FPU／客體記憶體／時鐘保持。此切片只接正常protected INT21；實模式DOS日期轉接不擴張。
- 跨日Gregorian／weekday採Go標準time.Date／AddDate與公開DOS欄位契約，不研究DOS核心。最晚日期之後拒絕而不回繞。現有AH2C的呼叫計數近似不改；它尚未有與calendar一致的系統時間契約，玩家依賴出現時另以公開規格補齊，不稱整體日期／時間或RNG exact。
- 正常探針用DOSGOLEM_MOO2_CALENDAR_EPOCH=1996-01-01，在執行前固定，兩排程同值；格式嚴格YYYY-MM-DD。這是受控平台初態，並非原版當年的日期或預設正式遊戲seed。未提供時原309停點應保持。不得把輸入值或weekday解釋成已證實原版亂數seed。
- 有界唯讀收據記錄date服務前後全部R／六段／flags／時鐘、epoch與後續最多8條原版caller／真實寫回；只記錄由原始CPU執行的結果，不幫程式跳轉或寫入遊戲欄位。
- 驗收：固定年／月／日／weekday、1980／2000 leap／2100界限、午夜前後、重複查詢、兩模式elapsed clock、來源／設定拒絕與完整核心保持；全部固定EXE全套，再兩正常同48M Esc／50M上限的明示epoch重生。前基線保持、真實2A返回、caller及下游原始資料消費／新停點才限定CONFORMED。

## 玩家鏈、未知與停止線

只補dosgolem原版平台依賴。Go remake規則、資料／UI／存檔未改；日期值與玩家／RNG的用途尚待有限caller觀測，未受控骰序不作預設驗收。255／299、完整鍵盤／主選單／玩家流程、人耳與整款remake未完成。不反組譯DOS日期內部。原版EXE／RAM／終端／PNG只留忽略workplace，提交自製原始碼與證據。

不可變鍵：固定EXE＋dosgolem高位LE0x240A32＋CD 21／AH2Ah；閉合後回填309與startup_probe_131.py護欄，較早不同DOS服務不受影響。

## READY審查

2026-10-03：公開DOS欄位契約足夠，日期來源明示由啟動設定而非猜補原版。以共用BIOS／device時計作day offset，拒絕未設定／失步／超界，能定義完整輸入、輸出及失敗保持。AH2C近似／RNG用途未知明列；不擴張主庫玩法、預設遊戲日期或原版same-state完成範圍。可實作上述保護模式服務與受控初態，正常caller／新停點與全部測試待驗。

## 正常收據與限定驗收

狀態限定CONFORMED：無預設日曆值；SetCalendarEpoch只接明示啟動日期與已附掛零時的兩時計。310的日期／跨日／設定與來源拒絕測試先通過，原版同形SUB consumer因CPU缺件失敗。有限原版診斷確認此停點，由[311-cpu386-sub-word-register-imm16.md](311-cpu386-sub-word-register-imm16.md)依公開Intel契約接線；日期測試未改預期或放寬斷言。探針第一版loopStep宣告順序編譯失敗，實際編譯收據保留，修自製觀測程式後同隔離自然命令重跑。這些不列原版缺陷。

Go1.24.13 linux/amd64，golang:1.24-bookworm映像sha256:1a6d4452c65dea36aac2e2d606b01b4a029ec90cc1ae53890540ce6173ea77ac；外層600s、2GiB／2CPU／128pids、UID:GID1000:1000、network none，原版ZIP／patch唯讀、重新展開417原檔與固定EXE，沿309命令只加日曆初態；不設定私有IRQ／BIOS prototype。

```text
GOMAXPROCS=2 GOCACHE=/tmp/go-cache DOSGOLEM_MOO2_EXE=/tmp/game/ORION2.EXE go test -p 2 -buildvcs=false ./... -count=1
DOSGOLEM_MOO2_CALENDAR_EPOCH=1996-01-01 DOSGOLEM_MOO2_HARDWARE_ESCAPE_AT_48000000=1 DOSGOLEM_MOO2_MAX_STEPS=50000000 DOSGOLEM_MOO2_SEPARATE_DOS=1 DOSGOLEM_MOO2_VBE_PNG=/src/workplace/moo2-vbe-311-full-game.png go run -buildvcs=false ./workplace/moo2-probe /tmp/game/ORION2.EXE --game-dir /tmp/game
DOSGOLEM_MOO2_CALENDAR_EPOCH=1996-01-01 DOSGOLEM_MOO2_HARDWARE_ESCAPE_AT_48000000=1 DOSGOLEM_MOO2_MAX_STEPS=50000000 DOSGOLEM_MOO2_SEPARATE_DOS=1 DOSGOLEM_MOO2_MOUSE_EVENT_AFTER_POSITION=1 DOSGOLEM_MOO2_VBE_PNG=/src/workplace/moo2-vbe-311-mouse-event.png go run -buildvcs=false ./workplace/moo2-probe /tmp/game/ORION2.EXE --game-dir /tmp/game
DOSGOLEM_MOO2_HARDWARE_ESCAPE_AT_48000000=1 DOSGOLEM_MOO2_MAX_STEPS=50000000 DOSGOLEM_MOO2_SEPARATE_DOS=1 DOSGOLEM_MOO2_VBE_PNG=/src/workplace/moo2-vbe-311-unconfigured.png go run -buildvcs=false ./workplace/moo2-probe /tmp/game/ORION2.EXE --game-dir /tmp/game
```

| 自製來源／本機收據 | SHA-256 |
| --- | --- |
| internal/machine/le_dos_calendar.go | 5b776d5a095b1af3ad9176c751eb0cb7e31d8dcc06b10ef9fa263be024c4f50e |
| internal/machine/le_startup.go | be64b3d75465f4a80f60fcd60cd64ae8b9b68703b618360ae58a185d3e2687e6 |
| internal/machine/le_dos_calendar_test.go | bacad5a9335e430252b734a13d8ef5f6dde673c04057b3b38d73f1b47bead507 |
| workplace/moo2-probe/main.go | 5da905aa2f100aac109329608e010975313709b8900e19360965c19db029b2af |
| workplace/moo2-310-calendar-tests.txt，原同形SUB consumer失敗 | 983222ff851492f188acf22a636b1e875acb4f00335ac272eb353d8a212d4031 |
| workplace/moo2-310-calendar-tests-diagnostic.txt，失敗細節 | eb3ad2565b8b40972a865c0479d87378b5fa547c4204a116fa0dde97bf7087a6 |
| workplace/moo2-310-draft-compile-failure.txt，觀測程式編譯失敗 | 183e9b12787b27c617c24a6b63a6c1ea20aeb825bb563db1501e8167b9ad3321 |
| workplace/moo2-311-sub-calendar-tests.txt，六測試通過 | 8acc8abf35de1678e4c26994c3349036e373c0b7eb4fc955cd83d19c526256b2 |
| workplace/full-test-311.txt，全部CPU／固定EXE全套通過 | ee661ea0e8fb6e464337402fbeb5a15a009a5a9c4c664abc8d313550b12978b2 |
| workplace/moo2-probe-311-full-game.txt.gz | 950f0e6690aad7f7546e2dcdcfb8ddd6aeeb4b398aa001c14a483b359d5ba1f8 |
| workplace/moo2-probe-311-mouse-event.txt.gz | 09b176610ba23b5f6b221564659a3666ed7bd8d2589b2f984a86e3e66ab4bc79 |
| workplace/moo2-probe-311-unconfigured.txt.gz | 5f736d9fa2d296bc138ba0782212e0b775e78d6a258fb569c1d2dcb57eed2589 |

已證實，正常入口使用明示受控日期：兩排程日期前完整309基線保持，只排除mtime／DTA四bytes與新增calendar_initial收據。實際AH2A兩筆如下；不是從日期輸入直接注入遊戲記憶體。

| 外層步數／高位LE | 共用Micros | 真正返回欄位／保持 |
| --- | --- | --- |
| 48796894／0x240A32 | 58553364 | EAX002B2AA8→002B2A01、CX0000→07CC、DX0000→0101；其他完整R／六段與flags216h保持 |
| 48796930／0x240A96 | 58553400 | EAX00002A05→00002A01、CX0000→07CC、DX0500→0101；其他完整R／六段與flags246h保持 |

兩次相隔36µs且沒有跨日，所以同為1996-01-01星期一；真正SUB、MOV CH,AL、SHL ECX,16與MOV CX,DX形成01600101h，於0x240A41／0x240AA5由原始MOV真正寫至SS:002BDB90／SS:002BDB8C，前後32byte窗口證實只對應四bytes變成01 01 60 01。每筆後8條原版caller共16條完整R／段／flags與可讀堆疊已驗；其他日期消費、caller中的記憶體欄位與RNG用途不猜名稱。完整SUB與初次寫回證據集中311，shift未定義旗標沿既有工具近似。

未設定日期的第三正常流程仍在原48796894步0x240A32拒絕，全部R／六段／flags保持；除了唯讀拒絕收據與PNG檔名，整段逐列等於309-full-game。CalendarState configured=false時epoch空與micros0是未配置占位，實際終態裝置／BIOS時計仍58553364，不能把該占位當時計停止。沒有悄悄選日期或以當前主機時間、重試挑值通過。

兩設定日期的自然第48797763步停高位LE0x210C7E，bytes66 03 05 A4 BE 29 00 66 A3 A2 BE 29 00 BB 00 01，word ADD記憶體來源尚未支援。完整R為F 0 2BDCDC 8 2BDBD4 2BDBE0 284324 2BDCA4，段8 188 188 0 20 188、flags202h；下一EIP210C81只為解碼，未運算成功。兩時計58554306，IRQ0 completed8022／failed=false，IRQ7 started304／completed304，DMA完成304／剩2011／credit25200。三PNG均1610444d26adb3135e7e933dd44912044bb636728af59c70e614945278d3c622，與已檢視黑色過場逐位元相同，主選單未見。

310限定CONFORMED只解明示日曆與公開日期服務、正常返回／有限原始caller與寫回。AH2C原呼叫計數近似未改善，系統日期／時間一致性、RNGseed、正式玩家初態與原版same-state／主選單及整款remake未驗收。下一步只核對0x210C7E的word ADD來源邊界／flags與下個66 A3原始寫回，不追runtime helper內部。

## 解析回填

DOS AH2Ah日期停點已由規格 310 接通。309須保留此標記與本檔連結，限定明示epoch與正常返回，不讓舊日期服務拒絕當現行已設定流程的阻塞。word SUB完整立即值停點已由規格 311 接通，見[311-cpu386-sub-word-register-imm16.md](311-cpu386-sub-word-register-imm16.md)；初始310診斷與失敗測試的SUB拒絕屬當時CPU基線，不重寫歷史。驗證入口apps/moo2/tools/startup_probe_131.py --check-dos-calendar-spec-backlinks；缺原始定位、明示初態、正常日期／寫回、未設定拒絕、收據或舊回填必拒絕。

全部53個回填函式與45個新增缺證據／較早標記與連結移除負例、兩個CLI通過。既有309索引正對照及310／311入口可查，受驗日曆／啟動／CPU／測試／探針來源SHA-256未變。本批三正常流程與全套Go容器均已正常退出移除，未清理其他專案或映像。

## 2026-10-03 ADD來源停點勘誤

word ADD記憶體來源停點已由規格 312 接通，見[312-cpu386-add-word-memory-source.md](312-cpu386-add-word-memory-source.md)。同一固定EXE／高位LE0x210C7E，來源0002h＋AX000Fh、flags216h及真正DS0188:0029BEA2兩byte寫回已驗。原拒絕收據保留；兩明示日期正常流程現在停0x14E3DE word CMP完整立即值，已見原版Loading畫面，其他日期／時間／RNG、音訊、玩家流程與本規格範圍未擴張。

## 2026-10-03 CMP完整立即值停點勘誤

word CMP完整立即值停點已由規格 313 接通，見[313-cpu386-cmp-word-register-imm16.md](313-cpu386-cmp-word-register-imm16.md)。固定EXE／高位LE0x14E3DE的六旗標與後續JL兩方向、兩正常外層212到訪已驗。原拒絕收據保留；新停點0x24C31B的INT33／AX0014h滑鼠服務，已見原版標題背景，主選單按鈕未見，其他CPU／日期／音訊／玩家路徑範圍未擴張。

AX0014h交換停點已由規格 314 接通，見[314-moo2-protected-mouse-callback-exchange.md](314-moo2-protected-mouse-callback-exchange.md)。同一固定EXE的24次交換與前三次C3返回已由正常入口重生；舊拒絕與本檔原範圍保持。兩流程目前到50M上限、高位LE0x23856E，主選單面板部分滑入，完整主選單／玩家路徑未驗。
