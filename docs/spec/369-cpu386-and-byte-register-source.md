# 369：byte目的AND與暫存器來源

狀態：**CONFORMED，限定工具CPU與正常續行**
日期：2026-10-04

## 原玩家阻塞與ISA

沿[368-moo2-overlay-banner-red](368-moo2-overlay-banner-red.md)同可寫180M正常開局，原165113094、dosgolem_high_le:169E49 bytes20 D0 59 C3 53 51 89 C1拒絕opcode20。官方1.31 ORION2.EXE SHA-256 4e11be14217b4aafa1839f333bf5eba037f98b0c44e9e4752c96c464c260419f，工具62cd4911727f17042cb5f8ce98e10b0fe80331ac；原收據8cf7b6f1e75b7759e3e40c8a7574cbf85150a7269bd1e99fdb3fae4445d882cd，CPU b8c1844163fddd7e3557e19fc51abcb9bcb9c1d021fd415dae376b2afbb9c72b。原R=[18900 0 601 F 2BDB00 2BDB34 1 FFFFFFEC]、段=[8 188 188 0 20 188]、flags216h，原FPU127F／status0／depth0／八stack bits0。原unsupported只fetch opcode使EIP至169E4A，未解ModRM或改其他核心；首兩bytes為AND AL,DL，原AL00、DL01已取，結果應00。下一59 POP ECX、C3 RET作原consumer，stack值由同native只讀擷取，不猜返回值。

