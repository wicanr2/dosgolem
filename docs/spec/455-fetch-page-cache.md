# 455：分頁微快取與取指令頁快取

狀態：**READY**（第 2 版，依審查 A、B 修訂），已實作（§7）。事後規格：實作 commit `9848a5c` 早於本規格，違反 SDD 與證據閘門，第 2 版補齊。
日期：2026-10-22
前置：[`015-execution-speed`](015-execution-speed.md) §4.2（`CPU.Code` 快路徑）、[`432-linear-memory-and-paging`](432-linear-memory-and-paging.md)（線性層、TLB、A20）、
[`434-legacy-core-segmented-mode`](434-legacy-core-segmented-mode.md)（分段模式取指令）、[`435-v86-hooks-and-idt-delivery`](435-v86-hooks-and-idt-delivery.md)（V86）

推論等級：已證實、強推論、假說、未知。程式位置以 `strike-commander` 分支 perf-cpu 為準。

## 1. 範圍與決定

只改執行速度，不改任何可觀察行為。兩項都是通用層能力（換一支 binary 仍成立）。

| 項目 | 檔案 | 作用 |
|---|---|---|
| 分頁微快取 | `internal/machine/paging.go`（`utlbR`、`utlbW`、`xlatRead`、`xlatWrite`）；`linear.go` 的 `readLin`／`writeLin` 改呼叫它們 | 分頁開啟時省掉 `xlatPage` 的兩層查表與權限判斷 |
| 取指令頁快取 | `internal/cpu/cpu.go`（`FetchPager`、`SetFetchPager`、`FlushFetch`、`fetchLin`）、`segmented.go` 的 `segFetch8`；`internal/machine/fetchpage.go`（`FetchRead8`、`flushFetch`） | 轉換開啟（V86、分段模式）時，同一個 4 KB 線性頁內取指令直接索引 `Mem`／`Ext` |

決定：

- 不做資料讀取頁快取：以同一機制接到 `read8`／`read16`／`read32`／`segRead`／`strRead` 試過，量不到加速，已還原（強推論：只量過整機牆鐘，雜訊大）。
- 不做 `Machine.Step` 的「下一事件步數」合併：最多省約 1%，要在匯出欄位 `VGAFrameEvery`、`IRQ0Every` 的每個寫入點重算，漏掉時計時器靜默停住（`strike_commander/workplace/perf/draft-2b-step-fixed-cost.md`）。
- 457（executeCore 值 switch）已撤回，見 [`457-executecore-value-switch`](457-executecore-value-switch.md)。

## 2. 新介面

- `cpu.FetchPager`：`FetchRead8(lin uint32) (uint8, []byte)`。回傳值與副作用必須與 `Bus.Read8(lin)` 完全相同；另回一個 4 KB slice，
  保證「下一次 `FlushFetch` 之前，同一線性頁任何位移的 `Read8` 都等於直接索引它，且沒有副作用」，不能保證回 nil。
- `(*cpu.CPU).SetFetchPager(p)`（含清空）、`(*cpu.CPU).FlushFetch()`。
- `(*machine.Machine).FetchRead8`。`syncCodeFastPath` 在 `xlat` 為真時 `SetFetchPager(m)`，否則 `SetFetchPager(nil)`（§3.3）。
- CPU 新欄位 `fpPager`、`fpTag`（線性頁號＋1，0＝無效，零值 CPU 安全）、`fpPage`；Machine 新欄位 `utlbR`、`utlbW`（結構尾端）、
  `noFetchPager`（測試用對照開關）。都不存檔、不進快照、不進 Digest。

**FetchPager 繞過 `CPU.Bus`**：裝上之後，同一頁的取指令不再呼叫 `Bus.Read8`。因此：

- 換掉 `CPU.Bus`（例如包一層記錄或監看的 Bus）的呼叫端必須先 `SetFetchPager(nil)`，否則新 Bus 看不到取指令。目前 repo 內沒有這種呼叫端；
  `workplace/moo2-probe/main.go:1706` 有（工作區工具，不在 repo 程式碼內），要用時照此處理。
- 以整個結構複製保存、還原 CPU 的呼叫端，還原後必須 `FlushFetch`：`oracle/call.go` 的 `CallBudget` 已照做。oracle 不開保護模式 profile，
  `xlat` 恆為假、不會裝 FetchPager，這一行目前是防禦性的（§5 的 M11）。

