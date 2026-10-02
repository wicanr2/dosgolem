# 298：dword 暫存器的單位左移

狀態：**CONFORMED**
日期：2026-10-02
範圍：cpu386 裸D1 /4、mod=11，八個32位暫存器，計數固定1。

## 原始定位與公開 CPU 契約

工具基線4cdf20e347500a5c996b955831e35ef68ca556e0。固定官方1.31 ORION2.EXE SHA-256 4e11be14217b4aafa1839f333bf5eba037f98b0c44e9e4752c96c464c260419f；417原檔、ZIP／patch／MOX.SET雜湊與兩自然命令沿 [297-cpu386-ror-dword-register-imm8.md](297-cpu386-ror-dword-register-imm8.md)。已證實：兩自然第42,349,111步外層高位LE0x2571C9的IRQ0呼叫內，原版停高位LE0x2520B7，bytes D1 E0 D1 E3 F7 05 28 2D 27 00 08 00 00 00 74 04；D1 /4的SHL EAX,1未支援。內層解碼後CS:EIP0008:002520B9、R為0 3D6978 8000 1 2723E0 2723F4 1 325048、段8 188 188 0 20 188、flags2。真正指令前態與下一消費端已由後文有限唯讀觀測保存，不把外層EIP當缺件位置。

[Intel80386 SAL／SHL](https://www.ardent-tool.com/CPU/docs/Intel/386/manuals/prref386/SAL.htm)定義D1 /4的dword單位左移，最低bit補0、CF取原最高bit、OF取結果最高bit XOR CF；SF／ZF／PF依結果。[Intel SDM2B](https://www.intel.com/content/dam/www/public/us/en/documents/manuals/64-ia-32-architectures-software-developer-vol-2b-manual.pdf)的SAL／SAR／SHL／SHR Flags Affected明示非零計數AF未定義。沿既有setLogicFlags清AF的工具模型，五定義旗標另驗，不宣稱硬體AF逐值一致。

## 候選實作與驗收

- 裸D1 /4、mod11的八目的共用契約；完整取得ModRM才發布32位結果與旗標，保留其他R／段／FPU／記憶體及非算術旗標。不在CPU加入遊戲位址、IRQ狀態或音訊特例。
- 截短、67／segment／REP／LOCK前綴與記憶體形狀拒絕，解碼EIP可前進；66 D1 word SHL與既有C1／D3／D1 ROR／RCL／SHR／SAR範圍保持。
- 全八目的／每bit與補集／符號及繞回邊界／全部低16位／64算術旗標初態，以整數乘2／整除／低byte位元計數的獨立oracle驗結果、五定義旗標及AF清除模型；完整原版初態、保持／拒絕與既有指令回歸。
- 未改CPU原版初態與公開契約審查READY後才實作。全部CPU、固定EXE全套、兩自然完整後態與真正消費後才限定CONFORMED；消費缺件保持READY。IRQ0返回與音訊全路徑分開，不以孤立SHL通過宣稱它們閉合。

## 停止線

只補阻塞的標準CPU形式，不追IRQ0 handler／ISR、driver／busy-wait、PCM／DAC／PIT／DMA硬體時序、runtime或圖形helper的內部。主庫玩法RE閘門不變；255座標／游標、主選單／正常玩家路徑、保護模式PCM／IRQ7、人耳與受控亂數及整款remake另驗。原版素材與完整終端／記憶體／gzip／PNG留本機。

## 唯讀初態與 READY 審查

已證實：未改CPU SHA-256 35ad5bdc11d470d670e044ff4b1894476056930793ade3df2dec7218ca91ca30，有限probe SHA-256 8505623b0a372bcfcdc427bc79b6f8e62597f06234bf1622e80f9acde3117655。同CPU的StepHook包裝器只在固定三個指令位址讀R／段／flags與64bytes，原樣轉送既有hook，不替換CPU／Bus／RealModeIO／IRQ0橋接、不跳指令或補時鐘。兩自然outer_step42349111的真正0x2520B7前態完全一致：R為0 3D6978 8000 1 2723E0 2723F4 1 325048、段8 188 188 0 20 188、flags2，IRQ0 active=true／failed=false／started6174／completed6173。

600秒／2GiB／2CPU／UID:GID1000:1000／network none／Go1.24.13，重新展開417原檔與固定官方1.31 EXE，沿297自然命令只換298-input輸出。兩gzip SHA-256 b8efabad77ec3cf291a187cd9297e44f2b3c5e7a5203c5373ff51a5adf57b6f4／f0c63376c9f90318231b6781e126265a74b20284d87ef1fc8cc921ce72995d21；CPU／DPMI／IRQ0橋接尚未修改。

公開D1 /4與五定義旗標／AF清除模型、完整原始前態與拒絕範圍足夠，298轉READY才實作。第一SHL EAX,1預期EAX仍0、flags46h，其他R／段保持；下一0x2520B9的SHL EBX,1把1→2、flags2。原始64bytes還顯示0x2520C5的JZ可跳過第二組SHL，後續0x2520CB的A3把EAX存DS:00272D40、0x2520D0的89 1D把EBX存DS:00272D44。TEST的bit08h決定EBX最後為2或4，必須記錄同次TEST來源／分支與完整寫回，不猜用途，也不把無關TEST當資料消費；缺件保持READY。

正式驗證僅將唯讀觀測擴到這些既有bytes的固定九位址與TEST來源／兩個目的dword，仍最多各三筆且原樣轉送既有hook。實作不改IRQ0或CPU時鐘，只補標準D1 /4；完整IRQ0返回與玩家路徑不能由指令孤立測試替代。


## 受限實作與兩個原版寫回

限定CONFORMED：裸D1 /4、mod11的八個dword目的更新結果及五個定義旗標，AF未定義，清除只驗工具模型。全部低16位／每bit與補集／符號邊界／64算術初旗標及完整R／段／FPU／記憶體保持、截短／前綴／記憶體拒絕均通過，既有word／C1／D3／ROR／RCL／SHR／SAR回歸通過。既有D1未知group負例由現在合法的E0改為仍未知D8，不移除護欄。

第一次完整CPU回歸發現既有C1 E0 01的單位OF未設定，D1新測試均通過，沒有改正確預期掩蓋問題。裸C1單位移OF契約由規格 299 補齊，見 [299-cpu386-c1-dword-single-shift-overflow.md](299-cpu386-c1-dword-single-shift-overflow.md)。299獨立DRAFT／公開契約反例審查READY後才窄修正；保留第一次失敗收據。相同映像／CPU命令乾淨重跑通過，固定官方EXE全套通過。

CPU SHA-256 7cb0cade0c7b66adc37e01d458f9f22a1a57e2112afa03e62c91417d3a8a2f7c；298測試b165bf74b220b597be3d52ce6b0888267986b00d6c68ac585340fde293b31f97；299測試7023724891cb9865589d28f7b9488392aed471ae70818c89585f528abd39e71f；九位址有限唯讀probe 2c279c0e8e3e4e6f5646a083f90f34b6feec088fd12ec4f032b4083b781eb360。GOMAXPROCS=2 go test -buildvcs=false ./internal/cpu386 -count=1 -v，moo2-298-cpu-tests.txt SHA-256 198c7c11abee84f928fea00f4da39d50b225ec29767bbd212d0cb92f53a16187；固定EXE全套DOSGOLEM_MOO2_EXE=/tmp/game/ORION2.EXE go test -p 2 -buildvcs=false ./... -count=1，full-test-298.txt SHA-256 f851638ad1926ee142d9c438160f7ba8475a59ebfdb2950dcd5565d386b7e9fb。

同Go1.24.13映像、600秒／2GiB／2CPU／128pids／UID:GID1000:1000／network none，重新展開417原檔及官方1.31 EXE；無CPU／時鐘／IRQ／遊戲資料注入。兩自然命令：

```text
DOSGOLEM_MOO2_MAX_STEPS=50000000 DOSGOLEM_MOO2_SEPARATE_DOS=1 DOSGOLEM_MOO2_VBE_PNG=/src/workplace/moo2-vbe-298-full-game.png go run -buildvcs=false ./workplace/moo2-probe /tmp/game/ORION2.EXE --game-dir /tmp/game
DOSGOLEM_MOO2_MAX_STEPS=50000000 DOSGOLEM_MOO2_SEPARATE_DOS=1 DOSGOLEM_MOO2_MOUSE_EVENT_AFTER_POSITION=1 DOSGOLEM_MOO2_VBE_PNG=/src/workplace/moo2-vbe-298-mouse-event.png go run -buildvcs=false ./workplace/moo2-probe /tmp/game/ORION2.EXE --game-dir /tmp/game
```

已證實：兩自然各七筆完整R／段／flags與記憶體相同，全部在outer_step42349111的同一次IRQ0內；outer_step不冒稱IRQ內指令序號。0x2520B7前態與未改CPU兩自然相同，R為0 3D6978 8000 1 2723E0 2723F4 1 325048、段8 188 188 0 20 188、flags2。全部七筆IRQ0 active=true／failed=false／started6174／completed6173、TEST來源DS:00272D28=00000003h。

| 高位LE線性 | 已執行結果與下一原版指令 |
| --- | --- |
| 0x2520B9 | EAX倍增後仍0、flags46h，下一D1 E3 |
| 0x2520BB | EBX由1倍增為2、flags2，下一TEST dword |
| 0x2520C5 | 3 AND 8=0，TEST令flags46h，下一JZ+4 |
| 0x2520CB | JZ到A3，第二組0x2520C7／0x2520C9沒有執行，EBX仍2 |
| 0x2520D0 | 原版A3已存EAX0至DS:00272D40，下一89 1D |
| 0x2520D6 | 原版89 1D已存EBX2至DS:00272D44 |

DS:00272D40／DS:00272D44真實寫回，目的連續八bytes初值全0，A3執行後前四bytes仍0，89 1D後為00000000 02000000。只有EBX有可見數值變更，不把EAX的0→0稱為記憶體突變。其他完整R／段保持，MOV保持TEST後flags46h；沒有猜欄位用途。第二組SHL分支自然未走，CPU全輸入測試不取代該遊戲分支原版驗收。兩gzip SHA-256 f6984adbd0f227ce8a0034c61189ae384aff2353ae9a3eb14be9872d5e51fd55／ced6336a1ae88b4d90905a4e4789451e346a1a7d94f7b56b397a7e61b7334b32，獨立算術與完整保持／分支／真實寫回核對通過。

兩自然同outer_step42349111轉停內層高位LE0x25179F，bytes 13 ED 03 34 AD 40 2D 27 00 0F BF E8 01 2F 0F BF，標準ADC EBP,EBP缺件。外層仍0x2571C9；內層解碼錯誤CS:EIP0008:002517A0、R為0 0 3D69C0 0 2723D8 0 71E1D0 3C6038、段8 188 188 0 20 188、flags847h。IRQ0 active=false／failed=true／started6174／completed6173，完整IRQ0返回尚未驗。等待DS:00271148仍4579；PIT mode2／Reload5966／Generation5，BIOSClock Micros43988016／Deliveries6194／Pending=false／InService=false，這是工具時鐘，不稱硬體wall-clock一致。C6成功返回／333步與來源收據保持；保護模式連續PCM／IRQ7未知。

VBE Bank7／StartY512／BankSets447／Writes4353364／DisplaySets7，兩PNG同先前已檢視黑圖SHA-256 1610444d26adb3135e7e933dd44912044bb636728af59c70e614945278d3c622。事件排程第1612067步注入x657／y189，仍不算255完整座標／游標或正常玩家路徑。主選單／受控亂數／音訊人耳與整款remake未完成。

## 解析回填

CPU契約鍵：cpu386裸D1／/4／mod11／32位暫存器／固定計數1；原版停點鍵：固定EXE雜湊＋高位LE0x2520B7＋D1 E0。293／294／295／296／297保存「dword SHL單位移停點已由規格 298 接通」及本檔連結，限定這筆IRQ0真實寫回，不擴張舊CPU／平台或所有IRQ0分支。驗證入口 apps/moo2/tools/startup_probe_131.py --check-shl-dword-one-spec-backlinks，缺原始定位／旗標／兩個原版寫回／未定義模型或任一舊標記須拒絕。完整原始素材、終端／gzip／PNG留本機。

dword ADC停點已由規格 300 接通，見 [300-cpu386-adc-dword-register.md](300-cpu386-adc-dword-register.md)。兩自然三組完整ADC／六旗標與索引ADD真實dword消費已驗，後續word XOR停高位LE0x24678C。原有CPU／平台證據範圍不擴張，完整IRQ0返回與正常玩家路徑仍未完成。

word XOR立即數停點已由規格 301 接通，見 [301-cpu386-xor-word-register-imm8.md](301-cpu386-xor-word-register-imm8.md)。兩自然完整word XOR／五旗標與PUSH EDI、PUSH EAX的兩個stack dword真正突變已驗；此IRQ0已返回，後續第42603292步停高位LE0x256171的66 93、XCHG AX,BX。原有CPU／平台與299原版自然OF=1限制保持，主選單／正常玩家路徑仍未完成。