[Intel 80386 AND](https://www.ardent-tool.com/CPU/docs/Intel/386/manuals/prref386/AND.htm)定義20 /r為AND r/m8,r8，逐bit只有兩側皆1才保留，CF／OF清0，PF／SF／ZF依結果；[附錄C](https://www.ardent-tool.com/CPU/docs/Intel/386/manuals/prref386/appc.htm)標AF未定義。本工具沿既有setLogicFlags8清AF模型，不稱硬體AF逐值一致或exception restart。

## READY預定契約

增加裸20 /r register與memory目的，支持八byte來源別名／目的別名及現有32位ModRM／SIB、DS／SS、signed位移／繞回／段末。沿既有reg8／setReg8、decodeAddress32、readSegment8／writeSegment8與setLogicFlags8；不改22／02／08、word／segment／67／F0／F2／F3或原flags helper。66仍依既有工具範圍拒絕，不外推硬體prefix支援。

register取兩舊值後只寫目的byte與五定義flags；其餘R高位、段、FPU／RAM保持。memory先decode完整位址、來源讀取成功，再單byte寫回，寫回成功後才發布flags；來源或寫回失敗保持R／段／flags／FPU。工具失敗可已消耗指令bytes，錯誤EIP沿現有模型；不是硬體rollback。結果不變仍正常寫一次。AF清0僅工程模型，非原硬體已證實。

## 驗收與原版續行

工程oracle以逐bit除法建立AND值，以popcount／零與符號建立flags，與production位元運算獨立。全byte pair×八來源×兩flags初態、全部register別名／重疊、全ModRM與SIB／相異DS SS、signed位移／wrap／段末、未知段／唯讀／越界／read與write拒絕、每byte截短及前綴拒絕。全R／段／nonzero FPU／鄰RAM保持，單byte寫回與flags成功發布契約有窄測。既有OR08／AND22／ADD02回歸保持。

CPU審查後加入獨立20分支，剝除新分支後逐byte等於368 CPU。窄測與固定官方EXE乾淨Go全套通過才執行同368正常fixture一次；所有舊原列到165113094拒絕前與37PNG保持，僅新有界observer列不混入舊列。私有probe增加首個169E49起最多三個正常CPU步，保存完整R／段／flags／FPU bits、code16／SS:ESP stack16、原step error與RAM只讀；原AND結果／精確EIP、下一POP與RET按原stack獨立核算。不注入核心、不加cap／改輸入／重送。

沿368私有可寫profiles與180M／1996-01-01，原418來源前後SHA-256保持；state保存實際大小／hash／UID／GID。原SAVE10.GAM及MOX.SET寫入前綴與368保持，後續state可能依法新增或改寫，按收據明示。遇新CPU拒絕或其他玩家阻塞即保存，不稱完整開局。原旗色持久語意／正式讀檔／RNG與remake同狀態未驗，主庫玩法RE-first保持。

## 工具與權利

既有Go1.24.13 image sha256:1a6d4452c65dea36aac2e2d606b01b4a029ec90cc1ae53890540ce6173ea77ac，600s／2GiB／2CPU／128pids／UID1000／network none，原ZIP／patch唯讀。新增本規格同次掛000-index；公開自製CPU／測試／spec／索引／368回填，私有profiles／原LOG／PNG／RAM／state不入Git。workplace/new-game-369-run.sh為原版入口，main研究沿既有docs/re/dosgolem-moo2-intake-20260930.md。

## READY審查

原368收據直接核對20D0、AL00／DL01與完整R／段／flags216h／FPU，CPU仍b8c18441…且沒有20分支。169E4A是拒絕後fetch opcode的定位，真正輸入仍169E49；不從after EIP推定指令已執行。Intel80386的20 /r與旗標表已核對，AND結果00／CF0 OF0 SF0 ZF1 PF1足夠；AF沿既有清0工具模型，完整工具flags應246h。memory沿291既有單byte成功發布契約與32位address helper，未知／失敗仍明示拒絕。審查腳本new-game-369-ready-review.py通過，369轉READY；只改工具CPU與自製測試，不改主庫玩法或既有輸入。

## CONFORMED限定結果

狀態：**CONFORMED，限定裸20 /r工具能力、原AND／POP／RET與相同輸入180M續行**。原完整開局、正式讀檔及remake同狀態未驗。

已證實原165113094在dosgolem_high_le:169E49執行20D0，AL00與DL01得到00；EIP169E4B，完整R保持，flags216h→246h。CF／OF／SF0、ZF／PF1符合Intel定義；AF清0僅既有工具模型。原165113095在169E4B執行59，SS188:ESP2BDB00真實stack給ECX0F，ESP2BDB04／EIP169E4C；165113096原C3依真實stack返回14DC1E，ESP2BDB08。兩步flags246h保持，其餘R／段／完整FPU保持；三筆observer只讀、RAM不變、error nil，callback12／12與IRQ43071／43071完成且inactive。CPU測試中的stack是工程fixture；上述返回值另由同native收據證實。

裸20分支剝除後逐byte等於62cd491的CPU，其他受版控internal及公開probe保持。獨立窄測1.011s通過，含八來源×全部byte pair×兩flags初態的1048576組memory值、全部register別名／重疊、ModRM／SIB及失敗發布契約。固定官方EXE乾淨Go全套通過，CPU38660.343s、machine1.861s；未掛8088外部測試資料，不外推該驗收。三私有observer變更逆轉後等於368。

原guest只跑一次，相同正常輸入、可寫state、180M cap及日期保持。368拒絕前11017共通原列按352既有mtime／DTA及每輪RAM雜湊正規化後保持，37PNG逐byte保持；本輪共39PNG。save_dos_diagnostic保留服務是否改RAM的關係，只正規化每輪雜湊。初版驗證誤把只讀observer當DOS服務也不寫RAM，原3F確實會寫RAM，故修正判準；另移除舊拒絕專屬四筆terminal診斷，窗口止於原late_startup_platform label=stop之前。這兩項是驗證語意與比較窗口修正，沒有重跑guest或變更原結果。四CLI拒絕及合法缺EXE正對照保持。

實際step_limit=180000000／EIP235AA3／unique_sites53798，無guest_cpu_stop、step_error或dos_exit。180M虛擬418789381µs，R=[0 18 0 0 2BD3A8 2BD3D8 3CA527 34CFD9]、段=[8 188 188 0 20 188]、flags246h、FPU127F／status0／depth0／八stack bits0；callback12／12、IRQ47499／47499完成且inactive，IF1。終圖親看星圖上的Enter Home Star Name、Sol候選與ACCEPT，尚未輸入確認。實際表指標298848、count3、stride55、165bytes SHA-256 d12f33061ee90916f4c35c9ee5177fc4e3760fb4ca791f572935e139c084d2ff；globals69a0c2f011924ac7d4f6960ddb98df646f158d49b48f0f43ed72c92cfa0956b9，header1eb307e414824282e81d17faf0ac5ab6752ae0f074ecd526f3cb4ce16847011c。這是新命名初態，不能套用舊旗色550byteguard或直接注入選擇。

同guest有界只讀state監測副本與終態一致：SAVE10.GAM208000bytes／0cf71fc5368861758da5a5dffbd791f8b711a48cfd295572eb9a21d27dea0f4d，MOX.SET553bytes／de8554b3284c9f9065efe6e4f2dfa71ab0c7e732e75eaea0d0ecf465ab86409f；sound.lbx4250888bytes保持。三檔UID／GID1000，全部state與368終態相同，原418來源前後保持。中途truncate或部分寫入副本不當終態；監測550s有界、原外層600s／2GiB／2CPU，容器與owned背景程序完成後清理。

## 回填與下一步

| 不可變鍵 | 語意與等級 | 受影響規格 | 回填 |
|---|---|---|---|
| 官方1.31／dosgolem_high_le:169E49／20D0 | 原AND與後續59／C3已證實，AF僅工具模型 | 339、341、362、367、368 | 舊368拒絕保持歷史來源，本369解出CPU阻塞，未證完整開局 |

本輪CPU SHA-256 a39e5b9f74e026fc1c2514d733808e94920b6789d1b68ef8eb8960dc72df5350。公開自製CPU、窄測、本規格、索引與五份回填；私有probe／LOG／PNG／RAM／state不入Git。主庫玩法RE-first保持。

實際Docker入口：
```text
python3 workplace/new-game-369-ready-review.py
bash workplace/new-game-369-full-run.sh
bash workplace/new-game-369-run.sh
python3 workplace/new-game-369-verify.py
```

下一步由本369實際180M表與完整CPU／FPU／clock／VBE／callback／IRQ建立母星命名正常ACCEPT契約，先核對候選字串與原返回端再READY。維持正常裝置輸入、受控release與首次匹配失敗即停，不代寫核心或RAM。固定日期不代表RNG seed；原名稱與旗色持久語意、正式讀檔、完整開局與remake同狀態仍未知。

## 370母星候選來源回填

[370](370-moo2-home-name-source.md)沿本規格相同輸入，以只讀原表DS188:298848 index2+24確認DS188:28439D的32byte為Sol補零，170M／180M保持。170M完整CPU／FPU／VBE／clock、target8:2136D1、mask2B／callback12／12及IRQ44492／44492已取，未送母星確認。index1+24→261AC2是原始窗口，非直接ACCEPT標籤；正常確認及持久writer仍未知。14282共通原列及39PNG保持，補觀察不改本輪原續行範圍。

## 371正常母星名稱確認回填

[371](371-moo2-home-name-normal-accept.md)已沿370原170M完整前置一次正常press，原24C31B查詢讀到BX1／CX550／DX260，首次安全release後，170024419原20DDDB／66A3A6C42600真正寫DS188:26C4A6 word0000→0100；32byteSol候選保持，命名視窗消失、回到星圖已親看。這是母星確認上下文，統治者名稱與正式持久writer不由共享buffer／store推定。後續174213914於1749C0／F6EC遇新CPU拒絕，未達180M／完整開局；原CPU與本文件舊收據保持。
