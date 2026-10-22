# 458：session 觀察器的唯讀實體 VGA 讀取與 VideoWrite 執行脈絡欄位

狀態：**READY**（第 3 版；第 1 版審查 `workplace/flicker/review458v1-A.md`、`review458v1-B.md`，第 2 版複審兩位審查者同意轉 READY，第 3 版處理複審的應改與建議；2026-10-22 轉 READY，紀錄見附錄 B），未實作
日期：2026-10-22
前置：[`006-layering`](006-layering.md)（通用層判準）、[`013-vga-planar`](013-vga-planar.md)、[`201-vga-unchained-256`](201-vga-unchained-256.md)、[`187-frame-clock`](187-frame-clock.md)、[`238-session-step-observer-draft`](238-session-step-observer-draft.md)（`StepView` 只在回呼內有效）、[`432-linear-memory-and-paging`](432-linear-memory-and-paging.md) §3.1、§6、[`434-legacy-core-segmented-mode`](434-legacy-core-segmented-mode.md)、[`435-v86-hooks-and-idt-delivery`](435-v86-hooks-and-idt-delivery.md)、[`442-segmented-return-to-real-mode`](442-segmented-return-to-real-mode.md)、[`443-segmented-32bit-code-and-stack`](443-segmented-32bit-code-and-stack.md) §2.3、§2.9、[`454-session-view-block-read-and-digest`](454-session-view-block-read-and-digest.md)、[`455-fetch-page-cache`](455-fetch-page-cache.md)、[`456-sc-observer-view-alloc`](456-sc-observer-view-alloc.md)
使用端：strike_commander 規格 014（目前在 `strike_commander/workplace/flicker/spec014-draft.md` 第 3 版，尚未進 `docs/spec/`）§3.4.1、§3.4.2、§3.5.1，裁決 D12、D13

程式位置以 dosgolem-sc `strike-commander` 分支 `2436db8` 為準。推論等級：已證實、強推論、假說、未知。本規格的事實都來自程式與既有規格，不需要遊戲動態證據。

用語：

| 用語 | 意義 |
|---|---|
| 觀察點 | `BeforeStep` 被呼叫的時點：中斷點與述詞檢查之後、該次 `Machine.Step()` 之前（`internal/machine/probe.go:197–212`；454 用語） |
| 實體 VGA | 顯示硬體掃出的內容：鏈結模式是實體 `Mem[A0000…]`，非鏈結與平面模式是 `VGA.Planes`。不經線性層與頁表 |
| 畫面色號 | `Machine.Indexed()` 的回傳內容（本體自 `internal/machine/machine.go:1463`），也是 `StepObserver.Frame` 收到的 `indexed`（`session/observer.go:172`） |
| 寫入脈絡 | 一筆 `VideoWrite` 發生當下 CPU 的執行模式：實模式、V86、16 位元保護模式、32 位元碼段 |
| 金樣雜湊 | 改動前（`2436db8`）以現行 `Indexed()` 對固定種子內容算出的 SHA-256，寫成測試常數（§5 動工前步驟） |

---

## 1. 為什麼要做

規格 014（覆蓋模式防閃爍）要在遊戲完成畫面的那一步讀整張畫面，並判斷 VGA 寫入是誰寫的。現況有三個缺口：

| 缺口 | 現況 | 等級 |
|---|---|---|
| 觀察點讀不到實體 VGA | `StepView.Read8` 走 `Peek8`（`session/observer.go:127–133`）；轉換開啟時 `Peek8` 經 `peekLin` 走頁表（`internal/machine/peek.go:12–16、50–61`）。JEMM 每幾千步改 A000 的線性對映（strike_commander RE 015 §7），經頁表讀到的不一定是螢幕內容。平面模式的 A0000 視窗 `Peek8` 一律回 0（`physical.go:42–44`） | 已證實（程式；RE 015 實測） |
| 觀察點沒有模式查詢 | `StepView` 只有暫存器、`Palette()`、`Read8`、`Read16`（`session/observer.go:94–139`）。014 §3.4.1 要「取像前確認 mode 13h 鏈結」，鏈結與非鏈結 13h 的畫面都是 64000 bytes，光看長度分不出來 | 已證實（程式） |
| VideoWrite 沒有執行模式 | `VideoWrite` 只有 `Step`、`CS`、`IP`、`Offset`、`Value`、`WriteMode`（`machine.go:719–725`），由 `writePhysRaw` 以 `CPU.OpAddr()` 填（`physical.go:62–65`）。同一個 `CS` 值在實模式、V86 是段落值，在分段模式是選擇子，呼叫端分不出 JEMM 保護模式的搬運與 V86 的遊戲碼 | 已證實（程式） |

### 1.1 不加 EIP 與 CodeBase（主代理裁決）

014 §2 寫「`CS`、`IP` 是 `uint16`，32 位元碼段的 EIP 會截斷」。這句在現行模型不成立：443 §2.3 把 D=1 碼段的 EIP 高 16 位元釘為 0 的不變式，違反即失敗關閉。`IP` 因此就是完整的 EIP。兩位審查者各自核實了守點（已證實）：

| 守點 | 位置 |
|---|---|
| 取指令：D=1 時在 `IP=FFFFh` 取 byte 失敗 | `internal/cpu/segmented.go:133–136` |
| 相對轉移與計數迴圈 | `internal/cpu/ops386.go:165–174`、`ops0f.go:208–216` |
| 遠轉移與遠返回的目標偏移高半 | `ops386.go:497–503`（`farOffOK`，延伸到 `:573`） |
| D=1 下的近程 `E8`、`E9`、`C2`、`C3`、`FF /2`、`FF /4`（運算元 32 位元）不支援，落 `execute32` 的 `default` 報錯 | `ops386.go:61–62`、`:471–472`、`:490` |
| V86 進出、例外、任務返回、IDT 遞送的 EIP | `v86.go:39`、`v86event.go:93`、`taskreturn.go:110`、`v86deliver.go:149–154`（閘目標碼段 D=1 拒絕） |

