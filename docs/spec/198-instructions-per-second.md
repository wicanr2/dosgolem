# 198 — 機器速度：每秒指令數（對應 DOSBox 的 cycles）

狀態：**READY**
日期：2026-09-17
前置：[`190-irq0-calibration.md`](190-irq0-calibration.md)（`IRQ0Base` 與標定分頻 17,000）、
[`191-two-clocks-relationship.md`](191-two-clocks-relationship.md)、[`197-key-hold-and-typematic.md`](197-key-hold-and-typematic.md)

---

## 1. 問題

指令數時鐘的機器速度固定是 `StepsPerSecond()`：165,000 道指令一刻（分頻 17,000），約每秒 11.58 M 道指令。
對拍時這是好事（與大富翁2 的標定一致）；**給人玩時是錯的**：

- 有些遊戲的進度綁在 CPU 指令數上（空迴圈計時、每畫一格就推進一步）。
  案例：DOS 版《銀河超能力戰記》第一場戰鬥約 1,193 萬道指令，在預設速度下約 1 秒打完，人玩不了；
  而敵人的攻擊推定綁在計時器上，速度一變連難度都變（`psychic_war_cht` 的 `docs/re/009` §5）。
- 那款遊戲說明書寫的是 IBM PC XT／AT，也就是每秒幾十萬道指令的機器。

`IRQ0Base` 已經是這顆旋鈕（標定分頻下一刻幾道指令，會存進狀態檔），但 probe 沒有開放，
而 `-tick` 只設一次 `IRQ0Every`，程式一重設分頻就被蓋回預設。

週期時鐘（`-cpuhz`）是另一條路，它的週期表以 386／486 為準，8088 級的速度會被高估約 5 倍（`docs/re/009`），
不適合拿來對應說明書上的舊機器。

## 2. 參照：DOSBox-X 的機型預設

DOSBox-X 的「Emulate CPU speed」選單（`src/gui/menu_callback.cpp`）：

| 機型 | cycles |
|---|---:|
| 8088 XT 4.77 MHz | 約 240 |
| 286 8 MHz | 約 750 |
| 286 12 MHz | 約 1,510 |

DOSBox 的 `cycles` 是每毫秒執行的指令數（normal core 一道指令一個 cycle），所以 240 cycles ≈ 每秒 240,000 道指令。

## 3. 規格

### 3.1 機器

- `Machine.SetInstructionsPerSecond(ips uint64)`：`IRQ0Base ＝ round(ips × 17000 ÷ PITBaseHz)`，然後重算 IRQ0 間隔；
  `ips ＝ 0` 還原 `DefaultIRQ0Every`。
- `Machine.InstructionsPerSecond() float64`：`IRQ0Base × PITBaseHz ÷ 17000`；未設時等於 `StepsPerSecond()`。
- 下列換算改用 `InstructionsPerSecond()`，跟著機器速度走：`HoldKey` 的 typematic 延遲與週期、`TonePCM` 的時間軸。
  `StepsPerSecond()`（套件函式）保留原意，當作預設速度；其他呼叫端不在本規格變更。
- 週期時鐘開著時，本設定不影響 IRQ0（照舊由週期決定），但換算仍依 `InstructionsPerSecond()`。

### 3.2 probe

- `-ips <數字|xt|at8|at12>`：預設名稱依 §2（240,000／750,000／1,510,000）。
- 在載入狀態檔之後套用（狀態檔會還原 `IRQ0Base`，與 `-cpuhz` 同樣的理由）。
- 與 `-cpuhz` 同時給時報錯。
- `-hold` 的 `ms` 換算改用機器速度：**先套 `-ips` 再解析 `-hold`**。

## 4. 驗收

1. `SetInstructionsPerSecond(240000)` 後，分頻 16571 的 IRQ0 間隔 ＝ `round(240000 ÷ (PITBaseHz ÷ 16571))` ±1（約 3,333）。
2. `InstructionsPerSecond()` 讀回設定值 ±0.1%；設 0 後讀回 `StepsPerSecond()`。
3. 狀態快照與狀態檔保存並還原設定（`IRQ0Base` 本來就在裡面，補測試）。
4. `HoldKey` typematic：同樣按住「1 秒份的指令數」，在 240,000 與預設速度下，重複次數相同（因為都換算成 1 秒）。
5. probe：`-ips xt` 配 `-load-state`，印出的 IRQ0 間隔是 XT 速度的值；`-ips` 與 `-cpuhz` 同時給報錯。
6. 反向對照：`SetInstructionsPerSecond` 不重算 IRQ0 間隔時，第 1 項失敗。

## 5. 不做

- 週期時鐘的週期表調整。
- 牆上時間對齊（屬前端）。
