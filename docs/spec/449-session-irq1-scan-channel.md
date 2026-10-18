# 449：session 的 IRQ1 掃描碼送鍵通道

狀態：**READY**（第 4 版，依三輪審查修訂），已實作（§5）
日期：2026-10-18
前置：[`197-key-hold-and-typematic`](197-key-hold-and-typematic.md)、[`199-live-session-api`](199-live-session-api.md)、
[`238-session-step-observer-draft`](238-session-step-observer-draft.md)、[`447-bios-int09-int15-keyboard-intercept`](447-bios-int09-int15-keyboard-intercept.md)、
[`448-session-config-legacy-pm-profile`](448-session-config-legacy-pm-profile.md)
修訂：199 §3.3（`KeyUp` 的取消範圍，§2.3）

dosgolem 的程式位置以 commit `2df4395` 為準。推論等級：已證實、強推論、假說、未知。

用語：

| 用語 | 意義 |
|---|---|
| 按下碼 | `Down` 排的第一個按下事件 |
| 重複碼 | 按住期間 typematic 排的按下事件 |
| make 事件 | 按下碼與重複碼的合稱 |
| 放開碼 | `Up` 排的放開事件 |
| 批次 | 一次 `Deliver` 的 `CapturedUpdate` |
| 回合 | 一次 `Deliver` 加上隨後的一次 `Advance`；回合邊界是兩次 `Advance` 之間的 `m.Steps` |
| 強制放開 | §2.4 的 `ReleaseHeldKeys` |
| 按住狀態 | `Machine` 內記錄哪些鍵按著及其 `downStep`、`delay`、`period`、`next` 的欄位 |

---

## 1. 為什麼要做

`session.Owner.Deliver` 只有一條送鍵路：把 `CapturedUpdate.BIOSKeys` 直接放進 BIOS 緩衝（`session/owner.go` 566 行 `o.dos.PushKey`），不送硬體 IRQ1，也不動埠 `60h`。《Strike Commander》的 OPTTEST 與 STRIKE 只從 `int 15h AH=4Fh` 鍵盤攔截收鍵（447 §1），這個呼叫只由 IRQ1 觸發的 BIOS `int 09h` 發出，所以走 session 的前端操作不了這兩支程式（已證實：程式碼；448 開頭第 27 行）。

`cmd/probe` 的 `-hold` 已經這樣送鍵，既有的試玩收據都靠它（`strike_commander` 的 `docs/re/010` §2 起飛）。它把掃描碼以絕對步數排進機器的 `timedKeys`（`internal/machine/keyhold.go` 30 到 54 行），不受 `KeyEvery` 節流；按住期間照 BIOS 預設 typematic 排重複碼。

### 1.1 一條通道就夠

`KbdIntercept` 開啟時，IRQ1 送進來的鍵經 BIOS 客體碼（447 §2.2）呼叫 `int 15h AH=4Fh`；處理常式以 `CF=1` chain 回去時，客體碼另把鍵放進 BDA 緩衝（447 §2.3、測試第 136 行）。所以讀 `int 16h` 的程式與收 `int 15h` 的程式都由同一條 IRQ1 通道餵到。

實測（已證實）：載入 `strike_commander` 的 `workplace/scratch/p472/s100.state`（冷開機第 10 億步，MENU 畫面），

```text
-kbd-int15 -load-state /scratch/s100.state -steps 1100000000 -hold 50@1010000000+3000000,1C@1040000000+3000000
```

- `probe449m-on.log`：`KeyIRQs=4 KeyStalls=0 KbdIntercepts=4 KbdTranslates=4`。
- 按鍵去向：第 1010001295 步 `int16-AH00-bda 00`（下方向鍵），第 1040001887 步 `int16-AH00-bda 0D`（Enter）。
- 第 10.3 億步的畫面與不送鍵的對照（`probe449m-off.log`）不同。

MENU 從 BDA 經 `int 16h` 讀鍵，鍵由 IRQ1 送入。本規格因此只加 IRQ1 通道，不依程式階段選通道。

## 2. 規則

### 2.1 輸入與拒絕

`session.CapturedUpdate` 加一個欄位：

```go
// ScanEvents 是硬體鍵盤事件（按下／放開），依序經 IRQ1 送出（docs/spec/449）。
ScanEvents []ScanEvent
```

`session.ScanEvent` 是 session 自己的值型別 `{Scan uint8; Break bool}`，不外露 `machine.KeyEvent`。`Scan` 是不含 bit7 的掃描碼，`Break` 為真表示放開。

`prepare` 在下列情形回錯誤。拒絕走既有的 `frontendFaultInputReceipt`（`owner.go` 348–351 行）：owner 進 `PhaseFailed` 並關閉，之後 `View`、`Deliver`、`Advance` 一律被拒。拒絕發生在 commit 之前，不排任何鍵、不動 BDA。這與既有拒絕路徑的代價相同：送錯掃描碼是前端的程式錯誤，不是玩家輸入。

| 條件 | 理由 |
|---|---|
| `ScanEvents` 與 `BIOSKeys` 同時非空 | `KbdIntercept` 開啟時 IRQ1 通道的鍵也會進 BDA 緩衝（447 第 71 行），同一批兩條都送，程式會讀到兩次 |
| 任一 `Scan` 為 `0` 或 `>= 80h` | `0` 不是鍵；bit7 由 `Break` 表達。`E0h`、`E1h` 等前綴不在本規格（§2.7） |

