# 198 — 機器速度：DOSBox 相容的 cycles

狀態：**READY**
日期：2026-09-17
前置：[`190-irq0-calibration.md`](190-irq0-calibration.md)（`IRQ0Base` 與標定分頻 17,000）、
[`191-two-clocks-relationship.md`](191-two-clocks-relationship.md)、[`197-key-hold-and-typematic.md`](197-key-hold-and-typematic.md)

---

## 1. 問題

指令數時鐘的機器速度固定是 `StepsPerSecond()`：165,000 道指令一刻（分頻 17,000），約每秒 11.58 M 道指令。
對拍時這是好事（與大富翁2 的標定一致）；**給人玩時是錯的**：

- 有些遊戲的進度綁在 CPU 上（空迴圈計時、每畫一格就推進一步）。
  案例：DOS 版《銀河超能力戰記》第一場戰鬥約 1,193 萬道指令，在預設速度下約 1 秒打完，人玩不了
  （`psychic_war_cht` 的 `docs/re/009` §5）。
- 那款遊戲說明書寫的是 IBM PC XT／AT；速度的參照是 DOSBox-X 同機型的 `cycles`。

週期時鐘（`-cpuhz`）的週期表以 386／486 為準，8088 級的速度會被高估約 5 倍，不能直接拿來對應 DOSBox 的 `cycles`。

## 2. 參照：DOSBox-X 的 cycles 怎麼算

機型預設（`src/gui/menu_callback.cpp`，「Emulate CPU speed」選單）：

| 機型 | cycles（每毫秒） |
|---|---:|
| 8088 XT 4.77 MHz | 240 |
| 286 8 MHz | 750 |
| 286 12 MHz | 1,510 |

normal core 的扣法（`src/cpu/core_normal.cpp`、`core_normal/string.h`、`src/hardware/iohandler.cpp`）：

1. **每道指令 1 個 cycle**：主迴圈 `while (CPU_Cycles-->0)` 每取一道指令扣一次；前綴走 `restart_opcode`，不另外扣。
2. **字串指令每一次迭代再扣 1 個**：`string.h` 的每個迴圈都是 `if ((--CPU_Cycles) <= 0) break;`。
   沒有 REP 時 `count = 1`，一樣扣一次。所以 `movsb` 是 2 個 cycle，`rep movsb`（CX ＝ N）是 1 ＋ N 個。
3. **I/O 延遲**：`CPU_CycleMax × io_delay_ns ÷ 1,000,000`（讀；寫再乘 3/4），8 位元預設
   `io_delay_ns ＝ 10⁹ × 8.5 ÷ 8,333,333 ≈ 1,020`。整數除法下 cycles < 981 時為 0、寫入 < 1,307 時為 0；
   **本規格涵蓋的三個機型預設中，只有 at12 的讀取會扣到 1**，不模擬（§5）。

**舊版本規格寫「一道指令一個 cycle」是錯的**：繪圖密集的程式大量使用 `rep movsb／stosb`，
照指令數設定速度會比 DOSBox-X 同樣 cycles 快很多（`psychic_war_cht` 實測：DOSBox-X 750 cycles 走到第一場戰鬥的牆上時間，
遠長於「dosgolem 指令數 ÷ 750,000」）。

## 3. 規格

### 3.1 CPU：DOSBox 計費

- `cpu.CPU.DOSBoxCost bool`。開著時 `Cycles` 只照 §2 第 1、2 條累計：每道指令 ＋1、字串指令每次迭代 ＋1；
  其他類別（前綴、記憶體、I/O、跳躍、中斷……）一律不計。關著時行為不變。

### 3.2 機器

- `Machine.SetDOSBoxCycles(perMs uint64)`：開週期時鐘、`CPU.DOSBoxCost ＝ true`、`CPUHz ＝ perMs × 1000`，重算 IRQ0 間隔。
  `perMs ＝ 0` 還原：關週期時鐘、關 DOSBox 計費、`CPUHz ＝ DefaultCPUHz`。
- `Machine.DOSBoxCycles() uint64`：目前設定（未設時 0）。
- 常數 `CyclesXT ＝ 240`、`CyclesAT8 ＝ 750`、`CyclesAT12 ＝ 1510`。
- `Machine.InstructionsPerSecond() float64`：DOSBox 計費開著時回 `CPUHz`（每秒 cycles，當作每秒指令數的上限近似）；
  否則照舊 `IRQ0Base × PITBaseHz ÷ 17000`。`HoldKey` 的 typematic、`TonePCM` 的時間軸用它換算。
- `DOSBoxCost` 存進快照與狀態檔（gob 加欄位，舊狀態檔讀進來是 false）。

### 3.3 probe

- `-cycles <數字|xt|at8|at12>`：每毫秒 cycles，呼叫 `SetDOSBoxCycles`。
- 在載入狀態檔之後套用（狀態檔會還原時鐘設定）；與 `-cpuhz` 同時給時報錯。
- **先套 `-cycles` 再解析 `-hold`**：`ms` 以 `InstructionsPerSecond()` 換算成指令數。
- 結尾摘要印出 DOSBox cycles 與牆上時間換算（cycles ÷ `CPUHz`）。

## 4. 驗收

1. 計費：`mov ax,bx` 1 個 cycle；`movsb` 2 個；`rep movsb`（CX ＝ 5）6 個；`out dx,al` 1 個；帶段前綴的 `mov` 1 個。
2. `SetDOSBoxCycles(240)`、分頻 16571：IRQ0 間隔 ＝ `240000 × 16571 × 264 ÷ 315,000,000` 個 cycle（整數，3,333）。
3. `SetDOSBoxCycles(0)` 還原：週期時鐘關、計費關、`InstructionsPerSecond()` ＝ `StepsPerSecond()`。
4. 快照與狀態檔保存並還原 `DOSBoxCost`。
5. `HoldKey` typematic：按住「2 秒份」，在 240 cycles 與預設速度下重複次數相同。
6. probe：`-cycles xt` 配 `-load-state` 生效；與 `-cpuhz` 同時給報錯；`parseCycles` 認得三個名稱與正整數。
7. 反向對照：字串指令迭代不計費時，第 1 項的 `movsb`／`rep movsb` 失敗。
8. 實機比對：《銀河超能力戰記》第一場戰鬥在 240 與 750 cycles 下的持續時間，與 DOSBox-X 同設定相差 ≤ 10%
   （在 `psychic_war_cht` 驗，結果記在該 repo 的 `docs/re/`）。

## 5. 不做

- I/O 延遲（§2 第 3 條）、`HLT` 讓出剩餘 cycles、中斷送達的成本。
- 週期時鐘 386 週期表的調整。
- 牆上時間對齊（屬前端）。
- `cpu386`（保護模式 LE）核心的 DOSBox 計費。
