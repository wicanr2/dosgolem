package cpu386

import (
	"bytes"
	"encoding/binary"
	"testing"
)

func TestCSAbsoluteESLoadSelectorsWidthAndPreservation(t *testing.T) {
	for _, prefix := range [][]byte{{0x66, 0x2e}, {0x2e, 0x66}} {
		for _, selector := range []uint16{0, 0x188, 0x190, 1, 2, 3, 0xffff} {
			mem := make(testBus, 256)
			code := append(append([]byte(nil), prefix...), 0x8e, 0x05, 32, 0, 0, 0)
			copy(mem, code)
			binary.LittleEndian.PutUint16(mem[160:], selector)
			mem[162], mem[163] = 0xff, 0xff
			binary.LittleEndian.PutUint16(mem[96:], 0xffff)
			c := New(mem)
			c.R = [8]uint32{1, 2, 3, 4, 5, 6, 7, 8}
			c.Seg = [6]uint16{0x180, 0x188, 0x1a0, 0, 0x20, 0x188}
			c.SetDescriptor(0x180, Descriptor{Base: 128, Limit: 63})
			c.SetDescriptor(0x188, Descriptor{Base: 64, Limit: 63})
			c.SetDescriptor(0x190, Descriptor{Base: 192, Limit: 63})
			c.EFlags = 0x200ed7
			r, s, f := c.R, c.Seg, c.EFlags
			before := append([]byte(nil), mem...)
			valid := selector == 0 || selector == 0x188 || selector == 0x190
			err := c.Step()
			want := s
			if valid {
				want[SegES] = selector
			}
			if (err == nil) != valid || c.R != r || c.Seg != want || c.EFlags != f || !bytes.Equal(mem, before) || (valid && c.EIP != 8) {
				t.Fatalf("CS word ES selector=%X seg=%X flags=%X err=%v", selector, c.Seg, c.EFlags, err)
			}
		}
	}
}

func TestCSAbsoluteESLoadRejectsShapesAndPartialRead(t *testing.T) {
	valid := []byte{0x66, 0x2e, 0x8e, 0x05, 32, 0, 0, 0}
	var cases [][]byte
	for i := 0; i < len(valid); i++ {
		cases = append(cases, append([]byte(nil), valid[:i]...))
	}
	cases = append(cases, []byte{0x2e, 0x8e, 0x05, 32, 0, 0, 0})
	for _, modrm := range []byte{0x0d, 0x15, 0x25, 0x2d, 0x35, 0x3d, 0xc0, 0x00, 0x45, 0x85, 0x04} {
		code := append([]byte(nil), valid...)
		code[3] = modrm
		cases = append(cases, code)
	}
	for _, prefix := range []byte{0x2e, 0xf0, 0xf2, 0xf3, 0x67} {
		cases = append(cases, append([]byte{prefix}, valid...))
	}
	for _, code := range cases {
		mem := testBus(append([]byte(nil), code...))
		c := New(mem)
		c.R = [8]uint32{1, 2, 3, 4, 5, 6, 7, 8}
		c.Seg[SegES] = 0x1a0
		c.EFlags = 0x200ed7
		r, s, f := c.R, c.Seg, c.EFlags
		before := append([]byte(nil), mem...)
		if err := c.Step(); err == nil || c.R != r || c.Seg != s || c.EFlags != f || !bytes.Equal(mem, before) {
			t.Fatalf("未列形狀／截短須拒絕 code=%X err=%v", code, err)
		}
	}
	for available := 0; available < 2; available++ {
		for _, descFailure := range []bool{false, true} {
			mem := make(testBus, 104)
			copy(mem, valid)
			c := New(mem)
			c.Seg[SegCS] = 0x180
			c.Seg[SegES] = 0x1a0
			c.EFlags = 0x200ed7
			limit := uint32(39)
			if descFailure {
				limit = uint32(31 + available)
			} else {
				c.Bus = mem[:96+available]
			}
			c.SetDescriptor(0x180, Descriptor{Base: 64, Limit: limit})
			r, s, f := c.R, c.Seg, c.EFlags
			before := append([]byte(nil), mem...)
			if err := c.Step(); err == nil || c.R != r || c.Seg != s || c.EFlags != f || !bytes.Equal(mem, before) {
				t.Fatalf("完整 word 來源必須可讀 available=%d descriptor=%t err=%v", available, descFailure, err)
			}
		}
	}
}

