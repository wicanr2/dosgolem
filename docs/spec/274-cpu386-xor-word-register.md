# 274 — 16 位元暫存器間的 XOR

狀態：**CONFORMED（有限 CPU／下一段載入）**
日期：2026-10-01
範圍：32-bit CPU 的 `66 31 /r`、ModRM=3 的 word 暫存器來源與目的；不擴充記憶體／反向 opcode／未審查前綴，不改 remake 玩法。

## 原始定位與擬議契約

[273-cpu386-short-sign-branches.md](273-cpu386-short-sign-branches.md) 固定 1.31 ORION2.EXE SHA-256 `4e11be14217b4aafa1839f333bf5eba037f98b0c44e9e4752c96c464c260419f`，無事件／受控事件第 20,100,561／20,100,596 步停於 **dosgolem 高位 LE 線性** `0x239A42`、`66 31 FF 8E C7 CD 2F 66 89 3D 22 BC 2A 00 66 C7`，EAX=F1684h、EBX=5、ECX=239F20h、EDX=1、EFLAGS=246h。word XOR DI,DI 尚未支援，完整 EDI／高半部已由下列只讀樣本擷取。DOSBox-X 候選 **CS:EIP** `0180:0036DA42 → 0180:0036DA45` 已由下列樣本核對，不以位址差猜資料布局。