拒絕條件在 paused 批次也照查，壞輸入不因面板開著而被默默吃掉。

`ScanEvents` 只有在 `Config.KbdIntercept` 為真，或程式自己裝了 `int 09h` 時才送得到。`KbdIntercept` 關閉且 `IVT[09h]` 在 stub 上時，事件停在 `timedKeys`（`keyhold.go` 103 行），由 §2.6 的 `KeyStalls` 呈現。

跨批次的雙送（前一批 `BIOSKeys`、後一批 `ScanEvents` 送同一個鍵）不在 owner 層擋；前端對同一支程式固定用一條通道（§4）。

### 2.2 按住狀態放在 `Machine`

按住狀態是 `Machine` 的私有欄位，以固定大小陣列儲存（128 個鍵各一組 `downStep`、`delay`、`period`、`next`，加上 `[2]uint64` 按著遮罩），不用 map：`Snapshot` 是值複製，不必深拷貝；迭代順序固定；沒有 nil 與空 map 的差別（`oracle/step_guard_test.go` 38 行對 `Snapshot()` 做 `reflect.DeepEqual`）。按住狀態隨 `Snapshot`／`Restore`（`internal/machine/snapshot.go` 127、223 行）一起保存與還原，與 `timedKeys` 一致（`LoadState` 例外，見下）。`oracle` 與 `session` 都經下列方法操作，不得出現第三份實作：

| 方法 | 行為 |
|---|---|
| `HeldKeyDown(scan) bool` | `scan` 為 `0` 或 `>= 80h` 回 false，不排程。已按著就不動，回 false。否則 `ScheduleKey(m.Steps, scan, false)`，記下該鍵的 `downStep = m.Steps`、`delay`、`period`、`next = m.Steps + delay`，回 true |
| `HeldKeyUp(scan) bool` | `scan` 為 `0` 或 `>= 80h` 回 false，不排程。沒按著就不動，回 false。否則取消該鍵 `Step >= max(m.Steps, downStep+1)` 的 make 事件（§2.3），`ScheduleKey(m.Steps, scan, true)`，清除按住紀錄，回 true |
| `ScheduleHeldRepeats(n)` | 依掃描碼由小到大，對每個按著且 `period > 0` 的鍵，把 `next <= m.Steps + n` 的重複碼排進去並推進 `next` |
| `ReleaseHeldKeys() int` | 依掃描碼由小到大對每個按著的鍵做 `HeldKeyUp`，回個數 |
| `HeldKeys() []uint8` | 按著的鍵，依掃描碼排序 |
| `HeldMask() [2]uint64` | 按著的鍵的位元遮罩（bit `scan` 立起表示按著），給收據用 |

- `delay = uint64(TypematicDelayMS/1000 × InstructionsPerSecond())`、`period = uint64(InstructionsPerSecond() / TypematicRate)`，與 `HoldKey` 同式（`keyhold.go` 46–48 行），在 `HeldKeyDown` 時取一次存起來。之後機器速度改變不影響已按著的鍵，與 `HoldKey` 排程時一次算好的行為相同。
- `period` 為 0 時不排重複碼，與 `HoldKey`（49 行的 `period > 0`）相同，也避免 `next += period` 不前進。現行 oracle 在 `period` 為 0 時改成 1（`live.go` 89–92 行）。`period` 為 0 需要 `InstructionsPerSecond` 小於 10.9：指令數時鐘下它是 `IRQ0Base × PITBaseHz / 17000`，`IRQ0Base` 為 0 時改用預設值（`dosbox_cycles.go` 44–53 行），最小也有約 70；週期時鐘下是 `CPUHz`。這個分支在現有 setter 下不可達，不列測試，也不算對 199 的修訂（§4）。`period > 0` 時 `delay` 至少是 5，所以重複碼一定晚於按下碼。
- 按下 `0` 或 `>= 80h` 回 false 是行為改變：現行 `oracle.KeyDown` 接受任何 `uint8`（`live.go` 50–61 行，map）。`E0h` 前綴本來就不能經 `ScheduleKey` 正確送出（`Scan` 會被當成帶 bit7），不影響既有呼叫者。
- `oracle.KeyDown`、`KeyUp`、`HeldKeys` 與 `scheduleRepeats`（`oracle/live.go` 50 到 100 行）改為委派給上列方法，`oracle.held` 欄位刪除。`Machine.LoadState`（`internal/machine/state.go` 254 行，`state.Load` 經它還原機器）在所有驗證通過之後的提交點（420 行 `return nil` 之前）清空按住狀態，gob 格式不變；讀檔失敗時機器維持原狀（270–320 行的註解），按住狀態也不變。理由：`LoadState` 會把 `Steps` 設成任意值，按住紀錄的 `next` 若落後，下次 `ScheduleHeldRepeats` 會一次補排大量過期重複碼；放在 `LoadState` 才能涵蓋所有讀檔入口，包括之後的 session 讀檔。`oracle.LoadStateFile` 原本的 `o.held = nil`（108 行）因此刪除。`LoadState` 不清 `timedKeys`，這是現行行為，本規格刻意保留；所以讀檔之後按住狀態為空，而舊時間線已排的事件仍會送出，兩者在這一點不一致。`LoadState` 也會換掉 `CPUHz`（362 行），清空 `delay`、`period` 因此是合理的。`LoadState` 的呼叫者（`internal/state/state.go` 93 行，經它進入 `oracle`、`cmd/probe`、`cmd/shots`、`cmd/clickgrid`、`cmd/step`、`cmd/buckrogers-*`）都不使用按住狀態，`HoldKey` 不經它，清空沒有副作用。`oracle.Restore` 經 `m.Restore` 一併還原按住狀態（現行實作不還原 `o.held`，還原後兩者可能不一致）。

