package machine

// 音訊觀測（`docs/spec/240`）：OPL 寫入、喇叭資料線與 PIT 通道 2 的變化，
// 各自帶步數交給觀測器。觀測器掛上時，`m.OPL`／`m.Speaker` 不再追加，
// 長時間執行不會無限增長。callback 不得改變機器狀態。

// PIT2Change 是 PIT 通道 2 一次完成的重載（分頻值或模式變化）。
type PIT2Change struct {
	Step    uint64
	Divisor uint32 // 1..65536（寫 0 代表 65,536）
	Mode    uint8  // 命令位元組 bits 3–1
}

// pit2 解讀通道 2 的命令與 42h 寫入。和通道 0 一樣只解讀、不推進時鐘。
type pit2 struct {
	divisor  uint32
	mode     uint8
	access   uint8
	lo       uint8
	haveLo   bool
	selected bool
}

// out 回報是否完成一次重載。
func (p *pit2) out(port uint16, v uint8) bool {
	switch port {
	case 0x43:
		p.selected = v>>6 == 2
		if !p.selected {
			return false
		}
		if acc := (v >> 4) & 3; acc != 0 { // 00 ＝ 鎖存，不動設定
			p.access, p.haveLo = acc, false
			p.mode = (v >> 1) & 7
		}
		return false
	case 0x42:
		switch p.access {
		case 1:
			p.set(uint32(v))
		case 2:
			p.set(uint32(v) << 8)
		default:
			if !p.haveLo {
				p.lo, p.haveLo = v, true
				return false
			}
			p.haveLo = false
			p.set(uint32(p.lo) | uint32(v)<<8)
		}
		return true
	}
	return false
}

func (p *pit2) set(d uint32) {
	if d == 0 {
		d = PITDefaultDivisor
	}
	p.divisor = d
}

// ObserveOPLWrites 掛上 OPL 寫入觀測器；nil 關閉。
func (m *Machine) ObserveOPLWrites(fn func(OPLWrite)) { m.onOPL = fn }

// ObserveSpeaker 掛上喇叭資料線觀測器；nil 關閉。
func (m *Machine) ObserveSpeaker(fn func(SpeakerSample)) { m.onSpeaker = fn }

// ObservePITChannel2 掛上 PIT 通道 2 觀測器；nil 關閉。
func (m *Machine) ObservePITChannel2(fn func(PIT2Change)) { m.onPIT2 = fn }

// PITChannel2 回通道 2 目前的分頻值與模式；沒設過時分頻值為 0。
func (m *Machine) PITChannel2() (uint32, uint8) { return m.pit2.divisor, m.pit2.mode }

// AdLibPresent 回 AdLib 是否設為存在。
func (m *Machine) AdLibPresent() bool { return m.oplPresent }

func (m *Machine) pit2Write(port uint16, v uint8) {
	if m.pit2.out(port, v) && m.onPIT2 != nil {
		m.onPIT2(PIT2Change{Step: m.Steps, Divisor: m.pit2.divisor, Mode: m.pit2.mode})
	}
}
