# 233 — 16 位元記憶體與立即數 TEST

狀態：**CONFORMED**
日期：2026-10-01
用途：補足 dosgolem CPU 在 MOO2 正版資料啟動時遇到的通用指令形狀；不據此宣稱原版畫面或玩法對拍。

## 證據與界限

1. dosgolem 既有的受控啟動診斷，以官方 1.31 `ORION2.EXE`（SHA-256 `4e11be14217b4aafa1839f333bf5eba037f98b0c44e9e4752c96c464c260419f`）、正版壓縮檔根層 417 個檔案及 `MOX.SET`（SHA-256 `bfd6855a41760b31156b96114b5b33c88f442ab8f8aae020c1740b3b486a3a80`）為輸入。於第 288215 步、**dosgolem 重定位 LE 線性位址** `0x151A21` 停在原始 bytes `66 F7 05 52 1C 1C 00 F0 FF`。私有 `workplace/moo2-probe-232-full-game.txt` SHA-256 `c032e24493aef6d936a810d10dfe109b9884de5a78cb5527fdebcaeec63cfb9e`。這是已證實的執行器缺口，來源欄位的遊戲語意未知。
2. [Intel IA-32 指令手冊第 2B 卷 `TEST` 條目](https://www.intel.com/content/dam/www/public/us/en/documents/manuals/64-ia-32-architectures-software-developer-vol-2b-manual.pdf)列出 `F7 /0 iw` 為 `TEST r/m16,imm16`，結果只更新 SF、ZF、PF，CF／OF 清零，AF 未定義。以 `66` 使既有 32 位程式碼的運算元變成 16 位；`modrm=05` 與 `disp32=001C1C52h` 解碼屬已證實的 Intel CPU 契約，並非原版動態收據。
3. DOSBox-X 2026.07.02 SDL2 heavy debugger，映像 `fd2-dosbox-x:debug-0d7b272b` ID `sha256:659e8abbf93646f59a1586341769bd4b8f3cd1c707859d7a5de4c56e4672b582`，以同一 1.31 EXE 與正版資料從啟動跑至有界等待，候選 **DOSBox-X CS:EIP** `0180:00375A21` 未命中；畫面仍黑，只觀察到圖形顯示模式，沒有玩家可見畫面。私有 `full-data-test-word-registers.json` SHA-256 `a36a0848eac58ee5647e1903e74c6ccab328879d44420e6e4f0292604b5be6cf`，畫面 SHA-256 `2b96a6124ce858301efab47c54aee2537ccebca77501e638d39f38d4b2836849`。不能由未命中推定原版不執行該指令，也不能由相同 EXE bytes 推定兩個執行器已同狀態。

## 受限實作契約

只擴充既有 `66 F7 /0 iw` 的 32 位有效位址記憶體形狀：用現有 `decodeAddress32` 決定來源位址和 DS／SS 預設段，擷取 16 位立即數，再以 `readSegment16` 讀來源；呼叫現有 `setLogicFlags16(source & immediate)`。它清除 CF／OF、依結果設定 SF／ZF／PF；現有工具也清除 AF，這是對未定義旗標的明示實作選擇，不能冒稱原版 AF 行為。暫存器、來源記憶體、其他旗標不變。截短指令、來源越界及非 `/0` 未支援形狀失敗即關閉，且在失敗前不改旗標或來源。未新增段覆寫或 repeat 前綴支援。

## READY 審查與驗收

此切片是 CPU 工具契約，Intel 一手指令定義、原始 bytes 與現有段／旗標工具足以界定；原版候選停點尚無動態收據，因此驗收只涵蓋 dosgolem 的正確執行及前進距離。合成測試涵蓋非零結果、零結果、DS／SS 選段、暫存器與來源不變、截短及越界拒絕。以相同正版資料自行走過第 288215 步，記錄下一自然停點或有界步數；全套 Go 測試含固定原檔整合測試通過。若原版對拍仍缺同狀態收據，本規格不得標為原版玩法 parity。

## CONFORMED 收據

`internal/cpu386/cpu.go` 已在既有 `F7 /0` word 分支支援記憶體來源；合成測試涵蓋上列輸入與失敗邊界。`DOSGOLEM_MOO2_EXE=/tmp/ORION2.EXE go test -buildvcs=false ./... -count=1` 全通過。以相同正版根層 417 檔案自行重生的 dosgolem 探針，已越過第 288215 步，最後於第 295276 步透過 DOS 退出服務，以代碼 1 回報 `Insufficient Memory!`、要求 8192 bytes、DOS space remaining 0 bytes；私有 `workplace/moo2-probe-233-full-game.txt` SHA-256 `c75e200fda4251e97752600df06c84c14d2fbc27eb232afcd50f60936d79e744`。這是合成環境的記憶體／服務阻塞，不是原版記憶體需求或玩法結果已被證實。下一步先比較 DOS 記憶體服務的玩家可見邊界與環境設定，不從退出訊息推導原版固定記憶體配置。
