# 270 — 32 位元記憶體目的與暫存器的比較

狀態：**CONFORMED（有限 CPU 與首個分支範圍）**
日期：2026-10-01
範圍：CPU 的 `39 /r` dword 記憶體目的、無 operand-size／segment／REP 前綴，不改 remake 玩法。

## 證據

- **已證實，自生停點**：[269-cpu386-imul-dword-register.md](269-cpu386-imul-dword-register.md) 的固定官方 1.31 ORION2.EXE SHA-256 `4e11be14217b4aafa1839f333bf5eba037f98b0c44e9e4752c96c464c260419f`，dosgolem `25b8e79e332bb8bb1359284e8d10822462a70a9b`。無事件／受控事件第 6,738,950／6,738,985 步在 **dosgolem 高位 LE 線性** `0x234C9A` 的 `39 0D 9B 0D 27 00 0F 8D 01 02 00 00 80 3D 94 0D` 拒絕。ECX=18h、EAX=EBX=EDX=0、EFLAGS=246h；新增只讀觀測確認 DS:00270D9Bh 的目的 dword=0，readable=true；兩條初始樣本均相同。
- **公開 CPU 契約**：[Intel 80386 CMP](https://www.ardent-tool.com/CPU/docs/Intel/386/manuals/prref386/CMP.htm)：39 /r 計算記憶體目的減暫存器來源，更新 CF／PF／AF／ZF／SF／OF，不寫回結果。與 3B /r 的來源方向不同。
- **已證實，同次輔助樣本**：DOSBox-X CS:EIP `0180:00368C9A → 0180:00368CA0`，目的 DS:0039ED9Bh 前後 dword 都為 0；資料位址由原指令 bytes 讀出。兩種位址空間與絕對資料布局不得當成一致。
- 現有 39 只收暫存器目的；3B 已用 decodeAddress32／readSegment32，sub32 已共用其他 CMP 形狀。全域前綴閘門在進入 39 前即拒絕 REPNE；記憶體與既有暫存器形狀皆保持此拒絕行為，不能由局部分支缺少 repne 判斷推論成接受。

## 擬議契約與驗收

對 mod 不為 3 的 39、無前綴形狀，以現有 32 位元解碼讀完整目的 dword，再以 sub32 比較 `memory - R[reg]`；暫存器、段與記憶體不寫回，非算術旗標保持。DS／SS 預設、位移與 SIB 由既有地址層處理；截短、無法讀完整目的及未知前綴明確拒絕，旗標不得發布半次結果。既有 39 暫存器與 3B 方向保持。

測試八個來源、正負／借位／溢位／零／同位／輔助借位、各記憶體地址形狀與 DS／SS 消費、分段界線、bus 讀取失敗與非寫回哨兵。加入實際原版記憶體輸入、旗標與第一個 JGE 分支，再跑固定 EXE 全套與兩條自然路徑。只核對有限 CPU 規格與使用點；不推論圖形 helper 用途，不把黑圖或局部樣本稱為正常玩家路徑。


## 固定輸入與同次結果

原版 ZIP 根層 417 檔 SHA-256 `3a28a52f5953ff6d8fc251548940500236752ee19b52b51581e71ec1a3373c2f`、官方 1.31 patch ZIP `908d6b7b37ad580039c5d108bab2c64b28f51ba735485287d284d5f5242b98e5`、上述 EXE、553 bytes MOX.SET `bfd6855a41760b31156b96114b5b33c88f442ab8f8aae020c1740b3b486a3a80`。Go 1.24.13；dosgolem 僅加入不注入狀態的只讀觀測，兩條自然初始輸入 gzip SHA-256 `667e3ba82cf5e450330748a7c00c5b22d6b61b30d5934f4786d896dd87882401`／`ae38ddb773504e2968e9a7b3064564122d3494664c17ed6a2c1564093daac135`，命令沿用 269。

DOSBox-X 2026.07.02 SDL2 heavy debugger、映像 ID `sha256:659e8abbf93646f59a1586341769bd4b8f3cd1c707859d7a5de4c56e4672b582`，`startup_probe_131.py --cmp-memory-register` 擷取 16 bytes=`39 0D 9B ED 39 00 0F 8D 01 02 00 00 80 3D 94 ED`。原版記憶體目的前後皆零，ECX=18h、EAX=EBX=EDX=0、ESI=A5940h、EDI=3CFE38h、EBP=3EBBA8h、ESP=3EBB74h、DS／ES／SS=188h、FS=0、GS=20h；EFLAGS=246h → 297h，其餘擷取狀態保持。[Intel Jcc](https://www.ardent-tool.com/CPU/docs/Intel/386/manuals/prref386/Jcc.htm) 定義 JGE 為 SF=OF；原版有界 LOG 2 及分支後 EV 確認 **CS:EIP** `0180:00368CA6`，實際不取分支，EFLAGS 仍 297h。沒有追下一個 byte CMP 的用途或圖形 helper 內部。

第一次 LOG 1 只記錄分支前 EIP=00368CA0，不能當作分支結果；擷取入口改為 LOG 2 並要求分支後定位，乾淨重跑後才作 READY 依據。最終私有 JSON／消費 LOG／終端 SHA-256 `af5583d7dd5477aba5b864b1c48f345e6b8c3b8c53c3269f88a7af2ce505726d`／`a98188c1d69a6bf02a2dcfa32e7e8d716ddc7df416e4956b25d5e327f054a490`／`8171fce2bbdb15f16f5fc585bcdc41849d4e11545a2e3372b76dbef9f3935e1a`。完整資料留本機，最小字節與具名狀態才可入回歸。

## READY 審查

Intel 已定義 39 /r 的目的減來源、完整六個算術旗標及非寫回；既有 address32／segment 讀取與 sub32 可沿用。實際兩側目的值、原版完整旗標與第一個 JGE 不取分支已核對，資料絕對位址差異明示。新形狀僅無前綴 dword 記憶體，既有暫存器與 3B 方向保持；截短或目的讀取失敗前不得發布旗標。八來源、正負邊界、DS／SS／SIB、讀取失敗、非寫回、原版最小樣本及自然重跑驗收已列明。DRAFT 審查後轉 READY，才實作；不追 helper 內部或修改玩法。


## 前綴勘誤與重新審查

初次 CPU 測試在新增的既有 REPNE 暫存器正例失敗，輸出 SHA-256 `5c26daa225e8d4172d9a7afd29aa9236a6f0934dc4bc29b2f0e7cc16ca08425a` 留於 `workplace/cpu-test-270-first-failure.txt`。這是新增規格／測試誤讀局部解碼，不能當成產品新缺陷。檢視 CPU Step 的全域 `repne && op != AE && op != A4 && op != A5` 之拒絕，證明 F2 39 原本就不接受；原版 CMP、記憶體方向與旗標證據不受影響。規格先回 DRAFT，訂正前綴事實；重新審查後保留全域閘門，改驗 F2 暫存器亦拒絕，不為通過測試放寬正式行為。


**重新 READY 審查**：固定 CMP 與 JGE 原版證據不變；全域前綴閘門先於局部分支執行，已有原始控制流及首次測試拒絕共同證明 F2 39 不接受。保留拒絕並把它納入負例；去掉重複的不可達局部 REPNE 檢查，不改全域行為。完整記憶體讀取後才發布旗標與無寫回驗收不變，重新 READY 後再訂正實作／測試。


第二次 CPU 測試的既有 3B 記憶體回歸樣本漏設 DS descriptor，輸出 SHA-256 `2b85fcb41501bc21da070ea6d5eaba6cb99e22bfc7f244df1001fe65ab44ce0a` 保存於 `workplace/cpu-test-270-second-failure.txt`。segmentLinear 本來就要求已登錄描述子；同一回歸樣本補上 DS=188h、Base=0、Limit=79 後重跑。這是新增測試環境不完整，正式解碼、地址層與拒絕閘門不變，不刪除回歸項或放寬讀取。


## CPU 實作與驗收

重新 READY 後，39 的記憶體分支以 decodeAddress32 取得 DS／SS 與地址，再 readSegment32 完整讀取目的，最後 sub32 計算目的減來源，只更新算術旗標。既有全域前綴拒絕、39 暫存器與 3B 方向不變。

`cmp_memory_register_test.go` 覆蓋八來源、16 個邊界值成對輸入，以有號較寬差、無號／半位元組借位與低 byte 同位建立獨立旗標預期；另驗 DS／SS、base、負 disp8／disp32、SIB 的無 base／比例索引、32 位元地址環繞、來源與地址暫存器別名、唯讀資料、段與記憶體哨兵。分段越界、bus 僅可讀 0–3 bytes、截短／前綴拒絕與既有 39／3B 回歸皆通過。原版具名架構樣本將資料目的移至 DS:32，保持相對 JGE bytes；EFLAGS=297h、JGE 不取分支與資料不寫回皆驗證。最小重定位測試不是正常玩家路徑或完整資料布局一致。

Go 1.24.13、映像 ID `sha256:1a6d4452c65dea36aac2e2d606b01b4a029ec90cc1ae53890540ce6173ea77ac`，以同一 `GOMAXPROCS=2 go test -p 2 -buildvcs=false ./internal/cpu386 -count=1` 乾淨重跑通過，私有輸出 SHA-256 `f741b064681cdc3ff79151be22d7af90cd735dbc7c87d2bb1374dedb8fb1a4c4`。兩次失敗分別為局部前綴判斷誤讀與回歸資料段漏設，不改正式算術或降低驗收範圍。

解析回填不可變鍵為固定 EXE 雜湊＋dosgolem 高位 LE `0x234C9A`／DOSBox-X CS:EIP `0180:00368C9A`＋`39 0D`。269 必須保留「記憶體 CMP 停點已由規格 270 接通」及本檔連結；`--check-cmp-memory-register-spec-backlinks` 核對，缺定位或舊標記必拒絕；全部既有回填函式亦通過。


## 全套與兩條自然路徑

加入實際最小樣本後，固定輸入的 `DOSGOLEM_MOO2_EXE=/tmp/game/ORION2.EXE GOMAXPROCS=2 go test -p 2 -buildvcs=false ./... -count=1` 全套通過，私有 `workplace/full-test-270.txt` SHA-256 `58df4137851ac567cdc133a724ff14cbc69a38b77662f15b9d1a0856fec26b20`。

`DOSGOLEM_MOO2_SEPARATE_DOS=1 go run -buildvcs=false ./workplace/moo2-probe /tmp/game/ORION2.EXE --game-dir /tmp/game`，另一路加 `DOSGOLEM_MOO2_MOUSE_EVENT_AFTER_POSITION=1`。兩條自然路徑的前三次目的值為 0、1、2，ECX=18h，不注入資料；無事件第 6,772,679 步、受控事件第 6,772,714 步在 **dosgolem 高位 LE 線性** `0x228DCE` 的 `66 39 07 7C 03 66 89 07 83 C7 02 66 39 1F 7F 03` 拒絕，原因是 39 operand-size word 尚未支援。EAX=0、EBX=9Fh、ECX=1DFh、EDX=1、ESI=2C0864h、DS／ES／SS=188h、EFLAGS=213h、DOS 呼叫 6；拒絕後 EIP=0x228DD0，受控回呼 started=1／completed=1。gzip SHA-256 `6c38fdc4b3ab55211ef9e53cebfa554551f069b27c4beb560263eea3ed0358f5`／`266791cd8a54111ef0053386ad52409df3c3c13ea8674cca0af8ad21fb2a0275`。

Active=true、Bank=7、StartY=512、BankSets=6、Writes=307200、DisplaySets=1；有效頁索引 SHA-256 `7818f5542a0404157573be6cffc0e0c8e68ce3c0f5d17d07ccdd9313fb700baf`、RGB `0b150fd32588b1daca5569992ebe559c0102c837306b1af4c44d35128ec58366`。另設 `DOSGOLEM_MOO2_VBE_PNG` 生成兩份 `workplace/moo2-vbe-270-*.png`，SHA-256 均 `1610444d26adb3135e7e933dd44912044bb636728af59c70e614945278d3c622`，與已檢視黑圖相同。270 僅 dword 記憶體目的比較、完整旗標與第一個 JGE 分支 CONFORMED，其他平台差異與 255／正常玩家畫面／音效／亂數／玩法同狀態仍保留。


## 後續停點解析回填

**word CMP 停點已由規格 271 接通**：[271-cpu386-cmp-word-destination.md](271-cpu386-cmp-word-destination.md) 已依 Intel word 契約與同次目的值／完整旗標／實際 JL 審查 READY。保留 270 的 dword 原始收據；原本 word 前綴拒絕負例由 271 正式 word 正例取代，其餘未知前綴／截短／資料讀取拒絕不變。271 自然重跑結果是下一現況入口，不代表完整資料布局與正常玩家路徑已驗。
