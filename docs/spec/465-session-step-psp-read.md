# 465：StepView 的載入計數、目前 PSP 與有界實體記憶體讀取

狀態：**CONFORMED**（第1版通用唯讀契約，2026-10-26）。只增加觀察回呼內的唯讀入口，不改執行、啟動、輸入或狀態檔。這不包含遊戲R8或正常玩家驗收。

前置：[238 觀察器](238-session-step-observer-draft.md)、[464 啟動前 PSP 讀取](464-session-boot-psp-read.md)。需求來自陸空戰將[016 R8](../../../strike_commander/docs/spec/016-player-frontend.md)的 MENU 結束判定。本文只定義通用讀取契約，不判定遊戲程式身分。

## 證據與缺口

- 已證實：`session/observer.go` 的 `StepView` 只在同一次 `BeforeStep` 內有效，既有 `live()` 負責過期視圖的誤用記錄。既有 `ReadBlock` 走 `machine.PeekBlock`，不能當成不經頁表的實體 RAM 讀取。
- 已證實：`internal/dos/current_psp.go` 已提供 `DOS.CurrentPSP()`，只回傳 DOS 的實際 `curPSP`。464 的 `ReadBootPSP` 以此值乘16，從 `machine.Mem` 複製實體位元組。
- 已證實：`internal/dos/exec.go:298,426,710` 在成功 spawn、成功 AL=03h overlay、成功監督載入後追加現有 `ExecLog`；失敗在追加之前返回。單純結束只改舊記錄的 Ended 等欄位；疊底 `terminate` 若有監督佇列，會呼叫 `spawnQueued`，成功時依該次載入追加。記錄本來就存在，這次只讀其長度，不增加計數器或事件。
- 已證實：464 所連六案的 PSP 相同，程式載入會重用同一位址。因此只以 PSP 變動判斷是否重查程式不足。新增載入計數供失效判斷，包含 overlay，不能稱為專屬 EXEC 或程式切換通知。
- 已證實的基準：上述程式在 fork `e2b521e827081d852332f76f1ead463b59d36f93`。464 的六案原始雜湊與零指令讀取證據由其「實作與驗收範圍」進入。這不證明本次新 API 或完整玩家流程已通過。
- 工程缺口：464 的 Owner 入口限首次輸入前一次使用，無法在遊戲執行中確認目前 PSP。`ProgramPath` 不隨子程式改變；不能據此猜測成功 EXEC。保留 464 的原窗口限制，另增加回呼內讀取。

## 介面與唯讀契約

```go
func (v StepView) CurrentPSP() uint16
func (v StepView) LoadCount() uint64
func (v StepView) ReadPSP(ranges []BootPSPRange) (BootPSPReceipt, error)
```

沿用 464 的值型別，`Offset` 從目前 PSP 的實體基址起算，`Dst` 為 caller 所有，不回傳內部 slice。

1. 每次方法先呼叫既有 `live()`。無效視圖的兩個 scalar 方法回0；`ReadPSP` 回零 receipt 及錯誤、不改任何 Dst。誤用的 latch 與下一次 observer fault 完全沿238／454，不新增獨立 fault 或繞過 `live()`。
2. 有效視圖的 `CurrentPSP` 只回 `o.dos.CurrentPSP()`，包含原值0；不替換成固定 `PSPSeg`。`LoadCount` 只回 `uint64(len(o.dos.ExecLog))`，不遍歷記錄、不讀檔名或補歷史，不把它變成新事件。兩者不讀客體記憶體、不新增配置。該計數只供同一Owner內差分；ExecLog不進snapshot，跨Owner／重啟／狀態讀回不得沿用舊計數當全域世代。
3. 有效視圖的 `ReadPSP` 先取得同一 DOS scalar，0拒絕。批次1至32個範圍，每個1至4096 bytes，總數不超16KiB。以 `uint64(PSP)*16 + uint64(Offset)` 計算，整段必須在 `machine.Mem` 長度內；不能溢位、回繞或套A20／CR3轉換。只有原始實體 RAM 複製，不呼叫 Peek、Read、匯流排、頁表或裝置方法。
4. 全部範圍通過後才依索引順序複製。重疊 Dst 由後項覆寫。任一範圍錯誤回零 receipt、所有 Dst 保持原bytes；普通參數錯誤不設 observer fault、不改 Owner phase。成功回傳實際 PSP。
5. 不保留 ranges、Dst 或 StepView，不推進指令，不補觀察，不送事件，不變更 DOS／CPU／RAM／頁表／音訊／顯示或檔案。既有 Owner 的單執行緒序列化與禁止回呼外保留視圖規則不變。
6. 同一回呼可重複呼叫。不讀寫464的 `bootPSPUsed`／`bootPSPClosed`，不重開 Owner 的一次性窗口。沒有新 snapshot 欄位、摘要欄位或 JSON 格式。
7. 不增加 observer 觸發時機。逐步與稀疏觀察均只在本來就有的回呼提供當時值，不承諾稀疏模式立即通知 PSP 變動。不翻轉 `SparseObserve` 預設。