session 的 `InstructionsPerSecond()` 在回合間不變：`SetClockPercent` 只在 `New` 呼叫（`owner.go` 302–306 行），`SetDOSBoxCycles` 沒有經 session 公開。oracle 的 `SetDOSBoxCycles` 是公開方法（`live.go` 20 行），可以在按住期間改速度，已按著的鍵沿用按下時的 `delay`、`period`。

### 2.3 與 `HoldKey` 逐步等價

定義：在 `m.Steps == a` 的回合邊界 `HeldKeyDown`，在 `m.Steps == a + d` 的回合邊界 `HeldKeyUp`，期間每次 `Advance` 先 `ScheduleHeldRepeats(budget)`。之後機器的 `timedKeys` 必須與 `HoldKey(scan, a, d, true)` 產生的 `(Step, Scan, Break)` 序列相同。

- `HoldKey` 的重複碼條件是 `t < end`（49 行），放開碼在 `end`（53 行）。
- 現行 `oracle.KeyUp` 用 `CancelTimedKeys(scan, now)`，只刪 `Step > now`（62 行）。`Step == now` 的重複碼會留下，排在放開碼前面，比 `HoldKey` 多一次 IRQ1。所以要刪 `Step >= now`。
- 但無條件刪 `Step >= now` 會把同一步的按下碼也刪掉：同一批 `[按下 k, 放開 k]`，或 oracle `ActionTap` 的 `MS` 為 0（`oracle/actions.go` 236–240 行）時，按下碼與放開碼同在 `now`，只剩孤立的放開碼。下限因此是 `max(now, downStep+1)`。重複碼一定在 `downStep + delay` 之後（§2.2，`delay` 至少 5），不會與按下碼混淆。
- `ScheduleHeldRepeats` 的上界用 `next <= m.Steps + n`（含）。落在回合邊界 B 的重複碼因此在前一回合就排好，`seq` 早於邊界 B 那一批新排的事件。`HoldKey` 一次排好整組，同樣是先排的鍵在前。用 `<` 的話，該重複碼要到邊界 B 的 `Advance` 才排，排在同批新按下的另一個鍵之後，兩鍵次序與 `HoldKey` 相反；單鍵時兩者的最終序列相同，只有回合中的 `ScanPendingAfter` 不同（§3.1 第 2、5 項）。
- 實作：新增 `CancelTimedKeysFrom(scan, fromStep)`，刪該鍵 `Step >= fromStep` 的 make 事件；既有 `CancelTimedKeys` 語意不變。
- 同一批的多個事件都排在同一個 `m.Steps`。`ScheduleKey` 以 `seq` 保持順序（30–39 行），`timedKeyTick` 每步最多送一個，`IF` 關或 IRQ1 遮蔽時等待（96–118 行），`CPU.Segmented()` 時不執行（`machine.go` 1498–1502 行，434 §5.3），而且在 `Step` 內 `m.Steps++` 之前執行，所以排在 `m.Steps == N` 的事件在第 N+1 道指令之前就能送出，與 probe 一開始就排好全部事件的時機相同。這就是送出順序與間距的定義，session 層不另加間隔。
- 等價的前提是不同鍵的事件不落在同一步。兩個按著的鍵在同一步重複時，`HoldKey` 依各組加入的先後排序，`ScheduleHeldRepeats` 依掃描碼排序，次序不同（§4）。

oracle `KeyUp` 的結果在 `Step == now` 邊界改變，修訂 199 §3.3；實作時同步改 `docs/spec/199` 第 50 行一帶的取消範圍敘述。既有測試 `TestKeyDownUpTypematicMatchesHoldKey`（`oracle/live_internal_test.go` 73 行）容許差 ±1，改後仍成立。oracle 的 `RunCycles(n)` 以 cycles 為單位呼叫 `scheduleRepeats(n)`（`live.go` 32–44 行），會多排到 `n` 個 cycle 之後的重複碼；多排的部分在放開時被取消，結果不變，維持現狀。

### 2.4 暫停、失焦與強制放開

既有規則「paused 批次不碰 DOS」（`owner.go` 489–492 行）不能照搬到 `ScanEvents`：面板開著時前端送的放開碼若被丟掉，鍵就卡在按下，面板關掉後遊戲仍以為按著。

規則（`deliveredPlan` 新增 `focusLost` 欄位，由 `prepare` 從 `update.FocusLost` 帶入）：

1. 非 paused、非 `FocusLost` 的批次：依序 commit 全部 `ScanEvents`。
2. 非 paused 的 `FocusLost` 批次：只 commit 其中的放開事件，按下事件忽略，再 `ReleaseHeldKeys()`。先按下再強制放開會讓遊戲看到同一步的一次多餘按鍵。
3. paused 批次：`ScanEvents` 全部不 commit，再 `ReleaseHeldKeys()`。`FocusLost` 與 paused 同時成立時照本條。

