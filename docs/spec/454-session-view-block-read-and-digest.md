# 454：session 唯讀視圖的批次讀取、送達計數與完整狀態摘要

狀態：**READY**（第 6 版；陸空戰將規格 012 的四輪獨立審查一併審過，處置見附錄 A；2026-10-22 轉 READY），已實作（2026-10-23）。實作收據：陸空戰將 repo `workplace/impl458/receipt.md`（§3.1 第 1–11 項、負對照 N1–N6／N8–N15、§3.2 靜態檢查與微基準）；主代理在容器內重跑 machine、session、repolint 套件通過，SingleStepTests 315 PASS，負對照「拿掉 `raiseIRQ` 的送達計數」兩套件皆失敗。實作差異：§2.3 沿用既有 `internal/dos/state_digest.go`；§3.1 第 8 項「轉換開、分頁關」組合因 `SetProtectedModeProfile` 無延伸記憶體時回錯而未測；§3.2 第 2 項比對機器碼時另去掉位元組欄（RIP 相對位移隨符號位置改變）；§3.1 第 4 項 IRQ7 沒有用 452 的 SB16 合成程式，改在觀察點白箱立起 `sb8.irr7`，仍經 `sb8IRQ7 → raiseIRQ` 送達，主代理接受（驗的是送達計數，不是 SB16 時序）。實作收據存於陸空戰將 repo 本機 `workplace/impl458/`（不進版控）
日期：2026-10-22
修訂：§2.1 送達來源與設置點、§2.3 已知缺漏表依 [`459`](459-mouse-irq12-v86-delivery.md) §7.2 修訂（各註「由 459 修訂」，2026-10-23）。
前置：[`006-layering`](006-layering.md)（通用層判準）、[`238-session-step-observer-draft`](238-session-step-observer-draft.md)（逐步觀察器）、[`432-linear-memory-and-paging`](432-linear-memory-and-paging.md) §5.3、§6（分頁錯誤、觀測轉換）、[`435-v86-hooks-and-idt-delivery`](435-v86-hooks-and-idt-delivery.md) §2.2、§2.4（V86 事件與 IDT 遞送）、[`444-legacy-sb16-dma8-irq7`](444-legacy-sb16-dma8-irq7.md) §2.7（IRQ7、`eventThisStep`）、[`450-session-boot-from-state`](450-session-boot-from-state.md)、[`452-session-sbdma-observer`](452-session-sbdma-observer.md)（選配擴充的寫法）
使用端：strike_commander 規格 012（`docs/spec/012-strike-3d-hd-rerasterize.md`）

dosgolem 的程式位置以 `strike-commander` 分支 commit `07cabfa` 為準；效能分支 perf-cpu 目前為 commit `1669fe4`，與 `07cabfa` 的行號有位移，實作時以 perf-cpu 併回後的行號為準。推論等級：已證實、強推論、假說、未知。

用語：

| 用語 | 意義 |
|---|---|
| 步驟嘗試 | 一次 `machine.Step()`（452 用語） |
| 觀察點 | `BeforeStep` 被呼叫的時點：中斷點與述詞檢查之後、該次步驟嘗試之前（`internal/machine/probe.go:197–212`） |
| 觀察到的 PC | 觀察點當下的 `CS:IP` |
| 送達 | 機器在步驟嘗試開始時（執行觀察到的 PC 那道指令之前）把控制權交給不是該指令的程式碼：回呼、週期遠呼叫、硬體 IRQ |

---

## 1. 為什麼要做

strike_commander 規格 012（3D 高解析，做法 A）在 STRIKE 的光柵化入口讀堆疊參數與頂點，在 3D 完成點讀 64000 bytes 的畫外緩衝，並要求 HD 開關的同狀態 A/B 逐位元組相同。現況有四個缺口：

