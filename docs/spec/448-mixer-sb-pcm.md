# 448 — 混音器的 SB 數位聲道

狀態：**READY**（呼叫端契約；觸發案例見 §2）
日期：2026-10-22
前置：[`445-real-mode-sb-single-dma`](445-real-mode-sb-single-dma.md)、[`240`](000-index.md)（音訊觀測與混音器）

---

## 1. 規則

| 項目 | 行為 |
|---|---|
| 機器端 | `Machine.ObserveSB(func(SBSample))`：445 的單次 DMA 每傳出一個 byte 就以 `SBSample{Step, Value}` 呼叫一次。掛上時 `SBPCM` 不再追加 |
| 混音器 | `Mixer.SBSample(SBSample)` 收事件；`Render` 依步數套用，取樣以零階保持到下一個取樣 |
| 位準 | 8 位元無號，80h 為靜音；輸出 `(v − 128) / 128 × 0.6`，左右聲道相同，與 OPL、喇叭相加後再夾值 |
| 時間 | 與 OPL 同一條時間軸：樣本序號 ＝ 步數 × 48000 ÷ 每秒步數 |

不做：重新取樣濾波、SB 喇叭開關（DSP D1／D3）的靜音、16 位元與立體聲。

## 2. 觸發案例

智冠中文版《魔法門 V》進入設施時播放中文語音（鐵匠鋪 15990 bytes、約 14706 Hz，445 §2）。
445 之後取樣已經依取樣率傳出，但混音器沒有 SB 聲道，前端聽不到。

## 3. 驗收

1. `audio/mixer/mixer_test.go` 的 `TestSBSampleHoldsUntilNext`：FFh 輸出 127/128×0.6、保持到下一個取樣、80h 之後靜音。
2. `go test ./...` 全綠（Xvfb）。
