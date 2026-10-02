package cpu386

import (
	"bytes"
	"encoding/binary"
	"testing"
)

// unsigned總和、signed範圍與低byte逐bit計數，不使用CPU旗標函式。
func adcDwordOracle(a, b, initial uint32) (uint32, uint32) {
	carry := uint64(initial % 2)
	sum := uint64(a) + uint64(b) + carry
	result := sum % 0x100000000
	flags := initial &^ (CF | PF | AF | ZF | SF | OF)
	if sum >= 0x100000000 {
		flags |= CF
	}
	if uint64(a%16)+uint64(b%16)+carry >= 16 {
		flags |= AF
	}
	sa, sb := int64(a), int64(b)
	if a >= 0x80000000 {
		sa -= 0x100000000
	}
	if b >= 0x80000000 {
		sb -= 0x100000000
	}
	signed := sa + sb + int64(carry)
	if signed < -0x80000000 || signed > 0x7fffffff {
		flags |= OF
	}
	if result == 0 {
		flags |= ZF
	}
	if result >= 0x80000000 {
		flags |= SF
	}
	ones := 0
	for n := result % 256; n > 0; n /= 2 {
		ones += int(n % 2)
	}
	if ones%2 == 0 {
		flags |= PF
	}
	return uint32(result), flags
}

func checkADCDwordRegisters(t *testing.T, dst, src byte, a, b, initial uint32) {
	code := []byte{0x13, 0xc0 | dst*8 | src}
	c, mem := inImmediateFixture(code)
	c.R[dst], c.R[src], c.EFlags = a, b, initial
	want := snapshotInImmediate(c)
	want.r[dst], want.flags = adcDwordOracle(c.R[dst], c.R[src], initial)
	if err := c.Step(); err != nil || c.EIP != 2 || snapshotInImmediate(c) != want || !bytes.Equal(mem, code) {
		t.Fatalf("dst=%d src=%d a=%X b=%X initial=%X：%v got=%+v want=%+v", dst, src, a, b, initial, err, snapshotInImmediate(c), want)
	}
}

func TestADCDwordAllRegisterPairsByteInputsAndCarry(t *testing.T) {
	for dst := byte(0); dst < 8; dst++ {
		for src := byte(0); src < 8; src++ {
			for a := uint32(0); a < 256; a++ {
				for b := uint32(0); b < 256; b++ {
					if dst == src && a != b {
						continue
					}
					for carry := uint32(0); carry < 2; carry++ {
						checkADCDwordRegisters(t, dst, src, a, b, 2|IF|DF|0x200000|PF|AF|ZF|SF|OF|carry)
					}
				}
			}
		}
	}
}

func TestADCDwordBitsSignedWrapNibbleEdgesAndInitialFlags(t *testing.T) {
	values := []uint32{0, 1, 15, 16, 0x7fffffff, 0x80000000, 0x80000001, 0xfffffffe, 0xffffffff, 0xaaaaaaaa, 0x55555555, 0xffff, 0x10000}
	for bit := uint(0); bit < 32; bit++ {
		values = append(values, uint32(1)<<bit, ^(uint32(1) << bit))
	}
	masks := [6]uint32{CF, PF, AF, ZF, SF, OF}
	for dst := byte(0); dst < 8; dst++ {
		for src := byte(0); src < 8; src++ {
			for _, a := range values {
				for _, b := range values {
					if dst == src && a != b {
						continue
					}
					for pattern := 0; pattern < 64; pattern++ {
						initial := uint32(2 | IF | DF | 0x200000)
						for bit, mask := range masks {
							if pattern&(1<<bit) != 0 {
								initial |= mask
							}
						}
						checkADCDwordRegisters(t, dst, src, a, b, initial)
					}
				}
			}
		}
	}
}

