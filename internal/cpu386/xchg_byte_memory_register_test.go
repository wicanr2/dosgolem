package cpu386

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"testing"
)

// 354以little-endian四byte視圖選lane，獨立於CPU的遮罩寫法。
func byteValue354(r [8]uint32, index int) byte {
	var data [4]byte
	binary.LittleEndian.PutUint32(data[:], r[index%4])
	return data[index/4]
}
func replaceByte354(r [8]uint32, index int, value byte) [8]uint32 {
	var data [4]byte
	binary.LittleEndian.PutUint32(data[:], r[index%4])
	data[index/4] = value
	r[index%4] = binary.LittleEndian.Uint32(data[:])
	return r
}
func xchgByteFixture354(code []byte) (*CPU, testBus) {
	mem := make(testBus, 1600)
	copy(mem, code)
	c := New(mem)
	c.Seg = [6]uint16{0, 0x188, 0x198, 0x1a0, 0x1a8, 0x190}
	c.SetDescriptor(0x188, Descriptor{Base: 512, Limit: 511, Writable: true})
	c.SetDescriptor(0x190, Descriptor{Base: 1024, Limit: 511, Writable: true})
	c.R = [8]uint32{16, 17, 18, 19, 20, 21, 22, 23}
	c.EFlags = 0xa5a52fd7
	c.FPUControl = 0x127f
	c.FPUStatus = 0x20
	c.FPUDepth = 2
	c.FPUStack = [8]float64{1.25, -2.5, 3, 4, 5, 6, 7, 8}
	return c, mem
}
func checkXCHGByteMemory354(t *testing.T, c *CPU, mem testBus, target uint32, index, size int) {
	t.Helper()
	want := snapshotInImmediate(c)
	oldMemory := mem[target]
	oldRegister := byteValue354(c.R, index)
	want.r = replaceByte354(c.R, index, oldMemory)
	expected := append([]byte(nil), mem...)
	expected[target] = oldRegister
	if err := c.Step(); err != nil || c.EIP != uint32(size) || snapshotInImmediate(c) != want || !bytes.Equal(mem, expected) {
		t.Fatalf("memory XCHG target=%X index=%d mem=%X reg=%X R=%X flags=%X err=%v", target, index, oldMemory, oldRegister, c.R, c.EFlags, err)
	}
}
func TestXCHGByteMemoryAllRegisterValuesAndFlags(t *testing.T) {
	for index := 0; index < 8; index++ {
		code := []byte{0x86, byte(index)<<3 | 5, 32, 0, 0, 0}
		for a := 0; a < 256; a++ {
			for b := 0; b < 256; b++ {
				c, mem := xchgByteFixture354(code)
				c.R = [8]uint32{0x98765432, 0xabcdeff0, 0x13579bdf, 0x2468ace0, 20, 21, 22, 23}
				c.R = replaceByte354(c.R, index, byte(a))
				mem[544] = byte(b)
				checkXCHGByteMemory354(t, c, mem, 544, index, len(code))
			}
		}
		for combo := uint32(0); combo < 64; combo++ {
			c, mem := xchgByteFixture354(code)
			c.EFlags = 2 | IF | DF | 0x200000
			for bit, flag := range []uint32{CF, PF, AF, ZF, SF, OF} {
				if combo/(1<<bit)%2 != 0 {
					c.EFlags |= flag
				}
			}
			mem[544] = 0xe7
			checkXCHGByteMemory354(t, c, mem, 544, index, len(code))
		}
	}
}
func TestXCHGByteMemoryAllModRMAndSIBWithRegisterAliases(t *testing.T) {
	for index := 0; index < 8; index++ {
		for mod := byte(0); mod < 3; mod++ {
			for rm := byte(0); rm < 8; rm++ {
				scales, indexes, bases := []byte{0}, []byte{4}, []byte{rm}
				if rm == 4 {
					scales = []byte{0, 1, 2, 3}
					indexes = []byte{0, 1, 2, 3, 4, 5, 6, 7}
					bases = indexes
				}
				for _, scale := range scales {
					for _, ix := range indexes {
						for _, base := range bases {
							code := []byte{0x86, mod<<6 | byte(index)<<3 | rm}
							if rm == 4 {
								code = append(code, scale<<6|ix<<3|base)
							}
							noBase := mod == 0 && base == 5
							offset := uint32(16 + base)
							stack := base == 4 || base == 5
							if noBase {
								offset = 32
								stack = false
							}
							if rm == 4 && ix != 4 {
								offset += uint32(16+ix) * uint32(1<<scale)
							}
							if mod == 1 {
								code = append(code, 0xfd)
								offset -= 3
							}
							if mod == 2 {
								code = binary.LittleEndian.AppendUint32(code, 32)
								offset += 32
							}
							if noBase {
								code = binary.LittleEndian.AppendUint32(code, 32)
							}
							c, mem := xchgByteFixture354(code)
							mem[512+offset] = 0xe7
							mem[1024+offset] = 0x9b
							target := 512 + offset
							if stack {
								target = 1024 + offset
							}
							checkXCHGByteMemory354(t, c, mem, target, index, len(code))
						}
					}
				}
			}
		}
	}
}
func TestXCHGByteMemoryWrapAndLastByte(t *testing.T) {
	for _, tc := range []struct {
		code         []byte
		base, target uint32
	}{
		{[]byte{0x86, 0xa0, 32, 0, 0, 0}, 0xfffffff0, 528},
		{[]byte{0x86, 0x0d, 255, 1, 0, 0}, 0, 1023},
		{[]byte{0x86, 0x15, 33, 0, 0, 0}, 0, 545},
	} {
		c, mem := xchgByteFixture354(tc.code)
		c.R[EAX] = tc.base
		mem[tc.target] = 0xe7
		checkXCHGByteMemory354(t, c, mem, tc.target, int((tc.code[1]>>3)&7), len(tc.code))
	}
}

