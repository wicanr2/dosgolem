package machine

// LEPIT0 是8254 通道 0 模式 2／3 的設定子集；週期由共享 BIOS 時鐘推進。
type LEPIT0 struct {
	Mode       uint8
	Configured bool
	Loaded     bool
	Reload     uint32
	Generation uint64
	low        byte
	highNext   bool
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