順序在滑鼠、面板與 `BIOSKeys` 之後。`prepare` 把 `ScanEvents` 複製進 `deliveredPlan`，與 `biosKeys` 相同，不沿用呼叫端的 slice。

`plan.paused` 包含 `plan.hostChanged`（489 行），所以任何被前端吃掉的事件都會放開全部按著的鍵：面板開關、縮放，以及點到前端外殼（`PanelEventPointerHostHit` 為 `ConsumedByHost`，`host/panel_plan.go`）。例如飛行中點一下外殼，按著的鍵全部放開。這是刻意的：前端吃掉的事件代表玩家的注意力離開遊戲。前端在恢復後依主機按鍵狀態重送按下碼（§4）。

允許這個例外的理由：paused 回合不推進機器（`Advance` 624–630 行），排在 `m.Steps` 的放開碼要到恢復後才送出，時點與「前端在恢復後第一批送放開碼」相同。它只改 `timedKeys` 與按住狀態，不寫 DOS 緩衝、客體記憶體與 CPU。

`Stopped`、`Failed`、`Closed` 之後 `Deliver` 一律被拒，按住狀態不會再被動到，`Close` 不另外處理。

### 2.5 `Advance` 內的重複碼

非 paused 回合的 `Advance` 在 `o.dos.Exited` 檢查之後（`owner.go` 631–638 行）、記錄 `MachineStepsBefore`（640 行）之前呼叫 `m.ScheduleHeldRepeats(uint64(budget))`。paused、已停止與 Booting 等提早返回的路徑都不呼叫。機器跑到一半失敗時，已預排的重複碼留在 `timedKeys`，owner 此時已 `Failed`，不再使用。

### 2.6 計數與收據

`InputReceipt` 新增：

| 欄位 | 意義 |
|---|---|
| `ScanEventsCommitted` | 本批 `HeldKeyDown`、`HeldKeyUp` 回 true 的次數 |
| `ScanReleasesForced` | §2.4 `ReleaseHeldKeys` 的回傳值 |
| `HeldScans` | commit 之後的 `HeldMask()` |

這三個欄位不加進 `DOSCallsCommitted`：該欄位只數 DOS 呼叫（`owner.go` 98–100 行的註解），送鍵走的是機器的 `timedKeys`。`HeldScans` 用固定大小陣列，`InputReceipt` 維持可以 `==` 比較。

`TickReceipt` 新增：

| 欄位 | 來源 |
|---|---|
| `ScanPendingBefore` | `ScheduleHeldRepeats` 之後、跑機器之前的 `m.TimedKeysPending()` |
| `ScanPendingAfter` | 跑機器之後的 `m.TimedKeysPending()` |
| `KeyIRQs` | 本回合 `m.KeyIRQs` 的增量 |
| `KeyStalls` | 本回合 `m.KeyStalls()` 的增量（單位是「被擋的步數」：`timedKeyTick` 每次被 stub 擋下加 1，`keyhold.go` 104 行，不是每個事件加 1） |

名稱避開既有的 `KeysPendingBefore`／`After`（DOS 的 BDA 緩衝深度）。

paused 回合：`ScanPendingBefore == ScanPendingAfter ==` 進入 `Advance` 時的 `m.TimedKeysPending()`，`KeyIRQs`、`KeyStalls` 為 0。其他提早返回的路徑（Stopped、Failed、Closed、Booting 等）與 `KeysPendingBefore`／`After` 的做法相同：進入 `Advance` 時把兩個 `ScanPending*` 設成當時的 `m.TimedKeysPending()`，`KeyIRQs`、`KeyStalls` 為 0。

`KeyIRQs == ScanPendingBefore − ScanPendingAfter` 的前提：`timedKeyTick` 送出時同一處彈出事件並加 `KeyIRQs`（`keyhold.go` 111–114 行），`IF` 關、IRQ1 遮蔽、stub 擋下、IDTR 守門時兩者都不動（守門時 `Advance` 回錯誤）；另一個會加 `KeyIRQs` 的是 FIFO 的 `keyTick`（`keyboard.go` 126 行），session 沒有 `QueueKey` 呼叫者，所以不發生。

`KbdIntercept` 關閉且 `IVT[09h]` 指向 stub 時 `KeyStalls` 增加（`keyhold.go` 103–106 行）。沒有這個數，前端分不出「沒送到」和「遊戲不理」。

`View` 不加按住狀態：`View` 在測試裡以 `==` 比較（`owner_test.go` 564、573 行）。`Digest`（`digest.go` 32 行起）只雜湊記憶體、CPU、色號與色盤，不含 `timedKeys` 與按住狀態，兩個按住狀態不同的 session 可能得到相同 `Digest`。

### 2.7 範圍外

- `E0h`、`E1h` 前綴。灰色方向鍵、右 Ctrl／Alt 需要 `E0h`；不帶前綴送的是數字鍵台的鍵。STRIKE 對兩者是否有不同反應未知（`strike_commander` 的 `docs/re/010` §3）。
- 視窗後端按鍵到掃描碼的對應，屬於 `cmd/<app>-play`。
- session 讀狀態檔。session 目前沒有讀檔入口；之後加入時要清空按住狀態，與 `oracle.LoadStateFile` 相同。
- 既有 `presentation.KeyboardBridge.DeliverDOSScan` 走 `QueueKey`（FIFO 加 `KeyEvery` 節流），沒有非測試呼叫者，不改也不用。

