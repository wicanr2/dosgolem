# 199 — 即時執行介面：依 cycles 跑、即時按鍵、串流音訊

狀態：**READY**
日期：2026-09-17
前置：[`198-dosbox-cycles.md`](198-dosbox-cycles.md)（DOSBox 相容 cycles）、[`197-key-hold-and-typematic.md`](197-key-hold-and-typematic.md)、
[`195-pit-channel2-tone-wav.md`](195-pit-channel2-tone-wav.md)、[`196-opl2-synth-skeleton.md`](196-opl2-synth-skeleton.md)

---

## 1. 問題

`oracle` 是給對拍寫的：按鍵在「第幾道指令」排好、音訊在跑完之後一次合成成 WAV。
給人玩的前端要的是另一種時間線：

- 每一幀（約 16.7 ms）跑「這一幀份」的機器時間，不是固定的指令數；
- 按鍵是「現在按下」「現在放開」，放開的時間事先不知道；
- 音訊要一幀一幀取出來送給音效卡，不能等整段跑完；
- 要能從 probe 產生的狀態檔直接開始（除錯、測試用）。

外部 module（例如 `psychic_war_cht` 的前端）import 不到 `internal/`，所以這些要做成 `oracle` 的公開介面。

## 2. 參照

- 速度與 cycles：規格 `198`。前端以牆上時間對齊時，每毫秒跑 `DOSBoxCycles()` 個 cycle。
- typematic：BIOS 預設延遲 500 ms、每秒 10.9 次（規格 `197` §3）。
- 喇叭方波：規格 `195` §3.1 的解碼規則（43h／42h／61h）；OPL2：規格 `196` 的合成器。

## 3. 規格

### 3.1 速度與時鐘

- `(*Oracle).SetDOSBoxCycles(perMs uint64)`、`DOSBoxCycles() uint64`：轉呼 `machine`（規格 `198` §3.2）。
- `(*Oracle).Cycles() uint64`：目前累計的 cycles。
- `(*Oracle).SetAdLib(present bool)`：388h 上有沒有 OPL2（與 probe `-adlib` 相同）。

### 3.2 依 cycles 執行

- `(*Oracle).RunCycles(n uint64) error`：跑到 `Cycles() ≥ 起點 ＋ n` 為止（跨過目標的那一道指令執行完就停）。
  停止條件、護欄、hook 與替身與 `RunUntil` 相同（程式結束回 `ExitError`、HLT 與跑出記憶體回錯誤）。
- 沒有開 DOSBox cycles（`DOSBoxCycles() == 0`）時回錯誤：指令數時鐘沒有「機器時間」可以對齊。
- 跑之前先處理 §3.3 的按住按鍵，排好這一段時間內到期的 typematic 事件。

### 3.3 即時按鍵

- `(*Oracle).KeyDown(scan uint8)`：鍵沒有按著時，在目前步數排一個按下碼（`machine.ScheduleKey`），記下按住的起點；已經按著就不動。
- `(*Oracle).KeyUp(scan uint8)`：鍵按著時，取消這個鍵在目前步數之後還沒送出的按下碼，再排一個放開碼；沒按著就不動。
- `(*Oracle).HeldKeys() []uint8`：目前按著的鍵，依掃描碼排序。
- typematic：按住的鍵在「起點 ＋ 延遲」之後每個週期補一個按下碼。步數換算用 `InstructionsPerSecond()`（規格 `198` §3.2）。
  `RunCycles(n)` 開始時，把落在 `[目前步數, 目前步數 ＋ n]` 之內的重複事件排進去（每道指令至少一個 cycle，所以步數不會超過 n）。
- `machine.CancelTimedKeys(scan uint8, afterStep uint64) int`：移除該鍵、步數大於 `afterStep` 的**按下**事件，回移除數。

### 3.4 狀態檔

- `(*Oracle).LoadStateFile(path string) error`、`SaveStateFile(path string) error`：轉呼 `internal/state`，與 probe `-load-state`／`-save-state` 同格式。
- ⚠ 狀態檔含原版的整份記憶體，**呼叫端負責不散布**（規格 `005` §5 的 `State` 不落地，這兩支是明確的例外，給除錯與測試用）。

