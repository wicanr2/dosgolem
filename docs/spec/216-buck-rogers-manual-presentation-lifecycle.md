# 216 — Buck Rogers 手冊 presentation lifecycle

狀態：**CONFORMED**  
日期：2026-09-21  
前置：[`007-buck-rogers-manual-event-adapter.md`](007-buck-rogers-manual-event-adapter.md)、
[`008-buck-rogers-manual-runtime-watcher.md`](008-buck-rogers-manual-runtime-watcher.md)、
專案第 18、24、85 階段原版收據。

## 目的與邊界

將既有 `apps/buckrogers.Watcher` 的已證實手冊題目世代邊界提供給未來 presentation layer。
這是 output-side metadata queue：它不得繪製、不得寫原版機器、不得傳遞英文原文或答案、不得
送鍵、不得改變 DOS／BIOS input、存檔或 catalog identity。

## 原版證據

- `2A33:01ED` 的精確 `In the Log Book on page` entry 會呼叫 `Collector.BeginEntry`，建立新的
  generation 並清空舊 visible。
- `026F:029C` clear 在第 18 階段的答錯重抽、以及第 24 階段正常第一題中，都可發生在 pending
  題目的內容尚未完整前；它不得清空 collector pending metadata。
- 僅 `2A33:0309` 的 `word?` 在 `0763:0424` 三重 guarded post-call 成功後才可完成題目並依
  exact catalog 產生 `DisplayRequest`。catalog miss、guard drop、nested frame、錯誤 caller／SS／SP
  都不產生 request。

上述位址均是原版執行期實模式 `segment:offset`。來源版本、state 與命令由專案
`docs/re/phase-24-manual-runtime-watcher.md` 及第 85 階段收據固定；本規格不新增原版語意。

## 擬定 typed 契約

```go
type ManualPresentationKind string

const (
    ManualPresentationBegin   ManualPresentationKind = "begin"
    ManualPresentationClear   ManualPresentationKind = "clear"
    ManualPresentationRequest ManualPresentationKind = "request"
)

type ManualPresentationEvent struct {
    Step       uint64
    Kind       ManualPresentationKind
    Generation uint64
    Request    DisplayRequest // 僅 Kind=request 時非零；其 Generation 必須相同。
}
```

`Watcher.PresentationEvents()` 回傳 queue 的 value-copy。`DisplayRequest` 與 event 只含 strings
及 value fields，因此改動呼叫端取得的切片或元素不得污染 watcher；queue 沒有 callback、renderer、
machine、oracle 或 input 參照。

## 擬定狀態轉移

1. 精確 begin 成功後，追加 `begin(step, generation)`。它必須先於同 generation 的任何 clear 或
   request，並代表 presentation 應移除前一 generation 段落。
2. 只有 collector 在 clear 前處於 pending 或 visible 手冊 context 時，`026F:029C` 才追加
   `clear(step, generation)`。clear 一律保留 collector pending；若是已完成 visible，presentation
   可移除該 generation 段落。沒有 active manual context 的通用 clear 不進 queue。
3. 僅 exact catalog hit 追加 `request(step, generation, request)`；`request.Generation` 必須等於事件
   generation。catalog miss、nil catalog、poison、guard failure、nested frame 與 stale return 不追加。
4. 原有 `Observations()` 與 `Requests()` 行為保持相容；此 queue 只新增 presentation metadata，
   不把舊 observation 作為 lifecycle API。

## READY 證據審查

`Collector` 的 `pending != nil` 與 `hasVisible` 都在 `apps/buckrogers/manual.go` 同一封裝內維護；
`BeginEntry`、`PostCall`、`ClearEntry` 的既有測試已證實其轉移。watcher 在呼叫 `ClearEntry` 前讀取
這兩個狀態，便可判斷該 exact clear 是否屬於 active manual context，完全不需要讀 framebuffer、
猜測 `Observations()` 陣列或導入新原版 hook。

`DisplayRequest` 為四個 value／string 欄位，沒有 slice、pointer、machine 或 input reference；
`ManualPresentationEvent` 以 value 持有它，且 queue accessor 回傳新切片，因此可以用既有
`Requests()` defensive-copy 模式驗證呼叫端不能回寫。begin、clear、request 的來源分別已由
008 的 exact watcher 分支、collector 及 catalog hit 證實；其餘分支只會記 observation 或 fail-closed，
不會產生 lifecycle event。

metadata receipt 會將 request 投影為 event/text key 與 rune count，而不序列化 `Translation`；
這維持第 24 階段的資料邊界。所需 typed input、狀態、失敗模式、垂直鏈與驗收均已明確，故本規格
升為 **READY**，只授權本節所列 queue 與收據投影實作。

## CONFORMED 收據

- `go test ./apps/buckrogers`、`go vet ./apps/buckrogers ./cmd/buckrogers-receipt` 與
  `go test -race ./apps/buckrogers` 均通過。新測試覆蓋 begin→pending-clear→request、visible clear、
  catalog miss、沒有 manual context 的 clear、guard failure、nested／stale failure 及 queue／request
  value-copy。
- 以第 24／85 階段相同的正常玩家 fixed state、唯讀 `/orig` 原版資料與零鍵盤注入重跑到
  #266,557,247。既有 request 仍為 generation 1、
  `manual.page34.deimos_prison.word10`／`manual.log.49.deimos_prison`、73 runes；新增 queue 僅為：
  #266,486,493 begin、#266,524,821 pending clear、#266,557,246 request，三筆均為 generation 1。
- queue 只由 watcher 的 value／string state 建成，沒有 `machine`、`dos`、`oracle` write 或 input
  reference。receipt 只投影 request key 與 rune count，沒有輸出 translation 全文、英文原文或答案。

因此此規格在固定版本、上述 state 與 lifecycle 範圍內為 **CONFORMED**。它不宣稱 14 行手冊
presenter、字型、RGBA 像素或 host UI 已完成；那些仍由專案規格 005 的後續 READY gate 管理。
