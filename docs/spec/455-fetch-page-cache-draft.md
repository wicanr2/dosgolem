# DRAFT：分頁微快取與取指令頁快取（perf-cpu 分支）

狀態：DRAFT。先實作後補規格（偏離任務第 4 點，見回報）。實作在 `dosgolem-sc-perf` 工作樹，未提交。

## 1. 範圍

只改執行速度，不改任何可觀察行為。兩項都是通用層能力（換一支 binary 仍成立），放 `internal/machine` 與 `internal/cpu`。

| 項目 | 檔案 | 作用 |
|---|---|---|
| 分頁微快取 | `internal/machine/paging.go`（`utlbR`、`utlbW`、`xlatRead`、`xlatWrite`），`linear.go` 的 `readLin`／`writeLin` 改呼叫它們 | 分頁開啟時省掉 `xlatPage` 的兩層查表與權限判斷 |
| 取指令頁快取 | `internal/cpu/cpu.go`（`FetchPager`、`SetFetchPager`、`FlushFetch`、`fetchLin`），`segmented.go` 的 `segFetch8`；`internal/machine/fetchpage.go`（`FetchRead8`、`flushFetch`） | `CPU.Code == nil` 時（轉換開啟：V86、分段模式），同一個 4 KB 線性頁內取指令直接索引 `Mem`／`Ext` |

不做：資料讀取頁快取。以同一機制接到 `read8`／`read16`／`read32`／`segRead`／`strRead` 試過，基準沒有加速（s760 2.12 s → 2.56 s，在雜訊範圍內偏慢），已還原。

## 2. 新介面

- `cpu.FetchPager`：`FetchRead8(lin uint32) (uint8, []byte)`。回傳值與副作用必須與 `Bus.Read8(lin)` 完全相同；另回一個 4 KB slice，保證「下一次 `FlushFetch` 之前，同一線性頁任何位移的 `Read8` 都等於直接索引它，且沒有副作用」，不能保證回 nil。
- `(*cpu.CPU).SetFetchPager(p)`、`(*cpu.CPU).FlushFetch()`。
- `(*machine.Machine).FetchRead8`。`machine.New` 在 `cpu.New` 之後裝上。
- CPU 新欄位 `fpPager`、`fpTag`（線性頁號＋1，0＝無效，零值 CPU 安全）、`fpPage`；Machine 新欄位 `utlbR`、`utlbW`（結構尾端）。都不存檔、不進快照、不進 Digest。

## 3. 等價條件

### 3.1 分頁微快取

- 條目鍵：線性頁號、`tlbGen`、user 身分。64 筆直接對映，讀寫各一份。
- 只在 `xlatPage` 成功後填入。成功表示共用 TLB 在目前世代有該頁條目且以同一身分通過權限；同世代同身分再查，`xlatPage` 必走命中分支、結果相同。
- 寫入份填入時 dirty 已寫回（命中分支 `tlbMarkDirty`，或 `walk` 時 `pteD` 已設），之後寫入命中沒有頁表副作用。讀取命中本來就沒有副作用。
- 失敗路徑不填，錯誤紀錄照舊每次產生。
- A20 遮罩仍在 `readLin`／`writeLin` 轉換之後套用，不進快取。

### 3.2 取指令頁快取

`FetchRead8` 先做一次原樣的 `Read8`，交出頁面要全部成立：

1. `m.xlat` 為真；
2. `onRead == nil`（沒有讀取監看）；
3. 分頁開啟時，讀取微快取在該頁有目前世代、目前 `cplUser` 的條目（剛才那次讀取轉換成功）；
4. 套 A20 後的實體頁整塊在 `Mem` 的 `[0, A0000h)` 或 `[C0000h, 1 MB)`，或整塊在 `Ext` 內。`A0000h–BFFFFh` 一律不交（平面模式讀取載入 VGA latch）。

快取只記轉換，位元組每次從 `Mem`／`Ext` 現讀，自我修改碼照常生效。分段模式的界限與 D=1 檢查仍在 `segFetch8` 先做。

## 4. 失效表

| 事件 | 經過 | 作廢 |
|---|---|---|
| `SetCR0`（PE、PG 改變）、`SetCR3`、`SetA20`、`SetProtectedModeProfile`、`applyPM`（`Restore`、`LoadState`） | `invalidateTLB` | 微快取（世代號）、取指令頁快取（`flushFetch`） |
| `tlbGen` 溢位 | `tlbClear` | 微快取整份清零 |
| xlat、paging、cplUser 改變（`SetCR0`、`SetEFlagsHi`、`SetPMCPL`、`SetProtectedModeProfile`、`applyPM`） | `recalcPM` → `syncCodeFastPath` | 取指令頁快取 |
| `SetA20`、`WatchReads`、`LoadState`、`Restore` | `syncCodeFastPath` | 取指令頁快取 |
| `Ext` 配置 | `SetExtendedMemoryKB` | 取指令頁快取 |

佐證：對 `internal/machine` 非測試檔 grep `cplUser`、`eflagsHi`、`pmCPL`、`cr0`、`xlat`、`paging`、`onRead`、`a20`、`Ext`、`Mem` 的直接賦值，只出現在 `linear.go` 的 setter 與 `recalcPM`、`pmstate.go` 的 `applyPM`、`machine.go` 的 `SetA20`／`WatchReads`、`state.go` 的 `LoadState`、`extmem.go`，以及 `dpmi.go` 的 `Mem` 擴充（屬 `LEMachine`，不用這台機器的 CPU）。`SetSegmented` 的機器端呼叫點（`handoff.go`、`taskreturn.go`、`v86deliver.go`、`linear.go`、`pmstate.go`）之後都接 `SetPMCPL`、`SetEFlagsHi` 或 `recalcPM`。

## 5. 測試

- `internal/machine/utlb_test.go`：`TestUTLBDifferential`（8 個種子、各 20 萬次隨機讀寫，穿插改 PTE 不作廢、`SetCR3`、`SetA20`、user／監督者，線性頁跨過 64 槽產生碰撞；對照每次直接走 `xlatPage` 的參考實作，逐步比回傳值與錯誤紀錄，最後比 `Mem`、`Ext`、`PageTableWrites`）；`TestUTLBInvalidatedByCR3`。
- `internal/machine/fetchpage_test.go`：`TestFetchPageDifferential`（V86 分頁程式含自我修改碼；中途改 PTE、`SetCR3`、切 A20；有快取與 `SetFetchPager(nil)` 兩台逐步比暫存器，最後比記憶體）。
- 負對照：微快取命中不比世代 → 兩支 UTLB 測試失敗；`FlushFetch` 改成空函式 → `TestFetchPageDifferential` 在 step 1500（SetCR3 之後）失敗。
- 同狀態 A/B：`session/sc_perf_long_test.go`（`DOSGOLEM_PERF_STATE` 閘門），s760、s800 各 4000 萬步，觀察器關／開，Digest、cpu32、Cycles、SaveState 全串流雜湊與加速前逐位元組相同。
- 長測試：飛行 Q、字幕 C11、存讀檔 F–K 收據與 `s011impl/lt/g-formal` 逐位元組相同。
