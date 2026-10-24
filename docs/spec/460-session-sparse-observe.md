# 460 session 稀疏觀察：觸發 IP 集合、步數期限與單次喚醒

狀態：**READY**（第 4 版，2026-10-24）。分期 (a) 已實作；新增分期 (b) 的 CS／簽章篩選尚未實作，契約見 §9。第 3 版三輪審查紀錄為陸空戰將 repo `workplace/perf100/review460-461*.md`；第 4 版兩位獨立審查均無阻擋，應改已處理，見 `workplace/perf100/review460b-v2-A.md`、`review460b-v2-B.md`。尚未 CONFORMED；`sparseDefault` 維持 false。實作收據與已知缺項見陸空戰將 repo `workplace/impl460/receipt.md`；本輪補驗與剩餘閘門見 §8。
基準：分期 (a) 為 dosgolem-sc `395b679`，分期 (b) 讀碼基準為 `4095084`。來源：strike_commander 計畫 002 §3 P 第 1 項；量測見 `workplace/perf100/report.md`。

§1–§7 保存分期 (a) 的介面與驗收。下列明文修訂及 §9 是分期 (b) 的擴充契約；(b) 的第二層、刷新、oracle、六類呼叫率與翻預設條件依 §9，不以 (a) 的舊量測取代。§8 是歷史補驗紀錄，保持原貌。

## 0. 處置紀錄

### 0.1 第 3 版

複審報告：`workplace/perf100/review460-461v2-A.md`（甲）、`review460-461v2-B.md`（乙），兩位皆無阻擋。主代理裁決：應改全部採納，取捨如下。

| 意見 | 處置 | 節 |
|---|---|---|
| 乙 N1、甲 1：每千步抽一個快照抓不到稀有漏觸發，快照成本未界定 | 主 oracle 改為「每個略過步以 §2.2 判斷式檢查」；深度快照限單元測試與低頻種子化隨機抽樣；排除清單照甲 1；先量單次成本再定抽樣率 | §5.2 |
| 甲 2：分層抽樣、W 鎖存時點 | 觸發 IP 步、`Force` 步、`NextWake` 讀取後 64 步內全查快照；W 的 `NextWake` 只在稀疏模式會觀察的步之後鎖存；稽核述詞與 §3.3 逐項對齊 | §5.2 |
| 甲 6：T1–T4 不可重寫述詞 | 述詞抽成單一函式 `ShouldObserve`，`RunUntilSparse`、稽核、T1–T4 共用 | §3.1、§5.2、§5.3 |
| 乙 N3：每回合第一步的 `Force` | `RunReceipt` 在每次 `Advance` 前設稽核的回合開始旗標 | §5.2 |
| 乙 N2：1 步回合誤傷呼叫率斷言 | 只對 §6.1 四個長情境斷言 ≤ 1%，其餘長測試只記錄；分母排除 `Force` 觀察 | §5.1、§5.4 |
| 甲 3：切分 A/B | 定向切分為主（逐步模式先記事件步，於事件步與事件步 +1 切），種子化隨機為輔；種子與切分點寫入收據 | §5.1 |
| 甲 4、乙 N5：S4 判別量 | 改為 `flight-log.tsv` 的 "redeliv" 列（Detail 含步數，`flight_redeliver.go:73-74`）；收據已含此檔（`flight_long_test.go:367-370`） | §5.3 |
| 甲 5：live 條件與接線負對照 | 補 S12（afterRet）、S13（recheck）、S14（Exited）、S15（第一步 Force）、S16（panic Force）、S17（viewMisuse Force） | §5.3 |
| 乙 N4：T2 前提 | 寫明 Frame 到期限之間沒有觸發 IP | §5.3 |
| 甲建議：預設開的時點 | §5 全過後才翻 `SessionConfig` 預設開 | §3.5、§7 |
| 甲建議：O2 做法；乙建議：反向檢查 | O2 以 `go/types` 取 `case` 常數識別字集合與 IP 表雙向比對 | §4 |
| 甲建議：5% 門檻 | 註明實作後量再調 | §5.4 |
| 甲建議：IPSet 8 KB 複製 | 每回合一次，備註 | §3.1 |
| 乙建議：測試包裝嵌入 | 盤點列為實作步驟 | §3.6 |

### 0.2 第 2 版

審查報告：`workplace/perf100/review460-461-A.md`（甲）、`review460-461-B.md`（乙）。主代理裁決：阻擋與應改全部採納。

| 意見 | 處置 | 節 |
|---|---|---|
| 甲阻擋 1：S3 無法失敗；Subtitles `pending`、Presenter `cuPend` 是死條件，§2.2 的 `cuPend` 描述錯 | 兩者從 `NextWake()` 刪除；§2.2 改寫（`cuPend` 是同一步內消費；字幕解析靠 SBDMA 之後的 `Force`）；S3 改為 S3′（拿掉 SBDMA 之後的 `Force`） | §2.2、§3.4、§5.3 |
| 甲阻擋 2、乙 B1(a)：稽核指紋不足 | 改為觀察器深度快照雜湊（排除四個簿記欄位），加不變式「略過步必 `Steps < NextWake()`」，可抽樣 | §5.2 |
| 乙 B1(b)：期限類漏觸發抓不到 | 四個期限來源各一個合成單元測試與必敗負對照 | §5.3 |
| 甲應改 3、乙 A2：`TriggerIPs` 計算時點 | 每次 `runObserved` 開始時算一次，只含已掛的觀察器；O2 涵蓋注入的窗表 | §3.1、§3.4、§4 |
| 甲應改 4、乙 A6：成本算術、效能量法 | §6.1 的「比率 × 36%」刪除，成本改為實作後量；驗收改確定性指標（觀察呼叫率每個長測試斷言 ≤ 1%、pprof cum 佔比）；牆鐘比較採同容器同 cpuset 交錯 N 輪比 CPU 秒、高負載輪作廢 | §5.4、§6.1 |
| 甲應改 5、乙 A7、主代理裁決：N1 | 接受等價論證，條件是隨機 Advance 切分 A/B（比對 Frames、Deliveries、SBDMALog）通過；不過就退回 N1 強制逐步 | §3.4、§5.1 |
| 甲應改 6：`FrameAt` 位置；O3 納音訊回呼 | `FrameAt` 在 Frame 包裝的 recover 內、早於 `Frame`；O3 擴及音訊回呼 | §3.3、§4 |
| 甲應改 7：`NextWake` 哨兵 | `0` 表示下一步 | §3.1 |
| 甲應改 8：修訂既有規格；VideoWrite Force 風險 | 列出 238、452、454 的修訂；AGENTS 2026-10-22 決定退路取像在下一次回掃，VideoWrite 不需 `Force`，風險降級 | §3.6、§4 O3 |
| 甲應改 9：S4 前提 | 先確認長測試有非 0 的撤銷計數，否則改單元測試 | §5.3 |
| 乙 A1：`RunUntilSparse` 細節、Force 接線 | 逐點列出 | §3.1、§3.3 |
| 乙 A3：預設值與開關 | 正式組預設開；`SparseAudit` 改為陸空戰將套件內未匯出的測試鉤子，不進 `session.Config`；「前端每格 Advance」標假說 | §3.5、§5.2、§6.2 |
| 乙 A4：零配置 | `TriggerIPs`、`NextWake`、`FrameAt` 的 AllocsPerRun 測試 | §5.5 |
| 乙 A5：嵌入型別 | 具體型別斷言與編譯期斷言測試 | §3.6、§5.5 |
| 乙建議：O2 自動化；觸發率未涵蓋 Force | O2 由匯出的 IP 表與 `go/ast` 列舉 `case` 常數比對；Force 頻率另記 | §4、§6.1 |
| Exited 檢查方式（主代理裁決） | 每步指標檢查 | §3.1 |

## 1. 範圍與決定

陸空戰將全開（中文覆繪、飛行、字幕、規格 014 呈現層）時，`session.runObserved` 每道指令都呼叫一次
`StepObserver.BeforeStep`。pprof（飛行 Q，`out/prof-flq-full/top-cum.txt`，重負載主機）：

| 節點 | cum |
|---|---|
| `runObserved.func10`（`observe` 閉包，`session/observer.go:253`） | 36.0% |
| 其中 `SessionObserver.BeforeStep`（`apps/strikecommander/receipt.go:87`） | 31.8% |
| `Presenter.Observe` | 13.7%（含每步呼叫的 `FlightObserver.CUState` 5.4%，`present.go:585`） |
| `FlightObserver.Observe` | 7.2% |
| `Subtitles.Observe` | 4.1% |
| `LabelObserver.Observe` | 3.3% |
| `machine.Step` | 58.9% |

基地對話（`out/prof-dlg-full`）：`func10` 32.9%、`BeforeStep` 29.2%。

盤點（§2）顯示四個觀察器在絕大多數步驟只比一次 IP 就返回。本規格讓 session 只在「可能有作用的步驟」呼叫
`BeforeStep`，其餘步驟在機器迴圈內以一次位元遮罩查表略過。

決定：

1. 新增選配擴充 `session.SparseObserver`。不實作或未開啟時照舊每步呼叫，行為不變（§3.6）。
2. 觸發類別為 IP、步數期限、單次喚醒；分期 (b) 在 IP 後加 CS 白名單與簽章必要區段（§9）。沒有線性 PC 條件或記憶體寫監看。
3. 第一層鍵是 `CPU.IP`，與 `StepView.IP()` 同值；(b) 第二層另取 CS 與唯讀 bytes，不用 `PCLinear()`（§9.3）。
4. 驗收以「稀疏與逐步兩種模式的全部陸空戰將長測試收據逐位元組相同」為準（§5）。
5. 本規格不改任何觀察器的判定規則、不改 009、010、014 的語意；只改呼叫時機與步數來源。