| 缺口 | 現況 | 等級 |
|---|---|---|
| 觀察到的 PC 不一定會執行 | `Machine.Step` 在 `CPU.Step` 之前先處理回呼、週期遠呼叫、IRQ0、IRQ1、IRQ7（`internal/machine/machine.go:1492–1523`）。送達時，觀察點看到的 PC 那道指令這一步不會執行，返回後同一個 PC 會再被觀察到一次。審查 B 的合成測試：PC = `0101` 命中 1540 次，其中 81 次是這種重複 | 已證實（程式；審查 B 實測） |
| 大塊讀取慢 | `StepView` 只有 `Read8`、`Read16`（`session/observer.go:127–139`）。審查 B 實測逐位元組讀 64000 bytes：開分頁 921 µs、線性 279 µs、實模式 292 µs；批次原型約 11 µs | 已證實（審查 B 實測） |
| 摘要不含 32 位元狀態 | `Digest` 的 CPU 雜湊只含 `R`、`Seg`、`IP`、`Flags`（`session/digest.go:57–66`），不含 `Hi`、`FSeg`、`GSeg`、`PM`（`internal/machine/state.go:68–69、157`）。RE 025 §3.2：掛錄製與不掛跑到同一步，`Digest` 全欄位相同，但只證明到 `Digest` 的範圍 | 已證實（程式；RE 025 收據） |
| 摘要不含 DOS 層 | `Digest` 只有 `DOSExited`；DOS 層另有自己的存檔（`internal/dos/state.go:152`） | 已證實（程式） |

依 006 的判準，「這一步有沒有送達」「讀一段線性記憶體」「整台機器與 DOS 的狀態摘要」換一支 binary 都成立，放 session、machine、dos。哪些位址要觀察、怎麼去重、畫格怎麼分組，是《Strike Commander》的知識，留在 `apps/strikecommander`（§4 決定 1）。本規格只放 012 實際使用的能力（§4 決定 3 到 5）。

## 2. 規則

### 2.1 送達計數

```go
// Deliveries 回累計的送達次數（docs/spec/454 §2.1）。
func (m *Machine) Deliveries() uint64
// Deliveries 回同一個值；保留的視圖回 0 並設 viewMisuse。
func (v StepView) Deliveries() uint64
```

machine 內部欄位 `deliveries uint64`，放在 `Machine` 結構的最後（§3.2 的靜態檢查以此為前提）。遞增點只有既有的三種送達：

| 來源 | 遞增位置（`07cabfa`） | 呼叫端 |
|---|---|---|
| 外部排入的回呼 | `tick` 內 `startCallback()` 回真之後（`machine.go:1631–1632`）。只在實模式剖面（保護模式剖面的滑鼠事件改走 IRQ12，計入下一列的硬體 IRQ；由 459 修訂，2026-10-23） | — |
| 週期遠呼叫 | `tick` 內 `CPU.FarCall(periodic…)` 之前（`machine.go:1640–1642`） | — |
| 硬體 IRQ | 共用的 `raiseIRQ`（`internal/machine/v86deliver.go:256`）開頭 | IRQ0 `machine.go:1701`、IRQ1 佇列 `keyboard.go:128`、IRQ1 定時鍵 `keyhold.go:234`、IRQ7 `sb_dma8.go:837`、IRQ12 `tick` 的 `mouseIRQ12`（`mouse_irq12.go:89`，以 `5b88abd` 為準；保護模式剖面，由 459 修訂，2026-10-23） |

- 這三種與 444 §2.7 的 `eventThisStep = true` 設置點一一對應，共七處（`keyboard.go:127`、`keyhold.go:233`、`machine.go:1632`、`1641`、`1700`、`sb_dma8.go:835`，加上 459 的 `mouse_irq12.go:88`；七處由 459 修訂，2026-10-23，新增那一處以 `5b88abd` 為準）。保護模式剖面的 IRQ12 每個滑鼠事件只計 1 次，樁內的 `int FBh`、`int FCh` 與使用者回呼入口不計（459 §6.2 第 12 項）。完整性驗收：實作者以 `grep -n "eventThisStep = true"` 與 `grep -n "raiseIRQ("` 列出兩組位置，寫進 §5，逐一對上本表。
- 遞增不放在 `commitV86Delivery`：那裡同時遞送 V86 的軟體中斷與例外（435 §2.4），放在那裡會把例外算進去。
- 不計任何例外。例外是指令本身執行的結果，不是機器在步驟嘗試開始時另外交出控制：
  - V86 下的 `INT n`（`internal/cpu/ops.go:99–104` 判斷 V86，`doInt` 回 `ErrV86Event`），不論 gate DPL 3、0 或 IOPL < 3；
  - `V86Exception`（#DE、#BR、#GP，`internal/cpu/v86event.go:29`），含 DPL 0 的 `CD nn` 改送 #GP；
  - 實模式的 #DE、#BR；
  - 分頁錯誤：dosgolem 不經 IDT 遞送 #PF（432 §5.3；`paging.go:105–109` 只回 `PageFaultError`），本規格不處理。
