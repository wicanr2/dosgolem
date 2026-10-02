# 302：AX 與 word 暫存器的短編碼交換

狀態：**CONFORMED**
日期：2026-10-02
範圍：cpu386的66 91–97，AX與七個word暫存器交換；不擴張裸dword短編碼、66 90或87 word暫存器／記憶體形狀。

## 原始定位與公開 CPU 契約

工具基線d013029fcddf4da8e8d7650660897223e6fa67ca，固定官方1.31 ORION2.EXE SHA-256 4e11be14217b4aafa1839f333bf5eba037f98b0c44e9e4752c96c464c260419f。417原檔、ZIP／patch／MOX.SET雜湊與自然命令沿 [301-cpu386-xor-word-register-imm8.md](301-cpu386-xor-word-register-imm8.md)。兩自然第42603292步停根CPU高位LE0x256171，bytes66 93 C1 CB 08 C3 90 8B C2 8A E2 8B DA C1 C8 18，標準XCHG AX,BX缺件。真正完整前態與下一ROR消費須有限唯讀保存，不以解碼後態取代。

[Intel80386 XCHG](https://www.ardent-tool.com/CPU/docs/Intel/386/manuals/prref386/XCHG.htm)定義90+r可交換AX與word暫存器。以兩個原值同時交換低16位，兩側高16位保持；全部旗標保持，不沿用XOR的算術旗標更新。只接91–97七個目的，90既有NOP與未支援66 90保持原範圍。

## 候選實作與驗收

- 完整取得66及opcode後，取兩側原低word，更新AX與七目的低16位，兩高16位及其他R／段／FPU／記憶體、全部旗標保持；ESP目的只交換SP，不做堆疊讀寫。
- 截短、重複66、67／segment／REP／LOCK及裸91–97拒絕，解碼EIP可前進；87 word暫存器與66 NOP未知邊界不擴張。既有87 dword、86 byte與裸90保持。
- 七目的×全部65536 AX低word×相同與補集兩種對側低word×兩初旗標；bit／補集／符號邊界配對與64初算術旗標，獨立整除拆高低word及舊值交換oracle驗全部保持；拒絕／截短與既有指令回歸。
- 真正完整前態與公開CPU契約審查READY後才實作；全部CPU／固定EXE全套及兩自然完整XCHG後態與下一ROR真正消費後才限定CONFORMED，消費缺件保持READY。
- 下一0x256173的C1 CB 08為ROR EBX,8，消費交換後完整EBX。須確認XCHG兩側高16位、原始低16位與下一完整ROR結果；0x256176的RET只記邊界，不追helper或caller內部。ROR公開契約與多位OF保留近似沿 [297-cpu386-ror-dword-register-imm8.md](297-cpu386-ror-dword-register-imm8.md)。

## 停止線

固定三位址各最多三筆唯讀StepHook保存完整R／段／flags／IRQ狀態及64bytes，原樣轉送既有hook，不替換CPU／Bus／時計／IRQ橋接、不注入資料或跳指令。只跨過標準CPU缺件，不追helper／runtime／driver／ISR或硬體wall-clock，不猜欄位用途。主庫玩法RE閘門不變；301只證明已驗自然IRQ0返回，其他分支、299原版自然OF=1、255／主選單／正常玩家路徑、保護模式PCM／IRQ7、人耳／受控亂數及整款remake仍另驗。原版素材及完整終端／記憶體／gzip／PNG留本機。

## 唯讀初態與 READY 審查

未改CPU SHA-256 37e637105219c22d92065f7c173b716667f43f91c525cc5a5bdcd2722bd6d39b，有限唯讀probe e75cb89312c6e5b1b107f93760e1da78f89b9b1bbe86fa813db5f3dfd0f1511a。沿301的Go1.24.13／600秒／2GiB／2CPU／128pids／UID:GID1000:1000／network none與fresh417原檔及固定1.31 EXE，兩自然命令只換302-input輸出，不改CPU。兩gzip SHA-256 1c034b27a6111197b8bdfd9e4491d2c9ddc806d290443309b9f9bfea18081436／3d6dc06f1a811b44e41cfdbededd3e4bac6280e1d2f525ee2e301a325144b535。

已證實：兩自然各一筆真正前態相同，outer_step42603292、高位LE0x256171，完整R依EAX ECX EDX EBX ESP EBP ESI EDI為A0A0A2E 2E0A40C0 2E0A2E0A 2E0A0A0A 2BDB6C 6EE4 70E2D2 3471A0、段8 188 188 0 20 188、flags206h；IRQ0 active=false／failed=false／started6228／completed6228。這是執行前StepHook保存的完整初態，兩種排程保持既有原版路徑與受控滑鼠事件條件。

[Intel SDM2C](https://www.intel.com/content/dam/www/public/us/en/documents/manuals/64-ia-32-architectures-software-developer-vol-2c-manual.pdf)印刷頁5-586／5-587確認90+rw交換AX與r16、全部旗標不受影響。66 90在硬體是NOP別名，本工具仍未支援；本輪不把工具拒絕邊界誤稱硬體非法。

完整初態、原始bytes與公開word交換／保持契約已足夠，302審查READY才實作。預期交換兩原低word後完整EAX=0A0A0A0Ah、完整EBX=2E0A0A2Eh，兩高16位、其他R／段及flags206h保持。下一ROR EBX,8以完整2E0A0A2Eh逐次循環8位，預期完整EBX=2E2E0A0Ah／flags206h；多位OF未定義保留只屬297工具模型。兩自然真正後態與ROR完整消費仍待驗。


## 受限實作與完整 ROR 消費

限定CONFORMED：66 91–97的七個word對側，取兩個原低word後交換，高16位／其餘R／段／FPU／記憶體及全部旗標保持；ESP目的只交換SP。截短／重複66／其他前綴／裸dword短編碼及未審查87 word暫存器拒絕，解碼EIP可前進。既有裸90、86 byte／87 dword保持，66 90仍屬工具未知範圍。

七目的×全部65536 AX低word×同值／補集兩對側值×兩初旗標，45項bit／補集／符號／繞回邊界配對×64算術初旗標，以整除拆兩原高低word並交叉組合的獨立oracle驗完整保持。另驗全部32旗標bit及補集原樣保存，這是CPU狀態保存驗證。原版完整初態與下一ROR逐次整除循環8位消費、拒絕／截短及既有byte／dword／NOP回歸已驗。CPU SHA-256 72e5ee4954b0b4616e32b3b3844b6ee8836b210c0746cd4b8a20c0b9b5113326，新測試089bfb445c7ef18b8dcc85baeec62b49ba0fcb564b364745335a89d8dae2fcec；probe仍e75cb89312c6e5b1b107f93760e1da78f89b9b1bbe86fa813db5f3dfd0f1511a。

全部CPU首次通過：GOMAXPROCS=2 go test -buildvcs=false ./internal/cpu386 -count=1 -v，moo2-302-cpu-tests.txt SHA-256 d05045f43ddfdf2021e0a6eee727ac4463aa7fe29310821169828fdd1ba75dbb。固定EXE全套首次通過：DOSGOLEM_MOO2_EXE=/tmp/game/ORION2.EXE go test -p 2 -buildvcs=false ./... -count=1，full-test-302.txt SHA-256 263845c9b6cd1109b941dcf14740b3b19926bb0b4e94385d9073271f3b99f8cc。

沿用golang:1.24-bookworm／Go1.24.13 linux/amd64、映像ID sha256:1a6d4452c65dea36aac2e2d606b01b4a029ec90cc1ae53890540ce6173ea77ac；600秒／2GiB／2CPU／128pids／UID:GID1000:1000／network none，fresh417原檔及固定1.31 EXE，不注入CPU／時計／IRQ橋接／資料或亂數。兩自然命令：

```text
DOSGOLEM_MOO2_MAX_STEPS=50000000 DOSGOLEM_MOO2_SEPARATE_DOS=1 DOSGOLEM_MOO2_VBE_PNG=/src/workplace/moo2-vbe-302-full-game.png go run -buildvcs=false ./workplace/moo2-probe /tmp/game/ORION2.EXE --game-dir /tmp/game
DOSGOLEM_MOO2_MAX_STEPS=50000000 DOSGOLEM_MOO2_SEPARATE_DOS=1 DOSGOLEM_MOO2_MOUSE_EVENT_AFTER_POSITION=1 DOSGOLEM_MOO2_VBE_PNG=/src/workplace/moo2-vbe-302-mouse-event.png go run -buildvcs=false ./workplace/moo2-probe /tmp/game/ORION2.EXE --game-dir /tmp/game
```

已證實：兩自然各三組、每組三筆完整XCHG→ROR→RET邊界相同，第一完整前態與未改CPU收據一致；各位址最多三筆，不把觀測上限當總呼叫數。三組前態步數42603292／42616626／42621546，三筆各隔一outer_step；IRQ0 active=false／failed=false，各組started／completed為6228／6231／6232且兩值相等。段8 188 188 0 20 188，全部flags206h。

| 組別 | 0x256171 原完整EAX／EBX | 0x256173 XCHG後完整EAX／EBX | 0x256176 ROR後完整EBX |
| --- | --- | --- | --- |
| 一 | 0A0A0A2Eh／2E0A0A0Ah | 0A0A0A0Ah／2E0A0A2Eh | 2E2E0A0Ah |
| 二 | 0A0A0A10h／100A0A0Ah | 0A0A0A0Ah／100A0A10h | 10100A0Ah |
| 三 | 0A0A0A10h／100A0A0Ah | 0A0A0A0Ah／100A0A10h | 10100A0Ah |

第一XCHG後完整EAX=0A0A0A0Ah、完整EBX=2E0A0A2Eh、flags206h，兩高16位與其他完整R／段保持；下一ROR完整EBX=2E2E0A0Ah。後兩組不同完整來源亦驗低word交換及下一完整ROR；ROR只更新EBX與定義CF，其餘旗標保持，八位OF未定義保留依297工具模型。三組完整R／段與原值交換／逐次整除循環獨立核對通過，RET只記邊界。

兩自然gzip SHA-256 fe3472fa0d7766a761b3b5f7cc5fab8cc22a296a16d0e947e3a093a62826be87／ae21d6b88b831f10addae20471effd45d0ce0e163de3d9067d0840829f4f44e9。兩次都到step_limit=50000000 eip=0x22FCD2，完整R為FFFFFFFF 4000 0 0 2BDB64 2BDBB4 FFFFFFFF 3D6978、段8 188 188 0 20 188、flags246h，bytesFF 0D 40 8E 2A 00 89 F0 5D 5F 5E C3 57 55 8B 15；沒有step_error或guest_cpu_stop，不把診斷上限當CPU拒絕。無事件unique_sites17667，受控滑鼠事件17697；首次收據稽核誤要求兩種輸入分支覆蓋數相同，修正為各自實際值後以同一收據重跑通過，沒有重跑原版或修改CPU。

終態IRQ0 active=false／failed=false／started7789／completed7789，等待DS:00271148=33 18 00 00即6195。PIT mode2／Reload5966／Generation5、BIOSClock Micros52095937／Deliveries7809／Pending=false／InService=false；僅屬工具時間，不稱硬體wall-clock一致。C6命令／handled返回及既有成功caller／來源SHA保持，保護模式連續PCM／IRQ7與人耳未知。

VBE Bank9／StartY512／BankSets497／Writes4744640／DisplaySets7，indexed SHA-256 8c5886995a63e609e0b33bd49e49a331d7de452437485428d9919c60461fa98c；RGB SHA-256 8f688f78c811b308a51b62769add0f23e00718eda758a10e54318ef369122bf7，兩PNG同SHA-256 d648932f847a2fe5b87723b6537f76e816d64fb60e05c13321d15ede50f3b21b。已檢視本機圖像：中央有星空／星雲片段，未見主選單。固定事件第1612067步注入x657／y189／buttons0，255完整座標／游標與正常玩家操作未驗；299原版自然OF=1、受控亂數與整款remake限制保持。

## 解析回填

CPU契約鍵：cpu386 66 91–97／AX與word暫存器／兩高16位與全部旗標保持；原版停點鍵：固定EXE雜湊＋高位LE0x256171＋66 93。293–301的九份同停點保存「word XCHG停點已由規格 302 接通」與本檔連結；原有其他CPU／平台及299原版自然OF=1證據範圍不擴張。驗證入口 apps/moo2/tools/startup_probe_131.py --check-xchg-ax-word-spec-backlinks，缺定位／兩高word／全部旗標保持／真正完整ROR消費／50M上限收據或任一舊標記須拒絕。主庫玩法RE閘門不變，原始素材／完整終端／記憶體／gzip／PNG留本機。

下一步先以有限唯讀觀測確認這個正常啟動流程的等待與鍵鼠入口，再用原版正常輸入重播；不為了跨過上限而跳指令、注入玩法資料或直接猜修等待條件，不深入helper／driver／ISR硬體時序。


## 後續平台觀測

2026-10-02的 [303-moo2-late-startup-platform-observation.md](303-moo2-late-startup-platform-observation.md) 已保存兩自然各11筆有限唯讀快照；原有CPU／平台來源、完整終態、XCHG／ROR與圖像收據保持。已證實保護模式未推進音訊時計，正常鍵盤入口尚未接通；等待因果仍未知，本檔的CPU限定CONFORMED及其他限制保持。