測試涵蓋範圍：`TestSeg443EIPInvariant`（`internal/machine/seg443_test.go:575`）只涵蓋取指令與相對轉移；遠轉移、近程轉移、V86 與任務返回的守點由程式核對，沒有專門測試（已證實：審查甲）。本規格不補這些測試，它們屬 443 的範圍。

另一個欄位 `CodeBase`（碼段線性基底）可讓呼叫端跨執行歸屬 JEMM 的 `98h` 段（基底每輪不同，`443-segmented-32bit-code-and-stack.md:86`：`X+7780h`），但 014 §3.4.2 的歸屬規則只需要 V86 位元，目前沒有消費者，不加（同 454 §4 決定 4 的原則）。

### 1.2 分層

依 006 §2 的判準（換一支 binary 仍成立嗎），「讀顯示硬體目前的畫面」與「一筆顯示記憶體寫入當下的 CPU 模式」對任何程式都成立，放 `internal/machine` 與 `session`。哪些常式算完成點、哪些寫入算歸屬，是《Strike Commander》的知識，留在 `apps/strikecommander`（014 §3.10）。

## 2. 規則

### 2.1 實體 VGA 讀取（machine）

```go
// IndexedKind 是畫面色號的來源（docs/spec/458 §2.1）。
type IndexedKind uint8

const (
	IndexedLinear    IndexedKind = iota // 實體 Mem[A0000…]，320×200；mode 13h 鏈結走這裡
	IndexedUnchained                    // 非鏈結 256 色（Mode X），四個平面組成 320×200
	IndexedPlanar                       // 16 色平面模式，VideoSize() 的 W×H，色號 0–15
)

// IndexedLayout 描述目前畫面色號的來源與尺寸。值型別。
type IndexedLayout struct {
	Kind IndexedKind
	Mode uint8 // BDA 0040:0049 的視訊模式（VideoMode()）
	W, H int
}

// IndexedLayout 回目前畫面色號的來源與尺寸，與 Indexed() 的分支條件相同。沒有副作用。
func (m *Machine) IndexedLayout() IndexedLayout

// IndexedInto 把目前畫面色號寫進 dst[:W*H]。len(dst) < W*H 時不寫入任何位元組、回 ok=false；
// 回傳的 IndexedLayout 不論 ok 都有效。沒有副作用，不配置。
func (m *Machine) IndexedInto(dst []byte) (l IndexedLayout, ok bool)
```

1. **同一本體**。`Indexed()` 改寫為「`IndexedLayout()` 取尺寸 → `make(W*H)` → `IndexedInto`」，並斷言 `ok`，不成立就 panic（不會發生，留作防線）。分支條件照現行順序：`Unchained()`（`vga.go:762`）→ `planarOn` → 其餘線性複製（`machine.go:1464–1474`）。`IndexedLayout` 用同一組條件決定 `Kind`，不另寫判斷。
2. **尺寸**（照現行程式，已證實）：

   | `Kind` | W×H | 出處 |
   |---|---|---|
   | `IndexedLinear` | 恆為 `VideoWidth×VideoHigh` = 320×200，不看模式編號 | `machine.go:1471–1473` |
   | `IndexedUnchained` | 恆為 320×200 | `machine.go:1467` |
   | `IndexedPlanar` | `VideoSize()`（`vga.go:570–578`；CRTC 設過時取 `VGA.Size()`，`vga.go:420–432`），例 640×480 = 307200 | `vga.go:649–652` |

   文字模式（mode 03h）與「模式編號是平面模式但 `planarOn` 為假」都走 `IndexedLinear`，W×H 為 320×200，內容是 `Mem[A0000…]` 的原樣複製、不是色號。呼叫端以 `Kind == IndexedLinear && Mode == 0x13` 判 mode 13h 鏈結（014 §3.4.1）。本規格不改這個現況。API 不寫死 64000，呼叫端以 `IndexedLayout()` 配緩衝一次、之後重用。
3. **不配置的分支**。`VGA.Pixels`（`vga.go:472–474`，經 `pixelsAt`，`vga.go:486` 起）與 `VGA.Unchained256`（`vga.go:739–758`）各拆出寫入呼叫端緩衝的形式（名稱由實作者定，例：`pixelsAtInto(dst, …)`、`unchained256Into(dst, …)`），原函式改為 `make` 後呼叫它。顯示起點、列距、分割線的處理一行不改。
4. **全覆寫**（不變式）：`ok` 為真時，`dst[:W*H]` 每個位元組都被寫入，不依賴 `dst` 原內容。`pixelsAt` 每列只寫 `w/8` 個位元組乘 8 個像素（`vga.go:488、497`），`w` 不是 8 的倍數時尾端會留空；實作要確認三種分支都逐點寫滿，否則補寫（§3.1 第 3 項以重用緩衝驗）。
5. **讀的東西**：`m.Mem[A0000:A0000+64000]`、`VGA.Planes`、`VGA.crtc`（`DisplayStart`、`Pitch`、`LineCompare`、`Size`）、BDA 的模式位元組（`bda.go:107`）。不經 `Read8`、`readLin`、`peekLin`、`readPhysRaw`；不載 VGA latch（對照會載入的 `physical.go:24–26`）；不觸發讀取監看。
6. **零副作用**（不變式，§3.1 第 5 項白箱測試）：不改 `Steps`、`Frames`、TLB 與世代（`tlbGen`）、`utlbR`／`utlbW`（`machine.go:711`）、455 的取指令頁快取、頁表項的 accessed／dirty、VGA latch、讀取監看紀錄、454 的送達計數（454 實作後）。

### 2.2 觀察點的轉接（session）

```go
// IndexedLayout 回 Machine.IndexedLayout()；保留的視圖回零值並設 viewMisuse。
func (v StepView) IndexedLayout() machine.IndexedLayout

// IndexedInto 只做一次 live() 檢查，之後呼叫 Machine.IndexedInto。
// 保留的視圖不寫 dst、回零值與 ok=false 並設 viewMisuse。
func (v StepView) IndexedInto(dst []byte) (machine.IndexedLayout, bool)
```