## 3. 驗收

### 3.1 單元測試

session 的測試在 package 內以既有的合成 owner（`startLoadedMachine`，例如 `owner_test.go` 595 行）建立，開機前直接對 `o.machine` 呼叫 `SetDOSBoxCycles(CyclesXT)`。合成程式不裝 `int 09h`，`KbdIntercept` 關閉時事件留在 `timedKeys` 可以數。`CyclesXT` 下 `InstructionsPerSecond` 為 240000，`delay = 120000`、`period = 22018`。每次 `Advance` 之前都要 `Deliver` 一個 update（沒有輸入時送空的），否則 `turnPending` 擋下；最後一個放開批次之後也要 `Advance` 一次才能取排程。參考機器是另一台以相同方式建立、只呼叫 `HoldKey` 的合成機器。

1. 拒絕（每個案例用新的 owner，失敗的 owner 已關閉）：`ScanEvents` 與 `BIOSKeys` 並存；`Scan` 為 `0`、`80h`、`E0h`；paused 批次中的 `Scan` 為 `0`。各自 `Deliver` 回錯誤，`Status().Phase == PhaseFailed`，並在 package 內確認 `o.machine.TimedKeysPending()` 與 BDA 深度（`o.dos.KeysPending()`）與送出前相同。
2. 等價：以 3 種 `Advance` 切法（一次切 `d`；每 7919 步切；切點恰好落在某個重複碼的步數上）在 `a` 按下、`a + d` 放開，`ScheduledKeys()` 必須與另一台合成機器的 `HoldKey(scan, a, d, true)` 完全相同。另取一個讓 `a + d` 恰好等於某個重複碼步數的 `d`，涵蓋 §2.3 的 `Step == end` 邊界。`d` 取 2 秒左右（`CyclesXT` 下約數十萬步），控制回合數。「切點落在重複碼上」的切法另逐回合斷言 `ScanPendingBefore`，期望值取自參考機器：等於參考機器 `ScheduledKeys()` 中 `Step <= 本回合終點` 的事件數，放開碼在放開之前的回合不計（session 端尚未排入）。落在邊界 B 的重複碼因此在 B 之前那一回合就計入。
3. 點按：同一批 `[按下 k, 放開 k]`，`ScheduledKeys()` 是同一步的按下碼與放開碼各一個，順序是先按下。oracle 以 `ActionTap` 且 `MS` 為 0 得到相同結果。
4. oracle：每段先呼叫 `o.scheduleRepeats(n)`（委派到 `m.ScheduleHeldRepeats(n)`），再 `o.Run(n)`（`oracle/run.go` 81–83 行，即 `RunUntil(Steps(n), Budget(n+1))`），不用 `RunCycles`（cycles 無法對齊到指定步數）。在第 2 項的條件下 oracle `KeyDown`、`KeyUp` 與 `HoldKey` 差 0。另驗：
   - `Save`、`KeyDown`、`Restore` 之後 `HeldKeys()` 為空。
   - `KeyDown(k)`、`Save`、`KeyUp(k)` 並推進幾段、`Restore` 之後 `HeldKeys()` 為 `{k}`，接著推進到放開，`ScheduledKeys()` 與參考機器 `HoldKey` 相同。
   - 按住期間 `SetDOSBoxCycles` 改速度：之後的重複碼仍依按下時的 `delay`、`period`，與參考機器在原速度下的 `HoldKey` 相同。
5. 面板、失焦與收據值：
   - 按著 `k` 時送 `[按下 k]` 的 `InputReceipt`：`ScanEventsCommitted == 1`、`ScanReleasesForced == 0`、`HeldScans` 只有 bit `k`。
   - 再送一次 `[按下 k]`，以及送 `[放開 m]`（`m` 沒按著）：`ScanEventsCommitted == 0`，`timedKeys` 不變。
   - 按著 `k` 時開面板：該批 `ScanReleasesForced == 1`、`HeldScans` 為零，排程最後一個是 `k` 的放開碼；paused 批次的按下碼不 commit；該 paused 回合 `ScanPending*` 等於進入 `Advance` 時的 `TimedKeysPending()`。
   - 按著 `k`、`m` 時，非 paused 的 `FocusLost` 批次帶 `[按下 j, 放開 k]`：`j` 不排，`k` 放開，`m` 也放開；`ScanEventsCommitted == 1`、`ScanReleasesForced == 1`、`HeldScans` 為零。
   - `Deliver` 之後、`Advance` 之前改寫呼叫端的 `ScanEvents` slice，排程不變。
6. 計數：
   - `KbdIntercept` 關閉：`KeyStalls > 0`、`KeyIRQs == 0`、`ScanPendingAfter == ScanPendingBefore`。
   - `KbdIntercept` 開啟（`Config{KbdIntercept: true}`，`New` 呼叫 `EnableKbdIntercept`，`IVT[09h]` 指向客體碼；`LoadCOM` 已設 `IF=1`）：`KeyIRQs == ScanPendingBefore − ScanPendingAfter`，`KeyStalls == 0`。
   - paused 回合各欄位符合 §2.6。
