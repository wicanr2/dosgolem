# 439 — 狀態檔保存 PIT 通道 0 的埠解碼分頻

狀態：**READY**
日期：2026-10-18
前置：[`191-bios-rom-tail`](191-bios-rom-tail.md)（狀態檔 v3）、[`190-opl-vgm-dump`](190-opl-vgm-dump.md) §3（`StepsPerSecondNow()`）、
[`438-mscdex-cd-audio-dump`](438-mscdex-cd-audio-dump.md)
案例：三國志V（san5）從狀態檔接續後倒 CD 音軌

---

## 1. 為什麼要做

機器裡有兩份 PIT 通道 0 的分頻：`Machine.PITDiv`（`setPITDiv` 維護，決定 IRQ0 間隔，已存進狀態檔）與
`pit.divisor`（`pit.go` 的埠解碼，`PITDivisor()`／`PITHz()` 讀它，沒存）。

`-load-state` 之後 `pit.divisor` 是 0，`PITHz()` 退回 65,536 分頻的 18.2 Hz，而 IRQ0 間隔照舊，
`StepsPerSecondNow()`（= `IRQ0Every × PITHz()`）因此小了「65,536 ÷ 分頻」倍。三國志V 的分頻是 4,096：
同樣跑到 MAIN，新開的執行算出每秒 11.58 M 道，從狀態檔接續的算出 0.72 M 道，倒出的 CD WAV 時間軸
拉長 16 倍（量測：`wicanr2/san5-remake` 的 `docs/re/013-main-to-title-menu.md`）。VGM 與 OPL WAV 有同樣的問題。

## 2. 語意

- 狀態檔多兩個欄位 `PITPortDivisor`、`PITPortWrites`，存 `pit.divisor` 與 `pit.writes`，讀回時照原樣設回。
- 不改版本號：gob 對缺少的欄位給零值，舊狀態檔讀進來兩欄是 0，行為與以前相同（分頻退回預設、`PITProgrammed()` 為假）。
- 兩份分頻的合併不在本規格（各有呼叫端，合併屬於另一次整理）。

## 3. 驗收

1. `internal/machine/pit_test.go` 的 `TestStateKeepsPITPortDivisor`：設分頻 4,096 後存讀，`PITHz()` 相同、`PITProgrammed()` 為真。
2. 負對照：不設回 `pit.divisor`，第 1 條必須失敗（2026-10-18 手動確認後還原）。
3. 三國志V：從狀態檔接續跑出的 `StepsPerSecondNow()` 與新開的執行相同。