### 3.5 串流音訊

- `(*Oracle).NewAudio(rate int) *Audio`（rate ≤ 0 用 44,100）。一台機器同時只能有一個 `Audio`。
- `(*Audio).Render() []int16`：回從上次 `Render`（第一次：建立時）到現在這段**機器時間**的單聲道取樣。
  - 取樣數：`(cycles 差 ÷ (DOSBoxCycles() × 1000)) × rate`，小數累進到下一次，長期不漂移。沒開 DOSBox cycles 時用 `InstructionsPerSecond()` 換算步數差。
  - 喇叭：以規格 `195` §3.1 的規則增量解碼 `PortLog`，方波振幅 ±0.25、靜音 0，相位跨幀連續。
  - OPL2：增量把 `m.OPL` 的寫入送進 `opl2.Synth`，取 `Sample()`；只有 `SetAdLib(true)` 時混入。
  - 事件以**寫入當下的 cycles** 定位：本段起點 cycles → 第 0 個取樣，終點 cycles → 最後一個取樣（`PortWrite`、`OPLWrite` 加 `Cycles` 欄位）。
    沒開 DOSBox cycles 時改用步數。
    ⚠ 不能用步數定位：字串指令讓一段時間內「步數：cycles」不均勻（繪圖多的地方一步好幾個 cycle），前端落後補跑 100 ms 一段時，
    音符起點會偏幾十毫秒（`psychic_war_cht` `docs/re/016`：聲音層配對率 24.6%）。
  - 兩者相加後夾在 [-1, 1]，乘 32,767。
  - **消化完的 `PortLog` 與 `m.OPL` 會清掉**，長時間遊玩時記憶體不會無限成長；需要完整紀錄的工具（`-dump-ports`、`-opl-log`、`ToneEvents`）不要與 `Audio` 同時用。
- 喇叭解碼抽成 `machine.ToneDecoder`（`Feed(PortWrite) (hz float64, changed bool)`），`ToneEvents` 改用它，結果不變。

## 4. 驗收

1. `RunCycles(n)`：跑完 `Cycles()` 增量 ≥ n，而且去掉最後一道指令之前 < n。
2. 沒開 DOSBox cycles 時 `RunCycles` 回錯誤。
3. 按鍵：`KeyDown` 後下一段執行送出按下碼；以 240 cycles 按住 2 秒（分多段 `RunCycles`）的按下碼數，與 `HoldKey` 同長度 typematic 開時相差 ≤ 1；
   `KeyUp` 後不再出現該鍵的按下碼，最後一個事件是放開碼。
4. `CancelTimedKeys` 只移除指定鍵、指定步數之後的按下碼。
5. 音訊：
   - 沒有聲音時，分 10 段 `Render` 的總取樣數與「總 cycles ÷ 每秒 cycles × rate」相差 ≤ 1，值全為 0；
   - 以埠寫入造 440 Hz 方波，取樣的過零頻率在 440 Hz ±1%；
   - OPL2 設一個音色並 Key-On 後有非零取樣；
   - `Render` 後 `PortLog` 與 `m.OPL` 長度為 0（反向對照：拿掉清除時此項失敗）；
   - 步數與 cycles 不成比例時（本段前 1% 的步數用掉 93% 的 cycles），在該處寫入的音從第 93% 的取樣附近開始（反向對照：改用步數定位時此項失敗）。
6. `ToneEvents` 改用 `ToneDecoder` 後，既有 `tone_test.go` 全部通過。
7. `SaveStateFile` → `LoadStateFile` 往返：步數、CPU 暫存器、記憶體相同。

## 5. 不做

- 牆上時間對齊的迴圈本身（前端的事）；音效卡輸出與緩衝管理。
- 滑鼠的即時介面（沿用既有 `MoveMouse`／`Click`）。
- `cpu386` 保護模式程式的 DOSBox 計費（規格 `198` §5）。
