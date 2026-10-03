package cpu386

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"testing"
)

type rorMemoryBus361 struct {
	testBus
	target uint32
	writes int
}

func (b *rorMemoryBus361) Write8(addr uint32, v byte) error {
	if addr != b.target+uint32(b.writes) || b.writes >= 4 {
		return fmt.Errorf("ROR非目的四byte寫入")
	}
	b.writes++
	return b.testBus.Write8(addr, v)
}
func rorMemoryCheck361(t *testing.T, c *CPU, mem testBus, target uint32, size int, immediate byte) {
	t.Helper()
	value := binary.LittleEndian.Uint32(mem[target : target+4])
	result, flags := rorDwordRegisterOracle(value, immediate, c.EFlags)
	want := snapshotInImmediate(c)
	want.flags = flags
	ram := append([]byte(nil), mem...)
	binary.LittleEndian.PutUint32(ram[target:target+4], result)
	if err := c.Step(); err != nil || c.EIP != uint32(size) || snapshotInImmediate(c) != want || !bytes.Equal(mem, ram) {
		t.Fatalf("ROR target=%X value=%X imm=%d err=%v", target, value, immediate, err)
	}
}
func TestRORDwordMemoryAllCountsFlagsAndBits(t *testing.T) {
	values := []uint32{0, 1, 0xffffffff, 0x7fffffff, 0x80000000, 0x80000001, 0x02000000, 0xffff0000, 0xaaaaaaaa, 0x55555555}
	for bit := uint(0); bit < 32; bit++ {
		values = append(values, uint32(1)<<bit, ^(uint32(1) << bit))
	}
	code := []byte{0xc1, 0x0d, 32, 0, 0, 0, 8}
	c, mem := xchgByteFixture354(code)
	initial := snapshotInImmediate(c)
	bus := &rorMemoryBus361{testBus: mem, target: 544}
	c.Bus = bus
	for imm := 0; imm < 256; imm++ {
		mem[6] = byte(imm)
		for pattern := 0; pattern < 64; pattern++ {
			flags := uint32(2 | IF | DF | 0x200000)
			for bit, flag := range []uint32{CF, PF, AF, ZF, SF, OF} {
				if pattern/(1<<bit)%2 != 0 {
					flags |= flag
				}
			}
			for _, value := range values {
				c.EIP, c.EFlags = 0, flags
				binary.LittleEndian.PutUint32(mem[544:548], value)
				bus.writes = 0
				result, wantFlags := rorDwordRegisterOracle(value, byte(imm), flags)
				want := initial
				want.flags = wantFlags
				wantWrites := 4
				if imm%32 == 0 {
					wantWrites = 0
				}
				if err := c.Step(); err != nil || c.EIP != 7 || snapshotInImmediate(c) != want || binary.LittleEndian.Uint32(mem[544:548]) != result || bus.writes != wantWrites {
					t.Fatalf("value=%X imm=%d flags=%X err=%v writes=%d", value, imm, flags, err, bus.writes)
				}
			}
		}
	}
	expected := make([]byte, len(mem))
	copy(expected, mem[:len(code)])
	copy(expected[544:548], mem[544:548])
	if !bytes.Equal(mem, expected) {
		t.Fatal("ROR修改相鄰memory")
	}
}
func TestRORDwordMemoryAllModRMAndSIB(t *testing.T) {
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
						code := []byte{0xc1, mod<<6 | 0x08 | rm}
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
						for _, imm := range []byte{0, 1, 8, 31, 32, 255} {
							for _, raw := range []uint32{0, 1, 0x80000000, 0xffffffff} {
								c, mem := xchgByteFixture354(append(append([]byte(nil), code...), imm))
								binary.LittleEndian.PutUint32(mem[512+offset:], raw)
								binary.LittleEndian.PutUint32(mem[1024+offset:], raw^0x13571357)
								target := 512 + offset
								if stack {
									target = 1024 + offset
								}
								rorMemoryCheck361(t, c, mem, target, len(code)+1, imm)
							}
						}
					}
				}
			}
		}
	}
}

