// Package opl2 是 YM3812（OPL2）的近似合成器（`docs/spec/196`）。
//
// 音高、節奏與音色都要接近真機：ADSR、TL、KSL、震音、顫音、波形 0–3、FM 與相加、回授。
// 波形以連續時間近似，不是逐週期複刻真機的定點運算。節奏模式沒做，用到時計數
// （Unsupported），不安靜忽略。
//
// 這個套件不認識機器：呼叫端依時間順序 Write，每產生一個取樣呼叫一次 Sample。
package opl2

import "math"

// NativeRate 是 YM3812 的內部取樣率（3.579545 MHz ÷ 72）。頻率式以它為基準。
const NativeRate = 49716.0

var multTable = [16]float64{0.5, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 10, 12, 12, 15, 15}

// 聲道 c 的調變器**槽位編號**（ops 的索引）；載波 ＝ 調變器 ＋ 3。
// ⚠ 不是暫存器位移：暫存器位移是 0,1,2,8,9,10,16,17,18（中間跳過 6、7、14、15），
// 經 slotOf 換算後才是連續的槽位 0–17。拿位移當索引會在聲道 6–8 越界。
var modSlot = [9]int{0, 1, 2, 6, 7, 8, 12, 13, 14}

// 暫存器位移 → 槽位（20h–35h 這一類）；-1 ＝ 不是運算子。
var slotOf = [22]int{0, 1, 2, 3, 4, 5, -1, -1, 6, 7, 8, 9, 10, 11, -1, -1, 12, 13, 14, 15, 16, 17}

type stage int

const (
	stOff stage = iota
	stAttack
	stDecay
	stSustain
	stRelease
)

type operator struct {
	sustainHold bool // 20h bit 5：EG 類型
	ksr         bool
	am          bool // 20h bit 7：震音
	vib         bool // 20h bit 6：顫音
	mult        uint8
	tl          uint8
	ksl         uint8 // 40h bit 7–6
	ar, dr      uint8
	sl, rr      uint8
	wave        uint8

	phase float64 // 0..1
	env   float64 // 衰減 dB，0 ＝ 最大、96 ＝ 靜音
	st    stage
}

type channel struct {
	fnum     uint16
	block    uint8
	keyOn    bool
	fb       uint8
	additive bool
	fbHist   [2]float64
}

// LFO 常數（YM3812 規格，`196` §3.2）：震音 3.7 Hz、顫音 6.4 Hz，都是全晶片共用的三角波。
const (
	amHz, vibHz   = 3.7, 6.4
	amShallowDB   = 1.0 // BDh bit 7 ＝ 0
	amDeepDB      = 4.8 // BDh bit 7 ＝ 1
	vibShallowCnt = 7.0 // BDh bit 6 ＝ 0，單位 cent
	vibDeepCnt    = 14.0
)

// Feature 是可以個別關掉的合成功能（診斷用）。預設全開；關掉是為了回答
// 「保真度的哪一項是被哪個功能推動的」——一次只改一個變因才看得出來。
type Feature uint8

const (
	FeatKSL Feature = 1 << iota
	FeatAM
	FeatVIB
	FeatExpAttack
)

// FeatureByName 把旗標名對到位元（呼叫端解析命令列用）。
var FeatureByName = map[string]Feature{"ksl": FeatKSL, "am": FeatAM, "vib": FeatVIB, "expatk": FeatExpAttack}

// Synth 是一個 OPL2 晶片的狀態。
type Synth struct {
	rate       float64
	off        Feature // 關掉的功能
	waveSelect bool
	amDeep     bool // BDh bit 7
	vibDeep    bool // BDh bit 6
	lfo        float64
	ops        [18]operator
	ch         [9]channel

	// Unsupported 記錄用到但沒實作的功能各幾次：rhythm、vibrato、tremolo、ksl。
	Unsupported map[string]int
}

// New 建一個輸出取樣率為 rate 的合成器。
func New(rate int) *Synth {
	s := &Synth{rate: float64(rate), Unsupported: map[string]int{}}
	for i := range s.ops {
		s.ops[i].env = 96
	}
	return s
}

