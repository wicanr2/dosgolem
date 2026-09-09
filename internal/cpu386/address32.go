package cpu386

import "fmt"

// decodeAddress32只計算位址；存取寬度及權限由指令本身驗證。
func (c *CPU) decodeAddress32(modrm uint8) (int, uint32, error) {
	mod, base := modrm>>6, modrm&7
	if mod == 3 {
		return 0, 0, fmt.Errorf("memory operand不可為register")
	}
	seg := SegDS
	var addr uint32
	noBase := mod == 0 && base == EBP
	if base == ESP {
		sib, err := c.fetch8()
		if err != nil {
			return 0, 0, err
		}
		scale, index := sib>>6, (sib>>3)&7
		base = sib & 7
		noBase = mod == 0 && base == EBP
		if index != ESP {
			addr = c.R[index] << scale
		}
	}
	if !noBase {
		addr += c.R[base]
		if base == ESP || base == EBP {
			seg = SegSS
		}
	}
	if mod == 1 {
		d, err := c.fetch8()
		if err != nil {
			return 0, 0, err
		}
		addr += uint32(int32(int8(d)))
	} else if mod == 2 || noBase {
		d, err := c.fetch32()
		if err != nil {
			return 0, 0, err
		}
		addr += d
	}
	return seg, addr, nil
}
