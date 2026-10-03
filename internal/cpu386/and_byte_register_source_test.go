package cpu386

import (
	"bytes"
	"encoding/binary"
	"testing"
)

func andByte369Value(a, b byte) byte {
	result := 0
	for bit := 0; bit < 8; bit++ {
		weight := 1 << bit
		if int(a)/weight%2 != 0 && int(b)/weight%2 != 0 {
			result += weight
		}
	}
	return byte(result)
}
func andByte369Flags(result byte, initial uint32) uint32 {
	flags := initial &^ (CF | PF | AF | ZF | SF | OF)
	if result == 0 {
		flags |= ZF
	}
	if result >= 128 {
		flags |= SF
	}
	ones := 0
	for n := int(result); n > 0; n /= 2 {
		ones += n % 2
	}
	if ones%2 == 0 {
		flags |= PF
	}
	return flags
}

func TestANDByte369AllSourcesAndBytePairs(t *testing.T) {
	for src := byte(0); src < 8; src++ {
		for a := 0; a < 256; a++ {
			for b := 0; b < 256; b++ {
				for _, initial := range []uint32{2 | IF | DF | 0x200000, 2 | IF | DF | 0x200000 | CF | PF | AF | ZF | SF | OF} {
					code := []byte{0x20, src<<3 | 5, 32, 0, 0, 0}
					c, mem := subByteMemoryFixture(code)
					host, shift := int(src%4), uint(0)
					if src >= 4 {
						shift = 8
					}
					c.R[host] = c.R[host]&^(0xff<<shift) | uint32(b)<<shift
					c.EFlags = initial
					mem[287], mem[288], mem[289] = 0x55, byte(a), 0xaa
					result := andByte369Value(byte(a), byte(b))
					want := snapshotInImmediate(c)
					want.flags = andByte369Flags(result, initial)
					before := append([]byte(nil), mem...)
					before[288] = result
					if err := c.Step(); err != nil || c.EIP != 6 || snapshotInImmediate(c) != want || !bytes.Equal(mem, before) {
						t.Fatalf("src=%d a=%X b=%X：%v", src, a, b, err)
					}
					if c.EFlags&AF != 0 {
						t.Fatal("AF 清0僅驗工具模型")
					}
				}
			}
		}
	}
}

func TestANDByte369AllModRMAndSIBWithDistinctSegments(t *testing.T) {
	for src := byte(0); src < 8; src++ {
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
							code := []byte{0x20, mod<<6 | src<<3 | rm}
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
							value := byte(c.R[src%4])
							if src >= 4 {
								value = byte(c.R[src%4] >> 8)
							}
							result := andByte369Value(0x81, value)
							before := append([]byte(nil), mem...)
							before[target] = result
							want := snapshotInImmediate(c)
							want.flags = andByte369Flags(result, c.EFlags)
							if err := c.Step(); err != nil || c.EIP != uint32(len(code)) || snapshotInImmediate(c) != want || !bytes.Equal(mem, before) {
								t.Fatalf("src=%d mod=%d rm=%d scale=%d index=%d base=%d：%v", src, mod, rm, scale, index, base, err)
							}
						}
					}
				}
			}
		}
	}
}

func TestANDByte369FailureDoesNotPublish(t *testing.T) {
	for _, kind := range []string{"未知段", "越界", "唯讀", "bus讀失敗", "bus寫失敗"} {
		t.Run(kind, func(t *testing.T) {
			c, mem := subByteMemoryFixture([]byte{0x20, 0x0d, 32, 0, 0, 0})
			mem[288] = 0x80
			bus := &subByteFailureBus{memory: mem, target: 288}
			c.Bus = bus
			switch kind {
			case "未知段":
				c.Seg[SegDS] = 0x200
			case "越界":
				c.SetDescriptor(0x188, Descriptor{Base: 256, Limit: 31, Writable: true})
			case "唯讀":
				c.SetDescriptor(0x188, Descriptor{Base: 256, Limit: 255})
			case "bus讀失敗":
				bus.failRead = true
			case "bus寫失敗":
				bus.failWrite = true
			}
			before := append([]byte(nil), mem...)
			want := snapshotInImmediate(c)
			writes := 0
			if kind == "bus寫失敗" {
				writes = 1
			}
			if err := c.Step(); err == nil || snapshotInImmediate(c) != want || !bytes.Equal(mem, before) || bus.writes != writes {
				t.Fatalf("拒絕需保持全部狀態：%v writes=%d", err, bus.writes)
			}
		})
	}
	for _, full := range [][]byte{{0x20, 0x0d, 32, 0, 0, 0}, {0x20, 0x0c, 0x25, 32, 0, 0, 0}, {0x20, 0x4c, 0x24, 32}, {0x20, 0x8c, 0x24, 32, 0, 0, 0}} {
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
		code := append([]byte{prefix}, []byte{0x20, 0x0d, 32, 0, 0, 0}...)
		c, mem := subByteMemoryFixture(code)
		for _, sel := range c.Seg {
			c.SetDescriptor(sel, Descriptor{Base: 256, Limit: 255, Writable: true})
		}
		mem[288] = 0x80
		before := append([]byte(nil), mem...)
		want := snapshotInImmediate(c)
		if err := c.Step(); err == nil || snapshotInImmediate(c) != want || !bytes.Equal(mem, before) {
			t.Fatalf("前綴 %X：%v", prefix, err)
		}
	}
}

