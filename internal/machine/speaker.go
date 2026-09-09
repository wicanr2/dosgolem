package machine

// PC 喇叭與 8253 通道 0（`docs/spec/016`）。
//
// 智冠《三國演義》的語音是**一個位元的取樣**：把系統計時器重設成
// 幾千赫，在 IRQ0 的處理常式裡每次把一個位元送到埠 `0x61`。
// 少了通道 0 的分頻值，中斷照舊每 165,000 道指令來一次——
// 一秒的語音要一億八千萬道指令，而且取樣間距全錯。
//
// ⚠ **沒有這一層的時候，症狀不是無聲，是「跑很久然後好像沒事發生」**：
// 程式在等資料指標走到結尾，而指標一秒才動一格。

const (
	// pitInputHz 是 8253 的輸入頻率（1.193182 MHz）。
	pitInputHz = 1193182

	// pitDefaultDivisor 是開機的分頻值：0 代表 65536，也就是 18.2065 Hz。
	pitDefaultDivisor = 65536

	// MinIRQ0Every 是 IRQ0 間隔的下限，單位是指令數。
	//
	// 分頻值可以小到 1（約 1.19 MHz），照比例算會變成每兩道指令一次中斷
	// ——處理常式自己跑不完，機器卡死在中斷裡。**卡死看起來像當掉，
	// 不像設定太快**，所以夾住，而且夾住要記一次（IRQ0Clamped）。
	MinIRQ0Every = 64
)

// SpeakerSample 是喇叭資料線的一次變化。
type SpeakerSample struct {
	// Step 是發生在第幾道指令。
	Step uint64
	// Level 是埠 0x61 的 bit1（喇叭資料線）。
	Level uint8
	// Gate 是 bit0（PIT 通道 2 的閘門）。
	//
	// **嗶聲與語音要分得出來**：嗶聲是「通道 2 產生方波 ＋ 閘門打開」，
	// 語音是「閘門關著、直接切 bit1」。混在一起的話，一段方波會被
	// 當成取樣收進波形。
	Gate uint8
}

// pit 是 8253 通道 0 的狀態。只做分頻值——模式與其他通道對
// 「中斷多久來一次」沒有影響。
type pit struct {
	// latchMode 是控制字的 bit5–4：1 ＝ 只低位、2 ＝ 只高位、3 ＝ 低後高。
	latchMode uint8
	// half 記「低位已經收到了，還在等高位」。
	half bool
	// lo 是先收到的低位。
	lo uint8
	// divisor 是目前的分頻值（0 視為 65536）。
	divisor uint32
}

// outPIT 收埠 0x43（控制字）與 0x40（通道 0 資料）。
// 回報分頻值有沒有變。
func (m *Machine) outPIT(port uint16, v uint8) bool {
	switch port {
	case 0x43:
		// bit7–6 是通道。**只認通道 0**：通道 2 是喇叭的方波產生器，
		// 它不改中斷頻率；通道 1 是 DRAM refresh。
		if v>>6 != 0 {
			return false
		}
		m.pit.latchMode = (v >> 4) & 3
		m.pit.half = false
		// bit5–4 ＝ 00 是「鎖住目前計數值」，不是設定新值。
		return false
	case 0x40:
		switch m.pit.latchMode {
		case 1: // 只低位
			m.pit.divisor = uint32(v)
		case 2: // 只高位
			m.pit.divisor = uint32(v) << 8
		default: // 3（低後高），0 也照這個走
			if !m.pit.half {
				m.pit.lo, m.pit.half = v, true
				return false
			}
			m.pit.divisor = uint32(m.pit.lo) | uint32(v)<<8
			m.pit.half = false
		}
		m.applyPITDivisor()
		return true
	}
	return false
}

// applyPITDivisor 把分頻值換算成 IRQ0Every。
//
// `DefaultIRQ0Every` 對應分頻值 65536（18.2065 Hz），所以比例式是
// 「維持每秒約三百萬道指令的機器速度」——與 DOSBox 預設的
// 3,000 cycles/ms 同一個模型（`docs/spec/016` §3.1）。
func (m *Machine) applyPITDivisor() {
	d := m.pit.divisor
	if d == 0 {
		d = pitDefaultDivisor
	}
	every := DefaultIRQ0Every * uint64(d) / pitDefaultDivisor
	if every < MinIRQ0Every {
		every = MinIRQ0Every
		m.IRQ0Clamped++
	}
	m.IRQ0Every = every
	// 下一次中斷從現在起算。**不重排的話**，分頻值從 65536 改成 100 之後
	// 仍要等原本排定的那一刻——最多空轉十六萬道指令，看起來像「設了沒用」。
	m.nextIRQ0 = m.Steps + every
}

// PITDivisor 是通道 0 目前的分頻值（0 表示還沒被設過，等同 65536）。
func (m *Machine) PITDivisor() uint32 { return m.pit.divisor }

// PITHz 是通道 0 目前的中斷頻率，給人看的。
func (m *Machine) PITHz() float64 {
	d := m.pit.divisor
	if d == 0 {
		d = pitDefaultDivisor
	}
	return float64(pitInputHz) / float64(d)
}

// StepsPerSecond 是這台虛擬機的名目速度：每秒幾道指令。
//
// **這是模型不是量測。** 它定義「指令數怎麼換算成時間」，
// 改 IRQ0Every 不影響它。波形要存成音檔時用得到。
func StepsPerSecond() float64 {
	return float64(DefaultIRQ0Every) * pitInputHz / pitDefaultDivisor
}

// outSpeaker 收埠 0x61 的喇叭兩個位元，只在值改變時記一筆。
func (m *Machine) outSpeaker(v uint8) {
	level, gate := (v>>1)&1, v&1
	if n := len(m.Speaker); n > 0 {
		last := m.Speaker[n-1]
		if last.Level == level && last.Gate == gate {
			return
		}
	} else if level == 0 && gate == 0 {
		// 開頭的靜音不用記——它是重置值，不是動作。
		return
	}
	m.Speaker = append(m.Speaker, SpeakerSample{Step: m.Steps, Level: level, Gate: gate})
}
