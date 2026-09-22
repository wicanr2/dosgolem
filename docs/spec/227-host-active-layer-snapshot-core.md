# 227 — Host active xlate layer 不可變快照純核心

狀態：**CONFORMED（純核心；非 host frontend）**
日期：2026-09-22
前置：[`226-host-machine-presentation-bridge.md`](226-host-machine-presentation-bridge.md)、
[`202-translation-overlay.md`](202-translation-overlay.md)。

## 目的與邊界

本規格將同一個 dosgolem step 執行緒內的 machine indexed／palette 與**目前已 active 的**
`xlate.Layer` 疊字，投影成 host 持有的 RGBA 結果。輸出不暴露 `Machine`、`xlate.Layer`、
DOS input、mouse、BIOS queue 或 IRQ。

它不是 layer lifecycle runner：不呼叫原 layer 的 `Frame`、不觸發 watcher、`Frozen` 或 `OnDrop`，
不寫 VRAM，亦不決定 Ebitengine 視窗、host chrome、倍率面板、鍵盤映射或遊戲 adapter。
正常玩家路徑與任何 Buck Rogers overlay 整合仍由 spec 004 的 DRAFT gate 管理。

## READY 契約與證據

`xlate.Layer.Snapshot`／`Restore` 已保存 active stamps、字型名稱、定色、透明格與指紋狀態，
可在指定字型 map 中還原至新的 layer。故 presenter 可先取得 `host.PresentationSnapshot`，再把
**當前** layer 序列化、還原為私有 clone，將 clone `Draw` 到新配置的 RGBA。原 layer 的 state 不會
因 presenter 而變動；caller 修改回傳 RGBA／indexed 也不能影響 machine 或下一張 snapshot。

使用條件是 layer owner 必須在相同 goroutine、相同 logical input 上先完成
`Layer.Frame(indexed, rgb)`；這一步可能更新 stamp 失效與 watcher，不能由唯讀 presenter 代做。
machine 與 layer 在 Snapshot 呼叫期間不得由其他 goroutine 改動。這是同執行緒序列點契約，不是
並行鎖或跨幀原子化保證。

`LayerSnapshotProvider` fail-closed 拒絕：nil source／layer、非正倍率、layer 與畫布尺寸不符、
未命名 active font，或 font map 未登錄同一個 font 指標。Snapshot／Restore 不包含 watchers、
`Frozen`、`OnDrop` 是刻意限制；它們是未來 lifecycle 擁有者的責任，不是 active stamp 的可繪製
狀態。

```go
func NewLayerSnapshotProvider(
    source host.FrameSource, layer *xlate.Layer, fonts map[string]*xlate.Font,
) (*LayerSnapshotProvider, error)

func (p *LayerSnapshotProvider) Snapshot(scale int) (LayerPresentationSnapshot, error)
```

## CONFORMED 純核心收據

`presentation/layer_snapshot_test.go` 以實際 `machine.New()`、VRAM 與 DAC、已由
`Layer.Frame` 定色的 active stamp 驗證：2× RGBA 有繪製結果；呼叫前後原 layer 的 JSON snapshot
逐 byte 相同；改動回傳 indexed／RGBA 後 machine framebuffer 與下一張 snapshot 不變。另驗證未命名
font、尺寸不符、零倍率與 nil layer 皆 fail-closed。

2026-09-22 Docker 驗證：`go test -race ./host ./presentation`、`go vet ./host ./presentation`。
本規格僅 CONFORMED 純 projection core；它不證明 active overlay 已由任何遊戲 adapter 在正常玩家
路徑上更新，也不代表 Linux frontend 可玩。
