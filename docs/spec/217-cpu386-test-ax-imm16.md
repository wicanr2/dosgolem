# 217 — 16 位累加器的 TEST 立即數

狀態：**CONFORMED**（限 `66 A9 iw` 的通用 CPU 契約）
日期：2026-10-01

## 原版問題與定位

固定 MOO2 1.31 `ORION2.EXE` SHA-256 `4e11be14217b4aafa1839f333bf5eba037f98b0c44e9e4752c96c464c260419f`。已綁定 DPMI、使用合成 PSP／環境的 dosgolem 診斷於第 2600 步、**重定位 LE 線性位址** `0x126570` 遇 bytes `66 A9 89 CF`，因 `A9` 的 16 位運算元前綴不支援而停止。這只是工具缺口，不是正常玩家路徑或玩法收據。

DOSBox-X 2026.07.02 SDL2 heavy debugger，映像 `fd2-dosbox-x:debug-0d7b272b` ID `sha256:659e8abbf93646f59a1586341769bd4b8f3cd1c707859d7a5de4c56e4672b582`，在固定原版的 **CS:EIP** `0180:0034A570 → 0180:0034A574` 同次 `LOG 2` 記錄 `test ax,CF89 → pop es`。進入 EAX=`003EBB00h`、EFLAGS=`0206h`；離開 EAX 不變、EFLAGS=`0286h`。`AX=BB00h` 與 `CF89h` 的逐位 AND 為 `8B00h`，SF=1、ZF=0、PF=1、CF=OF=0，與原版相符。版控 `apps/moo2/tools/startup_probe_131.py --test-word` 重生私有 `test-word-registers.json` SHA-256 `b907d497708f9af5e4f671ab5b4d3d3d15041aac383b4620c7ba6473ed59f2a5`、`test-word-logcpu.txt` SHA-256 `e624bc4be5bd0f955568077c3f4fcfc711927baf90ba94ca6a85254be805ceab`；原版檔與終端不入版控。兩工具位址基準不同，以指令 bytes 與控制流交叉定位，不以數字相近作證。

[Intel 指令手冊的 TEST 條目](https://www.intel.com/content/dam/www/public/us/en/documents/manuals/64-ia-32-architectures-software-developer-vol-2b-manual.pdf) 定義 `A9 iw` 為 `TEST AX,imm16`：取 16 位逐位 AND，只更新 SF／ZF／PF 並清 CF／OF，不寫回 AX；AF 未定義。現有 `setLogicFlags16` 為未定義 AF 選擇清零，此為工具的明示決定性近似，不宣稱與所有原版狀態一致。

## 擬議工具範圍與驗收

只增加無段覆寫、無 repeat 的 `66 A9 iw`。從指令流讀 little-endian `imm16`，以 EAX 低 16 位 AND 立即數，調用既有 `setLogicFlags16`；EAX 含高 16 位及記憶體均不改。原有無前綴 `A9 id` 維持；帶段覆寫或 repeat 仍拒絕。立即數截短時須在更新旗標前失敗即關閉，EAX 不變。

合成測試須涵蓋原版 `BB00h & CF89h`、高 16 位保留、ZF／PF／SF、清 CF／OF、截短立即數與未支援前綴，並回歸原有 `A9 id`。固定原檔有界診斷須越過第 2600 步並記錄下一停點；`go test -buildvcs=false ./internal/cpu386 ./internal/machine -count=1` 與帶固定原檔的 `go test -buildvcs=false ./... -count=1` 須通過。這個工具切片不修改 remake 規則、UI、資料或存檔；完整 MOO2 玩家路徑及同狀態玩法對拍仍待完成。

## 證據審查

原版同次 `LOG` 的指令、EAX 不變與 SF 更新符合 Intel `A9 iw` 契約；`8B00h` 的低位元組為零，PF=1。原版樣本的 CF／OF 與 AF 進入值均為零，因此清 CF／OF 的通用行為由 Intel 契約及合成測試補證，AF 則維持未定義且明示採既有清零近似。既有 `fetch16` 在兩位元組都成功後才交出立即數，`setLogicFlags16` 已套用於其他 16 位邏輯運算；可核准此窄範圍工具規格為 READY。未核准遊戲玩法規則、完整啟動或正常玩家路徑。

## 驗收結果與限制

`go test -buildvcs=false ./internal/cpu386 ./internal/machine -count=1` 與固定 1.31 原檔輸入的 `go test -buildvcs=false ./... -count=1` 全通過。合成測試覆蓋原版同形狀、非零高 16 位不變、ZF／PF／SF、清 CF／OF、截短立即數與非法前綴；既有 32 位 `A9 id` 測試仍通過。已綁定 DPMI、仍用合成 PSP／環境的固定真檔診斷越過第 2600 步，在第 4062 步、dosgolem **重定位 LE 線性位址** `0x139A53` 的 `CD 21` 停下；進入暫存器 EAX=`00171A99h`（AH=`1Ah`）、EDX=`001A5828h`，服務層回報未處理。此停點尚未經原版獨立核對，不能據此加入服務返回或宣稱正常玩家路徑。規格只在所列 CPU 範圍內標為 CONFORMED。
