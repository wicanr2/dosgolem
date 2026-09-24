# 233 — Ebitengine BIOS 鍵盤 transport 建構前檢

狀態：**限縮 CONFORMED；僅限 BIOS transport 建構／回合／交付前檢。**
日期：2026-09-24

## 玩家阻塞與證據

本規格只修一種已實測的前端同批部分副作用：`Game.New` 目前接受由
`presentation.NewKeyboardBridge` 建立、沒有 BIOS transport 的合法
`KeyboardBridge`，但 `Game.Update` 對已映射鍵一律呼叫 `DeliverBIOSKey`。
在同一批 canvas Down＋Enter，`MouseBridge` 先將左鍵送進 DOS，
隨後鍵盤橋因無 BIOS transport 回錯；錯誤雖被鎖存，左鍵仍留在 DOS。
Buck Rogers 主專案 `docs/re/issue-18-session-turn-ready-candidate-review.md`
及本 fork 的前修正反證 `frontend/ebiten/route_plan_pre_fix.go.txt` 中
`TestDraftUpdateLateKeyboardTransportErrorLeavesDOSMouseDown` 保留可重生反證。

輸入為 fork `cc0b17ac92b1e3aea8ff676936684ddcf8064594`、Go 1.26.7、
`eob-remake-go:1.26.7-ebiten2.9.9`。本規格無原版 EXE、遊戲狀態、
字型或位址；位址空間不適用，僅涉及 Go 物件與 DOS mouse／BIOS queue
副作用順序。`KeyboardBridge` 的 `bios` 與 `machine` 欄位為私有，
正常外部呼叫者不能在建構後改動；
`NewKeyboardBridgeWithBIOS` 已驗 `bios.M == m`；但
`internal/dos.DOS.M` 是公開可寫欄位，外部可在建構後改成另一台
machine。前修正反證 `presentation/keyboard_machine_alias_pre_fix.go.txt`
（SHA-256 `a92d3e28bcfeb2cb00be348ef0205441fecd91f639effdd36697e693639e80fa`）
證實現行 `DeliverBIOSKey` 不重驗時，鍵會無錯送進第二台 machine
的 BIOS queue。這些都是現行程式事實，不是原版遊戲語意推論。

## 限縮候選契約

正式 Ebitengine `Game` 是 BIOS 鍵盤 transport 的消費端。因此在
`Game.New` 的任何 `refreshLayout`、視窗變更或 DOS 輸入副作用**之前**，
必須用 `presentation` 套件提供的唯讀能力檢查確認所給橋接器可執行
`DeliverBIOSKey`：橋接器、其 panel、machine、BIOS DOS 均非 nil，
橋接器 panel 必須與 `Config.Panel` 為**同一指標**，且 DOS 屬同一
machine。若 panel 不同，`Game.Update` 的面板暫停判斷與
`DeliverBIOSKey` 的鍵盤路由可能看見不同狀態，不能視為有效組態。
任何一項失效，`Game.New` 回明確錯誤，
不得建立可執行的 `Game`；不以反射、虛構 BIOS 或偷偷改成 IRQ1
傳輸補洞。有效橋接器與其既有鍵位、同批鍵盤隔離、面板暫停、
滑鼠與倍率路由保持不變。

建議的最小 API 為
`KeyboardBridge.ValidateBIOSForPanel(panel *host.PanelController) error`；若正式實作
採等價名稱，須維持唯讀、無副作用，且不暴露 DOS 或 machine 指標。
`Game.New` 必須做此預檢，不能將第一次檢查延至已映射鍵。
由於 `DOS.M` 可變，每個 `Game.Update` 也須在路由**任何** pointer
或鍵盤前重做相同預檢；`KeyboardBridge.DeliverBIOSKey` 在呼叫
`PanelController.Route` 或 `PushKey` 前再驗 `bios.M == machine`，
防止已被改指向別台 machine 的 BIOS 接收鍵。這三處驗證不得
改動有效輸入的既有順序。若正式 caller
將來要支援非 BIOS 階段，須有獨立證據與 transport 契約；本規格不
替該階段作選擇。

## 驗收與停止線

1. 無 BIOS 的現有合法 `NewKeyboardBridge` 配置，在 `Game.New`
   即拒絕；DOS button、座標、callback queue、BIOS pending 與
   machine steps 無新增。先前同批 Down＋Enter 負例不再可建構。
2. `NewKeyboardBridgeWithBIOS` 的相同 machine 配置可建構，
   closed canvas Down＋Enter 保持 Move→Press 與 BIOS key 原有順序；
   Open／Apply／Cancel＋Enter 仍按規格 232 隔離。
