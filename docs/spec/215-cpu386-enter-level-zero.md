# 215 — 32 位堆疊的 ENTER 巢狀層級 0

狀態：**CONFORMED**（限無額外堆疊讀取的層級 0）
日期：2026-10-01

## 原版問題與證據

固定 MOO2 1.31 `ORION2.EXE` SHA-256 `4e11be14217b4aafa1839f333bf5eba037f98b0c44e9e4752c96c464c260419f`，dosgolem 以內嵌 MZ `0x26654` 載入，在第 2475 步、**重定位 LE 線性位址** `0x1005B` 遇 `C8 AC 00 00` 失敗即關閉。此為已綁定 DPMI、仍使用合成 PSP／環境的診斷，不能當正常玩家路徑。

DOSBox-X 2026.07.02 SDL2 heavy debugger，映像 `fd2-dosbox-x:debug-0d7b272b` ID `sha256:659e8abbf93646f59a1586341769bd4b8f3cd1c707859d7a5de4c56e4672b582`。其**CS:EIP** `0180:00362F5C` 的 `call 00234018` 經 `00234057` 與四次暫存器 push，抵達 `0180:0023405B` 的 `enter 00AC,00`；同次有界 `LOG 20` 的私有 `low-entry-logcpu.txt` SHA-256 `9d242f4d359cc331fbef1aad02a646ddf72e30a88eb09e7dd7e0641f883b5855`，其 JSON 收據 SHA-256 `2fd82053b138d260ed49b1675377f3106f9eb4ccaea8be20fbb0dabcfbc0b6fc`，由版控 `apps/moo2/tools/startup_probe_131.py --low-entry` 重生。這證實低位址停點確在原版控制流，但兩邊 PSP／環境仍不同，不保證完整狀態相同。

同一原版的 `--enter` 探針在 **CS:EIP** `0180:0023405B → 0180:0023405F` 以 `LOG 2` 加 `MEMDUMPBIN` 擷取：`SS=0188h`、`ESP=003EBC90h → 003EBBE0h`、`EBP=003EBCA4h → 003EBC8Ch`、`EFLAGS=0246h` 不變；`SS:003EBC8C` 四位元組 `90 20 3A 00 → A4 BC 3E 00`，即舊 EBP 的 little-endian 寫入。私有 `enter-registers.json` SHA-256 `f0081c2f07edfaed125a5dc6b95de4fd404200953c009db55046b3239f4d7169`、`enter-logcpu.txt` SHA-256 `1d458daf1ebe9d383baf105f9e386277909f6443419a21a16ae3beea56331ecb`、前後堆疊檔 SHA-256 分別為 `1a2db19b7f6380c9eed28e130e8224fe9245ab0eb0613af0f8254c02341269bf`／`ec5fc2e2555302df262bcda9052310184eda069fbd22cd9ffa4df4a01874d205`。原版檔及完整終端輸出只留私有工作區。

[Intel® 64／IA-32 架構軟體開發手冊第 2A 卷](https://www.intel.com/content/dam/www/public/us/en/documents/manuals/64-ia-32-architectures-software-developer-vol-2a-manual.pdf) 定義 `C8 iw ib` 的巢狀層級 0：推入舊 EBP，讓 EBP 指向該堆疊位置，再從 ESP 扣除 16 位立即數所指定的配置大小，旗標不變。原版 `0xAC` 大小與 ESP 總減量 `0xB0` 相符。

## 擬議實作與驗收

僅支援無前綴的 32 位 `C8 imm16 00`；讀取兩個立即數，當巢狀層級不是 0 時失敗即關閉。先確認 `ESP >= 4 + allocSize`、`SS:[ESP-4]` 的 4 位元組可寫，以及最終 `SS:[ESP-4-allocSize]` 在可寫段描述子界限內；**預檢失敗**時 EBP、ESP、旗標與記憶體不變。成功時把舊 EBP 寫入 `SS:[ESP-4]`，令 `EBP=ESP-4`、`ESP=EBP-allocSize`，不改旗標。16 位 operand-size、段覆寫、repeat 與巢狀層級非零維持拒絕。工具匯流排不提供完整分頁映射預檢或任意中途寫入失敗的交易性保證；不得為此對堆疊增加原版指令沒有的匯流排讀取。這是工具 CPU 切片，不修改 remake 的資料、玩法、畫面或存檔。

合成測試核對原版 `0xAC` 大小與舊 EBP 寫回、不同大小、段基址、旗標不變，以及不支援巢狀層級、堆疊下溢、SS 不可寫及段界限拒絕；另以可觀測匯流排確認不讀取堆疊。固定原檔的合成環境診斷須越過第 2475 步並記錄下一停點；`go test -buildvcs=false ./internal/cpu386 ./internal/machine -count=1` 與有固定原檔輸入的 `go test -buildvcs=false ./... -count=1` 均須通過。此切片不證明正常玩家路徑或同狀態玩法對拍。

## 證據審查

固定原版的 call／跳轉／push 順序、`ENTER` 前後 EBP／ESP、堆疊四位元組及不變旗標，與 Intel 的巢狀層級 0 契約一致。堆疊配置大小 `00ACh` 與原版 ESP 總減量 `00B0h` 相符；尚未觀測其他配置大小、巢狀層級或 16 位 operand-size，因此仍拒絕那些形狀。段基址與界限以現有描述子介面和合成測試驗證，不冒稱原版逐組合實測。上述範圍足以核准此工具 CPU 切片為 READY，未核准完整 MOO2 啟動或玩法對拍。

## 驗收結果與界線

第一輪實作與測試雖跨過原版停點，但為了模擬「最終 ESP 已映射」在 `ENTER` 前額外讀取最終位址及四個推入位置。通用匯流排可能有讀取副作用；Intel 契約不包含這些讀取，故原本的 CONFORMED 審查撤回，回到 DRAFT。已取得的原版紀錄與第 2512 步診斷仍是有效歷史證據，修正後須重跑同一驗收命令。

## 修正後證據審查

原版觀測仍足以固定推入值、EBP／ESP 與旗標；Intel 契約只要求推入舊 EBP，不要求讀取最終 ESP 所指內容。現有 `segmentLinear` 可在不觸發匯流排讀取下檢查 SS 描述子權限及上下界，寫入交給既有 `writeSegment32`。dosgolem 目前沒有完整分頁模型，故將頁面映射與任意匯流排寫入失敗留作明示限制；這比虛構讀取更符合此工具已支持的 CPU 契約。核准修正後的層級 0 範圍為 READY；須以「堆疊沒有額外讀取」測試重驗。

## 修正後驗收

移除額外匯流排讀取後，加入會拒絕任何堆疊讀取的測試匯流排；`ENTER` 仍成功推入舊 EBP。`go test -buildvcs=false ./... -count=1` 全套通過，包含固定 1.31 原檔回歸；同一有界診斷再次越過第 2475 步的 `0x1005B`，於第 2512 步、dosgolem **重定位 LE 線性位址** `0x109FF` 的 `66 3B 4D CE` 停下。原版對該下一停點尚無獨立收據。此規格只在列明的 32 位、巢狀層級 0 與可寫堆疊範圍內標為 CONFORMED；完整分頁錯誤、其他巢狀層級、正常玩家路徑及 remake 玩法均未驗收。
