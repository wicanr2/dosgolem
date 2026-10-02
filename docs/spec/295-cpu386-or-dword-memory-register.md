# 295：記憶體 dword 目的與暫存器的 OR

狀態：**CONFORMED**
日期：2026-10-02
範圍：cpu386 裸09 /r 的32位記憶體目的。既有 word／暫存器形式保持。

## 原始定位與公開契約

工具基線 a899e9e7e4f397d17054bdccae97faa0f6cf963c。固定官方1.31 ORION2.EXE SHA-256 4e11be14217b4aafa1839f333bf5eba037f98b0c44e9e4752c96c464c260419f，唯讀417原檔、命令與上一停點見 [294-cpu386-bsf-dword-register.md](294-cpu386-bsf-dword-register.md)。第41,223,220步停 dosgolem 高位LE線性0x23C36B，bytes 09 86 84 03 00 00 EB 25 8B 04 24 8B 94 86 04 04。ModRM=86，來源完整EAX=2000h，目的DS:[ESI+384h]；完整R／段／目的dword待未改CPU的有限唯讀觀測。

[原廠Intel80386 OR](https://www.ardent-tool.com/CPU/docs/Intel/386/manuals/prref386/OR.htm)定義09 /r將兩個32位來源逐bit聯集寫回第一運算元；CF／OF清除，SF／ZF／PF依結果，AF未定義。五定義旗標與既有setLogicFlags的AF清除工具模型分開驗，不稱未定義旗標的硬體一致。

## 候選實作與驗收

- 裸32位記憶體目的改用既有decodeAddress32，涵蓋ModRM／SIB、DS／SS預設段、disp8符號延伸／disp32與32位位址繞回。完整讀取、寫入成功才發布旗標；來源暫存器／段／FPU與非算術旗標保持。沒有遊戲位址特例。
- 八來源／位元及補集／符號邊界／全部低byte配對、全部記憶體ModRM／SIB／來源與位址別名、DS／SS區分、最後合法dword及越界，採獨立逐bit聯集與同位oracle。既有word／dword暫存器與mod1記憶體入口回歸。
- 截短／前綴拒絕、未知段／越界／唯讀與每byte讀寫失敗需測試。沿既有逐byte Bus 寫入工具模型：段與完整寬度先驗，後續Bus寫失敗可保留已成功byte，R／旗標不發布，不宣稱硬體例外原子性或restart一致。
- 未改CPU完整初態及公開契約審查READY後才實作。全部CPU／固定EXE與有無受控滑鼠事件的兩次自然重跑，核對完整後態／記憶體寫回與後續真實讀取，再限定CONFORMED；消費端缺件時保持READY。

## 停止線

不追helper／runtime／ISR／driver／busy-wait，不猜目的欄位用途。原始素材、完整終端／記憶體／gzip／PNG留本機。主庫玩法RE閘門不變；255、主選單／正常玩家路徑、音效／受控亂數及整款remake另驗。

## 唯讀初態與 READY 審查

已證實：未改CPU SHA-256 8d1d338408883a85abd8226c94624cd5045b126cf251034a8ab9155c123643ce，有限probe SHA-256 97ec3a413cdc788fd38977c1d64841067efd7aae72f42567b665cb752a4591fe。第41,223,220步完整R依EAX ECX EDX EBX ESP EBP ESI EDI：2000 D 6A0 2BDB38 2BDB04 FE040 3254E0 74；段CS DS ES FS GS SS：8 188 188 0 20 188，flags=206h。完整目的DS:00325864 dword=00000040h，四byte皆可讀。獨立逐bit聯集預期00002040h，五定義旗標CF／OF／ZF／SF／PF均0；非算術旗標保持，AF未定義的清除模型得flags=202h。

240秒／2GiB／2CPU／UID:GID=1000:1000／network none、golang:1.24-bookworm、Go1.24.13，重新展開417原檔與固定EXE。初態命令 DOSGOLEM_MOO2_MAX_STEPS=50000000 DOSGOLEM_MOO2_SEPARATE_DOS=1 go run -buildvcs=false ./workplace/moo2-probe /tmp/game/ORION2.EXE --game-dir /tmp/game；workplace/moo2-probe-295-input.txt.gz SHA-256 016bd5043acfcaa9341350ec44a9b36cf4977d0a887491d757f3a32e56d4c92c。

公開09 /r／五定義旗標與AF未定義、既有decodeAddress32及readSegment32／writeSegment32完整寬度和權限、逐byte Bus錯誤模型已審查。段／越界／唯讀拒絕不寫目的；讀失敗不寫，寫失敗可能部分byte已完成，但不發布旗標。截短的EIP沿既有解碼前進模型，不稱硬體exception一致。此邊界足夠，295轉READY才改CPU。0x23C371的EB 25應跳0x23C398，該處下一TEST檢查堆疊，不能充作OR資料的真實consumer；另有限觀測目的的實際讀取，缺件時保持READY。

## 已接 CPU，自然寫回與消費待驗

通用decodeAddress32取代原先只接受mod1／非ESP的特製解碼；記憶體讀寫與成功後setLogicFlags沿既有入口。CPU.go SHA-256 511a2ed33e0ed990c1c258e5e3098b51558fd9fde304b0584f5f5da207398916。全部八來源／76值配對、全部低byte配對／64旗標初態、所有ModRM／SIB與DS／SS及位址別名／繞回／段末dword、拒絕／逐byte讀寫失敗及word／dword暫存器保持已驗，獨立整除oracle核對結果與五定義旗標；AF清除及部分寫入另驗工具模型。

首次CPU失敗是測試借用的subByteFailureBus將所有byte寫入拒絕，不能測指定第n個byte失敗。改自製指定地址失敗Bus，CPU不變，同映像／GOMAXPROCS=2 go test -buildvcs=false ./internal/cpu386 -count=1 -v乾淨重跑全通過。失敗收據moo2-295-cpu-tests-failed.txt SHA-256 3f97879f7ecadfccee6f1688b1310c6af109a71cbdbb44f84985f252b951113a；成功moo2-295-cpu-tests.txt SHA-256 9333cc23374ef3409423be5d3b98b6210e3d2e9fe0fe7a7380f1c9fcc4f20ee3。

第一次原版自然重跑的診斷器包裝Bus，破壞dpmi.go freeMemoryInformation的c.Bus==h.m身分契約，有無事件分別提早停第5,870,288／5,870,323步、0x24C315的INT31/0500h。這兩份不計原版OR驗收，完整無效收據保留本機invalid-bus檔，SHA-256 3ba6582fff44b2d6129d2334b5c742bbe9933d53c03a358aeccfeea95b6881f9／256f24987111c75238026e1c0c2171f936bc74463eea78fbe9a75ef6830984db。CPU與DPMI不改，有限診斷改觀察SegmentRead8並原樣轉送既有hook返回值，保留Bus身分；只在指令Step期間觀察，排除診斷本身及原OR，最多三筆真正讀取。probe SHA-256 ddc0e470d9fd1b2f76fec4918b83f984e34cbf5981ae98f932e57cada0df3cf3。同映像／同命令重新展開乾淨原檔再跑，不把診斷問題寫成產品缺陷。

## 原版完整寫回與真正消費

限定CONFORMED：全部CPU與固定官方EXE全套 DOSGOLEM_MOO2_EXE=/tmp/game/ORION2.EXE go test -p 2 -buildvcs=false ./... -count=1 通過，full-test-295.txt SHA-256 6ab325d7a816db9a928a9429be55ae94e0e56aa8e06db12d24a1b6f86d3fae11。修正Bus觀測器後首次兩自然已到新停點，但讀取只命中目的+2的未變高byte，不能用它證明新增bit13的消費。保留high-byte-read收據 SHA-256 1e9117a5eb2e524247dc8ed10a459ce0a99661a44dc8e18c86629d52acb7c4c5／2b9e5039842078beed3b05fe88130ead35ef096368b5b6f0a9920ab22a53d6a0。再將有限觀測收窄至低兩byte／dword的取址端，最多16筆；兩自然實際各兩筆，正常執行步數與後態保持。最後probe SHA-256 647de0701fcdf4c582d561ebc5b8b54b2cb34ff6484b73d9908135e5a17ac1ed，CPU與DPMI不再改。

600秒／2GiB／2CPU／UID:GID=1000:1000／network none／Go1.24.13，重新展開417原檔與固定EXE。兩自然命令：

```text
DOSGOLEM_MOO2_MAX_STEPS=50000000 DOSGOLEM_MOO2_SEPARATE_DOS=1 DOSGOLEM_MOO2_VBE_PNG=/src/workplace/moo2-vbe-295-full-game.png go run -buildvcs=false ./workplace/moo2-probe /tmp/game/ORION2.EXE --game-dir /tmp/game
DOSGOLEM_MOO2_MAX_STEPS=50000000 DOSGOLEM_MOO2_SEPARATE_DOS=1 DOSGOLEM_MOO2_MOUSE_EVENT_AFTER_POSITION=1 DOSGOLEM_MOO2_VBE_PNG=/src/workplace/moo2-vbe-295-mouse-event.png go run -buildvcs=false ./workplace/moo2-probe /tmp/game/ORION2.EXE --game-dir /tmp/game
```

兩條完整R／段／目的資料逐筆一致，前態與未改CPU收據相同；不注入原版資料／時計／亂數。

| 步數／高位LE線性 | 真實資料／定義旗標 |
| --- | --- |
| 41,223,220／0x23C36B | 完整EAX=2000h、目的DS:00325864 dword=00000040h，flags=206h |
| 41,223,221／0x23C371 | 寫回DS:00325864 dword=00002040h，五定義旗標均0、模型flags=202h；完整R／段保持 |
| 41,223,222／0x23C398 | 原版EB25自然跳轉，完整R／段／flags保持，記憶體仍2040h |
| 41,255,293／0x23B693 → 0x23B699 | bytes 8B 86 84 03 00 00，MOV EAX,[ESI+384h]讀取完整2040h，完整EAX=2040h；其他R／段／flags=206h保持 |

MOV前完整R：3500B 2BDB68 3259A0 1234DD 2BDB68 18D88B 3254E0 3500B，段8 188 188 0 20 188；MOV後只EAX改2040h。因此OR新增bit13已被完整dword讀取，沒有拿未變高byte或下一條無關TEST冒稱消費。後續第42,346,935步0x23B52C的F6 86 85 03 00 00 04讀取目的+1 byte20h，TEST mask04結果0、flags246h、R／段保持，只記錄其真實讀取，不猜位元用途。

兩自然最後第42,347,254步停高位LE0x2454AE，bytes CD 31 8B 5D 14 83 FB 00 74 2F 66 8B 45 E2 66 8B；AX=0300h／BX=0066h。實模式INT66從1201:016A前進230步停1201:05D9，OUT 022Ch／C6h尚未支援，Returned=false；DPMI INT31未處理只是外層回報。下一步先公開SB16 DSP命令／DMA與sample duration契約、有限原版參數與caller審查，不深挖driver／ISR／DAC／PIT硬體wall-clock。

IRQ0 started=completed=6173、active=false／failed=false，等待DS:00271148=E3 11 00 00，即4579；PIT Mode2／Reload5966／Generation5，Micros43985557／Deliveries6193／Pending=false／InService=false。VBE Bank7／StartY512／BankSets447／Writes4353364／DisplaySets7，索引SHA-256 7818f5542a0404157573be6cffc0e0c8e68ce3c0f5d17d07ccdd9313fb700baf。兩自然gzip SHA-256 2d8bb4b224cdbb24440177d6ae4856de345b7602b36323f2f280c4eca862dac1／3c1bd060f88f34ca1aaf9d4055571b830ecfef834dcbb416b4d1af818925fdce。兩PNG同已檢視黑圖 SHA-256 1610444d26adb3135e7e933dd44912044bb636728af59c70e614945278d3c622。受控滑鼠事件已注入x657／y189，255／主選單／正常玩家路徑、音效／受控亂數及整款remake仍未完成。

## 後續 C6h 停點回填

SB16 C6h 停點已由規格 296 接通，見 [296-sb16-c6-auto-init-dma.md](296-sb16-c6-auto-init-dma.md)。原版完整C6 20 FF 07接受、實模式333步返回、原版MOV／CMP／JZ成功分支已驗；保護模式連續PCM／IRQ7仍未閉合。第42,347,639步轉停高位LE0x257662的dword ROR立即數10h；本檔原有CPU證據範圍不擴張。