- 寫法照既有的值接收者（`session/observer.go:63–72` 的 `live()`）與 454 §2.2 的 `ReadBlock`。消費端以 `*StepView` 呼叫，不因介面轉換配置（456）。
- 只在開著的視圖內有效。`Frame` 與 `VideoWrite` 回呼內沒有開著的視圖（`observer.go:230–235` 只在 `BeforeStep` 前後開關），在那裡呼叫算誤用，下一個觀察點回 `ObserverFault`（238 的既有行為）。
- 不開放寫入。`dst` 是呼叫端的記憶體，不指向機器內部。

### 2.3 與 `Frame` 同時點

`Frame` 在 `runObserved` 裝的 `SetOnFrame` 回呼內收到 `o.machine.Indexed()`（`session/observer.go:163–173`）。session 只由 `observer.go:256` 的 `RunUntilObserved` 驅動。某一步有 `Frame` 時，該步觀察點與 `onFrame` 之間沒有實體 VGA 寫入：

| 步驟 | 位置 | 會不會寫實體 VGA | 等級 |
|---|---|---|---|
| 觀察點 `observe(m)` | `probe.go:207–212`，在 `m.Step()` 之前 | 只讀 | 已證實（程式） |
| 分頁錯誤紀錄搬移 | `machine.go:1486–1489` | 否 | 已證實（程式） |
| SB16 8 位元 DMA 輪詢 | `machine.go:1491–1499`；`sb_dma8.go`、`dma8237.go` 沒有 `Write8`、`writePhysRaw`、`Mem[…] =`（容器內 grep 為空）；DMA 來源落在 `A0000h–BFFFFh` 時失敗關閉（`sb_dma8.go:216`） | 否 | 已證實（程式） |
| 輪詢經 `sb8Log` 呼叫 `SBDMA` 觀察回呼 | 452 §2.3；回呼內沒有開著的視圖、不得讀寫記憶體 | 否 | 已證實（程式與 452） |
| `tick` 的回呼與週期遠呼叫 | `machine.go:1648–1661`。觸發時 `return`，這一步**不呼叫** `onFrame`，回掃延到之後的步驟 | 只推堆疊 | 已證實（程式） |
| 回掃與 `onFrame` | `machine.go:1668–1673` | 否 | 已證實（程式） |
| `Steps++` | 只在 `machine.go:1541`，在 `tick` 之後 | 不適用 | 已證實（程式） |

結論：某一步有 `Frame` 時，該步觀察點的機器畫面狀態與 `onFrame` 當下相同，兩者看到同一個 `Steps` 值（已證實）。內容正確性（讀的是實體 VGA、三種來源解碼正確）不由這個順序證明，由 §3.1 第 1 到 4 項的獨立期望值驗；§3.1 第 6 項只驗時點。

`IndexedInto` 與 454 `ReadBlock` 分工不同：014 的 BufCU（畫外緩衝）在常規記憶體，要用客體線性位址經頁表讀，用 454；螢幕內容用本規格，繞過頁表。

### 2.4 VideoWrite 的寫入脈絡欄位

```go
// VideoWriteCtx 是一筆 A000 寫入當下的 CPU 執行模式（docs/spec/458 §2.4）。位元集合。
type VideoWriteCtx uint8

const (
	VWV86       VideoWriteCtx = 1 << 0 // EFLAGS.VM=1（m.v86，linear.go:38）
	VWSegmented VideoWriteCtx = 1 << 1 // 既有核心在分段模式（434）
	VWCode32    VideoWriteCtx = 1 << 2 // 分段模式且 CS 的 D=1（CPU.Code32()，segcache.go:33）
)

func (c VideoWriteCtx) V86() bool       { return c&VWV86 != 0 }
func (c VideoWriteCtx) Segmented() bool { return c&VWSegmented != 0 }
func (c VideoWriteCtx) Code32() bool    { return c&VWCode32 != 0 }

type VideoWrite struct {
	Step      uint64
	CS, IP    uint16
	Offset    uint32
	Value     uint8
	WriteMode uint8
	Ctx       VideoWriteCtx // 458 新增
}
```

1. **既有欄位與語意不變**：`CS`、`IP` 仍是 `CPU.OpAddr()`（`internal/cpu/cpu.go:511`，`opCS`、`opIP` 在 `CPU.Step` 開頭設定，`ops.go:26`）。`Offset` 仍是**實體**位址減 `A0000h`（`physical.go:62–64`）。呼叫點與次數不變：只在 `writePhysRaw` 的 `[A0000, B0000)` 分支、`onVideoWrite != nil` 時。
2. **Offset 是實體的後果**（寫明，回饋 014）：分頁把線性 `A0000` 對映到別處時，經線性 `A0000` 的寫入不產生 `VideoWrite`；別段線性位址對映到實體 `A0000` 時，經它的寫入會產生 `VideoWrite`，`Offset` 是實體位移。`VideoWrite` 與 `IndexedInto` 因此看同一塊記憶體（§3.1 第 4 項驗）。
3. **欄位來源**：由一支小函式組裝（例：`func (m *Machine) videoWriteCtx() VideoWriteCtx`），`writePhysRaw` 與 `physical_test.go` 的 `refWrite8` 都呼叫它。來源全部是既有欄位或方法，不新增狀態：

   | 位元 | 來源 | 成本 |
   |---|---|---|
   | `VWV86` | `m.v86`（衍生欄位 `xlat && VM`，`linear.go:33–43`）。VM 只能在 PE=1 時設定、與分段模式互斥（`linear.go:164–168`） | 一次布林讀取 |
   | `VWSegmented` | `m.CPU.Segmented()`（`segcache.go:26`） | 一次布林讀取 |
   | `VWCode32` | `m.CPU.Code32()`（`segcache.go:33`：`segmented && cache[CS].Attr&attrDB != 0`） | 一次位元測試 |