不在範圍：CPU 核心加速、前端合成、音訊、HD 3D；dosgolem 其他遊戲的觀察器（buckrogers、census 等不實作
`SparseObserver`，審查乙已核對）。

## 2. 盤點（`395b679`）

### 2.1 觸發 IP

四個觀察器都以 `switch v.IP()` 分派，分派前不看 CS；CS 在分支內比對。base 都是執行期學到的（puts 入口簽章、
shape 入口簽章），學到之前只能靠 IP 觸發（任何 CS）。

| IP | 觀察器與動作 | CS 如何決定 | 位置 |
|---|---|---|---|
| `034B` | Labels puts 入口：讀簽章、學 base（`cs − 3942h`） | 任何 CS；簽章決定 | `observer.go:318`、`391` |
| `03D3` | Labels puts 返回（`cur != nil`） | `base+3942h`，SS:SP 配對 | `observer.go:320`、`517` |
| `02B2` | Labels putc（`cur != nil`） | `base+3942h` | `observer.go:324`、`504` |
| `0000` | Labels shape 入口（004）與 blit 入口（001）；Presenter blit 窗開窗 | shape：base 未知時任何 CS（以 `cs − 3A97h` 候選學 base），已知時 `base+3A97h`；blit：`base+39FBh` | `observer.go:328`、`dialog.go:95`、`observer.go:551`；`present.go:150`、`615` |
| `008B` | Labels blit 返回（`paired`）；Presenter blit 窗關窗 | `base+39FBh`，SP 配對 | `observer.go:334`、`609`；`present.go:699` |
| `0002` | Labels MENU shape 入口（008） | 任何 CS，簽章決定，不學 base | `observer.go:338`、`dialog.go:131` |
| `0345` | Flight puts 入口：學 base（`cs − 5F7Ah`）、重驗節流 | 任何 CS；簽章決定 | `flight.go:294`、`363` |
| `02AC` | Flight putc（`cur != nil`） | `base+5F7Ah` | `flight.go:296`、`480` |
| `03CD` | Flight puts 返回（`cur != nil`） | `base+5F7Ah`，SS:SP 配對 | `flight.go:300`、`495` |
| `0182` | Flight 橫幅入口（`baseOK`） | 任何 CS（overlay，段每次不同）；兩段簽章決定 | `flight.go:304`、`598` |
| `01F2` | Flight 橫幅返回（`plq != nil`） | 入口記下的 CS，SS:SP 配對 | `flight.go:308`、`645` |
| `00A5`、`00D6` | CU 入口（`baseOK`） | `base+1D3Ah`，98 bytes 簽章 | `flight.go:312`、`cutracker.go:67`、`98` |
| `00A7`、`00D8` | CU 返回 | `base+1D3Ah`，IP 與 SS:SP 配對 | `cutracker.go:71`、`136` |
| `00EE` | MU | `base+1D3Ah`，簽章 | `cutracker.go:75` |
| `103F` | Subtitles 對話直譯器入口 | 任何 CS；簽章決定 | `subtitle.go:210` |
| `10F3`、`22BE` | Subtitles 物件擷取 | 任何 CS；簽章決定 | `subtitle.go:214`、`218` |

共 19 個 IP，全部是編譯期常數。Presenter 另以 `ipMask`（`present.go:418-421`）由窗表產生；窗表預設是
`presRoutines`（`present.go:150-153`，分期 (c) 只有 blit：`0000`、`008B`），但可由 `cfg.routines` 注入
（`present.go:414-417`），所以 Presenter 的 IP 以實際窗表為準（§3.4）。

SS:SP 窗：Labels、Flight 的 puts 與橫幅返回、CU 返回、Presenter 窗關窗都以入口記下的 SS:SP 配對。這些只在
上表的 IP 上比對，本身不需要每步觀察。

### 2.2 不依 IP 的工作

| 觀察器 | 條件 | 性質 | 處理 | 位置 |
|---|---|---|---|---|
| Labels | 可見集合非空、`baseKnown`、`Steps() − lastSig ≥ 5,000,000` 時重驗簽章，不符清空並 `gen++`、記 "sig" | 步數期限 | `NextWake` 期限 `lastSig + SigCheckSteps` | `observer.go:343-354` |
| Flight | `pendOn` 時先 `checkPend`：比 `Deliveries()` 與入口的 `d0`，增加就撤銷入口改動 | 下一步必觀察（入口那一步設、下一步消費） | `NextWake` 回 0 | `flight.go:290`、`flight_redeliver.go:35-42` |
| Flight | 可見集合非空、`baseOK`、`Steps() − lastSig ≥ 5,000,000` 時重驗 | 步數期限 | 期限 `lastSig + SigCheckSteps` | `flight.go:318-329` |
| Subtitles | `pending` 非空時解析 SBDMA start（`Steps() == e.Step` 才解析，否則記 `SkipTiming`） | `pending` 只在 SBDMA 回呼內加入（`subtitle.go:253`），在下一次 `Observe` 開頭清空（`subtitle.go:201-202`、`303-304`）；`BeforeStep` 結束時恆為空 | 不進 `NextWake`（死條件）；由 SBDMA 之後的 `Force` 喚醒下一步（§3.3） | `subtitle.go:201`、`302` |
| Subtitles | 最後一條 `SubWaiting` 且 `Steps() > LastDue + T` 時結束（`End = LastDue`、`ReasonTimeout`） | 步數期限 | 期限 `LastDue + T + 1` | `subtitle.go:204-208` |
| Presenter | 每步寫 `p.steps`、`p.cuOpen = flight.CUState()` | 簿記 | 由觀察點與 `FrameAt` 供給（§3.5） | `present.go:582-586` |
| Presenter | `recheck != nil`：完成點取像（`complete`、`cuAfterReturn` 內 `scheduleRecheck`）後的下一個觀察點重讀 VGA | 下一步必觀察（取像在 `dispatch` 或 `afterRet` 分支，晚於同一步的 `recheck` 檢查） | `NextWake` 回 0 | `present.go:588`、`885-919` |
| Presenter | `afterRet && steps != afterRetAt`：CU 返回那一步由 `onCU` 設旗標，下一步消費 | 下一步必觀察 | `NextWake` 回 0 | `present.go:591`、`958` |
| Presenter | `cuPend`：CU 入口被接受時由 `onCU`（在同一步的 `Flight.Observe` 內）設定，同一步稍後的 `Presenter.Observe` 立即消費（`present.go:596-599`） | 同步驟內 | 不進 `NextWake`（死條件） | `present.go:596`、`926` |
| Presenter | 窗堆疊非空時 `checkTimeouts`：`Steps() − w.entry ≥ timeout`（200,000）關窗 | 步數期限 | 期限 `min(w.entry + w.r.timeout)` | `present.go:600`、`732` |
| Presenter | `clearPend`：同一步內由 `OnClearArt` 設、在同一個 `Observe` 末清 | 同步驟內 | 不需處理 | `present.go:606` |
| SessionObserver | 每步寫 `s.steps`、`s.cur` | 簿記 | `s.steps` 由 `FrameAt` 供給（§3.4） | `receipt.go:88-90` |
| expoRun（N1） | `afterStep` 每步比 `dirty` 與兩個 generation | 事件之後的下一步 | §3.4 的條件式等價 | `exposure.go:951-992` |

不是每步的工作（已只在觸發 IP 或 Frame 執行，不需改）：CU 逾時 `cuTimeoutSteps`（`cutracker.go:88`）、待提交過期
`flightStaleSteps`（`flight.go:757`）、Flight 重驗節流 `flightReverifySteps`（`flight.go:389`）、Labels 的 `artBadCS`。

### 2.3 讀取的 StepView 欄位

| 觀察器 | 欄位 |
|---|---|
| Labels | `Steps`、`CS`、`IP`、`SS`、`SP`、`BP`、`DS`、`Read8` |
| Flight（含 cuTracker） | 上列加 `AX`、`CX`、`Deliveries`（`viewDeliveries`，`present.go:967`） |
| Subtitles | `Steps`、`CS`、`IP`、`SS`、`DS`、`DI`、`Read8` |
| Presenter | `Steps`、`CS`、`IP`、`SS`、`SP`、`DS`、`Read8`、`Deliveries`、`Palette`、`ReadBlock`、`IndexedLayout`、`IndexedInto` |

### 2.4 Frame 回呼的步數

`SessionObserver.Frame`（`receipt.go:125-143`）把 `s.steps`（最後一次 `BeforeStep` 的步數）交給 `OnFrame`、
`Flight.Frame`（`FlightCommit.FrameSteps`、`lastSig`、CU 逾時）、`OnFlightFrame`、`Subtitles.Frame`（`frameT`、阻擋 skip）。
`Presenter.Frame` 用 `p.steps` 寫 `derive` 的 `CaptureSteps`（`present.go:840`）與 `PresentStale.Steps`（`present.go:1241`）。

機器端：`onFrame` 在 `tick()` 內呼叫（`machine.go:1797-1802`），`tick()` 在 `m.Steps++`（`machine.go:1666`）之前。
所以 Frame 時的 `m.Steps` 等於同一次步驟嘗試之前 `BeforeStep` 看到的 `Steps()`。已證實（讀碼；審查甲、乙核對）。

