package cpu386

import (
	"bytes"
	"testing"
)

// 344：Intel SETcc的32列真值以位圖保存，索引依CF/PF/ZF/SF/OF。
// 測試不重用CPU或Jcc的條件運算式。
var setccTruth = [16]uint32{
	0xffff0000, 0x0000ffff, 0xaaaaaaaa, 0x55555555,
	0xf0f0f0f0, 0x0f0f0f0f, 0xfafafafa, 0x05050505,
	0xff00ff00, 0x00ff00ff, 0xcccccccc, 0x33333333,
	0x00ffff00, 0xff0000ff, 0xf0fffff0, 0x0f00000f,
}

func setccExpectedByte(flags uint32, opcode byte) byte {
	index := uint(0)
	for i, flag := range []uint32{CF, PF, ZF, SF, OF} {
		if flags&flag != 0 {
			index |= 1 << i
		}
	}
	return byte(setccTruth[opcode&15] >> index & 1)
}

func checkSETcc(t *testing.T, opcode, dest, ignored, initial byte, flags uint32) {
	t.Helper()
	code := []byte{0x0f, opcode, 0xc0 | ignored<<3 | dest}
	c, mem := inImmediateFixture(code)
	c.R = setleReplaceByte(c.R, dest, initial)
	c.EFlags = flags
	want := snapshotInImmediate(c)
	want.r = setleReplaceByte(want.r, dest, setccExpectedByte(flags, opcode))
	bus := &subByteFailureBus{memory: mem, failWrite: true}
	c.Bus = bus
	if err := c.Step(); err != nil || c.EIP != 3 || snapshotInImmediate(c) != want || !bytes.Equal(mem, code) || bus.writes != 0 {
		t.Fatalf("0F%X flags%X dest%d ignored%d initial%X：%v", opcode, flags, dest, ignored, initial, err)
	}
}

func TestSETccRegisterTruthAliasesAndIgnoredFields(t *testing.T) {
	for _, context := range []uint32{2 | IF | DF | 0x200000, 2 | 0x40000} {
		for combination := 0; combination < 64; combination++ {
			flags := context
			for i, flag := range []uint32{CF, PF, ZF, SF, OF, AF} {
				if combination>>i&1 != 0 {
					flags |= flag
				}
			}
			for opcode := byte(0x90); opcode <= 0x9f; opcode++ {
				for dest := byte(0); dest < 8; dest++ {
					for ignored := byte(0); ignored < 8; ignored++ {
						for _, initial := range []byte{0, 1, 0x80, 0xff} {
							checkSETcc(t, opcode, dest, ignored, initial, flags)
						}
					}
				}
			}
		}
	}
}

func TestSETccRegisterEveryInitialByte(t *testing.T) {
	for _, flags := range []uint32{2 | IF, 2 | IF | CF | PF | ZF | SF | OF | AF} {
		for opcode := byte(0x90); opcode <= 0x9f; opcode++ {
			for dest := byte(0); dest < 8; dest++ {
				for initial := 0; initial < 256; initial++ {
					checkSETcc(t, opcode, dest, 7, byte(initial), flags)
				}
			}
		}
	}
}