## 3. 規則

### 3.1 分頁微快取

- 條目鍵：線性頁號、`tlbGen`、user 身分。64 筆直接對映，讀寫各一份。`gen` 為 0 的條目永不有效（`tlbGen` 從 1 起算）。
- 只在 `xlatPage` 成功後填入。成功表示共用 TLB 在目前世代有該頁條目且以同一身分通過權限；同世代同身分再查，`xlatPage` 必走命中分支、結果相同。
- 寫入份填入時 dirty 已寫回（命中分支 `tlbMarkDirty`，或 `walk` 時 `pteD` 已設），之後寫入命中沒有頁表副作用。讀取命中本來就沒有副作用。
- 失敗路徑不填，錯誤紀錄照舊每次產生。A20 遮罩仍在轉換之後套用，不進快取。
- `tlbGen` 溢位時 `tlbClear` 把微快取整份清零（世代回到 1 之後舊條目的 gen 可能碰巧又是 1）。

### 3.2 取指令頁快取

`FetchRead8` 先做一次原樣的 `Read8`，交出頁面要全部成立：

1. `m.xlat` 為真；
2. `onRead == nil`（沒有讀取監看）；
3. 分頁開啟時，讀取微快取在該頁有目前世代、目前 `cplUser` 的條目（剛才那次讀取轉換成功）；
4. 套 A20 後的實體頁整塊在 `Mem` 的 `[0, A0000h)` 或 `[C0000h, 1 MB)`，或整塊在 `Ext` 內。`A0000h–BFFFFh` 一律不交（平面模式讀取載入 VGA latch）。
   頁尾以 uint64 計算：實體頁 `FFFFF000h` 的頁尾 `1_0000_0000h` 以 uint32 計會環繞成 0，第 1 版因此在該頁 panic（審查 A 應改 1），第 2 版修正。

快取只記轉換，位元組每次從 `Mem`／`Ext` 現讀，自我修改碼照常生效。分段模式的界限與 D=1 檢查仍在 `segFetch8` 先做。

### 3.3 只在轉換開啟時裝上

第 1 版在 `machine.New` 一律裝上 FetchPager。轉換關閉（實模式含 A20 開，`CPU.Code` 停用）時 `FetchRead8` 一律不交頁，
每個指令位元組因此多一層呼叫（審查 B 應改 D）。第 2 版改由 `syncCodeFastPath` 依 `xlat` 裝上或拿掉；`xlat` 只在 `recalcPM` 改變，
`recalcPM` 一定呼叫 `syncCodeFastPath`。

## 4. 失效表

| 事件 | 經過 | 作廢 |
|---|---|---|
| `SetCR0`（PE、PG 改變）、`SetCR3`、`SetA20`、`SetProtectedModeProfile`、`applyPM`（`Restore`、`LoadState`） | `invalidateTLB` | 微快取（世代號）、取指令頁快取（`flushFetch`） |
| `tlbGen` 溢位 | `tlbClear` | 微快取整份清零 |
| 每次 `recalcPM`（`SetCR0`、`SetEFlagsHi`、`SetPMCPL`、`SetProtectedModeProfile`、`applyPM`），以及 `SetA20`、`WatchReads`、`LoadState`、`Restore` | `syncCodeFastPath` → `SetFetchPager` | 取指令頁快取（不論 xlat、paging、cplUser 有沒有改變都清空） |
| `Ext` 配置 | `SetExtendedMemoryKB` | 取指令頁快取（防禦性：配置前 `Ext` 為空，不會交出 Ext 頁，且只能配置一次；§5 的 M3） |
| `oracle.CallBudget` 還原 CPU 結構 | 呼叫端 | 取指令頁快取 |

佐證（已證實，容器內 grep）：`internal/machine` 非測試檔對 `cplUser`、`eflagsHi`、`pmCPL`、`cr0`、`xlat`、`paging`、`onRead`、`a20`、`Ext`、`Mem` 的直接賦值，
只出現在 `linear.go` 的 setter 與 `recalcPM`、`pmstate.go` 的 `applyPM`、`machine.go` 的 `SetA20`／`WatchReads`、`state.go` 的 `LoadState`、`extmem.go`，
以及 `dpmi.go` 的 `Mem` 擴充（屬 `LEMachine`，不用這台機器的 CPU）。`SetSegmented` 的機器端呼叫點（`handoff.go`、`taskreturn.go`、`v86deliver.go`、
`linear.go`、`pmstate.go`）之後都接 `SetPMCPL`、`SetEFlagsHi` 或 `recalcPM`。repo 內以結構複製還原 CPU 的只有 `oracle/call.go`。

