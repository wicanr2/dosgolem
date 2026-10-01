package machine

import (
	"encoding/binary"

	"github.com/wicanr2/dosgolem/internal/cpu386"
)

// moo2VBEVideo 只建模固定 0101h 的視窗 A；規格 265 的硬體規格近似。
type moo2VBEVideo struct {
	ports                                                   *LEOPLPorts
	active                                                  bool
	bank                                                    uint16
	startY                                                  uint16
	base, granularity, window, stride, width, height, pages uint32
	framebuffer                                             []byte
	bankSets, writes                                        uint64
	displaySets                                             uint64
}

func newMOO2VBEVideo(ports *LEOPLPorts) *moo2VBEVideo {
	word := func(off int) uint32 { return uint32(binary.LittleEndian.Uint16(moo2VBEMode0101[off:])) }
	return &moo2VBEVideo{
		ports: ports, base: word(8) * 16, granularity: word(4) * 1024, window: word(6) * 1024,
		stride: word(16), width: word(18), height: word(20), pages: uint32(moo2VBEMode0101[29]) + 1,
		framebuffer: make([]byte, moo2VBETotalMemoryBlocks*64*1024),
	}
}

func (v *moo2VBEVideo) setMode() {
	clear(v.framebuffer[:v.stride*v.height*v.pages])
	v.active, v.bank, v.startY = true, 0, 0
	v.ports.device.dacMask = 0xff
}

func (v *moo2VBEVideo) contains(addr uint32) bool {
	return v != nil && v.active && addr >= v.base && uint64(addr) < uint64(v.base)+uint64(v.window)
}

func (v *moo2VBEVideo) offset(addr uint32) uint32 {
	return uint32(v.bank)*v.granularity + addr - v.base
}

func (v *moo2VBEVideo) control(c *cpu386.CPU) bool {
	if v == nil || !v.active {
		return false
	}
	switch c.R[cpu386.EBX] {
	case 0:
		bank := uint64(c.R[cpu386.EDX])
		if bank > 0xffff || bank*uint64(v.granularity)+uint64(v.window) > uint64(len(v.framebuffer)) {
			return false
		}
		v.bank = uint16(bank)
		v.bankSets++
	case 0x100:
		c.R[cpu386.EDX] = c.R[cpu386.EDX]&0xffff0000 | uint32(v.bank)
	default:
		return false
	}
	c.R[cpu386.EAX] = 0x4f
	return true
}

// displayStart 依規格 266 消費固定垂直起點，不改目前寫入視窗。
func (v *moo2VBEVideo) displayStart(c *cpu386.CPU) bool {
	if v == nil || !v.active {
		return false
	}
	switch c.R[cpu386.EBX] {
	case 0:
		y := uint64(c.R[cpu386.EDX])
		if c.R[cpu386.ECX] != 0 || y > 0xffff || (y+uint64(v.height))*uint64(v.stride) > uint64(len(v.framebuffer)) {
			return false
		}
		v.startY = uint16(y)
		v.displaySets++
	case 1:
		c.R[cpu386.ECX] &= 0xffff0000
		c.R[cpu386.EDX] = c.R[cpu386.EDX]&0xffff0000 | uint32(v.startY)
	default:
		return false
	}
	c.R[cpu386.EAX] = 0x4f
	return true
}

// MOO2VBEState 是診斷快照，不暴露可寫的顯存指標。
type MOO2VBEState struct {
	Active           bool
	Bank             uint16
	StartY           uint16
	BankSets, Writes uint64
	DisplaySets      uint64
}

func (m *LEMachine) VBEState() MOO2VBEState {
	if m.vbeVideo == nil {
		return MOO2VBEState{}
	}
	v := m.vbeVideo
	return MOO2VBEState{Active: v.active, Bank: v.bank, StartY: v.startY, BankSets: v.bankSets, Writes: v.writes, DisplaySets: v.displaySets}
}

// VBEIndexed 回固定模式有效起點的索引快照；不是 RAM 的 A0000 切片。
func (m *LEMachine) VBEIndexed() []byte {
	v := m.vbeVideo
	if v == nil || !v.active {
		return nil
	}
	start := uint32(v.startY) * v.stride
	return append([]byte(nil), v.framebuffer[start:start+v.stride*v.height]...)
}

// VBERGB 沿既有 DAC／像素遮罩查色，未建模回掃或 S3 類比時序。
func (m *LEMachine) VBERGB() []byte {
	indexed := m.VBEIndexed()
	if indexed == nil {
		return nil
	}
	palette := m.vbeVideo.ports.device.Palette()
	rgb := make([]byte, len(indexed)*3)
	for i, index := range indexed {
		color := palette[index]
		copy(rgb[i*3:i*3+3], color[:])
	}
	return rgb
}
