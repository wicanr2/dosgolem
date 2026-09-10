package machine

// DMA8237 僅實作第一控制器的程式設定介面；不冒充已執行資料傳輸。
type DMA8237 struct {
	Page      [4]uint8
	PageKnown [4]bool
	Base      [8]uint16
	Current   [8]uint16
	Known     [8]uint8
	Mode      [4]uint8
	Mask      uint8
	high      bool
}

func NewDMA8237() *DMA8237 { return &DMA8237{Mask: 15} }
func (d *DMA8237) In8(port uint16) (uint8, bool) {
	if ch, ok := dmaPageChannel(port); ok {
		return d.Page[ch], d.PageKnown[ch]
	}
	if port > 7 {
		return 0, false
	}
	shift := uint(0)
	bit := uint8(1)
	if d.high {
		shift = 8
		bit = 2
	}
	if d.Known[port]&bit == 0 {
		return 0, false
	}
	v := uint8(d.Current[port] >> shift)
	d.high = !d.high
	return v, true
}
func (d *DMA8237) Out8(port uint16, v uint8) bool {
	if ch, ok := dmaPageChannel(port); ok {
		d.Page[ch] = v
		d.PageKnown[ch] = true
		return true
	}
	if port <= 7 {
		shift := uint(0)
		bit := uint8(1)
		if d.high {
			shift = 8
			bit = 2
		}
		d.Current[port] = (d.Current[port] &^ (uint16(255) << shift)) | uint16(v)<<shift
		d.Base[port] = d.Current[port]
		d.Known[port] |= bit
		d.high = !d.high
		return true
	}
	switch port {
	case 0xa:
		bit := uint8(1) << (v & 3)
		if v&4 != 0 {
			d.Mask |= bit
		} else {
			d.Mask &^= bit
		}
		return true
	case 0xb:
		d.Mode[v&3] = v & 0xfc
		return true
	case 0xc:
		d.high = false
		return true
	case 0xd:
		d.high = false
		d.Mask = 15
		return true
	case 0xe:
		d.Mask = 0
		return true
	case 0xf:
		d.Mask = v & 15
		return true
	}
	return false
}

func dmaPageChannel(port uint16) (int, bool) {
	switch port {
	case 0x87:
		return 0, true
	case 0x83:
		return 1, true
	case 0x81:
		return 2, true
	case 0x82:
		return 3, true
	}
	return 0, false
}
