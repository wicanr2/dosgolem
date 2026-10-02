# 297：dword 暫存器的立即數右循環移位

狀態：**CONFORMED**
日期：2026-10-02
範圍：cpu386 裸C1 /1、mod=11，八個32位暫存器與全部imm8計數。

## 原始定位與公開 CPU 契約

工具基線56e1ff979b517740bf056668ee90800bfac27321。固定官方1.31 ORION2.EXE SHA-256 4e11be14217b4aafa1839f333bf5eba037f98b0c44e9e4752c96c464c260419f，417原檔、MOX.SET、ZIP／patch雜湊與兩自然命令沿 [296-sb16-c6-auto-init-dma.md](296-sb16-c6-auto-init-dma.md)。已證實：兩自然第42,347,639步停dosgolem高位LE線性0x257662，bytes C1 CA 10 A2 C0 26 27 00 66 8B C2 C1 CA 10 39 11。CA為/1、mod=11、EDX，完整EDX=0AFF0AFFh、flags297h；完整R／段與下一MOV消費待未改CPU的唯讀診斷。

[Intel80386 Rotate](https://www.ardent-tool.com/CPU/docs/Intel/386/manuals/prref386/RCL.htm)的C1 /1 ib為dword ROR，計數取低五位，舊CF不輸入資料。[Intel SDM2B](https://www.intel.com/content/dam/www/public/us/en/documents/manuals/64-ia-32-architectures-software-developer-vol-2b-manual.pdf)印刷頁4-524／4-525：遮罩計數0全部保持；非零CF為結果最高bit；計數1的OF為結果最高兩bit的XOR，其他SF／ZF／AF／PF保持。多位OF未定義，沿既有規格222／288保留模型，不冒稱硬體未定義值一致。

## 候選實作與驗收

- 將既有只接受08h的dword暫存器ROR擴成全部imm8遮罩計數；完整取得指令bytes才發布目的／CF與計數1 OF，零計數不發布。其他R／段／FPU／記憶體與非算術旗標保持。
- 裸C1 /1、mod=11；截短、67／segment／REP／LOCK前綴與記憶體形式拒絕，解碼EIP可前進，不稱硬體例外restart。既有D1／D3與word範圍不擴張。
- 全八目的／256計數／每bit與補集及邊界／CF與OF初態／保持旗標全清或全設，以逐次單bit整除循環的獨立oracle驗資料與定義旗標；未定義OF模型另驗。拒絕、完整原始初態與既有ROL／ROR路徑回歸。
- 未改CPU的完整初態與公開契約審查READY後才實作。全部CPU、固定官方EXE全套、兩自然完整後態及原版MOV AX,DX真實消費後才限定CONFORMED；消費缺件保持READY。

## 停止線

只跨過阻塞的標準CPU形式，不追圖形helper、runtime、driver／ISR／busy-wait或猜欄位用途。主庫玩法RE閘門不變；255座標／游標、主選單／正常玩家路徑、保護模式PCM／IRQ7、人耳與受控亂數及整款remake另驗。原始素材與完整終端／記憶體／gzip／PNG留本機，不進公開版控。

## 唯讀初態與 READY 審查

已證實：未改CPU SHA-256 511a2ed33e0ed990c1c258e5e3098b51558fd9fde304b0584f5f5da207398916，唯讀probe SHA-256 436eeb934caf8d613df6869991aefb9b6c6dca3f7a1386bd2ec834cba3de226c。兩自然第42,347,639步高位LE0x257662的完整R依EAX ECX EDX EBX ESP EBP ESI EDI：347010 6BB370 AFF0AFF 74A 2BDB70 2215 711120 347094，段CS DS ES FS GS SS：8 188 188 0 20 188，flags297h。兩gzip SHA-256 7162e7cb5a8a40a44edf2a08c79949d991c6686ccef00a65e074901015f15306／ef7822539ecae72070daaa9d67d96dd7262137681e48ec8040745012d2d866f7。600秒／2GiB／2CPU／UID:GID1000:1000／network none、Go1.24.13，沿296自然命令，只換297-input輸出檔。末端SHA摘要誤列不存在的full-test-297-input.txt而exit1；兩原版執行及gzip／PNG完整，獨立讀取確認，不當CPU或產品失敗。

公開C1 /1計數／旗標、OF未定義與保留模型及原始bytes／完整初態已足夠，297轉READY才實作。以單bit整除循環16次，預期完整EDX仍0AFF0AFFh、CF=0、模型flags296h，其他R／段保持；半word重複只是這筆真實資料，不以它代表所有輸入。下一A2儲存AL不改R／旗標，0x25766A的66 8B C2為MOV AX,DX，預期完整EAX=00340AFFh、EDX及其他R／段／flags保持；0x25766D才是第二ROR，不混作第一ROR後態。全部CPU／固定EXE／兩自然完整後態與MOV消費仍待驗。


## 受限實作與自然 MOV 消費

限定CONFORMED：既有C1 /1 dword暫存器ROR現在接受全部imm8，只取低五位；零計數全部保持，非零更新CF，計數1計算OF，多位OF保留明示為工具模型。SF／ZF／AF／PF、其他R／段／FPU／記憶體與非算術旗標保持。完整bytes取得後才發布，截短與未知前綴／word／記憶體／D3仍拒絕，既有D1 ROR與ROL回歸通過。原來222測試將07h當未知的負例，依READY範圍改由全部計數的正例驗，其他拒絕護欄保持。不是降低CPU判準，也沒有原版位址特例。

八目的×256計數×32位邊界／每bit及補集×四CF／OF組合×全清或全設保持旗標，以單bit整除循環oracle核對；完整原始R與隔離MOV AX,DX消費／截短／前綴／記憶體形狀及D1回歸通過。CPU SHA-256 35ad5bdc11d470d670e044ff4b1894476056930793ade3df2dec7218ca91ca30；新測試SHA-256 ad63176b71ad06cbe81eb478c16afc4bb9190d707cb87272d7d83951b3e8216b，唯讀probe仍436eeb934caf8d613df6869991aefb9b6c6dca3f7a1386bd2ec834cba3de226c。

全部CPU首次通過：GOMAXPROCS=2 go test -buildvcs=false ./internal/cpu386 -count=1 -v，本機moo2-297-cpu-tests.txt SHA-256 ff42f53fbaf8928ca27521c3cc7999089c454e315d4597f42b00221b0e45b4d5。固定官方EXE全套首次通過：DOSGOLEM_MOO2_EXE=/tmp/game/ORION2.EXE go test -p 2 -buildvcs=false ./... -count=1，full-test-297.txt SHA-256 1df3ef49982e47055e92dae7496b219992cd812e79c50b7a28f8e92b9b8badd5。

600秒／2GiB／2CPU／UID:GID1000:1000／network none／Go1.24.13，重新展開417原檔與固定官方1.31 EXE。兩自然命令：

```text
DOSGOLEM_MOO2_MAX_STEPS=50000000 DOSGOLEM_MOO2_SEPARATE_DOS=1 DOSGOLEM_MOO2_VBE_PNG=/src/workplace/moo2-vbe-297-full-game.png go run -buildvcs=false ./workplace/moo2-probe /tmp/game/ORION2.EXE --game-dir /tmp/game
DOSGOLEM_MOO2_MAX_STEPS=50000000 DOSGOLEM_MOO2_SEPARATE_DOS=1 DOSGOLEM_MOO2_MOUSE_EVENT_AFTER_POSITION=1 DOSGOLEM_MOO2_VBE_PNG=/src/workplace/moo2-vbe-297-mouse-event.png go run -buildvcs=false ./workplace/moo2-probe /tmp/game/ORION2.EXE --game-dir /tmp/game
```

已證實：兩排程各自重生兩組完整ROR→A2→MOV AX,DX→ROR鏈。第一組初態與未改CPU完全一致；第二組初態完整R為A0A0A06 6BB370 AFF0AFF 74A 2BDB70 8 711120 351AB4，兩組段同8 188 188 0 20 188。沒有注入遊戲資料／時計／亂數。

| 步數，第一組／第二組 | 高位LE線性 | 完整R及旗標 |
| --- | --- | --- |
| 42,347,639／42,348,715 | 0x257662 | EDX=0AFF0AFFh、flags297h |
| 42,347,640／42,348,716 | 0x257665 | EDX仍0AFF0AFFh、CF=0、模型flags296h，其他完整R／段保持 |
| 42,347,641／42,348,717 | 0x25766A | A2儲存AL後完整R／段／flags保持 |
| 42,347,642／42,348,718 | 0x25766D | MOV AX,DX真實消費，完整EAX=00340AFFh／0A0A0AFFh；其他完整R／段／flags296h保持 |
| 42,347,643／42,348,719 | 0x257670 | 第二ROR後EDX仍0AFF0AFFh、CF=0，其他完整R／段／flags保持 |

多位OF未定義，所以完整flags296h只表示沿既有保留模型，定義CF與保持旗標另驗；不稱原版硬體OF逐值一致。兩自然gzip SHA-256 21fe3aeec9f3ece1b9721b7fb067c1acfb3136343ef2ce27b193bdeeb4057768／8519d0012606a941046b6504bd06be289ce6b687e4471b09d000fad07f496c1f，MOV消費與完整目的外狀態已由獨立算術核對。

兩自然第42,349,111步外層高位LE0x2571C9的StepHook中，原版IRQ0分支遇到內層高位LE0x2520B7，bytes D1 E0 D1 E3 F7 05 28 2D 27 00 08 00 00 00 74 04；D1 /4的SHL EAX,1未支援。內層解碼停止CS:EIP=0008:002520B9，完整R為0 3D6978 8000 1 2723E0 2723F4 1 325048、段8 188 188 0 20 188、flags2；不能把外層EIP當缺件位址。只按公開CPU契約補SHL，不深入IRQ0 handler／ISR硬體時序。

IRQ0 started6174／completed6173、active=false／failed=true；原先277已驗的返回／等待樣本不因此升格為所有IRQ分支已閉合。等待DS:00271148=E3 11 00 00仍4579；PIT mode2／Reload5966／Generation5，BIOSClock Micros43987979／Deliveries6194／Pending=false／InService=false。C6返回／來源收據保持；保護模式連續PCM／IRQ7仍未知。VBE Bank7／StartY512／BankSets447／Writes4353364／DisplaySets7，兩PNG同已檢視黑圖SHA-256 1610444d26adb3135e7e933dd44912044bb636728af59c70e614945278d3c622。事件排程第1612067步已注入x657／y189。255、主選單／正常玩家路徑、音訊／人耳與受控亂數及整款remake未驗收。

## 解析回填

CPU不可變契約鍵：cpu386裸C1／/1／mod11／32位暫存器／imm8；原版停點鍵：固定EXE雜湊＋高位LE0x257662＋C1 CA 10。222／288的舊ROR立即數限制由297擴充，不重寫舊輔助oracle範圍，需保留「dword ROR立即數範圍由規格 297 擴充」及本檔連結。293／294／295／296的同ROR停點需保留「dword ROR停點已由規格 297 接通」及本檔連結。驗證入口 apps/moo2/tools/startup_probe_131.py --check-ror-dword-immediate-spec-backlinks，缺原始定位／完整MOV消費／未定義旗標邊界或任一舊標記須拒絕。主庫玩法閘門不變。

dword SHL單位移停點已由規格 298 接通，見 [298-cpu386-shl-dword-register-one.md](298-cpu386-shl-dword-register-one.md)。兩自然IRQ0內完整EAX0／EBX2與兩個dword真實寫回已驗，後續ADC停高位LE0x25179F。原有CPU／平台證據範圍不擴張，完整IRQ0返回與正常玩家路徑仍未完成。

dword ADC停點已由規格 300 接通，見 [300-cpu386-adc-dword-register.md](300-cpu386-adc-dword-register.md)。兩自然三組完整ADC／六旗標與索引ADD真實dword消費已驗，後續word XOR停高位LE0x24678C。原有CPU／平台證據範圍不擴張，完整IRQ0返回與正常玩家路徑仍未完成。

word XOR立即數停點已由規格 301 接通，見 [301-cpu386-xor-word-register-imm8.md](301-cpu386-xor-word-register-imm8.md)。兩自然完整word XOR／五旗標與PUSH EDI、PUSH EAX的兩個stack dword真正突變已驗；此IRQ0已返回，後續第42603292步停高位LE0x256171的66 93、XCHG AX,BX。原有CPU／平台與299原版自然OF=1限制保持，主選單／正常玩家路徑仍未完成。

word XCHG停點已由規格 302 接通，見 [302-cpu386-xchg-ax-word-register.md](302-cpu386-xchg-ax-word-register.md)。兩自然三組完整word交換／兩高16位與全部旗標保持、下一ROR EBX,8完整消費已驗，持續到50M上限且IRQ0 started7789／completed7789／failed=false。畫面已見星空片段，仍未驗主選單／正常玩家路徑；原有其他CPU／平台與299原版自然OF=1限制保持。


## 共用裝置時間後續

保護模式裝置時計缺口由規格 304 接線，見[304-le-shared-device-clock.md](304-le-shared-device-clock.md)。2026-10-03兩自然首block真正PCM2048個80h／兩時計44032078已驗；第42356668步以absolute IVT1201:0682停在未建模的保護模式IRQ7，pending保留。只解時間到首block，IRQ7轉送／連續PCM、人耳及正常玩家路徑仍未知，其他原有證據與限制保持。