type xchgByteBus354 struct {
	testBus
	readFail, writeFail uint32
	writes              []uint32
}

func (b *xchgByteBus354) Read8(addr uint32) (byte, error) {
	if addr == b.readFail {
		return 0, fmt.Errorf("受控讀取失敗")
	}
	return b.testBus.Read8(addr)
}
func (b *xchgByteBus354) Write8(addr uint32, v byte) error {
	b.writes = append(b.writes, addr)
	if addr == b.writeFail {
		return fmt.Errorf("受控寫入失敗")
	}
	return b.testBus.Write8(addr, v)
}
func TestXCHGByteMemoryFailureDoesNotPublishRegister(t *testing.T) {
	for _, kind := range []string{"未知段", "唯讀", "段外", "線性溢位", "讀失敗", "寫失敗"} {
		mem := make(testBus, 80)
		copy(mem, []byte{0x86, 0x25, 64, 0, 0, 0})
		mem[64] = 0xe7
		bus := &xchgByteBus354{testBus: mem, readFail: ^uint32(0), writeFail: ^uint32(0)}
		c := New(bus)
		c.Seg[SegDS] = 0x188
		c.SetDescriptor(0x188, Descriptor{Limit: 79, Writable: true})
		c.R[EAX] = 0x12345678
		c.EFlags = 0x2fd7
		switch kind {
		case "未知段":
			c.Seg[SegDS] = 0x200
		case "唯讀":
			c.SetDescriptor(0x188, Descriptor{Limit: 79})
		case "段外":
			c.SetDescriptor(0x188, Descriptor{Limit: 63, Writable: true})
		case "線性溢位":
			c.SetDescriptor(0x188, Descriptor{Base: 0xffffffe0, Limit: 79, Writable: true})
		case "讀失敗":
			bus.readFail = 64
		case "寫失敗":
			bus.writeFail = 64
		}
		want := snapshotInImmediate(c)
		expected := append([]byte(nil), mem...)
		if err := c.Step(); err == nil || snapshotInImmediate(c) != want || !bytes.Equal(mem, expected) {
			t.Fatalf("memory XCHG拒絕%s err=%v R=%X", kind, err, c.R)
		}
		count := 0
		if kind == "寫失敗" {
			count = 1
		}
		if len(bus.writes) != count {
			t.Fatalf("拒絕%s writes=%v", kind, bus.writes)
		}
	}
}
func TestXCHGByteMemoryRejectedPrefixesAndTruncation(t *testing.T) {
	code := []byte{0x86, 0x25, 64, 0, 0, 0}
	cases := [][]byte{}
	for n := 0; n < len(code); n++ {
		cases = append(cases, code[:n])
	}
	for _, prefix := range []byte{0x66, 0x26, 0x2e, 0x36, 0x3e, 0x64, 0x65, 0xf0, 0xf2, 0xf3, 0x67} {
		cases = append(cases, append([]byte{prefix}, code...))
	}
	cases = append(cases, []byte{0x86, 0x04}, []byte{0x86, 0x44}, []byte{0x86, 0x84})
	for _, code := range cases {
		mem := testBus(append([]byte(nil), code...))
		c := New(mem)
		c.R[EAX] = 0x12345678
		c.EFlags = 0x2fd7
		want := snapshotInImmediate(c)
		expected := append([]byte(nil), mem...)
		if err := c.Step(); err == nil || snapshotInImmediate(c) != want || !bytes.Equal(mem, expected) {
			t.Fatalf("未審查prefix／截短=%X err=%v", code, err)
		}
	}
}
func TestXCHGByteMemoryIdenticalValueStillWrites(t *testing.T) {
	for index := 0; index < 8; index++ {
		for _, value := range []byte{0, 1, 0x80, 0xff} {
			mem := make(testBus, 80)
			copy(mem, []byte{0x86, byte(index)<<3 | 5, 64, 0, 0, 0})
			mem[64] = value
			bus := &xchgByteBus354{testBus: mem, readFail: ^uint32(0), writeFail: ^uint32(0)}
			c := New(bus)
			c.Seg[SegDS] = 0x188
			c.SetDescriptor(0x188, Descriptor{Limit: 64, Writable: true})
			c.R = replaceByte354(c.R, index, value)
			c.EFlags = 0x2fd7
			want := snapshotInImmediate(c)
			expected := append([]byte(nil), mem...)
			if err := c.Step(); err != nil || c.EIP != 6 || snapshotInImmediate(c) != want || !bytes.Equal(mem, expected) || len(bus.writes) != 1 || bus.writes[0] != 64 {
				t.Fatalf("相同byte必寫 index=%d value=%X writes=%v err=%v", index, value, bus.writes, err)
			}
		}
	}
}
func TestXCHGByteRegisterAllPairsRemain(t *testing.T) {
	for a := 0; a < 8; a++ {
		for b := 0; b < 8; b++ {
			mem := testBus{0x86, 0xc0 | byte(a)<<3 | byte(b)}
			c := New(mem)
			c.R = [8]uint32{0x12345678, 0x9abcdef0, 0x13579bdf, 0x2468ace0, 4, 5, 6, 7}
			c.EFlags = 0x2fd7
			want := snapshotInImmediate(c)
			av, bv := byteValue354(c.R, a), byteValue354(c.R, b)
			want.r = replaceByte354(replaceByte354(c.R, a, bv), b, av)
			if err := c.Step(); err != nil || c.EIP != 2 || snapshotInImmediate(c) != want {
				t.Fatalf("原86 register a=%d b=%d R=%X err=%v", a, b, c.R, err)
			}
		}
	}
}