### 2.5 結論

真的需要每步的部分是零：每一項都是觸發 IP、步數期限、單次喚醒或同步驟內工作。唯四的簿記欄位（`s.steps`、
`s.cur`、`p.steps`、`p.cuOpen`）改由事件供給或證明不變。回掃與 SBDMA 已是事件，只需在其後補一次喚醒（§3.3）。

## 3. 設計

### 3.1 介面與機器迴圈

```go
// session 套件
type IPSet [1024]uint64 // 65536 位元；bit ip 為 1 表示 IP == ip 的步驟要觀察

func (s *IPSet) Add(ip uint16)
func (s *IPSet) Has(ip uint16) bool
func (s *IPSet) Union(o *IPSet)

const NoWake = ^uint64(0)

// SparseObserver 是 StepObserver 的選配擴充（規格 460）。Config.SparseObserve 為真且觀察器實作它時，
// session 只在 §3.3 的步驟呼叫 BeforeStep；否則照 238 每步呼叫。
type SparseObserver interface {
	StepObserver
	// TriggerIPs 回觸發 IP 集合。session 只在每次 runObserved 開始時讀一次並複製；回合內集合不變。
	TriggerIPs() IPSet
	// NextWake 回下一個無論 IP 都要觀察的步數（BeforeStep 時的 Steps() 值）。0 表示下一步；沒有期限回 NoWake。
	// session 只在 BeforeStep 正常返回之後讀。
	NextWake() uint64
	// FrameAt 在每次 Frame 之前呼叫，steps 是回掃當下的 machine.Steps（§2.4）。
	FrameAt(steps uint64)
}
```

機器端新增：

```go
type StepFilter struct {
	IPs    [1024]uint64 // 回合開始時由 session 複製
	Wake   uint64       // m.Steps >= Wake 時觀察；0 表示下一步
	Force  bool         // 下一步必觀察；觀察後清除
	Exited *bool        // 非 nil 且為真時必觀察（程式結束的停止點，observer.go:256）
}

func (m *Machine) RunUntilSparse(stop func(*Machine) bool, budget uint64, f *StepFilter, observe PreStepObserver) (Stop, error)
```

述詞只有一份，`RunUntilSparse`、稽核（§5.2）與期限單元測試 T1–T4（§5.3）都呼叫它，不各自重寫：

```go
// ShouldObserve 回步驟（目前 CS:IP 的 ip、目前 steps）在稀疏模式是否呼叫 BeforeStep（§3.3）。
func (f *StepFilter) ShouldObserve(ip uint16, steps uint64) bool {
	return f.Force || steps >= f.Wake || f.IPs[ip>>6]&(1<<(ip&63)) != 0 || (f.Exited != nil && *f.Exited)
}
```

`RunUntilSparse` 與 `RunUntilObserved`（`probe.go:197-220`）逐行相同，只有 `observe` 呼叫加條件：

1. 中斷點：照舊 `if i > 0 || len(m.breaks) == 0 { if m.atBreak() { return StopBreakpoint, nil } }`。
2. 停止述詞：照舊 `if i > 0 && stop != nil && stop(m)`。
3. 觀察：
   ```go
   if f.ShouldObserve(m.CPU.IP, m.Steps) {
   	f.Force = false
   	if err := observe(m); err != nil { return StopBudget, err }
   }
   ```
   `ShouldObserve` 要能內聯；若編譯器不內聯，`RunUntilSparse` 以同一運算式展開，並以測試比對兩者在隨機輸入下的結果相同。
4. `m.Step()` 錯誤照舊 `return StopBudget, err`；迴圈結束後的 `stop` 檢查照舊。

`Wake` 寫回：session 的 `observe` 閉包（`observer.go:253-278` 的稀疏版）在 `BeforeStep` 回 nil 之後設
`f.Wake = so.NextWake()`。`BeforeStep` 回錯或 panic 時不寫（迴圈即將停止）。

`Exited`：指向 `o.dos.Exited`，每步一次指標讀取（主代理裁決）。

`IPs` 是 8 KB 陣列，每次 `runObserved` 開始時複製一次；每回合一次，相對回合的步數可忽略。

### 3.2 觸發鍵與分段模式

- 鍵是 `m.CPU.IP`，即 `StepView.IP()`（`observer.go:98`）的值。四個觀察器的分派也是比這個值，所以集合與分派用同一個量，
  不需要知道 CS 基底、分段描述元或分頁。
- JEMM 分段模式（434）：`CPU.IP` 是選擇子內的位移；D=1 碼段依 443 不變式 `IP` 即完整 EIP（458 §2）。
- 刻意與 `atBreak`（`probe.go:222-237`，分段模式比 `PCLinear()`）不同：中斷點是位址語意，觸發集合是「觀察器會在這個 IP
  做事」的語意。改用線性位址會讓 base 未知前的「任何 CS」觸發無法表達。
- 分期 (b) 已列為翻預設前置：新增 CS／狀態與簽章區段篩選、每觀察步刷新，依 §9。§6.1 的 0.58% 是舊 (c) 窗表量測，不能代表 014 (d)。

### 3.3 何時呼叫 BeforeStep

`Config.SparseObserve` 為真且觀察器實作 `SparseObserver` 時，步驟 n 呼叫 `BeforeStep` 若且唯若下列任一成立：

1. `CPU.IP ∈ IPs`；分期 (b) 啟用第二層時，另須符合 §9.3 的規則。Force、Wake、Exited 仍直接放行。
2. `m.Steps ≥ Wake`。
3. `Force`。session 在下列時刻設定（逐點接線）：
   - `runObserved` 開始時（每次 `Advance` 的第一步；回合之間呼叫端可能改過觀察器）。
   - `SetOnFrame` 包裝（`observer.go:200-210`）：在 recover 範圍內，依序 `FrameAt(m.Steps)`、`Frame(...)`、設 `Force`。
   - `OnSBDMA` 包裝（`observer.go:235`）與 `ObserveModeChanges` 包裝（`observer.go:225`）：回呼之後設 `Force`。
   - `guard` 與 `VideoWrite` 包裝攔到 panic、設 `videoPanic` 時設 `Force`（錯誤照現行在下一步回報）。
   - `StepView.live()`（值接收者，經 `v.o` 指標寫 Owner 欄位）設 `viewMisuse` 時一併設 `o.filter.Force`。
4. `*Exited` 為真。

`VideoWrite` 與音訊回呼之後不設 `Force`（§4 O3）。成本：Frame 每 165,000 步一次（`DefaultVGAFrameEvery`，
`machine.go:248`），SBDMA 與模式設定更少。

### 3.4 陸空戰將 SessionObserver 的實作

- `TriggerIPs()`：回已掛觀察器的 IP 聯集，每次呼叫現算（session 每回合讀一次）：Labels 恆掛；`Flight`、`Subtitles` 非 nil
  才併入（它們在 `SessionObserver` 建好之後才賦值，`receipt.go` 的 `RunReceipt` 接線）；`present` 非 nil 時併入
  `Presenter.ipMask`（以實際窗表為準）。
- 分期 (b) 另實作 SparseRuler，回合開始與 BeforeStep 成功返回後刷新規則，見 §9.3、§9.4；第一層仍每回合一次。
- `NextWake()`：各觀察器 `nextWake()` 的最小值。
  - Labels：可見集合非空且 `baseKnown` 時 `lastSig + SigCheckSteps`，否則 `NoWake`。
  - Flight：`pendOn` 時 0；可見集合非空且 `baseOK` 時 `lastSig + SigCheckSteps`。取較小者。
  - Subtitles：最後一條 `SubWaiting` 時 `LastDue + T + 1`，否則 `NoWake`。
  - Presenter：`recheck != nil` 或 `afterRet` 時 0；窗堆疊非空時 `min(w.entry + w.r.timeout)`。
  - `afterRet` 的「`steps != afterRetAt`」：回 0 讓下一步觀察，下一步的 `Steps()` 必大於 `afterRetAt`（步驟嘗試失敗時迴圈即停止），
    條件照舊成立。
- `FrameAt(steps)`：設 `s.steps = steps`；`present` 非 nil 時設 `present.steps = steps`。
- N1（`afterStep`）：照舊在每次 `BeforeStep` 末呼叫。N1 的 generation 只在觀察點或 Frame 改變，Frame 之後有 `Force`，所以判定
  步數與逐步模式相同。主代理裁決接受此論證，條件是 §5.1 第 4 條的隨機切分 A/B 通過；不通過則 `ReceiptConfig` 擋下
  「稀疏＋N1」，N1 強制逐步模式。

### 3.5 開關與預設

- `session.Config.SparseObserve bool`：session 預設假（其他遊戲不受影響）。
- 陸空戰將 `SessionConfig`（`receipt.go:146`）預設稀疏，但這一步在 §5 全部通過之後才翻（§7 第 5 步）；之前以 `ReceiptConfig.sparse`
  （未匯出）與長測試環境變數 `DOSGOLEM_SC_SPARSE=1` 開啟。翻轉之後 `DOSGOLEM_SC_SPARSE=0` 改回逐步，供 A/B。
- 稽核（§5.2）是陸空戰將套件內 `SessionObserver` 的未匯出欄位，只由測試設定；不進 `session.Config`。

### 3.6 相容與既有規格修訂

