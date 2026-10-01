package cpu386

import (
	"bytes"
	"encoding/binary"
	"math/bits"
	"testing"
)

// 以寬位有號差、借位及同位計數建立 16 位預期，與核心公式分開。
func csCMPWordFlags(a, b uint16, controls uint32) uint32 {
	r := a - b
	f := controls &^ (CF | PF | AF | ZF | SF | OF)
	if a < b {
		f |= CF
	}
	if a&15 < b&15 {
		f |= AF
	}
	d := int32(int16(a)) - int32(int16(b))
	if d < -32768 || d > 32767 {
		f |= OF
	}
	if r == 0 {
		f |= ZF
	}
	if int16(r) < 0 {
		f |= SF
	}
	if bits.OnesCount8(byte(r))%2 == 0 {
		f |= PF
	}
	return f
}

func TestCSMemoryCMPImm8WidthsSignsAndFlags(t *testing.T) {
	values := []uint32{0, 1, 15, 16, 127, 128, 0x7fff, 0x8000, 0xffff, 0x7fffffff, 0x80000000, 0xffffff7f, 0xffffffff}
	for _, word := range []bool{false, true} {
		for _, a := range values {
			for imm := 0; imm < 256; imm++ {
				mem := make(testBus, 224)
				code := []byte{0x2e, 0x83, 0x3d, 32, 0, 0, 0, byte(imm)}
				if word {
					code = append([]byte{0x66}, code...)
				}
				copy(mem, code)
				binary.LittleEndian.PutUint32(mem[96:], a)
				c := New(mem)
				c.R = [8]uint32{1, 2, 3, 4, 5, 6, 7, 8}
				c.Seg = [6]uint16{0x180, 0x188, 2, 3, 4, 0x190}
				c.SetDescriptor(0x180, Descriptor{Base: 64, Limit: 159})
				c.SetDescriptor(0x188, Descriptor{Base: 128, Limit: 95})
				c.SetDescriptor(0x190, Descriptor{Base: 160, Limit: 63})
				c.EFlags = 2 | IF | DF | 0x200000 | CF | PF | AF | ZF | SF | OF
				r, s := c.R, c.Seg
				before := append([]byte(nil), mem...)
				want := cmpMemoryExpectedFlags(a, uint32(int32(int8(imm))), c.EFlags)
				if word {
					want = csCMPWordFlags(uint16(a), uint16(int16(int8(imm))), c.EFlags)
				}
				if err := c.Step(); err != nil || c.EIP != uint32(len(code)) || c.R != r || c.Seg != s || c.EFlags != want || !bytes.Equal(mem, before) {
					t.Fatalf("寬度 word=%t a=%X imm=%X flags=%X want=%X err=%v", word, a, imm, c.EFlags, want, err)
				}
			}
		}
	}
}

func TestCSMemoryCMPImm8AllModRMAndSIB(t *testing.T) {
	// 全部 32 位記憶體 ModR/M，SIB 的縮放／索引／基底獨立計算偏移。
	for _, word := range []bool{false, true} {
		for mod := byte(0); mod < 3; mod++ {
			for rm := byte(0); rm < 8; rm++ {
				n := 1
				if rm == 4 {
					n = 256
				}
				for si := 0; si < n; si++ {
					regs := [8]uint32{40, 44, 48, 52, 56, 60, 64, 68}
					code := []byte{0x2e, 0x83, mod<<6 | 0x38 | rm}
					offset := uint32(0)
					absolute := mod == 0 && rm == 5
					if rm == 4 {
						sib := byte(si)
						code = append(code, sib)
						base, index := sib&7, (sib>>3)&7
						absolute = mod == 0 && base == 5
						if !absolute {
							offset += regs[base]
						}
						if index != 4 {
							offset += regs[index] << (sib >> 6)
						}
					} else if !absolute {
						offset = regs[rm]
					}
					if absolute {
						code = append(code, 32, 0, 0, 0)
						offset += 32
					} else if mod == 1 {
						code = append(code, 0xf8)
						offset -= 8
					} else if mod == 2 {
						code = append(code, 16, 0, 0, 0)
						offset += 16
					}
					code = append(code, 0xff)
					if word {
						code = append([]byte{0x66}, code...)
					}
					mem := make(testBus, 1024)
					copy(mem, code)
					binary.LittleEndian.PutUint32(mem[128+offset:], 0x8000)
					c := New(mem)
					c.R = regs
					c.Seg[SegCS] = 0x180
					c.Seg[SegDS] = 0x188
					c.Seg[SegSS] = 0x190
					c.SetDescriptor(0x180, Descriptor{Base: 128, Limit: 767})
					c.SetDescriptor(0x188, Descriptor{Base: 800, Limit: 15})
					c.SetDescriptor(0x190, Descriptor{Base: 900, Limit: 15})
					c.EFlags = 2 | IF | DF
					r, s, f := c.R, c.Seg, c.EFlags
					before := append([]byte(nil), mem...)
					want := cmpMemoryExpectedFlags(0x8000, 0xffffffff, f)
					if word {
						want = csCMPWordFlags(0x8000, 0xffff, f)
					}
					if err := c.Step(); err != nil || c.EIP != uint32(len(code)) || c.R != r || c.Seg != s || c.EFlags != want || !bytes.Equal(mem, before) {
						t.Fatalf("位址 mod=%d rm=%d SIB=%X word=%t offset=%X err=%v", mod, rm, si, word, offset, err)
					}
				}
			}
		}
	}
}

