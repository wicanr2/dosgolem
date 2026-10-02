package cpu386

import (
	"bytes"
	"encoding/binary"
	"math/bits"
	"testing"
)

// 規格324：以較寬和、低位進位、有號範圍與popcount建立獨立預期。
func addByte00Flags(a, b byte, initial uint32) uint32 {
	flags := initial &^ (CF | PF | AF | ZF | SF | OF)
	wide := int(a) + int(b)
	result := byte(wide)
	if wide > 255 {
		flags |= CF
	}
	if int(a%16)+int(b%16) > 15 {
		flags |= AF
	}
	signed := int(int8(a)) + int(int8(b))
	if signed < -128 || signed > 127 {
		flags |= OF
	}
	if result == 0 {
		flags |= ZF
	}
	if int8(result) < 0 {
		flags |= SF
	}
	if bits.OnesCount8(result)%2 == 0 {
		flags |= PF
	}
	return flags
}

func TestByteADD00AllPairsAndIndependentFlags(t *testing.T) {
	for a := 0; a < 256; a++ {
		for b := 0; b < 256; b++ {
			for _, initial := range []uint32{2 | IF | DF | 0x200000, 2 | IF | DF | 0x200000 | CF | PF | AF | ZF | SF | OF} {
				c, mem := inImmediateFixture([]byte{0x00, 0xc3})
				c.R[EBX] = 0x12345600 | uint32(a)
				c.R[EAX] = 0xabcdef00 | uint32(b)
				c.EFlags = initial
				want := snapshotInImmediate(c)
				want.r[EBX] = 0x12345600 | uint32(byte(a+b))
				want.flags = addByte00Flags(byte(a), byte(b), initial)
				if err := c.Step(); err != nil || c.EIP != 2 || snapshotInImmediate(c) != want || !bytes.Equal(mem, []byte{0x00, 0xc3}) {
					t.Fatalf("a=%X b=%X initial=%X：%v", a, b, initial, err)
				}
			}
		}
	}
}

func TestByteADD00AllRegisterAliases(t *testing.T) {
	for dst := 0; dst < 8; dst++ {
		for src := 0; src < 8; src++ {
			for _, a := range []byte{0, 1, 15, 16, 127, 128, 255} {
				for _, b := range []byte{0, 1, 15, 16, 127, 128, 255} {
					c, mem := inImmediateFixture([]byte{0x00, byte(0xc0 | src<<3 | dst)})
					c.setReg8(dst, a)
					c.setReg8(src, b)
					dshift, sshift := uint(dst/4*8), uint(src/4*8)
					left, right := byte(c.R[dst%4]>>dshift), byte(c.R[src%4]>>sshift)
					want := snapshotInImmediate(c)
					want.r[dst%4] = want.r[dst%4]&^(0xff<<dshift) | uint32(byte(int(left)+int(right)))<<dshift
					want.flags = addByte00Flags(left, right, c.EFlags)
					before := append([]byte(nil), mem...)
					if err := c.Step(); err != nil || c.EIP != 2 || snapshotInImmediate(c) != want || !bytes.Equal(mem, before) {
						t.Fatalf("dst=%d src=%d：%v", dst, src, err)
					}
				}
			}
		}
	}
}

func TestByteADD00MemoryAllPairsAndIndependentFlags(t *testing.T) {
	for a := 0; a < 256; a++ {
		for b := 0; b < 256; b++ {
			for _, initial := range []uint32{2 | IF | DF | 0x200000, 2 | IF | DF | 0x200000 | CF | PF | AF | ZF | SF | OF} {
				c, mem := subByteMemoryFixture([]byte{0x00, 0x1d, 32, 0, 0, 0})
				c.R[EBX] = 0x12345600 | uint32(b)
				c.EFlags = initial
				mem[287], mem[288], mem[289] = 0x55, byte(a), 0xaa
				want := snapshotInImmediate(c)
				want.flags = addByte00Flags(byte(a), byte(b), initial)
				before := append([]byte(nil), mem...)
				before[288] = byte(a + b)
				if err := c.Step(); err != nil || c.EIP != 6 || snapshotInImmediate(c) != want || !bytes.Equal(mem, before) {
					t.Fatalf("a=%X b=%X：%v", a, b, err)
				}
			}
		}
	}
}

