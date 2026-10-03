package cpu386

import (
	"bytes"
	"encoding/binary"
	"testing"
)

// 規格372：有號範圍及重複加法oracle，不使用production乘法／截斷判準。
func imulByteProducts372() [256][256]int {
	var products [256][256]int
	for a := 0; a < 256; a++ {
		left := a
		if left >= 128 {
			left -= 256
		}
		for b := 0; b < 256; b++ {
			right := b
			if right >= 128 {
				right -= 256
			}
			value := 0
			if right >= 0 {
				for j := 0; j < right; j++ {
					value += left
				}
			} else {
				for j := 0; j > right; j-- {
					value -= left
				}
			}
			products[a][b] = value
		}
	}
	return products
}
func imulByteLane372(r [8]uint32, reg int, lane int, raw byte) [8]uint32 {
	var data [4]byte
	binary.LittleEndian.PutUint32(data[:], r[reg])
	data[lane] = raw
	r[reg] = binary.LittleEndian.Uint32(data[:])
	return r
}
func imulByteWant372(s inImmediateState, product int) inImmediateState {
	raw := (product%65536 + 65536) % 65536
	var data [4]byte
	binary.LittleEndian.PutUint32(data[:], s.r[EAX])
	binary.LittleEndian.PutUint16(data[:2], uint16(raw))
	s.r[EAX] = binary.LittleEndian.Uint32(data[:])
	s.flags &^= CF | OF
	if product < -128 || product > 127 {
		s.flags |= CF | OF
	}
	return s
}
func TestIMULByte372AllRegisterAliasesAndValues(t *testing.T) {
	products := imulByteProducts372()
	for src := 0; src < 8; src++ {
		code := []byte{0xf6, 0xe8 | byte(src)}
		c, mem := inImmediateFixture(code)
		bus := &imulWordReadBus357{testBus: mem}
		c.Bus = bus
		initial := snapshotInImmediate(c)
		for _, flag := range []uint32{2 | IF | DF | 0x200000, 0xa5a52fd7} {
			for a := 0; a < 256; a++ {
				for b := 0; b < 256; b++ {
					c.EIP = 0
					c.R = initial.r
					c.EFlags = flag
					c.R = imulByteLane372(c.R, EAX, 0, byte(a))
					c.R = imulByteLane372(c.R, src%4, src/4, byte(b))
					// AL來源與目的重疊時，真正舊AL亦為b。
					left := a
					if src == 0 {
						left = b
					}
					want := imulByteWant372(snapshotInImmediate(c), products[left][b])
					if err := c.Step(); err != nil || c.EIP != 2 || snapshotInImmediate(c) != want || !bytes.Equal(mem, code) || bus.writes != 0 {
						t.Fatalf("src=%d AL=%X source=%X flags=%X err=%v", src, left, b, flag, err)
					}
				}
			}
		}
	}
}
func TestIMULByte372AllMemoryValuesReadonlyAndNoWrites(t *testing.T) {
	products := imulByteProducts372()
	code := []byte{0xf6, 0x2d, 32, 0, 0, 0}
	c, mem := subByteMemoryFixture(code)
	c.SetDescriptor(0x188, Descriptor{Base: 256, Limit: 255})
	bus := &imulWordReadBus357{testBus: mem}
	c.Bus = bus
	initial := snapshotInImmediate(c)
	for _, flag := range []uint32{2 | IF | DF | 0x200000, 0xa5a52fd7} {
		for a := 0; a < 256; a++ {
			for b := 0; b < 256; b++ {
				c.EIP = 0
				c.R = initial.r
				c.EFlags = flag
				c.R = imulByteLane372(c.R, EAX, 0, byte(a))
				mem[288] = byte(b)
				before := append([]byte(nil), mem...)
				want := imulByteWant372(snapshotInImmediate(c), products[a][b])
				if err := c.Step(); err != nil || c.EIP != 6 || snapshotInImmediate(c) != want || !bytes.Equal(mem, before) || bus.writes != 0 {
					t.Fatalf("AL=%X memory=%X flags=%X err=%v", a, b, flag, err)
				}
			}
		}
	}
}
func TestIMULByte372AllModRMAndSIBWithDistinctSegments(t *testing.T) {
	for src := byte(5); src < 6; src++ {
		for mod := byte(0); mod < 3; mod++ {
			for rm := byte(0); rm < 8; rm++ {
				scales, indexes, bases := []byte{0}, []byte{4}, []byte{rm}
				if rm == 4 {
					scales = []byte{0, 1, 2, 3}
					indexes = []byte{0, 1, 2, 3, 4, 5, 6, 7}
					bases = []byte{0, 1, 2, 3, 4, 5, 6, 7}
				}
				for _, scale := range scales {
					for _, index := range indexes {
						for _, base := range bases {
							code := []byte{0xf6, mod<<6 | src<<3 | rm}
							if rm == 4 {
								code = append(code, scale<<6|index<<3|base)
							}
							noBase := mod == 0 && base == 5
							offset := uint32(16 + base)
							stack := base == 4 || base == 5
							if noBase {
								offset = 32
								stack = false
							}
							if rm == 4 && index != 4 {
								offset += uint32(16+index) * uint32(1<<scale)
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
							c, mem := subByteMemoryFixture(code)
							for i := range c.R {
								c.R[i] = uint32(16 + i)
							}
							ds, ss := 256+offset, 768+offset
							mem[ds], mem[ss] = 0x66, 0x77
							target := ds
							if stack {
								target = ss
							}
							mem[target] = 0x81
							before := append([]byte(nil), mem...)
							want := imulByteWant372(snapshotInImmediate(c), -127*16)
							bus := &imulWordReadBus357{testBus: mem}
							c.Bus = bus
							if err := c.Step(); err != nil || c.EIP != uint32(len(code)) || snapshotInImmediate(c) != want || !bytes.Equal(mem, before) || bus.writes != 0 {
								t.Fatalf("src=%d mod=%d rm=%d scale=%d index=%d base=%d：%v", src, mod, rm, scale, index, base, err)
							}
						}
					}
				}
			}
		}
	}
}

func TestIMULByte372FailureDoesNotPublish(t *testing.T) {
	for _, kind := range []string{"未知段", "越界", "bus讀失敗"} {
		t.Run(kind, func(t *testing.T) {
			c, mem := subByteMemoryFixture([]byte{0xf6, 0x2d, 32, 0, 0, 0})
			mem[288] = 0x80
			bus := &subByteFailureBus{memory: mem, target: 288}
			c.Bus = bus
			switch kind {
			case "未知段":
				c.Seg[SegDS] = 0x200
			case "越界":
				c.SetDescriptor(0x188, Descriptor{Base: 256, Limit: 31})
			case "bus讀失敗":
				bus.failRead = true
			}
			before := append([]byte(nil), mem...)
			want := snapshotInImmediate(c)
			if err := c.Step(); err == nil || snapshotInImmediate(c) != want || !bytes.Equal(mem, before) || bus.writes != 0 {
				t.Fatalf("拒絕發布或寫RAM：%v writes=%d", err, bus.writes)
			}
		})
	}
	for _, full := range [][]byte{{0xf6, 0xec}, {0xf6, 0x2d, 32, 0, 0, 0}, {0xf6, 0x2c, 0x25, 32, 0, 0, 0}, {0xf6, 0x6c, 0x24, 32}, {0xf6, 0xac, 0x24, 32, 0, 0, 0}} {
		for n := 0; n < len(full); n++ {
			code := append([]byte(nil), full[:n]...)
			c, mem := inImmediateFixture(code)
			want := snapshotInImmediate(c)
			if err := c.Step(); err == nil || snapshotInImmediate(c) != want || !bytes.Equal(mem, code) {
				t.Fatalf("截短 %X：%v", code, err)
			}
		}
	}
	for _, prefix := range []byte{0x66, 0x67, 0x26, 0x2e, 0x36, 0x3e, 0x64, 0x65, 0xf2, 0xf3, 0xf0} {
		for _, body := range [][]byte{{0xf6, 0xec}, {0xf6, 0x2d, 32, 0, 0, 0}} {
			code := append([]byte{prefix}, body...)
			c, mem := subByteMemoryFixture(code)
			want := snapshotInImmediate(c)
			before := append([]byte(nil), mem...)
			if err := c.Step(); err == nil || snapshotInImmediate(c) != want || !bytes.Equal(mem, before) {
				t.Fatalf("前綴 %X：%v", prefix, err)
			}
		}
	}
}
func TestIMULByte372WrapAndLastByte(t *testing.T) {
	products := imulByteProducts372()
	for _, v := range []struct {
		code        []byte
		reg, offset uint32
		source      byte
	}{
		{[]byte{0xf6, 0xa8, 32, 0, 0, 0}, 0xfffffff0, 272, 0x80},
		{[]byte{0xf6, 0x2d, 255, 0, 0, 0}, 0xffff0001, 511, 0xff},
	} {
		c, mem := subByteMemoryFixture(v.code)
		c.R[EAX] = v.reg
		mem[v.offset] = v.source
		bus := &imulWordReadBus357{testBus: mem}
		c.Bus = bus
		before := append([]byte(nil), mem...)
		want := imulByteWant372(snapshotInImmediate(c), products[byte(v.reg)][v.source])
		if err := c.Step(); err != nil || c.EIP != uint32(len(v.code)) || snapshotInImmediate(c) != want || !bytes.Equal(mem, before) || bus.writes != 0 {
			t.Fatalf("wrap／段末：%v", err)
		}
	}
}

// 原R／bytes是真的；目標舊byte55為合成fixture，不冒充原native。
func TestIMULByte372OriginalRegistersStoreAndJump(t *testing.T) {
	c, _ := inImmediateFixture([]byte{0xf6, 0xec})
	mem := make(testBus, 0x2bd500)
	c.Bus = mem
	c.R = [8]uint32{0xff01, 0x1a5, 2, 8, 0x2bd488, 0x2bd4e0, 0x171c80, 0x2bd4e0}
	c.Seg = [6]uint16{8, 0x188, 0x188, 0, 0x20, 0x188}
	c.EFlags = 0x246
	c.FPUControl = 0x127f
	c.FPUStatus = 0
	c.FPUDepth = 0
	c.FPUStack = [8]float64{}
	c.EIP = 0x1749c0
	c.SetDescriptor(8, Descriptor{Limit: uint32(len(mem) - 1), Writable: true})
	c.SetDescriptor(0x188, Descriptor{Limit: uint32(len(mem) - 1), Writable: true})
	copy(mem[c.EIP:], []byte{0xf6, 0xec, 0xa2, 6, 0x1f, 0x28, 0, 0xe9, 0x33, 0xf3, 0xff, 0xff})
	mem[0x281f06] = 0x55
	before := append([]byte(nil), mem...)
	want := snapshotInImmediate(c)
	want.r[EAX] = 0xffff
	if err := c.Step(); err != nil || c.EIP != 0x1749c2 || snapshotInImmediate(c) != want || !bytes.Equal(mem, before) {
		t.Fatalf("IMUL：%v", err)
	}
	before[0x281f06] = 0xff
	if err := c.Step(); err != nil || c.EIP != 0x1749c7 || snapshotInImmediate(c) != want || !bytes.Equal(mem, before) {
		t.Fatalf("工程A2：%v", err)
	}
	if err := c.Step(); err != nil || c.EIP != 0x173cff || snapshotInImmediate(c) != want || !bytes.Equal(mem, before) {
		t.Fatalf("工程E9：%v", err)
	}
}
