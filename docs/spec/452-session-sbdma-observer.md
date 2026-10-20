# 452：session 轉發 SB16 DMA 觀測事件（`SBDMAObserver`）

狀態：**READY**（第 3 版，依第一、二輪審查修訂，處理見 §6、§7），已實作（§5）
日期：2026-10-20
前置：[`006-layering`](006-layering.md)（通用層判準）、[`238-session-step-observer-draft`](238-session-step-observer-draft.md)（逐步觀察器）、[`240-opl-audio-output-draft`](240-opl-audio-output-draft.md) §3.3（`AudioObserver`）、
[`444-legacy-sb16-dma8-irq7`](444-legacy-sb16-dma8-irq7.md) §2.4、§2.5、§2.8、[`445-legacy-sb16-dma8-pause-resume`](445-legacy-sb16-dma8-pause-resume.md) §2.9、
[`446-legacy-sb16-idle-control-and-busy-cycle`](446-legacy-sb16-idle-control-and-busy-cycle.md)、[`448-session-config-legacy-pm-profile`](448-session-config-legacy-pm-profile.md)、
[`450-session-boot-from-state`](450-session-boot-from-state.md)

編號 452 取 `strike-commander` 分支 `docs/spec/` 目前最大號 451 的下一號（dosgolem-sc main 最大 428、`~/cht/dosgolem` 最大 442，審查 B 核對）。

dosgolem 的程式位置以 `strike-commander` 分支 commit `6e93eed` 為準。推論等級：已證實、強推論、假說、未知。

用語：

| 用語 | 意義 |
|---|---|
| 事件 | `machine.SBDMAEvent`（`internal/machine/sb_dma8.go:843–865`） |
| 回合 | 一次 `Deliver` 加上隨後的一次 `Advance`（449、451） |
| 步驟嘗試 | 一次 `machine.Step()` 呼叫；`m.Steps++` 在其中、執行指令之前（`internal/machine/machine.go:1524`） |

---

## 1. 為什麼要做

`machine` 已有 SB16 DMA 的觀測紀錄：每次 `start`、`complete`、`cancel`、`pause`、`resume`、`irq7` 等都組成一筆事件，`OnSBDMA` 非 nil 時逐筆呼叫，否則追加到上限 4096 筆的 `SBDMALog`（`sb_dma8.go:946–958`；444 §2.8 寫明這是「給 M3 語音字幕用的唯讀紀錄」）。

session 不轉發這些事件：

- `StepObserver` 與選配擴充只有 `BeforeStep`、`VideoWrite`、`Frame`、`AudioObserver{OPLWrite, SpeakerSample, PITChannel2}`、`ModeObserver{VideoModeChange}`（`session/observer.go:14–39`）。
- `session/*.go` 沒有引用 `OnSBDMA` 或 `SBDMALog`；`machine` 是 `Owner` 的私有欄位（已證實：程式碼）。

《Strike Commander》的語音字幕（strike_commander 規格 010）以 DMA `start` 為觸發、以 `Due`、`pause`、`resume`、`cancel` 定起訖（strike_commander RE 022 §3）。沒有這個介面時，app 只能在 `BeforeStep` 辨認 `out 22Ch` 與 DMA 埠寫入，自己重做 444 的 DSP 與 8237 狀態機。這是硬體層的通用行為，換一支 binary 仍成立，依 006 的判準應放通用層，不放 app。

## 2. 規則

### 2.1 介面

```go
// SBDMAObserver 是 StepObserver 的選配擴充（docs/spec/452）。
type SBDMAObserver interface {
	SBDMA(e machine.SBDMAEvent)
}
```

- 放在 `session/observer.go`，與 `AudioObserver`、`ModeObserver` 並列。
- 不另定 session 自己的事件型別：`AudioObserver` 已直接用 `machine.OPLWrite` 等型別（`session/observer.go:26–30`），同一模組的 app 可以引用。

### 2.2 轉發內容

