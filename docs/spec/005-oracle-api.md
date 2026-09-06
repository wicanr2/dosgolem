# 005 — 對外的 Go API

狀態：**READY**
日期：2026-09-04（DRAFT 2026-09-04，同日以 MVP-B 的實測結果定案）
前置：[`001`](001-scope-and-mvp.md) §5（M4）、[`004`](004-dos-bios-services.md)

---

## 1. 這一層存在的理由

整個專案的價值在這裡：**讓對拍的兩邊活在同一個 Go 行程裡**。

```go
o, _ := oracle.Load(exe, root)
o.RunUntil(oracle.PasswordScreen)   // 跑到防拷畫面
o.Click(102, 125)                   // 選綠色
o.Click(55, 105)                    // 點留白區
rent := o.Word(oracle.DS(0x1BE))    // 直接讀原版的變數
shot := o.Indexed()                 // 320×200 色號，不經過 X
```

## 2. 為什麼不能用 `internal/`

`internal/` 是 Go 的可見性邊界：`github.com/wicanr2/dosgolem/internal/...`
**只有 dosgolem 自己 import 得到**。`rich2`（`github.com/anr2/rich2`）是另一個
module，編譯期就會被拒絕。

所以要有一個 public package。名字定為 **`oracle`**（`docs/spec/082` 就是用這個詞
描述這件事），放在 repo 根目錄下的 `oracle/`。

`internal/` 維持現狀不動：`cpu`／`machine`／`dos` 是實作，`oracle` 是契約。
契約要窄——外面看得到的每一個型別與函式都是以後不能隨便改的東西。

## 3. 已定案的三個問題（原 DRAFT §3）

### 3.1 位址：型別化，而且以 **IDA 線性位址**為主

`rich2` 的每一份筆記都用 `RUN_full.EXE` 的 IDA 線性位址，
字串（`"ds:1BE"`）好讀但要到執行期才失敗。定案：

```go
type Addr struct{ Seg, Off uint16 }

func DS(off uint16) Addr    // rich2 筆記裡的 ds:XXXX
func IDA(linear uint32) Addr // rich2 筆記裡的五位線性位址
func Seg(seg, off uint16) Addr
```

換算（**MVP-B 實測，不是推的**）：

| 常數 | 值 | 怎麼定出來的 |
|---|---|---|
| `IDAOffset` | `0xEF00` | 執行期 `3014:167F` ＝ IDA 線性 `406BF`（防拷的計時器等待迴圈）|
| `DGROUPSeg` | `0x32F9` | `ds:` 的 IDA 線性基底 `41E90` 減 `IDAOffset` ＝ `32F90`；**那正是執行期的 SS**，編譯後 BASIC 的 DGROUP 與堆疊同段 |

驗證：`DS(0x1B5A)` 讀到 `00 00 80 3F` ＝ IEEE 754 的 `1.0f`，
正是 `rich2/CLAUDE.md` §4.1 用來獨立求解 DGROUP 基底的那個值。

> **`IDAOffset` 只在映像載在 `machine.LoadSeg` 時成立。**
> 換載入位置就要重算，而錯了不會報錯——只會讀到看起來合理的別的變數。
> `Load` 因此把它算出來存在 `Oracle` 上，不寫死成套件常數。

### 3.2 `RunUntil`：跑不到是**錯誤**，不是靜靜地回來

```go
func (o *Oracle) RunUntil(c Cond, opts ...RunOpt) error
type Cond func(*Oracle) bool
```

- 預設上限 `DefaultBudget`（1 億道指令，約 5 秒）。用 `Budget(n)` 改。
- **跑滿上限而條件沒成立要回 `*BudgetError`**，帶上停在哪、跑了幾道、
  畫面上有沒有東西。安靜地回來會讓「條件寫錯」與「程式真的沒走到」
  長得一模一樣（`~/diagnosis-notes/docs/03-silence-is-not-success`）。
- 程式自己結束（`AH=4Ch`）或 CPU 出錯也是錯誤，不是「條件不成立」。

內建條件：

| | 意義 |
|---|---|
| `At(addr)` | `CS:IP` 走到這裡 |
| `Called(addr)` | 這個位址被 `call` 進去（比 `At` 便宜，見 §5）|
| `Opened(name)` | 程式開了某個檔——**載入進度最可靠的路標** |
| `ScreenIdle(n)` | 畫面連續 `n` 道指令沒變 |
| `Steps(n)` | 純跑 `n` 道 |

### 3.3 畫面回**索引**，不回 RGB

`Indexed()` 回 320×200 色號陣列。`rich2` 的比對本來就在色號空間做，
而且 MVP-B 的驗收是逐點色號相同。`Palette()` 另外提供 256×3 的 RGB。

