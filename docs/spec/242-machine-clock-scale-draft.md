# 242 — 機器時脈比例（較慢的虛擬 CPU）

狀態：**READY**（2026-09-28，四輪獨立審查後）
日期：2026-09-28
前置：[`190-irq0-calibration`](190-irq0-calibration.md)（指令數時鐘的標定）、
[`238-session-step-observer`](238-session-step-observer-draft.md)、[`240-opl-audio-output`](240-opl-audio-output-draft.md)。

## 1. 為什麼要做

dosgolem 的時間由指令數驅動：計時器中斷、VGA 回掃、鍵盤中斷都以「每幾道指令一次」表示，名目速度
`StepsPerSecond()` 約 1158 萬指令／秒（386DX-33 量級）。前端每個主機畫格推進 `VGAFrameEvery × 70/60` 道指令，
所以主機**實際**每秒跑得出幾道指令，就決定遊戲的實際速度。

玩家回報 `buckrogers-play` 玩起來偏慢。本機量測（已證實）：冷開機 900 畫格，CPU 模擬本身約 121 奈秒一步，
加上覆繪觀測器後約 280 奈秒一步（CPU 剖析，Docker、Xvfb）。前者上限約 830 萬步／秒，已低於名目速度，
覆繪觀測器優化也補不回來。原版 1990 年的目標機器是 286／386SX，並不需要 386DX-33 的指令量。

## 2. 契約

### 2.1 machine

- 純函式 `machine.ScaleSteps(v uint64, percent int) uint64` = round(v × percent / 100)；v=0 回 0，v>0 時至少 1。
  機器與前端共用它，不各自寫公式。
- `Machine.SetClockPercent(p int) error`：p 須在 10–100（100 為現行）。守門條件：`m.Steps != 0` 或 `m.CycleClock`
  為真即回錯誤（週期時鐘不受 `IRQ0Base` 影響，兩者並用未定義）。
- p=100 時 `SetClockPercent` 不改任何欄位（只記錄比例），保證既有行為完全不變。
- p<100 且 `Steps == 0` 時依序：`IRQ0Base = ScaleSteps(DefaultIRQ0Every, p)`，`IRQ0Every` 依目前分頻以 `stepsPerTick(IRQ0Base, PITDiv)`
  重算（同 `recalcIRQ0`，含 `MinIRQ0Every` 夾限）；`nextIRQ0 = ScaleSteps(nextIRQ0, p)`；
  `VGAFrameEvery = ScaleSteps(DefaultVGAFrameEvery, p)`、`nextFrame = ScaleSteps(nextFrame, p)`；
  `KeyEvery = ScaleSteps(DefaultKeyIRQEvery, p)`、`nextKey = ScaleSteps(nextKey, p)`（開機時為 0，維持 0）。`New()` 的第一刻（`nextIRQ0 = DefaultIRQ0Every`
  而非依預設分頻的 636,085）偏密是既有行為，縮放照比例保留，不是新引入的差異。
- 之後 PIT 分頻寫入照常以 `IRQ0Base` 重算，比例持續有效。`int 1Ah`、`int 61h AH=0Ch` 等讀機器欄位的路徑自動一致。
- 新增 `Machine.ClockPercent()`（未設定時回 100）與 `Machine.StepsPerSecondScaled()` = `StepsPerSecond() × p/100`；
  `ScanLine`（CRTC 時序，預設不啟用）改用後者。`oracle/speaker.go`、`cmd/probe` 仍用未縮放的 `StepsPerSecond()`：
  它們只服務 p=100 的既有工具，本規格不改。
- 快照與存檔：`Snapshot` 與 `SaveState` 都補存 `VGAFrameEvery`、`nextFrame`、`KeyEvery`、`ClockPercent`
  （`SaveState` 另補 `nextKey`）。**還原時存檔決定時脈**：整組時鐘欄位照存檔覆寫，不經過 `SetClockPercent`，
  不保留還原前機器的比例。`ClockPercent` 缺席（0，242 之前的存檔）即視為 100 的存檔：`IRQ0Base`、`PITDiv`、
  `IRQ0Every`、`nextIRQ0` 照檔案（既有欄位本來就存著 100% 的值）；缺席的 `VGAFrameEvery`、`KeyEvery`：還原前機器縮放過（比例≠100）
  則設為 `DefaultVGAFrameEvery`、`DefaultKeyIRQEvery`，否則保留當下值（實作時發現 `cmd/probe` 在載入前自訂 `KeyEvery`，
  一律設回預設會改變既有行為）；缺席的 `nextFrame`、`nextKey` 維持還原當下的值（既有行為，
  既有工具都是新建機器後才還原，當下值即 100% 預設）。後果是還原後第一步 `Steps >= nextFrame` 立即成立，多一次
  提早的回掃後自行對齊，`nextKey` 則讓第一個排隊鍵不必等冷卻；這是現有所有「載入舊存檔再往下跑」收據的一部分，
  改掉會改變 p=100 的收據，本規格刻意不改。新格式存檔存有這兩個欄位，不受影響。`LoadState` 現有「`IRQ0Base` 為 0 時用 `DefaultIRQ0Every`」
  的回退保留，與此一致。還原後 `ClockPercent()` 回報存檔的比例。