- 觀察器不實作 `SparseObserver`，或 `SparseObserve` 為假：`runObserved` 照現行呼叫 `RunUntilObserved`，位元組不變。
- 嵌入型別：`presentObserver` 嵌入 `*SessionObserver`（`receipt.go:115`），方法提升後同樣滿足 `SparseObserver`，這是預期的。
  其他包裝型別若嵌入 `*SessionObserver` 又自帶每步工作，會被誤判為稀疏。處置：陸空戰將加編譯期斷言
  `var _ session.SparseObserver = (*SessionObserver)(nil)`、`= presentObserver{}`；測試以具體型別斷言確認 `RunReceipt` 交給
  session 的觀察器只有這兩種，census（`cmd/strikecommander-census`）的觀察器不滿足 `SparseObserver`。實作時另盤點全部測試檔中
  嵌入 `*SessionObserver` 或 `presentObserver` 的包裝型別，逐一確認其 `BeforeStep` 沒有每步工作，或讓它們以 `SparseObserve` 關閉執行。
- 要修訂的既有規格：
  - 238：`BeforeStep` 不再保證每步呼叫（稀疏模式的例外與 `SparseObserver` 指向 460）。
  - 452 §2.4：「消費端在下一次 `BeforeStep` 讀」在稀疏模式由 SBDMA 之後的 `Force` 保證，補一句。
  - 454：`Deliveries` 的讀取時點不變，補一句「稀疏模式下的觀察點見 460」。
- 014 完成點窗：開窗、關窗在觸發 IP；逾時改期限；`VideoWrite` 歸屬讀窗堆疊，堆疊只在觀察點改變；退路取像依 AGENTS
  2026-10-22 決定（未歸屬寫入後在下一次回掃取畫面）在 Frame 內完成，不需要 VideoWrite 之後的喚醒。
- `receipt_alloc_test.go` 的零配置要保住（§5.5）。

## 4. 決定性論證

設逐步模式為 P、稀疏模式為 Q，同一狀態檔、同一輸入、同一停止步數。

1. 機器序列相同：觀察器不寫機器（238），`StepFilter` 只讀 `CPU.IP`、`Steps`；兩者執行相同的指令序列，`VideoWrite`、`Frame`、
   `SBDMA`、`VideoModeChange` 回呼在相同步驟、以相同參數發生。
2. 不作用引理 L1：步驟 n 若 IP 不在集合、`n < Wake`、沒有 `Force`，則 P 在 n 呼叫的 `BeforeStep` 只改寫四個簿記欄位
   （`s.steps`、`s.cur`、`p.steps`、`p.cuOpen`），不改其他觀察器狀態、不產生輸出。逐項依據見 §2.2；IP 分派在集合外全落
   `default`。
3. 簿記欄位：`s.steps`、`p.steps` 的讀者由 `FrameAt` 與觀察點供給同值（§2.4）；`p.cuOpen` 依不變式 I1（每個回呼結束時
   `p.cuOpen == flight.CUState().InProgress`；追蹤器的 `cuIn` 只在 CU 觀察點與 `cuTracker.Frame` 改變，`onCU` 與
   `Presenter.Frame` 同步設定，`present.go:923-965`、`1203-1205`）在兩個觀察點之間不變；`s.cur` 只在 `BeforeStep` 內讀。
4. 期限正確：期限欄位（`lastSig`、`visible`、`baseKnown`、`baseOK`、`LastDue`、`State`、`T`、窗堆疊）只在觀察點或 Frame、SBDMA
   回呼內改變。觀察點之後立即重讀 `NextWake()`；Frame 與 SBDMA 之後有 `Force`，下一步重讀。
5. 由 1 到 4 對步數歸納：每一步之後兩模式的觀察器狀態相同，收據相同。

義務（以測試證明）：

- O1：每個不依 IP 的分支都進了 `nextWake()`。由 §5.2 的稽核與 §5.3 的期限單元測試檢查。
- O2：`TriggerIPs()` 涵蓋每個 `switch v.IP()` 的 case 與實際窗表。各觀察器在套件內提供 IP 表；測試以 `go/types` 解析
  `observer.go`、`flight.go`、`cutracker.go`、`subtitle.go` 中 `switch v.IP()` 的 `case` 運算式，取常數識別字集合與值，與 IP 表雙向比對
  （表內每個 IP 都有 case、每個 case 都在表內）；Presenter 以注入的窗表建一次，比 `ipMask`。
- O3：`VideoWrite` 與音訊回呼不改任何進入 `NextWake()` 的欄位。`Presenter.VideoWrite` 現改 `vwTotal`、`vwSinceCap`、
  `win.wrote`、`epWrote`、`needFallback`、`unattr`（`present.go:1161-1190`），都不進 `NextWake()`；`SessionObserver` 目前不實作
  `AudioObserver`。`needFallback` 目前只寫不讀（014 分期 (f) 未實作）；依 AGENTS 2026-10-22 決定，退路取像在下一次回掃，
  不需要 VideoWrite 之後的喚醒。若日後改為「未歸屬寫入之後的下一步」，必須把 `VideoWrite` 加進 §3.3 的 `Force` 並重估成本
  （全螢幕 blit 約 64,000 筆寫入）。

## 5. 驗收

### 5.1 收據逐位元組相同

1. 全部陸空戰將長測試（`apps/strikecommander/*_long_test.go`：flight Q／R／Q-legacy、composed、subtitle、labelframe、mainmenu、
   saveload、saveload_confirm、killboard、pointer_move、trigger、exposure）以 `DOSGOLEM_SC_SPARSE=0` 與 `=1` 各跑一次，收據目錄
   `diff -r` 為空，454 的 `MachineDigest` 相同。倍率 2、3 都比；呈現層開與關都比。
2. 以既有期望常數（459 重錄後，`798c3aa`）為獨立期望值：`=0` 必須與既有常數相同，`=1` 必須與 `=0` 相同。
3. 觀察呼叫率（非 `Force` 的 `BeforeStep` 次數 ÷ 步數）：§6.1 的四個長情境（主選單 m242、基地對話 s500＋c11、飛行 Q s800、
   飛行 R s760，各 3 億步）在 `=1` 時斷言 ≤ 1%。其餘長測試只記錄，不斷言（短回合的第一步 `Force` 會把比率推高，不代表成本）。
4. 切分 A/B（N1 的條件）：同一長測試把推進拆成多次 `Advance`（切分不送輸入），`=1` 與 `=0` 各跑；比對收據、`Frames`、
   `Deliveries()`、`SBDMALog`，以及曝光收據 N1 的判定步數。全部相同才接受 §3.4 的 N1 論證。
   - 定向切分（主）：先以 `=0` 跑一次記下事件步（Frame、SBDMA、CU 入口與返回、Presenter 開窗與關窗、puts 與橫幅入口、
     §2.2 各期限的到期步），從中以固定種子選至多 64 個事件，於事件步與事件步 +1 各切一刀。
   - 隨機切分（輔）：固定種子，每段 1 到 2,000 萬步。
   - 種子與全部切分點寫入收據（`splits.tsv`），重跑以同一份切分點重現。

### 5.2 稽核

陸空戰將 `SessionObserver` 的未匯出稽核欄位（只由測試設定）。session 以逐步模式執行；稽核在 `SessionObserver` 內維護一份影子
`StepFilter`，以 §3.1 的同一個 `ShouldObserve` 判斷每一步在稀疏模式會不會被觀察。影子的輸入與 §3.3 逐項對齊：

| §3.3 條件 | 影子如何得到 |
|---|---|
| 觸發 IP | 回合開始時取 `TriggerIPs()` |
| `Wake` | 只在影子判為「會觀察」的步、`BeforeStep` 執行完之後讀 `NextWake()` 鎖存（與稀疏模式相同時點） |
| `Force`：回合第一步 | `RunReceipt` 在每次 `Advance` 之前設稽核的回合開始旗標 |
| `Force`：Frame、SBDMA、VideoModeChange 之後 | `SessionObserver` 自身的這三個回呼被呼叫時設定 |
| `Force`：panic、viewMisuse | 稽核執行中出現即直接判稽核失敗（測試不應觸發） |
| `Exited` | 不需影子：逐步模式在程式結束後的第一步即停（`observer.go:256`），稽核看不到結束之後的步 |

檢查分三層：

1. 判斷式 oracle（每個略過步，主檢查）：對每個影子判為略過的步，在呼叫觀察器之前以 §2.2 的原始條件直接檢查觀察器狀態，任一成立即
   失敗。條件逐條取自 §2.2：Labels 與 Flight 簽章層（`visible` 非空、base 啟用、`Steps − lastSig ≥ 5,000,000`）、Flight `pendOn`、
   Subtitles 等待逾時（`SubWaiting` 且 `Steps > LastDue + T`）、Subtitles `pending` 非空、Presenter `recheck != nil`、
   `afterRet && Steps != afterRetAt`、`cuPend`、窗逾時；另加「IP 在任一觀察器的 IP 表內」。這些條件不經 `NextWake()`，所以能抓到
   `nextWake()` 的漏寫（O1）。成本是每步十餘次欄位比較。
