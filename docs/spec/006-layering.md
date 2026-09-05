# 006 — 分層：通用的、runtime 的、程式專屬的

日期：2026-09-05
狀態：**READY**（使用者裁決 2026-09-05：「允許拆遷依照你的規劃」）

---

## 1. 為什麼現在要分

第一個案例（`rich2`）把機器層與觀測層推到能用了。第二個案例出現時，
第一件被問到的事就是「哪些可以直接用、哪些要自己寫」——而現在的目錄
回答不了這個問題：`oracle/rich2/` 裡混著三種性質完全不同的東西。

如果不分，第二個案例只能 **fork**。fork 之後兩邊的機器層各自演化，
補在 A 的 DOS 服務 B 拿不到，這個工具就退化成一次性腳本的模板。

## 2. 四層，以及判準

| 層 | 位置 | 判準：什麼東西屬於這裡 |
|---|---|---|
| **機器** | `internal/cpu`／`internal/dos`／`internal/machine` | 只依賴 **Intel 手冊或 DOS/BIOS 規格**。看不到任何特定程式 |
| **觀測** | `oracle/` | 只依賴機器層。提供「問得到、可重播、走得到」的原語，不知道被觀測的是什麼程式 |
| **runtime** | `runtime/<編譯器>/` | 依賴**某個編譯器或執行期的慣例**，跨程式成立。例：MS BASIC 的 `RND`、陣列描述子、`retf N` 的參數慣例 |
| **程式** | `apps/<程式>/` | 依賴**某一支 binary 的位址**。換一個程式全部作廢 |

判準用一句話講：**「換一支 binary 之後，這段程式碼還成立嗎？」**

- 還成立，而且不必改 → 機器層或觀測層
- 換一支**同一個編譯器編的** binary 還成立 → runtime 層
- 不成立 → 程式層

### 2.1 這個判準怎麼用在現況上

