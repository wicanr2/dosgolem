package cpu386

import (
	"bytes"
	"encoding/binary"
	"testing"
)

// 356沿344的字面32bit真值位圖建立預期，不調CPU或Jcc布林表。
func checkSETccMemory356(t *testing.T, c *CPU, mem testBus, target uint32, opcode byte, size int) {
	t.Helper()
	want := snapshotInImmediate(c)
	before := append([]byte(nil), mem...)
	before[target] = setccExpectedByte(c.EFlags, opcode)
	bus := &xchgByteBus354{testBus: mem, readFail: target, writeFail: ^uint32(0)}
	c.Bus = bus
	if err := c.Step(); err != nil || c.EIP != uint32(size) || snapshotInImmediate(c) != want || !bytes.Equal(mem, before) || len(bus.writes) != 1 || bus.writes[0] != target {
		t.Fatalf("SETcc memory op=%X target=%X flags=%X writes=%v：%v", opcode, target, c.EFlags, bus.writes, err)
	}
}
func TestSETccMemoryTruthAndAllInitialBytes(t *testing.T) {
	for _, context := range []uint32{2 | IF | DF | 0x200000, 2 | 0x40000} {
		for combo := 0; combo < 64; combo++ {
			flags := context
			for bit, flag := range []uint32{CF, PF, ZF, SF, OF, AF} {
				if combo>>bit&1 != 0 {
					flags |= flag
				}
			}
			for opcode := byte(0x90); opcode <= 0x9f; opcode++ {
				for ignored := byte(0); ignored < 8; ignored++ {
					code := []byte{0x0f, opcode, ignored<<3 | 5, 32, 0, 0, 0}
					c, mem := xchgByteFixture354(code)
					c.EFlags = flags
					mem[543], mem[545] = 0x55, 0xaa
					for initial := 0; initial < 256; initial++ {
						c.EIP = 0
						mem[544] = byte(initial)
						checkSETccMemory356(t, c, mem, 544, opcode, len(code))
					}
				}
			}
		}
	}
}
func TestSETccMemoryWrapLastByteAndIdenticalValue(t *testing.T) {
	for _, tc := range []struct {
		code         []byte
		base, target uint32
	}{
		{[]byte{0x0f, 0x94, 0xa0, 32, 0, 0, 0}, 0xfffffff0, 528},
		{[]byte{0x0f, 0x95, 0x0d, 255, 1, 0, 0}, 0, 1023},
		{[]byte{0x0f, 0x9e, 0x15, 33, 0, 0, 0}, 0, 545},
	} {
		for _, flags := range []uint32{2 | IF, 2 | IF | ZF | SF | AF} {
			c, mem := xchgByteFixture354(tc.code)
			c.R[EAX] = tc.base
			c.EFlags = flags
			mem[tc.target] = setccExpectedByte(flags, tc.code[1])
			checkSETccMemory356(t, c, mem, tc.target, tc.code[1], len(tc.code))
		}
	}
}
func TestSETccMemoryFailureDoesNotPublish(t *testing.T) {
	for opcode := byte(0x90); opcode <= 0x9f; opcode++ {
		for _, kind := range []string{"未知段", "唯讀", "段外", "線性溢位", "寫失敗"} {
			c, mem := xchgByteFixture354([]byte{0x0f, opcode, 0x25, 32, 0, 0, 0})
			mem[544] = 0xe7
			bus := &xchgByteBus354{testBus: mem, readFail: ^uint32(0), writeFail: ^uint32(0)}
			c.Bus = bus
			switch kind {
			case "未知段":
				c.Seg[SegDS] = 0x200
			case "唯讀":
				c.SetDescriptor(0x188, Descriptor{Base: 512, Limit: 511})
			case "段外":
				c.SetDescriptor(0x188, Descriptor{Base: 512, Limit: 31, Writable: true})
			case "線性溢位":
				c.SetDescriptor(0x188, Descriptor{Base: 0xfffffff0, Limit: 511, Writable: true})
			case "寫失敗":
				bus.writeFail = 544
			}
			want := snapshotInImmediate(c)
			before := append([]byte(nil), mem...)
			count := 0
			if kind == "寫失敗" {
				count = 1
			}
			if err := c.Step(); err == nil || snapshotInImmediate(c) != want || !bytes.Equal(mem, before) || len(bus.writes) != count {
				t.Fatalf("op%X %s不發布：%v writes=%v", opcode, kind, err, bus.writes)
			}
		}
	}
}
func TestSETccMemoryPrefixesAndAllTruncation(t *testing.T) {
	for opcode := byte(0x90); opcode <= 0x9f; opcode++ {
		for _, prefix := range []byte{0x66, 0x67, 0x26, 0x2e, 0x36, 0x3e, 0x64, 0x65, 0xf0, 0xf2, 0xf3} {
			code := []byte{prefix, 0x0f, opcode, 0x05, 32, 0, 0, 0}
			c, mem := xchgByteFixture354(code)
			for _, selector := range c.Seg {
				c.SetDescriptor(selector, Descriptor{Base: 512, Limit: 511, Writable: true})
			}
			c.SetDescriptor(c.Seg[SegCS], Descriptor{Limit: uint32(len(mem) - 1), Writable: true})
			// CS覆寫時目的offset32仍為合法可寫位置，排除只因無效目的而拒絕。
			mem[32], mem[544] = 0xe7, 0xe7
			bus := &xchgByteBus354{testBus: mem, readFail: ^uint32(0), writeFail: ^uint32(0)}
			c.Bus = bus
			want := snapshotInImmediate(c)
			before := append([]byte(nil), mem...)
			if err := c.Step(); err == nil || snapshotInImmediate(c) != want || !bytes.Equal(mem, before) || len(bus.writes) != 0 {
				t.Fatalf("prefix%X opcode%X：%v", prefix, opcode, err)
			}
		}
		for _, full := range [][]byte{
			{0x0f, opcode, 0x05, 32, 0, 0, 0},
			{0x0f, opcode, 0x04, 0x25, 32, 0, 0, 0},
			{0x0f, opcode, 0x44, 0x24, 0xfd},
			{0x0f, opcode, 0x84, 0x24, 32, 0, 0, 0},
		} {
			for n := 0; n < len(full); n++ {
				c, mem := inImmediateFixture(full[:n])
				bus := &xchgByteBus354{testBus: mem, readFail: ^uint32(0), writeFail: ^uint32(0)}
				c.Bus = bus
				want := snapshotInImmediate(c)
				before := append([]byte(nil), mem...)
				if err := c.Step(); err == nil || snapshotInImmediate(c) != want || !bytes.Equal(mem, before) || len(bus.writes) != 0 {
					t.Fatalf("截短%X：%v", full[:n], err)
				}
			}
		}
	}
}
func TestSETccMemoryCMPConsumers(t *testing.T) {
	values := []uint32{0, 1, 2, 0xffffffff, 0xfffffffe, 0x7fffffff, 0x7ffffffe, 0x80000000, 0x80000001, 0x55555555, 0xaaaaaaaa, 0x00010000, 0xffff0000}
	for n := uint(0); n < 32; n++ {
		values = append(values, 1<<n, ^uint32(1<<n))
	}
	for _, a := range values {
		for _, b := range values {
			conditions := map[byte]bool{0x92: a < b, 0x93: a >= b, 0x94: a == b, 0x95: a != b, 0x96: a <= b, 0x97: a > b, 0x9c: int32(a) < int32(b), 0x9d: int32(a) >= int32(b), 0x9e: int32(a) <= int32(b), 0x9f: int32(a) > int32(b)}
			for _, opcode := range []byte{0x92, 0x93, 0x94, 0x95, 0x96, 0x97, 0x9c, 0x9d, 0x9e, 0x9f} {
				code := []byte{0x39, 0xd8, 0x0f, opcode, 0x05, 32, 0, 0, 0}
				c, mem := xchgByteFixture354(code)
				c.R[EAX], c.R[EBX] = a, b
				if err := c.Step(); err != nil || c.EIP != 2 {
					t.Fatalf("前置CMP：%v", err)
				}
				expected := byte(0)
				if conditions[opcode] {
					expected = 1
				}
				want := snapshotInImmediate(c)
				before := append([]byte(nil), mem...)
				before[544] = expected
				bus := &xchgByteBus354{testBus: mem, readFail: 544, writeFail: ^uint32(0)}
				c.Bus = bus
				if err := c.Step(); err != nil || c.EIP != 9 || snapshotInImmediate(c) != want || !bytes.Equal(mem, before) || len(bus.writes) != 1 || bus.writes[0] != 544 {
					t.Fatalf("CMP %X %X SET%X memory：%v", a, b, opcode, err)
				}
			}
		}
	}
}

func TestSETccMemoryAllModRMAndSIBAndIgnoredFields(t *testing.T) {
	for opcode := byte(0x90); opcode <= 0x9f; opcode++ {
		selectedFlags := []uint32{}
		for _, result := range []byte{0, 1} {
			for combination := 0; combination < 32; combination++ {
				flags := uint32(2 | IF | DF | AF)
				for bit, flag := range []uint32{CF, PF, ZF, SF, OF} {
					if combination>>bit&1 != 0 {
						flags |= flag
					}
				}
				if setccExpectedByte(flags, opcode) == result {
					selectedFlags = append(selectedFlags, flags)
					break
				}
			}
		}
		if len(selectedFlags) != 2 {
			t.Fatal("真值正負對照不足")
		}
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
								code := []byte{0x0f, opcode, mod<<6 | byte(index)<<3 | rm}
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
								for _, flags := range selectedFlags {
									c.EIP = 0
									c.EFlags = flags
									checkSETccMemory356(t, c, mem, target, opcode, len(code))
								}
							}
						}
					}
				}
			}
		}
	}
}
