# 218 — 受保護模式 DOS 設定 DTA 指標

狀態：**CONFORMED**（只限保存 DTA 指標）
日期：2026-10-01

## 問題與原版邊界

固定 MOO2 1.31 `ORION2.EXE` SHA-256 `4e11be14217b4aafa1839f333bf5eba037f98b0c44e9e4752c96c464c260419f`。已綁定 DPMI、仍用合成 PSP／環境的 dosgolem 診斷在第 4062 步、**重定位 LE 線性位址** `0x139A53` 遇 `CD 21`；進入 EAX=`00171A99h`、EDX=`001A5828h`、DS=`0188h`，服務層拒絕 `AH=1Ah`。這只是工具診斷。

DOSBox-X 2026.07.02 SDL2 heavy debugger，映像 `fd2-dosbox-x:debug-0d7b272b` ID `sha256:659e8abbf93646f59a1586341769bd4b8f3cd1c707859d7a5de4c56e4672b582`，在固定原版的 **CS:EIP** `0180:0035DA53 → 0180:0035DA55` 命中 `INT 21h/AH=1Ah`。進入 EAX=`00381A99h`、EDX=`003C3828h`、DS=`0188h`、EFLAGS=`0246h`；返回後這些值及 EBX、ECX、ES、SS、ESP 均不變。版控 `apps/moo2/tools/startup_probe_131.py --dta` 可重生私有 `dta-registers.json` SHA-256 `3e58143819ae32406a06027591a19bb14abfdf99eec644c8363886ef4048d68b`。原版與合成診斷的 EAX 高位及 DTA 偏移不同，不能宣稱同狀態。

同次原版 `--dta-find` 後續在 **CS:EIP** `0180:0035DA59` 呼叫 `AH=4Eh` 搜尋 `MOX.SET`，且設定的 DTA 位址於搜尋後改變，證明先前指標進入檔案搜尋路徑；但如何寫搜尋保留區、成功匹配與後續 `AH=4Fh` 不屬此規格。私有字串與搜尋收據雜湊及限制見 MOO2 專案 `docs/re/dosgolem-moo2-intake-20260930.md`。[Microsoft MS-DOS Programmer's Reference 的 Function 1AH](https://www.bitsavers.org/pdf/microsoft/msdos_2.0/8411-200-00_MS-DOS_2.0_Programmers_Reference_1983.pdf) 定義 `DS:DX` 指向 DTA、沒有返回資料；`internal/dos/int21.go` 的 16 位 DOS 層也保存此指標，僅作平台契約交叉參照，不把 16 位搜尋結果套給本服務。

## 擬議工具契約

受保護模式 `FD2StartupDOS.Handle` 收到 `INT 21h/AH=1Ah` 時，以當下的 `DS` selector 與完整 32 位 `EDX` 保存 DTA 指標，不讀寫指標所指記憶體；EAX、其他通用暫存器、段暫存器與 EFLAGS 不變。再次呼叫應替換所存指標。這是 DTA 設定，不是檔案搜尋成功承諾；未接入 `AH=4Eh／4Fh` 之前仍須對它們失敗即關閉。

合成測試核對指標保存、替換、指標與目前 DS 的生命週期分離、暫存器／旗標／記憶體不變，並以固定原檔診斷確認越過第 4062 步停於下一未支援功能。CPU／machine 及全套 Go 測試須通過。此工具切片不修改 remake 規則、UI、資料或存檔；原版 DTA 寫入、完整遊戲資料與同狀態玩家路徑仍未知。

## 證據審查

原版 `AH=1Ah` 呼叫前後一般暫存器與旗標不變；Microsoft 契約只定義設定 DTA 指標，並無資料回傳。原版觀測的 EDX 高半字非零，因此此 32 位 protected-mode 工具不能截成 `DX`；保存完整 EDX 是本工具的強推論，須以再次設定及不同 DS 的合成測試驗證。原版 DTA 保留區在後續失敗的 `AH=4Eh` 被改寫，這是另一服務的行為，不能反向推定 `AH=1Ah` 已寫記憶體。只核准保存 selector 與偏移的 READY 範圍；搜尋結果及原版 DTA bytes 保持未核准。

## 驗收結果與限制

`go test -buildvcs=false ./internal/cpu386 ./internal/machine -count=1` 及固定 1.31 原檔輸入的 `go test -buildvcs=false ./... -count=1` 全通過。合成測試核對完整 32 位偏移、呼叫當下 selector、再次設定、後續 DS／EDX 改動不污染已存指標、一般暫存器／旗標／記憶體不變。固定原檔的合成 PSP／環境診斷越過第 4062 步，於第 4066 步、dosgolem **重定位 LE 線性位址** `0x139A59` 的 `CD 21` 停在未接線 `AH=4Eh`；原版在其 **CS:EIP** `0180:0035DA59` 也呼叫此功能。這只完成 DTA 指標設定契約；首次搜尋、DTA 保留區、正常玩家路徑與 remake 同狀態玩法均未完成。
