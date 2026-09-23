# 232 — Ebitengine 面板回合鍵盤隔離

狀態：**CONFORMED；僅限 `frontend/ebiten.Game.Update` 的同批鍵盤閘門。**
日期：2026-09-24

## 依據與範圍

使用者已選定「設定面板開啟時，鍵盤只給 host，不送 DOS」及「Apply／Cancel
收合當回合仍暫停 DOS，下一關閉回合才恢復」。專案規格 004 與
`docs/re/phase-189-ebiten-panel-pause-resume-draft.md` 保存決策及 X11 原型；
`docs/re/phase-193-frontend-session-fake-draft.md` 保存回合 fake；
`docs/re/phase-194-formal-ebiten-panel-pause-conformance.md` 與規格 231
已限縮驗收正式 `Game.Update` 的 `Advance` 回合閘門。

已證實的正式缺口：本機 fork `540a522` 的 `Game.Update` 在同一輸入批次
先處理 pointer、後處理 keyboard。面板起點已開啟時，若 pointer Apply
先收合，後續 Enter 經 `KeyboardBridge` 觀察到 closed panel 而進入
BIOS queue；`TestUpdateMixedHostAndDOSInputKeepsPauseGate` 明寫該
`KeysPending=1`。Cancel 同型。這違背已確認的面板鍵盤隔離，但不表示
DOS CPU 當回合已推進；規格 231 的暫停閘門仍正確。

本規格只修同一個 `Update` 的已映射鍵盤事件；不改鍵位 mapping、滑鼠、
倍率提交、畫布幾何、`Advance` 呼叫排程、原版程式、DOS 規則、存檔或
譯文。原版 EXE／手冊／字型不參與，原始位址空間不適用；此處證據是
host 決策、正式程式與可重生單回合輸入測試。完整 typed session、
cold boot、手冊與多作用層仍由專案規格 004／019 管理，不因本規格升級。

## READY 行為契約

1. `Game.Update` 在處理任何 pointer 或 keyboard 前讀取面板起點狀態。
   起點 `Open=true` 時，該批所有鍵盤事件不得呼叫
   `KeyboardBridge.DeliverBIOSKey`，即使同批 Apply／Cancel 收合面板。
2. 起點關閉但同批 host pointer Open 後，後續鍵盤仍不得進 DOS；
   現有 `PanelController` 的 open-panel route 保持有效。
3. 起點關閉、同批沒有 host 開啟面板時，已映射的正常 DOS 鍵維持
   既有轉送；未知鍵仍不製造 DOS 事件。
4. host pointer 仍先於 keyboard 路由；倍率 Apply／Cancel、MouseBridge
   清理、錯誤失敗即關閉及規格 231 的零 `Advance` 排程均不得退化。
   本規格不引入未定的完整 `InputBatch`／`TickReceipt` API。

## 實作後驗收與權利邊界

正式 `frontend/ebiten` 套件的可注入 `frameInput` 測試至少覆蓋：
Open＋Enter、Select＋Enter、Apply＋Enter、Cancel＋Enter、面板起點
開啟時多個 mapped keys、closed canvas＋Enter；檢查 BIOS queue、
host 面板／倍率狀態、DOS mouse 路由和 `Advance` 次數。保留 panel／
route／layout／Advance 故障後不重試的既有測試。Docker／Xvfb 中跑
正式套件測試、靜態檢查與競態檢查；可選真實 X11 同批事件測試只能
補強，不能以假造事件當成原版同狀態收據。

通過後僅可標此鍵盤隔離子契約 CONFORMED；完整 session 與可玩前端
仍未完成。不得將原版資料、掃描手冊、私有 state、倚天字型或其可還原
衍生物加入 Git 或公開包。

## 2026-09-24 限縮符合性

正式 `Game.Update` 現在於路由前讀取 `before.Open`，在
`before.Open || panelEventThisUpdate` 時跳過整批鍵盤轉送。Pointer 先於
keyboard 的既有順序不變；Apply／Cancel 仍完成倍率與收合，但同批
Enter／A／Left 均不進 BIOS。關閉面板後下一回合的正常畫布＋Enter
仍會轉送。既有面板暫停 `Advance` 閘門與失敗即關閉測試保持通過。

正式套件測試已檢查 Apply／Cancel＋多鍵、closed→Open＋Enter、
closed canvas＋Enter、host mouse 零轉送、BIOS queue、面板／倍率終態
及下一回合恢復。獨立程式審查確認未見此範圍內漏送、誤送或錯誤路徑
退化。`eob-remake-go:1.26.7-ebiten2.9.9`、Go 1.26.7、無網路
Docker／Xvfb 中的 `go test ./frontend/ebiten ./host ./presentation`、
相同套件的 `go vet` 及 `go test -race ./frontend/ebiten` 均通過。

此符合性不聲稱實體 X11 同批按鍵時序、原版 DOS 同狀態、完整 typed
session、冷開機或玩家前端可玩。`New` 已拒絕 nil Keyboard；測試私下
破壞 `Game.keys` 的非正式情境不在此鍵盤批次契約內。
