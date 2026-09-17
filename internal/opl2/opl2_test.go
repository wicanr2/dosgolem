package opl2

import (
	"math"
	"testing"
)

const rate = 22050

// pureTone 設一個相加模式、只有載波出聲的聲道 0。
func pureTone(s *Synth, fnum uint16, block uint8, tl uint8) {
	s.Write(0xC0, 0x01) // 相加、無回授
	s.Write(0x20, 0x21) // 調變器：持續、MULT 1
	s.Write(0x40, 0x3F) // 調變器 TL 63（靜音）
	s.Write(0x23, 0x21) // 載波：持續、MULT 1
	s.Write(0x43, tl)   // 載波 TL
	s.Write(0x63, 0xF0) // 載波 AR 15、DR 0
	s.Write(0x83, 0x0F) // 載波 SL 0、RR 15
	s.Write(0xA0, uint8(fnum))
	s.Write(0xB0, 0x20|block<<2|uint8(fnum>>8))
}

func render(s *Synth, n int) []float64 {
	out := make([]float64, n)
	for i := range out {
		out[i] = s.Sample()
	}
	return out
}

func rms(x []float64) float64 {
	t := 0.0
	for _, v := range x {
		t += v * v
	}
	return math.Sqrt(t / float64(len(x)))
}

func rising(x []float64) int {
	n := 0
	for i := 1; i < len(x); i++ {
		if x[i-1] <= 0 && x[i] > 0 {
			n++
		}
	}
	return n
}

// fnumFor 回 block 4 下最接近 hz 的 F-Number。
func fnumFor(hz float64) uint16 { return uint16(math.Round(hz * math.Pow(2, 16) / NativeRate)) }

func TestPureTonePitch(t *testing.T) {
	s := New(rate)
	pureTone(s, fnumFor(440), 4, 0)
	got := rising(render(s, rate/2)) * 2
	if got < 431 || got > 449 {
		t.Errorf("一秒 %d 個週期，要 440 ±2%%", got)
	}
}

func TestKeyOffReleases(t *testing.T) {
	s := New(rate)
	fn := fnumFor(440)
	pureTone(s, fn, 4, 0)
	before := rms(render(s, rate/10))
	s.Write(0xB0, 4<<2|uint8(fn>>8)) // Key-Off
	render(s, rate/20)
	after := rms(render(s, rate/10))
	if after > before*0.01 {
		t.Errorf("RR 15 放開 50 ms 後 RMS %.4f，放開前 %.4f", after, before)
	}
}

func TestTotalLevelAttenuates(t *testing.T) {
	a := New(rate)
	pureTone(a, fnumFor(440), 4, 0)
	b := New(rate)
	pureTone(b, fnumFor(440), 4, 8) // 6 dB
	ra, rb := rms(render(a, rate/4)), rms(render(b, rate/4))
	if r := rb / ra; r < 0.45 || r > 0.55 {
		t.Errorf("TL 8 對 TL 0 的 RMS 比 %.3f，要約 0.5", r)
	}
}

func TestFMChangesWaveform(t *testing.T) {
	plain := New(rate)
	pureTone(plain, fnumFor(440), 4, 0)
	fm := New(rate)
	pureTone(fm, fnumFor(440), 4, 0)
	fm.Write(0xC0, 0x00) // FM
	fm.Write(0x40, 0x00) // 調變器全開
	fm.Write(0x60, 0xF0) // 調變器 AR 15
	fm.Write(0xB0, 0x00)
	fm.Write(0xB0, 0x20|4<<2|uint8(fnumFor(440)>>8)) // 重新 Key-On 讓調變器起音
	a, b := render(plain, rate/4), render(fm, rate/4)
	d := make([]float64, len(a))
	for i := range a {
		d[i] = a[i] - b[i]
	}
	if rms(d) <= 0.1*rms(a) {
		t.Errorf("FM 與純正弦差值 RMS %.4f，太像", rms(d))
	}
}

func TestRhythmModeCounted(t *testing.T) {
	s := New(rate)
	s.Write(0xBD, 0x20)
	if s.Unsupported["rhythm"] != 1 {
		t.Errorf("節奏模式沒被記下：%v", s.Unsupported)
	}
}