- 消費端若在可能產生例外的 PC 上觀察，要自行處理重複；012 的資料命中點都是函式入口與返回點，不會產生例外（012 §3.2.1，審查核對）。
- 計數不進狀態檔、不進 `Digest`、不進 `MachineDigest`。`BootFromState` 之後從 0 起算；消費端只用差值。
- 計數本身不影響執行（§3.1 第 7 項）。
- 消費端的用法（012 §3.2.1）：觀察點看到 PC = p，記下計數；下一個觀察點計數不變，表示這一步沒有送達，p 那道指令已開始執行；變了，同一個 p 之後會再被觀察到。

### 2.2 批次讀取

```go
// PeekBlock 把線性位址 a 起 len(dst) bytes 讀進 dst，與逐位元組 PeekOK 的結果相同（docs/spec/454 §2.2）。
// ok 為假表示範圍內有頁不存在，該處位元組為 FFh。沒有副作用。len(dst) = 0 時回 true。
func (m *Machine) PeekBlock(a uint32, dst []byte) (ok bool)

// ReadBlock 只做一次 live() 檢查，之後呼叫 PeekBlock。保留的視圖讀到全 0、ok 為假並設 viewMisuse。
func (v StepView) ReadBlock(a uint32, dst []byte) (ok bool)
```

- 語意：逐位元組等同 `PeekOK(a + uint32(i))`（`internal/machine/peek.go:31–38`），`a + i` 以 `uint32` 環繞。`Peek8` 沒有副作用：不動 VGA latch、不觸發讀取監看、走頁表時不填 TLB、不寫 accessed／dirty（已證實：`peek.go:3–11、47–61`）。`PeekBlock` 保持這些性質，也不填效能分支的 `utlb`。
- TLB 有世代符合的舊條目時，`peekLin` 用它（看到 CPU 此刻會看到的，`peek.go:47–49`）；`PeekBlock` 每頁同樣先查 TLB 再走頁表，結果與 `PeekOK` 相同。
- 實作：轉換開啟時，每個 4 KiB 線性頁只做一次上述查詢，再對該頁內的實體範圍套 A20 後整段複製；轉換關閉時，依 `hmaAddr` 與 `peekPhysRaw` 的分段（低記憶體、HMA、`Ext`）整段複製。平面模式的 A0000 視窗照 `Peek8` 現況回 0。
- 呼叫端以 `段:位移` 讀、位移在 64 KiB 內環繞時，自己切成至多兩段（012 §3.2.3）。

### 2.3 完整狀態摘要

```go
// MachineDigest 回機器與 DOS 層的狀態摘要（docs/spec/454 §2.3）。
func (o *Owner) MachineDigest() (FullDigest, error)

type FullDigest struct {
	Machine [32]byte // machine.SaveState 的 gob 經 machine.StateDigest
	DOS     [32]byte // dos.SaveState 的 gob 經 dos.StateDigest
}

// StateDigest 對一段 dos SaveState gob 計算與型別註冊順序、map 寫入順序無關的摘要（internal/dos，新增）。
func StateDigest(r io.Reader) ([32]byte, error)
```

- `Machine`：既有的 `machine.StateDigest`（`internal/machine/state_digest.go:11–13`），涵蓋 `Hi`、`FSeg`、`GSeg`、`PM`（`state.go:166–167、157`）。
- `DOS`：在 `internal/dos` 新增 `StateDigest`，做法照 `machine/state_digest.go`：gob 解碼回 DOS 的存檔結構 → `json.Marshal`（map 依鍵排序）→ SHA-256。不直接雜湊 gob 串流：gob 的型別描述依註冊順序出現，審查 B 實測兩個行程對同一狀態得到不同雜湊。
- DOS 狀態含 `SaveRoot` 一類的主機絕對路徑；比較 A/B 時三組使用同一個 `SaveRoot`（012 §3.9），本摘要不改寫路徑。
- 呼叫限制同 `Digest`：已關閉、回合進行中回錯誤（`digest.go:36–45`）。
- 兩次 SaveState 都不得改變狀態（§3.1 第 11 項）。
- 成本：編碼整份狀態，只在收據的截圖點與停止點呼叫。

