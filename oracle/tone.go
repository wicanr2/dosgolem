package oracle

import (
	"io"

	"github.com/wicanr2/dosgolem/internal/machine"
)

// PC 喇叭的方波（PIT 通道 2，`docs/spec/195`）：音樂與嗶聲走這一條，語音走 `Speaker`。

// ToneEvent 是方波的一次變化：從第 Step 道指令起頻率是 Hz（0 ＝ 靜音）。
type ToneEvent = machine.ToneEvent

// ToneEvents 回方波的變化序列。
func (o *Oracle) ToneEvents() []ToneEvent { return o.m.ToneEvents() }

// ToneWAV 把方波寫成 8 位元單聲道 WAV；rate 為 0 用 22,050。
// 沒有任何發聲事件時回錯誤——沒聲音要講出來，不寫一個空檔案。
func (o *Oracle) ToneWAV(w io.Writer, rate int) error {
	if rate <= 0 {
		rate = 22050
	}
	pcm, err := o.m.TonePCM(rate)
	if err != nil {
		return err
	}
	return writeWAV8(w, rate, pcm)
}