4. **判讀表**：

   | 情境 | `Ctx` | `CS` 的意義 | `IP` | 例 |
   |---|---|---|---|---|
   | 實模式 | 0 | 段落值 | 16 位元位移 | JEMM 常駐前的程式（例：`SCCD.EXE` 載入 JEMM 之前）、不用 JEMM 的其他遊戲 |
   | V86 | `VWV86` | 段落值（實模式樣式） | 16 位元位移 | JEMM 常駐後的 `MENU.EXE`、`OPTTEST.EXE`、`STRIKE.EXE` 遊戲碼（強推論：RE 015 §7 在 OPTTEST 期間觀察到 JEMM 改頁表，分頁開著而遊戲碼以實模式樣式執行即為 V86） |
   | 16 位元保護模式 | `VWSegmented` | 選擇子 | 16 位元位移 | JEMM ring 0（選擇子 `18h`） |
   | 32 位元碼段 | `VWSegmented｜VWCode32` | 選擇子 | 完整 EIP（§1.1） | JEMM 選擇子 `98h` 的 `CU`／`XC` 複製常式 |
   | 442 殘留快取實模式（PE=0、分段模式） | `VWSegmented` | 最後一次載入的值；基底取自快取 | 16 位元位移 | 離開保護模式的過渡 |

   「分段模式下 `Seg[CS]` 存選擇子」已證實（審查乙核對 `internal/cpu/segmented.go:154–180、267`）。442 列不會有 `VWCode32`：分段模式下清 PE 時 CS 為 D=1 會被 `SetCR0` 拒絕（`linear.go:127–129`，443 §2.9），實模式載入清 CS 的 D/B（`segmented.go:184` 的 `loadSegReal`，規則見同檔 `:176–183`）；離開分段模式時 `leaveSegmented` 清空所有段快取（`handoff.go:105–112`）。
5. **時點**：欄位反映寫入**當下**的 CPU 狀態。指令內改變模式的情形（V86 經 IDT 遞送時清 VM，`v86deliver.go:245`；`iretd` 回 V86）只寫堆疊，堆疊不在 A000，實務上不影響（強推論）。非 CPU 指令來源的 A000 寫入沿用現況，`CS`、`IP`、`Ctx` 都是發出該服務的那道指令的脈絡：`WriteBytes`（`machine.go:1449–1456`，例：DOS 讀檔進 A000）、BIOS 寫像素（`internal/dos/bios.go:785` 經 `d.M.Write8`）。這是現況，不是回歸。
6. **大小與配置**：現行結構 8 + 2 + 2 + 4 + 1 + 1 = 18 bytes，對齊到 24；加 `Ctx` 一個 byte 仍是 24。以值傳給 `func(VideoWrite)`，不裝箱、不配置。以 `unsafe.Sizeof(VideoWrite{}) == 24` 釘住。
7. **既有字面與消費端**（審查甲已在容器內核對）：建 `VideoWrite` 的字面都用具名欄位（`physical.go:64`、`physical_test.go:50`、`apps/buckrogers` 與 `cmd/buckrogers-text-receipt` 的測試），加欄位不破壞編譯，零值 `Ctx` 等於「實模式」，與這些測試的情境相符；沒有以 `==` 比較整個結構的程式。`onVideoWrite` 的呼叫點只有 `physical.go:64` 與 `physical_test.go:50`。逐欄讀取的消費端不受影響：`cmd/buckrogers-text-receipt/body_icon_video_trace.go:329–339` 逐欄寫雜湊，不含新欄位；`cmd/strikecommander-census/main.go:286` 只讀 `Step`、`Offset`；`apps/strikecommander/receipt.go:94` 是空的。
8. **差分測試接點**：`physical_test.go:34–88` 的 `refWrite8` 複製了 `VideoWrite` 的建法，`:176` 以 `%+v` 比對回呼序列。`refWrite8` 改呼叫同一支 `videoWriteCtx()`。`newDiffRig` 不開 profile，新欄位兩邊都是 0，這個差分測試不驗新欄位；它的參考實作在轉換開啟時不可比（`refWrite8` 不走 `writeLin`），所以不擴成新欄位的驗證，新欄位由 §3.1 第 10、11 項驗。
9. **content-safe**：`Value` 的既有限制（`machine.go:717–718`）不變。`Ctx` 不含畫面內容。

### 2.5 狀態檔、摘要與時序

- 不改狀態檔格式；`Ctx` 與 `IndexedLayout` 不進 `SaveState`、`Snapshot`、`session.Digest`（`session/digest.go:50–74`）、`machine.StateDigest`（`state_digest.go:11–13`）、454 的 `MachineDigest`。
- 不改 `BeforeStep`、`Frame`、`VideoWrite` 回呼的時點與次數。
- 不改 `(*Machine).Step`、`tick`、`Read8`、`Write8` 的本體。`writePhysRaw` 只動 `onVideoWrite != nil` 的分支，這個分支只在有 A000 寫入時走。
- 不動 `internal/cpu`（只呼叫既有的 `Segmented()`、`Code32()`、`OpAddr()`）。

### 2.6 範圍外

- `Frame` 改用呼叫端緩衝：`Frame` 的 `indexed` 承諾是「複本，改它不動機器」，現有觀察器可能保留這個切片，不改。
- 翻頁中「顯示中那一頁」以外的頁：`Indexed()` 怎麼解讀，本規格就怎麼回。
- LE／DOS4GW 平坦核心（`LEMachine`）的影片寫入：`le_machine.go:169` 的 `Write8` 不經 `Machine.writePhysRaw`，不在本規格。
- 平面模式設定的清除（`resetMode`）不經 `VideoWrite`，現況由 `ModeObserver` 補（`session/observer.go:33–39`）。
- BIOS 在平面模式寫像素走 `PlanarPutPixel`（`internal/dos/bios.go:782`，`vga.go:629–638`），直接改 `Planes`，不經 `writePhysRaw`，也不產生 `VideoWrite`，與 `resetMode` 同類。`IndexedInto` 讀得到它的結果。
- 讀色盤：既有 `StepView.Palette()`（`observer.go:119–125`）已是複本，不改。

## 3. 驗收

期望值一律獨立於被測本體：不拿 `Indexed()` 或 `Frame` 的輸出當 `IndexedInto` 的期望值（兩者共用本體，§2.1 第 1 條）。

### 3.1 單元測試