2. 不變式（每個略過步）：W「`Steps() < 鎖存的 NextWake`」；I1「每次回呼結束時 `p.cuOpen == flight.CUState().InProgress`」。
3. 深度快照雜湊（取樣）：略過步呼叫觀察器前後各取一次，必須相同。
   - 走訪：reflect 走訪 Labels、Flight（含 cuTracker、SHP 快取）、Subtitles、Presenter；指標跟隨並防環；map 依鍵排序後雜湊。
   - 排除：四個簿記欄位 `s.steps`、`s.cur`、`p.steps`、`p.cuOpen`；`func` 欄位；不可變設定（catalog、圖集、`resolve`、`cfg`、窗表
     `routines`、`ipMask`）；輸出累積只取長度（各 `log`、`captures`、`windows`、`cuRecs`、`stale`、`Skips`、`Subs`、`redeliv`、`clipped`、
     Presenter 合成快取 `cache`）；暫存緩衝（`p.vga`、`p.tmp`、`p.bufC`）。審查列舉的 `lastSig`、`lastVerify`、Labels `cur`／`paired`／
     `pairSP`、Subtitles `obj`／`prevKey`／`prevAcc`、Flight `pend`、Presenter `stack`／`lastRetWin`／`lastRet`／`snapValid`／`fbDesc`、
     `unattr`（取鍵集合與計數）都在範圍內。
   - 單元測試：每個略過步都取。
   - 長測試：分層取樣。觸發 IP 步、`Force` 步、`NextWake` 鎖存之後 64 步內的略過步全取；其餘以固定種子的隨機取樣。取樣率在實作後先量
     單次快照成本再定，目標是稽核長測試的總時間不超過逐步模式的 3 倍；種子與取樣率寫入稽核輸出。
- 失敗訊息：步驟 n、IP、失敗的層與條件（oracle 的條件名、不變式名，或雜湊不同的觀察器與第一個不同的欄位路徑）。

### 5.3 負對照

每條都必須讓 §5.1、§5.2 或指定的單元測試失敗：

| 編號 | 突變 | 預期失敗 |
|---|---|---|
| S1 | 集合拿掉 `03CD`（Flight puts 返回） | flight Q 收據（可見集合、FlightCommit）不同 |
| S2 | 集合拿掉 `0000` | saveload 或 labelframe 收據不同 |
| S3′ | 拿掉 SBDMA 之後的 `Force` | subtitle c11 出現 `SkipTiming`，收據不同 |
| S4 | Flight `pendOn` 不進 `nextWake()` | 判別量是 `flight-log.tsv`（`flight_long_test.go:367-370` 寫出）的 "redeliv" 列，其 Detail 含步數（`flight_redeliver.go:73-74`）：漏喚醒時撤銷改在下一個觀察點發生，Detail 的步數不同。前提：Q 或 R 的 `flight-log.tsv` 至少有一列 "redeliv"，且在紀錄上限 4,096 筆之內；不成立就以合成送達的單元測試代替。稽核 oracle（`pendOn`）同時報錯 |
| S5 | Presenter 窗逾時不進 `nextWake()` | 期限單元測試 T3 失敗；稽核 W 報錯 |
| S6 | Labels 簽章期限不進 `nextWake()` | 期限單元測試 T1 失敗；稽核 W 報錯 |
| S6′ | Flight 簽章期限不進 `nextWake()` | 期限單元測試 T2 失敗 |
| S6″ | Subtitles 等待逾時不進 `nextWake()` | 期限單元測試 T4 失敗 |
| S7 | `FrameAt` 不呼叫 | flight `FrameSteps`、subtitle `frameT`、014 `CaptureSteps` 不同 |
| S8 | Frame 之後不設 `Force` | 期限單元測試 T2（升格改 `lastSig` 後的期限）失敗；§5.1 第 4 條切分 A/B 的 N1 判定步數不同 |
| S9 | `Presenter.Observe` 不再覆寫 `cuOpen`，且 `onCU` 的 timeout 分支不設 false | 稽核 I1 報錯 |
| S12 | Presenter `afterRet` 不進 `nextWake()` | 稽核 oracle（`afterRet`）報錯；飛行 Q、R 的 014 收據中 cu-return 取像與 `CUResidue` 紀錄的步數不同 |
| S13 | Presenter `recheck` 不進 `nextWake()` | 稽核 oracle（`recheck`）報錯；014 收據模式的 `RecaptureChecked`／`RecaptureSkipped` 計數不同 |
| S14 | `ShouldObserve` 拿掉 `Exited` 條件 | session 單元測試：合成程式在第 n 步結束（int 21h AH=4Ch），稀疏模式回報的停止步數必須與逐步模式相同，突變後不同 |
| S15 | 拿掉 `runObserved` 開始時的 `Force` | session 單元測試：合成觀察器在兩次 `Advance` 之間由外部改變狀態使 `NextWake` 變小（例：設一個「下一步即到期」的期限），稀疏模式的下一次 `BeforeStep` 步數必須與逐步模式相同，突變後延後 |
| S16 | 拿掉 panic（`guard`、`VideoWrite`、Frame 包裝）之後的 `Force` | session 單元測試：合成觀察器在 `VideoWrite` 內 panic，`Advance` 回報觀察器錯誤時的 `MachineStepsAfter` 必須與逐步模式相同，突變後延後到下一個觀察點 |
| S17 | 拿掉 viewMisuse 之後的 `Force` | session 單元測試：合成觀察器保留 `StepView` 並在 `VideoWrite` 內使用，回報步數同 S16 |

期限單元測試（合成視圖，以 `RunUntilSparse` 或呼叫同一個 `ShouldObserve` 的測試驅動推進，不另寫述詞；每條都有對應的必敗負對照 S5、S6、S6′、S6″）：

- T1 Labels：可見集合非空、base 已知；在 `lastSig + 5,000,000` 那一步讓簽章不符。斷言該步清空、`gen++`、"sig" 紀錄的步數等於期限步。
- T2 Flight：先以 Frame 升格（`lastSig` = Frame 步），再在期限步讓簽章不符；Frame 到期限之間合成視圖的 IP 一律不在觸發集合內
  （否則中途的觸發步會順帶完成檢查，S6′、S8 抓不到）。斷言停用與清空的步數等於期限步。
- T3 Presenter：開 blit 窗後不返回；斷言 `WindowTimeout` 紀錄的步數等於 `entry + 200,000`。
- T4 Subtitles：SBDMA complete 使字幕條進入 `SubWaiting`；在 `LastDue + T + 1` 之前的最後一步 Frame 看到仍在等待，期限步之後的 Frame
  看到結束；斷言 `ReasonTimeout` 與兩次 Frame 的 `current()` 結果。
- T1、T3、T4 同 T2：設定事件與期限步之間，合成視圖的 IP 一律不在觸發集合內，且沒有 Frame 以外的回呼。

### 5.4 效能

確定性指標（驗收條件）：

1. 分期 (a) 呼叫率依 §5.1 第 3 條的原四類；分期 (b) 擴為 §9.7.3 的六類。比率為 (Observed−Forced)/總步數。
2. 分期 (a) 原成本條件：飛行 Q 全開 1.5 億步，`runObserved` 稀疏 `observe` 閉包的 cum 佔比 ≤ 5%（現況 `func10` 36.0%）。5% 是暫定門檻，實作後先量
   一次再調；調整要寫進 WORKLOG 與本節，附量測輸出路徑。(b) 依較新的 014 D44 與 §9.7.3，pprof 只報告，仍保留 ≤1% 呼叫率硬閘門。

牆鐘比較（報告用，不作為通過條件）：

- 同一容器、同一 `--cpuset-cpus`（P 核），稀疏與逐步兩個二進位交錯跑 N = 5 輪，比 CPU 秒（`getrusage`）的中位數。
- 每輪前後記 `uptime`；1 分鐘負載高於 cpuset 核數的輪作廢並補跑。
- Present 收據模式（`cfg.Receipt`）多做重讀抽查與紀錄，量得的全開時間偏高，報告要註明。
- 計畫 002 的「全開即時倍率 ≥ 1.2」不由本規格單獨保證：現況全開與只跑模擬差約 1.4 到 2.4 倍，本規格只消除觀察成本；到 1.2 倍仍需
  CPU 核心加速（另案）。倍率換算用 `IRQ0Every × PITHz()`，須在規格 461 實作之後。

### 5.5 介面測試

- `TriggerIPs`、`NextWake`、`FrameAt` 的 `testing.AllocsPerRun` 為 0；既有 `TestSessionObserverBeforeStepAllocs` 不變。
- 編譯期斷言與具體型別斷言（§3.6）。
- session 單元測試（合成觀察器）：觸發 IP、期限、`NextWake` 回 0、Frame 與 SBDMA 之後的 `Force`、`Exited`、panic 與 viewMisuse 的回報步數，
  各與逐步模式的同一觀察器比對呼叫序列。

## 6. 量測與未知

### 6.1 觸發率（已量，2026-10-23）

量法：`workplace/perf100/obs/`。`patch/obs_count.go` 在 `SessionObserver.BeforeStep` 開頭判斷「稀疏模式這一步會不會觀察」，只計數；
`build-obs.sh` 建置（dosgolem-sc `395b679` 加 perf100 掛鉤），`run-all.sh` 依序跑四個情境，全開，各 3 億步。輸出
`obs/out/{menu,dlg,flq,flr}/test.log`。計數不含 §3.3 的 `Force`（Frame 之後每 165,000 步一次、每次 Advance 一次）。

兩種濾法：只比 IP（本版）；IP＋CS＋狀態（分期 (b) 的上界：每個 IP 只在分支真的會做事時觸發）。

| 情境（狀態） | 只比 IP 的觀察步 | 比率 | IP＋CS＋狀態 | 比率 | 非 IP 喚醒 |
|---|---|---|---|---|---|
| 主選單 m242 | 794,997 | 0.265% | 115 | 0.00004% | 1 |
| 基地對話 s500＋c11 | 214,597 | 0.072% | 3,721 | 0.0012% | 78 |
| 飛行 Q s800 | 1,739,182 | 0.580% | 314,437 | 0.105% | 5,684 |
| 飛行 R s760 | 1,135,102 | 0.378% | 166,528 | 0.056% | 3,356 |

