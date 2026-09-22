# 226 — Host machine 畫面與鍵盤橋接純核心

狀態：**CONFORMED（純核心；非 host frontend）**
日期：2026-09-22
前置：[`004-dosgolem-host-frontend-draft.md`](004-dosgolem-host-frontend-draft.md)、
[`225-host-panel-event-routing-core.md`](225-host-panel-event-routing-core.md)。

## 目的與邊界

本規格只接通兩條已可由 dosgolem 通用 API 證實的邊界：

1. 同一個執行緒從 `Machine` 讀出當前 indexed framebuffer、palette 與畫布尺寸，交給
   `host.PresentationSnapshotProvider`；
2. 面板已關閉且 `PanelController` 明示許可時，才將一個**已完成 backend 映射**的 DOS scan code
   排入硬體鍵盤佇列。

它不建立 Ebitengine 視窗，不做 Ebitengine key 到 DOS scan code 的映射，不做 host chrome
或 pointer hit test，不轉送 DOS mouse，不持久化設定，也不引用 `apps/*`。它不處理
`xlate.Layer` 的 active stamp snapshot；該 layer 的 `Frame`／`Draw` 會維護生命週期，尚沒有
可獨立複製的通用契約，因此仍是 spec 004 的 DRAFT 缺口。

此 bridge 不能當作玩家路徑收據，尤其不證明任何遊戲、繁中覆繪、手冊 presenter 或 Ebitengine
事件迴圈已接通。

## READY 證據審查

`Machine.VideoSize()` 回目前畫面尺寸，`Machine.Indexed()` 文件明定回傳複本，`Machine.Palette()`
回傳值型別；因此單一 goroutine 在兩次 `Step` 間依序讀取三者時，host 沒有寫入 VRAM、DAC、
DOS memory、BIOS queue、IRQ 或存檔的路徑。

這不是並行安全承諾：若另一 goroutine 同時 `Step`，indexed 與 palette 可能屬於不同的
presentation instant。故 `MachineFrameSource.ReadPresentationFrame` 的 caller contract 是必須與
`Machine.Step` 位於同一 goroutine；bridge 本身不得 Step machine。

`Machine.QueueKey(scan)` 已是通用硬體鍵盤 API，明定排入按下與放開的一對 scan code。spec 225
已完整決定鍵盤焦點：面板開啟必須回 `ForwardToDOS=false`，關閉才回 true。故 scan code 已由
未來 backend 明確映射後，以下純核心可達 READY；它不猜測快捷鍵或把未命中 pointer 送進 DOS。

## 契約

```go
func NewMachineFrameSource(*machine.Machine) (*MachineFrameSource, error)
func (s *MachineFrameSource) ReadPresentationFrame() (host.IndexedFrame, error)

func NewKeyboardBridge(*host.PanelController, *machine.Machine) (*KeyboardBridge, error)
func (b *KeyboardBridge) DeliverDOSScan(scan uint8) (host.PanelState, host.InputRoute, error)

func NewKeyboardBridgeWithBIOS(*host.PanelController, *machine.Machine, *dos.DOS) (*KeyboardBridge, error)
func (b *KeyboardBridge) DeliverBIOSKey(key dos.Key) (host.PanelState, host.InputRoute, error)
```

`MachineFrameSource` 必須拒絕 nil machine，並且 fail-closed 拒絕無效尺寸或 indexed 長度與
`width * height` 不相等的 frame。它只回傳 `host.IndexedFrame`；後續
`PresentationSnapshotProvider` 再驗證、複製 indexed bytes，讓 presenter 修改 snapshot 不會影響
machine。

`KeyboardBridge.DeliverDOSScan` 必須先呼叫
`PanelController.Route(PanelEventKeyboard)`，只有回傳 `ForwardToDOS=true` 才可呼叫 `QueueKey`。
面板開啟時不得改鍵盤佇列；關閉時寫入的一組 make／break code 是明確、刻意的原版輸入，不是
「零副作用」。

不同原版畫面可能讀 BIOS `int 16h` 而不是 IRQ1。`DeliverBIOSKey` 因此只接受呼叫端明示的
`dos.Key{Scan, ASCII}`，同樣先走 `PanelController`，關閉面板才 `DOS.PushKey`。它不從 scan
猜 ASCII、不混送 IRQ1，且 constructor 必須拒絕不屬於同一 machine 的 DOS service。

## CONFORMED 純核心收據

實作位於 `presentation/machine.go` 與 `presentation/keyboard.go`，測試位於同 package：

- machine source 回報 320×200、正確 indexed byte 與 DAC 6→8 位 palette；修改 host snapshot 後
  machine VRAM 與下一張 snapshot 都不變；nil source fail-closed；
- 關閉面板時 bridge 將 scan `0x1e` 排成 `0x1e,0x9e`；開啟面板後送 `0x30`，佇列保持不變；
  nil dependencies fail-closed。
- 明示 BIOS `{Scan:0x1c, ASCII:0x0d}` 在關閉面板時才使 `KeysPending` 增加；面板開啟時
  保持不變，nil／不同 machine DOS 均 fail-closed。

2026-09-22 已在 Docker 以 `go test -race ./host ./presentation` 與
`go vet ./host ./presentation` 通過。這只 CONFORM 上述純資料／鍵盤轉送核心；spec 004 仍為
DRAFT，直到 active xlate layer 的同執行緒整體 presentation snapshot、正式 Ebitengine hit test／
key mapping，以及正常玩家路徑同狀態收據都完成。
