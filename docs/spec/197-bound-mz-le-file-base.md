# 197 — 內嵌 MZ 的 LE 資料頁基址

狀態：**CONFORMED**（僅明示內嵌 MZ 基址的格式載入與入口前綴）
日期：2026-09-30

## 證據與勘誤

1996 光碟版 `Orion2.exe`：SHA-256 `7ae2ac2e5904ca330009af2827279d889906b0b9b7a8854c38eb707a56e955b5`。1.31 `ORION2.EXE`：SHA-256 `4e11be14217b4aafa1839f333bf5eba037f98b0c44e9e4752c96c464c260419f`。兩者在原始檔案偏移 `0x26654` 均有第二個 `MZ`；該 MZ 的 `e_lfanew=0x2C90`，指向 `0x292E4` 的 `LE`。1996 版 LE 標頭 `DataPagesOffset=0x6F040`，首資料頁在 `0x95694`；1.31 對應值為 `0x6F000`、`0x95654`。1996 版首頁 bytes 為 `CC EB FD 90...`，object 1 `+0xFFF18` 對應原檔 `0x1955AC`，bytes 為 `EB 76 57 41 54 43 4F 4D`，亦即跳過 `WATCOM` 標記的 Watcom 啟動入口。

IDA Pro 9.4 的 `Orion2.exe.i64`（SHA-256 `4a01791fcf877ed87a740a54748694ab34a02675e3117dac052aeaa3f883944e`；原始輸入 MD5 `bacb10a92454d2f9b211eb9fe67ec099`）在 IDA 線性位址 `0x10FF18` 的 `start` 映至原檔 `0x1955AC`，同樣是 `EB 76 WATCOM`。公開 [Open Watcom 32 位 DOS 啟動碼](https://raw.githubusercontent.com/open-watcom/open-watcom-v2/master/bld/clib/startup/a/cstrt386.asm)也以短跳躍及 `WATCOM` 標記辨認啟動碼；此公開來源只作交叉驗證，不把其完整控制流當作 MOO2 玩法證據。

**勘誤（已證實）**：195 的 `LoadLEAt(original, 0x292E4)` 使用檔案零點作資料頁基址，從 `0x6F040` 複製錯誤頁面；其 `0x10FF18` 的 `66 3B 15 ...`、5,359 步及 `POP EDX` 停點都不是 MOO2 真正入口執行收據。195 的明示 LE 標頭解析與 fixup 表盤點仍成立；196 的通用 CPU `66 3B 15` 指令測試仍成立，但不是 MOO2 啟動必需能力。

## 契約

- 新增 `InspectLEInMZ(data, mzBase)`、`LoadLEInMZ(data, mzBase)`。只依呼叫者明示的非零內嵌 MZ 基址，驗證 `MZ`、`e_lfanew`、LE 簽章、所有邊界與溢位。不得掃描檔案、猜測基址或修改原版 bytes。
- LE 標頭相對表格仍以 LE 標頭為基址；`DataPagesOffset` 僅以內嵌 MZ 為基址，套用到每個物理 page。原有檔案零點 `InspectLE`／`LoadLE` 與明示 LE 標頭 `InspectLEAt`／`LoadLEAt` 行為保持原契約。
- `cmd/leprobe -mz-base 0x26654` 列出 MZ、LE、資料頁基址及重定位預覽；與 `-offset` 互斥。FD2 專用 `-execute-entry-prefix` 仍只適用預設入口。
- 真檔驗證兩版本 object 1 `+0xFFF18` 都是 `EB 76 WATCOM`，並以未修改原版自然執行記錄第一個新停點。這只驗收啟動前綴；不宣稱玩法或畫面對拍。

## 證據審查

`0x26654 + 0x2C90 = 0x292E4`；`0x26654 + 0x6F040 = 0x95694`；`0x95694 + 0xFFF18 = 0x1955AC`。三個落點分別由原始 `MZ`／`LE`／`CC EB FD`／`EB 76 WATCOM` bytes 與 IDA 原檔映射獨立核對。若任一原檔雜湊、位址或 bytes 不符，真檔探針失敗即關閉。沒有推定外層堆疊或 DOS/4GW 私有狀態。

## 驗收

合成前綴 fixture 驗證資料頁必須從內嵌 MZ 基址讀取，拒絕錯誤基址與超界 `e_lfanew`，且不改寫輸入。兩版固定雜湊真檔由 `LoadLEInMZ(original, 0x26654)` 均讀出 `EB 76 WATCOM`。`go test ./...`、1996 版獨立真檔入口測試均通過；`cmd/leprobe -mz-base 0x26654` 重生兩版 object／fixup 摘要。自然執行均在第 11 步的 LE 線性位址 `0x10FFB5` 要求 `INT 21h/AH=30h`，`EAX=0x3000`、`EBX=0x50484152`（`PHAR`）；未接 DOS 服務，尚無玩法對拍。