已知缺漏：下列執行期狀態不在兩份 SaveState 內（審查 B 核對 `state.go:190、378`、`callback.go:27–36、65–85`），`MachineDigest` 相同不代表它們相同。`nextFrame`、`NextKey`、`NextIRQ0`、`IRQ0Pending`、`Ticks` 已在存檔內，不列。實作者再核對一次，把結果寫進 §5：

| 狀態 | 意義 |
|---|---|
| `keyQueue`、`timedKeys` | 待送的按鍵 |
| `cbQueue`、`cbActive`、`cbSaved`、`cbMade` | 待發與進行中的回呼。保護模式剖面的 `cbQueue` 與 UIR 進 PM 版本 4，`cbActive`、`cbSaved` 恆為初值（459 §5.1）；該剖面只剩 `cbMade` 不在存檔內（由 459 修訂，2026-10-23） |
| `periodic` | 週期遠呼叫的設定與下次步數 |
| `Frames` | 回掃計數 |
| `CPU.FPUCW` | x87 控制字（`internal/cpu/cpu.go:167`） |
| §2.1 的送達計數 | 刻意不進 |

使用端（012 §3.9）同時比 `Digest`、`MachineDigest` 與自己的觀察序列，缺漏項由觀察序列間接涵蓋。

### 2.4 時序、唯讀與觀察器

- 本規格不改 `BeforeStep` 的呼叫時點與次數。
- 新方法只在開著的視圖內有效；保留的視圖呼叫它們回 0 並設 `viewMisuse`。
- `StepView` 以值傳遞（`observer.go:58–61`），新方法照既有的值接收者寫法。消費端若以介面持有視圖，值轉介面每次多一次配置；審查 B 實測值轉介面 84–90 ns、指標 74–82 ns，消費端宜持 `*StepView`（012 §3.2.1）。
- 不開放寫入。

### 2.5 狀態檔與 `BootFromState`

計數不進狀態檔。`PeekBlock`、`ReadBlock`、`MachineDigest` 只讀。狀態檔格式不變。

### 2.6 與效能分支的關係

- 本規格動到 `machine` 的地方：`Machine` 結構尾端一個欄位、三處既有送達分支內的遞增、新的 `PeekBlock`。每步都走的路徑不變（§3.2 的靜態檢查）。
- perf-cpu（工作樹 `~/cht/dosgolem-sc-perf`，目前 `1669fe4`）優化主迴圈。審查 B 核對：該分支的改動不碰本規格會動的檔案。順序：perf-cpu 先提交併回，454 在其上實作。
- 錨點：同一狀態、同一輸入、同一步數的 `Digest` 全欄位與 `MachineDigest`（§2.3 的正規化版），在 454 併入前後必須相同。不用 perf-cpu 自己的「完整 gob 串流」錨點，因為它跨行程不決定。

### 2.7 範圍外

- 位址篩選模式、`RunUntilHits`（§4 決定 3）。
- 32 位元暫存器、`FS`、`GS`、`PC()` 的讀取方法（§4 決定 4）。
- 讀取足跡（§4 決定 5）。
- 例外的重複觀察（§4 決定 7）。
- 依提交點把事件分組成畫格（§4 決定 1）。

## 3. 驗收

### 3.1 單元測試

