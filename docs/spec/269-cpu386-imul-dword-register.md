# 269 — 單運算元的 32 位元有號暫存器乘法

狀態：**CONFORMED（有限 CPU 與首個消費範圍）**
日期：2026-10-01
範圍：32-bit CPU 的 F7 /5、ModRM mod=3、無 operand-size／segment／REP 前綴；不改 remake 玩法。

## 證據與擬議契約

[268](268-cpu386-imul-word-register.md) 的固定 EXE SHA-256 `4e11be14217b4aafa1839f333bf5eba037f98b0c44e9e4752c96c464c260419f` 兩條路徑，在 **dosgolem 高位 LE 線性** `0x234B5F` 的 `F7 EB` 拒絕。EAX=F0h、EBX=280h、ECX=EDX=0、EFLAGS=246h。候選 DOSBox-X CS:EIP `0180:00368B5F → 0180:00368B61` 已由同次輔助樣本核對，兩種位址空間不得混用。

[Intel 80386 IMUL](https://www.ardent-tool.com/CPU/docs/Intel/386/manuals/prref386/IMUL.htm) 定義 F7 /5 dword 的完整有號積寫入 EDX:EAX；CF／OF 在積不能表示成低 dword 的符號延伸時同時設置，否則清除。其他算術旗標未定義；沿用 word MUL／IMUL 保存近似，不宣稱原版旗標逐值一致。先讀兩來源，再發布兩個完整暫存器；EAX／EDX 別名須使用原值。EIP 前進兩個 bytes，其他暫存器、段、記憶體與非定義旗標保持。

## 驗收

八個來源、零／正負邊界、最小 int32 相乘、正負溢位、完整 64 位元積、高位、別名與狀態哨兵；未知／截短／記憶體形狀仍拒絕，保留 word 有號／無號與 dword 無號乘法。加入固定原版架構樣本與第一個 XOR 消費，再全套測試及兩條自然重跑。保存位址、雜湊、未定義旗標與完整布局差異；正常玩家路徑與玩法同狀態不因 CPU 測試升格。


## 同次輔助結果與 READY 審查

**已證實**：DOSBox-X 2026.07.02 SDL2 heavy debugger、映像 ID `sha256:659e8abbf93646f59a1586341769bd4b8f3cd1c707859d7a5de4c56e4672b582`，`startup_probe_131.py --imul-dword-register` 命中 **CS:EIP** `0180:00368B5F → 0180:00368B61`。16 bytes=`F7 EB 33 DB 33 D2 66 8B 1D 42 1A 3D 00 03 C3 A3`，後續 word MOV 的絕對位址與 dosgolem `42 3A 2A 00` 不同，完整資料布局不可稱為同狀態。

輸入 EAX=F0h、EBX=280h、ECX=EDX=ESI=0、EDI=3A2090h、EBP=3EBBA8h、ESP=3EBB74h、DS／ES／SS=188h、FS=0、GS=20h、EFLAGS=246h。乘法後 EAX=25800h、EDX=0，其餘擷取暫存器／段保持，EFLAGS=206h。CF／OF 清除與規格一致，原版未定義 ZF 清除；dosgolem 保存為明示近似。第一個 XOR EBX／EBX 重新定義算術旗標，不將未定義差異當成定義旗標一致。

私有 JSON／消費 LOG／終端 SHA-256 `08aaefe24232f9eb5dff628b1583b2ab6dbe3dcb6ceaa5adbc1636cb90e6b3ec`／`010f07822d2dbd81a3932abd7c59151884134b469f1a29c947e579d0e6715181`／`a9ff40ee8294f0416b770cb03778eca2580a311d29bf50f21c9db1edc20bc140`。完整原版資料留本機，最小架構樣本與固定指令可作回歸。

Intel 定義完整 64 位元有號積與 CF／OF，已確認固定原版形狀及實際積；其他旗標保存沿用既有近似。兩來源先讀後寫處理 EAX／EDX 別名，八來源及邊界／拒絕／消費端驗收已列明。DRAFT 審查後轉 READY，才實作；不放寬記憶體或其他前綴，也不追 helper 內部。

## 實作與 CPU 驗收

以隔離 dosgolem `c0c1487222b5d0544f9551e78bf76c0a9e7fae43` 加規格 268 工作樹為原版重跑起點，READY 後加入 dword 暫存器 F7 /5 分支。以有號 64 位元先取得完整積，再發布 EDX:EAX；只改 CF／OF，保存其他旗標為近似。`imul_dword_register_test.go` 使用任意精度整數獨立產生積與溢位預期，覆蓋八來源、15 個邊界值成對輸入與兩種旗標，驗 EAX／EDX 別名、高 dword、其他暫存器／段／記憶體、截短／前綴／記憶體形狀拒絕。word 有號／無號、dword 無號及兩／三運算元 IMUL 的既有測試保持。

實際原版最小架構樣本確認 EAX=`25800h`、EDX=0、CF／OF 清除，首個 XOR EBX／EBX 及下一 XOR EDX／EDX 保留 EAX 乘積並重定義算術旗標。原版 IMUL EFLAGS=206h，dosgolem=246h；未定義 ZF 保存差異不能寫成完整旗標一致。

Go 1.24.13、映像 ID `sha256:1a6d4452c65dea36aac2e2d606b01b4a029ec90cc1ae53890540ce6173ea77ac`，`GOMAXPROCS=2 go test -p 2 -buildvcs=false ./internal/cpu386 -count=1` 通過，輸出 `workplace/cpu-test-269.txt` SHA-256 `381ddab03d13d086873980fd1b8a8af26120e448c0ba3b8e7434555a7ddeb827`。

解析回填不可變鍵：固定 EXE 雜湊＋dosgolem 高位 LE `0x234B5F`／DOSBox-X CS:EIP `0180:00368B5F`＋`F7 EB`；268 必須保留「dword IMUL 停點已由規格 269 接通」與本檔連結。`--check-imul-dword-register-spec-backlinks` 自動核對，缺舊標記或定位必拒絕，全部既有回填函式亦通過。最終驗收範圍不包含正常玩家畫面、音效、受控亂數及 Go remake 玩法同狀態。


## 全套與兩條自然路徑

沿用 268 所列固定正版 ZIP 根層 417 檔、官方 patch、EXE 與 553 bytes MOX.SET，輸入雜湊不變。`DOSGOLEM_MOO2_EXE=/tmp/game/ORION2.EXE GOMAXPROCS=2 go test -p 2 -buildvcs=false ./... -count=1` 全套通過，私有 `workplace/full-test-269.txt` SHA-256 `866e3e9326063aaac6c508f8f63db64dc55a6e9fea08dee9bf8f27b5aa7b2aab`。

`DOSGOLEM_MOO2_SEPARATE_DOS=1 go run -buildvcs=false ./workplace/moo2-probe /tmp/game/ORION2.EXE --game-dir /tmp/game`，另一路加 `DOSGOLEM_MOO2_MOUSE_EVENT_AFTER_POSITION=1`。無事件第 6,738,950 步、受控事件第 6,738,985 步，自行越過兩種有號乘法及有限消費端，在 **dosgolem 高位 LE 線性** `0x234C9A` 的 `39 0D 9B 0D 27 00 0F 8D 01 02 00 00 80 3D 94 0D` 拒絕：CMP dword ModRM 0D 尚未支援。EAX=EBX=EDX=0、ECX=18h、ESI=A5940h、DS／ES／SS=188h、EFLAGS=246h、DOS 呼叫 6；拒絕後 EIP=0x234C9C，受控回呼 started=1／completed=1。gzip SHA-256 `70be009fe10982a86b2f9c9929ffbea27fd5a02dcee685d9d737d46c4877ce54`／`67a3821afbfa52561ced7685fff1fffb51f4a59cfef74649784ce5a03f77bba4`。下一步只核對公開 CMP 契約、實際記憶體輸入與第一個分支，不追圖形／runtime helper 內部。

Active=true、Bank=7、StartY=512、BankSets=6、Writes=307200、DisplaySets=1。有效索引 SHA-256 仍 `7818f5542a0404157573be6cffc0e0c8e68ce3c0f5d17d07ccdd9313fb700baf`，RGB 仍 `0b150fd32588b1daca5569992ebe559c0102c837306b1af4c44d35128ec58366`。另設 `DOSGOLEM_MOO2_VBE_PNG` 產生 `workplace/moo2-vbe-269-full-game.png`／`moo2-vbe-269-mouse-event.png`，PNG SHA-256 均 `1610444d26adb3135e7e933dd44912044bb636728af59c70e614945278d3c622`；與已檢視黑圖相同，不重做目視檢查。

269 僅在 dword 暫存器有號完整積、CF／OF 與首個 XOR 消費範圍 CONFORMED。未定義 ZF 差異、其他平台布局、DIV 旗標／例外、SAR 的 AF、DTA 保留區仍明示。255 仍 READY；正常玩家畫面、音效、受控亂數與 Go remake 玩法同狀態仍未完成。CPU 支援不計入玩法矩陣分母。


## 後續停點解析回填

**記憶體 CMP 停點已由規格 270 接通**：[270-cpu386-cmp-memory-register.md](270-cpu386-cmp-memory-register.md) 已依 Intel 契約、兩側目的值與原版完整旗標／實際 JGE 分支審查 READY。保留 269 的原始停點與收據；270 的自然重跑結果是下一現況入口。絕對資料位址、未定義乘法旗標與正常玩家路徑限制不因這項 CPU 延伸升格。
