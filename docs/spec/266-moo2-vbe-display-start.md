# 266 — MOO2 非零 VBE 顯示起點

狀態：**CONFORMED（有限起點與消費端，硬體規格近似）**
日期：2026-10-01
範圍：固定 MOO2 模式 0101h 的垂直顯示起點與索引／RGB 消費；不修改 remake 玩法。

## 證據

- **已證實，自生停點**：dosgolem `dea1659135461bd75c796657cbaf202674d32b47`，Go 1.24.13，官方 1.31 `ORION2.EXE` SHA-256 `4e11be14217b4aafa1839f333bf5eba037f98b0c44e9e4752c96c464c260419f`、正版 ZIP 根層 417 檔及固定 MOX.SET。規格 [265-moo2-vbe-window-control.md](265-moo2-vbe-window-control.md) 的兩條自然路徑切換區段 5–9 並寫入 307,200 bytes，無事件第 6,738,834 步、設定後事件第 6,738,869 步，在 **dosgolem 高位 LE 線性** `0x228CA7` 的 `CD 10 61 FC C3 00 00 00 00 60 66 8B 1D 5E 3A 2A` 拒絕。EAX=`4F07h`、BX=CX=0、DX=`200h`、EFLAGS=`246h`。完整輸入與診斷雜湊見 265。
- **公開平台契約**：[VESA VBE 2.0 Rev 1.1，Function 07h，印刷頁 29](https://www.phatcode.net/res/221/files/vbe20.pdf)：BL=0 設定、BL=1 讀回，BH=0；CX 是首像素，DX 是首掃描線。必須容納完整顯示頁；不可用起點時保持原狀。BL=80h 的回掃等待另屬時序範圍，本切片不接受。
- **既有固定模式**：規格 238／265 的 stride／width=640、height=480、8-bit 索引、2 MiB 顯存。Y=512 的起點是 `512 * 640 = 327680` bytes，恰為第 5 個 64 KiB 區段。不能繼續從零頁取畫面。
- **已證實，同次輔助返回**：DOSBox-X 2026.07.02 SDL2 heavy debugger、映像 ID `sha256:659e8abbf93646f59a1586341769bd4b8f3cd1c707859d7a5de4c56e4672b582`，`startup_probe_131.py --vbe-display-start` 命中 **CS:EIP** `0180:0035CCA7 → 0180:0035CCA9`，bytes=`CD 10 61 FC C3 00 00 00 00 60 66 8B 1D 5E 1A 3D`。與 dosgolem 前 12 bytes 相同，末段絕對資料位址不同，兩個位址空間不能混用。EAX=`4F07h`、EBX=ECX=ESI=0、EDX=`200h`、EDI=`3A2090h`、EBP=`3EBBA8h`、ESP=`3EBB80h`、DS／ES／SS=`188h`、FS=0、GS=`20h`、EFLAGS=`246h`；返回 EAX=`4Fh`，其餘擷取欄位保持。第一個 caller 為 POPAD／CLD／RET，未擷取當次完整 VRAM 差分。
- 私有 JSON／caller LOG／完整終端 SHA-256 `a4cd66a51d17c0c9d503bb36e9940236b3f5f05de539ccdfa7f265a09700c537`／`9ddaaebb3a2ab3fca0aad131a5c0e8acfa5920f96e7b24a5fa62453de5184881`／`730d9888d6a0d951d59a75ae6ed4eb9545cd2edaa61a92ee05e646e6edae1235`，完整收據留本機。

## 擬議契約

已附掛且啟用 0101h 時，直接 INT 10h 精確 EAX=`4F07h`／EBX=0，只接受完整 ECX=0、EDX 在 uint16 範圍，並以 uint64 驗證 `Y * stride + height * stride <= 顯存容量`。本固定模式的顯示寬等於掃描線寬，X 非零未建模；不猜水平環繞。成功更新起點，回完整 EAX=`4Fh`，其他暫存器／段／旗標保持。不修改顯存或目前視窗區段。

EBX=1 讀回 X=0／目前 Y 到 CX／DX，保留兩者高半部，回 EAX=`4Fh`；其他欄位保持。未知模式、EAX／BX 高位、X、子功能／回掃形式或無法容納完整頁的 Y，明確拒絕且不改架構與裝置。未啟用或未附掛仍只保留規格 237 的歷史精確全零設定，不把舊返回當作像素更新。

`VBEIndexed` 及 `VBERGB` 必須使用同一有效起點，快照仍是獨立複本；不用 Mem 的窗口內容冒充顯示頁。0101h 重設起點到零、既有清頁契約保持；03h 停用後無畫面快照。診斷保存起點、索引雜湊及實際寫入計數；可選本機 PNG 必須由當次 RGB 消費產生，不能公開原版美術或把檔案存在當作畫面對拍。

標為 **hardware-spec approximation（硬體規格近似）**，不模擬 S3、類比掃描、回掃等待、水平平移或完整 LE 序列化。規格 255 正常座標／游標、音效、受控亂數與 Go remake 玩法同狀態仍獨立待驗。

## 驗收

原版輔助探針取得實際 bytes、同次輸入／返回與第一個 caller；有限服務測試核對非目的欄位保持、讀回、合法邊界與未知形狀拒絕。透過 CPU／DPMI 共用映射寫兩個不同頁，設定起點後核對索引／RGB、區段保持、快照隔離與重設，不以直接設定 framebuffer 代替玩家寫入。固定 EXE 全套測試與兩條自然原檔重跑，記錄當次起點、索引雜湊與下一結果。237／265 回填，正負自動護欄、索引、擁有權及 Docker 清理一起核對。

## READY 審查

公開 VBE 契約與固定同次使用點足以定義 Y 的單位、成功返回及完整頁邊界；既有顯存映射已證實實際寫入落在第 5 個區段。只需將同一畫面消費端改用有效起點，無須追 BIOS driver。新裝置與模式重設回零；附掛但未啟用保留精確全零歷史返回，讀回／非零仍拒絕。存取檢查以 uint64 計算，狀態只在完整驗證後提交；區段與顯存不受設定影響。零／512／最後合法 Y／越界、未知形狀與兩條像素消費驗收已列出。DRAFT 審查後轉 READY，再實作。

## 實作與驗收

`moo2_vbe_video.go` 保存有效 Y，完成全部邊界驗證後更新；`VBEIndexed` 及 `VBERGB` 使用同一位移，不改視窗 bank 或顯存。`le_startup.go` 保存有限服務狀態，未啟用時保留歷史全零返回，0101h 重設回零。`moo2_vbe_display_start_test.go` 驗原版具名架構、讀回高半部保存、Y=0／512／2796 最後合法值、2797／65535／高位／X／未知子功能拒絕，以及透過 CPU MOV 與 DPMI 共用映射寫入不同頁後的整頁索引／RGB／遮罩消費、快照隔離、區段保持及重設。`workplace/moo2-probe` 可用 `DOSGOLEM_MOO2_VBE_PNG` 保存當次 RGB 的本機 PNG；編碼／關閉失敗明確報錯。

Go 1.24.13，映像 ID `sha256:1a6d4452c65dea36aac2e2d606b01b4a029ec90cc1ae53890540ce6173ea77ac`。`go test -buildvcs=false ./internal/machine -run 'TestMOO2(VBE|ProtectedVBE)' -count=1` 通過；包含原版樣本後 `DOSGOLEM_MOO2_EXE=/tmp/game/ORION2.EXE GOMAXPROCS=2 go test -p 2 -buildvcs=false ./... -count=1` 全套通過，`workplace/full-test-266.txt` SHA-256 `de01f4ef9623961307f6d7d4a432c0b2a47cd21a9d784be01416776799823bdf`。

**已證實，自生起點消費與下一停點**：同一固定 EXE／完整資料／MOX.SET／分離 DOS arena，`DOSGOLEM_MOO2_SEPARATE_DOS=1 go run -buildvcs=false ./workplace/moo2-probe /tmp/game/ORION2.EXE --game-dir /tmp/game`，另一路增加 `DOSGOLEM_MOO2_MOUSE_EVENT_AFTER_POSITION=1`。兩條均自行接受 Y=512，診斷 Active=true、Bank=9、StartY=512、BankSets=5、Writes=307200、DisplaySets=1；保留已寫入內容。無事件第 6,738,873 步、設定後事件第 6,738,908 步，在 **dosgolem 高位 LE 線性** `0x234B10` 的 `66 83 C3 18 81 FB E0 01 00 00 7C 13 33 DB 66 BB` 拒絕，EAX=0、EBX=`F0h`、ECX=EDX=0、DS／ES／SS=`188h`、EFLAGS=`246h`、DOS 呼叫 6；拒絕後 EIP=`0x234B13`。受控回呼 started=1／completed=1。

兩份 gzip SHA-256 `edc4f60dcc59ccd1237a0381e1eda792a108f04f9b99e4cf62a95374b2a6ca4a`／`81c282173de1eae0a7161767c7f2715319048a57888379d4d745ab7afd73a019`。有效起點索引 SHA-256 均 `7818f5542a0404157573be6cffc0e0c8e68ce3c0f5d17d07ccdd9313fb700baf`，RGB SHA-256 均 `0b150fd32588b1daca5569992ebe559c0102c837306b1af4c44d35128ec58366`。兩份本機 PNG（命令另設 `DOSGOLEM_MOO2_VBE_PNG=/src/workplace/moo2-vbe-266-<路徑>.png`）SHA-256 均 `1610444d26adb3135e7e933dd44912044bb636728af59c70e614945278d3c622`；已檢視為 640×480 黑色，索引仍全零。不能據此宣稱正常玩家畫面、原版像素一致或可玩完成；已驗不同頁選擇的證據是受控消費端測試，非此黑圖。

解析回填不可變鍵：固定 EXE 雜湊＋DOSBox-X `0180:0035CCA7`／dosgolem 高位 LE `0x228CA7`＋`CD 10 61 FC C3`／`4F07h`。237 保留「非零起點已由規格 266 接通」，265 保留「非零顯示停點已由規格 266 接通」及本檔連結；`--check-vbe-display-start-spec-backlinks` 自動核對。上一輪索引的 265「原版使用點待核對」已被其實際收據推翻，本輪只修索引，不重開已完成項。

僅在有限起點、服務返回與兩條像素消費路徑 CONFORMED；hardware-spec approximation、平台布局與既有未定義旗標／DTA／例外差異保留。255 仍 READY，原版正常玩家畫面、音效、受控亂數及 Go remake 玩法同狀態未完成。

**word ADD 停點已由規格 267 接通**：[267-cpu386-add-word-register-immediate.md](267-cpu386-add-word-register-immediate.md) 保留 `0x234B10`／`66 83 C3 18` 的固定定位，接 word 暫存器帶符號立即值、旗標與高半部保存。舊黑圖仍是本規格歷史收據，不因 CPU 解碼延伸而升格。