func TestRORDwordMemoryWrapAndLastDword(t *testing.T) {
	for _, tc := range []struct {
		code         []byte
		base, target uint32
	}{
		{[]byte{0xc1, 0x88, 32, 0, 0, 0, 8}, 0xfffffff0, 528},
		{[]byte{0xc1, 0x0d, 252, 1, 0, 0, 1}, 0, 1020},
		{[]byte{0xc1, 0x0d, 33, 0, 0, 0, 31}, 0, 545},
	} {
		c, mem := xchgByteFixture354(tc.code)
		c.R[EAX] = tc.base
		binary.LittleEndian.PutUint32(mem[tc.target:], 0x80000001)
		rorMemoryCheck361(t, c, mem, tc.target, len(tc.code), tc.code[len(tc.code)-1])
	}
}
func TestRORDwordMemoryFailureAndPartialWriteModel(t *testing.T) {
	for _, imm := range []byte{0, 1, 8} {
		code := []byte{0xc1, 0x0d, 32, 0, 0, 0, imm}
		for _, kind := range []string{"未知段", "唯讀", "段外", "最後三byte", "線性溢位"} {
			c, mem := xchgByteFixture354(code)
			binary.LittleEndian.PutUint32(mem[544:], 0x12345678)
			bus := &xchgByteBus354{testBus: mem, readFail: ^uint32(0), writeFail: ^uint32(0)}
			c.Bus = bus
			switch kind {
			case "未知段":
				c.Seg[SegDS] = 0x200
			case "唯讀":
				c.SetDescriptor(0x188, Descriptor{Base: 512, Limit: 511})
			case "段外":
				c.SetDescriptor(0x188, Descriptor{Base: 512, Limit: 31, Writable: true})
			case "最後三byte":
				binary.LittleEndian.PutUint32(mem[2:6], 509)
			case "線性溢位":
				c.SetDescriptor(0x188, Descriptor{Base: 0xfffffff0, Limit: 511, Writable: true})
			}
			want := snapshotInImmediate(c)
			ram := append([]byte(nil), mem...)
			if err := c.Step(); err == nil || snapshotInImmediate(c) != want || !bytes.Equal(mem, ram) || len(bus.writes) != 0 {
				t.Fatalf("kind=%s imm=%d err=%v", kind, imm, err)
			}
		}
		for byteIndex := uint32(0); byteIndex < 4; byteIndex++ {
			for _, write := range []bool{false, true} {
				c, mem := xchgByteFixture354(code)
				binary.LittleEndian.PutUint32(mem[544:], 0x12345678)
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
				var data [4]byte
				result, _ := rorDwordRegisterOracle(0x12345678, imm, c.EFlags)
				binary.LittleEndian.PutUint32(data[:], result)
				if write && imm != 0 {
					writes = int(byteIndex) + 1
					copy(ram[544:544+byteIndex], data[:byteIndex])
				}
				err := c.Step()
				if write && imm == 0 {
					if err != nil {
						t.Fatalf("count0應無write err=%v", err)
					}
				} else {
					if err == nil {
						t.Fatal("應拒絕每byte read／write失敗")
					}
				}
				if snapshotInImmediate(c) != want || !bytes.Equal(mem, ram) || len(bus.writes) != writes {
					t.Fatalf("imm=%d write=%t byte=%d err=%v writes=%d", imm, write, byteIndex, err, len(bus.writes))
				}
			}
		}
	}
}
func TestRORDwordMemoryTruncationPrefixesAndOtherShapes(t *testing.T) {
	for _, full := range [][]byte{
		{0xc1, 0x0d, 32, 0, 0, 0, 8}, {0xc1, 0x0c, 0x25, 32, 0, 0, 0, 8},
		{0xc1, 0x4c, 0x24, 0xfd, 8}, {0xc1, 0x8c, 0x24, 32, 0, 0, 0, 8},
	} {
		for cut := 0; cut < len(full); cut++ {
			c, mem := xchgByteFixture354(full)
			binary.LittleEndian.PutUint32(mem[544:], 0x12345678)
			bus := &xchgByteBus354{testBus: mem, readFail: uint32(cut), writeFail: ^uint32(0)}
			c.Bus = bus
			want := snapshotInImmediate(c)
			ram := append([]byte(nil), mem...)
			if err := c.Step(); err == nil || snapshotInImmediate(c) != want || !bytes.Equal(mem, ram) || len(bus.writes) != 0 {
				t.Fatalf("cut=%d code=%X err=%v", cut, full, err)
			}
		}
	}
	for _, prefix := range []byte{0x26, 0x2e, 0x36, 0x3e, 0x64, 0x65, 0x66, 0x67, 0xf0, 0xf2, 0xf3} {
		code := append([]byte{prefix}, []byte{0xc1, 0x0d, 32, 0, 0, 0, 8}...)
		c, mem := xchgByteFixture354(code)
		for _, seg := range c.Seg {
			c.SetDescriptor(seg, Descriptor{Base: 512, Limit: 511, Writable: true})
		}
		binary.LittleEndian.PutUint32(mem[544:], 0x12345678)
		want := snapshotInImmediate(c)
		ram := append([]byte(nil), mem...)
		if err := c.Step(); err == nil || snapshotInImmediate(c) != want || !bytes.Equal(mem, ram) {
			t.Fatalf("prefix=%X err=%v", prefix, err)
		}
	}
	for _, group := range []byte{0, 2, 3, 4, 5, 6, 7} {
		code := []byte{0xc1, group<<3 | 5, 32, 0, 0, 0, 8}
		c, mem := xchgByteFixture354(code)
		binary.LittleEndian.PutUint32(mem[544:], 0x12345678)
		want := snapshotInImmediate(c)
		ram := append([]byte(nil), mem...)
		if err := c.Step(); err == nil || snapshotInImmediate(c) != want || !bytes.Equal(mem, ram) {
			t.Fatalf("group=%d err=%v", group, err)
		}
	}
	for _, code := range [][]byte{{0xd1, 0x0d, 32, 0, 0, 0}, {0xd3, 0x0d, 32, 0, 0, 0}, {0xd3, 0xc8}} {
		c, mem := xchgByteFixture354(code)
		binary.LittleEndian.PutUint32(mem[544:], 0x12345678)
		want := snapshotInImmediate(c)
		ram := append([]byte(nil), mem...)
		if err := c.Step(); err == nil || snapshotInImmediate(c) != want || !bytes.Equal(mem, ram) {
			t.Fatalf("D1／D3 code=%X err=%v", code, err)
		}
	}
}