// Write 寫一個暫存器。
func (s *Synth) Write(reg, val uint8) {
	switch {
	case reg == 0x01:
		s.waveSelect = val&0x20 != 0
	case reg == 0xBD:
		if val&0x20 != 0 {
			s.Unsupported["rhythm"]++
		}
		s.amDeep = val&0x80 != 0
		s.vibDeep = val&0x40 != 0
	case reg >= 0x20 && reg <= 0x35:
		if o := s.op(reg - 0x20); o != nil {
			o.sustainHold = val&0x20 != 0
			o.ksr = val&0x10 != 0
			o.mult = val & 0x0F
			o.am = val&0x80 != 0
			o.vib = val&0x40 != 0
		}
	case reg >= 0x40 && reg <= 0x55:
		if o := s.op(reg - 0x40); o != nil {
			o.tl = val & 0x3F
			o.ksl = val >> 6
		}
	case reg >= 0x60 && reg <= 0x75:
		if o := s.op(reg - 0x60); o != nil {
			o.ar, o.dr = val>>4, val&0x0F
		}
	case reg >= 0x80 && reg <= 0x95:
		if o := s.op(reg - 0x80); o != nil {
			o.sl, o.rr = val>>4, val&0x0F
		}
	case reg >= 0xA0 && reg <= 0xA8:
		c := &s.ch[reg-0xA0]
		c.fnum = c.fnum&0x300 | uint16(val)
	case reg >= 0xB0 && reg <= 0xB8:
		i := int(reg - 0xB0)
		c := &s.ch[i]
		c.fnum = c.fnum&0xFF | uint16(val&0x03)<<8
		c.block = val >> 2 & 0x07
		on := val&0x20 != 0
		if on && !c.keyOn {
			for _, k := range []int{modSlot[i], modSlot[i] + 3} {
				s.ops[k].st = stAttack
			}
		} else if !on && c.keyOn {
			for _, k := range []int{modSlot[i], modSlot[i] + 3} {
				if s.ops[k].st != stOff {
					s.ops[k].st = stRelease
				}
			}
		}
		c.keyOn = on
	case reg >= 0xC0 && reg <= 0xC8:
		c := &s.ch[reg-0xC0]
		c.fb = val >> 1 & 0x07
		c.additive = val&0x01 != 0
	case reg >= 0xE0 && reg <= 0xF5:
		if o := s.op(reg - 0xE0); o != nil {
			o.wave = val & 0x03
		}
	}
}

func (s *Synth) op(off uint8) *operator {
	if int(off) >= len(slotOf) || slotOf[off] < 0 {
		return nil
	}
	return &s.ops[slotOf[off]]
}

// rateTime 回「有效速率」對應的時間倍率：2^-(有效速率 − 1)。R ＝ 0 回 0（不動）。
func rateScale(r uint8, c *channel, ksr bool) float64 {
	if r == 0 {
		return 0
	}
	off := int(c.block)*2 + int(c.fnum>>9&1)
	if !ksr {
		off >>= 2
	}
	eff := float64(r) + float64(off)/4
	return math.Pow(2, -(eff - 1))
}

// kslScale 是 KSL 位元對應的強度比例。**位元 1 比位元 2 強**（3 dB/oct vs 1.5），
// 這是 OPL 的慣例不是筆誤（`196` §3.2）。
var kslScale = [4]float64{0, 0.5, 0.25, 1}

// kslAtten 回 KSL 造成的衰減（dB）：每八度 6 dB × 強度比例。
//
// 八度數以 FNum 的高 4 位（hi）與 Block 合起來算：`Block − 4 ＋ log2(hi)`。
// ⚠ hi ＝ 0 是特例，**不衰減**——真機的分段表在那一格填一個大到永遠被截成 0 的數，
// 用 log2(hi＋1) 去「避開 log2(0)」會在 hi ＝ 0 時多扣 18 dB（第一版就是這樣錯的）。
// 真機用 16 段整數表近似同一條對數曲線，兩者差 ≤ 1.5 dB（`196` §4 第 7 項）。
func kslAtten(ksl uint8, block uint8, fnum uint16) float64 {
	hi := fnum >> 6
	if ksl == 0 || hi == 0 {
		return 0
	}
	oct := float64(block) - 4 + math.Log2(float64(hi))
	if oct <= 0 {
		return 0
	}
	return kslScale[ksl] * 6 * oct
}

