# 231 — Linux Ebitengine 設定面板的 DOS 推進閘門

狀態：**CONFORMED（僅限 `frontend/ebiten.Game.Update` 的 `Advance` 呼叫排程）**  
日期：2026-09-23  
前置：`219-host-scale-selection-apply-core`、`225-host-panel-controller`、
`230-linux-ebiten-host-event-loop-draft` 的既有事件路由；後者整體仍為 DRAFT。

## 範圍與依據

玩家已選定：設定面板展開時暫停 DOS；Open、Select、Cancel、Apply 所在的更新回合
均不推進，面板收合後的**下一個**關閉回合才恢復。此規格只控制注入
`frontend/ebiten.Config.Advance` 的呼叫次數，不改 DOS 指令、虛擬時間、
鍵盤／滑鼠分流、原版程式、譯文、字型或存檔。它不是完整 session、冷開機
或 Buck Rogers 玩家入口的 READY 聲明。

READY 前證據如下：

- 專案 `docs/re/phase-189-ebiten-panel-pause-resume-draft.md`：真實 Ebitengine/X11、
  ignored caller 的 Cancel／Apply 路徑，開面板 89 回合與兩次收合同回合均零 DOS Step，
  下一關閉回合各前進 16 步；這是原型證據，不冒稱正式前端已驗。
- 專案 `docs/re/phase-193-frontend-session-fake-draft.md`：fake 對 Open／Select／
  Cancel／Apply 與下回合恢復的窮舉收據；故障後拒絕再 Advance。原型來源雜湊
  與 Docker 命令在該文件。
- `frontend/ebiten/game.go` 的既有 `Game.Update` 在事件路由後無條件呼叫
  `g.advance()`，明確構成待修缺口。獨立證據審查認定上述兩份原型足以定義
  此狹窄回合規則；正式程式測試列為實作後驗收，不倒置 READY 閘門。

這是 host 回合排程，不涉及原版地址空間；原版 EXE 雜湊、DOS callsite 與
相同狀態 A/B 不適用於本規格的 READY 判準，也不能因此免除玩家前端整體驗收。

## 輸入、狀態與輸出

輸入是單一 Ebitengine `Update` 的 mouse/key/focus edge、既有
`host.PanelController` 狀態與注入的 `Advance func() error`。狀態只需要
「回合開始時面板是否開啟」、「本回合是否處理 Open／SelectScale／Cancel／Apply」
與「路由後面板是否仍開啟」。不得僅依**回合末** `Open=false` 判斷：Cancel／Apply
已在同回合收合，仍須零呼叫。

1. 先完成既有輸入路由。若回合開始或結束時面板開啟，或本回合任一
   Open／SelectScale／Cancel／Apply 已被 host 消費，`Advance` 呼叫數必須是零。
   即使同一回合先開又關，也不能漏掉這個 transition 旗標。
2. 只有上述三個條件全否、面板關閉且路由成功，才可在該回合呼叫
   `Advance` **至多一次**。Cancel／Apply 後下一個無面板 transition 的
   關閉回合才符合條件。
3. 既有 host 命中事件不能因暫停判斷而轉送 DOS；既有已接受 DOS mouse
   Down 的放開清理仍照 `MouseBridge` 契約執行。倍率套用不得重啟 DOS。
4. 面板狀態讀取、輸入路由或 layout 更新失敗時，本回合不得呼叫
   `Advance`；`Advance` 自身失敗後，後續更新不得再次呼叫它。錯誤須由
   `Game.Update` 回傳，不得吞掉。

`Advance` 目前是無參數 callback，因此本規格只保證**呼叫次數**，不聲稱
每次實際執行多少 DOS instruction。完整 typed budget／step receipt、phase、
observer、active composite、save root 與 cold boot 仍由專案 spec 004／Issue #18
處理，保持 DRAFT。

## 實作後驗收與停止線

- 以可注入的單回合輸入 seam 測試正式 `Game.Update`：Open、Select、Cancel、
  Apply、面板持續開啟皆零 `Advance`；收合後下一關閉回合恰一次。包含
  同回合混合 host／鍵盤／pointer edge，並確認不改既有輸入分流。
- 注入 panel／route／layout／`Advance` 故障，驗證當回合與後續更新不偷跑。
- 在 Docker／有界 Xvfb 以正式 `Game.Update` 跑真實 2×／3× 回調，記錄
  Cancel／Apply 收合當回合與下一關閉回合的 callback 呼叫數；若使用
  原版 state，另記 DOS step 錨點。單元測試不取代此實體收據。
- 任何測試只能把此**局部閘門**升為 CONFORMED；不得宣稱手冊、其他
  繁中作用層、完整 DOS 同狀態、存讀檔或 Linux 玩家前端已完成。

原版遊戲、掃描手冊、已購字型及私有 checkpoint 只留使用者本機 ignored
`workplace/`；本規格不需也不儲存其內容。

## 2026-09-24 限縮符合性

正式 `Game.Update` 已在同一回合記錄面板起始／結束狀態及 host transition，
只在三者都不要求暫停時呼叫注入的 `Advance`。`frontend/ebiten/game_test.go`
以可注入 frame input 驗 Open、Select、Cancel、Apply、持續展開、同回合開又關、
下一關閉回合、混合鍵盤／畫布滑鼠及 panel／route／layout／Advance 故障後
不再推進。Apply 同回合 Enter 仍依既有 pointer-before-key 分流進 BIOS，
但同回合不呼叫 `Advance`；此鍵盤批次政策沒有在本規格改動或宣稱完成。

`frontend/ebiten/pause_physical_test.go` 的 opt-in 真實 X11 測試使用合成畫布，
正式 `Game.Update` 於 2×、3× 各完成 Cancel 與 Apply；四個收合回合為
`88／157／196／264`，第一次恢復呼叫分別在後續回合
`89／158／197／265`，最終倍率回到 2×。測試在
`eob-remake-go:1.26.7-ebiten2.9.9`、Go 1.26.7、無網路且有界 Xvfb 的
Docker 中通過；受影響元件的 `go test`、`go vet` 與前端／收據命令
`go test -race` 亦通過。獨立程式審查已核對混合事件、錯誤邊界與
原型／正式測試的範圍，僅核准此局部 CONFORMED。

測試未載入原版；所以這個狀態**不**證明實際 DOS instruction 數、
原版完整 machine／DOS 同狀態、冷開機、完整 session、手冊、中文作用層
或存讀檔。上述項目仍由專案 spec 004 與 Issue #16／#18 管理。