7. 兩鍵次序：
   - 同步重複：同一批先按下 `j`、再按下 `k`（`j > k`），兩鍵同一步按下，`delay`、`period` 相同，重複碼同步。每個重複步上 `k` 在 `j` 之前（依掃描碼）。這一項的期望來自 §2.2 的排序規則，不與 `HoldKey` 比對（§2.3 已說明兩者在此不同）。
   - 邊界上的重複碼：先按下 `k`，在 `k` 的某個重複碼步數 B 的回合邊界按下 `j`。B 這一步 `k` 的重複碼排在 `j` 的按下碼之前，與參考機器 `HoldKey(k, …)` 再 `HoldKey(j, …)` 的序列相同。前提是之後兩鍵的重複碼不同步：`CyclesXT` 下 `j` 的重複碼在 `B + 120000 + i × 22018`，`k` 的在 `B + i × 22018`，120000 不是 22018 的整數倍，不會相撞。
8. Machine 層：
   - `Snapshot`、`HeldKeyDown(k)`、`Restore` 之後 `HeldKeys()` 為空；`HeldKeyDown(k)`、`Snapshot`、`HeldKeyUp(k)`、`Restore` 之後 `HeldKeys()` 為 `{k}`。
   - `HeldKeyDown(k)` 之後 `LoadState` 成功讀入一個狀態：`HeldKeys()` 為空、`ScheduleHeldRepeats` 不排任何事件。`LoadState` 讀入壞資料而失敗：`HeldKeys()` 仍為 `{k}`。
   - `HeldKeyDown(7Fh)` 回 true；`HeldKeyDown(0)`、`HeldKeyDown(80h)` 回 false 且 `timedKeys` 不變。
9. 零值錨點：不給 `ScanEvents` 時，448 §5.1 的 680 步錨點（Memory `74169ded…`、CPU `65451108…`）不變。

負對照（各關一次，對應測試必須失敗）：

| 關掉 | 必須失敗 |
|---|---|
| `HeldKeyUp` 改回 `CancelTimedKeys(scan, now)`（`Step > now`） | 第 2、4 項 |
| `HeldKeyUp` 的下限改成無條件 `now`（不取 `downStep+1`） | 第 3 項 |
| `ScheduleHeldRepeats` 的上界改成 `next < m.Steps + n` | 第 2 項的逐回合計數、第 7 項的邊界次序 |
| `Advance` 不呼叫 `ScheduleHeldRepeats` | 第 2 項 |
| `ScheduleHeldRepeats` 改成依按下先後排序 | 第 7 項的同步重複 |
| `Snapshot`／`Restore` 不帶按住狀態，或 `Restore` 一律清空 | 第 4、8 項的 `{k}` 斷言 |
| `LoadState` 不清按住狀態 | 第 8 項 |
| `LoadState` 在驗證之前就清空 | 第 8 項的讀檔失敗斷言 |
| `delay`、`period` 在每次 `ScheduleHeldRepeats` 重算 | 第 4 項的改速度斷言 |
| paused 回合也呼叫 `ScheduleHeldRepeats` | 第 5 項的 paused `ScanPending*` 斷言 |
| `ScanEventsCommitted` 改成數全部事件 | 第 5 項 |
| `prepare` 沿用呼叫端 slice | 第 5 項 |
| `HeldKeyDown` 的範圍檢查改成 `>= 7Fh` | 第 8 項 |
| 刪掉 paused 批次的強制放開 | 第 5 項 |
| 刪掉 `FocusLost` 的強制放開，或不忽略其中的按下事件 | 第 5 項 |
| paused 時略過 `ScanEvents` 的 commit 閘 | 第 5 項 |
| 刪掉並存拒絕 | 第 1 項 |
| 刪掉 `Scan` 範圍檢查 | 第 1 項 |

### 3.2 觸發案例（主代理執行，結果寫進 §5）

**輸入**：`SaveRoot`、`BootInput`、`Config` 與 448 §3.2 相同（`KbdIntercept` 為真）。

**步驟**：`BootOriginal` 之後以下列回合邊界切 `Advance`，在各邊界 `Deliver` 對應的 `ScanEvents`，其餘回合送空的 update：

| 回合邊界（`m.Steps`） | `ScanEvents` |
|---|---|
| 1010000000 | 按下 `50h` |
| 1030000000 | 放開 `50h` |
| 1050000000 | 按下 `48h` |
| 1053000000 | 放開 `48h` |
| 1090000000 | 無 |

各段 `Advance` 的預算依序是 1010000000、20000000、20000000、3000000、37000000、10000000，最後 `Advance(1)`。

在第 1030000000、1090000000 步的回合邊界，先取 `Digest()` 再 `Deliver`（`Digest` 在回合進行中被拒，`digest.go` 39–41 行），對應 probe `-shots` 在 `m.Steps == N`、執行第 N+1 道之前存檔（`cmd/probe/main.go` 956–959 行）。第 1100000000 步以 448 的同一個觀察器在 `BeforeStep` 看到 `Steps == 1100000000` 時記暫存器（同 448 §3.2，再 `Advance(1)`）。

**期望值**：`cmd/probe` 以同一組選項冷開機，獨立於被測程式。上表的序列就是這組 probe 指令的對應，步數一一相同。

