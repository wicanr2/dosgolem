# 268 — 單運算元的 16 位元有號暫存器乘法

狀態：**CONFORMED（有限 CPU 與首個消費範圍）**
日期：2026-10-01
範圍：32-bit CPU 的 `66 F7 /5`、ModRM mod=3；不修改 remake 玩法。

## 證據

- **已證實，自生停點**：[267](267-cpu386-add-word-register-immediate.md) 的固定官方 1.31 ORION2.EXE SHA-256 `4e11be14217b4aafa1839f333bf5eba037f98b0c44e9e4752c96c464c260419f`，dosgolem `c0c1487222b5d0544f9551e78bf76c0a9e7fae43`。無事件／受控事件分別第 6,738,883／6,738,918 步，在 **dosgolem 高位 LE 線性** `0x234B43` 的 `66 F7 EB` 拒絕。EAX=1、EBX=5、ECX=EDX=0、DS／ES／SS=188h、EFLAGS=246h。
- **公開 CPU 契約**：[Intel 80386 IMUL](https://www.ardent-tool.com/CPU/docs/Intel/386/manuals/prref386/IMUL.htm)：F7 /5 word 將帶符號 AX 與來源低 word 的完整乘積寫入 DX:AX；CF／OF 在完整乘積不能表示成低 word 的符號延伸時同時設置，否則清除。SF／ZF／AF／PF 未定義。
- **已證實，同次輔助樣本**：DOSBox-X 2026.07.02 SDL2 heavy debugger、映像 ID `sha256:659e8abbf93646f59a1586341769bd4b8f3cd1c707859d7a5de4c56e4672b582`，`startup_probe_131.py --imul-word-register` 命中 **CS:EIP** `0180:00368B43 → 0180:00368B46`。16 bytes=`66 F7 EB A3 7A ED 39 00 33 DB 33 C9 33 C0 33 D2`；第一個 A3 的 DOSBox-X 絕對操作位址與 dosgolem `7A 0D 27 00` 不同，不能宣稱完整記憶體布局一致。EAX=1、EBX=5、ECX=EDX=ESI=0、EDI=3A2090h、EBP=3EBBA8h、ESP=3EBB74h、DS／ES／SS=188h、FS=0、GS=20h、EFLAGS=246h；乘法後 EAX=5、EDX=0、其餘擷取暫存器／段保持，EFLAGS=206h。CF／OF 與規格一致，但原版未定義 ZF 清除，dosgolem 保存；差異明列，不算逐值旗標對拍。
- 現有 word MUL 僅修改 CF／OF，保存未定義旗標。沿用這項明示的工程近似，不宣稱未定義旗標原版逐值一致。

## 擬議契約

無 segment／REP 前綴且 word、mod=3、group=5 時，接受八個來源。寫回前先讀兩來源，AX／DX 別名不能受提前寫回污染。完整有號 32 位元乘積分拆到 AX／DX，兩個暫存器的高半部、其他暫存器、段與記憶體保持。CF／OF 依有號 word 可表示範圍更新；其他旗標保持，是平台近似。EIP 前進三個 bytes。未知前綴、記憶體形狀與截短維持明確拒絕，狀態不發布半次結果。

## 驗收

八來源與正／負／零、32767／-32768、正負溢位邊界、AX／DX 別名、完整積／高位保存、定義旗標、未定義旗標近似與其他狀態哨兵。保留既有 word MUL、兩／三運算元 IMUL 與拒絕形狀。加入實際原版最小樣本及第一個 A3 存值，再跑固定原檔全套及兩條自然路徑。補上原始定位回填護欄與索引；黑圖不算正常玩家畫面，255 與玩法同狀態仍未完成。


私有 JSON／第一個消費端 LOG／完整終端 SHA-256 `50c8e64a5a3905af139746bd6398f0b2a9716ce2c46b6167e5e944e45065397a`／`99185da5a3ed77b85fe0e333bbb4f0d1e860cbb28b99e0f505479adb0e254d3c`／`e35f0a6c285c7837960f1497a50ac777e560ffe260e16d195831648c9783afa2`。完整資料留本機；架構最小樣本與指令 bytes 可作回歸，A3 資料位址移至測試專用位置並明示。

## READY 審查

Intel 定義完整有號 DX:AX 乘積與 CF／OF，其他算術旗標未定義。原版輸入及同次輔助結果已核對；保存未定義旗標沿用既有 word MUL 的平台近似，第一個 A3 不讀旗標，下一 XOR 會重新定義算術旗標。限定八個暫存器來源及無 segment／REP 形狀，先讀後寫解決 AX／DX 別名；拒絕界線與完整積／高半部／定義旗標／首個消費驗收已列明。DRAFT 審查後轉 READY，才實作；不追 helper 內部或推論玩法。

## 實作與 CPU 驗收

READY 後，`internal/cpu386/cpu.go` 加入 word 暫存器 group 5 分支。乘積先以有號 32 位元計算，再發布 DX:AX，保留兩個高半部；只改 CF／OF。測試 `imul_word_register_test.go` 覆蓋八來源、15 個帶符號邊界值的成對輸入及兩種初始旗標，特別核對 AX／DX 別名讀取、完整積與符號延伸判準、其他暫存器／段／記憶體保持、未定義旗標保存近似及未知／截短拒絕。既有 word／dword MUL、兩／三運算元 IMUL 回歸通過。

實際原版具名輸入另有最小測試。DOSBox-X A3 操作位址 `0039ED7Ah`，dosgolem LE 原指令為 `00270D7Ah`；測試只把存值目的移至資料位址 64，確認完整 EAX=5 被存值，再由 XOR 重定義算術旗標。原版 IMUL EFLAGS=206h，dosgolem=246h；只比較定義 CF／OF，明示未定義 ZF 差異。沒有把重定位測試稱為正常玩家路徑或完整記憶體同狀態。

Go 1.24.13，映像 ID `sha256:1a6d4452c65dea36aac2e2d606b01b4a029ec90cc1ae53890540ce6173ea77ac`，`GOMAXPROCS=2 go test -p 2 -buildvcs=false ./internal/cpu386 -count=1` 通過，私有輸出 `workplace/cpu-test-268.txt` SHA-256 `df1dd325364a58f2ca750e8d922e3c58215fa9ee6715f8602ff7368023a697eb`。

解析回填不可變鍵為固定 EXE 雜湊＋dosgolem 高位 LE `0x234B43`／DOSBox-X CS:EIP `0180:00368B43`＋`66 F7 EB`。267 須保留「word IMUL 停點已由規格 268 接通」及本檔連結；`--check-imul-word-register-spec-backlinks` 自動核對，缺定位或舊標記必拒絕。


## 全套與兩條自然路徑

固定正版 ZIP SHA-256 `3a28a52f5953ff6d8fc251548940500236752ee19b52b51581e71ec1a3373c2f` 根層 417 檔、官方 patch ZIP SHA-256 `908d6b7b37ad580039c5d108bab2c64b28f51ba735485287d284d5f5242b98e5`、上述 EXE 與 MOX.SET SHA-256 `bfd6855a41760b31156b96114b5b33c88f442ab8f8aae020c1740b3b486a3a80`。`DOSGOLEM_MOO2_EXE=/tmp/game/ORION2.EXE GOMAXPROCS=2 go test -p 2 -buildvcs=false ./... -count=1` 全套通過，私有輸出 SHA-256 `e75040d49e081e8551dc94d06d5edfbf7cb68e4696848cdc15aaacd192125331`。

`DOSGOLEM_MOO2_SEPARATE_DOS=1 go run -buildvcs=false ./workplace/moo2-probe /tmp/game/ORION2.EXE --game-dir /tmp/game`，另一路加 `DOSGOLEM_MOO2_MOUSE_EVENT_AFTER_POSITION=1`。無事件第 6,738,892 步、受控事件第 6,738,927 步，自行越過 word IMUL 與 A3，在 **dosgolem 高位 LE 線性** `0x234B5F` 的 `F7 EB 33 DB 33 D2 66 8B 1D 42 3A 2A 00 03 C3 A3` 拒絕，原因是 dword F7 /5 尚未支援。EAX=F0h、EBX=280h、ECX=EDX=0、DS／ES／SS=188h、EFLAGS=246h、DOS 呼叫 6；拒絕後 EIP=0x234B61。受控回呼 started=1／completed=1。私有 gzip SHA-256 `92d63d22a845d695808fad6d51a1a94faff3eb020db20a977483e0ab4c5140e6`／`391af2813ec33e2602aae500de1775797917219b1946783cdf41f30c9b571387`。

Active=true、Bank=9、StartY=512、Writes=307200。索引 SHA-256 `7818f5542a0404157573be6cffc0e0c8e68ce3c0f5d17d07ccdd9313fb700baf`、RGB `0b150fd32588b1daca5569992ebe559c0102c837306b1af4c44d35128ec58366`；另設 `DOSGOLEM_MOO2_VBE_PNG` 產生兩份 `workplace/moo2-vbe-268-*.png`，SHA-256 均 `1610444d26adb3135e7e933dd44912044bb636728af59c70e614945278d3c622`。與已檢視黑圖逐位元相同，不重做目視檢查或聲稱玩家畫面完成。268 僅在 word 暫存器有號乘法／CF／OF 與首個消費範圍 CONFORMED；255、正常玩家畫面、音效、受控亂數與 Go remake 玩法同狀態仍未完成。


## 後續停點解析回填

**dword IMUL 停點已由規格 269 接通**：[269-cpu386-imul-dword-register.md](269-cpu386-imul-dword-register.md) 已依公開 CPU 契約與同次輔助樣本審查 READY。保留 268 的原始停點與收據；269 自然重跑結果是下一個現況入口。字組／雙字組分別驗收，不把未定義旗標或平台布局差異升格為一致。
