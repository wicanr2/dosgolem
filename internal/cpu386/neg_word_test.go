package cpu386

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"testing"
)

// 規格358以整數餘數、nibble借位與位元計數，不呼叫CPU旗標helper。
func negWordValue358(raw uint16) uint16 { return uint16((65536 - int(raw)) % 65536) }
func negWordFlags358(raw uint16, old uint32) uint32 {
	result := int(negWordValue358(raw))
	flags := old &^ (CF | PF | AF | ZF | SF | OF)
	if raw != 0 {
		flags |= CF
	}
	if raw%16 != 0 {
		flags |= AF
	}
	if raw == 32768 {
		flags |= OF
	}
	if result == 0 {
		flags |= ZF
	}
	if result >= 32768 {
		flags |= SF
	}
	ones := 0
	for n := result % 256; n > 0; n /= 2 {
		ones += n % 2
	}
	if ones%2 == 0 {
		flags |= PF
	}
	return flags
}

type negWordBus358 struct {
	testBus
	target uint32
	writes int
}

func (b *negWordBus358) Write8(addr uint32, v byte) error {
	if addr != b.target+uint32(b.writes) || b.writes >= 2 {
		return fmt.Errorf("NEG非目的兩byte寫入")
	}
	b.writes++
	return b.testBus.Write8(addr, v)
}
func negWordMemoryCheck358(t *testing.T, c *CPU, mem testBus, target uint32, size int) {
	t.Helper()
	value := binary.LittleEndian.Uint16(mem[target : target+2])
	want := snapshotInImmediate(c)
	want.flags = negWordFlags358(value, c.EFlags)
	ram := append([]byte(nil), mem...)
	binary.LittleEndian.PutUint16(ram[target:target+2], negWordValue358(value))
	if err := c.Step(); err != nil || c.EIP != uint32(size) || snapshotInImmediate(c) != want || !bytes.Equal(mem, ram) {
		t.Fatalf("word NEG target=%X source=%04X err=%v", target, value, err)
	}
}
func TestNEGWordMemoryAllValuesAndFlags(t *testing.T) {
	code := []byte{0x66, 0xf7, 0x1d, 32, 0, 0, 0}
	c, mem := xchgByteFixture354(code)
	initial := snapshotInImmediate(c)
	bus := &negWordBus358{testBus: mem, target: 544}
	c.Bus = bus
	for pattern := 0; pattern < 64; pattern++ {
		flags := uint32(2 | IF | DF | 0x200000)
		for bit, flag := range []uint32{CF, PF, AF, ZF, SF, OF} {
			if pattern/(1<<bit)%2 != 0 {
				flags |= flag
			}
		}
		for value := 0; value < 65536; value++ {
			raw := uint16(value)
			binary.LittleEndian.PutUint16(mem[544:546], raw)
			c.EIP, c.EFlags = 0, flags
			bus.writes = 0
			want := initial
			want.flags = negWordFlags358(raw, flags)
			if err := c.Step(); err != nil || c.EIP != 7 || snapshotInImmediate(c) != want || binary.LittleEndian.Uint16(mem[544:546]) != negWordValue358(raw) || bus.writes != 2 {
				t.Fatalf("raw=%04X flags=%X err=%v", raw, flags, err)
			}
		}
	}
	expected := make([]byte, len(mem))
	copy(expected, code)
	copy(expected[544:546], mem[544:546])
	if !bytes.Equal(mem, expected) {
		t.Fatal("NEG修改相鄰memory")
	}
}
func TestNEGWordRegisterAllValuesAndHighWords(t *testing.T) {
	for reg := 0; reg < 8; reg++ {
		code := []byte{0x66, 0xf7, 0xd8 | byte(reg)}
		c, mem := xchgByteFixture354(code)
		for i := range c.R {
			c.R[i] = 0xa5b60000 + uint32(i)*0x10000 + 0x1357
		}
		initial := snapshotInImmediate(c)
		for _, flags := range []uint32{2 | IF | DF | 0x200000, 0xa5a52fd7} {
			for value := 0; value < 65536; value++ {
				raw := uint16(value)
				c.R = imulWordReplace357(initial.r, reg, raw)
				c.EIP, c.EFlags = 0, flags
				want := initial
				want.r = imulWordReplace357(initial.r, reg, negWordValue358(raw))
				want.flags = negWordFlags358(raw, flags)
				if err := c.Step(); err != nil || c.EIP != 3 || snapshotInImmediate(c) != want || !bytes.Equal(mem[:len(code)], code) {
					t.Fatalf("reg=%d raw=%04X err=%v", reg, raw, err)
				}
			}
		}
		expected := make([]byte, len(mem))
		copy(expected, code)
		if !bytes.Equal(mem, expected) {
			t.Fatal("register NEG寫入memory")
		}
	}
}
func TestNEGWordMemoryAllModRMAndSIB(t *testing.T) {
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
						code := []byte{0x66, 0xf7, mod<<6 | 0x18 | rm}
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
						for _, raw := range []uint16{0, 1, 0x8000, 0xffff} {
							c, mem := xchgByteFixture354(code)
							binary.LittleEndian.PutUint16(mem[512+offset:], raw)
							binary.LittleEndian.PutUint16(mem[1024+offset:], raw^0x1357)
							target := 512 + offset
							if stack {
								target = 1024 + offset
							}
							negWordMemoryCheck358(t, c, mem, target, len(code))
						}
					}
				}
			}
		}
	}
}
func TestNEGWordMemoryWrapAndLastWord(t *testing.T) {
	for _, tc := range []struct {
		code         []byte
		base, target uint32
	}{
		{[]byte{0x66, 0xf7, 0x98, 32, 0, 0, 0}, 0xfffffff0, 528},
		{[]byte{0x66, 0xf7, 0x1d, 254, 1, 0, 0}, 0, 1022},
		{[]byte{0x66, 0xf7, 0x1d, 33, 0, 0, 0}, 0, 545},
	} {
		c, mem := xchgByteFixture354(tc.code)
		c.R[EAX] = tc.base
		binary.LittleEndian.PutUint16(mem[tc.target:], 0x8001)
		negWordMemoryCheck358(t, c, mem, tc.target, len(tc.code))
	}
}
func TestNEGWordMemoryFailureAndPartialWriteModel(t *testing.T) {
	code := []byte{0x66, 0xf7, 0x1d, 32, 0, 0, 0}
	for _, kind := range []string{"未知段", "唯讀", "段外", "最後byte", "線性溢位"} {
		c, mem := xchgByteFixture354(code)
		binary.LittleEndian.PutUint16(mem[544:], 0x1234)
		bus := &xchgByteBus354{testBus: mem, readFail: ^uint32(0), writeFail: ^uint32(0)}
		c.Bus = bus
		switch kind {
		case "未知段":
			c.Seg[SegDS] = 0x200
		case "唯讀":
			c.SetDescriptor(0x188, Descriptor{Base: 512, Limit: 511})
		case "段外":
			c.SetDescriptor(0x188, Descriptor{Base: 512, Limit: 31, Writable: true})
		case "最後byte":
			binary.LittleEndian.PutUint32(mem[3:7], 511)
		case "線性溢位":
			c.SetDescriptor(0x188, Descriptor{Base: 0xfffffff0, Limit: 511, Writable: true})
		}
		want := snapshotInImmediate(c)
		ram := append([]byte(nil), mem...)
		if err := c.Step(); err == nil || snapshotInImmediate(c) != want || !bytes.Equal(mem, ram) || len(bus.writes) != 0 {
			t.Fatalf("%s err=%v", kind, err)
		}
	}
	for byteIndex := uint32(0); byteIndex < 2; byteIndex++ {
		for _, write := range []bool{false, true} {
			c, mem := xchgByteFixture354(code)
			binary.LittleEndian.PutUint16(mem[544:], 0x1234)
			bus := &xchgByteBus354{testBus: mem, readFail: ^uint32(0), writeFail: ^uint32(0)}
			c.Bus = bus
			if write {
				bus.writeFail = 544 + byteIndex
			} else {
				bus.readFail = 544 + byteIndex
			}
			want := snapshotInImmediate(c)
			ram := append([]byte(nil), mem...)
			writes := 0
			var result [2]byte
			binary.LittleEndian.PutUint16(result[:], negWordValue358(0x1234))
			if write {
				writes = int(byteIndex) + 1
				copy(ram[544:544+byteIndex], result[:byteIndex])
			}
			if err := c.Step(); err == nil || snapshotInImmediate(c) != want || !bytes.Equal(mem, ram) || len(bus.writes) != writes {
				t.Fatalf("write=%t byte=%d err=%v", write, byteIndex, err)
			}
		}
	}
}
func TestNEGWordTruncationPrefixesAndOtherGroups(t *testing.T) {
	for _, full := range [][]byte{{0x66, 0xf7, 0xd8}, {0x66, 0xf7, 0x1d, 32, 0, 0, 0}, {0x66, 0xf7, 0x1c, 0x25, 32, 0, 0, 0}, {0x66, 0xf7, 0x5c, 0x24, 0xfd}, {0x66, 0xf7, 0x9c, 0x24, 32, 0, 0, 0}} {
		for cut := 0; cut < len(full); cut++ {
			c, mem := xchgByteFixture354(full)
			binary.LittleEndian.PutUint16(mem[544:], 0x1234)
			bus := &xchgByteBus354{testBus: mem, readFail: uint32(cut), writeFail: ^uint32(0)}
			c.Bus = bus
			want := snapshotInImmediate(c)
			ram := append([]byte(nil), mem...)
			if err := c.Step(); err == nil || snapshotInImmediate(c) != want || !bytes.Equal(mem, ram) || len(bus.writes) != 0 {
				t.Fatalf("cut=%d code=%X err=%v", cut, full, err)
			}
		}
	}
	for _, prefix := range []byte{0x26, 0x2e, 0x36, 0x3e, 0x64, 0x65, 0x67, 0xf0, 0xf2, 0xf3} {
		for _, suffix := range [][]byte{{0x66, 0xf7, 0xd9}, {0x66, 0xf7, 0x1d, 32, 0, 0, 0}} {
			code := append([]byte{prefix}, suffix...)
			c, mem := xchgByteFixture354(code)
			for _, seg := range c.Seg {
				c.SetDescriptor(seg, Descriptor{Base: 512, Limit: 511, Writable: true})
			}
			binary.LittleEndian.PutUint16(mem[544:], 0x1234)
			want := snapshotInImmediate(c)
			ram := append([]byte(nil), mem...)
			if err := c.Step(); err == nil || snapshotInImmediate(c) != want || !bytes.Equal(mem, ram) {
				t.Fatalf("prefix=%02X err=%v", prefix, err)
			}
		}
	}
	// TEST／register MUL與IMUL保持；memory群組1／2／4／5／6／7不擴張。
	for _, group := range []byte{1, 2, 4, 5, 6, 7} {
		code := []byte{0x66, 0xf7, group<<3 | 5, 32, 0, 0, 0}
		c, mem := xchgByteFixture354(code)
		binary.LittleEndian.PutUint16(mem[544:], 0x1234)
		want := snapshotInImmediate(c)
		ram := append([]byte(nil), mem...)
		if err := c.Step(); err == nil || snapshotInImmediate(c) != want || !bytes.Equal(mem, ram) {
			t.Fatalf("group=%d err=%v", group, err)
		}
	}
}