[Intel XOR](https://www.ardent-tool.com/CPU/docs/Intel/386/manuals/prref386/XOR.htm) 定義 word 目的的每個 bit 為兩來源該 bit 不同的結果，寫回目的低 word；清 CF／OF，依 word 結果決定 SF／ZF／低 byte 的 PF。AF 未定義；沿用 dosgolem 邏輯旗標層的清 AF 模型，不把未定義旗標宣稱成硬體全值對齊。目的高半部、其他暫存器／段／記憶體及非算術旗標保持；先完整取 ModRM，其他形式拒絕，不發布運算狀態。

## 驗收與停止線

八來源×八目的×word 邊界，獨立逐 bit／同位期望、非零／零／正負／高半部／別名、CF／OF 清除與非算術旗標保持；截短與未知前綴、記憶體形式拒絕。保留 byte／dword XOR 行為。原版完整 EDI、高半部、定義旗標與下一 MOV ES 的 selector 消費，需先擷取後審查 READY 才實作。不追後續 INT 2F 平台內部。CPU、固定原檔全套與兩條明示 50M 上限自然路徑後才限定 CONFORMED，255、主選單／玩家操作、音效、受控亂數與 Go remake 玩法同狀態仍未完成。

## 原版樣本與 READY 審查

**已證實**：DOSBox-X 2026.07.02 SDL2 heavy debugger、映像 ID `sha256:659e8abbf93646f59a1586341769bd4b8f3cd1c707859d7a5de4c56e4672b582`，`startup_probe_131.py --xor-word-register`，固定上述 EXE、417 原檔與 MOX.SET（553 bytes、SHA-256 `bfd6855a41760b31156b96114b5b33c88f442ab8f8aae020c1740b3b486a3a80`），未注入暫存器或旗標。實際命中 **CS:EIP** `0180:0036DA42 → 0180:0036DA45`，16 bytes=`66 31 FF 8E C7 CD 2F 66 89 3D 22 9C 3D 00 66 C7`。

EAX=F1684h、EBX=5、ECX=36DF20h、EDX=1、ESI=3EBB20h、EDI=38EC2Bh、EBP=0、ESP=3EBAD0h，DS／ES／SS=188h、FS=0、GS=20h、EFLAGS=246h。XOR 後 EDI=380000h、高半部保持、低 word=0、EFLAGS=246h，其餘擷取狀態保持。LOG 2 與後續 EV 實際確認 **CS:EIP** `0180:0036DA47`、下一 MOV ES 將 ES=0，其餘狀態保持。JSON／消費 LOG／終端 SHA-256 `9662eb7f0ee609340a17aa067b8f5b30b3d8fd796fff4970f9eed515e38bd16c`／`866447078bda5e5a7f5b47e045099ed2d3216f2bb6ea6787a880ece43452eba3`／`94fa43208094452683a6bb10df48de6f6cd31b7579d85b93f1c4a1ef07e4bfd0`；完整資料只留本機。

dosgolem 起點 `5307099b55cd239f9ea5badd2ca42a245d951404` 加只讀掛鉤，原版兩條仍在 20,100,561／20,100,596 步拒絕。無事件 EDI=260C2Bh、ESI=2BDB40h、ECX=239F20h、ESP=2BDAF0h，其餘選定原始欄位與 EFLAGS=246h；資料布局與 DOSBox-X 不同，沒有宣稱完整同狀態。觀測 gzip SHA-256 `6c804481395649eab3e6cbf219a6c62c0ceef13f6b45cc354a2232723ea8d13e`／`cdfef81a349cf270662698958f176c8a65aec24c10e8d55a946576051628bdc8`。

公開 word 契約、原版完整目的／高半部、定義旗標與 MOV ES 消費已足夠；AF 在本樣本原先就是 0，不能由此推定其他硬體未定義結果。沿用清 AF 的模型，測試將定義五旗標與模型 AF 分開核對。DRAFT 審查後轉 READY 才實作；限於八 word 暫存器與下一段載入，不追 INT 2F 或 helper，其他拒絕界線保持。

## 首輪 CPU 回歸樣本訂正

新 word 的全部配對／定義旗標、高半部與 MOV ES 通過；首輪唯一失敗為舊 dword XOR 的回歸樣本把旗標誤填成 206h。其結果 B9F90003h 的最高 bit=1，依 [Intel Appendix C](https://www.ardent-tool.com/CPU/docs/Intel/386/manuals/prref386/appc.htm) 的 SF 契約應為 286h；word／byte 的結果 3 才是 206h。實際既有 dword CPU 回傳 286h 正確，規格與 production 不變，只修正測試期望。失敗收據 `workplace/cpu-test-274-first-failure.txt` SHA-256 `28410c1b77d8b114077dc7d886870ef4a8a69cc23b33d74975a8630d94ed499b` 保留。

## CPU 實作與驗收

READY 後，31 的 word 暫存器分支只計算兩低 word，寫回目的低 word 並保存高半部；沿用 setLogicFlags16，定義五旗標與非算術旗標保持，AF 仍是清除的模型近似。記憶體與未審查前綴拒絕，既有 byte／dword 路徑保持。

`xor_word_register_test.go` 的八來源×八目的×16 邊界值成對×兩舊旗標組，以逐 bit 不同判定、較窄結果有號值與低 byte 同位獨立驗結果／定義旗標；AF 清除另列為模型檢查。來源目的別名、目的高半部與其他暫存器／段／記憶體、截短與全部未知前綴／記憶體形式拒絕均通過。原版最小樣本只重定位代碼，EDI=38EC2Bh→380000h、EFLAGS=246h、下一 ES=0 與其餘完整擷取狀態保持已驗。byte／word／dword 的高半部與各寬度 SF 差別均有回歸，不把未定義 AF 升格成硬體逐值對拍。

Go 1.24.13、映像 ID `sha256:1a6d4452c65dea36aac2e2d606b01b4a029ec90cc1ae53890540ce6173ea77ac`，相同 `GOMAXPROCS=2 go test -p 2 -buildvcs=false ./internal/cpu386 -count=1` 訂正測試後乾淨重跑通過，私有 `workplace/cpu-test-274.txt` SHA-256 `c7376ff82feaa8997b933170e88244bf43fafc7870704148b0ea1aa77a549f9b`。

固定 EXE 雜湊＋高位 LE `0x239A42`／DOSBox-X CS:EIP `0180:0036DA42`＋`66 31 FF` 為回填不可變鍵。273 必須保留「word XOR 停點已由規格 274 接通」與本檔連結；`--check-xor-word-register-spec-backlinks` 正例、刪原定位／舊標記的拒絕例、全部既有回填函式、Python 語法與擁有權均通過。此有限 CPU 與下一段載入，不代替正常玩家路徑或完整時間／資料布局同狀態。
## 固定原版全套與兩條自然路徑

同一 Go 映像、417 根層原檔及官方 1.31 EXE，`DOSGOLEM_MOO2_EXE=/tmp/game/ORION2.EXE GOMAXPROCS=2 go test -p 2 -buildvcs=false ./... -count=1` 全套通過，`workplace/full-test-274.txt` SHA-256 `4d94cf6a4c8d621baf6172315ab9c67cab338c28a7428f55b9617d76fae5ef8e`。

`DOSGOLEM_MOO2_MAX_STEPS=50000000 DOSGOLEM_MOO2_SEPARATE_DOS=1 DOSGOLEM_MOO2_VBE_PNG=<本機輸出> go run -buildvcs=false ./workplace/moo2-probe /tmp/game/ORION2.EXE --game-dir /tmp/game`，事件一路另加 `DOSGOLEM_MOO2_MOUSE_EVENT_AFTER_POSITION=1`，兩條均由 dosgolem 自行重生。XOR 前 EDI=260C2Bh，其他完整觀測欄位見上；自然執行越過 XOR／MOV ES，ES=0、flags=246h。無事件第 20,100,563 步、事件第 20,100,598 步停止於 **高位 LE 線性** `0x239A47` 的 `CD 2F 66 89 3D 22 BC 2A 00 66 C7 05 24 BC 2A 00`，拒絕後 EIP=239A49h，error=INT 2F 未處理。EAX=F1684h、EBX=5、ECX=239F20h、EDX=1、DS／SS=188h、ES=0、EFLAGS=246h、DOS 呼叫 6，事件回呼 started=1／completed=1。AX=1684h 的平台查詢尚未建模，不因低 word 相同假定其完整返回契約。

兩條 gzip SHA-256 `5c0c42d06340369d93f6abb2240106e4dd7dfb63192ba4fca7b38ec4d7dbb208`／`21f0880d38e18710824bb46166692500fec7775c0b51b620520b5bbccd66ca16`。VBE Active=true、Bank=2、StartY=0、BankSets=441、Writes=4046164、DisplaySets=6，索引 SHA-256 `7818f5542a0404157573be6cffc0e0c8e68ce3c0f5d17d07ccdd9313fb700baf`、RGB `0b150fd32588b1daca5569992ebe559c0102c837306b1af4c44d35128ec58366`、兩 PNG 均 `1610444d26adb3135e7e933dd44912044bb636728af59c70e614945278d3c622`，與先前已檢視黑圖逐位元相同，不重做目視驗收。完整原檔、終端、gzip 與 PNG 均只留本機。

CPU 與原版有限指令／消費樣本、全套及兩条自然路徑已驗，274 限定 CONFORMED；255 仍 READY。AF 的清除為模型近似，工具初始布局不同，未宣稱硬體全旗標、完整時間／同狀態、主選單、正常玩家輸入、音效、受控亂數或 Go remake 玩法已完成。下一步只查公開 INT 2Fh/AX=1684h 邊界與實際 caller 參數／返回／消費，不深入平台、runtime 或圖形 helper。

後續回填：VTD 空入口停點已由規格 275 接通，見 [275-moo2-protected-vtd-entry-query.md](275-moo2-protected-vtd-entry-query.md)。上述 INT 2F 拒絕與下一步是本規格當時的歷史收據；現行停點以後續規格與自然收據為準，不重開 word XOR。
