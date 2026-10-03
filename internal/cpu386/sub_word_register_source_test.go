package cpu386

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"testing"
)

// 規格359的獨立oracle以整數差、餘數與大小關係計算。
func subWordValue359(dst, src uint16) uint16 {
	d := int64(dst) - int64(src)
	return uint16((d%65536 + 65536) % 65536)
}
func subWordFlags359(dst, src uint16, old uint32) uint32 {
	result := int(subWordValue359(dst, src))
	flags := old &^ (CF | PF | AF | ZF | SF | OF)
	if dst < src {
		flags |= CF
	}
	if dst%16 < src%16 {
		flags |= AF
	}
	signedDst, signedSrc := int(dst), int(src)
	if signedDst >= 32768 {
		signedDst -= 65536
	}
	if signedSrc >= 32768 {
		signedSrc -= 65536
	}
	diff := signedDst - signedSrc
	if diff < -32768 || diff > 32767 {
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

var subWordBounds359 = []uint16{0, 1, 2, 15, 16, 17, 127, 128, 255, 256, 0x7ffe, 0x7fff, 0x8000, 0x8001, 0xfffe, 0xffff}

type subWordBus359 struct {
	testBus
	target uint32
	writes int
}

func (b *subWordBus359) Write8(addr uint32, v byte) error {
	if addr != b.target+uint32(b.writes) || b.writes >= 2 {
		return fmt.Errorf("SUB非目的兩byte寫入")
	}
	b.writes++
	return b.testBus.Write8(addr, v)
}
func subWordMemoryCheck359(t *testing.T, c *CPU, mem testBus, target uint32, size int, srcReg byte) {
	t.Helper()
	dst, src := binary.LittleEndian.Uint16(mem[target:target+2]), uint16(c.R[srcReg])
	want := snapshotInImmediate(c)
	want.flags = subWordFlags359(dst, src, c.EFlags)
	ram := append([]byte(nil), mem...)
	binary.LittleEndian.PutUint16(ram[target:target+2], subWordValue359(dst, src))
	if err := c.Step(); err != nil || c.EIP != uint32(size) || snapshotInImmediate(c) != want || !bytes.Equal(mem, ram) {
		t.Fatalf("SUB target=%X dst=%04X src=%04X err=%v", target, dst, src, err)
	}
}
func TestSUBWordMemoryAllSourcesAndBoundaries(t *testing.T) {
	code := []byte{0x66, 0x29, 0x05, 32, 0, 0, 0}
	c, mem := xchgByteFixture354(code)
	c.R[EAX] = 0xa5b61357
	initial := snapshotInImmediate(c)
	bus := &subWordBus359{testBus: mem, target: 544}
	c.Bus = bus
	for _, flags := range []uint32{2 | IF | DF | 0x200000, 0xa5a52fd7} {
		for _, dst := range subWordBounds359 {
			for value := 0; value < 65536; value++ {
				src := uint16(value)
				c.R = imulWordReplace357(initial.r, EAX, src)
				c.EIP, c.EFlags = 0, flags
				bus.writes = 0
				binary.LittleEndian.PutUint16(mem[544:546], dst)
				want := initial
				want.r = c.R
				want.flags = subWordFlags359(dst, src, flags)
				if err := c.Step(); err != nil || c.EIP != 7 || snapshotInImmediate(c) != want || binary.LittleEndian.Uint16(mem[544:546]) != subWordValue359(dst, src) || bus.writes != 2 {
					t.Fatalf("dst=%04X src=%04X flags=%X err=%v", dst, src, flags, err)
				}
			}
		}
	}
	expected := make([]byte, len(mem))
	copy(expected, code)
	copy(expected[544:546], mem[544:546])
	if !bytes.Equal(mem, expected) {
		t.Fatal("SUB修改相鄰memory")
	}
}
func TestSUBWordRegisterAliasesAndHighWords(t *testing.T) {
	for srcReg := 0; srcReg < 8; srcReg++ {
		for dstReg := 0; dstReg < 8; dstReg++ {
			code := []byte{0x66, 0x29, 0xc0 | byte(srcReg<<3|dstReg)}
			c, mem := xchgByteFixture354(code)
			for i := range c.R {
				c.R[i] = 0xa5b60000 + uint32(i)*0x10000 + 0x1357
			}
			initial := snapshotInImmediate(c)
			check := func(dst, src uint16, flags uint32) {
				c.R = imulWordReplace357(initial.r, dstReg, dst)
				c.R = imulWordReplace357(c.R, srcReg, src)
				c.EIP, c.EFlags = 0, flags
				oldDst, oldSrc := uint16(c.R[dstReg]), uint16(c.R[srcReg])
				want := initial
				want.r = imulWordReplace357(c.R, dstReg, subWordValue359(oldDst, oldSrc))
				want.flags = subWordFlags359(oldDst, oldSrc, flags)
				if err := c.Step(); err != nil || c.EIP != 3 || snapshotInImmediate(c) != want {
					t.Fatalf("srcReg=%d dstReg=%d dst=%04X src=%04X flags=%X err=%v", srcReg, dstReg, oldDst, oldSrc, flags, err)
				}
			}
			for _, flags := range []uint32{2 | IF | DF | 0x200000, 0xa5a52fd7} {
				if srcReg == dstReg {
					for v := 0; v < 65536; v++ {
						check(uint16(v), uint16(v), flags)
					}
				}
				for _, dst := range subWordBounds359 {
					for _, src := range subWordBounds359 {
						check(dst, src, flags)
					}
				}
			}
			expected := make([]byte, len(mem))
			copy(expected, code)
			if !bytes.Equal(mem, expected) {
				t.Fatal("register SUB寫入memory")
			}
		}
	}
}
func TestSUBWordMemoryAllInitialFlagsAndRegisters(t *testing.T) {
	for srcReg := byte(0); srcReg < 8; srcReg++ {
		code := []byte{0x66, 0x29, srcReg<<3 | 5, 32, 0, 0, 0}
		c, mem := xchgByteFixture354(code)
		for pattern := 0; pattern < 64; pattern++ {
			flags := uint32(2 | IF | DF | 0x200000)
			for bit, flag := range []uint32{CF, PF, AF, ZF, SF, OF} {
				if pattern/(1<<bit)%2 != 0 {
					flags |= flag
				}
			}
			for _, dst := range subWordBounds359 {
				for _, src := range subWordBounds359 {
					c.R[srcReg] = 0xa5b60000 | uint32(src)
					c.EIP, c.EFlags = 0, flags
					binary.LittleEndian.PutUint16(mem[544:546], dst)
					subWordMemoryCheck359(t, c, mem, 544, 7, srcReg)
				}
			}
		}
	}
}
func TestSUBWordMemoryAllModRMAndSIB(t *testing.T) {
	for srcReg := byte(0); srcReg < 8; srcReg++ {
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
							code := []byte{0x66, 0x29, mod<<6 | srcReg<<3 | rm}
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
								subWordMemoryCheck359(t, c, mem, target, len(code), srcReg)
							}
						}
					}
				}
			}
		}
	}
}
func TestSUBWordMemoryWrapAndLastWord(t *testing.T) {
	for _, tc := range []struct {
		code         []byte
		base, target uint32
	}{
		{[]byte{0x66, 0x29, 0x80, 32, 0, 0, 0}, 0xfffffff0, 528},
		{[]byte{0x66, 0x29, 0x05, 254, 1, 0, 0}, 0, 1022},
		{[]byte{0x66, 0x29, 0x05, 33, 0, 0, 0}, 0, 545},
	} {
		c, mem := xchgByteFixture354(tc.code)
		c.R[EAX] = tc.base
		binary.LittleEndian.PutUint16(mem[tc.target:], 0x8001)
		subWordMemoryCheck359(t, c, mem, tc.target, len(tc.code), EAX)
	}
}
func TestSUBWordMemoryFailureAndPartialWriteModel(t *testing.T) {
	code := []byte{0x66, 0x29, 0x05, 32, 0, 0, 0}
	for _, kind := range []string{"未知段", "唯讀", "段外", "最後byte", "線性溢位"} {
		c, mem := xchgByteFixture354(code)
		c.R[EAX] = 0xa5b61357
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
			c.R[EAX] = 0xa5b61357
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
			binary.LittleEndian.PutUint16(result[:], subWordValue359(0x1234, 0x1357))
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
func TestSUBWordTruncationPrefixesAndOtherSUB(t *testing.T) {
	for _, full := range [][]byte{{0x66, 0x29, 0xc1}, {0x66, 0x29, 0x05, 32, 0, 0, 0}, {0x66, 0x29, 0x04, 0x25, 32, 0, 0, 0}, {0x66, 0x29, 0x44, 0x24, 0xfd}, {0x66, 0x29, 0x84, 0x24, 32, 0, 0, 0}} {
		for cut := 0; cut < len(full); cut++ {
			c, mem := xchgByteFixture354(full)
			c.R[EAX] = 0xa5b61357
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
		for _, suffix := range [][]byte{{0x66, 0x29, 0xc1}, {0x66, 0x29, 0x05, 32, 0, 0, 0}} {
			code := append([]byte{prefix}, suffix...)
			c, mem := xchgByteFixture354(code)
			for _, seg := range c.Seg {
				c.SetDescriptor(seg, Descriptor{Base: 512, Limit: 511, Writable: true})
			}
			c.R[EAX] = 0xa5b61357
			binary.LittleEndian.PutUint16(mem[544:], 0x1234)
			want := snapshotInImmediate(c)
			ram := append([]byte(nil), mem...)
			if err := c.Step(); err == nil || snapshotInImmediate(c) != want || !bytes.Equal(mem, ram) {
				t.Fatalf("prefix=%02X err=%v", prefix, err)
			}
		}
	}

	// 既有2B word仍支援；只核對不改原分支。
	for _, code := range [][]byte{{0x66, 0x2b, 0xc1}, {0x66, 0x2b, 0x05, 32, 0, 0, 0}} {
		for _, dst := range subWordBounds359 {
			for _, src := range subWordBounds359 {
				c, mem := xchgByteFixture354(code)
				c.R[EAX] = 0xa5b60000 | uint32(dst)
				c.R[ECX] = 0x13570000 | uint32(src)
				binary.LittleEndian.PutUint16(mem[544:], src)
				want := snapshotInImmediate(c)
				want.r = imulWordReplace357(c.R, EAX, subWordValue359(dst, src))
				want.flags = subWordFlags359(dst, src, c.EFlags)
				ram := append([]byte(nil), mem...)
				if err := c.Step(); err != nil || c.EIP != uint32(len(code)) || snapshotInImmediate(c) != want || !bytes.Equal(mem, ram) {
					t.Fatalf("既有2B word err=%v", err)
				}
			}
		}
	}
}