- p=100 時所有既有行為與收據不變。

### 2.2 session

- `session.Config.ClockPercent int`（0 視為 100），在 `New` 內與 `AdLib` 同處、第一道指令之前呼叫 `SetClockPercent`；
  錯誤即 `New` 失敗。`StateDigest` 新增 `ClockPercent`。

### 2.3 buckrogers-play

- 旗標 `-clock`（百分比 10–100）。預設 50：使用者回報 100 偏慢；本機量測 p=100 約 27 fps、p=50 約 58 fps（§5）。非法值在開機前 `die`，不夾限。
- `stepsPerHostFrame` 改為執行期值：`ScaleSteps(DefaultVGAFrameEvery, clock) × 70 / 60`；它同時給 `Advance` 的預算與
  `mixer.New(stepsPerHostFrame × 60)`，音畫同一時基。同一個百分比放進 `session.Config.ClockPercent`。
- 遊戲執行中不能改時脈；改設定要重新啟動。

## 3. 不做什麼

- 不在執行中動態改時脈；不與 `CycleClock` 並用。守門只擋「已開 `CycleClock` 再設比例」；反方向（先設比例再開
  `CycleClock`／`SetCPUHz`）不攔，屬 oracle／probe 工具的已知邊界，`buckrogers-play` 不會走到。
- 不改 CPU 模擬器本身（另案優化）。

## 4. 驗收

1. 單元測試：`ScaleSteps` 的四捨五入、v=0 回 0、v>0 下限 1；p=100 不改任何欄位；p=50 時 `IRQ0Base`、`IRQ0Every`、
   `VGAFrameEvery`、`KeyEvery`、`nextIRQ0`、`nextFrame` 為依公式的值且 `nextKey` 維持 0；PIT 寫入 17,000 後
   `IRQ0Every = stepsPerTick(IRQ0Base, 17000)`；`Steps != 0`、`CycleClock`、p 超出範圍皆回錯誤；`Snapshot`／`Restore`
   與 `SaveState`／`LoadState` 往返都保留上述欄位與比例；交叉情境：先 `SetClockPercent(50)`，再載入一份真實的舊格式
   存檔（`IRQ0Base` 等既有欄位非零、`ClockPercent` 與新欄位缺席），還原後所有時鐘欄位都是 100% 的值、
   `ClockPercent()` 回 100；再跑到下一次正常回掃，只多出恰好一次提早回掃（與 p=100 載入同一檔的現況相同）。
2. p=100：既有 machine、session 單元測試全過；Buck phase254 七條回歸與前一基準相同。
3. Buck：以 `buckrogers-play -frames N -shot` 冷開機到功能選單，p=50 與 p=100 用**同一份**按鍵排程與畫格數：
   截圖逐位元組相同（遊戲時間節奏不變）、`IRQ0Clamped == 0`；每畫格的 CPU 時間（`-cpuprofile` 總樣本 ÷ 畫格數）
   p=50 約為 p=100 的 50%，容忍 ±15%（CPU 模擬之外的固定成本不隨比例縮）。
4. 實機：使用者以選定時脈遊玩，回報速度與音樂節拍；發行版預設值依回報決定。

## 5. 量測（2026-09-28）

- Buck 冷開機到功能選單（1250 畫格、同一按鍵排程，Docker＋Xvfb、2 核）：p=100 為 240,625,000 步、CPU 樣本 44.18 秒、
  實耗 47.0 秒（約 27 fps）；p=50 為 120,312,500 步、CPU 樣本 17.57 秒（p=100 的 40%）、實耗 21.4 秒（約 58 fps）。
  兩者截圖逐位元組相同，`IRQ0Clamped` 皆為 0；p=100 截圖與既有參照相同。
