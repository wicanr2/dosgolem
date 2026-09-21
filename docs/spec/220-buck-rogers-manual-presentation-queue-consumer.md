# 220 — Buck Rogers 手冊 presentation queue consumer

狀態：**DRAFT（純核心）**  
日期：2026-09-21  
前置：[`216-buck-rogers-manual-presentation-lifecycle.md`](216-buck-rogers-manual-presentation-lifecycle.md)、
[`217-buck-rogers-manual-multiline-presenter-core.md`](217-buck-rogers-manual-multiline-presenter-core.md)。

## 目的與邊界

把 `Watcher.PresentationEvents()` 的 append-only value snapshot 依序餵給既有
`RuntimeManualOverlay`。consumer 是 `apps/buckrogers` 的純協調核心：它只保存已成功套用的
event value prefix 與 presenter reference；不得由 `Observations()`、framebuffer、原版 machine 或
任何推測重建事件。

它不得繪製、呼叫 `Frame` 或 `Draw`、讀寫 VRAM、建立字型、碰觸 catalog、送鍵、判讀答案、改動
DOS／BIOS input、原版規則、檔案或存檔。這也不是 runtime loop、CLI 或正常玩家路徑的接線。

## 已證實輸入契約

spec 216 已 CONFORMED 下列 public value event：

```go
type ManualPresentationEvent struct {
    Step       uint64
    Kind       ManualPresentationKind
    Generation uint64
    Request    DisplayRequest
}
```

其中 `DisplayRequest` 只含 `uint64` 與 string；因此 event 為可比較的 value。`
PresentationEvents()` 回傳新的 slice，watcher 不會讓 consumer 回寫其歷史。spec 217 的
`RuntimeManualOverlay.Apply(event)` 已為 begin、clear、request 定義嚴格 lifecycle 驗證，且在
錯誤前驗證 generation、state 與 request identity；consumer 不得重複這些遊戲／catalog 判讀。

## 擬定 typed 契約

```go
type ManualPresentationConsumer struct { /* 私有 overlay 與 consumed prefix */ }

func NewManualPresentationConsumer(overlay *RuntimeManualOverlay) (*ManualPresentationConsumer, error)
func (c *ManualPresentationConsumer) Consume(events []ManualPresentationEvent) (newEvents int, err error)
func (c *ManualPresentationConsumer) Consumed() ([]ManualPresentationEvent, error)
```

建構子拒絕 nil presenter。`Consume` 對每個 snapshot 先檢查：其長度不可少於已消費 prefix，且
`events[:len(consumed)]` 必須逐筆 value-equal。任何縮短或歷史 mutation 都在套用新 event 前失敗
即關閉，cursor、presenter 與 action 均不得變動。

通過 prefix 檢查後，consumer 由 cursor 起逐筆呼叫 `overlay.Apply`。僅在該呼叫成功後，才將同一
event value append 到私有 history。若第 N 個新 event 失敗，回傳此前成功數與 error；先前成功的
event 保持已消費，失敗 event 及其後 event 不前進，且不做不可能安全的 presenter rollback。相同
snapshot 的重送只會重試該失敗 event，不會重套 prefix。完整已消費 snapshot 則成功回傳 zero，
不重複增加 overlay action。

`Consumed` 回傳 slice copy；呼叫端改動其元素或長度不得污染 consumer。nil receiver 的所有方法
都必須回 error。

## DRAFT 驗收與停止線

測試必須覆蓋：begin→clear→request 的逐次 append、完整 snapshot replay no-op、prefix mutation、
snapshot shrink、未知／不合 lifecycle 的 event、部分失敗後 cursor 只停在失敗 event 前，以及
history defensive copy 與 nil receiver。每個成功／失敗案例都要檢查 consumer cursor 及
`RuntimeManualOverlay` 的可見 state／actions 沒有越界變動。

實作前須確認 event 的完整 value 比較確實可編譯，`Apply` 對失敗 event 不會有未被契約允許的部分
狀態變動，且新檔案沒有 `machine`、`oracle`、input、command 或 renderer 依賴。正常遊戲餵入 watcher、
原版同狀態收據、字型原始來源、RGBA A/B 及 host frontend 均是後續工作，不能由本核心宣稱完成。

## READY 證據審查

目前 `ManualPresentationEvent`、`DisplayRequest` 的所有欄位均為 `uint64`、string 或其 value struct；
以完整 event 做 Go value equality 可編譯，能同時偵測 step、kind、generation 與 request identity 的
歷史漂移。watcher accessor 已回傳 slice copy，而 consumer 自己再保存 value append，兩層皆不存在可回寫
原版觀察器的 pointer／slice alias。

`RuntimeManualOverlay.Apply` 對 begin 在 generation 驗證後才 reset、對 clear 在 generation 與 state
驗證後才 clear、對 request 在 identity／catalog／build 成功後才換入 layers 與 append action；錯誤 event
不會產生成功 action。故 consumer 可將「先 Apply 成功、再 append cursor」作為唯一提交點，並在中段錯誤
時誠實保留已提交 prefix，不宣稱 transaction rollback。

此協調器只需 `fmt` 的錯誤訊息，不需 import `machine`、`oracle`、input、`cmd` 或 renderer。typed input、
cursor transition、失敗模式、測試與停止線均已具體，故本規格升為 **READY（純核心）**；只授權本節的
consumer 與單元測試，不授權 runtime loop 或玩家畫面接線。

## CONFORMED 純核心收據

2026-09-21，新增 `ManualPresentationConsumer`。它保有私有 event value prefix，在讀入每份
`PresentationEvents()` snapshot 時先完整比對已消費歷史；縮短與任一欄位漂移均會在呼叫 presenter
前拒絕。新 event 只在 `RuntimeManualOverlay.Apply` 成功後才 append，故完整 replay 回傳 zero、不會
重複 action；中段失敗只保留此前成功 prefix，下一次同 snapshot 只重試失敗 event。

Docker 的 `go test ./apps/buckrogers`、`go vet ./apps/buckrogers`、
`go test -race ./apps/buckrogers` 均通過。測試覆蓋 begin→clear→request 分批 append、完整 replay、
history mutation、snapshot shrink、無 begin request、未知 kind、部分失敗、defensive-copy 與 nil
consumer／presenter。consumer 檔案唯一直接 import 是標準庫 `fmt`，沒有 `machine`、`oracle`、input、
`cmd`、renderer 或原版資料依賴。

因此本規格升為 **CONFORMED（純核心）**。它只證實 value queue 至 presenter lifecycle 的一次性
消費，不會自行接收 watcher、驅動畫面、繪製中文、提供字型、變更遊戲答案或建立原版同狀態收據；
那些各自仍須獨立 DRAFT→READY gate。