## 應用層界線

目前 PSP 不是程式名稱，也不是成功 EXEC 事件。應用程式可在讀到 PSP 或載入計數改變時重查已驗證的本機 MZ 識別位元組，不能只憑 PSP 不同、或載入計數增加就宣稱進入特定子程式。計數包含 overlay，重新核對後仍可能是同一程式。識別資料、重定位、唯一匹配與未知情況由遊戲規格及測試處理。

正常玩家前端目前保持逐步觀察。R8 要以本入口替代猜測，需另驗冷開機、MENU 循環與進入下一支程式的連續性。零步 snapshot 及合成 DOS 測試不能取代這條玩家路徑，也不能宣稱全部未知 snapshot 均可識別。

## 最小驗收

- 合成 Owner：有效回呼的 scalar 與實際 DOS PSP 相同；改變合成 DOS 的 PSP 後讀取跟隨實際值。物理位置填不同於線性頁表對映的 bytes，讀取必須等於獨立實體期望。這是測試 fixture，正式實作不得更改 PSP。
- 合成成功／失敗 spawn、overlay 與結束路徑：載入計數等於原本ExecLog長度，成功載入增加，失敗與無後續載入的結束不新增。另驗疊底結束帶監督佇列，成功交接時計數依新載入增加，失敗不增加。至少一例成功重載前後PSP相同而計數增加。新Owner從其實際清單長度開始，不靠snapshot補歷史。
- 成功／參數拒絕前後比較完整 memory、CPU、machine／DOS 摘要及464窗口旗標；無額外 guest 指令、observer 或事件。修改 Dst 不回寫 RAM；重疊 Dst 按索引順序覆寫。
- PSP0、空批次、33範圍、單範圍0／4097、總量超額、Offset尾端越界；先合法範圍後非法範圍的批次不得部分寫入。所有失敗回零 receipt。
- nil Owner 的零視圖回零值及讀取錯誤，沒有 Owner 可設 fault；保留到下一次 BeforeStep 及在 Frame 回呼使用過期視圖驗既有 observer fault 行為。過期讀不改 Dst。
- 真負對照：固定PSP、固定載入計數、錯走線性讀取、漏 `live()`、先複製後驗證，對應測例必失敗。既有464與session回歸、race、vet通過。通用入口與遊戲R8驗收分開記錄。

已經[獨立規格審查甲](../../../strike_commander/workplace/impl016/review465-spec-a-001.md)與[乙](../../../strike_commander/workplace/impl016/review465-spec-b-001.md)，兩份對最後草稿均無阻擋／未解應改。受審DRAFT2 SHA為 `d2f418a5465df903ea490317cad844bb9b9a34cdf3713040a8cd325b272b95e8`。疊底結束帶監督佇列的計數語意已依實際呼叫鏈修正；READY只授權上述通用入口，不代表實作或玩家路徑完成。

## 實作與通用契約驗收

新增`session/step_psp.go`，SHA `b97a33f9b9fefe3e551f2a62bac4ffa4f33e076e25827633ad8cd52d9f01a29d`，既有production檔未因465改動。[另一作者的測試收據](../../../strike_commander/workplace/impl016/step465-test-receipt-001.md)保存7項頂層、27個子案例；測試SHA `cadd88dbe57ce42ed6a9ae14cbc9fd6b477d7a33d1eaf6b6e5639468ce75f277`。測試從真runObserved取得有效視圖；合成DOS載入經已安裝的INT21 handler，不是直接append假紀錄，也不宣稱執行了遊戲的INT opcode。

[獨立production審查甲](../../../strike_commander/workplace/impl016/review465-production-a-001.md)與[乙](../../../strike_commander/workplace/impl016/review465-production-b-001.md)均無阻擋／應改。主代理已讀兩份報告，並親跑[五項負對照及回歸](../../../strike_commander/workplace/impl016/logs/step465-validation-002.json)：固定PSP、固定載入計數、錯用線性讀取、移除live守門、部分複製皆在指定assertion失敗；完整session、465與464的race及vet通過。結果SHA `b38141d20f155b19ca8ca9f821205b578e16b2b104e98b3706da403706ee14c4`，來源前後SHA不變。

首次負對照因Docker暫存掛載不可執行而未跑到測試，原log保留且不計有效負控；指定可執行tmpfs後同image及同測試乾淨重跑。通用契約在上述範圍CONFORMED；同狀態原版接續、MZ唯一分類、MENU生命週期、平台GUI與玩家流程保持各自閘門。