- 全部 Kind 都轉發，不過濾：`start`、`complete`、`cancel`、`pause`、`resume`、`control-idle`、`eoi`、`defaulted`、`irq7`（`sb_dma8.go` 的 `sb8Event` 呼叫點 265、404、442、479、496、515、796、827、836 行）。
- 整個 `SBDMAEvent` 以值傳遞，欄位不挑、不改。字串欄位（`Kind`、`CPUMode`、`Format`、`Clock`、`SHA256`）在 Go 是不可變的，不需複製。
- 偵測用的 4 bytes 傳輸、靜音、ADPCM 都照樣轉發。哪些算語音、哪些要忽略是遊戲知識，由 app 依 `Kind`、`Format`、`Length`、`Samples` 過濾（444 §2.8 末段）。
- 欄位意義照 444 §2.8 與 445 §2.9，本規格不改。

### 2.3 安裝與移除

在 `runObserved`（`session/observer.go:141` 起）比照 `AudioObserver`：

```go
if so, ok := o.observer.(SBDMAObserver); ok {
	o.machine.OnSBDMA = func(e machine.SBDMAEvent) { guard("SBDMA", func() { so.SBDMA(e) }) }
}
```

- `defer` 內設回 `o.machine.OnSBDMA = nil`，與 `ObserveModeChanges(nil)` 同一段。
- 觀察器沒有實作 `SBDMAObserver` 時不碰 `OnSBDMA`，`SBDMALog` 照現況累積（上限 4096，滿了丟最舊的）。
- panic 由既有的 `guard` 鎖存（`session/observer.go:169–179`）：這一步驟嘗試照常完成，下一次嘗試以觀察器錯誤停下，與 `OPLWrite` 相同。
- `Config.SBDMA8` 為假時機器沒有 `sb8`，不會產生事件，回呼永遠不被呼叫。是否要在 `New` 時對「實作了 `SBDMAObserver` 但 `SBDMA8` 為假」回錯誤，見 §4 第 1 項。

### 2.4 時序與唯讀

- 回呼在步驟嘗試之內呼叫，與 `BeforeStep` 同一個 goroutine。
- 回呼內沒有開著的 `StepView`：`BeforeStep` 已返回，`viewOpen` 為假（`session/observer.go:215–220`）。在回呼內使用保留下來的 `StepView` 會設 `viewMisuse`，下一次 `BeforeStep` 前以觀察器錯誤停下（既有行為，不改）。
- 回呼不得改變執行狀態。它只拿到值，拿不到 `Machine`。
- 需要讀記憶體的消費端，在下一次 `BeforeStep` 讀。那時看到的是事件所在步驟嘗試執行完的狀態。事件的 `Step` 與下一次 `BeforeStep` 的 `Steps()` 關係如下（已證實：程式碼）：

| Kind | 產生位置 | `e.Step` | 下一次 `BeforeStep` 的 `Steps()` |
|---|---|---|---|
| `start`、`cancel`、`pause`、`resume`、`control-idle`、`eoi` | 指令執行中（`Out8`），`m.Steps` 已遞增 | 本道指令的序號 N | N |
| `complete` | 步驟嘗試開頭的 `sb8Poll`，`m.Steps` 尚未遞增（444 §2.5） | 上一道指令的序號 | `e.Step + 1` |
| `irq7`、`defaulted` | `m.Steps++` 之前的 IRQ7 判斷（444 §2.7） | 上一道指令的序號 | `e.Step + 1` |

  依據：`RunUntilObserved` 在每次 `Step()` 之前呼叫觀察（`internal/machine/probe.go:207–212`），`sb8Event` 取 `m.Steps`（`sb_dma8.go:906`）。
- 回合在事件那一步結束時，下一次 `BeforeStep` 在下一次 `Advance` 的開頭，步數關係不變。其間的 `Deliver` 只寫輸入相關的狀態（BIOS 鍵盤緩衝、DOS 滑鼠，449、451）；消費端讀的區域不是這些時，讀到的值與在同一回合內讀相同（強推論）。
- 這個時點與 probe 的 `-dump-mem-at S`（`m.Steps == S`、下一次 `Step` 之前，`cmd/probe/main.go:992–998`）相同，所以 probe 收據可以當消費端的獨立期望。

### 2.5 狀態檔、摘要與 `SBDMALog`