1. **線性**：鏈結 mode 13h，以固定種子填 `Mem[A0000:A0000+64000]`。期望值是同一段 `m.Mem` 的直接切片複本。斷言 `IndexedInto` 前 64000 bytes 相等、`Kind == IndexedLinear`、`Mode == 0x13`、W×H = 320×200。
2. **非鏈結**：mode 13h 後清鏈結位元，設顯示起點、列距與分割線為非零（例：起點 `0x0123`、列距 `0x28`、分割線落在第 100 列）。兩個期望值，兩者都要相符：
   - 測試內手算：依 201 的公式，像素 (x, y) 為 `Planes[x&3][(row + x>>2) & (PlaneSize−1)]`，`row = 起點 + y×列距`，`y > 分割列` 時 `row = (y − 分割列 − 1) × 列距`；測試自寫迴圈，不呼叫 `Unchained256`。單位照現行程式寫死（`vga.go:741–747`）：起點 = `DisplayStart()`；列距 = `crtc[13h] × 2` bytes（為 0 時退回 `320/4`）；分割列 = `LineCompare() / (crtc[09h]&1Fh + 1)`，以掃描線換算成像素列。
   - 金樣雜湊：同一組內容在改動前（`2436db8`）以 `Indexed()` 算出的 SHA-256。
3. **平面**：mode 12h（640×480），`DisplayStart` 非零，加一個 mode 0Dh 案。期望值同上兩種：測試內手算（像素 (x, y) 的色號 = 四個平面在位移 `(row + x/8) & (PlaneSize−1)` 的第 `7 − x%8` 位元依序組成 bit0 到 bit3）與改動前的金樣雜湊。單位照現行程式寫死（`vga.go:473、486–497`）：`row = 起點 + y×列距`，`y > 分割列` 時 `row = (y − 分割列 − 1) × 列距`；起點 = `DisplayStart()`；列距 = `Pitch(w)`（`vga.go:444–449`）；分割列直接用 `LineCompare()`，**不**除每列掃描數（與非鏈結不同，照現況）。
   另驗**緩衝重用**（三種來源都做）：`dst` 先填哨兵 `0xA5`，呼叫一次；換內容再呼叫一次；兩次的 `dst[:W*H]` 都等於各自的期望值，沒有殘留哨兵或上一次的內容；`len(dst) > W*H` 時尾端仍是哨兵。
4. **非恆等分頁（machine 層）**：以 `RunUntilObserved`（`probe.go:197`）與 `SetOnFrame` 在 machine 層建，不經 session（審查甲、乙：分頁輔助在 machine 套件，`bench_step_test.go:113` 的 `newPagedProg`、`v86deliver_test.go:385` 的 `TestV86DeliverPaging`）。頁表把線性 `A0000–AFFFF` 對映到 `Ext` 內的實體頁 E，另一段線性位址 L 對映到實體 `A0000`。V86 程式在線性 `A0000` 寫圖樣 P1、在 L 寫圖樣 P2；P1 ≠ P2，且 E 的初值、實體 `A0000` 的初值、P1、P2 兩兩不同。斷言：
   - 每次 `Frame` 那一步的觀察點，`IndexedInto` 等於實體 `Mem[A0000…]` 的直接複本，含 P2、不含 P1；
   - 經線性 `A0000` 的寫入不產生 `VideoWrite`；經 L 的寫入產生 `VideoWrite`，`Offset` 是實體位移，`Ctx == VWV86`（§2.4 第 2 條）。
5. **零副作用（白箱）**：分頁開、TLB 有世代符合的條目、`utlb` 有條目、取指令頁快取已裝（455）、平面模式 latch 為非零值。前置另要兩條，否則 N8 會假通過：
   - 線性 `A0000` 所在的頁**不在** TLB 與 `utlbR` 內，且該頁 PTE 的 accessed 位元為 0。經 `readLin` 讀它時 `xlatRead`（`paging.go` 約 160 行）會填 TLB、`utlb` 並設 accessed，突變因此留下痕跡；
   - 掛一個涵蓋實體 `A0000–AFFFF` 的讀取監看（`physical.go:27–28`），經 `readPhysRaw` 的讀取會留下監看紀錄。

   呼叫 `IndexedInto` 前後，`tlbGen`、TLB 內容、`utlbR`／`utlbW`、取指令頁快取狀態、頁表項的 accessed／dirty、`VGA.latch`、讀取監看紀錄、`Steps`、`Frames`、`machine.StateDigest` 全部相同。另在 machine 層建兩台相同的機器，一台每步呼叫 `IndexedInto`、一台不呼叫，跑 1000 步，每步比對 `CPU` 暫存器、`IP`、`Flags`，結束時比 `machine.StateDigest`（`session.Digest` 在回合中不可取，`digest.go:43–45`）。這組整機比對跑兩輪：鏈結 mode 13h 一輪；平面模式一輪，程式在迴圈中讀寫 A0000 視窗（讀取會載 latch，`physical.go:24–26`），每步另比 `VGA.latch`。454 已實作時另斷言 `Deliveries()` 不變。
