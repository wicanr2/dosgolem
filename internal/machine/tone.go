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

// ToneEvents 從埠寫入紀錄重建方波的變化序列（`195` §3.1）。只在頻率（含靜音）改變時記一筆。
func (m *Machine) ToneEvents() []ToneEvent {
	var out []ToneEvent
	rw := uint8(3)    // 通道 2 的存取方式；BIOS 預設先低後高
	lowNext := true   // 先低後高時，下一個位元組是不是低位元組
	var lo uint8      // 先低後高時暫存的低位元組
	var div uint32    // 目前分頻值（0 ＝ 65536）
	divSet := false   // 分頻值被設過沒有
	gateData := false // 61h 的 bit0 與 bit1 都是 1
	cur := 0.0        // 上一筆記下的頻率；開機是靜音，從 0 起算，一開始的靜音不記
	emit := func(step uint64) {
		hz := 0.0
		if gateData && divSet {
			d := div
			if d == 0 {
				d = 65536
			}
			hz = PITBaseHz / float64(d)
		}
		if hz != cur {
			out = append(out, ToneEvent{Step: step, Hz: hz})
			cur = hz
		}
	}
	for _, w := range m.PortLog {
		switch w.Port {
		case 0x43:
			if w.Val>>6&3 != 2 {
				continue
			}
			if a := w.Val >> 4 & 3; a != 0 { // 0 ＝ 鎖存命令，不改存取方式
				rw, lowNext = a, true
			}
		case 0x42:
			switch rw {
			case 1:
				div, divSet = uint32(w.Val), true
			case 2:
				div, divSet = uint32(w.Val)<<8, true
			default:
				if lowNext {
					lo, lowNext = w.Val, false
					continue
				}
				div, divSet, lowNext = uint32(lo)|uint32(w.Val)<<8, true, true
			}
			emit(w.Step)
		case 0x61:
			gateData = w.Val&3 == 3
			emit(w.Step)
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
	sps := StepsPerSecond()
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
