# 220 — 無前綴 byte 記憶體目的 CMP

狀態：**CONFORMED**（限無前綴 `38 /r` 的工具切片）
日期：2026-10-01

## 玩家阻塞與原版輸入

固定 MOO2 1.31 `ORION2.EXE` SHA-256 `4e11be14217b4aafa1839f333bf5eba037f98b0c44e9e4752c96c464c260419f`；dosgolem 已綁定 DPMI、以合成 PSP／環境載入，於第 4168 步、**重定位 LE 線性位址** `0x148224` 的 `38 10 A8 03` 停在未支援的 ModRM `10h`。這不是正常玩家路徑收據。

DOSBox-X 2026.07.02 SDL2 heavy debugger，映像 `fd2-dosbox-x:debug-0d7b272b` ID `sha256:659e8abbf93646f59a1586341769bd4b8f3cd1c707859d7a5de4c56e4672b582`；固定原版 **CS:EIP** `0180:0036C224 → 0180:0036C226` 的同次 `LOG 2` 顯示 `cmp [eax],dl → test al,03`。進入 EAX=`003EC0DCh`、EDX=`0`、DS=`0188h`、EFLAGS=`0202h`；來源 DS:`003EC0DCh` 的一個 byte 前後皆 `00h`，離開 EAX／EDX／DS 不變，EFLAGS=`0246h`（CF=0、ZF=1、SF=0、OF=0、AF=0、PF=1）。版控 `apps/moo2/tools/startup_probe_131.py --cmp-byte` 可重生私有 `cmp-byte-registers.json` SHA-256 `c9763a12d3aa3af48b8c878e97c983ede9c8fb771471f31c1ef8ef90e142b72d`、`cmp-byte-logcpu.txt` SHA-256 `12adee632c97e7ae0d6ddcbd09a393c988602d0329ef7b00c90fe45a3e05e32b`；前後記憶體 SHA-256 均為 `6e340b9cffb37a989ca544e6bb780a2c78901d3fb33738768511a30617afa01d`。原檔與完整擷取留在未版控工作區。

## 擬議通用 CPU 契約

[Intel® 64／IA-32 架構軟體開發手冊第 2A 卷的 `CMP` 條目](https://www.intel.com/content/dam/www/public/us/en/documents/manuals/64-ia-32-architectures-software-developer-vol-2a-manual.pdf) 將 `38 /r` 定義為 `CMP r/m8,r8`：以 r/m8 減 r8 更新 CF、PF、AF、ZF、SF、OF，不寫回任一運算元。`10h` 解碼為 `mod=00`、`reg=DL`、`r/m=[EAX]`。原版這次兩值皆零，只證明零差與記憶體不變；不同值結果、EBP／SIB 預設段與邊界依 Intel 契約和合成測試，不冒稱原版多樣本對拍。

在無 operand-size／segment／repeat 前綴時，`38 /r` 的暫存器目的與既有記憶體 disp8 形狀維持；記憶體目的統一由現有 `decodeAddress32` 讀取一個 byte，依其 DS／SS 預設段，用 `sub8(記憶體值, reg8)` 只更新旗標。未登錄段、記憶體越界、截短 ModRM／SIB／位移時失敗即關閉，旗標與資料不可改動。此為通用 CPU 工具切片；不改 remake 玩法、資料、UI 或存檔。

## 驗收與停止線

證據審查：同次 `LOG` 的原版指令、來源 byte 前後擷取及六旗標與 Intel `38 /r` 定義相容；`decodeAddress32` 已是本核心其他記憶體指令的共用位址解碼入口。原版只測到 `mod=00,r/m=EAX,reg=DL`，其餘編碼形狀以手冊與合成測試為通用 CPU 契約。這足以核准上述無前綴工具形狀，並不核准 MOO2 正常玩家路徑完成。

合成測試須覆蓋 `38 10` 的零差與異值、方向、資料及暫存器不變、EBP→SS、非 EBP→DS、越界及截短位移失敗。固定原檔診斷須越過第 4168 步並列新停點；`go test -buildvcs=false ./internal/cpu386 ./internal/machine -count=1` 與固定原檔輸入的 `go test -buildvcs=false ./... -count=1` 須通過。驗收僅限此 CPU 工具切片與合成啟動路徑；PSP／環境、正常遊玩與同狀態 remake 對拍仍未知。

## 有限驗收結果

`internal/cpu386/cpu.go` 的 `38 /r` 記憶體形狀改用 `decodeAddress32` 選 DS／SS 並讀來源；暫存器形狀維持原處理。合成測試覆蓋原版零差、異值方向、不寫回、EBP 與 SIB ESP 的 SS、段界限、截短位移／SIB、repeat 前綴拒絕。`go test -buildvcs=false ./internal/cpu386 ./internal/machine -count=1`、固定原檔輸入的 `go test -buildvcs=false ./... -count=1` 均通過。合成 PSP／環境的固定真檔診斷越過第 4168 步，至第 4944 步、**dosgolem 重定位 LE 線性位址** `0x146903` 的 `26 8A 1E 42 84` 停於未支援的帶 ES 覆寫 byte 載入。下一停點還沒有原版對應收據；不以此推論正常玩家路徑或玩法已對齊。
