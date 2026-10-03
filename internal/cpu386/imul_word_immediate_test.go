package cpu386

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"testing"
)

// 規格357用signed範圍與little-endian視圖，獨立於CPU的截斷判準。
func imulWordSigned357(raw uint16) int64 {
	v := int64(raw)
	if v >= 32768 {
		v -= 65536
	}
	return v
}
func imulWordReplace357(r [8]uint32, reg int, raw uint16) [8]uint32 {
	var lane [4]byte
	binary.LittleEndian.PutUint32(lane[:], r[reg])
	binary.LittleEndian.PutUint16(lane[:2], raw)
	r[reg] = binary.LittleEndian.Uint32(lane[:])
	return r
}
func imulWordWant357(before inImmediateState, reg int, source uint16, imm int64) inImmediateState {
	product := imulWordSigned357(source) * imm
	raw := (product%65536 + 65536) % 65536
	before.r = imulWordReplace357(before.r, reg, uint16(raw))
	before.flags &^= CF | OF
	if product < -32768 || product > 32767 {
		before.flags |= CF | OF
	}
	return before
}

type imulWordReadBus357 struct {
	testBus
	writes int
}

func (b *imulWordReadBus357) Write8(addr uint32, value byte) error {
	b.writes++
	return fmt.Errorf("IMUL不可寫入memory")
}
func imulWordAppendImm357(code []byte, op byte, imm int64) []byte {
	if op == 0x6b {
		return append(code, byte(imm))
	}
	return binary.LittleEndian.AppendUint16(code, uint16(imm))
}
func imulWordCheck357(t *testing.T, c *CPU, mem testBus, reg int, source uint16, imm int64, size int) {
	t.Helper()
	expected := imulWordWant357(snapshotInImmediate(c), reg, source, imm)
	ram := append([]byte(nil), mem...)
	if err := c.Step(); err != nil || c.EIP != uint32(size) || snapshotInImmediate(c) != expected || !bytes.Equal(mem, ram) {
		t.Fatalf("word IMUL reg=%d source=%04X imm=%d eip=%X R=%X flags=%X err=%v", reg, source, imm, c.EIP, c.R, c.EFlags, err)
	}
}
func TestIMULWordImmediateAllBytePairsAndWordImmediates(t *testing.T) {
	boundaries := []uint16{0x8000, 0x8001, 0xff00, 0xff7f, 0xff80, 0xfffe, 0xffff, 0, 1, 2, 127, 128, 255, 256, 0x7ffe, 0x7fff}
	for _, op := range []byte{0x6b, 0x69} {
		code := imulWordAppendImm357([]byte{0x66, op, 0x3d, 32, 0, 0, 0}, op, 0)
		c, mem := xchgByteFixture354(code)
		c.R[EDI] = 0x5aa5a5f4
		c.SetDescriptor(0x188, Descriptor{Base: 512, Limit: 511})
		bus := &imulWordReadBus357{testBus: mem}
		c.Bus = bus
		initial := snapshotInImmediate(c)
		flags := []uint32{2 | IF | DF | 0x200000, 0xa5a52fd7}
		for _, flag := range flags {
			for raw := 0; raw < 65536; raw++ {
				sources := boundaries
				if op == 0x6b {
					sources = []uint16{uint16(raw)}
				}
				count := 1
				if op == 0x6b {
					count = 256
				}
				for _, source := range sources {
					binary.LittleEndian.PutUint16(mem[544:546], source)
					for ib := 0; ib < count; ib++ {
						immRaw := raw
						if op == 0x6b {
							immRaw = ib
							mem[7] = byte(ib)
						} else {
							binary.LittleEndian.PutUint16(mem[7:9], uint16(raw))
						}
						imm := int64(immRaw)
						if op == 0x6b && imm >= 128 {
							imm -= 256
						}
						if op == 0x69 && imm >= 32768 {
							imm -= 65536
						}
						c.EIP, c.EFlags = 0, flag
						// 保留每次結果的高word，同時核對全部R／段／nonzero FPU。
						expected := initial
						expected.flags = flag
						expected = imulWordWant357(expected, EDI, source, imm)
						if err := c.Step(); err != nil || c.EIP != uint32(len(code)) || snapshotInImmediate(c) != expected || bus.writes != 0 {
							t.Fatalf("op=%02X source=%04X imm=%d flags=%X err=%v", op, source, imm, flag, err)
						}
					}
				}
			}
		}
		ram := append([]byte(nil), code...)
		copy(ram, mem[:len(code)])
		expectedMemory := make([]byte, len(mem))
		copy(expectedMemory, ram)
		copy(expectedMemory[544:546], mem[544:546])
		if !bytes.Equal(mem, expectedMemory) || bus.writes != 0 {
			t.Fatal("IMUL修改來源／相鄰memory")
		}
	}
}
func TestIMULWordImmediateAllRegisterAliases(t *testing.T) {
	boundaries := []uint16{0x8000, 0x8001, 0xff7f, 0xff80, 0xffff, 0, 1, 127, 128, 0x7fff}
	for _, op := range []byte{0x6b, 0x69} {
		immediates := []int64{-128, -2, -1, 0, 1, 2, 127}
		if op == 0x69 {
			immediates = append(immediates, -32768, -32767, 128, 32766, 32767)
		}
		for reg := 0; reg < 8; reg++ {
			for sourceReg := 0; sourceReg < 8; sourceReg++ {
				for _, source := range boundaries {
					for _, imm := range immediates {
						code := imulWordAppendImm357([]byte{0x66, op, 0xc0 | byte(reg)<<3 | byte(sourceReg)}, op, imm)
						c, mem := xchgByteFixture354(code)
						for i := range c.R {
							c.R[i] = 0x12340000 + uint32(i)*0x10001 + 0x5678
						}
						c.R = imulWordReplace357(c.R, sourceReg, source)
						imulWordCheck357(t, c, mem, reg, source, imm, len(code))
					}
				}
			}
		}
	}
}
func TestIMULWordImmediateAllModRMAndSIBAliases(t *testing.T) {
	for _, op := range []byte{0x6b, 0x69} {
		for reg := 0; reg < 8; reg++ {
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
								code := []byte{0x66, op, mod<<6 | byte(reg)<<3 | rm}
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
								code = imulWordAppendImm357(code, op, -2)
								c, mem := xchgByteFixture354(code)
								binary.LittleEndian.PutUint16(mem[512+offset:], 0x8123)
								binary.LittleEndian.PutUint16(mem[1024+offset:], 0x7fff)
								source := uint16(0x8123)
								if stack {
									source = 0x7fff
								}
								imulWordCheck357(t, c, mem, reg, source, -2, len(code))
							}
						}
					}
				}
			}
		}
	}
}
func TestIMULWordImmediateReadonlyLastWordAndAddressWrap(t *testing.T) {
	for _, op := range []byte{0x6b, 0x69} {
		for _, tc := range []struct {
			code         []byte
			base, target uint32
		}{
			{[]byte{0x66, op, 0x38}, 32, 544},
			{[]byte{0x66, op, 0x3d, 254, 1, 0, 0}, 0, 1022},
			{[]byte{0x66, op, 0xb8, 32, 0, 0, 0}, 0xfffffff0, 528},
		} {
			code := imulWordAppendImm357(tc.code, op, -1)
			c, mem := xchgByteFixture354(code)
			c.R[EAX] = tc.base
			c.SetDescriptor(0x188, Descriptor{Base: 512, Limit: 511})
			binary.LittleEndian.PutUint16(mem[tc.target:], 0x8000)
			imulWordCheck357(t, c, mem, int(tc.code[2]>>3&7), 0x8000, -1, len(code))
		}
	}
}
func TestIMULWordImmediateSourceFailureDoesNotPublish(t *testing.T) {
	for _, op := range []byte{0x6b, 0x69} {
		for _, kind := range []string{"未知段", "段外", "最後byte", "線性溢位", "讀第一byte失敗", "讀第二byte失敗"} {
			code := imulWordAppendImm357([]byte{0x66, op, 0x3d, 32, 0, 0, 0}, op, -2)
			c, mem := xchgByteFixture354(code)
			bus := &xchgByteBus354{testBus: mem, readFail: ^uint32(0), writeFail: ^uint32(0)}
			c.Bus = bus
			binary.LittleEndian.PutUint16(mem[544:], 0x8000)
			switch kind {
			case "未知段":
				c.Seg[SegDS] = 0x200
			case "段外":
				c.SetDescriptor(0x188, Descriptor{Base: 512, Limit: 31})
			case "最後byte":
				binary.LittleEndian.PutUint32(mem[3:7], 511)
			case "線性溢位":
				c.SetDescriptor(0x188, Descriptor{Base: 0xfffffff0, Limit: 511})
			case "讀第一byte失敗":
				bus.readFail = 544
			case "讀第二byte失敗":
				bus.readFail = 545
			}
			want := snapshotInImmediate(c)
			ram := append([]byte(nil), mem...)
			if err := c.Step(); err == nil || snapshotInImmediate(c) != want || !bytes.Equal(mem, ram) || len(bus.writes) != 0 {
				t.Fatalf("op=%02X %s err=%v", op, kind, err)
			}
		}
	}
}
func TestIMULWordImmediateTruncationDoesNotPublish(t *testing.T) {
	for _, op := range []byte{0x6b, 0x69} {
		for _, head := range [][]byte{{0x66, op, 0xc1}, {0x66, op, 0x3d, 32, 0, 0, 0}, {0x66, op, 0xbc, 0x24, 32, 0, 0, 0}} {
			code := imulWordAppendImm357(head, op, -2)
			for cut := 0; cut < len(code); cut++ {
				c, mem := xchgByteFixture354(code)
				binary.LittleEndian.PutUint16(mem[544:], 0x8000)
				// 取指直接讀Bus；只拒絕第一個缺byte，來源memory仍完整可讀。
				bus := &xchgByteBus354{testBus: mem, readFail: uint32(cut), writeFail: ^uint32(0)}
				c.Bus = bus
				want := snapshotInImmediate(c)
				ram := append([]byte(nil), mem...)
				if err := c.Step(); err == nil || snapshotInImmediate(c) != want || !bytes.Equal(mem, ram) {
					t.Fatalf("op=%02X cut=%d err=%v", op, cut, err)
				}
			}
		}
	}
}
func TestIMULWordImmediateUnsupportedPrefixes(t *testing.T) {
	for _, op := range []byte{0x6b, 0x69} {
		for _, prefix := range []byte{0x26, 0x2e, 0x36, 0x3e, 0x64, 0x65, 0x67, 0xf0, 0xf2, 0xf3} {
			for _, head := range [][]byte{{prefix, 0x66, op, 0xc1}, {prefix, 0x66, op, 0x3d, 32, 0, 0, 0}} {
				code := imulWordAppendImm357(head, op, 5)
				c, mem := xchgByteFixture354(code)
				for _, seg := range c.Seg {
					if seg != c.Seg[SegCS] {
						c.SetDescriptor(seg, Descriptor{Base: 512, Limit: 511, Writable: true})
					}
				}
				binary.LittleEndian.PutUint16(mem[32:], 3)
				binary.LittleEndian.PutUint16(mem[544:], 3)
				c.R[ECX] = 3
				want := snapshotInImmediate(c)
				ram := append([]byte(nil), mem...)
				if err := c.Step(); err == nil || snapshotInImmediate(c) != want || !bytes.Equal(mem, ram) {
					t.Fatalf("op=%02X prefix=%02X err=%v", op, prefix, err)
				}
			}
		}
	}
}
