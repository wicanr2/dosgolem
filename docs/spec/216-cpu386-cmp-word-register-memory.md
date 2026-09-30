# 216 — 16 位暫存器與 32 位位址記憶體 CMP

狀態：**CONFORMED**（限無段覆寫／repeat 的 16 位來源記憶體）
日期：2026-10-01

## 原版問題、位址與證據

固定 MOO2 1.31 `ORION2.EXE` SHA-256 `4e11be14217b4aafa1839f333bf5eba037f98b0c44e9e4752c96c464c260419f`；dosgolem 以內嵌 MZ `0x26654` 載入後，已綁定 DPMI、仍用合成 PSP／環境的診斷在第 2512 步、**重定位 LE 線性位址** `0x109FF` 遇 `66 3B 4D CE`，因 16 位記憶體來源未接線而失敗即關閉。這不是正常玩家路徑收據。

DOSBox-X 2026.07.02 SDL2 heavy debugger，映像 `fd2-dosbox-x:debug-0d7b272b` ID `sha256:659e8abbf93646f59a1586341769bd4b8f3cd1c707859d7a5de4c56e4672b582`，以固定原版於其 **CS:EIP** `0180:002349FF → 0180:00234A03` 同次 `LOG 2` 顯示 `cmp cx,[ebp-0032] → jl`，記憶體註記為 `ss:[003EBB08]=0001`。進入 CX=`0000h`、EBP=`003EBB3Ah`、SS=`0188h`、EFLAGS=`0246h`，離開 CX、EBP 不變、EFLAGS=`0297h`；`SS:003EBB08` 前後 bytes 均為 `01 00`。版控 `apps/moo2/tools/startup_probe_131.py --cmp-word` 重生的私有 `cmp-word-registers.json` SHA-256 `b4df0818c2059df4bef4a29c76508908e10e05ba9f8730b7102197da1a9b3969`、`cmp-word-logcpu.txt` SHA-256 `eaf818725b21cc1b76ca9830b81f0da46587ac91401a4dd0ee2d652a8f9a4c51`；前後二位元組檔 SHA-256 均為 `47dc540c94ceb704a23875c11273e16bb0b8a87aed84de911f2133568115f254`。原版檔與完整終端留在私有工作區。原版此時 DS 與 SS 剛好同為 `0188h`，**預設 SS 的理由來自原版反組譯註記、位址解碼器與 Intel 規格，不能由數值相等本身推得**。

[Intel® 64／IA-32 架構軟體開發手冊第 2A 卷](https://www.intel.com/content/dam/www/public/us/en/documents/manuals/64-ia-32-architectures-software-developer-vol-2a-manual.pdf) 定義 `3B /r` 的 16 位形狀為 `CMP r16,r/m16`，依目的減來源更新 CF、OF、SF、ZF、AF、PF，但不寫回運算元。`4D` 的 `mod=01`、`reg=ECX`、`r/m=EBP`，`CEh` 為有號 disp8 `-32h`；既有 `decodeAddress32` 對 EBP 基底選 SS。原版 CX=`0` 與來源=`1` 的 16 位差為 `FFFFh`，符合上述旗標與未改記憶體。

## 擬議實作與驗收

無 segment／repeat 前綴、32 位位址、16 位運算元的 `66 3B /r`，當 ModRM 是記憶體形狀時使用現有 `decodeAddress32` 取得預設 DS／SS 與偏移，從選定描述子讀 `word`，以暫存器低 16 位減來源，只更新六個算術旗標，不寫暫存器或記憶體。已存在的絕對位址與暫存器來源形狀保持同一語意；不擴到帶段覆寫或 repeat 的形式。來源越界、未登錄段及截短位移須失敗即關閉，旗標與運算元不變。這是通用 CPU 工具切片，無 remake 規則、UI、資料或存檔變更。

合成測試核對原版 `SS:[EBP-32h]`、不同 DS／SS base、非 EBP 的 DS 來源、記憶體及暫存器不變、段界限與截短位移拒絕；既有絕對位址及暫存器 CMP 測試保持通過。固定真檔有界診斷須越過第 2512 步並記錄下一停點；`go test -buildvcs=false ./internal/cpu386 ./internal/machine -count=1` 與有固定原檔輸入的 `go test -buildvcs=false ./... -count=1` 須通過。此處仍不是原版正常玩家路徑或玩法同狀態對拍。

## 證據審查

固定原版的同次 `LOG`、SS 記憶體前後二位元組與 Intel `3B /r` 編碼一致。原版 DS=SS，不能只靠當次值區分預設段；`decodeAddress32` 的 EBP→SS 路由與不同段基址的合成測試是必要補證。通用 `sub16` 已用於既有 word CMP；擴到記憶體來源只需在成功讀取後呼叫它，避免記憶體或暫存器寫回。可核准此無覆寫、無 repeat 的工具 CPU 範圍為 READY；未核准完整 MOO2 啟動、玩法或正常玩家路徑。

## 驗收結果與界線

`go test -buildvcs=false ./internal/cpu386 ./internal/machine -count=1` 及固定 1.31 原檔輸入的 `go test -buildvcs=false ./... -count=1` 均通過。合成測試以不同 DS／SS base 證明 EBP 位移讀 SS、非 EBP 位址讀 DS，並檢查運算元與記憶體不變、段界限、未登錄 SS 與截短位移拒絕；既有絕對位址及暫存器 CMP 回歸仍通過。已綁定 DPMI 的合成 PSP／環境診斷越過第 2512 步與 dosgolem **重定位 LE 線性位址** `0x109FF`，於第 2600 步 `0x126570` 的 `66 A9 89 CF` 停在未支援的 16 位 `TEST`。此新停點尚未由原版獨立核對。規格只在已列原版樣本及合成測試範圍內標為 CONFORMED；完整 MOO2 正常玩家路徑與 remake 同狀態玩法仍沒有收據。
