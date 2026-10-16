# 445 — 一般機器 SB16 8 位元 DMA 的暫停與恢復（DSP D0h／D4h）

狀態：**DRAFT**（第 1 版）
日期：2026-10-17
前置：[`444-legacy-sb16-dma8-irq7`](444-legacy-sb16-dma8-irq7.md)、[`309-sb16-pause-resume-dma8`](309-sb16-pause-resume-dma8.md)

---

## 1. 為什麼要做

《Strike Commander》SB16 設定（`SC.CFG` 含 `-ma -ds`），probe 帶 `-mode13-phase -sound-blaster -adlib -sb-dma8`，冷開機：MENU 的音效偵測通過（DMA 完成 3 次、IRQ7 遞送 3 次），
播放 59750 bytes 的 8 位元取樣（第 16823080 步開始、第 79099541 步完成並遞送 IRQ7），第 125256614 步開始播 18430 bytes，第 131427746 步驅動在 `B5C5:03F9`
寫 `22Ch←D0h`，444 失敗即關閉：「DSP 寫入 22Ch←D0h 未被接受」（`strike_commander` 的 `workplace/logs/probe470.log`、`workplace/scratch/p470/sbdma.tsv`，已證實）。

`D0h` 是單一位元組命令，暫停 8 位元 DMA 請求，適用單次與自動初始化；`D4h` 恢復被 `D0h` 暫停的傳輸（Creative「Sound Blaster Series Hardware Programming Guide」
印刷頁 6-24、6-26，見規格 309 的引用）。`internal/machine/sb_dsp.go` 約 214–216 行已在無待收參數時把 `D0h`／`D4h` 交給 `ControlDMA8(paused bool)`；
一般機器的 444 沒有設這個回呼，所以回 false。LE 路徑由 309 接上，不在本規格範圍。

## 2. 規則

`SetSBDMA8(true)` 時（`wireSB8`）一併設 `soundBlaster.ControlDMA8`，指向新的 `sb8Control(paused bool) bool`。選項關閉時不設，行為不變。

1. **前提**：有進行中的 DMA 傳輸（`sb8.active` 且命令屬 `14h`、`74h`–`77h`、`16h`、`17h`）。沒有進行中的傳輸、或進行中的是 `80h` 靜音，回 false（失敗即關閉，沿用 444 的
   DSP 拒絕路徑）。理由：309 對 idle 控制未知，採拒絕；靜音沒有 DMA 請求可暫停。
2. **`D0h`（暫停）**：未暫停時，記 `pausedAt = sb8Now()`，設 `paused = true`；已暫停時不動作（冪等）。暫停期間：
   - `sb8Poll` 不完成傳輸（`paused` 為真時直接返回）。
   - `sb8Progress`（DMA 位址與計數讀值）以 `pausedAt` 當作「現在」計算，讀值固定在暫停那一刻。
   - IRQ7、PIC、DMA 暫存器其餘狀態不動；已待處理的 IRQ7 照常遞送。
3. **`D4h`（恢復）**：已暫停時，`d = sb8Now() − pausedAt`，`start += d`、`due += d`（溢位時失敗即關閉，同 444 §2.4），清 `paused`；未暫停時不動作（冪等）。
   恢復後剩餘取樣數與暫停前相同，不補暫停期間的取樣。
4. **取消與重設**：`sb8Cancel`、DSP 重設（`226h`）、新傳輸開始時清 `paused`。
5. **觀測**：`SBDMAEvent` 增加事件種類 `pause`、`resume`（CS:IP 取本道指令起點；傳輸欄位照帶）。`resume` 另在 `Due` 欄寫新的到期時刻。
6. **狀態檔**：`sbDMA8State` 加匯出欄位 `Paused bool`、`PausedAt uint64`。版本仍為 4（第二個 gob 值的結構多兩個欄位；舊的版本 4 狀態檔讀入時兩欄為零值，即未暫停）。
   `sb8Apply` 驗證：`Paused` 時必須 `Active` 且 `PausedAt` 介於 `Start` 與 `Due` 之間，否則拒絕（同 444 的不一致處理）。`StateDigest` 照 444 的接續規則涵蓋新欄位。
7. 範圍外：16 位元控制（`D5h`、`D6h`）、自動初始化的暫停（444 沒有自動初始化）、`D0h` 期間的取樣信用模型（沒有音訊輸出）。

## 3. 驗收

1. 選項關閉：`go test ./internal/... ./cmd/probe ./session ./oracle` 與 `go test ./apps/...` 與改動前相同；LE 的 309 測試不變。
2. `internal/machine` 新測試（選項開啟）：
   - `14h` 傳輸中 `D0h`：之後 N 步不完成、DMA 位址與計數讀值固定；`D4h` 後在 `原 due + 暫停步數` 那一步完成並遞送 IRQ7（步號寫死）。
   - `74h` ADPCM 一組。
   - 重複 `D0h`、重複 `D4h` 冪等；未暫停時 `D4h` 不動作。
   - 無傳輸時 `D0h`、`80h` 靜音中 `D0h` 失敗即關閉。
   - 暫停中 `226h` 重設、`sb8Cancel` 清 `paused`；暫停中開始新傳輸清 `paused`。
   - 暫停中已待處理的 IRQ7 照常遞送。
   - 存讀檔：暫停中存檔、讀回後繼續暫停，`D4h` 後完成步號與不存讀相同；不一致的 `Paused` 狀態被拒。
   - 觀測紀錄有 `pause`、`resume` 事件與正確的 `Due`。
3. 突變自檢：暫停時 `sb8Poll` 照常完成、`D4h` 不平移 `due`、`sb8Progress` 不凍結、重設不清 `paused`、`sb8Apply` 不驗證，各至少一個測試失敗。
4. `gofmt`、`go build ./...`、`go vet ./internal/... ./cmd/probe` 無輸出；SingleStepTests 不得 skip；錨點不變。
5. 觸發案例（需原版，主代理另附收據）：同 §1 設定冷開機，越過第 131427746 步的 `D0h`，記錄後續 `D4h` 是否出現、MENU 的去向與下一個停點。
