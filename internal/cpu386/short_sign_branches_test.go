package cpu386

import (
	"bytes"
	"fmt"
	"maps"
	"testing"
)

type shortSignBus struct {
	data   map[uint32]byte
	reads  []uint32
	writes int
}

func (b *shortSignBus) Read8(address uint32) (uint8, error) {
	b.reads = append(b.reads, address)
	value, ok := b.data[address]
	if !ok {
		return 0, fmt.Errorf("受控取指越界")
	}
	return value, nil
}
func (b *shortSignBus) Write8(address uint32, value uint8) error {
	b.writes++
	return fmt.Errorf("分支不應寫入記憶體")
}

func shortSignExpectedEIP(start uint32, displacement byte, taken bool) uint32 {
	result := int64(uint64(start)) + 2
	if taken {
		offset := int64(displacement)
		if displacement >= 128 {
			offset -= 256
		}
		result += offset
	}
	return uint32(result)
}

func TestShortSignBranchesAllFlagsAndDisplacements(t *testing.T) {
	flagBits := []uint32{CF, PF, AF, ZF, SF, OF}
	for _, op := range []byte{0x78, 0x79} {
		for mask := 0; mask < 64; mask++ {
			for displacement := 0; displacement < 256; displacement++ {
				bus := &shortSignBus{data: map[uint32]byte{0: op, 1: byte(displacement)}}
				c := New(bus)
				c.R = [8]uint32{0x11110000, 0x22220000, 0x33330000, 0x44440000, 0x55550000, 0x66660000, 0x77770000, 0x88880000}
				c.Seg = [6]uint16{1, 2, 3, 4, 5, 6}
				c.EFlags = 2 | IF | DF | 0x200000
				for index, flag := range flagBits {
					if mask&(1<<index) != 0 {
						c.EFlags |= flag
					}
				}
				regs, segs, flags := c.R, c.Seg, c.EFlags
				expected := maps.Clone(bus.data)
				taken := mask&16 != 0
				if op == 0x79 {
					taken = !taken
				}
				want := shortSignExpectedEIP(0, byte(displacement), taken)
				if err := c.Step(); err != nil || c.EIP != want || c.R != regs || c.Seg != segs || c.EFlags != flags || bus.writes != 0 || !maps.Equal(bus.data, expected) {
					t.Fatalf("短符號分支 op=%X flags=%X 位移=%X EIP=%X want=%X err=%v", op, flags, displacement, c.EIP, want, err)
				}
				if len(bus.reads) != 2 || bus.reads[0] != 0 || bus.reads[1] != 1 {
					t.Fatalf("短分支取指寬度=%v", bus.reads)
				}
			}
		}
	}
}

func TestShortSignBranchesEIPWrap(t *testing.T) {
	for _, start := range []uint32{0, 1, 0xffffff7f, 0xfffffffd, 0xfffffffe, 0xffffffff} {
		for _, op := range []byte{0x78, 0x79} {
			for _, sign := range []uint32{0, SF} {
				for delta := 0; delta < 256; delta++ {
					bus := &shortSignBus{data: map[uint32]byte{start: op, start + 1: byte(delta)}}
					c := New(bus)
					c.EIP = start
					c.EFlags = 2 | IF | sign | OF | ZF | CF
					flags := c.EFlags
					taken := sign != 0
					if op == 0x79 {
						taken = !taken
					}
					want := shortSignExpectedEIP(start, byte(delta), taken)
					if err := c.Step(); err != nil || c.EIP != want || c.EFlags != flags || bus.writes != 0 || len(bus.reads) != 2 {
						t.Fatalf("短符號分支環繞 start=%X op=%X delta=%X EIP=%X want=%X err=%v", start, op, delta, c.EIP, want, err)
					}
				}
			}
		}
	}
}