3. nil bridge、不同 panel、錯 machine BIOS 與任何可從
   `presentation` 同包測試注入的無效私有欄位，都在視窗／DOS
   副作用前失敗。公開的錯 machine BIOS 建構已由
   `NewKeyboardBridgeWithBIOS` 拒絕；不能把它寫成原先由
   `Game.New` 發現的錯誤。正式測試包含正例、負例、
   `go test ./frontend/ebiten ./presentation`、`go vet` 與競態檢查。
4. 有效建構後、下一 `Update` 前將公開的 `bios.M` 改指第二台
   machine：`Update` 在 pointer 路由前回錯，兩台 BIOS queue、
   DOS mouse、machine steps 均無**新增**副作用；單獨呼叫
   `DeliverBIOSKey` 亦在 panel route 與 PushKey 前拒絕。
   驗收還須保留舊版反證，證明若沒有這兩個回合／交付前重驗，
   建構時預檢並不足夠。

`MouseBridge` 目前只持有不透明的 `MouseOutput`；本規格不保證它
與鍵盤 BIOS DOS 屬同一 machine，也不把正例固定的同一 `*dos.DOS`
外推到任何任意 caller 配置。要保證跨設備同機須另訂型別契約。

這只能移除「缺 BIOS transport」、panel 不一致及**回合開始前**
已被改指向他台 machine 的 BIOS 這些已證實配置錯誤；
若同一回合的 mouse 輸出、其他回呼或外部 goroutine 在前檢後
再變更 `DOS.M`，交付前重驗會拒絕錯送鍵，卻不能回滾較早的 DOS
mouse 動作。此處不宣稱完整同批原子性或免於資料競態；
完整來源獨占與提交邊界仍屬規格 019。也不保證修好其他後段失敗，
**不**證明 Issue #18 的整批路由原子性：panel/layout/滑鼠、過期
source generation、其他可失敗提交及唯一 session owner 仍須由專案
規格 019 審查。未完成該契約及原版同狀態／正常玩家路徑前，
不得稱 Linux 可玩版符合。本規格不攜帶、修改或散布原版素材。

## 2026-09-24 獨立 READY 審查

獨立唯讀審查依序發現並促成兩項訂正：原先無參數的
`ValidateBIOS` 漏掉 `Game.Panel` 與橋接器 panel 同一性；
其後又發現公開可寫 `DOS.M` 能在有效建構後把 BIOS key 無錯送至
別台 machine。上述現行程式與 ignored 反證重跑後，審查核准
**本節限縮 READY**：正式實作須逐項完成建構前、每個 Update
路由前、每次 BIOS 交付前三個檢查，並保留相應正反例。

審查不核准完整 session 或輸入批次原子性；若回合中有重入改變
`DOS.M`，交付前檢只能拒絕錯鍵，先前滑鼠副作用仍須由規格 019
處理。READY 是接線許可，不是已通過正式測試或原版同狀態的
CONFORMED 聲明。

## 2026-09-24 限縮實作與驗收收據

正式 `presentation.KeyboardBridge.ValidateBIOSForPanel` 唯讀檢查同一
panel、machine 與 `DOS.M`；`Game.New` 在字型、layout、視窗前檢，
`Game.Update` 在每回合 layout／輸入路由前檢，`DeliverBIOSKey`
在 panel route／BIOS enqueue 前再檢。既有有效路由順序未修改。

獨立唯讀審查核對三處前檢位置、`DOS.PushKey` 對當下 `DOS.M` 的
寫入與有效路由順序，未發現漏檢；審查要求補上 `Game.New` 前
改指 `DOS.M` 的零副作用負例，已補齊。無 BIOS、不同外部 panel、
建構前或回合前換機、同包無效私有欄位均有拒絕測試；有效同機
Down＋Enter 與面板同批隔離仍由既有正式測試覆蓋。修正前反證
保存在 `route_plan_pre_fix.go.txt` 與
`keyboard_machine_alias_pre_fix.go.txt`，不再編入現行測試。

驗證環境為既有 `eob-remake-go:1.26.7-ebiten2.9.9`、Go 1.26.7，
離線唯讀來源 Docker／Xvfb；`go test -count=1 ./presentation ./frontend/ebiten`、
`go vet ./presentation ./frontend/ebiten` 與
`go test -race -count=1 ./presentation ./frontend/ebiten` 全部通過。
這是 Go 前端配置與失敗副作用的合成驗收，無原版 EXE 或
正常玩家路徑；不證明同批原子性、並發改指的回滾、滑鼠橋同機、
規格 019 的 session 契約或 Linux 可玩版完成。
