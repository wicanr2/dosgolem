package machine

// LEPIT0 是8254通道0模式3的設定子集；尚未推進時鐘或派送IRQ0。
type LEPIT0 struct {
	Configured bool
	Loaded     bool
	Reload     uint32
	Generation uint64
	low        byte
	highNext   bool
}

func (p *LEPIT0) Out8(port uint16, v byte) bool {
	if port == 0x43 {
		if v != 0x36 {
			return false
		}
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
	p.Reload = uint32(p.low) | uint32(v)<<8
	if p.Reload == 0 {
		p.Reload = 65536
	}
	p.highNext = false
	p.Loaded = true
	p.Generation++
	return true
}