6. **與 `Frame` 同時點（session）**：合成 `.COM`（照 454 §3.1 第 1 項的建法），mode 13h 迴圈寫像素，`VGAFrameEvery` 設小（例 997 步），跑至少 50 次回掃。觀察器每個觀察點以 `IndexedInto` 存進固定緩衝；`Frame` 內比對收到的 `indexed` 與最近一次的緩衝，必須相同。另一案：回掃那一步正好有排入的回呼（`startCallback` 觸發），回掃延後，比對仍相同。這一項只驗時點（§2.3），不驗內容正確性。
7. **布局**：mode 13h 鏈結、非鏈結、mode 12h、mode 03h、「模式編號 12h 但 `planarOn` 為假」各一案，`Kind`、`Mode`、`W`、`H` 符合 §2.1 第 2 條（後兩案為 `IndexedLinear`、320×200）。
8. **緩衝不足**：`len(dst) = W*H − 1` 時 `ok=false`，`dst` 全部仍是哨兵；回傳的 `IndexedLayout` 與 `IndexedLayout()` 相同。
9. **誤用與不配置**：保留的視圖呼叫 `IndexedInto`、`IndexedLayout`：不寫 `dst`、回零值與 `ok=false`，`Advance` 回「觀測器在回呼外使用了 StepView」；`Frame` 回呼內呼叫同樣判誤用。`testing.AllocsPerRun` 斷言 `Machine.IndexedInto`（三種來源）與 `StepView.IndexedInto`（以 `*StepView` 呼叫）每次 0 配置。
10. **`Ctx`：實模式與 V86**：沿用 435 的 V86 測試建法（`v86deliver_test.go:149` 起），V86 程式 `mov [es:di], al` 寫 A000，`Ctx == VWV86`；同一程式在實模式跑，`Ctx == 0`。`CS`、`IP` 兩案都等於該道指令起點。
11. **`Ctx`：分段模式**：沿用 `seg443_test.go` 的 JEMM 形狀建法（`TestSeg443JemmCU`、`TestSeg443JemmCopy`，`:350`、`:412`），16 位元碼段（選擇子 `18h` 一類）與 D=1 碼段（`98h` 一類）各寫一次 A000：前者 `Ctx == VWSegmented`，後者 `Ctx == VWSegmented|VWCode32`；兩者 `CS` 等於當時的選擇子值，`IP` 等於指令起點。另以 442 殘留快取實模式寫一次，`Ctx == VWSegmented`。
12. **`Ctx` 組裝白箱**：直接呼叫 `videoWriteCtx()`，以 `CPU.SetSegmented(false)` 加 `CPU.SetSegCache(cpu.CS, …D=1…)`（`segcache.go:30、39`）造出違反 434 不變式的狀態，斷言結果不含 `VWCode32`（`Code32()` 先看 `segmented`）。
13. **大小與寫入路徑配置**：`unsafe.Sizeof(VideoWrite{}) == 24`；`ObserveVideoWrites` 裝空回呼，對 A0000 寫一個 byte，`testing.AllocsPerRun` 為 0。`physical_test.go` 的差分測試照常全過。

### 3.2 回歸與不變

1. 改動前後 `go test -count=1 ./...` 套件結果相同（無顯示器容器內 3 個 ebiten 套件改動前就失敗，照 449 到 452 的記法）。
2. SingleStepTests：以掛載的語料（`internal/cpu/singlestep_test.go:20` 的 `../../testdata/8088`，掛載方式同 433 §8.2）實際執行，PASS 數與改動前相同、0 skip，`knownGaps`（`singlestep_test.go:212`）不變；`git diff internal/cpu` 為空。
3. 長測試收據逐位元組相同：448 到 452 的 session 長測試；strike_commander 規格 001 到 012 的長測試（`Present` 關）；Buck Rogers 的 `cmd/buckrogers-text-receipt` 收據。理由：`Indexed()` 改寫只換配置位置，`Frame` 與 `Digest` 的 `IndexedSHA256`（`digest.go:67`）內容不變；既有消費端不讀 `Ctx`（§2.4 第 7 條）。
4. 同狀態錨點：同一狀態檔、同一輸入、同步數，`session.Digest` 全欄位與 `machine.StateDigest` 改動前後相同；454 實作後另加 `MachineDigest`。

### 3.3 效能

主門檻是決定性的檢查：

1. 配置：§3.1 第 9、13 項為 0。
2. `(*Machine).Step` 的機器碼改動前後相同，命令與正規化照 454 §3.2 第 2 項。本規格不碰 `Step`，比對應該逐行相同；不同就列差異並查明。
3. `writePhysRaw` 的機器碼差異只落在 `onVideoWrite` 分支（逐行列進 §7）。

微基準（只報數字，不報整機倍數，不作門檻）：

| 基準 | 內容 |
|---|---|
| `BenchmarkIndexedIntoLinear` | 鏈結 13h，64000 bytes |
| `BenchmarkIndexedIntoUnchained` | 非鏈結 13h，64000 bytes |
| `BenchmarkIndexedIntoPlanar640x480` | 16 色平面，307200 bytes |
| `BenchmarkVideoWriteObserved` | 裝空回呼，連續寫 A0000 視窗，改動前後每筆 ns |

報 ns/op、allocs/op、配對交錯 10 輪的中位數與 A/A 範圍，連同 `loadavg`、cpuset、Go 版本。454 §3.2 實測 A/A 範圍 28% 到 40%，時序不足以判定小差距，只作紀錄。

### 3.4 負對照（實作子代理逐列執行，結果寫進 §7）

| 編號 | 突變 | 必須失敗 |
|---|---|---|
| N1 | `IndexedInto` 的線性分支改以 `Peek8(0xA0000+i)` 讀 | §3.1 第 4 項（非恆等分頁，P1 出現、P2 消失）。恆等對映會假通過，所以第 4 項必做 |
| N2 | `IndexedInto` 的平面分支改用 `PixelsFrom(0, …)`（忽略顯示起點與分割線） | §3.1 第 3 項手算與金樣 |
| N3 | 非鏈結分支忽略 `LineCompare` | §3.1 第 2 項手算與金樣 |
| N4 | `IndexedInto` 另寫一份分支條件，把 `Unchained()` 與 `planarOn` 的判斷順序對調 | §3.1 第 2、7 項非鏈結案 |
| N5 | `IndexedInto` 內部 `make` 暫存再 `copy` | §3.1 第 9 項 |
| N6 | `len(dst)` 不足時寫入可寫的部分 | §3.1 第 8 項 |
| N7 | 平面分支每列跳過一欄（只寫前 `w/8 − 1` 個位元組的像素） | §3.1 第 3 項緩衝重用（殘留哨兵） |
| N8 | `IndexedInto` 讀第一個位元組時經 `readLin`（填 TLB、`utlb`） | §3.1 第 5 項 |
| N9 | `StepView.IndexedInto` 不做 `live()` 檢查 | §3.1 第 9 項 |
| N10 | `VWV86` 恆為 0 | §3.1 第 4、10 項 |
| N11 | `VWSegmented` 恆為 0 | §3.1 第 11 項 16 位元案 |
| N12 | `VWCode32` 恆為 0 | §3.1 第 11 項 D=1 案 |
| N13 | `videoWriteCtx()` 的 `VWCode32` 改讀 `cache[CS].Attr&attrDB`，不看 `segmented` | §3.1 第 12 項白箱 |