| 現在在哪 | 東西 | 應該在哪 | 理由 |
|---|---|---|---|
| `oracle/basic.go` | `Array`／`Dim`／`Phys` | **runtime/basic** | BASIC 陣列描述子的版面（前兩個 word 是 `(位移, 段)`、列主序）是 BASIC 的慣例，不是通用 DOS 的 |
| `oracle/rich2/rng.go` | `LCGNext`／`RNDState`／`SetRNDState`／`TraceRND`／`RNDCall`／`RNDTrace` | **runtime/basic** | LCG 常數是從 DGROUP 讀出來的、`RND` 進入點是 BASIC runtime 的。**與大富翁2 無關** |
| `oracle/rich2/rng.go` | `DirPickCaller`／`DirectionPicks` | 留在程式層 | `0x11A32` 是大富翁2 的呼叫端 |
| `oracle/rich2/` 其餘 | `SolvePassword`／`ToBoard`／`PlayTurn`／`WatchDispatch`／`Deck`／`Hand`／`Desc*`／`Var*`／`Col*`／按鈕座標 | **apps/rich2** | 全是那一支 binary 的位址與流程 |
| `cmd/probe`／`cmd/state` | 通用探針 | 留在 `cmd/` | 吃 exe 路徑，不認識遊戲 |
| `cmd/solvepw`／`play`／`dice`／`rng`／`parity` | rich2 專屬 | **apps/rich2/cmd/** | 名字就是遊戲流程 |

> **`Array` 的位置是這份規格裡最容易搞錯的一項。** 它現在在 `oracle/`
> 看起來很合理——「讀陣列」聽起來很通用。但**通用的部分是 `o.Word`**，
> 「描述子前兩個 word 是 `(位移, 段)`」是編譯器的版面決定的。
> 換一個 Turbo Pascal 編的程式，那個解讀直接錯，而且**不會報錯**，
> 只會讀出一片看起來像資料的東西。

## 3. 目標結構

```
internal/cpu, internal/dos, internal/machine   機器
oracle/                                        觀測（Load/RunUntil/Click/Save/Search/Indexed/OnCall/PNG）
runtime/basic/                                 MS BASIC 編譯程式：RND、陣列描述子、呼叫慣例
apps/rich2/                                    大富翁2：位址、狀態、流程、攔截點
apps/rich2/cmd/                                大富翁2 的可執行工具
cmd/                                           通用工具（probe、state）
docs/spec/, docs/findings/                     規格與筆記
```

新增一個 runtime：`runtime/<名字>/`，照 `runtime/basic/` 的形狀。
新增一個程式：`apps/<名字>/`，照 `apps/rich2/` 的形狀。

### 2.2 拆完之後才看清楚的一條：**演算法通用，位址不通用**

第一版把 `RNDEntry = 0x2E3AA`、`Randomize`、狀態變數偏移都當成
「BASIC runtime 的常數」搬進 `runtime/basic`。**那是錯的**——runtime 是
**連結進去**的，同一份 MS BASIC 落在不同 binary 會在不同位址；DGROUP 的
配置也隨程式而異。

所以 `runtime/basic` 收的是**公式與流程**（LCG、追蹤、驗證），
四個位址收在 `basic.Config` 由呼叫端給：

```go
var BASIC = basic.Config{
	RNDEntry: 0x2E3AA, Randomize: 0x2E423,
	StateLo: 0x22D1, StateHi: 0x22D3,
}
tr := basic.TraceRND(o, BASIC)
```

**拿別的程式的值套過來不會報錯**，只會一次都攔不到——所以
`Trace.Verify` 的第一個檢查就是「一次都沒呼叫 RND，Config 的位址可能不對」。

## 4. 搬遷計畫（分三階段，每一階段各自可驗證）

| 階段 | 動作 | 驗收 |
|---|---|---|
| A | 建 `runtime/basic/`，把 `LCGNext`／`RNDState`／`SetRNDState`／`TraceRND`／`RNDCall`／`RNDTrace` 搬過去；`oracle/rich2/` 留型別別名轉接 | 兩邊 `go test ./...` 全綠，`rich2` 的 `-tags oracle` 不必改 |
| B | `Array`／`Dim` 搬進 `runtime/basic/`，`oracle` 留 `Phys` 與 `Word`／`Bytes` | 同上 |
| C | `oracle/rich2/` → `apps/rich2/`，`cmd/` 的 rich2 工具跟著搬；**這一階段會改 `rich2` 的 import 路徑** | `rich2` 改 import 之後對拍全綠 |

實際搬完的結果（2026-09-05）：

```
oracle/basic.go       → runtime/basic/array.go   （Phys 與 WriteU8/16 留在 oracle/write.go）
oracle/rich2/rng.go   → runtime/basic/rng.go     （位址那一半留在 apps/rich2/rng.go 的 basic.Config）
oracle/rich2/         → apps/rich2/
cmd/{solvepw,play,dice,rng,parity,state} → apps/rich2/cmd/
cmd/probe                                         留著——它吃 exe 路徑，不認識任何遊戲
```

`cmd/state` 原本以為是通用的，一看 import 就知道不是：它讀的是大富翁2 的
陣列描述子。**判準不是名字，是 import。**

使用者 2026-09-05 裁決照這個規劃拆，`rich2` 同一輪一起改，所以三階段
一次做完、**轉接層不做**——留下來只會變成沒有人用的技術債。
唯一保留的相容措施是 `apps/rich2` 裡四個薄包裝
（`RNDState`／`SetRNDState`／`TraceRND`／`LCGNext`）與兩個型別別名，
它們省掉每個呼叫端都要傳 `BASIC`，不是為了相容而留的。

## 5. 不做的事

- **不為了分層而抽象。** 沒有第二個實例的介面不要先設計——
  `Place`／`Dispatch` 這種形狀在第二個程式出現之前不知道對不對。
- **不把機器層做成外掛。** CPU 與 DOS 服務就是照規格補，補到夠用為止；
  補的人是誰的案例不重要。
- **不維護「支援哪些程式」的清單。** 只有被實際跑過的路徑算數，
  清單會過期；要知道能不能跑就跑 `cmd/probe` 看 `Unimplemented`。
