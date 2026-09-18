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

// `196` §4 第 7 項：KSL 的連續公式與真機的 16 段整數表相差 ≤ 1.5 dB。
// kslRefTable 是 YM3812 把 FNum 高 4 位對到「要從 Block×8 扣掉多少」的分段表（單位 0.75 dB），
// **只在這裡當驗收的對照曲線用**，不進實作（`196` §5）。
func TestKslMatchesChipTable(t *testing.T) {
	kslRefTable := [16]float64{64, 32, 24, 19, 16, 12, 11, 10, 8, 6, 5, 4, 3, 2, 1, 0}
	worst := 0.0
	for ksl := uint8(1); ksl <= 3; ksl++ {
		for block := uint8(0); block < 8; block++ {
			for hi := 0; hi < 16; hi++ {
				ref := float64(block)*8 - kslRefTable[hi]
				if ref < 0 {
					ref = 0
				}
				want := ref * 0.75 * kslScale[ksl]
				got := kslAtten(ksl, block, uint16(hi)<<6)
				if d := math.Abs(got - want); d > worst {
					worst = d
				}
			}
		}
	}
	if worst > 1.5 {
		t.Fatalf("與真機分段表最大差 %.2f dB，要 ≤ 1.5", worst)
	}
	if kslAtten(0, 7, 0x3FF) != 0 {
		t.Error("KSL 位元 0 不該衰減")
	}
	// 位元 1 比位元 2 強（OPL 的慣例）
	if kslAtten(1, 7, 0x3FF) <= kslAtten(2, 7, 0x3FF) {
		t.Error("KSL 位元 1 應該比位元 2 強")
	}
	// 每八度 6 dB（位元 3）
	d := kslAtten(3, 7, 0x3FF) - kslAtten(3, 6, 0x3FF)
	if math.Abs(d-6) > 0.01 {
		t.Errorf("位元 3 每八度應該 6 dB，得 %.2f", d)
	}
}

// `196` §4 第 8 項：震音讓持續音的 RMS 以約 3.7 Hz 起伏，深度位元 1 起伏更大。
func TestTremolo(t *testing.T) {
	swing := func(deep bool) float64 {
		s := New(rate)
		if deep {
			s.Write(0xBD, 0x80)
		}
		pureTone(s, 0x200, 4, 0)
		s.Write(0x23, 0xA1) // 載波：震音 ＋ 持續 ＋ MULT 1
		x := render(s, rate)
		w := rate / 100 // 10 ms 窗
		var lo, hi = math.Inf(1), 0.0
		for i := rate / 4; i+w < len(x); i += w { // 跳過起音
			v := rms(x[i : i+w])
			lo, hi = math.Min(lo, v), math.Max(hi, v)
		}
		return hi / lo
	}
	shallow, deep := swing(false), swing(true)
	if shallow <= 1.02 {
		t.Errorf("淺震音應該看得到起伏，hi/lo=%.3f", shallow)
	}
	if deep <= shallow {
		t.Errorf("深震音應該起伏更大：淺 %.3f 深 %.3f", shallow, deep)
	}
}

