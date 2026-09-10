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