1. **IRQ0 送達（實模式）**：`.COM` 合成程式在迴圈中呼叫子程式 S；設定 IRQ0 頻率，使至少一次 IRQ0 落在 S 入口的觀察點。斷言：有送達的那一次，下一個觀察點的 `Deliveries` 加 1；同一個 PC 之後再被觀察到一次；依 012 §3.2.1 去重後的 S 命中次數，等於程式實際呼叫 S 的次數（程式自己在記憶體計數）。
2. **IRQ0 送達（V86）**：沿用 `TestV86DeliverIRQ` 的建法（`internal/machine/v86deliver_test.go:234`），IRQ0 在 V86 下經 IDT 遞送時 `Deliveries` 加 1。
3. **回呼與週期遠呼叫**：同第 1 項，改以排入的回呼與 `periodic` 觸發，各一案。
4. **IRQ1（佇列與定時鍵）、IRQ7**：沿用 449 的按鍵注入、`keyhold` 的定時鍵與 452 測試的 SB16 合成程式，各觸發一次落在觀察點的送達。
5. **V86 `INT n` 不計**：V86 合成程式（沿用 435 的 V86 測試建法）在觀察點執行 `CD 15`，三案：gate DPL 3、gate DPL 0（改送 #GP）、IOPL < 3。各案中該觀察點到下一個觀察點 `Deliveries` 不變，同一個 PC 不再被觀察到（以 PC 序列斷言）。
6. **無送達不變**：沒有中斷的程式跑 10 萬步，計數是 0。
7. **計數不影響執行**：同一程式、同一輸入，觀察器讀與不讀計數，停止點的 `Digest` 與 `MachineDigest` 相同。
8. **PeekBlock 等價**（必做）：以下每種情形，`PeekBlock` 與逐位元組 `PeekOK` 的結果與 `ok` 相同：
   - 轉換關閉：低記憶體；跨 `0xFFFFF` 到 `0x100000`；從 `0x10FFE0` 起 32 bytes（跨過 HMA 尾端 `0x10FFEF`）；A20 開與關（關時環繞回 0）；`Ext` 存在與不存在；`0xA0000–0xBFFFF` 照 `Peek8` 現況。
   - 轉換開、分頁關：A20 開與關；`Ext` 存在與不存在；平面模式開啟時讀 A0000 視窗。
   - V86 分頁：沿用 `internal/machine/bench_step_test.go` 的 `BenchmarkStepV86Paged` 一類建法建頁表，範圍跨兩個頁，其中一頁的實體位址不連續；另一個範圍含一個不存在的頁（`ok` 為假，該頁位元組為 `FFh`）；改寫頁表項後不沖 TLB 再讀（兩者都看到舊轉換）。
   - `len(dst) = 0`；`a` 接近 `0xFFFFFFFF` 時 `a + i` 環繞。
   - 白箱：呼叫前後 TLB、`utlb`、accessed／dirty 位元、讀取監看計數不變。
9. **ReadBlock 誤用**：保留的視圖呼叫 `ReadBlock` 得全 0、`ok` 為假；呼叫 `Deliveries` 得 0；`Advance` 回「觀測器在回呼外使用了 StepView」。
10. **完整性**：§2.1 的兩組 `grep` 結果與遞增點一一對應（寫進 §5）。
11. **MachineDigest**：兩台機器只差 `Hi[AX]` 時 `Digest` 相同、`MachineDigest.Machine` 不同；只差 DOS 的一個檔案 handle 狀態時 `MachineDigest.DOS` 不同；兩行程案（見 N13）結果相同。呼叫前後 `Digest` 相同，接著 1000 步與不呼叫者逐步相同。

### 3.2 效能

主門檻是決定性的靜態檢查：

1. `deliveries` 欄位在 `Machine` 結構的最後（以 `reflect` 或 `unsafe.Offsetof` 在測試中斷言）。
2. `(*Machine).Step` 的機器碼在改動前後相同（正規化後）。命令：

   ```sh
   go build -o /tmp/dg ./cmd/probe
   go tool objdump -s '^github.com/wicanr2/dosgolem/internal/machine.\(\*Machine\).Step$' /tmp/dg \
     | sed -E 's/^\s*\S+:[0-9]+\s+0x[0-9a-f]+\s+//; s/0x[0-9a-f]+//g' > step.txt
   ```

   改動前後的 `step.txt` 逐行相同。`Step` 內聯的 `tick` 等若因遞增而不同，比對範圍改為不含送達分支的區塊，並把差異逐行列進 §5。
3. `BenchmarkPeekBlock64000` 三種模式（實模式、線性、V86 分頁）：V86 分頁 ≤ 100 µs，且比逐位元組快 5 倍以上。這一項的差距遠大於雜訊，照常量。

時序附註（不作門檻）：`BenchmarkStepRealMode`、`BenchmarkStepV86`（`bench_step_test.go:44、73`）改動前後配對交錯 10 輪的中位數差、A/A 範圍，連同 `loadavg`、cpuset、Go 版本寫進 §5。審查 B 實測 A/A 範圍 RealMode 約 28%、V86 約 40%，時序量測不足以判定 2% 級的差。若要以時序判定，A/A 範圍超過門檻時判「不確定」並加輪數，不放寬門檻。

### 3.3 負對照（實作子代理逐列執行，結果寫進 §5）

