# 272 — 16 位元記憶體的遞增

狀態：**CONFORMED（有限 CPU 與下一載入範圍）**
日期：2026-10-01
範圍：32-bit CPU 的 `66 FF /0`、word 記憶體目的；不改 remake 玩法，不擴充 word 暫存器 INC 或未審查前綴。

## 證據與擬議契約

[271-cpu386-cmp-word-destination.md](271-cpu386-cmp-word-destination.md) 的固定 1.31 ORION2.EXE SHA-256 `4e11be14217b4aafa1839f333bf5eba037f98b0c44e9e4752c96c464c260419f`，無事件／受控事件第 6,822,373／6,822,408 步在 **dosgolem 高位 LE 線性** `0x21CA3D` 的 `66 FF 40 04` 拒絕。EAX=10000h、ECX=1DFh、EDX=0、EFLAGS=282h；目的為 DS:[EAX+4]，只讀觀測已確認 word=0。DOSBox-X 候選 **CS:EIP** `0180:00350A3D → 0180:00350A41` 已由下列原版樣本核對，不由代碼位址差猜資料指標。

[Intel INC](https://www.ardent-tool.com/CPU/docs/Intel/386/manuals/prref386/INC.htm) 的 word FF /0 定義目的加一、更新 OF／SF／ZF／AF／PF，保留 CF。結果以 16 位元環繞；其他旗標、暫存器與段保持。共用 address32、readSegment16、writeSegment16、add16；讀取與寫入均成功後才發布旗標。既有 word DEC 的順序與行為保持。descriptor 預檢失敗不得寫入；既有逐 byte 匯流排寫入若第二 byte 失敗，可能已寫第一 byte，應明示目前模型差異，不能聲稱原子例外。

## 驗收與停止線

以獨立期望值核對 word 邊界、五旗標與 CF 的兩種初值、DS／SS／SIB／位移／地址別名、只寫兩 bytes、唯讀與界線拒絕、部分讀取／寫入失敗時不發布旗標。保留 word DEC、dword INC／DEC 及未知群組／前綴拒絕。固定原版目的 word、完整旗標與下一 A1 消費端需由輔助樣本核對，再審查 READY；不注入原版狀態，不追 helper 內部。CPU 測試、固定原檔全套與兩條 dosgolem 自然路徑通過後才有限 CONFORMED。圖像、音效、受控亂數與 Go remake 玩法同狀態均仍未完成，255 仍 READY。

## 原版樣本與 READY 審查

**已證實**：DOSBox-X 2026.07.02 SDL2 heavy debugger、映像 ID `sha256:659e8abbf93646f59a1586341769bd4b8f3cd1c707859d7a5de4c56e4672b582`，固定上述 EXE、417 根層檔及 MOX.SET（553 bytes、SHA-256 `bfd6855a41760b31156b96114b5b33c88f442ab8f8aae020c1740b3b486a3a80`）。`startup_probe_131.py --inc-word-memory` 實際命中 **CS:EIP** `0180:00350A3D → 0180:00350A41`；16 bytes=`66 FF 40 04 A1 A8 22 3D 00 8A 40 0B 24 20 25 FF`。EAX=FE30h、EBX=508050h、ECX=1DFh、EDX=0、ESI=0、EDI=3A2090h、EBP=3EBBA8h、ESP=3EBB74h，DS／ES／SS=188h、FS=0、GS=20h。DS:FE34h 的 word 由 0 變 1，EFLAGS=282h → 202h，其他擷取狀態保持。

有界 LOG 2 與後續 EV 確認下一 A1 已執行：DS:003D22A8 的 dword=0000FE30h，**CS:EIP** `0180:00350A46`、EAX=FE30h、EFLAGS=202h。這是最小可觀察的載入消費；不聲稱已解出目的欄位的玩法意義。JSON／消費 LOG／終端 SHA-256 `227301ea8b9741ceaf2b7fa3cf0aae638db1e4db8dc6f5745d67cd1e5728ee15`／`d063afe8d016757ef4413cce60489e9d646c0305f1e3147ab40c110502696e9b`／`50a84accb42d8c1a9e63f5b1db2bcad5b25d6c6229e47ba8f6a6df68636d1e7c`；完整資料只留本機。

dosgolem 起點 `a1a80765b6b1d527e2874057377afd1caab7a920` 加只讀觀測，無事件／受控事件目的 DS:10004h 的 word 均 0、EAX=10000h、EFLAGS=282h。gzip SHA-256 `fb9b45a6f3615754d08110549ea15a0f25f962a3dad483a55b11ace702cee509`／`a9d853654074c6ef13cca527375859c785994bcdff7a695f5b8693fe2844548b`。兩側絕對指標與布局不同，只比可重定位的 word 輸入／輸出與旗標；未宣稱完整狀態一致。

公開契約、固定原版目的與五旗標／CF、下一 A1 的實際結果均具備。精確兩 bytes 存取、地址解碼與失敗順序沿用既有層；CPU 測試必須驗兩種 CF、其他旗標保存與部分匯流排失敗近似。DRAFT 審查後轉 READY，才實作。原子例外、未審查前綴、word 暫存器 INC 與 helper 內部不擴充；圖像／音效／亂數／玩法同狀態仍未完成。

## CPU 實作與驗收

READY 後，word FF 同時接受記憶體 /0 INC 與既有 /1 DEC；讀完整 word，再完整嘗試寫回，成功後才由 add16／sub16 更新旗標並恢復原 CF。暫存器形式、其他群組、segment／REP／LOCK／地址寬度等未審查前綴不放寬。

`inc_word_memory_test.go` 以 16 組數值邊界、CF 兩初值及算術旗標的兩組舊值，獨立使用有號範圍、低 nibble 與低 byte 同位核對結果。descriptor 恰至目的第二 byte，鄰接 byte、高半部、其他暫存器及段保持；DS／SS、SIB、負位移、32 位元地址環繞與來源地址別名均已驗。唯讀、完整目的界線與兩個 byte 的受控讀／寫失敗不發布旗標；第二 byte 寫入失敗明示並測得第一 byte 已改，沿用目前匯流排模型，不冒稱 x86 原子例外。原版最小樣本只重定位兩個資料地址，0→1、EFLAGS=202h 與下一 A1 的值／旗標保持均通過。既有 word DEC 與 dword INC／DEC 全部 CPU 回歸保持。

Go 1.24.13、映像 ID `sha256:1a6d4452c65dea36aac2e2d606b01b4a029ec90cc1ae53890540ce6173ea77ac`，`GOMAXPROCS=2 go test -p 2 -buildvcs=false ./internal/cpu386 -count=1` 通過，私有 `workplace/cpu-test-272.txt` SHA-256 `ed4bf0766d93be14ad0b08dba6abe6d95902783cba3534ef0a51b287bf2d4623`。

固定 EXE 雜湊＋高位 LE `0x21CA3D`／DOSBox-X CS:EIP `0180:00350A3D`＋`66 FF 40 04` 為回填不可變鍵。271 須保留「word INC 停點已由規格 272 接通」及本檔連結；`--check-inc-word-memory-spec-backlinks` 的正例、刪原始定位與刪舊標記的拒絕例、所有既有回填函式、Python 語法及 UID/GID 均通過。有限 CPU 與下一載入不代表玩法或完整布局一致。

## 全套與兩條自然路徑

固定原版輸入的 `DOSGOLEM_MOO2_EXE=/tmp/game/ORION2.EXE GOMAXPROCS=2 go test -p 2 -buildvcs=false ./... -count=1` 全套通過，私有 `workplace/full-test-272.txt` SHA-256 `164688d8b3a2698f054ed2b00cf5fc3f40cc0b288d30dbb22f26c4cfdf27a3b5`。

`DOSGOLEM_MOO2_SEPARATE_DOS=1 go run -buildvcs=false ./workplace/moo2-probe /tmp/game/ORION2.EXE --game-dir /tmp/game`，另一路加 `DOSGOLEM_MOO2_MOUSE_EVENT_AFTER_POSITION=1`；兩條只讀觀測皆為 DS:10004h word=0、EAX=10000h、EFLAGS=282h，不注入狀態。無事件第 7,684,074 步、受控事件第 7,684,109 步，均越過 word INC、前進 861,701 指令，在 **dosgolem 高位 LE 線性** `0x21C2D6` 的 `78 06 2B C2 79 02 EB EC 5E 61 C3 68 24 00 00 00` 因短 JS 尚未支援而拒絕。拒絕後 EIP=0x21C2D7；EAX=2、EBX=Eh、ECX=10000h、EDX=1、DS／ES／SS=188h、EFLAGS=202h、DOS 呼叫 6，受控回呼 started=1／completed=1。兩側本次擷取暫存器相同，先前布局差異仍保留，不推論完整狀態一致。gzip SHA-256 `8c2ffdb964ca9c547416f16b026911fcd559f516c176dccf0d2a5fbef982032c`／`c3ef39afef2949572ccd154f872376f456a6f2b8df33824e0b470edea844ee38`。

Active=true、Bank=2、StartY=0、BankSets=17、Writes=921600、DisplaySets=2。有效索引 SHA-256 `7de0420d874f91251343b50c1d39944115b806e92f80aa0f77f3687ef8ec3e48`、RGB `4c6ae38e1537960ec75f35b60574a1a11ad54b06737ef2ad8c5e16fbd26a8244`；兩份 `workplace/moo2-vbe-272-*.png` SHA-256 皆 `dd4c21dd11e57ec86ef759182fd3ebadd6b53870b54de1ef192e2553820286db`。PNG 已實際檢視：640×480 白底黑色 Simtex 啟動標誌，首次可辨識圖像，已非前次黑圖；尚未進入主選單或可操作玩家畫面，不宣稱最終畫面對拍。

272 僅在 word 記憶體 INC、定義旗標與下一 A1 範圍 CONFORMED；255 仍 READY，主選單、音效、受控亂數與 Go remake 玩法同狀態未完成。部分匯流排寫入、IMUL 未定義 ZF、DIV 旗標／例外、SAR 的 AF、DTA 保留區與平台布局限制仍明示。下一步依公開 Jcc 契約核對短 JS、實際 SF 與第一個分支；DOSBox-X 候選 CS:EIP `0180:003502D6` 尚是地址假說，不追 helper 內部。

## 後續停點解析回填

短 JS 停點已由規格 273 接通：[273-cpu386-short-sign-branches.md](273-cpu386-short-sign-branches.md) 連接固定 EXE 的高位 LE `0x21C2D6`／DOSBox-X CS:EIP `0180:003502D6`／`78 06`，公開 JS／JNS 契約及原版 SF=0 的實際不取／取分支。兩工具初始 EAX／ECX 不同，未宣稱時間或完整狀態一致；本規格的 INC 結論與 255 的未知仍保持，後續實作驗收以 273 為準。
