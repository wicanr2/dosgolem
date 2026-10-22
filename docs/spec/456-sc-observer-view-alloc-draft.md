# DRAFT 2a：SessionObserver 每步三次 StepView 介面轉換配置

狀態：DRAFT（實作前筆記）。範圍：`apps/strikecommander/receipt.go` 的 `SessionObserver.BeforeStep`。不動 session、machine。

## 證據

`perf/prof/fl-on1.pprof`（sc-opt2，飛行 Q 截到 8100000000，run on23，牆鐘 17.1 s；同段 run off 7.4 s）：
`runtime.convT` 累計 31.6%、`mallocgc` 27%。來源是 `BeforeStep(v session.StepView)` 把 `v`（16 bytes 的值型別，
非指標形）分別轉成 `View`、`FlightView`、`SubtitleView` 介面傳給三個觀察器，每次轉換都在堆上配置一份複本，每步三次。

## 做法

`SessionObserver` 加一個欄位 `cur session.StepView`。`BeforeStep` 先 `s.cur = v`，再把 `&s.cur` 傳給三個 `Observe`。
`*session.StepView` 的方法集含值接收者方法，所以滿足三個介面；指標形介面轉換不配置。
每個方法呼叫讀到的是 `s.cur`，在 BeforeStep 內就是 `v`，讀值與原本逐位元組相同（StepView 的方法都只讀 owner 狀態）。

## 語意差異與條件

- 正常使用（觀察器只在 Observe 呼叫內使用 view）：完全相同。
- 違約保留 view（session 238 契約禁止）：原本任何之後的使用都因 token 不符被判為誤用；改後在回呼外使用仍判為誤用
  （`s.cur` 保留舊 token，`viewOpen` 為假），但在「下一次 BeforeStep 內」使用會讀到新 view 而不判誤用。
  本套件三個觀察器沒有把 view 存進欄位或閉包（容器內 grep：無 `View` 型別欄位、無 `= v` 指派、無 goroutine）。
- 不存檔、不影響 Digest。

## 驗收

- `perf/fbench.sh` 同一截短 Q（on23）新舊二進位：停止 Digest、每張 shot 的 idx 與 2、3 倍合成圖 SHA、`flight-q-on23/` 收據目錄逐位元組相同。
- 長測試 Q、C11、存讀檔 F–K 對 `s011impl/lt/g-formal` 逐位元組相同。
- `go vet`、`go test` 同基準。