func TestCSMemoryCMPImm8RejectsOtherShapesAndTruncation(t *testing.T) {
	valid := []byte{0x2e, 0x83, 0x3d, 32, 0, 0, 0, 0}
	var cases [][]byte
	for i := 0; i < len(valid); i++ {
		cases = append(cases, append([]byte(nil), valid[:i]...))
	}
	for group := byte(0); group < 8; group++ {
		if group != 7 {
			code := append([]byte(nil), valid...)
			code[2] = group<<3 | 5
			cases = append(cases, code)
		}
		cases = append(cases, []byte{0x2e, 0x83, 0xc0 | group<<3, 0})
	}
	for _, prefix := range []byte{0x26, 0x36, 0x3e, 0x64, 0x65} {
		code := append([]byte(nil), valid...)
		code[0] = prefix
		cases = append(cases, code)
	}
	for _, prefix := range []byte{0x2e, 0xf0, 0xf2, 0xf3, 0x67} {
		cases = append(cases, append([]byte{prefix}, valid...))
	}
	for _, code := range cases {
		mem := testBus(append([]byte(nil), code...))
		c := New(mem)
		c.R = [8]uint32{1, 2, 3, 4, 5, 6, 7, 8}
		c.EFlags = 0x200ed7
		r, s, f := c.R, c.Seg, c.EFlags
		before := append([]byte(nil), mem...)
		if err := c.Step(); err == nil || c.R != r || c.Seg != s || c.EFlags != f || !bytes.Equal(mem, before) {
			t.Fatalf("非法形狀必須拒絕且保持資料 code=%X flags=%X err=%v", code, c.EFlags, err)
		}
	}
	for _, width := range []int{2, 4} {
		for available := 0; available < width; available++ {
			for _, descFailure := range []bool{false, true} {
				mem := make(testBus, 104)
				code := valid
				if width == 2 {
					code = append([]byte{0x66}, valid...)
				}
				copy(mem, code)
				c := New(mem)
				c.Seg[SegCS] = 0x180
				limit := uint32(39)
				if descFailure {
					limit = uint32(31 + available)
				} else {
					c.Bus = mem[:96+available]
				}
				c.SetDescriptor(0x180, Descriptor{Base: 64, Limit: limit})
				c.EFlags = 0x200ed7
				r, s, f := c.R, c.Seg, c.EFlags
				before := append([]byte(nil), mem...)
				if err := c.Step(); err == nil || c.R != r || c.Seg != s || c.EFlags != f || !bytes.Equal(mem, before) {
					t.Fatalf("完整來源讀取失敗 width=%d available=%d descriptor=%t err=%v", width, available, descFailure, err)
				}
			}
		}
	}
}

func TestCSMemoryCMPImm8OriginalFiniteSample(t *testing.T) {
	mem := make(testBus, 0x3a0000)
	copy(mem[0x378d9a:], []byte{0x2e, 0x83, 0x3d, 0xfe, 0xf9, 0x39, 0, 0})
	c := New(mem)
	c.EIP = 0x378d9a
	c.R = [8]uint32{0, 0, 0x174e, 0x174e, 0x6838, 8, 0xc8, 8}
	c.Seg = [6]uint16{0x180, 0x188, 0x188, 0, 0x20, 0xd0}
	c.SetDescriptor(0x180, Descriptor{Limit: uint32(len(mem) - 1)})
	c.EFlags = 0x46
	r, s := c.R, c.Seg
	before := append([]byte(nil), mem...)
	if err := c.Step(); err != nil || c.EIP != 0x378da2 || c.EFlags != 0x46 || c.R != r || c.Seg != s || !bytes.Equal(mem, before) {
		t.Fatalf("原版有限 CMP 樣本不符 EIP=%X flags=%X err=%v", c.EIP, c.EFlags, err)
	}
}