| 斷言 | 期望 | 出處 |
|---|---|---|
| 第 1030000000 步 `IndexedSHA256` | `bc24e8ab4c8d6bc817911e9f8e71a3961125cf823aa74e8d96d5e5fb36d0e114` | `p449ref30/r103.png` |
| 第 1030000000 步 `PaletteSHA256` | `ea48da3ee0628b0cf7609482ed5946e7ae78939b8c45d2fb0850a8bb566a5b60` | `p449ref30/r103.pal` |
| 第 1090000000 步 `IndexedSHA256` | `02e210d55663e8ec7e3d9efc059d3cdd3741eed4191c55b573720f7e1fa45098` | `p449ref30/r109.png` |
| 第 1090000000 步 `PaletteSHA256` | `ea48da3ee0628b0cf7609482ed5946e7ae78939b8c45d2fb0850a8bb566a5b60` | `p449ref30/r109.pal` |
| 第 1100000000 步暫存器 | CS:IP `0018:581D`，AX `BAB6`，BX `13DC`，CX `2000`，DX `0020`，DS `0020`，ES `00D8`，SS:SP `0050:13BA` | `probe449ref30.log` 終態 |
| `KeyIRQs` 總數（各回合 `TickReceipt.KeyIRQs` 加總，到第 1100000000 步） | 18 | `probe449ref30.log` 的 `-kbd-int15` 摘要行 |
| `KeyStalls` 總數 | 0 | 同上 |

probe 指令（`strike_commander` 的 `tools/probe/psc.sh`，`workplace/bin/probe-sc` 與 448 §3.2 同一個檔，SHA-256 `a62b5c79…9afeb`）：

```text
psc.sh p449ref30 probe449ref30 -dos-files 30 -kbd-int15 -steps 1100000000 \
  -hold 50@1010000000+20000000,48@1050000000+3000000 -shots 1030000000:/scratch/r103.png,1090000000:/scratch/r109.png
```

`psc.sh` 本身不帶 `-dos-files`；漏給時 `SCCD.EXE` 連 `JEMM.OVL` 都沒開（`probe449ref.log`：開過的檔 0 個，停在模式 `03h`），所以參考值一定要帶 `-dos-files 30`。

MENU 在這段讀走 16 個 make 事件（`probe449ref30.log` 的按鍵去向：`50h` 的按下碼 1 個加重複碼 14 個，`48h` 的按下碼 1 個），`KeyIRQs` 18 含兩個放開碼。第一個重複碼在第 1015791848 步讀走，間隔約 106 萬步。`50h` 按住 2000 萬步長於 `delay`（`InstructionsPerSecond` 的一半），`48h` 按住 300 萬步不觸發重複（`probe449m-on.log`，每鍵 2 個事件）。這段沒有涵蓋 `Step == end` 邊界（第 15 個重複碼約在第 1030.68 百萬步，晚於放開），邊界只由 §3.1 第 2 項負責。

**負對照**：拿掉 `ScheduleHeldRepeats` 跑一次，`KeyIRQs` 總數與第 1100000000 步暫存器必須改變。畫面斷言不能當負對照：MENU 只有兩個選項，下移停在底端，關掉重複時兩個時點的 `IndexedSHA256` 與上表相同。佐證是 probe 加 `-hold-typematic=false` 的同一組指令（`probe449nt.log`）：`KeyIRQs=4`，終態 CS:IP `0018:4D85`，兩張截圖的 SHA-256 與上表相同。

**執行環境**：同 448 §3.2，長測試以 `DOSGOLEM_SC_ROOT` 控制，在 Docker 內執行。

### 3.3 回歸

`go vet ./...`、`go test ./...` 在 Docker 內全過；`apps/` 與 `cmd/` 的既有測試結果與改動前相同，特別是 `oracle` 的 `live_internal_test.go`、`actions_internal_test.go`、`step_guard_test.go`，以及呼叫 `(*Oracle).KeyDown`／`KeyUp` 的 `oracle/actions.go` `RunActions` 與 `cmd/step`。

## 4. 已知限制

- typematic 照 `HoldKey`：每個按著的鍵都重複。真機 8042 只重複最後按下的鍵。先與既有收據一致；要改成真機行為時連 probe `-hold` 一起改並重錄收據。
- 兩個按著的鍵在同一步重複時，次序與 `HoldKey` 不同（§2.3）。
- `CPU.Segmented()` 期間 `timedKeyTick` 不執行（§2.3），事件累積到離開分段模式後才依序送出。STRIKE 在 JEMM 下飛行時 IRQ1 送得到（`strike_commander` 的 `docs/re/010` §2），但送出時點可能因此延後。
- 強制放開之後，仍按著的鍵不自動重按，由前端依主機按鍵狀態決定是否重送按下碼。
- 跨批次的雙送不由 owner 擋：前端對同一支程式固定用 `BIOSKeys` 或 `ScanEvents` 其中一條。
- `KbdIntercept` 關閉且程式沒有裝 `int 09h` 時，事件一直留在 `timedKeys`；按住期間每回合再加重複碼，每次 `ScheduleKey` 都重新排序，長按時成本上升。到 `int 09h` 裝好時，過期的重複碼會在放開碼之前依序送出，與 `HoldKey` 相同。
- `Digest` 不含按住狀態與 `timedKeys`（§2.6），§3.2 只能以 `KeyIRQs` 與暫存器偵測送鍵偏差。
- §3.2 的暫存器比對是局部的，理由同 448 §4。
- `period` 為 0 的分支（§2.2）在現有 setter 下不可達，沒有測試。
- `ScheduleKey` 每次重排整個 `timedKeys`（`keyhold.go` 33 行）。前端對很大的 `budget` 呼叫 `Advance` 時，`ScheduleHeldRepeats` 一次排入大量重複碼（`CyclesXT` 下每 10 億步約 4.5 萬個）。前端以畫格為單位推進時每回合只有幾個；改成插入排序不在本規格。