## 5. 測試與負對照

測試（`internal/machine`）：

| 測試 | 內容 |
|---|---|
| `TestUTLBDifferential` | 8 個種子各 20 萬次隨機讀寫，穿插改 PTE 不作廢、`SetCR3`、`SetA20`、user／監督者，線性頁跨過 64 槽；對照每次直接走 `xlatPage` 的參考實作 |
| `TestUTLBInvalidatedByCR3` | 改 PTE 不作廢保留舊轉換，`SetCR3` 後讀到新頁 |
| `TestFetchPageDifferential` | V86 分頁程式含自我修改碼；中途改 PTE、`SetCR3`、切 A20；與 `noFetchPager` 對照機逐步比 |
| `TestFetchPageCPLSwitch` | PE=1、VM=0、CPL 0 在監督者頁執行，切 CPL 3（只經 recalcPM），之後取指令必須頁錯誤；與對照機逐步比錯誤與紀錄 |
| `TestFetchPageWatchReadsMidRun` | V86 執行中裝讀取監看，監看觸發次數與對照機相同 |
| `TestFetchPageTopOf4GB` | 線性頁映到實體 `FFFFF000h`：`FetchRead8` 不 panic、與 `Read8` 相同、不交頁 |
| `TestFetchPagePageEqualsRead8` | 屬性測試：400 組隨機頁表（低記憶體、A0000–BFFFF、C0000 以上、Ext、Ext 之外、`FFFFF000h`）、A20、平面模式、監督者讀後切使用者、讀取監看；交出的頁 4096 個位移都等於 `Read8` 且無頁錯誤，有監看時不交頁 |
| `BenchmarkStepRealA20` | 實模式 A20 開（§3.3 的路徑） |

變異表（`strike_commander/workplace/perf/mutants.sh`，容器內複製工作樹逐一套用，跑 `internal/machine` 的 FetchPage／UTLB／Paging／PageFault 測試與 `oracle`；
結果 `workplace/perf/log/mutants.log`）：

| 編號 | 突變 | 結果 |
|---|---|---|
| M1 | `FlushFetch` 改成空函式 | 被抓到：`TestFetchPageDifferential` |
| M2 | `syncCodeFastPath` 不換裝、不清空（`New` 裝一次） | 被抓到：`TestFetchPageCPLSwitch`、`TestFetchPageWatchReadsMidRun` |
| M3 | `SetExtendedMemoryKB` 不 `flushFetch` | 存活：等價突變。配置前 `Ext` 為空，`FetchRead8` 不可能交出 Ext 頁；`Ext` 只能配置一次（第二次回錯誤）。保留為防禦性作廢 |
| M4 | `FetchRead8` 不比 `cplUser` | 被抓到：`TestFetchPageCPLSwitch`、`TestFetchPagePageEqualsRead8` |
| M5 | `FetchRead8` 不排除 A0000–BFFFF | 被抓到：`TestFetchPagePageEqualsRead8` |
| M6 | `FetchRead8` 不看 `onRead` | 被抓到：`TestFetchPageWatchReadsMidRun`、`TestFetchPagePageEqualsRead8` |
| M7 | 微快取命中不比世代 | 被抓到：`TestPagingJEMMMappingState1`、`TestPagingStaleTranslations`、`TestUTLBDifferential` |
| M8 | `FetchRead8` 不套 A20 | 被抓到：`TestFetchPagePageEqualsRead8` |
| M9 | `invalidateTLB` 不 `flushFetch` | 被抓到：`TestFetchPageDifferential` |
| M10 | 頁尾改回 uint32 | 被抓到：`TestFetchPageTopOf4GB`（panic） |
| M11 | `oracle.CallBudget` 還原後不 `FlushFetch` | 存活：在目前程式是等價突變。oracle 不開保護模式 profile，`xlat` 恆為假，不裝 FetchPager |

## 6. 未知

