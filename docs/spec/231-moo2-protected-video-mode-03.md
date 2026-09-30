# 231 — MOO2 保護模式視訊模式 03h 啟動呼叫

狀態：**CONFORMED**
日期：2026-10-01
用途：讓固定 1.31 原檔的啟動診斷越過已觀測的 `INT 10h/AX=0003h`。此規格不宣稱畫面可繪製。

## 證據與邊界

原檔 `ORION2.EXE` SHA-256 `4e11be14217b4aafa1839f333bf5eba037f98b0c44e9e4752c96c464c260419f`。DOSBox-X 2026.07.02 SDL2 heavy debugger 映像 `fd2-dosbox-x:debug-0d7b272b`，ID `sha256:659e8abbf93646f59a1586341769bd4b8f3cd1c707859d7a5de4c56e4672b582`。受版控探針 `apps/moo2/tools/startup_probe_131.py --video-mode-03` 從固定原檔自然走到 **DOSBox-X CS:EIP** `0180:003802B2` 的 `CD 10`；呼叫前 EAX=`00000003`、EBX=`01800188`、ECX=`01880188`、EDX=`00200000`、EFLAGS=`0216h`。同次在 `0180:003802B4` 返回，這些暫存器及旗標相同。呼叫與返回各自的私有 LOG SHA-256 為 `adff52520b95af86bee1bbb85a2db09d5ee43f9d59b20c16c6fe936b4ea11181` 及 `d94c025a57c2bb806cebdf3bca8f86e56cb449eae9c7c224922109e5ac7703d2`；暫存器 JSON 為 `8b7a2695732e8768acae0df0cb0dd25327f69b338a13d39265408cd751cb8cda`。caller 隨後將返回暫存器寫入記錄，EAX 仍為 3。

[DOSBox-X 視訊 BIOS 原始碼](https://github.com/joncampbell123/dosbox-x/blob/master/src/ints/int10.cpp) 的 `INT10_Handler` 於 AH=`00h` 以 AL 設定模式，這是平台契約。原檔只證實模式 03h 的呼叫與上述返回；未驗證 BDA、畫面、文字緩衝區或其他 `INT 10h` 功能。

dosgolem 合成 PSP／環境下的固定原檔在第 6256 步、**重定位 LE 線性位址** `0x15C2B2` 停於 `CD 10`，EAX=`3`。兩執行器的記憶體與堆疊初態不同，僅比較該呼叫的函式、低位參數及不變返回，不把它稱為整段同狀態對拍。

## 擬議契約與驗收

只在 `NewMOO2StartupDOS` 設定中接受保護模式 `INT 10h/AX=0003h`，將受控目前模式記為 03h；保持所有一般暫存器、段與 EFLAGS。其他 `INT 10h` 參數及 FD2 一般設定仍拒絕。不以單純記錄模式冒稱視訊呈現。

合成測試覆蓋模式記錄、暫存器／段／旗標不變及拒絕邊界；固定原檔整合測試從 LE entry 經第 6256 步，單步到 `0x15C2B4`，再記錄下一個自然停點。含原檔的全套 Go 測試通過；正常玩家路徑與畫面仍另需驗證。

READY 審查：原版入口、同次返回及 caller 消費端構成固定呼叫的最小充分證據；公開平台原始碼只支持 AH=00h/AL=03h 的視訊模式語意。合成服務狀態不代表 BDA、字元 VRAM 或實際畫面；這些若成為後續停點的必要條件，須回到證據與規格。此限縮契約不修改 MOO2 remake 玩法。

CONFORMED 驗收：`internal/machine/le_startup.go` 僅在 MOO2 設定接受精確 `EAX=00000003`，記錄模式 03h 並保留全部暫存器、段與旗標；其他模式及一般 FD2 仍拒絕。合成測試與固定原檔整合測試均通過，後者自行抵達第 6256 步、**dosgolem 重定位 LE 線性位址** `0x15C2B2`，單步至 `0x15C2B4`。含原檔 `go test -buildvcs=false ./... -count=1` 全通過，私有 `workplace/full-test-231.txt` SHA-256 `66f262c63281b32a8d619f190b0d710956ffecdf158624cd0603f796b57921ee`。診斷工具在 DOS 服務標示 `Exited` 時停止，固定合成環境第 10173 步 `INT 21h/AH=4Ch` 以代碼 1 結束，主控台為 `Unable to open mox.set\r\n`；私有 `workplace/moo2-probe-231-exit.txt` SHA-256 `8ee984ad0f300255f41f4259729102a07eb193461cc00b307d35b0a2e2f694d6`。先前未在退出邊界停住而於第 10175 步遇 `66 2E 8E 1D`，屬診斷工具誤走退出後程式碼，**不列為新的 CPU 缺口**。這只確認缺檔分支，沒有正常玩家畫面；下一步應為有根據的 `MOX.SET` 輸入路徑。