func TestADCDwordRejectsWithoutPublishing(t *testing.T) {
	for dst := byte(0); dst < 8; dst++ {
		for src := byte(0); src < 8; src++ {
			code := []byte{0x13, 0xc0 | dst*8 | src}
			cases := [][]byte{nil, {0x13}, {0x13, dst*8 | 5, 32, 0, 0, 0}, {0x13, 0x40 | dst*8 | src, 32}, {0x13, 0x80 | dst*8 | src, 32, 0, 0, 0}}
			for _, prefix := range []byte{0x66, 0x67, 0x26, 0x2e, 0x36, 0x3e, 0x64, 0x65, 0xf2, 0xf3, 0xf0} {
				cases = append(cases, append([]byte{prefix}, code...))
			}
			for _, instruction := range cases {
				c, mem := inImmediateFixture(instruction)
				want := snapshotInImmediate(c)
				before := append([]byte(nil), mem...)
				if err := c.Step(); err == nil || snapshotInImmediate(c) != want || !bytes.Equal(mem, before) {
					t.Fatalf("未知／截短 %X：%v", instruction, err)
				}
			}
		}
	}
}

func TestADCDwordOriginalFullStateAndIndexedADDConsumer(t *testing.T) {
	code := []byte{0x13, 0xed, 0x03, 0x34, 0xad, 0x40, 0x2d, 0x27, 0, 0x0f, 0xbf, 0xe8}
	mem := make(testBus, 0x272d48)
	copy(mem, code)
	binary.LittleEndian.PutUint32(mem[0x272d44:0x272d48], 2)
	c := New(mem)
	c.R = [8]uint32{0, 0, 0x3d69c0, 0, 0x2723d8, 0, 0x71e1d0, 0x3c6038}
	c.Seg = [6]uint16{8, 0x188, 0x188, 0, 0x20, 0x188}
	c.SetDescriptor(0x188, Descriptor{Base: 0, Limit: uint32(len(mem) - 1), Writable: true})
	c.EFlags = 0x847
	want := snapshotInImmediate(c)
	before := append([]byte(nil), mem...)
	want.r[EBP], want.flags = 1, 2
	if err := c.Step(); err != nil || c.EIP != 2 || snapshotInImmediate(c) != want || !bytes.Equal(mem, before) {
		t.Fatalf("原版完整ADC：%v", err)
	}
	want.r[ESI], want.flags = 0x71e1d2, 6
	if err := c.Step(); err != nil || c.EIP != 9 || snapshotInImmediate(c) != want || !bytes.Equal(mem, before) {
		t.Fatalf("原版索引ADD真實來源／結果：%v", err)
	}
	want.r[EBP] = 0
	if err := c.Step(); err != nil || c.EIP != 12 || snapshotInImmediate(c) != want || !bytes.Equal(mem, before) {
		t.Fatalf("索引消費後MOVSX覆寫：%v", err)
	}
}

func TestADCDwordKeepsExistingArithmeticPaths(t *testing.T) {
	for _, test := range []struct {
		code          []byte
		result, flags uint32
	}{
		{[]byte{0x03, 0xc3}, 3, 0x606},
		{[]byte{0x01, 0xd8}, 3, 0x606},
		{[]byte{0x66, 0x03, 0xc3}, 3, 0x606},
		{[]byte{0x2b, 0xc3}, 0xffffffff, 0x697},
		{[]byte{0x3b, 0xc3}, 1, 0x697},
		{[]byte{0x1b, 0xc3}, 0xfffffffe, 0x693},
	} {
		c, mem := inImmediateFixture(test.code)
		c.R[EAX], c.R[EBX] = 1, 2
		want := snapshotInImmediate(c)
		want.r[EAX], want.flags = test.result, test.flags
		if err := c.Step(); err != nil || c.EIP != uint32(len(test.code)) || snapshotInImmediate(c) != want || !bytes.Equal(mem, test.code) {
			t.Fatalf("既有指令 %X：%v got=%+v want=%+v", test.code, err, snapshotInImmediate(c), want)
		}
	}
}