註：「非 IP 喚醒」用第 1 版的條件計數，含 `sub-pending`（死條件，實際由 `Force` 喚醒，結果相同）。

命中最多的 IP（只比 IP）：

| IP | 主選單 | 對話 | 飛行 Q | 飛行 R | 說明 |
|---|---|---|---|---|---|
| `0182` | 0.176% | 0.036% | 0.034% | 0.010% | 橫幅入口；主選單時 Flight base 未啟用，全為誤觸 |
| `03D3` | 0.088% | 0.000% | 0.009% | 0.005% | OPTTEST puts 返回；主選單時 `cur` 為 nil，全為誤觸 |
| `00A5` | 0.000% | 0.000% | 0.170% | 0.065% | CU 一般入口；限制後為 0 |
| `00A7` | 0.001% | 0.032% | 0.103% | 0.059% | 同上，返回 |
| `02B2` | 0 | 0.000% | 0.093% | 0.046% | OPTTEST putc；飛行中 `cur` 為 nil |
| `02AC` | 0.000% | 0.000% | 0.049% | 0.015% | 飛行 putc；限制後 0.0014%、0.0009% |
| `0345` | 0.000% | 0.000% | 0.030% | 0.022% | 飛行 puts 入口，不限 CS |
| `034B` | 0.000% | 0.000% | 0.028% | 0.009% | OPTTEST puts 入口，不限 CS |
| `0002` | 0.000% | 0.000% | 0.008% | 0.010% | MENU shape，不限 CS |
| `0000` | 0.000% | 0.001% | 0.001% | 0.000% | |

`0000`、`0002` 的 CS 分佈：`0000` 每個情境只出現在 2 到 7 個 CS，最多一個 CS 1,671 次／3 億步（對話 `3CD8`，即 Labels base 0241
的 `base+3A97h`），總量不到 0.001%。`0002` 出現在 3 到 12 個 CS，最多一個 CS 23,935 次（飛行 R `22E9`），總量 0.008% 到 0.010%。
沒有觀察到 3D 內迴圈落在集合位移上的情形（任一 IP 最高 0.176%）。

成本：觀察步的單步成本高於全體平均（觀察步會進分支讀記憶體、比簽章、取像），不能以平均每步成本乘觀察比率推算。實作後以 §5.4
的 pprof cum 佔比直接量。原版「(b) 暫不做」的取捨已由 014 (d) 新窗表的超標量測推翻；P-b1／P-b2 顯示必須採 CS 加簽章區段，見 §9.2。舊表保留為 (a) 的歷史量測。

附帶觀察：這次主機負載較低，加計數掛鉤的全開倍率（以 `PITDiv` 換算，2 核）為主選單 0.457、對話 0.408、飛行 Q 0.605、飛行 R 0.369，
比 report.md 的重負載數字高；單次量測，不取代 §5.4。

### 6.2 未知

- 單次喚醒在中斷送達時仍是下一步：送達發生在 `Step` 內（`tick`），`m.Steps++` 照常執行（`machine.go:1765-1790`、`1666`）。強推論；
  §5.5 的 session 單元測試覆蓋。
- 前端每格 `Advance` 一次（假說，未讀前端程式碼）：每回合第一步有 `Force`，若成立，每格多一次觀察，對成本無影響。

### 6.3 已裁決

- N1：接受 §3.4 等價論證，條件是 §5.1 第 4 條（定向與隨機切分）通過；否則 N1 強制逐步。
- 程式結束的停止點：`StepFilter.Exited` 每步指標檢查。
- 分期 (b)：READY，契約見 §9；尚未實作。
- 預設開：須 (b)、014 (d) 與 §9.7.3 全部通過，才走 §7 第 5 步；目前維持 false。

## 7. 實作順序

1. machine `StepFilter`、`ShouldObserve`、`RunUntilSparse`；session `SparseObserver`、`IPSet`、`Config.SparseObserve`、§3.3 的 `Force` 接線；
   session 單元測試與 S14 到 S17。
2. 陸空戰將各觀察器的 IP 表與 `nextWake()`、`SessionObserver` 的三個方法、`ReceiptConfig.sparse` 與 `DOSGOLEM_SC_SPARSE=1`（預設仍關）；
   嵌入包裝型別盤點（§3.6）。
3. 稽核（§5.2，先量單次快照成本定取樣率）與期限單元測試 T1 到 T4；§5.3 負對照。
4. §5.1 全量對拍（含定向與隨機切分）；§5.4 量測與門檻調整；238、452、454 修訂。
5. §5、分期 (b)、014 (d) 與本規格 §9.7.3 的前置條件全部通過之後，才把 `SessionConfig` 翻成預設稀疏，`DOSGOLEM_SC_SPARSE=0` 改為關閉開關；翻轉後 §5.1 第 1、2 條再跑一次。

## 8. 2026-10-24 接手驗證紀錄

基準為 `b2896ee` 加未提交的 460／461 實作。工具為 `dosgolem-go-ebiten:1.24-r1`、Go 1.24.13、Linux amd64。
收據根目錄為陸空戰將 repo `workplace/impl460/out/codex-validation-r1/`；輸入檔名與 SHA-256 在 `input-sha256.json`，1,024 份 Go 輸入的 SHA-256 在 `source-sha256.json`。
`source-manifest-check.json` 證實正式 Go 來源與診斷建立時相同；其後只增加 S9 單元測試。
步數使用 dosgolem `machine.Steps`，稽核的 IP 使用 `StepView.IP()`，不表示線性 PC。

| 項目 | 本輪結果 | 證據 |
|---|---|---|
| 全庫回歸 | 47 個套件 PASS、`go vet ./...` PASS；GUI 套件在 Xvfb 內測試 | `short/gotest-all.log`、`short/vet.log` |
| Q 定向切分 | 兩模式 2,367 個收據檔、138 個檢查點相同 | `compare-flightQ.json` |
| R 固定種子隨機切分 | 兩模式 820 個收據檔、47 個檢查點相同，種子 4601 | `compare-flightR.json` |
| E 的 N1 | 兩模式的定向切分收據相同；各模式切分與不切分也相同 | `compare-expoE-ab.json`、`compare-expoE-{dense,sparse}-splits.json` |
| R 的 N1 | 回掃升格會改變狀態的路徑中，兩模式 831 個收據檔相同 | `compare-n1-flightR.json` |
| S8 | E 無差異：E 沒有 Flight，`OnFrame` 只更新 `frameSteps`，不設 `dirty`。R 有 Frame 內的 Flight commit，突變後 3 個曝光檔不同；首筆 N1 判定從 7,601,880,001 延到 7,601,886,086 步 | `negative-S8.json`、`negative-S8-flightR.json`、`negative-details.json`；`exposure.go` 的 Frame 與 Flight commit 回呼 |
| S9 | 新增 `TestSparseCUTimeoutAuditI1`，由 CU 入口與逾時返回建立前提。正向 PASS，S9 突變讓 I1 失敗 | `short/s9-positive.log`、`short/s9-negative.log` |
| S13 | 普通存讀檔收據 457 檔相同；影子稽核在 F、G、I、J、K 共抓到 73 次漏掉 `recheck`，五例均 FAIL | `negative-S13-receipts.json`、`negS13-audit/sparse-audit.txt` |

診斷複本只補讀值，不進正式程式。在各 shot 與停止點比較 `Frames`、`Deliveries()`、全部 SBDMA 回呼事件、退路 `SBDMALog` 與完整 DOS 存檔狀態。
原始 DOS 摘要包含測試暫存路徑，逐次不同。本輪比較只排除明名的 `DOSSHA256` 欄，同時另比 DOS 完整狀態的正規化摘要；只正規化根目錄與其下 handle 路徑，其他欄位不排除。
因此不宣稱原始 `DOSSHA256` 相同。比對工具為陸空戰將 repo `workplace/impl460/codex-compare.py`。

尚未滿足的閘門：

- §5.2 的飛行長測試仍未補足「鎖存後 64 步全取快照」。本輪 S13 稽核只驗述詞，快照窗與隨機率皆為 0，不能當作正向 64 步驗收。
- §5.3 的 S8 不會讓既有合成驅動 T2 失敗：T2 的 Force 由驅動設定。本輪由 session 接線單元測試及 R 的 N1 長測試補證；條文對應仍待審查修訂。
- §5.1 的 DOS 路徑正規化方法尚未回填驗收條文。現有證據保存原始摘要與完整正規化比較結果。
- 預設翻轉仍依較新的陸空戰將交接 `docs/plan/003-handoff-2026-10-24.md`：460 分期 (b)、014 分期 (d) 合入，並通過擴充情境呼叫率閘門後再處理。

N=5 牆鐘交錯量測仍未做，屬 §5.4 的報告項目，不列為本輪通過聲明。


## 9. 分期 (b)：CS 與入口簽章篩選（READY，未實作）

程式基準為 4095084；證據檔案位於陸空戰將 repo 的 workplace/perf100/。草案第 1 版、第二版與兩份審查由該專案交接 003 及 spec460b-draft.md 索引。兩位均無阻擋，修正 CU 逾時、014 節號、注入窗表、聯集掩護與抽選指紋證據限制後整合。

