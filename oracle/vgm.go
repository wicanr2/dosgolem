package oracle

import (
	"io"

	"github.com/wicanr2/dosgolem/internal/vgm"
)

// OPL 的暫存器寫入倒成 VGM（`docs/spec/190-opl-vgm-dump`）。
//
// `OPLWrites()` 回的那一串已經是樂譜了，只是沒有人能播。這裡把它編成
// 標準格式，外部播放器就能離線合成——原版的 FM 配樂因此不必拿 MIDI
// 去套 General MIDI 音色庫（那得到的是「同一首曲子、不同的音源」）。

// VGMStats 是倒出來的東西的摘要。
type VGMStats = vgm.Stats

// StepsPerSecond 是這台機器現在的名目速度（指令／秒），
// 由計時器自己的設定推出來：`IRQ0Every × PITHz`。
//
// ⚠ **用 `-tick` 釘死間隔時它與 `machine.StepsPerSecond()` 不同。**
// 那個常數的前提是 IRQ0 間隔跟著 PIT 分頻走；釘死之後前提不成立，
// 而兩者可以差到四倍（`docs/spec/190-opl-vgm-dump` §3）。
func (o *Oracle) StepsPerSecond() float64 { return o.m.StepsPerSecondNow() }

// OPLVGM 把目前的 OPL 暫存器寫入序列寫成 VGM。
//
// stepsPerSecond 傳 0 ＝ 用 `StepsPerSecond()`。零點取第一筆寫入——
// 開機到第一個音符之間的空白是載入時間，不是樂曲的一部分。
//
// 搭 `ClearOPL()` 框出「只有這一首」：`OnFileOpen` 收到曲檔的檔名時
// 清一次，之後收到的就只有那一首（`ClearOPL` 不動暫存器檔，
// 所以驅動開機灌進去的預設音色還在）。
//
// ⚠ **序列是空的會回錯誤。** 最常見的原因是沒叫 `AdLib(true)`：
// 偵測失敗的程式整段跳過音樂路徑，而空序列與「這個程式沒有音樂」
// 長得一模一樣。
func (o *Oracle) OPLVGM(w io.Writer, stepsPerSecond float64) (VGMStats, error) {
	return o.m.OPLVGM(w, stepsPerSecond)
}