- 不進狀態檔，不進 `Digest`（`session/digest.go:50–75` 只含記憶體、CPU 暫存器、Indexed、色盤、`Ext` 等）。
- 有觀察器時，`Advance` 期間的事件不進 `SBDMALog`（`sb8Log` 在 `OnSBDMA` 非 nil 時只呼叫它）。`SBDMALog` 不在狀態檔、不在 `Digest`，session 也沒有讀它的 API，所以同一起點、同一輸入下，有無 `SBDMAObserver` 的 `Digest` 相同（強推論；§3.1 第 8 項驗證）。遊戲上由 strike_commander 規格 010 §4.2 的三種執行驗證：不裝觀察器（不安裝 `OnSBDMA`）、裝了實作 `SBDMAObserver` 的觀察器但字幕為 nil（照樣安裝 `OnSBDMA`）、掛上字幕。安裝與否只看觀察器型別是否實作本介面，不看 app 內部欄位。
- `BootFromState`（450）之後只看得到啟動後才發生的事件。狀態檔存檔當下進行中的傳輸，之後會有 `complete`、`irq7`，但沒有對應的 `start`；消費端要能忽略找不到 `start` 的事件。

### 2.6 與 `-sb-dma-log` 的關係

- probe 的 `-sb-dma-log` 自己設 `m.OnSBDMA`（`cmd/probe/sbdma8.go:55`），不經 session；probe 與 session 不共用同一台機器，互不影響。
- 事件的 `Fields()` 與 `SBDMAEventFields` 同序（`sb_dma8.go:867–888`），消費端可以寫出與 `-sb-dma-log` 欄位相同的 TSV，逐列比對 probe 收據（§3.2、strike_commander 規格 010 §4.2）。

### 2.7 `SHA256` 的成本

- `sb8Event` 對 `start`、`complete` 無條件計算（`Command ≠ 80h` 時，`sb_dma8.go:919–921`），不管有沒有人消費。安裝 `SBDMAObserver` 不增加雜湊成本。
- 每次讀 `n` 個實體位址（DMA 長度，最多 64 KiB，`peekPhysRaw` 不寫記憶體）算一次 SHA-256。《Strike Commander》對話每句一到兩次 `start`，句長 1 到 4 秒（strike_commander RE 022 §3），相對每秒約 1158 萬步的模擬量可以忽略（強推論，未計時）。
- 產品端是否需要：strike_commander 規格 010 的 `CONV.PAK` 字幕以對話物件定 key，不用雜湊；過場 `MID*VOC.PAK` 與飛行中 `SPEECH.PAK` 不經對話物件，日後要用雜湊對回條目（RE 023 §6）。所以欄位保留。
- 改成延遲計算（只在有消費端時算）會改 444 §2.8 的行為與 probe 的收據，不在本規格範圍。

### 2.8 範圍外

- 聲音輸出、混音（444 §2.10）。
- 把 `SBDMALog` 或目前傳輸的狀態（進行中、`Due`、已傳數）開放給前端查詢。啟動後第一個 `start` 之前的那段播放，需要的話另立規格。
- 16 位元 DMA、自動初始化 DMA 等 444 未涵蓋的模式；那些模式加入時，若新增 Kind，照本規格一律轉發。

## 3. 驗收

### 3.1 單元測試（`session`）

以合成程式驅動 SB16（沿用 `internal/machine/sb_dma8_test.go` 對 `22Ch` 與 8237 埠的寫法，或在 `session` 測試中以 `.COM` 樁寫入相同序列）。觀察器同時記下每次 `BeforeStep` 的 `Steps()` 與收到的事件。