// stepEnv 推進一個取樣的包絡（`196` §3.2）。
func (s *Synth) stepEnv(o *operator, c *channel) {
	dt := 1000 / s.rate // 一個取樣幾 ms
	switch o.st {
	case stAttack:
		// 指數逼近 0：真機每步減去目前值的一個比例，起音是先快後慢的曲線（`196` §3.2）。
		if o.ar == 15 {
			o.env = 0
		} else if k := rateScale(o.ar, c, o.ksr); k > 0 {
			if s.on(FeatExpAttack) {
				tau := 2826 * k / 4 // 走完 4 個時間常數 ≈ 98%
				o.env *= math.Exp(-dt / tau)
			} else {
				o.env -= 96 * dt / (2826 * k) // 舊的 dB 線性起音（診斷用）
			}
		}
		if o.env <= 0.05 {
			o.env = 0
			o.st = stDecay
		}
	case stDecay:
		target := float64(o.sl) * 3
		if o.sl == 15 {
			target = 93
		}
		if k := rateScale(o.dr, c, o.ksr); k > 0 {
			o.env += 96 * dt / (39281 * k)
		}
		if o.env >= target {
			o.env = target
			o.st = stSustain
		}
	case stSustain:
		if !o.sustainHold {
			if k := rateScale(o.rr, c, o.ksr); k > 0 {
				o.env += 96 * dt / (39281 * k)
			}
		}
	case stRelease:
		if k := rateScale(o.rr, c, o.ksr); k > 0 {
			o.env += 96 * dt / (39281 * k)
		}
	}
	if o.env >= 96 {
		o.env = 96
		if o.st == stRelease {
			o.st = stOff
		}
	}
}

func (s *Synth) wave(o *operator, ph float64) float64 {
	x := math.Sin(2 * math.Pi * ph)
	if !s.waveSelect {
		return x
	}
	frac := ph - math.Floor(ph)
	switch o.wave {
	case 1:
		if x < 0 {
			return 0
		}
	case 2:
		return math.Abs(x)
	case 3:
		if math.Mod(frac, 0.5) >= 0.25 {
			return 0
		}
		return math.Abs(x)
	}
	return x
}

// tri 回 0..1 的三角波（相位 0..1）。
func tri(ph float64) float64 {
	ph -= math.Floor(ph)
	if ph < 0.5 {
		return ph * 2
	}
	return 2 - ph*2
}

// output 算一個運算子這個取樣的輸出並推進相位。mod 是加到相位上的弧度。
// c 用來算 KSL（要 Block 與 FNum）。
func (s *Synth) output(o *operator, c *channel, freq, mod float64) float64 {
	if o.st == stOff {
		return 0
	}
	atten := o.env + float64(o.tl)*0.75
	if s.on(FeatKSL) {
		atten += kslAtten(o.ksl, c.block, c.fnum)
	}
	if o.am && s.on(FeatAM) {
		d := amShallowDB
		if s.amDeep {
			d = amDeepDB
		}
		atten += d * tri(s.lfo*amHz)
	}
	if o.vib && s.on(FeatVIB) {
		cents := vibShallowCnt
		if s.vibDeep {
			cents = vibDeepCnt
		}
		freq *= math.Pow(2, cents*(2*tri(s.lfo*vibHz)-1)/1200)
	}
	amp := math.Pow(10, -atten/20)
	y := s.wave(o, o.phase+mod/(2*math.Pi)) * amp
	o.phase += freq * multTable[o.mult] / s.rate
	o.phase -= math.Floor(o.phase)
	return y
}

// Sample 產生下一個取樣（-1..1）。
func (s *Synth) Sample() float64 {
	sum := 0.0
	for i := range s.ch {
		c := &s.ch[i]
		m, cr := &s.ops[modSlot[i]], &s.ops[modSlot[i]+3]
		s.stepEnv(m, c)
		s.stepEnv(cr, c)
		if m.st == stOff && cr.st == stOff {
			continue
		}
		f := float64(c.fnum) * NativeRate / math.Pow(2, float64(20-int(c.block)))
		fb := 0.0
		if c.fb > 0 {
			fb = (c.fbHist[0] + c.fbHist[1]) / 2 * math.Pi * math.Pow(2, float64(c.fb)-4)
		}
		mo := s.output(m, c, f, fb)
		c.fbHist[1], c.fbHist[0] = c.fbHist[0], mo
		if c.additive {
			sum += mo + s.output(cr, c, f, 0)
		} else {
			sum += s.output(cr, c, f, mo*4*math.Pi)
		}
	}
	s.lfo += 1 / s.rate // 全晶片共用的 LFO 相位（秒）
	v := sum * 0.12
	return math.Max(-1, math.Min(1, v))
}

// Disable 關掉指定功能（診斷用，見 Feature）。
func (s *Synth) Disable(f Feature) { s.off |= f }

func (s *Synth) on(f Feature) bool { return s.off&f == 0 }
