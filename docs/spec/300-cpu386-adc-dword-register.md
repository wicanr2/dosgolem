# 300：dword 暫存器的帶進位加法

狀態：**CONFORMED**
日期：2026-10-02
範圍：cpu386裸13 /r、mod11，八來源／八目的dword暫存器；不擴張其他ADC或記憶體形狀。

## 原始定位與公開 CPU 契約

工具基線a44c7eaae7f4ac3143a04f183f62ecf91190e124，固定官方1.31 ORION2.EXE SHA-256 4e11be14217b4aafa1839f333bf5eba037f98b0c44e9e4752c96c464c260419f；417根檔及ZIP／patch／MOX.SET與自然命令沿 [298-cpu386-shl-dword-register-one.md](298-cpu386-shl-dword-register-one.md)。已證實：兩自然outer_step42349111在外層0x2571C9的IRQ0呼叫內停高位LE0x25179F，bytes13 ED 03 34 AD 40 2D 27 00 0F BF E8 01 2F 0F BF，13 ED為ADC EBP,EBP。錯誤後態不當完整前態，後文另保存真正唯讀前態。

[Intel80386 ADC](https://www.ardent-tool.com/CPU/docs/Intel/386/manuals/prref386/ADC.htm)及[Intel SDM2A](https://www.intel.com/content/dam/www/public/us/en/documents/manuals/64-ia-32-architectures-software-developer-vol-2a-manual.pdf)印刷頁3-26／3-27定義13 /r的dword ADC：目的＋來源＋原CF，取低32位寫目的。CF為unsigned進位，OF為signed溢位；SF／ZF／PF依結果，AF按低四bit進位。六算術旗標皆定義，其他旗標保持。

## 候選實作與驗收

- 只接受裸13 /r、mod11。完整ModRM取得後，以原目的／來源／CF一起計算；來源等於目的時讀舊值，不先寫再讀。33位總和驗CF，原兩操作數與最終結果驗OF／AF，不能用兩次add32覆蓋第一次進位。
- 其他R／段／FPU／記憶體及非算術旗標保持；截短、66／67／segment／REP／LOCK前綴與記憶體形狀拒絕，解碼EIP可前進，不聲稱硬體例外restart。既有ADD／SBB／其他路徑保持。
- 64來源／目的組合、相同暫存器別名、全部byte輸入對／兩CF、每bit與補集／符號／繞回／AF邊界及算術初旗標，以uint64總和／signed整數範圍／低byte bit計數獨立oracle驗六旗標與完整保持；未知拒絕與既有ADD等回歸。
- 真正IRQ0完整前態及公開契約審查READY後才實作。全部CPU、固定EXE全套、兩自然ADC後態與真正下一資料消費後才限定CONFORMED；消費缺件保持READY。
- 原始bytes顯示下一0x2517A1的03 34 AD 40 2D 27 00把EBP作×4索引，讀DS:[EBP*4+00272D40]加至ESI，0x2517A8的0F BF E8才覆寫EBP。須驗原版實際索引／完整dword來源與ESI結果，不把之後覆寫EBP冒稱ADC消費。

## 停止線

僅補標準CPU缺件；最多四個固定位址各三筆的唯讀StepHook完整R／段／flags／IRQ狀態／既有八byte來源與64bytes，原樣轉送既有hook，不替換CPU／Bus／橋接、不跳指令或補時計。不深入IRQ0 handler／ISR／driver硬體時序、runtime或圖形helper，不猜欄位用途。主庫玩法RE閘門不變；255／主選單／正常玩家路徑、保護模式PCM／IRQ7、人耳／受控亂數與整款remake另驗。原始素材及完整終端／記憶體／gzip／PNG留本機。


## 唯讀初態與 READY 審查

已證實：未改CPU SHA-256 7cb0cade0c7b66adc37e01d458f9f22a1a57e2112afa03e62c91417d3a8a2f7c，四位址有限唯讀probe 1e3eaa8993f7a070ccfe9d9ef49b739a669647fe70b9cc3e215537d26f9c9da1。Go1.24.13／600秒／2GiB／2CPU／128pids／UID:GID1000:1000／network none，fresh417原檔與固定1.31 EXE，沿298兩自然命令只換300-input輸出，不跑全套且不修改CPU。兩gzip SHA-256 bcceec1caa5de96ab5d4278d96a042a0f930000ca71d30efe79183aaf26b6a72／acb8987441fc587a5f4d8d507b0f7e4e676cf71510602ef9c2e42ebc4f7f67fe。

兩自然各一筆真正IRQ0前態相同：outer_step42349111，高位LE0x25179F，完整R為0 0 3D69C0 0 2723D8 0 71E1D0 3C6038、段8 188 188 0 20 188、flags847h。IRQ0 active=true／failed=false／started6174／completed6173；DS:00272D40／DS:00272D44連續八bytes=00000000 02000000。只讀資料與既有hook原樣轉送，不改時計、橋接或CPU狀態。

公開13 /r六定義旗標／來源目的別名／保持與拒絕契約，真正原始bytes／完整前態與下一索引消費已足夠，300審查READY才實作。預期ADC讀兩次舊EBP0與原CF1，EBP=0+0+1=1、flags2；其他完整R／段保持。下一原版03 34 AD 40 2D 27 00以1×4索引DS:00272D44，讀完整dword2加至ESI，預期ESI=0071E1D2h／flags6，目的外保持。0x2517A8的MOVSX EBP,AX覆寫EBP在索引消費之後，不能取代前一ADD驗收；兩自然真正後態與完整來源／結果消費仍待驗。


## 受限實作與真正索引消費

限定CONFORMED：裸13 /r、mod11以原兩dword操作數與原CF一起計算，六算術旗標皆定義；其餘R／段／FPU／記憶體與非算術旗標保持。完整ModRM取得後才發布，來源目的相同時消費兩次舊值。未知前綴／記憶體／其他ADC及截短拒絕，解碼EIP可前進；ADD／word ADD／SUB／CMP／SBB回歸保持。

全部64來源／目的組合，兩CF、byte輸入配對、77項bit／補集／符號／繞回／低nibble邊界與64算術初旗標，以unsigned總和／signed範圍／低四bit進位與PF位元計數獨立核對。來源目的相同的合法初態只有兩值相同，不假造不同初值。完整原始R／段／flags與隔離索引ADD／MOVSX、保持／拒絕與既有指令回歸已驗。CPU SHA-256 b7407d43ddaf3eb8d20732acd64f2c7cfb3933924615b29cf7e7e32c7c99c021；新測試84a5e771b5e9efbadcd37dfad0fdbbb6b30eb7b21eab79615b145777834643d6；probe仍1e3eaa8993f7a070ccfe9d9ef49b739a669647fe70b9cc3e215537d26f9c9da1。

全部CPU首次通過：GOMAXPROCS=2 go test -buildvcs=false ./internal/cpu386 -count=1 -v，moo2-300-cpu-tests.txt SHA-256 1909949178aad7344e64788a254c4141990a4e2fcea26c90462efac491c75513。固定EXE全套首次通過：DOSGOLEM_MOO2_EXE=/tmp/game/ORION2.EXE go test -p 2 -buildvcs=false ./... -count=1，full-test-300.txt SHA-256 f8de03a799e482ecb7490ca8876bf9f9b0198c41da5f1ce607c126b828d65302。

沿用golang:1.24-bookworm、Go1.24.13 linux/amd64，映像ID sha256:1a6d4452c65dea36aac2e2d606b01b4a029ec90cc1ae53890540ce6173ea77ac；600秒／2GiB／2CPU／128pids／UID:GID1000:1000／network none，fresh417原檔與固定1.31 EXE。沒有CPU狀態／時計／IRQ橋接／資料或亂數注入。兩自然命令：

```text
DOSGOLEM_MOO2_MAX_STEPS=50000000 DOSGOLEM_MOO2_SEPARATE_DOS=1 DOSGOLEM_MOO2_VBE_PNG=/src/workplace/moo2-vbe-300-full-game.png go run -buildvcs=false ./workplace/moo2-probe /tmp/game/ORION2.EXE --game-dir /tmp/game
DOSGOLEM_MOO2_MAX_STEPS=50000000 DOSGOLEM_MOO2_SEPARATE_DOS=1 DOSGOLEM_MOO2_MOUSE_EVENT_AFTER_POSITION=1 DOSGOLEM_MOO2_VBE_PNG=/src/workplace/moo2-vbe-300-mouse-event.png go run -buildvcs=false ./workplace/moo2-probe /tmp/game/ORION2.EXE --game-dir /tmp/game
```

已證實：兩自然各三組、每組四筆完整ADC→索引ADD→MOVSX鏈完全相同；各自最多三次有限唯讀觀測，不因診斷截斷宣稱總呼叫數。全部outer_step42349111，IRQ0 active=true／failed=false／started6174／completed6173、段8 188 188 0 20 188，來源八bytes保持00000000 02000000。第一前態與未改CPU兩自然相同，後兩組完整R為0 80000000 3D69C0 0 2723D8 0 71E1D2 3C6040及0 0 3D69C0 0 2723D8 0 71E1D2 3C6048。

| 組別 | 0x25179F 原CF／flags | 0x2517A1 ADC後EBP／flags | 索引與完整dword | 0x2517A8 ADD後完整ESI／flags |
| --- | --- | --- | --- | --- |
| 一 | 1／847h | EBP=1／flags2 | DS:00272D44 dword=2 | 完整ESI=0071E1D2h／flags6 |
| 二 | 0／86h | EBP=0／flags46h | DS:00272D40 dword=0 | 完整ESI=0071E1D2h／flags6 |
| 三 | 1／847h | EBP=1／flags2 | DS:00272D44 dword=2 | 完整ESI=0071E1D4h／flags6 |

ADC目的外完整R／段保持，ADD只更新ESI與六旗標；下一0x2517AB的MOVSX EBP,AX將EBP覆寫為0，ESI／其他R／段與flags6保持。這是兩個不同索引的原版真正來源消費，不把之後的EBP覆寫當ADC消費。兩自然gzip SHA-256 039cae78c78a4cacd0371db655eee68c0ed1a8e537a70b10bab54ec56fc51e11／aa7729a378fdee9cc98defc0c18991e1579843de64e5261576ce4fcbeb363083；獨立unsigned／signed／nibble／PF計算與完整保持、索引0／1及來源dword核對通過。

兩自然同outer_step42349111在外層0x2571C9的IRQ0呼叫內轉停高位LE0x24678C，bytes66 83 F7 01 57 50 E8 6A BA 00 00 A1 EC 15 2B 00，word XOR DI,1缺件。內層解碼錯誤CS:EIP0008:0024678F、R為325048 3D6978 3D69C0 1 272400 FC4 0 0、段8 188 188 0 20 188、flags46h；下一真正前態仍需有限唯讀，錯誤後態不當前態。IRQ0 active=false／failed=true／started6174／completed6173，完整返回未知。

PIT mode2／Reload5966／Generation5，BIOSClock Micros44005445／Deliveries6194／Pending=true／InService=false，等待DS:00271148仍4579；工具時鐘不是硬體wall-clock證據。C6實模式333步成功返回／來源保持，保護模式連續PCM／IRQ7未知。VBE Bank7／StartY512／BankSets447／Writes4353364／DisplaySets7，兩PNG同先前已檢視黑圖SHA-256 1610444d26adb3135e7e933dd44912044bb636728af59c70e614945278d3c622。事件排程第1612067步注入x657／y189，255／主選單／正常玩家路徑、人耳／受控亂數與整款remake未驗收。299的原版自然OF=1同狀態限制不因此解除。

## 解析回填

CPU契約鍵：cpu386裸13／/r／mod11／32位暫存器／原CF；原版停點鍵：固定EXE雜湊＋高位LE0x25179F＋13 ED。293–299保存「dword ADC停點已由規格 300 接通」及本檔連結，原有CPU／平台證據範圍不擴張。驗證入口 apps/moo2/tools/startup_probe_131.py --check-adc-dword-register-spec-backlinks，缺原始定位／六旗標／真正索引ADD來源或完整結果／任一舊標記須拒絕。主庫玩法閘門不變，原始素材及完整終端／記憶體／gzip／PNG留本機。
