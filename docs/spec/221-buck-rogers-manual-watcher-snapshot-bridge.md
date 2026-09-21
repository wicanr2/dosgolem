# 221 — Buck Rogers 手冊 watcher snapshot bridge

狀態：**CONFORMED（純核心）**
日期：2026-09-21
前置：[`216-buck-rogers-manual-presentation-lifecycle.md`](216-buck-rogers-manual-presentation-lifecycle.md)、
[`217-buck-rogers-manual-multiline-presenter-core.md`](217-buck-rogers-manual-multiline-presenter-core.md)、
[`220-buck-rogers-manual-presentation-queue-consumer.md`](220-buck-rogers-manual-presentation-queue-consumer.md)。

## 目的與邊界

將已證實的 `Watcher.PresentationEvents()` defensive value snapshot 交給既有
`ManualPresentationConsumer`，使未來 runtime 有唯一、窄的 watcher→consumer 接點。bridge 只保存
`*Watcher` 與 `*ManualPresentationConsumer` reference，不保存 cursor、request、renderer、framebuffer
或原版狀態；consumer 仍是唯一的歷史與 cursor authority。

bridge 不得讀 `Observations()`、直接讀 machine／oracle、安裝 hook、改寫 watcher、畫 RGBA、呼叫
`Frame`／`Draw`、載入字型、送鍵、改答案、碰觸 DOS／BIOS input、檔案或存檔。它不是 command、遊戲 loop、
host frontend 或正常玩家路徑接線。

## 已證實前提

- spec 216 已 CONFORM：`Watcher.PresentationEvents()` 使用 `append([]ManualPresentationEvent(nil), ...)`
  回傳新的 value slice；正式 watcher 僅在 exact begin、active-context clear 與 catalog-hit request
  append answer-free event，未從 `Observations()` 轉換。
- spec 220 已 CONFORM：`Consume` 對已消費 prefix 做完整 value equality，僅在
  `RuntimeManualOverlay.Apply` 成功後 append cursor；重播無 action，中段錯誤只保留成功 prefix。
- `Watcher.PresentationEvents()` 對 nil receiver 沒有契約，因此 bridge 必須在呼叫它之前拒絕 nil watcher；
  constructor 與 `Sync` 均須 fail-closed。

## 擬定 typed 契約

```go
type ManualPresentationBridge struct { /* 私有 watcher 與 consumer */ }

func NewManualPresentationBridge(watcher *Watcher, consumer *ManualPresentationConsumer) (*ManualPresentationBridge, error)
func (b *ManualPresentationBridge) Sync() (newEvents int, err error)
```

constructor 只接受非 nil watcher 與 consumer。`Sync` 先驗證 receiver 與兩個 reference，隨後只執行：

```go
return b.consumer.Consume(b.watcher.PresentationEvents())
```

它不得保存第二份 snapshot 或 cursor、不得 catch-and-ignore consumer error、不得 retry／重排 event。回傳值與
錯誤保留 consumer 的語意：完整重播回傳 zero；歷史漂移／縮短在 presenter 前拒絕；中段 error 保留成功
prefix，下一次 `Sync` 從同一失敗位置重試。

## DRAFT 驗收與停止線

測試必須用 watcher presentation value 建立 begin→clear→request，驗證分批 append、完整 replay zero-op、
watcher 提供的歷史漂移會被 consumer 拒絕且不重複 action、非法新 event 的 consumer error 會原樣成為
bridge error，以及 nil watcher／consumer／receiver 一律 error。測試可直接構造 package-private synthetic
watcher state，但不得稱為正常玩家原版收據。

實作前必須確認 `PresentationEvents()` 是唯一讀取點、其 slice copy 足以避免 bridge 回寫 watcher，且
consumer 無須也不應保存第二份 cursor。若未來需要 concurrent Sync，必須另開 DRAFT；本規格只保證既有
watcher／consumer 的 serial owner 呼叫，不新增鎖或未證實的 goroutine 語意。

## READY 證據審查

目前 watcher 的唯一 public lifecycle accessor 就是 `PresentationEvents()`；其實作直接複製
`presentation` slice，而 `ManualPresentationEvent` 及內含 `DisplayRequest` 均是 value／string fields。
bridge 對該 accessor 的單一呼叫既不會得到 watcher 內部 slice，也沒有任何必要接觸
`Observations()`、collector、machine 或 oracle。

consumer 已把所有 cursor、prefix equality、逐筆 Apply、partial failure 與 defensive history 封裝；
bridge 若再保存 state，只會形成兩份可能漂移的 history，沒有新增原版證據或玩家可見價值。nil watcher
必須在 accessor 前阻擋，nil consumer／receiver 同理；其餘 error 由 consumer 完整傳回，避免 bridge
將無效 lifecycle 偽裝成已同步。

因此 typed reference、單一轉送、失敗模式、serial ownership 與測試範圍均已明確，故本規格升為
**READY（純核心）**，只授權本節 bridge 及 synthetic test；不授權任何 command、frame loop、字型或
正常玩家 runtime 接線。

## CONFORMED 純核心收據

2026-09-21，新增 `ManualPresentationBridge`。constructor 在呼叫 watcher accessor 前拒絕 nil
watcher／consumer；`Sync` 不保存 state，只把 `PresentationEvents()` 的當前 defensive snapshot 原樣
交給 consumer。完整 watcher snapshot replay 回傳 zero，沒有重複 presenter action；consumer 的
history-drift、非法 lifecycle 與 partial-failure error 都未被 bridge 吞掉或改寫。

Docker 的 `go test ./apps/buckrogers`、`go vet ./apps/buckrogers`、
`go test -race ./apps/buckrogers` 均通過。測試覆蓋 begin→clear→request 的分批 watcher append、完整
replay、合成 history drift、clear generation 不符的 partial failure、重試失敗 event，以及 nil
watcher／consumer／bridge。bridge 檔案唯一直接 import 是標準庫 `fmt`，沒有 machine、oracle、input、
`cmd`、renderer 或 font 依賴。

因此本規格升為 **CONFORMED（純核心）**。它只證實 watcher value snapshot 至 consumer 的窄轉送；
不會接原版 hook、command、frame loop、字型、RGBA 或玩家畫面，也不會改變答案、輸入、原版資料或存檔。
