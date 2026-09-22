# 225 — Host 面板事件路由純核心

狀態：**CONFORMED（純核心；非 host frontend）**
日期：2026-09-22
前置：[`219-host-scale-selection-apply-core.md`](219-host-scale-selection-apply-core.md)、
專案 `docs/spec/004-dosgolem-host-frontend-draft.md`。

## 目的與邊界

本規格將已由使用者確認的四項 host 操作語意限縮為零依賴純核心：

1. host hit 先被 host 消費，絕不轉送 DOS；
2. 面板開啟時，鍵盤由 host 消費，不轉送 DOS；關閉後鍵盤才有資格轉送；
3. Apply 提交倍率後一律自動收合面板，包含 active 與 selected 本來相同的情況。
4. 未按 Apply 的關閉是 Cancel：selected 重設為當前 active 後才收合；下一次開啟不保留暫選。

它只處理已完成 hit test 的 typed event 與記憶體 state。它不得開視窗、畫 UI、做座標 hit test、
持有 `Machine`、讀寫 VRAM／DAC、送 BIOS queue／IRQ／DOS mouse、持久化檔案、建立 Ebitengine
loop 或接觸任何 game adapter。`ForwardToDOS` 是 backend 可採用的許可訊號，不是本 package
自行送入 DOS 的命令。

本規格不決定未命中 pointer 是否轉送 DOS mouse、host 文案、鍵盤快捷鍵、session／跨重啟持久化或前端 backend。
那些仍在專案 spec 004 的 DRAFT 前沿，不能藉此核心猜補。

## 已證實決定與 typed 契約

使用者已確認：Linux-first、Go／Ebitengine 是日後 frontend 選項；設定面板開啟時鍵盤停止送入
遊戲；Apply 後自動收合。這些是產品決定，不從 DOS 執行檔或 `cmd/probe` 注入能力推論。

```go
type PanelEventKind uint8
const (
    PanelEventOpen PanelEventKind = iota + 1
    PanelEventSelectScale
    PanelEventApply
    PanelEventCancel
    PanelEventKeyboard
    PanelEventPointerHostHit
    PanelEventPointerMiss
)

type PanelEvent struct { Kind PanelEventKind; Scale OutputScale }
type InputRoute struct { ConsumedByHost bool; ForwardToDOS bool }
type PanelState struct { Open bool; Scales ScaleState }

func NewPanelController(initial OutputScale) (*PanelController, error)
func (c *PanelController) Snapshot() (PanelState, error)
func (c *PanelController) Route(event PanelEvent) (PanelState, InputRoute, error)
```

`PanelEventOpen`、`PanelEventSelectScale`、`PanelEventApply` 和 `PanelEventPointerHostHit` 都回
`ConsumedByHost=true, ForwardToDOS=false`。Select 只在面板開啟時呼叫既有
`ScaleController.Select`，因此不改 active scale。Apply 只在面板開啟時呼叫
`ScaleController.Apply`，成功後必須設 `Open=false`，不論 `changed` 是否為 false。

`PanelEventCancel` 只在面板開啟時有效：先讀取 active scale，成功將 selected 重設為該值後才設
`Open=false`。所以 active2／selected3 的 Cancel 終態必為 active2／selected2／closed；重新 Open
仍顯示 2×。Cancel 是 host hit，回 `ConsumedByHost=true, ForwardToDOS=false`。

`PanelEventKeyboard` 在 `Open=true` 時回 host consume／zero forwarding；`Open=false` 時只回
`ForwardToDOS=true`。這不含實際 scan code，故不可能在本層更動 BIOS queue。`PointerMiss` 回兩個
false，保留後端以後的 mouse forwarding 決定。

不合法 scale、未知 event、重複 Open、面板關閉時的 Select／Apply 和 nil receiver 必須回 error，
且不更動 state。這個 fail-closed 規則避免 host frontend 的時序錯誤意外進入 DOS。

## READY 證據審查與停止線

上述四項已確認決定完整指定了本核心處理的 state、輸入、輸出、失敗模式與邊界；session-only
持久化範圍並不需要任何 I/O 欄位，故不會被實作猜補。
`host/panel.go` 直接 import 僅有標準庫 `fmt`，沒有 `internal/machine`、`internal/dos`、`oracle`、
`apps/*`、`cmd/*` 或 Ebitengine，通用層位置符合既有分層。

故此純核心契約曾達 **READY**，只授權 `host.PanelController` 與其單元測試；不授權 playable
frontend、DOS input bridge、設定檔或畫面 presenter。

## CONFORMED 純核心收據

已實作 `host/panel.go` 與 `host/panel_test.go`。測試覆蓋：

- 關閉面板的 keyboard 才回 `ForwardToDOS=true`；開啟後同一類 keyboard 被 host 消費；
- host pointer hit、Open、Select、Apply 的每筆 route 都零 DOS forwarding；
- 2×→3× Select 不改 active，Apply 後 active／selected 同為 3× 且面板收合；同倍率 Apply 也收合；
- active2／selected3 的 Cancel 原子收合為 active2／selected2；再開面板仍是 2×；
- PointerMiss 保持未決；不合法時序、scale 與未知 event 都原子失敗。

2026-09-22 在 Docker 中，`go test -race ./host` 與 `go vet ./host` 通過。本機提交
`ecac7934039ca16e9f5d2951540b6f2cb5f34985` 建立了初版核心；Cancel 實作為
`b09769454d2831a72d302085572678bf28f1c6bb`，均沒有推送遠端。

因此本規格為 **CONFORMED（純核心）**。它不證明 Ebitengine window 可玩、host hit test 正確、
DOS forwarding 實作安全或任何原版玩家路徑；那些要等專案 spec 004 的 DRAFT 前置完整後另行驗收。
