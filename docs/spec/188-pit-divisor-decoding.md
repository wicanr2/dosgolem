# 188 — PIT 除數解碼：頻率是量出來的，不是某一款遊戲的常數

狀態：**READY**
日期：2026-09-09
動機：rich2 的逐幀對拍要拿「原版每秒幾刻」當單位，而那個數字先前
寫死在 remake 與 dosgolem 兩邊的註解裡

---

## 1. 問題

「原版跑 70.187 Hz」這件事先前只存在於**註解**：
`DefaultVGAFrameEvery` 的說明寫「因為 rich2 把 PIT 設成 70.187 Hz」，
rich2 的 `screen.OriginalFrameHz` 也自己寫了一份 `1193182 / 17000`。

兩個問題：

- **換一支程式就不成立。** 《臥龍傳》的 `YNSOUND.COM` 寫的是 4,096
  （291.3 Hz），《炎龍騎士團2》的 AIL 寫 0（＝ 65,536，18.2 Hz）。
  拿 70.187 去換算它們的計時刻會差四倍到四十倍。
- **沒有東西會在它錯掉時報錯。** 除數是程式在執行期寫進 8254 的，
  註解裡的數字與實際跑的值之間沒有任何連結。

## 2. 事實：除數在埠寫入裡，每支程式都一樣

8254 的通道 0 重載固定是兩步：

| 埠 | 內容 |
|---|---|
| `0x43` | 命令位元組：bits 7–6 通道、5–4 存取方式（`01` 只低、`10` 只高、`11` 先低後高、`00` 鎖存不改設定）、3–1 模式、0 BCD |
| `0x40` | 除數，依存取方式送一或兩個 byte |

**模式與 BCD 不影響除數**，所以解碼不挑模式——挑了就會漏掉用模式 2 的程式
（rich2 用模式 3、`RUN_audio.EXE` 的驅動用模式 2）。

寫 0 代表 65,536，這是 8254 的定義，不是「沒設定」。兩者要分開回答：
`PITProgrammed()` 說有沒有被寫過，`PITDivisor()` 說現在是多少。

輸入時脈是硬體常數：NTSC 彩色副載波 `315/88` MHz ×4 ÷12 ＝ `315/264` MHz
＝ **1,193,181.8181… Hz**。文獻常寫 1,193,182，那是四捨五入（差 3×10⁻⁷）；
兩邊常數要對拍就得用精確值，否則會差在小數第五位。

## 3. 分層

除數解碼屬於**觀測層**（`internal/machine`），不是程式層：任何 DOS 程式都寫同一組
埠，不必知道那支程式長什麼樣（判準見 `006-layering`）。

實模式的 `Machine` 與保護模式的 `LEMachine` **共用同一份解碼器**——
保護模式的遊戲一樣是 `out 43h` / `out 40h`。

```go
const PITBaseHz = 315e6 / 264   // 硬體常數
const PITDefaultDivisor = 65536 // BIOS 開機值

func (m *Machine) PITDivisor() uint32    // 目前的除數
func (m *Machine) PITProgrammed() bool   // 程式自己設過沒有
func (m *Machine) PITHz() float64        // PITBaseHz / 除數
// LEMachine 有同名的三支
```

## 4. 指令數換算

dosgolem 的時間由**指令數**驅動，所以「一刻幾道指令」要能從除數換算：

```go
const StepsPerSecond = DefaultIRQ0Every * PITBaseHz / 17000
func PITStepsPerTick(divisor uint32) uint64
func (m *Machine) PITStepsPerTick() uint64
```

`StepsPerSecond` 不是量到的硬體規格，是從對拍釘住的 `DefaultIRQ0Every`
（rich2 的一刻 ＝ 165,000 道指令）反推的機器速度 ≈ 11.58 M 指令／秒，
落在 386DX-33／486SX 的量級。有了它，除數換成指令數就不必再引用
任何一款遊戲的數字。

⚠ **機器不自動跟隨。** `IRQ0Every` 維持由呼叫端設定：既有的錄影與逐幀對拍
都釘在固定的 `IRQ0Every` 上，讓它隨遊戲的埠寫入改速度會把那些收據全部作廢。
要跟隨的呼叫端自己寫 `m.IRQ0Every = m.PITStepsPerTick()`。

## 5. 觀測層 API

```go
func (o *Oracle) TimerDivisor() uint32   // 遊戲寫進去的除數
func (o *Oracle) TimerProgrammed() bool
func (o *Oracle) TimerHz() float64
func (o *Oracle) StepsPerTick() uint64   // 目前的 IRQ0Every
```

## 6. 驗收

1. 合成埠序列：先低後高、只低、只高、寫 0 → 65,536、其他通道與鎖存命令
   不改設定、沒被設過時回 BIOS 的 18.2 Hz。
   （`internal/machine/pit_test.go`，全數通過）
2. `PITStepsPerTick(17000)` 剛好等於 `DefaultIRQ0Every`。同上，通過。
3. **實跑原版**：`apps/rich2` 走到棋盤後 `TimerDivisor()` ＝ 17,000、
   `TimerHz()` ＝ 70.187166 Hz、`TimerProgrammed()` ＝ true
   （`apps/rich2/state_test.go` 的 `TestPITDivisorIsGameProgrammed`，
   16.1 秒，通過）。
4. **保護模式實跑**：FD2 的 AIL 走到 `0x3E882` 之後 `PITProgrammed()` ＝ true、
   除數 ＝ 65,536、頻率 ＝ 18.2065097 Hz
   （`internal/machine/le_machine_test.go` 的 `TestFD2ProgramsPITDivisor`，通過）。
5. **跨儲存庫**：rich2 的 `screen.OriginalFrameHz` 與 `TimerHz()` 相差
   小於 1e-6（`rich2/internal/parity/timerhz_test.go` 的
   `TestOriginalFrameHzMatchesOracle`，16.4 秒，通過）。

第 3 與第 5 條是這份規格存在的理由：**沒有它們，「原版 70.187 Hz」
就只是一句沒有人在檢查的註解。**