## 4. 決定

| 編號 | 事項 | 決定 | 理由 |
|---|---|---|---|
| 1 | VGA 讀取放哪一層 | `internal/machine` 提供本體，`session.StepView` 轉接 | 006 §2；014 D12 已裁 |
| 2 | 加 `EIP uint32` | 不加（主代理裁決） | §1.1：443 §2.3 不變式使它恆等於 `uint32(IP)`，守點經兩位審查者核實 |
| 3 | 加 `CodeBase uint32` | 不加（主代理裁決） | §1.1：目前沒有消費者 |
| 4 | 模式查詢 `IndexedLayout` | 納入本規格 | 014 §3.4.1 要先判鏈結 13h；鏈結與非鏈結長度相同 |
| 5 | `Ctx` 用位元集合 | 位元集合 | 一個 byte 留給日後擴充模式位元，結構維持 24 bytes |
| 6 | 欄位取「寫入當下」還是「指令起點」的模式 | 寫入當下 | 都讀既有狀態，零額外成本；兩者只在模式切換指令內不同，那些指令不寫 A000（§2.4 第 5 條） |
| 7 | 與 454 的關係 | 不依賴 | 本規格不用 `PeekBlock`、`Deliveries`、`MachineDigest`。兩份都在 `session/observer.go` 加 `StepView` 方法，彼此不衝突，合併順序不拘；454 先落地時，§3.1 第 5 項與 §3.2 第 4 項加上 454 的斷言 |
| 8 | 探針 | 不需要 | 事實都來自程式。014 的 P4、P5 是本規格的消費端驗證，不是本規格的閘門 |

## 5. 分期

動工前（實作者，結果寫進 §7）：

1. 金樣雜湊在 `2436db8` 的獨立 worktree 產生（不在實作分支上算），以現行 `Indexed()` 對 §3.1 第 2、3 項的固定內容計算。內容的亂數用 `rand.New(rand.NewSource(固定值))` 或測試自寫的 LCG，不用全域亂數。測試常數旁以註解記下產生命令與 commit `2436db8`。§7 附收據：同一測試檔（新 API 的呼叫以建置標籤或暫時改寫隔開，只留金樣比對）在 `2436db8` 上通過。
2. 容器內 grep 重核：`VideoWrite{` 的所有字面、`onVideoWrite` 的呼叫點、`.Indexed()` 的呼叫端、`Steps++` 只有 `machine.go:1541` 一處、`sb_dma8.go` 與 `dma8237.go` 沒有記憶體寫入。與 §2.3、§2.4 第 7 條不一致時先修本規格。

| 期 | 內容 | 完成條件 | 解鎖 014 |
|---|---|---|---|
| (a) | `IndexedKind`、`IndexedLayout`、`Machine.IndexedInto`；`pixelsAt`、`Unchained256` 拆出寫入緩衝形式；`Indexed()` 改寫 | §3.1 第 1 到 5、7、8 項與第 9 項的 machine 部分；N1 到 N8 | 無（要 (b)） |
| (b) | `StepView.IndexedLayout`、`StepView.IndexedInto` | §3.1 第 6 項與第 9 項的 session 部分；N9 | 分期 (c)（取像） |
| (c) | `VideoWriteCtx`、`videoWriteCtx()`、`VideoWrite.Ctx`；`refWrite8` 同步 | §3.1 第 4 項的 `VideoWrite` 斷言、第 10 到 13 項；N10 到 N13 | 分期 (f)（JEMM 歸屬、未歸屬寫入偵測） |
| 收尾 | §3.2、§3.3；`000-index.md` 登記 458 | 全部 | |

三期可以分開提交；每期都要 §3.2 第 1、3 項通過。

## 6. READY 條件

已滿足（2026-10-22）：

1. 第 2 版經兩位互不相見的獨立審查複審，兩位都同意轉 READY；複審的應改與建議已在第 3 版處理（附錄 B.2）。
2. §1.1、§4 決定 2、3 為主代理裁決（不加 EIP、不加 CodeBase）。

## 7. 實作收據

（未實作）

## 附錄 A：給規格 014 的回饋

| 014 位置 | 現文 | 依本規格應改為 |
|---|---|---|
| §2「VideoWrite 涵蓋」列 | 「`CS`、`IP` 是 `uint16`，32 位元碼段的 EIP 會截斷」 | `IP` 是完整 EIP（443 §2.3 不變式，§1.1 已證實）；缺的是執行模式，由 458 `Ctx` 提供 |
| §2「VideoWrite 涵蓋」列 | 「分頁寫入經 `writeLin`」 | 補：`Offset` 是實體位移；線性 `A0000` 被對到別處時那些寫入沒有 `VideoWrite`，別段線性對到實體 `A0000` 時有（458 §2.4 第 2 條） |
| §3.4.1 | 「取像前確認目前是 mode 13h 鏈結模式（`physical.go:42–44`）」 | 以 `StepView.IndexedLayout()` 判 `Kind == IndexedLinear && Mode == 0x13`；`physical.go:42–44` 是 `peekPhysRaw` 的平面分支，不是查詢介面 |
| §3.4.1 第 3 條、§3.4.2、D13 | 「V86 旗標與完整 EIP」 | `VideoWrite.Ctx.V86()`；不加 EIP（458 決定 2） |
| §3.4.2 BufCU | 經 454 批次讀 | 不變（BufCU 是客體線性位址，用 454；螢幕用 458） |
| §3.5.1 | 模式設定觸發退路 | 平面模式設定的清除與 BIOS 平面寫像素（`PlanarPutPixel`）都不經 `VideoWrite`；前者照現文以 `ModeObserver` 補，後者只在平面模式出現，014 的取像條件（鏈結 13h）不涉及 |
| §6 P4 | 每次回掃比對 | 時點由 458 §2.3 程式順序證實，內容由 458 §3.1 的獨立期望值證實；P4 留作長流程抽查。觀察器不必每步複製：記下第一次 `Frame` 的步數 S，之後只在 S + k × 回掃間隔附近的觀察點複製，直到該次 `Frame` 到來（回呼延後時回掃會晚幾步） |