1. **轉發完整**：同一程式跑兩次。甲的觀察器實作 `SBDMAObserver`；乙不實作，跑完後由測試讀 `o.machine.SBDMALog`。甲收到的事件序列，每筆 `Fields()` 與乙的 `SBDMALog` 逐筆相同。序列至少含 `start`、`complete`、`irq7`、`eoi`、`pause`、`resume`、`cancel`（`D0h`、`D4h`、暫停中的新 `14h` 造成隱式取消）、一筆靜音 `80h`、一筆 `control-idle`（沒有傳輸時送 `D0h`，446 §2.3），一筆 `defaulted`（IRQ7 拉起時 IVT[0Fh] 仍指向 `StubSeg`，444 §2.7 第 5 步；在程式裝處理常式之前觸發一次傳輸完成即可），以及一筆未經暫停、由 DSP 重設（`226h←1`）造成的 `cancel`（`sb_dma8.go:142、421–424`）。測試明文斷言九種 Kind（`start`、`complete`、`cancel`、`pause`、`resume`、`control-idle`、`eoi`、`defaulted`、`irq7`）都至少出現一次。
2. **時序**：`start`、`pause`、`resume`、`cancel`、`eoi`、`control-idle` 之後下一次 `BeforeStep` 的 `Steps()` 等於 `e.Step`；`complete`、`irq7`、`defaulted` 之後等於 `e.Step + 1`。在 `start` 之後的那次 `BeforeStep` 以 `Read8` 讀 DMA 緩衝，SHA-256 等於 `e.SHA256`。這一條只在合成程式不開分頁時成立：`Read8` 走 `Peek8`，以線性位址經分頁轉換讀取；`sb8SHA` 走 `peekPhysRaw`，直接讀實體位址（`sb_dma8.go:935–944`）。開分頁時兩者可能讀到不同的 bytes，本規格不要求相等。
3. **跨回合**：讓 `Advance` 的預算恰好停在 `start` 那一步之後。事件在本回合送達；下一回合第一次 `BeforeStep` 的 `Steps()` 等於 `e.Step`。
4. **移除**：`Advance` 返回後 `o.machine.OnSBDMA == nil`；之後以 `machine` 直接執行一段程式，事件進 `SBDMALog`。
5. **不實作**：觀察器不實作 `SBDMAObserver` 時，`Advance` 期間 `OnSBDMA` 為 nil，`SBDMALog` 增加。
6. **panic**：`SBDMA` 內 panic，`Advance` 回觀察器錯誤，錯誤字串含 `SBDMA`；機器停在發生 panic 那一步驟嘗試完成之後（與 `OPLWrite` 的既有測試同法）。
7. **誤用**：`SBDMA` 內使用保留的 `StepView` 讀記憶體，得到 0，`Advance` 回「觀測器在回呼外使用了 StepView」。
8. **摘要**：測試 1 的甲、乙在同一停止步的 `Digest` 全同。這一項沒有對應的突變（安裝 `OnSBDMA` 本來就不改執行狀態），作為回歸護欄。
9. **選項關閉**：`Config.SBDMA8` 為假、觀察器實作 `SBDMAObserver` 時，`New` 不回錯誤，`Advance` 期間回呼一次都不被呼叫（§4 第 1 項）。

### 3.2 遊戲上的驗收

本規格不放遊戲專屬的長測試（通用層不依賴遊戲 app）。以《Strike Commander》實機驗證轉發結果的工作在 strike_commander 規格 010 §4.2 的「輸入重現的前置步驟」：session 經本介面記下的 `start` 列，與 probe `-sb-dma-log` 收據逐欄比對 `Step`、`Samples`、`Due`、`SHA256`。session 的滑鼠與按鍵輸入（`move`、`down`、`up`、`kdown`、`kup`）與 probe 的 `-clicks`、`-hold` 語意不同，對應方式與不一致時的處置寫在 010 §4.2。

### 3.3 負對照（實作子代理逐列執行，結果寫進 §5）

| 編號 | 突變 | 必須失敗 |
|---|---|---|
| N1 | 不安裝 `OnSBDMA` | 3.1 第 1 項 |
| N2 | `defer` 不設回 nil | 3.1 第 4 項 |
| N3 | 只轉發 `start` | 3.1 第 1 項 |
| N4 | 回呼不經 `guard` | 3.1 第 6 項（panic 穿出 `Advance`） |
| N5 | 不論觀察器是否實作都設 `OnSBDMA`（空函式） | 3.1 第 5 項 |
| N6 | 轉發前清空 `SHA256` | 3.1 第 1、2 項 |
| N7 | 在回呼前後開關 `viewOpen`，讓回呼可以讀記憶體 | 3.1 第 7 項 |
| N8 | 過濾掉 `control-idle`、`defaulted` | 3.1 第 1 項 |

實作子代理證明為等價突變的列，在 §5 記錄處置，不退回 DRAFT。

### 3.4 回歸