func TestCSAbsoluteESLoadUsesExistingResolver(t *testing.T) {
	for _, allow := range []bool{false, true} {
		mem := make(testBus, 128)
		copy(mem, []byte{0x66, 0x2e, 0x8e, 0x05, 96, 0, 0, 0})
		binary.LittleEndian.PutUint16(mem[96:], 0x1a0)
		c := New(mem)
		c.Seg[SegCS] = 0x180
		c.Seg[SegES] = 0x188
		c.SetDescriptor(0x180, Descriptor{Limit: 127})
		c.EFlags = 0x246
		calls := 0
		c.SegmentLoadOK = func(selector uint16, destination int) bool {
			calls++
			if selector != 0x1a0 || destination != SegES {
				t.Fatalf("selector resolver 的輸入不符 %X %d", selector, destination)
			}
			return allow
		}
		r, s := c.R, c.Seg
		before := append([]byte(nil), mem...)
		err := c.Step()
		want := s
		if allow {
			want[SegES] = 0x1a0
		}
		if (err == nil) != allow || calls != 1 || c.R != r || c.Seg != want || c.EFlags != 0x246 || !bytes.Equal(mem, before) {
			t.Fatalf("既有 selector resolver allow=%t seg=%X err=%v", allow, c.Seg, err)
		}
	}
}

func TestCSAbsoluteESLoadKeepsOtherModRMRejected(t *testing.T) {
	for m := 0; m < 256; m++ {
		if m == 0x05 || m == 0x1d {
			continue
		}
		mem := make(testBus, 128)
		copy(mem, []byte{0x66, 0x2e, 0x8e, byte(m), 96, 0, 0, 0})
		binary.LittleEndian.PutUint16(mem[96:], 0x188)
		c := New(mem)
		c.Seg[SegCS] = 0x180
		c.Seg[SegES] = 0x190
		c.SetDescriptor(0x180, Descriptor{Limit: 127})
		c.SetDescriptor(0x188, Descriptor{Limit: 127})
		c.EFlags = 0x246
		r, s := c.R, c.Seg
		before := append([]byte(nil), mem...)
		if err := c.Step(); err == nil || c.EIP != 4 || c.R != r || c.Seg != s || c.EFlags != 0x246 || !bytes.Equal(mem, before) {
			t.Fatalf("其餘 CS ModR/M 必須在來源解碼前拒絕 %X EIP=%X err=%v", m, c.EIP, err)
		}
	}
}

func TestCSAbsoluteESLoadOriginalFiniteSample(t *testing.T) {
	mem := make(testBus, 0x3a0000)
	copy(mem[0x378dba:], []byte{0x66, 0x2e, 0x8e, 0x05, 0x06, 0xfa, 0x39, 0})
	copy(mem[0x39fa06:], []byte{0x88, 0x01, 0xff, 0xff})
	c := New(mem)
	c.EIP = 0x378dba
	c.R = [8]uint32{0, 0, 0x174e, 0x174e, 0x6808, 0x6808, 0xc8, 8}
	c.Seg = [6]uint16{0x180, 0x188, 0x188, 0, 0x20, 0xd0}
	c.SetDescriptor(0x180, Descriptor{Limit: uint32(len(mem) - 1)})
	c.SetDescriptor(0x188, Descriptor{Limit: uint32(len(mem) - 1)})
	c.EFlags = 0x46
	r, s := c.R, c.Seg
	before := append([]byte(nil), mem...)
	if err := c.Step(); err != nil || c.EIP != 0x378dc2 || c.R != r || c.Seg != s || c.EFlags != 0x46 || !bytes.Equal(mem, before) {
		t.Fatalf("原版有限 ES 載入樣本不符 EIP=%X seg=%X flags=%X err=%v", c.EIP, c.Seg, c.EFlags, err)
	}
}