func TestSETccRegisterSignedAndUnsignedCMPConsumers(t *testing.T) {
	values := []uint32{0, 1, 2, 0xffffffff, 0xfffffffe, 0x7fffffff, 0x7ffffffe, 0x80000000, 0x80000001, 0x55555555, 0xaaaaaaaa, 0x00010000, 0xffff0000}
	for n := uint(0); n < 32; n++ {
		values = append(values, 1<<n, ^uint32(1<<n))
	}
	for _, a := range values {
		for _, b := range values {
			// 以數值關係獨立核算，涵蓋正負跨界與signed overflow。
			conditions := map[byte]bool{0x92: a < b, 0x93: a >= b, 0x94: a == b, 0x95: a != b, 0x96: a <= b, 0x97: a > b, 0x9c: int32(a) < int32(b), 0x9d: int32(a) >= int32(b), 0x9e: int32(a) <= int32(b), 0x9f: int32(a) > int32(b)}
			for _, opcode := range []byte{0x92, 0x93, 0x94, 0x95, 0x96, 0x97, 0x9c, 0x9d, 0x9e, 0x9f} {
				for _, dest := range []byte{0, 4} {
					code := []byte{0x39, 0xd8, 0x0f, opcode, 0xc0 | dest}
					c, mem := inImmediateFixture(code)
					c.R[EAX], c.R[EBX] = a, b
					r := c.R
					if err := c.Step(); err != nil || c.EIP != 2 || c.R != r {
						t.Fatalf("CMP：%v", err)
					}
					value := byte(0)
					if conditions[opcode] {
						value = 1
					}
					want := snapshotInImmediate(c)
					want.r = setleReplaceByte(want.r, dest, value)
					if err := c.Step(); err != nil || c.EIP != 5 || snapshotInImmediate(c) != want || !bytes.Equal(mem, code) {
						t.Fatalf("CMP %X %X SET%X dest%d：%v", a, b, opcode, dest, err)
					}
				}
			}
		}
	}
}

func TestSETccRegisterKeepsJccTruth(t *testing.T) {
	for combination := 0; combination < 32; combination++ {
		flags := uint32(2 | IF | AF)
		for i, flag := range []uint32{CF, PF, ZF, SF, OF} {
			if combination>>i&1 != 0 {
				flags |= flag
			}
		}
		for opcode := byte(0x80); opcode <= 0x8f; opcode++ {
			code := []byte{0x0f, opcode, 2, 0, 0, 0, 0, 0, 0}
			c, mem := inImmediateFixture(code)
			c.EFlags = flags
			want := snapshotInImmediate(c)
			eip := uint32(6 + 2*setccExpectedByte(flags, opcode))
			if err := c.Step(); err != nil || c.EIP != eip || snapshotInImmediate(c) != want || !bytes.Equal(mem, code) {
				t.Fatalf("Jcc%X flags%X：%v", opcode, flags, err)
			}
		}
	}
}

func TestSETccRegisterRejectsUnknownMemoryPrefixesAndTruncation(t *testing.T) {
	for opcode := byte(0x90); opcode <= 0x9f; opcode++ {
		codes := [][]byte{{}, {0x0f}, {0x0f, opcode}}
		for _, prefix := range []byte{0x66, 0x67, 0x26, 0x2e, 0x36, 0x3e, 0x64, 0x65, 0xf0, 0xf2, 0xf3} {
			codes = append(codes, []byte{prefix, 0x0f, opcode, 0xc0})
		}
		for _, code := range codes {
			c, mem := inImmediateFixture(code)
			want := snapshotInImmediate(c)
			if err := c.Step(); err == nil || snapshotInImmediate(c) != want || !bytes.Equal(mem, code) {
				t.Fatalf("拒絕%X：%v", code, err)
			}
		}
		for mod := byte(0); mod < 3; mod++ {
			for reg := byte(0); reg < 8; reg++ {
				for rm := byte(0); rm < 8; rm++ {
					code := []byte{0x0f, opcode, mod<<6 | reg<<3 | rm, 0, 32, 0, 0, 0}
					c, mem := subByteMemoryFixture(code)
					// 356：有效memory已接通，舊負例限定未知selector拒絕。
					c.Seg[SegDS], c.Seg[SegSS] = 0x200, 0x200
					want := snapshotInImmediate(c)
					before := append([]byte(nil), mem...)
					bus := &subByteFailureBus{memory: mem, failWrite: true}
					c.Bus = bus
					if err := c.Step(); err == nil || snapshotInImmediate(c) != want || !bytes.Equal(mem, before) || bus.writes != 0 {
						t.Fatalf("memory%X：%v", code, err)
					}
				}
			}
		}
	}
	for _, opcode := range []byte{0xa2, 0xa3} {
		c, _ := inImmediateFixture([]byte{0x0f, opcode, 0xc0})
		want := snapshotInImmediate(c)
		if err := c.Step(); err == nil || snapshotInImmediate(c) != want {
			t.Fatalf("相鄰未支援0F%X：%v", opcode, err)
		}
	}
}
