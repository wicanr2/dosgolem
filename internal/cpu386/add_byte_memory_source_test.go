package cpu386

import (
	"bytes"
	"encoding/binary"
	"math/bits"
	"testing"
)

// 規格355：沿用獨立byte視圖與規格324的較寬加法oracle，未呼叫CPU add8。
func checkADDByteSource355(t *testing.T, c *CPU, mem testBus, target uint32, index, size int) {
	t.Helper()
	want := snapshotInImmediate(c)
	a, b := byteValue354(c.R, index), mem[target]
	want.r = replaceByte354(c.R, index, byte(int(a)+int(b)))
	want.flags = addByte00Flags(a, b, c.EFlags)
	before := append([]byte(nil), mem...)
	if err := c.Step(); err != nil || c.EIP != uint32(size) || snapshotInImmediate(c) != want || !bytes.Equal(mem, before) {
		t.Fatalf("ADD memory來源 target=%X index=%d a=%X b=%X flags=%X：%v", target, index, a, b, c.EFlags, err)
	}
}
func TestADDByteSourceMemoryAllRegisterValuesAndFlags(t *testing.T) {
	for index := 0; index < 8; index++ {
		code := []byte{0x02, byte(index)<<3 | 5, 32, 0, 0, 0}
		for a := 0; a < 256; a++ {
			for b := 0; b < 256; b++ {
				c, mem := xchgByteFixture354(code)
				c.R = [8]uint32{0x98765432, 0xabcdeff0, 0x13579bdf, 0x2468ace0, 20, 21, 22, 23}
				c.R = replaceByte354(c.R, index, byte(a))
				mem[544] = byte(b)
				for _, initial := range []uint32{2 | IF | DF | 0x200000, 2 | IF | DF | 0x200000 | CF | PF | AF | ZF | SF | OF} {
					c.R = replaceByte354(c.R, index, byte(a))
					c.EFlags = initial
					c.EIP = 0
					checkADDByteSource355(t, c, mem, 544, index, len(code))
				}
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
			checkADDByteSource355(t, c, mem, 544, index, len(code))
		}
	}
}
func TestADDByteSourceMemoryAllModRMAndSIBWithRegisterAliases(t *testing.T) {
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
							code := []byte{0x02, mod<<6 | byte(index)<<3 | rm}
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
							checkADDByteSource355(t, c, mem, target, index, len(code))
						}
					}
				}
			}
		}
	}
}
func TestADDByteSourceMemoryWrapAndLastByte(t *testing.T) {
	for _, tc := range []struct {
		code         []byte
		base, target uint32
	}{
		{[]byte{0x02, 0xa0, 32, 0, 0, 0}, 0xfffffff0, 528},
		{[]byte{0x02, 0x0d, 255, 1, 0, 0}, 0, 1023},
		{[]byte{0x02, 0x15, 33, 0, 0, 0}, 0, 545},
	} {
		c, mem := xchgByteFixture354(tc.code)
		c.R[EAX] = tc.base
		mem[tc.target] = 0xe7
		checkADDByteSource355(t, c, mem, tc.target, int((tc.code[1]>>3)&7), len(tc.code))
	}
}

func TestADDByteSourceMemoryReadonlyNeverWrites(t *testing.T) {
	for index := 0; index < 8; index++ {
		c, mem := xchgByteFixture354([]byte{0x02, byte(index)<<3 | 5, 32, 0, 0, 0})
		c.SetDescriptor(0x188, Descriptor{Base: 512, Limit: 511})
		mem[544] = 0xff
		bus := &xchgByteBus354{testBus: mem, readFail: ^uint32(0), writeFail: 544}
		c.Bus = bus
		checkADDByteSource355(t, c, mem, 544, index, 6)
		if len(bus.writes) != 0 {
			t.Fatalf("唯讀來源不得寫入：%v", bus.writes)
		}
	}
}
func TestADDByteSourceMemoryFailureDoesNotPublish(t *testing.T) {
	for _, kind := range []string{"未知段", "段外", "線性溢位", "讀失敗"} {
		c, mem := xchgByteFixture354([]byte{0x02, 0x25, 32, 0, 0, 0})
		bus := &xchgByteBus354{testBus: mem, readFail: ^uint32(0), writeFail: ^uint32(0)}
		c.Bus = bus
		switch kind {
		case "未知段":
			c.Seg[SegDS] = 0x200
		case "段外":
			c.SetDescriptor(0x188, Descriptor{Base: 512, Limit: 31, Writable: true})
		case "線性溢位":
			c.SetDescriptor(0x188, Descriptor{Base: 0xfffffff0, Limit: 511, Writable: true})
		case "讀失敗":
			bus.readFail = 544
		}
		want := snapshotInImmediate(c)
		before := append([]byte(nil), mem...)
		if err := c.Step(); err == nil || snapshotInImmediate(c) != want || !bytes.Equal(mem, before) || len(bus.writes) != 0 {
			t.Fatalf("%s不發布狀態：%v", kind, err)
		}
	}
}
func TestADDByteSourceRegister02And22Remain(t *testing.T) {
	for _, op := range []byte{0x02, 0x22} {
		for dst := 0; dst < 8; dst++ {
			for src := 0; src < 8; src++ {
				c, mem := xchgByteFixture354([]byte{op, 0xc0 | byte(dst)<<3 | byte(src)})
				c.R = [8]uint32{0x12345678, 0x9abcdef0, 0x13579bdf, 0x2468ace0, 4, 5, 6, 7}
				a, b := byteValue354(c.R, dst), byteValue354(c.R, src)
				result := byte(int(a) + int(b))
				want := snapshotInImmediate(c)
				want.flags = addByte00Flags(a, b, c.EFlags)
				if op == 0x22 {
					result = a & b
					want.flags = c.EFlags &^ (CF | PF | AF | ZF | SF | OF)
					if result == 0 {
						want.flags |= ZF
					}
					if result >= 128 {
						want.flags |= SF
					}
					if bits.OnesCount8(result)%2 == 0 {
						want.flags |= PF
					}
				}
				want.r = replaceByte354(c.R, dst, result)
				before := append([]byte(nil), mem...)
				if err := c.Step(); err != nil || c.EIP != 2 || snapshotInImmediate(c) != want || !bytes.Equal(mem, before) {
					t.Fatalf("原register op=%X dst=%d src=%d：%v", op, dst, src, err)
				}
			}
		}
	}
	c, mem := xchgByteFixture354([]byte{0x22, 0x05, 32, 0, 0, 0})
	want := snapshotInImmediate(c)
	before := append([]byte(nil), mem...)
	if err := c.Step(); err == nil || snapshotInImmediate(c) != want || !bytes.Equal(mem, before) {
		t.Fatalf("未審查22 memory仍拒絕：%v", err)
	}
}
func TestADDByteSourceMemoryRejectedPrefixesAndTruncation(t *testing.T) {
	code := []byte{0x02, 0x25, 64, 0, 0, 0}
	cases := [][]byte{}
	for n := 0; n < len(code); n++ {
		cases = append(cases, code[:n])
	}
	for _, prefix := range []byte{0x66, 0x26, 0x2e, 0x36, 0x3e, 0x64, 0x65, 0xf0, 0xf2, 0xf3, 0x67} {
		cases = append(cases, append([]byte{prefix}, code...))
	}
	cases = append(cases, []byte{0x02, 0x04}, []byte{0x02, 0x44}, []byte{0x02, 0x84})
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