## 5. 實作收據（2026-10-18）

實作 commit `73ee63b`。實作由子代理完成，測試由主代理在 Docker 內重跑。

### 5.1 單元測試與回歸

- `go vet ./...` 通過；`session`、`oracle`、`internal/machine`、`cmd/step` 的 `go test -count=1` 全過（主代理重跑）。
- §3.1 對應的測試：

| 項 | 測試 |
|---|---|
| 1 | `TestScanEventsRejectedBeforeCommit` |
| 2 | `TestScanEventsHoldMatchesHoldKey`（`d` 取 480000 與 472288，後者落在重複碼上；三種切法） |
| 3 | `TestScanEventsTapSameBatch`、`TestHeldKeyOracleTapZeroMS` |
| 4 | `TestHeldKeyOracleMatchesHoldKeyExactly`、`TestHeldKeyOracleSaveRestore`、`TestHeldKeyOracleSpeedChangeKeepsDelayPeriod` |
| 5 | `TestScanEventsPanelFocusAndReceipts` |
| 6 | `TestScanEventsTickCounters` |
| 7 | `TestScanEventsTwoKeyOrder` |
| 8 | `TestHeldKeysSnapshotRestore`、`TestHeldKeysLoadStateClearsAtCommit`、`TestHeldKeyDownRange`、`TestCancelTimedKeysFromIncludesStep` |
| 9 | 既有 `TestLegacyConfigZeroValueAnchor` 不變；另加 `TestScanEventsZeroValueReceipts` |

- 回歸：改動前後 `go test -count=1 ./...` 的套件結果相同（42 ok、3 FAIL）。3 個 FAIL 是 `apps/phantasie/cmd/phantasie-play`、`cmd/buckrogers-play`、`frontend/ebiten`，容器內沒有顯示器，ebiten 初始化失敗，改動前就失敗。

### 5.2 負對照

子代理逐列改壞再還原，17 列使對應測試失敗（輸出在 `strike_commander` 的 `workplace/i449/neg/`）。主代理在複製樹上獨立重做兩列：`CancelTimedKeysFrom` 改成 `>`，第 2、4 項失敗；`ScheduleHeldRepeats` 上界改成 `<`，第 2、7 項失敗。

「paused 回合也呼叫 `ScheduleHeldRepeats`」這一列測不出來，是等價突變：§2.4 規則 3 讓每個 paused 批次 commit 之後按住集合必定為空，paused 回合預排重複碼沒有對象。§2.5「paused 回合不呼叫」因此由規則 3 保證，不另設測試。

### 5.3 觸發案例

`TestIRQ1ScanTrigger/Base` 通過（162 秒），`SaveRoot` 用 `strike_commander` 的 `workplace/root-session448`（448 §5 的複製樹）。§3.2 的全部斷言相符：

| 斷言 | 結果 |
|---|---|
| 第 1030000000 步 Indexed、Palette | `bc24e8ab…e114`、`ea48da3e…5b60`，相同 |
| 第 1090000000 步 Indexed、Palette | `02e210d5…5098`、`ea48da3e…5b60`，相同 |
| 第 1100000000 步暫存器 | `0018:581D` AX `BAB6` BX `13DC` CX `2000` DX `0020` DS `0020` ES `00D8` SS:SP `0050:13BA`，相同 |
| `KeyIRQs` 總數、`KeyStalls` 總數 | 18、0，相同 |

各回合（`ScanPendingBefore→After`，`KeyIRQs`）：1010000000 起 15→0、15；1030000000 起 1→0、1；1050000000 起 1→0、1；1053000000 起 1→0、1；其餘為 0。第一回合的 15 是按下碼 1 個加預排到回合尾的重複碼 14 個，與 probe 讀走的 make 事件數相同。

負對照 `TestIRQ1ScanTrigger/NoRepeats`：`KeyIRQs` 4，第 1100000000 步 CS:IP `0018:4D85`、AX `00D8`、CX `00AE`、SP `13BC`，與 Base 不同；兩張截圖不變，與 §3.2 的預測相同。這組值也與 probe `-hold-typematic=false`（`probe449nt.log`）的 `KeyIRQs=4`、CS:IP `0018:4D85` 一致。

### 5.4 實作期間的處置

- 第 5 項的 slice 複製測試改在 package 內以 `prepare`、改呼叫端 slice、`commit` 的順序驗證；`Deliver` 是同步完成的，照 §3.1 字面在 `Deliver` 之後改 slice 驗不到這條規則。
- 第 2 項的逐回合 `ScanPendingBefore` 只對 `d = 480000` 斷言。`d` 落在重複碼上時，最後一回合 session 已預排 `end` 那一步的重複碼（放開時才取消），參考機器的 `HoldKey` 因 `t < end` 沒有這個事件，兩者在該回合本來就不同；最終排程相同由同一項的序列比對負責。
- 第 8 項的讀檔失敗另加一種能解碼、但延伸記憶體大小不符的狀態檔，涵蓋「解碼後、驗證前就清空」的錯誤位置。
- 長測試負對照的開關是未匯出欄位 `Owner.noHeldRepeats`，沒有新增公開 API。