| 編號 | 突變 | 必須失敗 |
|---|---|---|
| N1 | 刪去 `raiseIRQ` 開頭的遞增 | 第 1、2、4 項 |
| N2 | 刪去週期遠呼叫的遞增 | 第 3 項（週期案） |
| N3 | 刪去 `startCallback` 後的遞增 | 第 3 項（回呼案） |
| N4 | 把遞增從 `raiseIRQ` 移到 `machine.go:1701` 的 IRQ0 呼叫端一處（`keyboard.go:128` 等不遞增） | 第 4 項（IRQ1 佇列案） |
| N5 | 同 N4，另在 `keyboard.go:128`、`keyhold.go:234` 加回，只缺 `sb_dma8.go:837` | 第 4 項（IRQ7 案） |
| N6 | 在 `commitV86Delivery` 遞增 | 第 5 項 |
| N8 | 每步都遞增 | 第 6 項 |
| N9 | `PeekBlock` 跨頁時沿用第一頁的轉換 | 第 8 項 V86 分頁 |
| N10 | `PeekBlock` 不套 A20 | 第 8 項 A20 關 |
| N11 | `PeekBlock` 填 `utlb` | 第 8 項白箱 |
| N12 | `ReadBlock` 不做 `live()` 檢查 | 第 9 項 |
| N13 | `dos.StateDigest` 直接雜湊 gob 串流 | 第 11 項兩行程案：測試以 helper process（`os.Args` 帶旗標重跑自身）在子行程先註冊一個無關的 gob 型別、再存同一份 DOS 狀態，父子兩行程的摘要必須相同；突變時不同 |
| N14 | `MachineDigest.DOS` 回固定值 | 第 11 項 |
| N15 | 遞增只在 `raiseIRQ` 的實模式分支（V86 經 IDT 的分支不遞增） | 第 2 項 |

### 3.4 回歸

改動前後 `go test -count=1 ./...` 的套件結果相同（3 個 ebiten 套件在無顯示器容器裡改動前就失敗，照 449 到 452 的記法）。448 到 452 的長測試、strike_commander 規格 001 到 011 的長測試重跑通過。§2.6 的錨點在 454 併入前後相同。`000-index.md` 登記 454。

## 4. 決定

| 編號 | 事項 | 決定 | 理由 |
|---|---|---|---|
| 1 | 畫格分組放通用層 | 不放 | 提交點與畫格起訖是遊戲流程（RE 024 §5），依 006 屬 apps；沒有第二個實例前不抽象（006 §5） |
| 2 | 命中重複的處理 | session 只提供計數，去重規則由消費端定 | 哪些命中要去重，依消費端而定 |
| 3 | 位址篩選模式與 `RunUntilHits` | 不在本規格 | 012 的產品路徑仍走逐步的 `SessionObserver`；審查 B 實測篩選本身每步多 8 ns；既有觀察器改用篩選要各自重驗（另立規格） |
| 4 | 32 位元暫存器與 `FS`、`GS` 讀取 | 不加 | 012 沒有消費者 |
| 5 | 讀取足跡 | 不加 | 既有 `machine.WatchReads`（`machine.go:969–970`）看實體位址，HMA 與快速路徑不經過它（RE 025 §3.1）；012 的輸入範圍已由 RE 025 §5 定完 |
| 6 | `MachineDigest` 的位置 | 放本規格 | 012 §3.9 的 A/B 以它為前提 |
| 7 | 計數的判準 | 只計機器在步驟嘗試開始時把控制權交給非該指令的程式碼（回呼、週期遠呼叫、硬體 IRQ）；例外一律不計 | 例外是指令本身的結果；dosgolem 沒有會讓同一道指令重來的例外遞送（#PF 不經 IDT，V86 的 #GP 由監督程式模擬後跳過）。消費端在可能產生例外的 PC 上觀察時自行處理；#PF 遞送若日後實作，另議 |

## 5. 實作收據

（未實作）

## 附錄 A：審查處置

### A.1 第 1 輪（第 2 版審查，`workplace/m6/review-012-2a.md`、`review-012-2b.md`）