改動前後 `go test -count=1 ./...` 的套件結果相同（3 個 ebiten 套件在無顯示器容器裡改動前就失敗，照 449、450、451 的記法）。444 到 446 的單元測試、449、450、451 的長測試、strike_commander 規格 001 到 008 的長測試重跑通過。`000-index.md` 登記 452（`internal/repolint` 會檢查）。

## 4. 決定

| 編號 | 事項 | 決定 | 理由 |
|---|---|---|---|
| 1 | 觀察器實作 `SBDMAObserver` 但 `Config.SBDMA8` 為假 | 照常建立，永遠沒有事件，`New` 不回錯誤（§3.1 第 9 項） | 與 `AudioObserver` 一致：`AdLib` 為假時同樣照常安裝、沒有事件（`session/observer.go:183–187`）。配置錯誤由 app 自己的收據檢查（strike_commander 規格 010 §4.2 的長測試斷言 `SBDMA8` 為真）。第一、二輪審查都沒有異議 |

## 5. 實作收據

2026-10-20，strike_commander 實作子代理。程式：`session/observer.go`（介面與 `runObserved` 的安裝、`defer` 移除）；測試：`session/sbdma_observer_test.go`。執行環境：Docker `dosgolem-go-ebiten:1.24-r1`，`--network none`。

**實作細節**：`runObserved` 以 `so, sbdma := o.observer.(SBDMAObserver)` 判斷一次；`defer` 只在 `sbdma` 為真時把 `OnSBDMA` 設回 nil，不實作時完全不碰 `OnSBDMA`（§2.3「不碰」）。

**合成程式**（`.COM`，不開分頁）分六段：(1) 未裝 IRQ7 處理常式時一次 16 bytes 的 `14h` 傳輸（輪詢 `08h` 的 TC），之後讀 `22Eh` 清 `IRQPending`；(2) 裝處理常式（讀 `22Eh`、設旗標、`out 20h,20h`）後一次 `14h`；(3) `14h` 後 `D0h`、`D4h`、`D0h`，暫停中改寫通道 1 的 DMA 再送 `14h`；(4) 沒有傳輸時送 `D0h`；(5) 靜音 `80h`；(6) `14h` 後寫 `226h←1`。

**結果**（`go test -count=1 -run SBDMA -v ./session/`，全部 PASS）：

| 項目 | 測試 | 結果 |
|---|---|---|
| 1、2、8 | `TestSBDMAObserverForwardsEveryEvent` | 23 筆事件與乙的 `SBDMALog` 逐筆 `Fields()` 相同；九種 Kind 計數：start 6、complete 4、cancel 2、pause 2、resume 1、control-idle 1、eoi 3、defaulted 1、irq7 3；最後兩筆是 start、cancel（DSP 重設）；每筆的下一次 `Steps()` 符合 §2.4 表；4 筆 `14h` start 的 `Read8` 雜湊等於 `e.SHA256`；甲、乙 `Digest` 相同 |
| 3 | `TestSBDMAObserverAcrossTurns` | 預算停在第一筆 start 那一步，本回合送達 1 筆；下一回合第一次 `Steps()` 等於 `e.Step`，雜湊相同 |
| 4 | `TestSBDMAObserverRemovedAfterTurn` | 返回後 `OnSBDMA == nil`；直接 `Step()` 的 start 進 `SBDMALog` |
| 5 | `TestSBDMAObserverNotImplemented` | `Advance` 期間 `OnSBDMA` 一直是 nil，`SBDMALog` 增加 |
| 6 | `TestSBDMAObserverPanicIsObserverFault` | 錯誤含 `SBDMA`，停在第一筆 start 那一步之後 |
| 7 | `TestSBDMAObserverRetainedViewMisuse` | 回呼內讀得 0，回「觀測器在回呼外使用了 StepView」 |
| 9 | `TestSBDMAObserverWithoutSBDMA8` | `New` 成功，跑 20000 步回呼 0 次 |

第 7 項的保留視圖取「最近一次 `BeforeStep` 的視圖」（token 與 `Owner` 目前的相同，只差 `viewOpen` 已關）。若取第一次的視圖，token 已過期，N7 的突變（只開關 `viewOpen`）仍會被 token 擋下，N7 成為等價突變；取最近一次才能讓第 7 項區分 `viewOpen` 的開關。