### 9.1 範圍與已有裁決

| 項目 | 契約 |
|---|---|
| 第一層 | 保留 (a) 的 IP 全集；每回合建立一次 |
| 第二層 | 第一層命中後，比 CS 白名單或入口簽章區段 |
| 規則刷新 | 回合開始，以及每次 BeforeStep 成功返回後 |
| 任意 CS 的入口 | 採 (b2)，通常比簽章 Off=8 起的 4 bytes |
| 規則容量 | 每 IP 最多 4 個 CS、5 個簽章；超量保守放行並計數 |
| 每次呼叫改變的條件 | 保留 cur、plq、paired 等條件，不引入狀態鍵快取 |
| 返回 IP | 帶 CS；未知 base 的呈現窗使用開窗與最近返回記錄的 CS |
| 預設開關 | 維持 false；本實作與 014 (d) 全部通過後，才可另行翻轉 |

來源：交接文件「主代理已做的裁決」、014 第 13 版 §3.11、第一版 Q1–Q8，以及 §9.2 的探針。這些是既有路線的具體化，沒有改遊戲規則、資料或玩家操作。

本草案只處理篩選條件、刷新、獨立稽核與驗收。不增加模擬器存檔欄位，不寫原版記憶體，不實作 014 (d)，不把模型探針視為產品驗收。

### 9.2 證據與限制

#### 9.2.1 P-b1／P-b2 結果

來源：`workplace/perf100/pb1/report.md`；原始測試碼 `pb1/src/apps/strikecommander/pb1probe_test.go`，輸入快照 `pb1/src-b2896ee.tar`，原始收據 `pb1/runs/{l,n,h,q,r}/`。

| 情境 | 步數 | (a) 只比 IP | (b1) 加 CS | (b2) Off8 k4 命中步 | (b2) 比率 |
|---|---:|---:|---:|---:|---:|
| L 主選單 | 280,000,001 | 5.1856% | 4.8414% | 1,290 | 0.0005% |
| N MENU 流程 | 200,000,001 | 6.9099% | 3.3092% | 2,698 | 0.0013% |
| H STRIKE 載入 | 460,000,000 | 1.4393% | 0.5051% | 12,658 | 0.0028% |
| Q 飛行 | 1,345,000,000 | 1.8778% | 0.3825% | 367,818 | 0.0273% |
| R 飛行 | 445,000,000 | 2.0208% | 0.2662% | 37,934 | 0.0085% |

- 已證實，限探針樣本：只加 CS 無法讓 L、N 達到 1%。Off8 k4 與 Off0 k12 的命中步數相同，前者讀取較少。
- 完整簽章條件 A／抽選欄位指紋變動 B：L 1290／1290、N 2698／2698、H 12658／12478、Q 367818／367818、R 37934／37934。所有配置「B 發生而 A 或篩選未命中」均為 0。B 只比較探針抽選的欄位、nil 與集合長度，不能捕捉所有內容替換，不等同 460 深度快照。H 的 A 與 B 不相等，不宣稱兩種動作定義完全相同。
- 規則數為 14–27。Off8 k4 的刷新數依序為 1318、2715、12674、368160、38041。
- CS 超過 4 個的次數為 0。IP 0006 在 O、S base 都未知時需要 5 個簽章，故將第一版的 Sig[4] 改為 Sig[5]。
- L、N 熱點 3A5F:000A／000C 的前置不符，已否定「樣本內是同簽章程式庫複本」的假說。沒有建立 CS 排除表；同一段之後仍可能載入新程式。

限制：探針的 (d) 是模型，時間層的逾時、checkPend 與 base 停用未由上述指紋完整覆蓋。此表不是正式呼叫率，也不是 CONFORMED 證據。

#### 9.2.2 簽章與位址基準

程式定位均為 dosgolem-sc 4095084 的 `apps/strikecommander/`，CS:IP 是觀察器使用的暫存器值，不是 IDA 或檔案偏移。

| 項目 | 已證實的讀碼結果 |
|---|---|
| puts | observer.go 的 sigPutsEntry，16 bytes；Off8 起有 4 個已知 bytes |
| shape | dialog.go 的 sigShapeEntry，16 bytes；Off8 起有 4 個已知 bytes |
| plaque | flight.go 的 sigPlaqueHead，32 bytes，起點 0182；完整判定另讀 01DC 的 sigPlaqueLoop。只取 head 的必要子條件是保守超集合 |
| 字幕 | subtitle.go 的 sigConvEntry／sigConvDispatch 各 16 bytes；sigConvFast 僅 9 bytes |
| 位址 | observer.go 的 lin(seg,off) = (uint32(seg)<<4)+uint32(off)，off 為 uint16；完整簽章比較也用此式 |
| 讀取 | session.StepView.Read8 轉呼叫 Machine.Peek8；讀取無副作用 |

sigConvFast 的 Off8 只剩 1 byte，必須縮短為 N=1，不能讀未屬於簽章的 3 bytes。P-b1 五案未觸發這個縮短，須由合成正負測試補足。

### 9.3 介面與保守退路

型別放在 `internal/machine`，session 用別名匯出，避免反向匯入。

```go
type SigPrefix struct {
    Off uint8
    N uint8
    B [8]byte
}
type TriggerRule struct {
    IP uint16
    Any bool
    NCS uint8
    CS [4]uint16
    NSig uint8
    Sig [5]SigPrefix
}
type TriggerRules struct {
    N int
    R [128]TriggerRule
    Overflow int
}
type SparseRuler interface {
    TriggerRules(dst *TriggerRules)
}
```

- TriggerRule 內 Any、CS、Sig 是「或」。每 IP 合併成一條，依 IP 排序。只讀 R[:N] 及各有效長度，尾端舊槽不影響結果。
- TriggerRules 只讀觀察器狀態，不讀機器、StepView 或記憶體。前置的 bytes 是既有完整簽章的常數。
- 通常取 Off=8、N=min(4,len(sig)-8)。若區段含萬用 byte，縮為同一區段中已知的連續必要 bytes；沒有可用 bytes 時改 Any。不可把萬用值轉成固定 byte。
- 同一 IP 的 CS 或 Sig 超量，該 IP 改 Any，Overflow 計數。總 IP 超過 128，整個第二層停用，仍走第一層並計數；不能只丟掉第 129 個 IP。計數須能從稽核或統計讀到。
- 無效 N、NCS、NSig 或零長度前置不得造成 panic 或漏觀察。建構器須產生合法資料；防禦檢查遇到無效規則時保守放行並可診斷。
- 不實作 SparseRuler 的觀察器維持 (a) 行為。第一層必須包含各狀態下第二層所有 IP；缺少第一層 IP 是契約錯誤，測試必須抓到。

StepFilter 增加 Rules、HasRules、IPHits。保留 Force、Wake、Exited、Observed、Forced 的既有意義。ShouldObserve 的順序：

1. Force、steps>=Wake 或 Exited 先放行，完全繞過第二層。
2. 第一層 IP 未命中，略過。
3. 第一層命中計入 IPHits。未啟用第二層時放行。
4. 已啟用時，按 IP 二分搜尋。找不到則略過；找到後依 Any、CS、Sig 判斷。

簽章讀取為 `Peek8((uint32(cs)<<4)+uint32(uint16(ip+Off+i)))`。必須先做 16 位元 offset 環繞，再組位址；不得用 PCLinear、描述元 base 或其他位址空間。即使目前 CPU 在保護模式，也要與觀察器當步讀到的位址一致。此規定只描述既有觀察器契約，不宣稱是 CPU 的實際取指位址。

實作要保留第一層未命中的熱路徑。CS 與記憶體讀取只在第一層命中後進行；第二層可另放不內聯函式。型別與函式的最終參數形式以滿足上述語意和零配置為準。

### 9.4 規則刷新與時間層

每次 runObserved 開始建立 IP 全集、重建第二層並設 Force。每次 BeforeStep 成功返回後，先以新狀態重建第二層並更新 NextWake，再執行下一步。回錯或 panic 時按原有停止流程，不讀失效 StepView。

| 狀態來源 | 為何不會漏觀察 |
|---|---|
| BeforeStep 學 base、停用 base、開關窗、cur／plq／paired 改變 | 同一步成功返回後刷新 |
| Frame、SBDMA、模式設定回呼 | session 既有 Force 保證下一步觀察，再刷新；不能依賴第二層先命中 |
| 回合外 Attach 或改設定 | 回合開始刷新且第一步 Force |
| overlay／程式載入改記憶體 | 任意 CS 入口仍用當步 Peek8，不快取比較結果，不永久排除 CS |
| VideoWrite、一般音訊回呼 | 不得改 TriggerRules 或 NextWake 所依賴的欄位；新增 462 的普通 PCM 回呼同樣遵守 |

時間條件不靠 IP 規則取代：NextWake 沿用既有 checkPend、簽章重驗、字幕等待與呈現窗等期限。到期步先由 Wake 放行，BeforeStep 處理後更新規則。CU 逾時沿用 460 §2.2 的契約，只在既有 CU 觸發 IP 或 Frame 檢查，不新增獨立 deadline。已過期的最近返回 CS 晚移除，只會增加觀察，允許；新增有效 CS 或期限提前不得延後刷新／喚醒。

義務 O1 沿用 460 的期限契約；O2b 是「篩選為假時，觀察器不做簿記以外的事」；O3b 是「非強制回呼不改規則或期限依賴」。不使用狀態鍵最佳化。

### 9.5 各分支的規則