func TestShortSignBranchesTruncatedAndRejectedPrefixes(t *testing.T) {
	for _, op := range []byte{0x78, 0x79} {
		for _, sign := range []uint32{0, SF} {
			mem := testBus{op}
			c := New(mem)
			c.R = [8]uint32{1, 2, 3, 4, 5, 6, 7, 8}
			c.EFlags = 2 | IF | OF | sign
			regs, flags := c.R, c.EFlags
			if err := c.Step(); err == nil || c.EIP != 1 || c.R != regs || c.EFlags != flags || mem[0] != op {
				t.Fatalf("取／不取分支都必讀位移 op=%X flags=%X err=%v", op, flags, err)
			}
		}
		for _, prefix := range []byte{0x66, 0x26, 0x2e, 0x36, 0x3e, 0x64, 0x65, 0xf2, 0xf3, 0x67, 0xf0} {
			mem := testBus{prefix, op, 6}
			c := New(mem)
			c.R = [8]uint32{1, 2, 3, 4, 5, 6, 7, 8}
			c.Seg = [6]uint16{1, 2, 3, 4, 5, 6}
			c.EFlags = 2 | IF | SF | OF
			regs, segs, flags := c.R, c.Seg, c.EFlags
			expected := append([]byte(nil), mem...)
			if err := c.Step(); err == nil || c.R != regs || c.Seg != segs || c.EFlags != flags || !bytes.Equal(mem, expected) {
				t.Fatalf("未審查前綴須拒絕 code=%X err=%v", mem, err)
			}
		}
	}
}

func TestShortSignBranchesOriginalJSAndJNS(t *testing.T) {
	// 保留原版 JS／SUB／JNS 的完整順序，只重定位代碼位置。
	mem := testBus{0x78, 6, 0x2b, 0xc2, 0x79, 2, 0xeb, 0xec, 0x5e, 0x61, 0xc3}
	c := New(mem)
	c.R = [8]uint32{1, 0, 1, 0xe, 0x3ebb9c, 0x3ebc06, 0x470, 0x3a2090}
	c.Seg = [6]uint16{0x180, 0x188, 0x188, 0, 0x20, 0x188}
	c.EFlags = 0x202
	regs, segs := c.R, c.Seg
	expected := append([]byte(nil), mem...)
	if err := c.Step(); err != nil || c.EIP != 2 || c.R != regs || c.Seg != segs || c.EFlags != 0x202 || !bytes.Equal(mem, expected) {
		t.Fatalf("原版 JS 不取分支：EIP=%X flags=%X err=%v", c.EIP, c.EFlags, err)
	}
	regs[EAX] = 0
	if err := c.Step(); err != nil || c.EIP != 4 || c.R != regs || c.Seg != segs || c.EFlags != 0x246 || !bytes.Equal(mem, expected) {
		t.Fatalf("原版 SUB 產生 JNS 輸入：R=%X flags=%X err=%v", c.R, c.EFlags, err)
	}
	if err := c.Step(); err != nil || c.EIP != 8 || c.R != regs || c.Seg != segs || c.EFlags != 0x246 || !bytes.Equal(mem, expected) {
		t.Fatalf("原版 JNS 取分支：EIP=%X flags=%X err=%v", c.EIP, c.EFlags, err)
	}
}

func TestShortSignBranchesKeepNearForms(t *testing.T) {
	for _, op := range []byte{0x88, 0x89} {
		for _, sign := range []uint32{0, SF} {
			mem := testBus{0x0f, op, 4, 0, 0, 0}
			c := New(mem)
			c.EFlags = 2 | IF | sign | OF | ZF
			flags := c.EFlags
			want := uint32(6)
			taken := sign != 0
			if op == 0x89 {
				taken = !taken
			}
			if taken {
				want = 10
			}
			if err := c.Step(); err != nil || c.EIP != want || c.EFlags != flags {
				t.Fatalf("既有近符號分支 op=%X flags=%X EIP=%X err=%v", op, flags, c.EIP, err)
			}
		}
	}
}
