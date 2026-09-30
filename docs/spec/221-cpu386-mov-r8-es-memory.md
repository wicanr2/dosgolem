# 221 — ES 覆寫的 byte 記憶體來源 MOV

狀態：**CONFORMED**（2026-10-01，限本規格的 CPU 工具切片）
日期：2026-10-01

## 阻塞、輸入與位址

固定 MOO2 1.31 `ORION2.EXE` SHA-256 `4e11be14217b4aafa1839f333bf5eba037f98b0c44e9e4752c96c464c260419f`。dosgolem 使用合成 PSP／環境及已綁定 DPMI 的固定真檔診斷，在第 4944 步、**重定位 LE 線性位址** `0x146903` 遇到 `26 8A 1E 42 84`；前 3 bytes 為待處理指令 `26 8A 1E`，後面的 `42` 是另一條指令。這只是工具缺口，不是正常玩家路徑收據。

DOSBox-X 2026.07.02 SDL2 heavy debugger 映像 `fd2-dosbox-x:debug-0d7b272b`，image ID `sha256:659e8abbf93646f59a1586341769bd4b8f3cd1c707859d7a5de4c56e4672b582`。以相同雜湊原檔在 **DOSBox-X CS:EIP** `0180:0036A903 → 0180:0036A906` 的同次 `LOG 2` 得到 `mov bl,es:[esi] → inc edx`。原版執行前 EBX=`0000000Fh`、ESI=`003EBA74h`、ES=DS=`0188h`、EFLAGS=`0246h`；ES:`003EBA74h` 的 byte 為 `30h`。執行後 EBX=`FFFFFF30h`，ESI、ES、DS、EFLAGS 與來源 byte 不變。版控 `apps/moo2/tools/startup_probe_131.py --es-byte-load` 可重生私有收據：`es-byte-load-registers.json` SHA-256 `8312772e1b0de3fff25a493cbdd99847ee2b9a8b97d62b24b5ed22061b428511`，`es-byte-load-logcpu.txt` SHA-256 `f751a6bc439f1380b3eaa5adeccb196d6e18e4f0a2000dc2398bec340aed9837`，前後來源 byte 檔 SHA-256 均為 `5feceb66ffc86f38d952786c6d696c79c2dbc239dd4e91b46729d73a27fb57e9`。原檔與完整收據只留未版控工作區。

## 擬議通用 CPU 契約

[Intel® 64 與 IA-32 軟體開發手冊第 2A 卷](https://www.intel.com/content/dam/www/public/us/en/documents/manuals/64-ia-32-architectures-software-developer-vol-2a-manual.pdf) 的指令格式把 `26h` 定為 ES 段覆寫；[第 2B 卷](https://www.intel.com/content/dam/www/public/us/en/documents/manuals/64-ia-32-architectures-software-developer-vol-2b-manual.pdf) 的 `MOV` 條目定義 `8A /r` 為 `MOV r8,r/m8`。本次 `1Eh` 是 `mod=00,reg=BL,r/m=[ESI]`；讀 ES:[ESI] 的一個 byte 到 BL，只改 EBX 低 8 位，不改旗標或來源。**原版樣本 ES=DS，不能單獨證明覆寫確實選 ES**；不同 ES／DS 基址的合成測試與手冊契約才驗證這部分。

僅擴充無 operand-size／repeat 前綴、32 位址的 `26 8A /r` **記憶體來源**。以既有 `decodeAddress32` 解碼 ModRM／SIB／位移，再把結果的段選擇覆寫為 ES，經 `readSegment8` 讀入指定 byte register。暫存器來源、未知或重複段前綴、未註冊 ES selector、越界、截短 ModRM／SIB／位移應失敗即關閉；資料暫存器、旗標與來源記憶體不得因失敗改動。既有無覆寫及 CS 覆寫行為維持。

## 審查與驗收

審查原版同次 `LOG`、EV、來源前後擷取及位址基準，確認原版只支持 `26 8A 1E` 這個實際樣本；其餘記憶體 ModRM 形狀來自 Intel 契約與既有共用解碼器，不標成原版實測。驗收需有 ES/DS 不同基址、EBP 預設 SS 被 ES 覆寫、SIB、低 byte／高 byte register、來源不寫回、旗標不變、非法前綴、無效 selector、越界及截短輸入測試。固定真檔診斷須越過第 4944 步並記錄下一停點；`go test -buildvcs=false ./internal/cpu386 ./internal/machine -count=1` 與固定原檔輸入的全套測試須通過。

READY 審查結論：`LOG 2` 首行、前後 EV、單一 byte 前後檔共同支持 `BL←30h`，且 EFLAGS 與來源未變；`0x146903` 和 `0036A903` 明確屬於不同工具的位址空間，不以數值相減代替原版證據。ES=DS 的原版樣本不足以單獨證明段覆寫，故把這項通用處理器契約限定為手冊加合成異段測試；合成 PSP／環境不支持任何正常玩家路徑宣稱。這些限制不阻礙有限的 CPU 工具切片，准予按上述形狀實作。

此規格只為原版觀測工具補通用 CPU 能力，不改 remake 的玩法、資料、UI 或存檔。合成 PSP／環境與原版不同，完成此切片也不構成玩家路徑或同狀態對拍。

## 有限驗收結果

`internal/cpu386/cpu.go` 的既有 `8A` 記憶體來源分支納入 ES 覆寫，仍由 `decodeAddress32` 取得 32 位有效位址，覆寫段後以 `readSegment8` 讀取。合成測試使用不同 DS／SS／ES 基址驗證原版 `26 8A 1E`、EBP 與 SIB 的預設 SS 覆寫、高位 byte 暫存器、來源與旗標不變，以及無效 selector、段越界、截短 ModRM／SIB／位移、repeat 與暫存器來源拒絕。既有 ES 位移測試仍通過。

`golang:1.24-bookworm` 無網路容器內，以 SHA-256 核對後從原版 1.31 ZIP 暫時擷取 EXE，執行 `go test -buildvcs=false ./internal/cpu386 ./internal/machine -count=1` 及 `DOSGOLEM_MOO2_EXE=/tmp/ORION2.EXE go test -buildvcs=false ./... -count=1`，均通過。固定真檔但合成 PSP／環境的 `workplace/moo2-probe` 從第 4944 步前進至第 5392 步；下一個未支援形狀為 **dosgolem 重定位 LE 線性位址** `0x14822D` 的 `C1 CA 08`，報告中的 CPU EIP 已先吃掉 opcode／ModRM 而顯示 `0x14822F`。私有 `workplace/moo2-probe-221.txt` SHA-256 `7f1d74911be0ea3b1ebf4cf16e02661cefc54b1d3bcfda41eabbbf4923b3ebc1`，最終全套測試輸出 `workplace/full-test-221.txt` SHA-256 `1babab5727e97dfbb7db6d396ded1c4b3c7fc3479ae3d4aa82d5e176a23e5a99`；兩者不入 Git。新 `C1` 停點尚未獲原版獨立核對，不能當作玩家流程或玩法同狀態收據。