| 編號 | 類別 | 處置 |
|---|---|---|
| B 阻擋 2 | 阻擋 | §2.1 送達計數 |
| B 應改 1、A 應改 7 | 應改 | 篩選模式移除（§4 決定 3） |
| B 應改 6 | 應改 | `MachineDigest` 改為機器加 DOS 兩份摘要，列已知缺漏 |
| B 應改 7、A 應改 10 | 應改 | `machine.PeekBlock` |
| A 應改 8 | 應改 | V86 分頁單元測試必做 |
| A 應改 9 | 應改 | 行號更正 |
| B 建議 | 採用 | 沒有消費者的方法不加；引用帶檔名 |
| A 應改 15 | 應改 | 改稱 RE 025 |

### A.2 第 2 輪（第 3 版審查，`workplace/m6/review-012-3a.md`、`review-012-3b.md`）

| 編號 | 類別 | 處置 |
|---|---|---|
| A、B 阻擋 B1（`CD 15` 使 `Incomplete` 加 1） | 阻擋 | 刪除 `Incomplete`；§3.1 第 5 項 |
| A 建議、B 應改 13 | 應改 | 補 `keyhold.go:234`；`le_dma_clock.go:101` 不在送達流程內；存取器名稱 |
| B 應改 9、10、11 | 應改 | §3.1 第 8 項邊界；缺漏表加 `FPUCW` |
| B 應改 3 | 應改 | §2.4 |
| B 已核、建議 | 採用 | `PeekBlock` 不填 `utlb`；分期順序；V86 `CD 15` 案例 |

### A.3 第 3 輪（第 4 版審查，`workplace/m6/review-012-5a.md`、`review-012-5b.md`）

| 編號 | 類別 | 處置 |
|---|---|---|
| B 應改 5 | 應改 | §2.4 消費端持 `*StepView` |
| B 應改 7 | 應改 | DOS 摘要正規化；同 `SaveRoot` |
| B 應改 9 | 應改 | 負對照補 `startCallback`、`keyTick`、`sb8IRQ7` |
| B 應改 10 | 應改 | 「轉換開、分頁關」案例 |
| B 建議 | 採用 | 遞增放 `raiseIRQ`；DPL 3／0 測試；回歸加錨點 |

### A.4 第 4 輪（第 5 版審查，`workplace/m6/review-012-6a.md`、`review-012-6b.md`）與主代理裁決

| 編號 | 類別 | 處置 |
|---|---|---|
| A 阻擋 1、B 阻擋 1（fault 類計入無法實作） | 阻擋 | 主代理裁決：只計回呼、週期遠呼叫、`raiseIRQ` 三種既有送達點；所有例外不計（§2.1、§4 決定 7）；刪去 #PF 測試與對應負對照 |
| A 應改 7 | 應改 | §2.1 寫明不在 `commitV86Delivery` 遞增；§3.1 第 5 項加 IOPL < 3 案；N6 |
| A 建議：`ops.go:99–104` 與 `doInt` 分開寫 | 採用 | §2.1 |
| B 應改 1 | 應改 | §2.1 與 `eventThisStep` 六處對應，§3.1 第 10 項完整性驗收 |
| B 應改 2 | 應改 | §3.1 第 2 項 V86 IRQ0；N15 |
| B 應改 3 | 應改 | N4、N5 寫成具體修改；N13 以 helper process 改變型別註冊順序 |
| B 應改 4 | 應改 | §2.3 在 `internal/dos` 新增 `StateDigest`（解碼 → `json.Marshal` → SHA-256） |
| B 應改 5 | 應改 | 缺漏表刪 `nextFrame`，加 `cbActive`、`cbSaved`、`cbMade` |
| B 應改 6 | 應改 | 主代理裁決：§3.2 主門檻改為靜態檢查（欄位在結構尾端、`(*Machine).Step` 正規化機器碼比對）；時序降為附註，A/A 超過門檻判不確定 |
| B 應改 7 | 應改 | §2.6 錨點改 `Digest` 加正規化 `MachineDigest`；perf-cpu `1669fe4` |
| N7（#PF 不遞增） | 刪除 | 隨 #PF 測試一起刪除；其餘負對照編號不重排 |
| B 建議：HMA 邊界措辭 | 採用 | §3.1 第 8 項改為從 `0x10FFE0` 起跨過 `0x10FFEF` |
| B 建議：PeekBlock 測試補項 | 採用 | TLB 未沖、平面模式在轉換開時、`len = 0`、`uint32` 環繞 |
