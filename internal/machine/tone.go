package machine

import "fmt"

// PC 喇叭的方波：PIT 通道 2 ＋ 埠 61h 的閘門與資料致能（`docs/spec/195`）。
//
// 音樂與嗶聲走這一條：61h 只在音符開始、結束各寫一次，聲音本身是通道 2
// 以分頻值產生的方波。`Speaker` 只收資料線電平（語音用），這條路在它裡面
// 幾乎是空的——看起來像「沒出聲」，其實是「方波沒被合成」。

// ToneEvent 是喇叭方波的一次變化：從第 Step 道指令起頻率是 Hz（0 ＝ 靜音）。
type ToneEvent struct {
	Step uint64
	Hz   float64
}

// ToneDecoder 逐筆吃埠寫入，追蹤通道 2 的方波頻率（`195` §3.1）。串流音訊（`199` §3.5）與 ToneEvents 共用。
type ToneDecoder struct {
	rw       uint8   // 通道 2 的存取方式；BIOS 預設先低後高
	lowNext  bool    // 先低後高時，下一個位元組是不是低位元組
	lo       uint8   // 先低後高時暫存的低位元組
	div      uint32  // 目前分頻值（0 ＝ 65536）
	divSet   bool    // 分頻值被設過沒有
	gateData bool    // 61h 的 bit0 與 bit1 都是 1
	cur      float64 // 目前頻率；開機是靜音
}

// NewToneDecoder 造一個開機狀態（靜音、先低後高）的解碼器。
func NewToneDecoder() *ToneDecoder { return &ToneDecoder{rw: 3, lowNext: true} }

// Hz 是目前的方波頻率，0 ＝ 靜音。
func (t *ToneDecoder) Hz() float64 { return t.cur }

// Feed 吃一筆埠寫入；頻率（含靜音）因此改變時回新頻率與 true。
func (t *ToneDecoder) Feed(w PortWrite) (float64, bool) {
	switch w.Port {
	case 0x43:
		if w.Val>>6&3 != 2 {
			return t.cur, false
		}
		if a := w.Val >> 4 & 3; a != 0 { // 0 ＝ 鎖存命令，不改存取方式
			t.rw, t.lowNext = a, true
		}
		return t.cur, false
	case 0x42:
		switch t.rw {
		case 1:
			t.div, t.divSet = uint32(w.Val), true
		case 2:
			t.div, t.divSet = uint32(w.Val)<<8, true
		default:
			if t.lowNext {
				t.lo, t.lowNext = w.Val, false
				return t.cur, false
			}
			t.div, t.divSet, t.lowNext = uint32(t.lo)|uint32(w.Val)<<8, true, true
		}
	case 0x61:
		t.gateData = w.Val&3 == 3
	default:
		return t.cur, false
	}
	hz := 0.0
	if t.gateData && t.divSet {
		d := t.div
		if d == 0 {
			d = 65536
		}
		hz = PITBaseHz / float64(d)
	}
	if hz == t.cur {
		return hz, false
	}
	t.cur = hz
	return hz, true
}

// ToneEvents 從埠寫入紀錄重建方波的變化序列（`195` §3.1）。只在頻率（含靜音）改變時記一筆。
func (m *Machine) ToneEvents() []ToneEvent {
	var out []ToneEvent
	dec := NewToneDecoder()
	for _, w := range m.PortLog {
		if hz, changed := dec.Feed(w); changed {
			out = append(out, ToneEvent{Step: w.Step, Hz: hz})
		}
	}
	return out
}

// TonePCM 把方波合成成 8 位元單聲道取樣（`195` §3.2）：發聲 20h／E0h、靜音 80h，相位連續。
// rate 為 0 時用 22,050。時間軸是 StepsPerSecond。
func (m *Machine) TonePCM(rate int) ([]uint8, error) {
	if rate <= 0 {
		rate = 22050
	}
	ev := m.ToneEvents()
	start := -1
	for i, e := range ev {
		if e.Hz > 0 {
			start = i
			break
		}
	}
	if start < 0 {
		return nil, fmt.Errorf("通道 2 的方波一次都沒響過——這一段沒有音樂或嗶聲")
	}
	ev = ev[start:]
	first, last := ev[0].Step, ev[len(ev)-1].Step
	if ev[len(ev)-1].Hz > 0 && m.Steps > last {
		last = m.Steps // 最後一個音還沒關就結束了，算到最後一道指令
	}
	sps := m.InstructionsPerSecond() // 跟著機器速度走（`198`）
	n := int(float64(last-first) / sps * float64(rate))
	if n <= 0 {
		return nil, fmt.Errorf("方波只有 %d 道指令長，不足一個取樣", last-first)
	}
	pcm := make([]uint8, n)
	phase := 0.0
	j := 0
	for i := range pcm {
		step := first + uint64(float64(i)/float64(rate)*sps)
		for j+1 < len(ev) && ev[j+1].Step <= step {
			j++
		}
		hz := ev[j].Hz
		if hz <= 0 {
			pcm[i] = 0x80
			continue
		}
		if phase < 0.5 {
			pcm[i] = 0xE0
		} else {
			pcm[i] = 0x20
		}
		phase += hz / float64(rate)
		phase -= float64(int(phase))
	}
	return pcm, nil
}
