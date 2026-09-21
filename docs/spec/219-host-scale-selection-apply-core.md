# 219 — Host 倍率預選與 Apply 純核心

狀態：**CONFORMED（純核心；非 host frontend）**  
日期：2026-09-21  
前置：[`004-dos-bios-services.md`](004-dos-bios-services.md)、
[`202-translation-overlay.md`](202-translation-overlay.md)、專案 spec 004 與第八十一、八十二、八十四階段收據。

## 目的與邊界

建立沒有 backend 的 dosgolem 通用 host state，實現已確認的 C 語意：2×／3× option 只改
`selectedScale`，Apply 才能將它原子提交為 `activeScale`。它只保存兩個 output value，供未來
host presenter 讀取；不得開視窗、畫控制列、做 hit test、送事件、重啟 DOS 或接觸 machine、VRAM、
BIOS／IRQ、DOS mouse／keyboard、檔案、存檔、translation、catalog 或任何 game adapter。

本規格不選擇預設 2×或3×，也不定義 Apply 後面板收合、session／跨重啟持久化、前端 backend、
鍵盤 focus 或未命中 host hit event 的 forwarding。

## 已證實前提

- 使用者已明確選 C，排除 option 點擊立即套用：state 必須同時保存 active 與 selected；只有
  Apply 可改 active。這是產品決定，不從現有命令列或 DOS input 推論。
- dosgolem 現有 Buck Rogers overlay constructors 已明示只接受 2 或3，而 `xlate.Layer.Draw` 只以
  呼叫端給定的 scale 做 output RGBA，沒有 setter 或 host frontend。
- phase 84 已證實目前 repository 沒有既有視窗／frontend／event loop；`cmd/probe` mouse flags 是
  決定性 DOS 注入，不能重用為 host UI。因此核心必須在新通用 `host` package，且不得 import
  `internal/machine`、`internal/dos`、`oracle`、`apps/*` 或 `cmd/*`。

## 擬定 typed 契約

```go
package host

type OutputScale uint8

const (
    OutputScale2 OutputScale = 2
    OutputScale3 OutputScale = 3
)

type ScaleState struct {
    ActiveScale   OutputScale
    SelectedScale OutputScale
}

type ScaleController struct { /* 私有 ScaleState */ }

func NewScaleController(initial OutputScale) (*ScaleController, error)
func (c *ScaleController) Select(scale OutputScale) (ScaleState, error)
func (c *ScaleController) Apply() (state ScaleState, changed bool, err error)
func (c *ScaleController) Snapshot() (ScaleState, error)
```

`New` 只接受2或3，初始 active 與 selected 相同。`Select` 只接受2或3，只更新 selected；`Apply`
只可將 current selected 複製到 active，並在兩者原本不同時回 `changed=true`。成功回傳皆為 value copy；
caller 對回傳 state 的修改不能影響 controller。nil controller 與所有非法 scale 都回 error，且不得有
部分狀態變動。

## DRAFT 驗收與停止線

純核心測試必須覆蓋：initial 2／3、Select 2→3／3→2、Select 不改 active、Apply 的原子提交、
idempotent Apply、非法 zero／one／four、nil receiver 與 defensive-copy。`go vet`、package test 與
race detector 都必須通過。

這只證實 host state，不是可操作的玩家 UI 或實際 runtime scale switch。實作前仍需證據審查：確認
本 API 不表達未決的面板／持久化行為，且新 package 沒有 DOS／遊戲依賴。後端接線、host hit event
隔離、同 raw frame 重繪與正常玩家 switch A/B 全部留在後續 DRAFT。

## READY 證據審查

C 已固定 Select 與 Apply 的唯一玩家可見狀態轉移；初始 scale 是未來 backend 的明示 constructor
輸入，並非產品預設。面板收合、持久化、focus 與 forwarding 都不會改變此 controller 的兩個 value
或 API，故可留在 package 外而不猜補。repository 現有 package inventory 亦證實沒有可重用 host
implementation，將零依賴 state 放入新 `host` package 可避免把通用機制放進 `apps/buckrogers/` 或
DOS subsystem。

typed state、transition、失敗模式、測試與停止線已完整，故本規格升為 **READY（純核心）**，只授權
本節 API 和測試；不授權任何 backend、input forwarding、持久化或 player-visible switch。

## CONFORMED 純核心收據

2026-09-21，新增零依賴 `host` package 的 `ScaleController` 與單元測試：

- 2×／3× initial state 都將 active 與 selected 設為相同值；Select 只變 selected，Apply 才原子
  複製 selected 至 active。回選既有值與重複 Apply 都回 `changed=false`，沒有其他 state。
- zero、one、four、255、nil controller 都失敗即關閉，且失敗 Select 後的 snapshot 維持原狀；
  returned `ScaleState` 是 value copy，外部修改不能污染 controller。
- Docker 中 `go test ./host`、`go vet ./host`、`go test -race ./host` 都通過；`go list` 的直接 import
  僅為標準庫 `fmt`，沒有 DOS、machine、oracle、`apps` 或 `cmd` 依賴。

因此本規格升為 **CONFORMED（純核心）**。這不會繪製或重繪 RGBA、不會開啟 host window，也不能
證明玩家可切換倍率；backend／hit event 隔離、Apply 後面板狀態、持久化與同狀態玩家收據仍必須
另建 DRAFT。