**負對照**（每條在 dosgolem-sc 工作樹副本上做字串突變，只跑 `-run SBDMA ./session/`；工具與輸出在 strike_commander `workplace/i010/neg452.json`、`neg/N*/result.txt`）：

| 編號 | 失敗的測試 | 結果 |
|---|---|---|
| N1 | 第 1 項（甲的 `SBDMALog` 有 23 筆）、第 3、4 項 | 失敗，如期 |
| N2 | 第 4 項（返回後 `OnSBDMA` 仍非 nil） | 失敗，如期 |
| N3 | 第 1 項（甲收到 6 筆，乙 23 筆） | 失敗，如期 |
| N4 | 第 6 項（`sbdma boom` panic 穿出） | 失敗，如期 |
| N5 | 第 5 項（不實作時 `OnSBDMA` 非 nil）；第 1 項（乙的 `SBDMALog` 0 筆） | 失敗，如期 |
| N6 | 第 1 項（第 0 筆 `Fields()` 不同）、第 3 項（`Read8` 雜湊與空 `SHA256` 不同） | 失敗，如期 |
| N7 | 第 7 項（回呼內讀得 FBh，要 0） | 失敗，如期 |
| N8 | 第 1 項（甲收到 21 筆，乙 23 筆） | 失敗，如期 |

**遊戲上**（§3.2，strike_commander 規格 010 §4.2 前置步驟）：C11、T10、H12、K1、K2 五個案例，session 經本介面記下的 start、complete、pause、resume、cancel 與 probe 收據逐列比 `Kind`、`Step`、`Due`，start 另比 `Samples`、`SHA256`，分別 24、36、48、7、1 列全部相同。

**回歸**（§3.4）：改動前後 `go test -count=1 ./...` 套件結果相同（各 43 ok、6 FAIL；FAIL 為 3 個 ebiten 套件、`phantasie-play`，以及 `apps/strikecommander` 的 `TestBuiltFont` 因環境變數指向不含測試字的正式字型，兩邊相同）；`internal/repolint` 通過；448、449、450 的長測試（`TestLegacyPMProfileTrigger`、`TestIRQ1ScanTrigger`、`TestBootFromStateTrigger`）、451 的 `TestPointerMoveTrigger`、strike_commander 規格 001 到 008 的長測試全部通過。

## 6. 第一輪審查處理

審查 A、B 的原文在 strike_commander `workplace/s010/review-1a.md`、`review-1b.md`。

| 編號 | 類別 | 處理 |
|---|---|---|
| A-9 | 應改 | §3.1 第 2 項的 `Read8` 與 `e.SHA256` 相等限定在不開分頁的合成程式，寫明 `Peek8`（線性）與 `peekPhysRaw`（實體）的差別 |
| A-10 | 應改 | 原 §3.2 的遊戲長測試移到 strike_commander 規格 010 §4.2 的前置步驟；session 與 probe 輸入語意不同的處置寫在 010 |
| A 建議：§3.2 移到 010 | 採用 | 同上；本規格 §3.2 只留指引，N1 不再引用 3.2 |
| B-A12 | 應改 | §3.1 第 1、2 項補 `control-idle`、`defaulted`，新增 N8；檔名定為 `452-session-sbdma-observer.md`；待定第 1 項引用的「010 長測試斷言 `SBDMA8`」已寫進 010 §4.2 |

## 7. 第二輪審查處理

審查原文在 strike_commander `workplace/s010/review-2a.md`、`review-2b.md`。審查 A 判定 452 可直接 READY；兩份都無阻擋。

| 編號 | 類別 | 處理 |
|---|---|---|
| 2A 建議：DSP 重設的 `cancel` | 採用 | §3.1 第 1 項補一筆未經暫停、由 DSP 重設造成的 `cancel` |
| 2B：§2.5 的 A/B 說法 | 採用 | 改為引用 010 §4.2 的三種執行，寫明安裝 `OnSBDMA` 只看觀察器型別 |
| 2B：測試 1 斷言九種 Kind | 採用 | §3.1 第 1 項明文斷言九種 Kind 都出現 |
| 2B：`SBDMA8` 為假的測試 | 採用 | 新增 §3.1 第 9 項；§4 由待定改為決定 |
| 2B：第 8 項沒有突變 | 採用 | §3.1 第 8 項註明作為回歸護欄 |
