# 271 — 16 位元目的與暫存器的比較

狀態：**CONFORMED（有限 CPU 與首個分支範圍）**
日期：2026-10-01
範圍：32-bit CPU 的 `66 39 /r`，word 記憶體或暫存器目的，無 segment／REP 前綴，不改 remake 玩法。

## 證據與擬議契約

[270-cpu386-cmp-memory-register.md](270-cpu386-cmp-memory-register.md) 的固定 1.31 ORION2.EXE SHA-256 `4e11be14217b4aafa1839f333bf5eba037f98b0c44e9e4752c96c464c260419f`，無事件／受控事件第 6,772,679／6,772,714 步在 **dosgolem 高位 LE 線性** `0x228DCE` 的 `66 39 07` 拒絕。EAX=0、EBX=9Fh、ECX=1DFh、EDX=1、EFLAGS=213h，新增只讀觀測確認 DS:[EDI] 的 word=0、readable=true，兩條初始樣本均相同。DOSBox-X 候選 **CS:EIP** `0180:0035CDCE → 0180:0035CDD1` 已由同次輔助樣本核對，不用代碼位址差替資料地址猜值。

[Intel CMP](https://www.ardent-tool.com/CPU/docs/Intel/386/manuals/prref386/CMP.htm) 定義 39 /r 的 word 目的減 word 暫存器來源，更新六種算術旗標、不寫回結果。既有 sub16 已供其他 CMP 使用；32 位元地址解碼與 readSegment16 已存在。讀完整目的低 word 後才發布旗標，所有暫存器完整 32 位元、段與記憶體保持；非算術旗標保持。暫存器目的以原低 word 比較，記憶體地址仍是 32 位元模式，DS／SS 與 SIB 沿用既有層。截短、界線與未審查前綴拒絕，不新增硬體時序或平台例外實作。

## 驗收

八來源與八暫存器目的、正負／借位／溢位／輔助借位／零／同位、高半部及非寫回；記憶體寬度恰為兩 bytes、DS／SS／SIB、地址別名、部分讀取失敗與拒絕界線。原版固定目的輸入、完整旗標與第一個 JL 分支；保留 270 的 dword 比較與 3B 方向。270 的 word 前綴拒絕負例須由 271 的正式 word 正例替代，其餘拒絕不放寬。固定原檔全套及兩條自然重跑後才限定 CONFORMED，正常玩家畫面／音效／亂數／玩法同狀態仍未完成。


## 同次輔助樣本與 READY 審查

原版輸入與工具沿用 270 的固定 EXE、417 檔與 553 bytes MOX.SET；dosgolem 以 `25b8e79e332bb8bb1359284e8d10822462a70a9b` 加 270 工作樹、只讀觀測為起點。兩條輸入觀測 gzip SHA-256 `d047677632fde99a29ceedcb0411a4a0342e3072ebf6835c881dc53dc6eb471e`／`39b806d659120da28f5df9b4aea40e2111123bc2c5aeec6c55cab6df7148214a`；不注入原版資料。

**已證實**：DOSBox-X 2026.07.02 SDL2 heavy debugger、映像 ID `sha256:659e8abbf93646f59a1586341769bd4b8f3cd1c707859d7a5de4c56e4672b582`，`startup_probe_131.py --cmp-word-destination` 命中 **CS:EIP** `0180:0035CDCE → 0180:0035CDD1`，16 bytes=`66 39 07 7C 03 66 89 07 83 C7 02 66 39 1F 7F 03`。EAX=0、EBX=9Fh、ECX=1DFh、EDX=1、ESI=451054h、EDI=3EC164h、EBP=3EBBA8h、ESP=3EBB50h、DS／ES／SS=188h、FS=0、GS=20h，DS:[EDI] 前後 word 皆 0；EFLAGS=213h → 246h，其餘擷取狀態保持。有界 LOG 2 與分支後 EV 實際確認 **CS:EIP** `0180:0035CDD3`，JL 不取分支、EFLAGS=246h。原版與 dosgolem 絕對指標／資料布局不同，沒有宣稱全狀態或畫面一致。

JSON／消費 LOG／終端 SHA-256 `dabfcb5897c4d8650cfe568442bc1352674f1938572f196a98bee9f35a591e25`／`58b1476265461c2ac090d47c7e9c5e93e5ce1053a6a6b3cd0be4c9f10b26ffd9`／`6ffb3a247d37d81b0f80ecb1c3b5accbea02092d0f465fd0b96788125278171a`；完整資料留本機，只提交最小語意樣本。

Intel 定義 word 目的減來源與六旗標，sub16、address32 與 readSegment16 已有共用消費。兩側初始 word 相同，原版完整旗標與 JL 實際不取分支已核對。範圍明列 word 記憶體／暫存器目的；高半部／段／記憶體不寫回、先完整讀取、未知前綴及部分讀取拒絕、保留 dword 與 3B 均有驗收。270 的 word 拒絕負例由本規格正式正例取代並回填，不默默放寬。DRAFT 審查後轉 READY，才實作；停止於有限 CPU／第一個分支，不追圖形 helper 內部。


## CPU 實作與驗收

READY 後，39 word 暫存器分支將兩低 word 交由 sub16，不寫回；記憶體分支共用 address32，完整 readSegment16 成功後才發布旗標。270 的原 word 前綴拒絕負例以本規格正式 word 正例取代，所有其餘前綴、部分讀取與既有 dword／3B 回歸保持。

`cmp_word_destination_test.go` 的八來源／八暫存器目的與 16 邊界值成對輸入，以較寬有號差、借位與低 byte 同位獨立驗旗標；八個記憶體來源另將 descriptor 限制在目的最後 byte，確認讀取恰為兩 bytes。高半部、其他暫存器／段／記憶體、DS／SS、負位移、SIB、環繞與來源地址別名、部分讀取與拒絕均通過。原版最小樣本只重定位 EDI 的目的地址，EFLAGS=246h、JL 實際不取分支與無寫回已驗；重定位不是完整布局一致。

Go 1.24.13、映像 ID `sha256:1a6d4452c65dea36aac2e2d606b01b4a029ec90cc1ae53890540ce6173ea77ac`，`GOMAXPROCS=2 go test -p 2 -buildvcs=false ./internal/cpu386 -count=1` 通過，私有 `workplace/cpu-test-271.txt` SHA-256 `03adc98d6c9e63eeb196207237c71e81a4ccaf8beea6a1efef1b6db17efb6bd2`。

解析回填不可變鍵為固定 EXE 雜湊＋dosgolem 高位 LE `0x228DCE`／DOSBox-X CS:EIP `0180:0035CDCE`＋`66 39 07`。270 須保留「word CMP 停點已由規格 271 接通」與本檔連結；`--check-cmp-word-destination-spec-backlinks` 核對，缺定位或舊標記必拒絕，全部既有回填函式亦通過。有限 CPU／分支驗收不代表正常玩家畫面、音效、亂數與 Go remake 玩法同狀態。


## 全套與兩條自然路徑

固定原版輸入的 `DOSGOLEM_MOO2_EXE=/tmp/game/ORION2.EXE GOMAXPROCS=2 go test -p 2 -buildvcs=false ./... -count=1` 全套通過，私有 `workplace/full-test-271.txt` SHA-256 `7b337b6d123caede6767cf2ca5ff314be56ac075420eecf8228ea73c6e607c37`。

`DOSGOLEM_MOO2_SEPARATE_DOS=1 go run -buildvcs=false ./workplace/moo2-probe /tmp/game/ORION2.EXE --game-dir /tmp/game`，另一路加 `DOSGOLEM_MOO2_MOUSE_EVENT_AFTER_POSITION=1`。兩條自然路徑前三次 DS:EDI 為 002C00D4h、002C00D8h、002C00DCh，word 皆 0、EAX=0、EFLAGS=213h；不注入資料。無事件第 6,822,373 步、受控事件第 6,822,408 步，在 **dosgolem 高位 LE 線性** `0x21CA3D` 的 `66 FF 40 04 A1 A8 42 2A 00 8A 40 0B 24 20 25 FF` 拒絕，原因是 word FF 目前只支援記憶體 DEC、尚未支援 INC。EAX=10000h、ECX=1DFh、EDX=0、DS／ES／SS=188h、EFLAGS=282h、DOS 呼叫 6；無事件 EBX=3DC050h、受控事件 EBX=3DD050h，布局差異明示。拒絕後 EIP=0x21CA40，受控回呼 started=1／completed=1。gzip SHA-256 `7fec533d3f45b0d9a8ddeb8d5807a01d895faf98f3c0e97743555a93834397f7`／`8b633c0bb5f40764ce156978a50293daacf5102f62b146ac7290499c15811a67`。下一步只核對公開 word INC 契約、實際目的值／CF 保存與後續消費，不追 runtime／圖形 helper 內部。

Active=true、Bank=7、StartY=512、BankSets=6、Writes=307200、DisplaySets=1。有效索引 SHA-256 `7818f5542a0404157573be6cffc0e0c8e68ce3c0f5d17d07ccdd9313fb700baf`、RGB `0b150fd32588b1daca5569992ebe559c0102c837306b1af4c44d35128ec58366`；另設 `DOSGOLEM_MOO2_VBE_PNG` 產生兩份 `workplace/moo2-vbe-271-*.png`，SHA-256 均 `1610444d26adb3135e7e933dd44912044bb636728af59c70e614945278d3c622`，與已檢視黑圖逐位元相同。

271 僅在 word 目的比較、完整旗標與第一個 JL 分支 CONFORMED；255 仍 READY，正常玩家畫面、音效、受控亂數與 Go remake 玩法同狀態未完成。IMUL 未定義 ZF 保存差異、DIV 旗標／例外、SAR 的 AF、DTA 保留區與平台布局限制仍保留；CPU 支援不計入玩法矩陣分母。
