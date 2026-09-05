package machine

import "github.com/wicanr2/dosgolem/internal/cpu386"

// FD2StartupDOS 是固定雜湊 FD2.EXE 在 DOS/4GW 已載入後所需的兩個啟動服務。
// 它不是一般 DOS 或 DOS/4GW 模擬器；未列呼叫與錯誤順序一律拒絕。
type FD2StartupDOS struct {
	calls int
}

func (s *FD2StartupDOS) Calls() int { return s.calls }

func (s *FD2StartupDOS) Handle(c *cpu386.CPU, number uint8) bool {
	if number != 0x21 {
		return false
	}
	switch s.calls {
	case 0:
		if uint8(c.R[cpu386.EAX]>>8) != 0x30 || c.R[cpu386.EBX] != 0x50484152 {
			return false
		}
		c.Seg[cpu386.SegDS] = 0x0160
		c.Seg[cpu386.SegES] = 0x0028
		c.Seg[cpu386.SegGS] = 0x0020
		c.Seg[cpu386.SegSS] = 0x0160
		c.SegmentRead16 = func(selector uint16, offset uint32) (uint16, bool) {
			if selector == 0x0028 && offset == 0x002c {
				return 0x0030, true
			}
			return 0, false
		}
		c.R[cpu386.EAX] = c.R[cpu386.EAX]&0xffff0000 | 0x1606
	case 1:
		if uint16(c.R[cpu386.EAX]) != 0xff00 || uint16(c.R[cpu386.EDX]) != 0x0078 {
			return false
		}
		c.R[cpu386.EAX] = 0x4734ffff
		c.Seg[cpu386.SegGS] = 0x0020
	default:
		return false
	}
	s.calls++
	return true
}
