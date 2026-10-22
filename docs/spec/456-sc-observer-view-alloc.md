# 456：陸空戰將 SessionObserver 每步不配置 StepView

狀態：**READY**（第 2 版，依審查 A、B 修訂），已實作（§7）。事後規格：實作 commit `f97abb2` 早於本規格。
日期：2026-10-22
前置：[`238-session-step-observer-draft`](238-session-step-observer-draft.md)（`StepView` 只在回呼內有效、誤用偵測）、
[`006-layering`](006-layering.md)（只動 apps 層）

推論等級：已證實、強推論、假說、未知。

## 1. 範圍與決定

範圍：`apps/strikecommander/receipt.go` 的 `SessionObserver.BeforeStep`。不動 session、machine。

證據（已證實）：飛行案例 Q 截到第 8100000000 步、run on23 的 CPU profile（`strike_commander/workplace/perf/prof/fl-on1.pprof`，
二進位 `bin/sc-opt2.test`，`9848a5c` 加基準測試）：`runtime.convT` 累計 31.6%、`mallocgc` 27%。來源是 `BeforeStep(v session.StepView)`
把 `v`（16 bytes 的值型別，非指標形）分別轉成 `View`、`FlightView`、`SubtitleView` 介面傳給三個觀察器，每次轉換在堆上配置一份複本，每步三次。

決定：`SessionObserver` 加欄位 `cur session.StepView`；`BeforeStep` 先 `s.cur = v`，再把 `&s.cur` 傳給三個 `Observe`。
`*session.StepView` 的方法集含值接收者方法，滿足三個介面；指標形介面轉換不配置。

不做：只取一次 IP 傳給三個觀察器（約 5%，要改三個 `Observe` 的簽章；主代理裁決不做）。

## 2. 規則

- `BeforeStep` 每次呼叫不配置（`TestSessionObserverBeforeStepAllocs` 以 `testing.AllocsPerRun` 斷言 0）。
- 每個方法呼叫讀到的是 `s.cur`，在 `BeforeStep` 內就是 `v`，讀值與以值傳遞相同（`StepView` 的方法只讀 owner 狀態）。

## 3. 語意變化

- **動態型別改變**：三個觀察器收到的介面值，動態型別由 `session.StepView` 變成 `*session.StepView`。對介面做
  `v.(session.StepView)` 型別斷言的下游不再成立（會失敗）；`receipt.go` 開頭的 `var _ View = session.StepView{}` 等編譯期斷言仍成立，
  因為它只檢查方法集。本套件沒有這種型別斷言（已證實，容器內 grep `StepView`）。
- **誤用偵測縮小**：違約保留 view 時，原本任何之後的使用都因 token 不符被判為誤用。改後：
  - 回呼外使用仍判誤用（`s.cur` 保留上一步的 token，`viewOpen` 為假），由 session 的 `TestReusedViewFieldOutsideCallbackIsDetected` 釘住；
  - 在下一次 `BeforeStep` 內使用保留的指標會讀到新 view，不再判誤用（強推論，未加測試：這是本規格接受的縮小）。
  本套件三個觀察器沒有把 view 存進欄位或閉包（已證實：無 `View` 型別欄位、無 `= v` 指派、無 goroutine）。
- 不存檔、不影響 Digest。

## 4. 負對照

`strike_commander/workplace/perf/mutants456.sh`（容器內複製工作樹套用突變；結果 `log/mutants456.log`）：

| 編號 | 突變 | 結果 |
|---|---|---|
| N1 | `BeforeStep` 改回以值傳 `v` | 被抓到：`TestSessionObserverBeforeStepAllocs`，「每次配置 3.0 次」 |
| N2 | session 不設 `viewMisuse`（回呼外使用不偵測） | 被抓到：`TestReusedViewFieldOutsideCallbackIsDetected`，回合跑完 20 步而非第 5 步後 ObserverFault |

## 5. 未知

- 實際遊玩畫格率的變化：未量。
- 整機倍數不寫：§7 的牆鐘範圍只用來說明配置機制的影響方向。

## 6. 驗收

- 截短 Q（on23）新舊二進位：停止 Digest、每張 shot 的 idx 與 2、3 倍合成圖 SHA、`flight-q-on23/` 211 檔逐位元組相同。
- 長測試收據與 `s011impl/lt/g-formal` 逐位元組相同。

## 7. 實作收據

- commit：`f97abb2`；第 2 版的測試（`apps/strikecommander/receipt_alloc_test.go`、`session/observer_reuse_test.go`）由主代理提交。
- 截短 Q on23 牆鐘（`strike_commander/workplace/perf/fbench.sh`，同設定交錯）：`9848a5c` 17.1、15.4、15.0 s；`f97abb2` 10.4、9.8 s；
  主代理另以 `git archive` 各建一版複驗 17.6 s 對 11.1 s。兩組範圍不重疊（強推論：配置與 GC 的節省，幅度隨主機負載變動）。
- 收據比對：三組交叉比對 HASH／SHOT 12 行相同、`flight-q-on23/` 211 檔相同；長測試 Q、C11、存讀檔 F–K 對 g-formal 相同。
- 第 2 版合併前長測試全套（D、E、H、MenuCase、PointerMove、LabelOverlay、Flight Q/R、Subtitle、SaveLoad 與 session 三支觸發案例）見
  [`455-fetch-page-cache`](455-fetch-page-cache.md) §7，同一棵工作樹、同一輪。
- 負對照：§4。`TestSessionObserverBeforeStepAllocs` 在第 2 版工作樹 0 次配置。