func TestByteADD00OriginalRegistersMOVSXAndINC(t *testing.T) {
	mem := make(testBus, 0x2bdc00)
	copy(mem[0x17122b:], []byte{0x00, 0xc3, 0x0f, 0xbf, 0xc2, 0x42})
	c := New(mem)
	c.EIP = 0x17122b
	c.R = [8]uint32{0, 0, 1, 0, 0x2bdb50, 0x2bdb84, 0x2bdb68, 0x2bdb68}
	c.Seg = [6]uint16{8, 0x188, 0x188, 0, 0x20, 0x188}
	c.EFlags = 0x247
	c.SetDescriptor(8, Descriptor{Limit: uint32(len(mem) - 1)})
	want := snapshotInImmediate(c)
	want.flags = 0x246
	if err := c.Step(); err != nil || c.EIP != 0x17122d || snapshotInImmediate(c) != want {
		t.Fatalf("原始ADD暫存器形狀：%v", err)
	}
	want.r[EAX] = 1
	if err := c.Step(); err != nil || c.EIP != 0x171230 || snapshotInImmediate(c) != want {
		t.Fatalf("MOVSX EAX,DX：%v", err)
	}
	want.r[EDX] = 2
	want.flags = 0x202
	if err := c.Step(); err != nil || c.EIP != 0x171231 || snapshotInImmediate(c) != want {
		t.Fatalf("INC EDX：%v", err)
	}
}

func TestByteADD00MemoryAllModRMAndSIBWithDistinctSegments(t *testing.T) {
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
							code := []byte{0x00, mod<<6 | src<<3 | rm}
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
							result := byte(int(0x81) + int(value))
							before := append([]byte(nil), mem...)
							before[target] = result
							want := snapshotInImmediate(c)
							want.flags = addByte00Flags(0x81, value, c.EFlags)
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

func TestByteADD00MemoryFailureDoesNotPublish(t *testing.T) {
	for _, kind := range []string{"未知段", "越界", "唯讀", "bus讀失敗", "bus寫失敗"} {
		t.Run(kind, func(t *testing.T) {
			c, mem := subByteMemoryFixture([]byte{0x00, 0x0d, 32, 0, 0, 0})
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
	for _, full := range [][]byte{{0x00, 0x0d, 32, 0, 0, 0}, {0x00, 0x0c, 0x25, 32, 0, 0, 0}, {0x00, 0x4c, 0x24, 32}, {0x00, 0x8c, 0x24, 32, 0, 0, 0}} {
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
		code := append([]byte{prefix}, []byte{0x00, 0x0d, 32, 0, 0, 0}...)
		c, mem := subByteMemoryFixture(code)
		for _, sel := range c.Seg {
			c.SetDescriptor(sel, Descriptor{Base: 256, Limit: 255, Writable: true})
		}
		// 指令仍從原CS取出，避免負例只因讀錯code而拒絕。
		c.SetDescriptor(8, Descriptor{Limit: uint32(len(mem) - 1)})
		mem[288] = 0x80
		before := append([]byte(nil), mem...)
		want := snapshotInImmediate(c)
		if err := c.Step(); err == nil || snapshotInImmediate(c) != want || !bytes.Equal(mem, before) {
			t.Fatalf("前綴 %X：%v", prefix, err)
		}
	}
}

func TestByteADD00MemoryWrapAndLastByte(t *testing.T) {
	c, mem := subByteMemoryFixture([]byte{0x00, 0x88, 32, 0, 0, 0})
	c.R[EAX] = 0xfffffff0
	c.R[ECX] = 4
	mem[272] = 1
	want := snapshotInImmediate(c)
	want.flags = addByte00Flags(1, 4, c.EFlags)
	before := append([]byte(nil), mem...)
	before[272] = 5
	if err := c.Step(); err != nil || c.EIP != 6 || snapshotInImmediate(c) != want || !bytes.Equal(mem, before) {
		t.Fatalf("32位地址繞回：%v", err)
	}
	c, mem = subByteMemoryFixture([]byte{0x00, 0x0d, 255, 0, 0, 0})
	c.R[ECX] = 0x80
	mem[511] = 1
	want = snapshotInImmediate(c)
	want.flags = addByte00Flags(1, 0x80, c.EFlags)
	before = append([]byte(nil), mem...)
	before[511] = 0x81
	if err := c.Step(); err != nil || c.EIP != 6 || snapshotInImmediate(c) != want || !bytes.Equal(mem, before) {
		t.Fatalf("段末單 byte：%v", err)
	}
}