所有下列條件都取自 4095084 的實際分支。Prefix 表示 §9.3 的必要簽章區段，完整簽章仍由觀察器檢查。

| IP／分支 | 第二層規則 | 原始定位 |
|---|---|---|
| 034B 標籤 puts | Prefix(sigPutsEntry)；baseKnown 時另加 CS{base+3942}，保留壞簽章停用 base 的路徑 | observer.go putsEntry |
| 03D3／02B2 標籤返回／putc | cur!=nil 時 CS{base+3942}；不額外要求 baseOK | observer.go putsReturn／putc |
| 0000 標籤 shape／blit | baseKnown&&baseOK 時 CS{base+3A97,base+39FB}；否則 Prefix(sigShapeEntry) | dialog.go shapeEntry；observer.go blitEntry |
| 008B 標籤 blit 返回 | baseKnown&&baseOK&&paired 時 CS{base+39FB} | observer.go blitReturn |
| 0002 MENU shape | Prefix(sigShapeEntry) | dialog.go menuShapeEntry |
| 0345 飛行 puts | Prefix(sigPutsEntry)，任何 base 狀態都保留，供學 base 與重驗 | flight.go putsEntry |
| 02AC／03CD 飛行 putc／返回 | cur!=nil 時 CS{base+5F7A}；不額外要求 baseOK | flight.go putc／putsReturn |
| 0182 橫幅入口 | baseOK 時 Prefix(sigPlaqueHead)，Off 以 0182 為起點 | flight.go plaqueEntry |
| 01F2 橫幅返回 | plq!=nil 時 CS{plq.cs} | flight.go plaqueReturn |
| 00A5／00D6、00A7／00D8、00EE CU／MU | Flight.baseOK&&tracker.baseOK 時 CS{tracker.base+1D3A} | flight.go Observe；cutracker.go Observe |
| 103F／10F3／22BE 字幕 | 各自完整簽章的 Prefix；22BE 使用 Off8 N1 | subtitle.go Observe |

同一 IP 的規則要與呈現層聯集。CS 算式照原分支使用 uint16 環繞，不加「合理段址」推測。

呈現層 (c)：遍歷實際 p.routines，分別對 r.entryIP／r.retIP 建規則。progOPTTEST 只有 labels.Base() 有效時放 CS{base+r.seg}；progAbs 用 CS{r.seg}。預設窗表才是 0000／008B 與 base+39FB，不把預設值硬編進規則建構器。

呈現層 (d) 未實作，本草案先固定下列約束，待其實作時由同一原則產生規則：

| 類別 | 入口 | 返回 |
|---|---|---|
| OPTTEST／STRIKE，base 已知 | CS{base+seg} | 同 CS，包含 ReturnWithoutEntry |
| OPTTEST／STRIKE，base 未知 | Prefix(入口簽章) | 開著的同常式窗入口 CS，以及最近返回表內的 CS |
| MENU | Prefix(入口簽章)；_memcpy_0 僅 MENU 期間放入 | 同上 |

最近返回以 (常式,SS:SP) 記錄，沿用 014 的 200,000 步有效期。未知 base／MENU 返回不因任意 CS 的 IP 命中就新增 ReturnWithoutEntry；必須由開窗或最近返回記錄確認 CS。已知 base 的返回仍保留其無開窗診斷。此契約須與 014 (d) 正式規格同步，不可只在 filter 偷偷略掉原觀察器會做的診斷。

### 9.6 獨立稽核與正確性論證

460 的 L1 允許多餘觀察只改簿記。因此核心條件是無漏觀察，不要求所有保守放行都產生動作。等價證明仍須同時滿足 Wake、回呼與回合切分契約。

| 稽核項目 | 要求 |
|---|---|
| rawMayAct | 手寫獨立分支述詞，使用原始 CS 條件和完整簽章，不呼叫產品 TriggerRules／Match 或 Prefix 建構器 |
| 時間 oracle | 直接讀原始 deadline／pending 狀態，不以被測 NextWake 自己作期望值 |
| 影子 filter | 每回合與每個影子觀察步後刷新，與 session 同時點 |
| 深度快照 | 沿用 460 的 64 步窗口與非簿記快照，不能降為 0 或 4 來通過；保留既有正樣本覆蓋要求 |
| 規則不變式 R | 略過步抽樣重建規則，比較有效 N 條及有效 CS／Sig；忽略尾端舊槽、padding、診斷計數。發現會擴大必要集合的外部變更即失敗 |
| 定向切分事件 | 增加 rules、base，保留 IP、Wake、Frame、CU 等事件 |

不變式 R 可允許已到期記錄仍在快取造成的保守超集合，但必須逐項證明只是晚移除，不得把新增必要條件的落差放過。比較器不能直接忽略所有差異。

A/B 的 MachineDigest 各欄、完整 DOS 狀態與輸出都按原閘門比較。依 014 §4.4，兩側使用同一個固定 SaveRoot 路徑，逐次還原相同 fixture 與時間戳，再依序執行。不得只把 DOS 路徑正規化後宣稱 raw 雜湊相同；舊輪的正規化比較仍只是補充診斷。

### 9.7 測試

#### 9.7.1 正向

| 編號 | 內容 |
|---|---|
| U-b1 | Match 的缺 IP、Any、CS、Prefix 正負、非零 Off、16 位元環繞、舊槽、空前置與容量退路 |
| U-b2 | 無 SparseRuler 時與 (a) 判定一致；Force／Wake／Exited 優先；第一層未命中不讀記憶體 |
| U-b3 | 各表列分支在各種 base／cur／plq／paired 狀態，規則拒絕時非簿記快照不變；每個必要分支有完整簽章正樣本。保守 Prefix／Any 不要求反向等價 |
| U-b4 | 觀察步內改 base／開窗後下一步規則更新；回錯不刷新；Frame／SBDMA／模式回呼走真實 session Force 接線 |
| U-b5 | 回合 n／n+1 切分、Attach 與不切分輸出相同，允許額外的回合首 Force |
| U-b6 | 全合成狀態的規則 IP 均在第一層，正常輸入 Overflow=0；另造各級超量驗證只增加觀察 |
| U-b7 | 建構、Match、ShouldObserve 零配置；既有 BeforeStep 配置測試不退步 |
| U-b8 | 橫幅以入口保存的 plq.cs 返回；sigConvFast Off8 N1 的正負測試；改後面非簽章 bytes 不改結果 |

#### 9.7.2 負對照

| 編號 | 突變 | 預期被抓到 |
|---|---|---|
| S20 | 全部 Any | L／N／H 等呼叫率超標，A/B 仍相同 |
| S21 | 任意 CS 的 Sig 改 Any | L≥4%、N>1%；不能以模型結果代替產品測試 |
| S22 | 0345 在 base 未知時誤帶 base CS | Q 漏學 base，獨立 rawMayAct 與輸出比較失敗 |
| S23 | 034B 拿掉已知 base 的 CS 退路 | 壞簽章停用 base 的合成測試與稽核失敗 |
| S24 | 只在回合開始刷新 | Q 漏 putc／返回；規則不變式 R 或切分比較失敗 |
| S25 | 0345 Prefix 任一必要 byte 改錯 | 完整簽章 oracle 抓漏觀察 |
| S26 | Prefix 改用 PCLinear | 非同一線性位址的保護模式合成測試失敗 |
| S27 | 規則依賴 VideoWrite 改動的 win.wrote | 合成序列的不變式 R 失敗 |
| S28 | (d) MENU 返回漏開窗 CS | N 窗無法正常關閉、逾時或 A/B 不同，稽核抓漏返回 |

S22、S25 使用 Present=off，或明確突變合併後的全部 0345 Prefix，避免 (d) 的 STRIKE puts 同 IP 規則掩護 Flight 來源突變。收據必須記錄採用的方式。

沿用 S1–S19。原 S8 的實際 Frame Force 接線使用 session 測試與 R 的 N1 突變驗證；E 沒有相關前提，手動設 Force 的單元測試也不單獨代表真接線已驗證。S13 一般輸出可不變，須由影子 oracle 抓到漏重驗。

#### 9.7.3 產品呼叫率與翻預設

- 呼叫率為 (Observed−Forced)/總步數。另報 IPHits、Prefix 讀取數、Overflow、各原因 Force 與刷新數，避免把第二層成本藏在呼叫率之外。
- 正式情境是原四案加已裁決 N、H，共六類：主選單 L、基地對話 D、MENU N、STRIKE 載入 H、飛行 Q、飛行 R。D 的兩份既有 fixture 不相同，原 460 的 s500+c11 是閘門；014 的 p539/s500b 可另記。不得因探針只有五案而漏掉 D。
- 原四案沿用各 3 億步要求。N、H 沿用已定義的完整路徑與停止點，記錄實際步數，不硬延長到會越過路徑的 3 億步。六類在 Present=on、(d) 完整窗表下各須≤1%，另報 Present=off 對照。
- 倍率 2、3；Present 開、關；全部既有 A/B、切分、原版 CPU／記憶體／indexed／palette／DOS 雜湊、64 步稽核正樣本與全部負對照須通過。
- 014 (d) 未合入時可驗證 (c)，但不得翻 sparseDefault。S28 或完整窗表呼叫率缺測也不得翻。
- pprof 與 N=5 wall-clock 只報告，不取代正確性與≤1% 閘門，也不拿舊 (a) 的成本估計當 (b) 測量結果。
- 本輪不翻預設。未來全部通過後另做開關變更，並重跑原規格要求的驗證。