func TestANDByte369KeepsAllRegisterPairs(t *testing.T) {
	for dst := byte(0); dst < 8; dst++ {
		for src := byte(0); src < 8; src++ {
			code := []byte{0x20, 0xc0 | src<<3 | dst}
			c, mem := inImmediateFixture(code)
			want := snapshotInImmediate(c)
			a := byte(want.r[dst%4])
			b := byte(want.r[src%4])
			shift := uint(0)
			if dst >= 4 {
				shift = 8
				a = byte(want.r[dst%4] >> 8)
			}
			if src >= 4 {
				b = byte(want.r[src%4] >> 8)
			}
			result := andByte369Value(a, b)
			want.r[dst%4] = want.r[dst%4]&^(0xff<<shift) | uint32(result)<<shift
			want.flags = andByte369Flags(result, c.EFlags)
			if err := c.Step(); err != nil || c.EIP != 2 || snapshotInImmediate(c) != want || !bytes.Equal(mem, code) {
				t.Fatalf("既有 register dst=%d src=%d：%v", dst, src, err)
			}
		}
	}
}

// register重疊依兩操作數初值核算，涵蓋高低byte與相同目的／來源。
func TestANDByte369RegisterAliasesAndIndependentFlags(t *testing.T) {
	for dst := 0; dst < 8; dst++ {
		for src := 0; src < 8; src++ {
			for _, a := range []byte{0, 1, 0x55, 0x80, 0xff} {
				for _, b := range []byte{0, 1, 0xaa, 0x80, 0xff} {
					for _, initial := range []uint32{2 | IF | DF | 0x200000, 2 | IF | DF | 0x200000 | CF | PF | AF | ZF | SF | OF} {
						c, mem := inImmediateFixture([]byte{0x20, byte(0xc0 | src<<3 | dst)})
						c.setReg8(dst, a)
						c.setReg8(src, b)
						c.EFlags = initial
						shift, srcShift := uint(dst/4*8), uint(src/4*8)
						left, right := byte(c.R[dst%4]>>shift), byte(c.R[src%4]>>srcShift)
						result := andByte369Value(left, right)
						want := snapshotInImmediate(c)
						want.r[dst%4] = want.r[dst%4]&^(0xff<<shift) | uint32(result)<<shift
						want.flags = andByte369Flags(result, initial)
						before := append([]byte(nil), mem...)
						if err := c.Step(); err != nil || c.EIP != 2 || snapshotInImmediate(c) != want || !bytes.Equal(mem, before) {
							t.Fatalf("dst=%d src=%d a=%X b=%X flags=%X：%v", dst, src, a, b, initial, err)
						}
					}
				}
			}
		}
	}
}
func TestANDByte369WrapLastByteAndUnchangedWrite(t *testing.T) {
	for _, v := range []struct {
		code        []byte
		reg, offset uint32
		left, right byte
	}{
		{[]byte{0x20, 0x88, 32, 0, 0, 0}, 0xfffffff0, 272, 1, 4},
		{[]byte{0x20, 0x0d, 255, 0, 0, 0}, 0, 511, 0x81, 0x80},
		{[]byte{0x20, 0x0d, 32, 0, 0, 0}, 0, 288, 0x81, 0xff},
	} {
		c, mem := subByteMemoryFixture(v.code)
		c.R[EAX] = v.reg
		c.R[ECX] = uint32(v.right)
		mem[v.offset] = v.left
		bus := &subByteFailureBus{memory: mem, target: v.offset}
		c.Bus = bus
		result := andByte369Value(v.left, v.right)
		want := snapshotInImmediate(c)
		want.flags = andByte369Flags(result, c.EFlags)
		before := append([]byte(nil), mem...)
		before[v.offset] = result
		if err := c.Step(); err != nil || c.EIP != uint32(len(v.code)) || snapshotInImmediate(c) != want || !bytes.Equal(mem, before) || bus.writes != 1 {
			t.Fatalf("wrap／段末／不變仍單byte寫：%v writes=%d", err, bus.writes)
		}
	}
}

// 原R與20D0是真的；stack資料只為工程POP／RET樣本，不當原版stack收據。
func TestANDByte369OriginalRegistersAndPOPReturn(t *testing.T) {
	c, _ := inImmediateFixture([]byte{0x20, 0xd0, 0x59, 0xc3})
	mem := make(testBus, 0x2bdb10)
	c.Bus = mem
	c.R = [8]uint32{0x18900, 0, 0x601, 0xf, 0x2bdb00, 0x2bdb34, 1, 0xffffffec}
	c.Seg = [6]uint16{8, 0x188, 0x188, 0, 0x20, 0x188}
	c.EFlags = 0x216
	c.EIP = 0x169e49
	c.SetDescriptor(8, Descriptor{Limit: uint32(len(mem) - 1), Writable: true})
	c.SetDescriptor(0x188, Descriptor{Limit: uint32(len(mem) - 1), Writable: true})
	copy(mem[0x169e49:], []byte{0x20, 0xd0, 0x59, 0xc3})
	binary.LittleEndian.PutUint32(mem[0x2bdb00:], 0x12345678)
	binary.LittleEndian.PutUint32(mem[0x2bdb04:], 0x169e60)
	before := append([]byte(nil), mem...)
	want := snapshotInImmediate(c)
	want.flags = 0x246
	if err := c.Step(); err != nil || c.EIP != 0x169e4b || snapshotInImmediate(c) != want || !bytes.Equal(mem, before) {
		t.Fatalf("原20D0：%v", err)
	}
	want.r[ECX] = 0x12345678
	want.r[ESP] += 4
	if err := c.Step(); err != nil || c.EIP != 0x169e4c || snapshotInImmediate(c) != want || !bytes.Equal(mem, before) {
		t.Fatalf("工程POP：%v", err)
	}
	want.r[ESP] += 4
	if err := c.Step(); err != nil || c.EIP != 0x169e60 || snapshotInImmediate(c) != want || !bytes.Equal(mem, before) {
		t.Fatalf("工程RET：%v", err)
	}
}