- 整機加速倍數：未知。整機基準（s760、s800 裸 RunUntil 4000 萬步）的牆鐘與 CPU 時間在同設定下波動 2–3 倍，中位數差 −4% 到 −28%，不寫倍數（審查 B 應改 C）。
- 實際遊玩畫格率：未量。

## 7. 實作收據

- commit：第 1 版 `9848a5c`；第 2 版修正（uint64 頁尾、只在 xlat 裝 FetchPager、oracle 還原後清空、測試、`BenchmarkStepRealA20`）在 `1669fe4` 之上，由主代理提交。
- SingleStepTests：`/home/anr2/cht/dosgolem/testdata/8088` 唯讀掛載，315 PASS、0 FAIL，已知差距 F6.7 6218、F7.7 6371，與 `07cabfa` 相同。
- 同狀態錨點（`machine.StateDigest`，與行程無關）：`07cabfa` 與第 2 版工作樹，從 `i021/st/s760.state`、`s800.state` 各跑 4000 萬步（`session/sc_perf_long_test.go`，
  `DOSGOLEM_PERF_STATE`），觀察器關、開共 8 次：

  | 狀態 | StateDigest | Digest 的 mem |
  |---|---|---|
  | s760（+4000 萬步） | `6364114d28e65a898f35bb5d16502ab5e499d3006be3ea19f864ee047e1a424e` | `e49e3b7b…520d` |
  | s800（+4000 萬步） | `27dedd3e3754da70bd292665661a27d7baa5ba703d3fee164e3e3feff64cadca` | `0437988d…e6aa` |

- 微基準（`internal/machine` 的 `BenchmarkStep*`，`-benchtime 3000000x`，三個版本交錯 10 輪，`--cpus 2`，主機另有負載；
  `workplace/perf/microbench.sh`、`log/microbench.tsv`）。ns/op 最小值（中位數）：

  | 基準 | `07cabfa` | `f97abb2`（第 1 版） | 第 2 版工作樹 |
  |---|---|---|---|
  | StepV86Paged | 58.9（134.2） | 47.1（101.6） | 44.5（98.8） |
  | StepV86 | 48.1（111.4） | 41.7（93.2） | 39.8（92.9） |
  | StepRealA20 | 45.6（105.2） | 54.5（115.6） | 43.3（94.6） |
  | StepRealMode（對照，路徑未改） | 30.5（76.5） | 37.0（89.5） | 34.5（77.9） |

  主機負載呈兩群（安靜時約 30–60 ns，忙碌時約 75–180 ns），各版的全距互相重疊，對照組 StepRealMode 的最小值也有約 ±20% 的差。
  StepV86Paged 在安靜群的最小三個值（`07cabfa` 58.9、60.2、64.2；第 2 版 44.5、45.6、56.9）與中位數都較低，寫成「分頁 V86 每步約快 20–25%（強推論）」。
  StepRealA20 第 1 版較 `07cabfa` 慢、第 2 版回到同級（假說：方向與審查 B 的臨時基準一致，幅度在雜訊內）。
- 合併前長測試（`strike_commander/workplace/perf/lt-full.sh r2`，工作樹 `1669fe4` 加第 2 版改動，每組 `git.txt` 記 rev-parse 與 porcelain；
  收據在 `workplace/perf/lt/r2/`）：
  - 與 `s011impl/lt/g-formal` 逐位元組相同：ComposedLabelCaseD（case-d 37 檔）、KillBoardCaseE（case-e 與 shot 檔）、LabelFrameCaseH（121 檔）、
    MenuCase（L 121、M1 73、M2 73、N 85 檔）、FlightCase（Q 2353、R 806 檔，對 `fl2`）、SubtitleCase（C11 50、H12 35 檔）、SaveLoadCase（F–K）。
  - 只看 PASS：PointerMoveTrigger；LabelOverlayTrigger 以測試圖集（`s011impl/font-test`，不給預期表，同 `negq.sh` 的案例 A）PASS，
    case-a、b、c 收據與 `07cabfa` 同設定重跑逐位元組相同（以正式圖集跑 A、B、C 會因缺字失敗，與本改動無關）；
    session 的 BootFromStateTrigger、IRQ1ScanTrigger、LegacyPMProfileTrigger 全部 PASS（測試內含既有錨點）。
  - `go vet ./...` 通過；`go test ./internal/... ./session/... ./apps/strikecommander/...` 通過；SingleStepTests 315／0／0。