// `196` §4 第 9 項：顫音讓頻率週期變動。
//
// ⚠ 不要用「兩段的過零數不同」來驗：±7 cent 是 0.4% 的頻率變動，
// 78 ms 裡約 30 次過零只差 0.12 次，四捨五入之後兩段一模一樣——**測不到不等於沒效果**。
// 改看過零間距的離散程度，對小幅調變才有解析度。
func TestVibrato(t *testing.T) {
	jitter := func(vib bool) float64 {
		s := New(rate)
		pureTone(s, 0x200, 4, 0)
		if vib {
			s.Write(0x23, 0x61) // 載波：顫音 ＋ 持續 ＋ MULT 1
		}
		x := render(s, rate)[rate/4:] // 跳過起音
		// 過零點要用線性內插：±7 cent 只讓一個週期（約 50 個取樣）差 0.2 個取樣，
		// 取整數索引的話整個訊號都被量化雜訊蓋掉（第一版就是這樣量到「沒效果」）。
		var zc []float64
		for i := 1; i < len(x); i++ {
			if x[i-1] <= 0 && x[i] > 0 {
				zc = append(zc, float64(i-1)+x[i-1]/(x[i-1]-x[i]))
			}
		}
		if len(zc) < 20 {
			t.Fatalf("過零數太少（%d）", len(zc))
		}
		gaps := make([]float64, 0, len(zc)-1)
		for i := 1; i < len(zc); i++ {
			gaps = append(gaps, zc[i]-zc[i-1])
		}
		mean := 0.0
		for _, g := range gaps {
			mean += g
		}
		mean /= float64(len(gaps))
		v := 0.0
		for _, g := range gaps {
			v += (g - mean) * (g - mean)
		}
		return math.Sqrt(v/float64(len(gaps))) / mean
	}
	off, on := jitter(false), jitter(true)
	if on <= off*1.5 {
		t.Errorf("顫音應該讓過零間距明顯抖動：關 %.5f 開 %.5f", off, on)
	}
}

// `196` §4 第 10 項：起音是指數，前半段比後半段快（線性的話兩段一樣）。
func TestAttackIsExponential(t *testing.T) {
	s := New(rate)
	pureTone(s, 0x200, 4, 0)
	s.Write(0x63, 0x40) // 載波 AR 4（慢起音）、DR 0
	x := render(s, rate/2)
	env := []float64{}
	w := rate / 500 // 2 ms 窗
	for i := 0; i+w < len(x); i += w {
		env = append(env, rms(x[i:i+w]))
	}
	peak := 0.0
	peakAt := 0
	for i, v := range env {
		if v > peak {
			peak, peakAt = v, i
		}
	}
	if peakAt < 4 {
		t.Fatalf("起音太快，量不到形狀（峰值在第 %d 窗）", peakAt)
	}
	half := env[peakAt/2]
	if half <= peak/2 {
		t.Errorf("指數起音在一半時間應該已經超過一半振幅：half=%.4f peak=%.4f", half, peak)
	}
}

// docs/spec/196：OnlyChannel 只影響「加不加進輸出」，不影響包絡與時間軸。
func TestOnlyChannel(t *testing.T) {
	setup := func(s *Synth) {
		pureTone(s, 0x200, 4, 0) // 聲道 0
		// 聲道 1：相加、載波出聲
		s.Write(0xC1, 0x01)
		s.Write(0x21, 0x21)
		s.Write(0x41, 0x3F)
		s.Write(0x24, 0x21)
		s.Write(0x44, 0x00)
		s.Write(0x64, 0xF0)
		s.Write(0x84, 0x0F)
		s.Write(0xA1, 0x00)
		s.Write(0xB1, 0x20|4<<2|2)
	}
	all := New(rate)
	setup(all)
	xa := render(all, rate/4)

	one := New(rate)
	setup(one)
	one.OnlyChannel(0)
	xb := render(one, rate/4)

	if rms(xb) >= rms(xa) {
		t.Errorf("只留一個聲道應該比較小聲：全部 %.4f 單一 %.4f", rms(xa), rms(xb))
	}
	if rms(xb) == 0 {
		t.Error("只留聲道 0 之後沒有聲音")
	}
	// -1 恢復全部：與從頭全開的結果相同
	back := New(rate)
	setup(back)
	back.OnlyChannel(3)
	back.OnlyChannel(-1)
	xc := render(back, rate/4)
	for i := range xa {
		if xa[i] != xc[i] {
			t.Fatalf("OnlyChannel(-1) 之後應該與全開相同，第 %d 個取樣 %.6f vs %.6f", i, xa[i], xc[i])
		}
	}
	// 超出範圍當 -1
	bad := New(rate)
	setup(bad)
	bad.OnlyChannel(99)
	if rms(render(bad, rate/4)) != rms(xa) {
		t.Error("超出範圍的聲道編號應該當成全開")
	}
}
