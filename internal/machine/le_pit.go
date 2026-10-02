package machine

// LEPIT0 是8254 通道 0 模式 2／3 的設定子集；週期及模式 2 鎖存由共享 BIOS 時鐘推進。
type LEPIT0 struct {
	Mode         uint8
	Configured   bool
	Loaded       bool
	Reload       uint32
	Generation   uint64
	low          byte
	highNext     bool
	readCount    uint16
	readPending  bool
	readHighNext bool
}

func (p *LEPIT0) Out8(port uint16, v byte) bool {
	if port == 0x43 {
		if v != 0x34 && v != 0x36 {
			return false
		}
		p.Mode = (v >> 1) & 7
		p.Configured = true
		p.Loaded = false
		p.highNext = false
		p.readCount, p.readPending, p.readHighNext = 0, false, false
		return true
	}
	if port != 0x40 || !p.Configured {
		return false
	}
	if !p.highNext {
		p.low = v
		p.highNext = true
		return true
	}
	reload := uint32(p.low) | uint32(v)<<8
	if reload == 1 {
		return false
	}
	if reload == 0 {
		reload = 65536
	}
	p.Reload = reload
	p.highNext = false
	p.Loaded = true
	p.Generation++
	return true
}

// 規格 282：只鎖存已載入模式 2；共用既有虛擬時計，不建立硬體邊緣模型。
func (p *LEPIT0) latchMode2(b *LEBIOSClock) bool {
	if b == nil || !p.Configured || !p.Loaded || p.Mode != 2 || p.Reload < 2 || p.Reload > 65536 {
		return false
	}
	if p.readPending {
		return true
	}
	phase := uint64(0)
	if b.generation == p.Generation {
		phase = b.credit
	}
	const unit uint64 = 264 * 1000000
	if phase >= uint64(p.Reload)*unit {
		return false
	}
	p.readCount = uint16(uint64(p.Reload) - phase/unit)
	p.readPending, p.readHighNext = true, false
	return true
}

func (p *LEPIT0) readLatchedMode2() (byte, bool) {
	if !p.readPending {
		return 0, false
	}
	if !p.readHighNext {
		p.readHighNext = true
		return byte(p.readCount), true
	}
	value := byte(p.readCount >> 8)
	p.readCount, p.readPending, p.readHighNext = 0, false, false
	return value, true
}
