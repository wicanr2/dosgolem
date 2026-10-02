# 301：word 暫存器與符號延伸 imm8 的 XOR

狀態：**CONFORMED**
日期：2026-10-02
範圍：cpu386的66 83 /6、mod11、八word暫存器目的與全部imm8；不擴張81、記憶體或其他前綴形狀。

## 原始定位與公開 CPU 契約

工具基線752abc607e04b87d830d4a6451af9e51dfc26b78。固定官方1.31 ORION2.EXE SHA-256 4e11be14217b4aafa1839f333bf5eba037f98b0c44e9e4752c96c464c260419f，417原檔與ZIP／patch／MOX.SET雜湊及自然命令沿 [300-cpu386-adc-dword-register.md](300-cpu386-adc-dword-register.md)。已證實：兩自然outer_step42349111在外層0x2571C9的IRQ0呼叫內停高位LE0x24678C，bytes66 83 F7 01 57 50 E8 6A BA 00 00 A1 EC 15 2B 00，word XOR DI,1未支援。真正完整前態與下一PUSH目的須有限唯讀保存，錯誤後態不當前態。

[Intel80386 XOR](https://www.ardent-tool.com/CPU/docs/Intel/386/manuals/prref386/XOR.htm)定義83 /6的imm8符號延伸到word，再與目的低16位逐bit比較是否不同；目的高16位保持。CF／OF清0，SF取word bit15，ZF按word零值，PF只按低byte。AF未定義；沿 [274-cpu386-xor-word-register.md](274-cpu386-xor-word-register.md)及setLogicFlags16清AF模型，五定義旗標與模型分開驗，不稱硬體AF逐值一致。

## 候選實作與驗收

- 66 83 /6、mod11，完整取得ModRM與imm8後才發布目的低16位與旗標；全部256個imm8延伸為16位，不能先零延伸。其他R／段／FPU／記憶體及非算術旗標保持。
- 截短、67／segment／REP／LOCK前綴、word記憶體形狀拒絕，解碼EIP可前進。既有word ADD／SUB／CMP／OR／AND、31 word XOR與83 dword XOR／byte XOR範圍保持。
- 全部低16位×256立即數×兩初旗標，八目的×word bit／補集／零／符號與繞回邊界×256立即數×64算術初旗標，以逐bit不同／整除符號延伸／低byte bit計數獨立oracle驗結果、五旗標、清AF模型與高16位／完整保持；未知拒絕與既有指令回歸。
- 真正IRQ0完整前態／公開契約審查READY後才實作。全部CPU、固定EXE全套及兩自然完整後態與PUSH真正stack dword消費後才限定CONFORMED，消費缺件保持READY。
- 0x246790的57為PUSH EDI，須驗完整EDI與SS:[ESP-4]的四bytes、ESP減4、flags保持；後續50的PUSH EAX作保持抽樣。只驗呼叫邊界，不追CALL目標內部。

## 停止線

固定四位址各最多三筆唯讀StepHook記錄完整R／段／flags／IRQ狀態、既有八byte堆疊與64bytes，原樣轉送既有hook，不替換CPU／Bus／IRQ橋接、不跳指令、補時計或注入資料。不深入IRQ0 handler／ISR／driver硬體時序、runtime或圖形helper，不猜欄位用途。主庫玩法RE閘門不變；255／主選單／正常玩家路徑、保護模式PCM／IRQ7、人耳／受控亂數與整款remake另驗；299原版自然OF=1限制保持。原始素材及完整終端／記憶體／gzip／PNG留本機。

## 唯讀初態與 READY 審查

未改CPU SHA-256 b7407d43ddaf3eb8d20732acd64f2c7cfb3933924615b29cf7e7e32c7c99c021，有限唯讀probe 67a2cf361ac195ac72aaa85513eb0f0099f446aa2167203fb1769008afadfeaa。沿300的Go1.24.13／600秒／2GiB／2CPU／128pids／UID:GID1000:1000／network none與fresh417原檔及固定1.31 EXE，兩自然命令只換301-input輸出，不改CPU。兩gzip SHA-256 d332d8884501b80ac70f359cf3284ef29ba2f4c485256d1cfecc12c8fa656547／cd56243a83ed5519ed924cc3a6977a573019299e981e2b89b46fbfa59db2cf57。

已證實：兩自然各一筆真正前態相同，outer_step42349111、高位LE0x24678C，完整R為325048 3D6978 3D69C0 1 272400 FC4 0 0、段8 188 188 0 20 188、flags46h；IRQ0 active=true／failed=false／started6174／completed6173。SS:002723F8／SS:002723FC連續八bytes為52672400 78693D00，兩個dword原值分別00246752h／003D6978h。這是執行前StepHook保存的初態。

[Intel SDM2C](https://www.intel.com/content/dam/www/public/us/en/documents/manuals/64-ia-32-architectures-software-developer-vol-2c-manual.pdf)印刷頁5-594／5-595再次確認83 /6 word符號延伸與五定義旗標，AF未定義。依[Intel80386 PUSH](https://www.ardent-tool.com/CPU/docs/Intel/386/manuals/prref386/PUSH.htm)，下一裸57消費完整EDI、ESP減4並寫stack dword，旗標保持。

公開CPU契約、完整前態、原始bytes與下一真實消費已足夠，301審查READY才實作。預期XOR令完整EDI=1／flags2，目的外保持且記憶體不變；PUSH EDI令ESP=002723FCh、八bytes=52672400 01000000；PUSH EAX令ESP=002723F8h、八bytes=48503200 01000000，兩PUSH flags2且其他R／段保持。兩自然真正後態與兩個stack dword數值突變仍待驗，不追後續CALL目標內部。

## 測試勘誤與回歸

第一次全部CPU收據moo2-301-cpu-tests-first.txt SHA-256 59cd8a70fec7de6e18004bad33361b885d01b8972205574e2aa11120e44fb73b：301全部word輸入／八目的旗標／真正PUSH隔離測試通過。失敗有兩項：293仍把66前綴的word XOR列為負例；新增byte XOR回歸誤期望保留AF。依既有286及setLogicFlags8，AF未定義採清除模型，正確期望flags602h。只退休已合法化的word XOR舊負例並修正新增測試期望，CPU未再修改。267的66 83 F3舊負例同樣由全部word XOR正例接替，未知word ADC／SBB與其他拒絕形狀保持。相同映像／命令乾淨重跑全部CPU通過；原版自然後態仍另驗。


## 受限實作與真正堆疊消費

限定CONFORMED：66 83 /6、mod11、八word目的及全部imm8。先取得完整ModRM／imm8，符號延伸到16位，目的高16位與其餘R／段／FPU／記憶體／非算術旗標保持。五定義旗標與AF清除模型分開核對；拒絕截短、67／segment／REP／LOCK或word記憶體形狀。既有word ADD／SUB／CMP／OR／AND／31 XOR、83 dword XOR與80 byte XOR保持。

全部65536低word×256立即數×兩初旗標，八目的×45項bit／補集／零／符號／繞回邊界×256立即數×64初旗標，以逐bit比較／整除符號延伸／低byte位元計數獨立驗結果與完整保持。原版完整初態隔離XOR→PUSH EDI→PUSH EAX驗兩次完整dword數值突變，記憶體其餘bytes保持。CPU SHA-256 37e637105219c22d92065f7c173b716667f43f91c525cc5a5bdcd2722bd6d39b；新測試05ccc9a9eb0df8b90b371c4f493a0213924ff2f00b6986b6951c03657c2f7450；probe保持67a2cf361ac195ac72aaa85513eb0f0099f446aa2167203fb1769008afadfeaa。

全部CPU乾淨重跑通過：GOMAXPROCS=2 go test -buildvcs=false ./internal/cpu386 -count=1 -v，moo2-301-cpu-tests.txt SHA-256 7ee5e1795791547f35233adb0b578dac033107368ca5690d6a8e9b6ae331956e。固定EXE全套通過：DOSGOLEM_MOO2_EXE=/tmp/game/ORION2.EXE go test -p 2 -buildvcs=false ./... -count=1，full-test-301.txt SHA-256 4df4f49481a1f920ac0812d4694c98bf8d70a9ba9d8fa2f327b7d0bce0ffb7ac。第一次失敗收據保留，沒有再次修改CPU或改正確契約。

沿用golang:1.24-bookworm、Go1.24.13 linux/amd64、映像ID sha256:1a6d4452c65dea36aac2e2d606b01b4a029ec90cc1ae53890540ce6173ea77ac；600秒／2GiB／2CPU／128pids／UID:GID1000:1000／network none，fresh417原檔與固定1.31 EXE。沒有CPU狀態／時計／IRQ橋接／資料或亂數注入。兩自然命令：

```text
DOSGOLEM_MOO2_MAX_STEPS=50000000 DOSGOLEM_MOO2_SEPARATE_DOS=1 DOSGOLEM_MOO2_VBE_PNG=/src/workplace/moo2-vbe-301-full-game.png go run -buildvcs=false ./workplace/moo2-probe /tmp/game/ORION2.EXE --game-dir /tmp/game
DOSGOLEM_MOO2_MAX_STEPS=50000000 DOSGOLEM_MOO2_SEPARATE_DOS=1 DOSGOLEM_MOO2_MOUSE_EVENT_AFTER_POSITION=1 DOSGOLEM_MOO2_VBE_PNG=/src/workplace/moo2-vbe-301-mouse-event.png go run -buildvcs=false ./workplace/moo2-probe /tmp/game/ORION2.EXE --game-dir /tmp/game
```

已證實：兩自然各一組四筆完整XOR→PUSH EDI→PUSH EAX，相同完整前態與未改CPU收據一致；outer_step42349111、IRQ0 active=true／failed=false／started6174／completed6173、段8 188 188 0 20 188。這是有限觀測，不宣稱總呼叫數。

| 高位LE指令位址 | 完整EDI | 完整ESP | flags | SS:002723F8／SS:002723FC八bytes |
| --- | --- | --- | --- | --- |
| 0x24678C，XOR前 | 00000000h | 00272400h | 46h | 52672400 78693D00 |
| 0x246790，XOR後／PUSH EDI前 | 00000001h | 00272400h | 2 | 52672400 78693D00 |
| 0x246791，PUSH EDI後／PUSH EAX前 | 00000001h | 002723FCh | 2 | 52672400 01000000 |
| 0x246792，PUSH EAX後／CALL前 | 00000001h | 002723F8h | 2 | 48503200 01000000 |

XOR後完整EDI=00000001h／flags2，目的外完整R保持；PUSH EDI寫SS:002723FC dword=1，原值003D6978h；PUSH EAX寫SS:002723F8 dword=00325048h，原值00246752h。兩PUSH各令ESP減4，其他完整R／段與flags2保持。這是原版真實兩個dword數值突變，後續CALL只記邊界，不追目標內部。兩自然gzip SHA-256 ccd184dd64aa984b77e21b6624fca6a7407c95b938dfa959203af32ef0901c13／339eb37ec3be286d6e42b95e7f87d8913ad9423cbf381052a77993fc3025b27b；逐bit與完整保持／實際目的位置／寫值／初態匹配獨立稽核通過。

兩自然前進至第42603292步，比原停點多254181步，轉停高位LE0x256171，bytes66 93 C1 CB 08 C3 90 8B C2 8A E2 8B DA C1 C8 18，標準XCHG AX,BX缺件。根CPU解碼後EIP256173h、EAX0A0A0A2Eh／EBX2E0A0A0Ah／ECX2E0A40C0h／EDX2E0A2E0Ah／flags206h；下一真正完整前態仍須有限唯讀保存，不以解碼後態取代。

這次IRQ0已返回，後續自然終態active=false／failed=false／started6228／completed6228。等待DS:00271148=1A 12 00 00即4634；PIT mode2／Reload5966／Generation5，BIOSClock Micros44292270／Deliveries6248／Pending=false／InService=false。只證明此自然樣本，工具時鐘不稱硬體wall-clock一致；277／其他IRQ0分支證據範圍不擴張。

C6實模式333步成功返回／來源收據保持，保護模式連續PCM／IRQ7及人耳仍未知。VBE Bank9／StartY512／BankSets452／Writes4660564／DisplaySets7，indexed SHA-256 9d4d567f9cbe2a0e0069255c8f979c1ba07e90fad5974e5a0cf96a6c1469ae56已有變化，但兩PNG仍同先前已檢視黑圖SHA-256 1610444d26adb3135e7e933dd44912044bb636728af59c70e614945278d3c622。事件第1612067步注入x657／y189，仍不算255完整座標／游標、主選單／正常玩家路徑或remake玩法同狀態。299原版自然OF=1限制、音訊／人耳／受控亂數及整款remake未驗收保持。

## 解析回填

CPU契約鍵：cpu386 66 83／/6／mod11／word暫存器／符號延伸imm8；原版停點鍵：固定EXE雜湊＋高位LE0x24678C＋66 83 F7 01。293–300保存「word XOR立即數停點已由規格 301 接通」與本檔連結，293的word前綴拒絕只由本規格窄形狀接通；原有其他CPU／平台證據不擴張。驗證入口 apps/moo2/tools/startup_probe_131.py --check-xor-word-immediate-spec-backlinks；缺定位／五旗標／AF模型／兩個真正stack dword寫入／IRQ0返回或任一舊標記須拒絕。主庫玩法RE閘門不變，原版素材及完整終端／記憶體／gzip／PNG留本機。

word XCHG停點已由規格 302 接通，見 [302-cpu386-xchg-ax-word-register.md](302-cpu386-xchg-ax-word-register.md)。兩自然三組完整word交換／兩高16位與全部旗標保持、下一ROR EBX,8完整消費已驗，持續到50M上限且IRQ0 started7789／completed7789／failed=false。畫面已見星空片段，仍未驗主選單／正常玩家路徑；原有其他CPU／平台與299原版自然OF=1限制保持。


## 共用裝置時間後續

保護模式裝置時計缺口由規格 304 接線，見[304-le-shared-device-clock.md](304-le-shared-device-clock.md)。2026-10-03兩自然首block真正PCM2048個80h／兩時計44032078已驗；第42356668步以absolute IVT1201:0682停在未建模的保護模式IRQ7，pending保留。只解時間到首block，IRQ7轉送／連續PCM、人耳及正常玩家路徑仍未知，其他原有證據與限制保持。