## 附錄 B：審查紀錄與處理對照

| 輪 | 對象 | 審查 | 結論 |
|---|---|---|---|
| 1 | 第 1 版 | 甲 `review458v1-A.md`、乙 `review458v1-B.md`（互不相見） | 不轉 READY；共同阻擋為 oracle 不獨立 |
| 2 | 第 2 版 | 甲、乙複審 | 兩位都同意轉 READY；乙一條應改，甲乙合併四條建議，第 3 版處理（B.2） |

### B.1 第 1 版審查處理對照

審查甲（`review458v1-A.md`）：

| 條 | 意見 | 處理 |
|---|---|---|
| 阻擋 1 | 等價測試以 `Indexed()` 為 oracle，N1 到 N3 不會失敗 | 採：§3 開頭原則；§3.1 第 1 項比 `m.Mem` 直接切片，第 2、3 項測試內手算加改動前金樣雜湊，第 4 項非恆等分頁寫明兩段內容與初值兩兩不同；N1 到 N3 指向這些測試 |
| 應改 1 | 非恆等分頁案放錯層 | 採：§3.1 第 4 項改 machine 層 `RunUntilObserved` 加 `SetOnFrame`，移到分期 (a) |
| 應改 2 | N12 殺不掉 | 採：改白箱，造違反不變式的狀態直接測 `videoWriteCtx()`（§3.1 第 12 項、N13） |
| 應改 3 | 線性與非鏈結 W×H 恆為 320×200；mode 03h；平面編號但 `planarOn` 為假 | 採：§2.1 第 2 條表；§3.1 第 7 項 |
| 應改 4 | N13 的 rig 不常設、`refWrite8` 在 xlat 下不可比 | 採：刪除第 1 版 N13；§2.4 第 8 條寫明差分測試不驗新欄位與原因 |
| 應改 5 | READY 第 3 條循環；grep 結果 | 採：移到 §5 動工前；審查甲的 grep 結果寫入 §2.4 第 7 條（含 `strikecommander-census/main.go:286`） |
| 已核對 | 決定 2 不變式守點；同源；24 bytes；LE 不經 `writePhysRaw` | 寫入 §1.1、§2.3、§2.4 第 6 條、§2.6 |
| 建議 1 | §1 補守點與測試涵蓋 | 採：§1.1 |
| 建議 2 | §2.3 加 `sb8Log` 呼叫 `SBDMA` 一列 | 採 |
| 建議 3 | 442 列改寫 | 採：「最後一次載入的值；基底取自快取」 |
| 建議 4 | `ok=false` 時 layout 仍有效 | 採：§2.1 API 註解、§3.1 第 8 項 |
| 建議 5 | 決定 5 理由 | 採 |
| 建議 6 | 平面 `resetMode` 不經 `VideoWrite` | 採：§2.6、附錄 A |

審查乙（`review458v1-B.md`）：

| 條 | 意見 | 處理 |
|---|---|---|
| B1 | oracle 不獨立 | 同甲阻擋 1 |
| S1 | 緩衝重用殘留 | 採：§2.1 第 4 條全覆寫不變式；§3.1 第 3 項重用測試；N7「跳過一欄」 |
| S2 | N12 是等價突變 | 同甲應改 2 |
| S3 | 442 列應有 `VWCode32` | 不採：分段模式下清 PE 時 CS 為 D=1 由 `SetCR0` 拒絕（`linear.go:127–129`，443 §2.9），`decodePMV3` 也只在 PE=1 接受 D=1（`443-segmented-32bit-code-and-stack.md:240`）；`segmented.go:184` 是 `loadSegReal`，會清 CS 的 D/B。理由寫入 §2.4 第 4 條 |
| S4 | `Offset` 是實體位址 | 採：§2.4 第 1、2 條；§3.1 第 4 項；附錄 A |
| S5 | 非恆等分頁案改 machine 層；回合中不可取 `Digest` | 採：§3.1 第 4 項；§3.1 第 5 項改 machine 層逐步比暫存器、結束比 `StateDigest` |
| R1 | 主代理裁決核實 | 採：§1.1；`Seg[CS]` 存選擇子改為已證實（§2.4 第 4 條） |
| R2 | 同源順序寫成可 grep 的不變式 | 採：§5 動工前第 2 步（不列為 READY 條件，避免循環） |
| R3 | 既有消費者不受影響 | 採：§2.4 第 7 條 |
| R4 | `Ctx` 組裝抽小函式 | 採：`videoWriteCtx()`，`refWrite8` 共用 |
| R5 | `Indexed()` 斷言 `ok` | 採：§2.1 第 1 條 |
| R6 | BIOS 寫點出處 | 採：§2.4 第 5 條 `internal/dos/bios.go:785` |

### B.2 第 2 版複審處理對照

| 來源 | 意見 | 處理 |
|---|---|---|
| 乙 應改 | §3.1 第 5 項與 N8 的前置要讓 `readLin` 留痕 | 採：前置寫明線性 A0000 那一頁不在 TLB、`utlbR` 內且 PTE accessed=0（`xlatRead`，`paging.go` 約 160 行）；另掛涵蓋實體 A0000 的讀取監看（`physical.go:27–28`） |
| 甲乙 建議 1 | 手算公式單位寫死 | 採：§3.1 第 2 項（非鏈結：分割列 = `LineCompare()/(crtc[09h]&1Fh+1)`，列距 = `crtc[13h]×2`，`vga.go:741–747`）；第 3 項（平面：`LineCompare()` 不除掃描數，列距 `Pitch(w)`，`vga.go:444–449、473、486–497`） |
| 甲乙 建議 2 | 金樣在獨立 worktree 產生，記命令與 commit，附在 `2436db8` 通過的收據；固定亂數源 | 採：§5 動工前第 1 步 |
| 甲乙 建議 3 | 1000 步整機比對加平面模式 | 採：§3.1 第 5 項第二輪，每步另比 `VGA.latch` |
| 甲乙 建議 4 | BIOS 平面寫點 `PlanarPutPixel` 不經 `VideoWrite` | 採：§2.6、附錄 A |