> **不要在這一層做 RGB 合成。** 一旦回 RGB，比對就得處理「色號不同但顏色相同」
> 與「調色盤循環相位」兩件事——MVP-B 實測那 10 個循環色（240–249）
> 相位差 2 而畫面完全相同。索引空間裡這個問題不存在。

## 4. 輸入：對齊**指令數**，不是牆上的時鐘

這是整份規格最重要的一節。`rich2` 現在 54 支腳本、6,116 行，
瓶頸不是 IPC 是 sleep（`docs/spec/082` §1：0.058 秒的 exec 旁邊掛 0.35–2.2 秒
的等待，比例 6–38 倍），而且**慢與不穩是同一個原因**。

```go
func (o *Oracle) MoveMouse(x, y int)
func (o *Oracle) Click(x, y int) error       // 移動 → 按下 → 按住 → 放開
func RightButton() ClickOpt                  // Click改送右鍵按下／放開
func (o *Oracle) Type(s string) error        // 餵給 int 21h AH=3Fh
func (o *Oracle) PressKey(key Key)            // 以IRQ1送Set 1 make／break掃描碼
```

三條實測出來的紀律：

1. **按住時間用指令數**（`DefaultHold`，200 萬道）。遊戲輪詢 `int 33h` 的頻率
   很低，按下與放開隔太近會整個被跳過——DOSBox 那邊同一題要點三次才生效
   一次（`rich2/docs/playtest/001` §5.6）。
2. **移動要在程式的 `AX=4` 之後**。程式進防拷畫面時會用 `AX=4` 設一次游標
   位置（MVP-B 實測在第 42,406,064 道）；在那之前移動會被它蓋掉，
   而且**畫面看起來完全正常**。`Click` 因此先 `RunUntil(mouseSettled)`。
3. **`Click` 回 error**。點了沒反應要說出來，不要讓呼叫端拿「畫面沒變」
   去猜是點錯位置還是遊戲還沒準備好。

`Click`預設維持左鍵。`RightButton`只改IBM／Microsoft滑鼠契約中的按鍵位元與callback事件：
按住狀態`BX=2`，右鍵按下／放開事件分別是`0008h`／`0010h`；hover、hold、settle、畫面回應及
失敗即關閉規則完全相同。這是平台輸入能力，不含任何EOB專用座標或語意。

2026-09-07驗收：`internal/dos.TestMouseCallbackReportsRightButtonContract`釘住callback的
`AX=0008h`／`BX=2`；EOB1真實資料測試再由`RightButton`進入角色交換並完成取消，證實公開API到
原版consumer的垂直鏈。右鍵增量契約為CONFORMED；本文件其餘READY範圍不因此整體升格。

## 5. 攔截與狀態

```go
func (o *Oracle) OnCall(addr Addr, fn func(*Oracle))  // 攔繪製常式讀參數
func (o *Oracle) Save() *State
func (o *Oracle) Restore(s *State)
func (o *Oracle) PortReads() map[uint16]uint64        // I/O埠讀取次數的唯讀副本
```

- `OnCall` 的價值是**把判準從像素換成參數**：原版自己傳給繪製常式的
  座標與字串，比從畫面反推可靠（`rich2/docs/lessons.md` D34 就是像素判準誤中）。
- `Save`／`Restore` 是 1 MB 記憶體 ＋ CPU ＋ DOS 狀態的深拷貝。
  開機到防拷畫面要 4,200 萬道指令（約 2 秒）；走到棋盤還要更多。
  **從同一個狀態展開多個變體**是 `rich2` D33／D34「走到罕見畫面很貴」的解。
- `State` 是不透明的；**不落地成檔案**（那等於散布原版的記憶體映像）。
- `PortReads`只供定位「程式繞過DOS／BIOS，直接輪詢硬體」；回傳副本，呼叫端不可藉此修改
  machine狀態。它證明埠被讀，不自行推導該埠語意或輸入契約。
- `Type`與`PressKey`是不同契約：前者維持DOS標準輸入相容性；後者只供自行掛接
  `int 09h`的程式。`PressKey`送IBM PC/AT Set 1 make碼，再送最高位為1的break碼。

## 6. 亂數

```go
func (o *Oracle) OnRandom(fn func(seq int, value float32))
```

`rich2/WORKLIST.md` P1.1：原版 BASIC 的 `RANDOMIZE` 與 remake 的 24 位元 RNG
沒有可重播對應。要接這條線得先反組譯 BASIC runtime 的 `RND`，
**位址還沒解**——所以這一節標 `DRAFT`，介面先留著，不實作。

## 7. 硬規則

1. **不含任何原版檔案。** `Load` 吃玩家自備的路徑，`State` 不落地。
2. **`internal/` 不對外。** 要暴露什麼就在 `oracle` 明確地包一層，
   不要為了省事把內部型別漏出去——漏出去的那一刻它就變成契約。
3. **每一個「跑到某處」都要有上限**，逾時是錯誤。
4. **回索引不回 RGB**（§3.3）。
