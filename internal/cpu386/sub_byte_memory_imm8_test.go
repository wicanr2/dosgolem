package cpu386

import (
	"bytes"
	"encoding/binary"
	"errors"
	"math/bits"
	"testing"
)

// 用較寬差值、低 nibble 借位與有號界限建立獨立預期，不呼叫 sub8。
func subByteMemoryFlags(a, b byte, initial uint32) uint32 {
	difference := int(a) - int(b)
	result := byte(difference)
	flags := initial &^ (CF | PF | AF | ZF | SF | OF)
	if difference < 0 {
		flags |= CF
	}
	if a&15 < b&15 {
		flags |= AF
	}
	signed := int(int8(a)) - int(int8(b))
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

func subByteMemoryFixture(code []byte) (*CPU, testBus) {
	c, _ := inImmediateFixture(code)
	mem := make(testBus, 1024)
	copy(mem, code)
	c.Bus = mem
	c.Seg[SegDS], c.Seg[SegSS] = 0x188, 0x190
	c.SetDescriptor(0x188, Descriptor{Base: 256, Limit: 255, Writable: true})
	c.SetDescriptor(0x190, Descriptor{Base: 768, Limit: 255, Writable: true})
	return c, mem
}

func TestSUBByteMemoryAllValuesAndIndependentFlags(t *testing.T) {
	for a := 0; a < 256; a++ {
		for b := 0; b < 256; b++ {
			for _, initial := range []uint32{2 | IF | DF | 0x200000, 2 | IF | DF | 0x200000 | CF | PF | AF | ZF | SF | OF} {
				code := []byte{0x80, 0x2d, 32, 0, 0, 0, byte(b)}
				c, mem := subByteMemoryFixture(code)
				c.EFlags = initial
				mem[287], mem[288], mem[289] = 0x55, byte(a), 0xaa
				before := snapshotInImmediate(c)
				before.flags = subByteMemoryFlags(byte(a), byte(b), initial)
				if err := c.Step(); err != nil || c.EIP != 7 || snapshotInImmediate(c) != before || mem[288] != byte(a-b) || mem[287] != 0x55 || mem[289] != 0xaa || !bytes.Equal(mem[:7], code) {
					t.Fatalf("a=%X b=%X flags=%X：%v", a, b, c.EFlags, err)
				}
			}
		}
	}
}

func TestSUBByteMemoryModRMAndSIBAllAddressConsumers(t *testing.T) {
	// 所有 memory ModRM，並把 DS／SS 同 offset 放不同值。
	for mod := byte(0); mod < 3; mod++ {
		for rm := byte(0); rm < 8; rm++ {
			scales := []byte{0}
			indexes := []byte{4}
			bases := []byte{rm}
			if rm == 4 {
				scales = []byte{0, 1, 2, 3}
				indexes = []byte{0, 1, 2, 3, 4, 5, 6, 7}
				bases = []byte{0, 1, 2, 3, 4, 5, 6, 7}
			}
			for _, scale := range scales {
				for _, index := range indexes {
					for _, base := range bases {
						code := []byte{0x80, mod<<6 | 0x28 | rm}
						if rm == 4 {
							code = append(code, scale<<6|index<<3|base)
						}
						noBase := mod == 0 && base == 5
						offset := uint32(16)
						stack := base == 4 || base == 5
						if noBase {
							offset = 32
							stack = false
						}
						if rm == 4 && index != 4 {
							offset += uint32(16) << scale
						}
						if mod == 1 {
							code = append(code, 0xf0)
							offset -= 16
						}
						if mod == 2 {
							code = binary.LittleEndian.AppendUint32(code, 0xfffffff0)
							offset -= 16
						}
						if noBase {
							code = binary.LittleEndian.AppendUint32(code, 32)
						}
						code = append(code, 8)
						c, mem := subByteMemoryFixture(code)
						for i := range c.R {
							c.R[i] = 16
						}
						ds, ss := 256+offset, 768+offset
						mem[ds], mem[ss] = 0x66, 0x77
						target := ds
						if stack {
							target = ss
						}
						mem[target] = 0x16
						before := append([]byte(nil), mem...)
						before[target] = 0x0e
						state := snapshotInImmediate(c)
						state.flags = subByteMemoryFlags(0x16, 8, c.EFlags)
						if err := c.Step(); err != nil || c.EIP != uint32(len(code)) || snapshotInImmediate(c) != state || !bytes.Equal(mem, before) {
							t.Fatalf("mod=%d rm=%d scale=%d index=%d base=%d：%v", mod, rm, scale, index, base, err)
						}
					}
				}
			}
		}
	}
	// 有效地址按 32 位元加法繞回，最後 byte 仍須在段界內。
	c, mem := subByteMemoryFixture([]byte{0x80, 0xaf, 48, 0, 0, 0, 8})
	c.R[EDI] = 0xfffffff0
	mem[288] = 0x16
	if err := c.Step(); err != nil || mem[288] != 0x0e {
		t.Fatalf("有效地址繞回：%v", err)
	}
}

type subByteFailureBus struct {
	memory              testBus
	target              uint32
	failRead, failWrite bool
	writes              int
}

func (b *subByteFailureBus) Read8(addr uint32) (byte, error) {
	if addr == b.target && b.failRead {
		return 0, errors.New("自製來源失敗")
	}
	return b.memory.Read8(addr)
}
func (b *subByteFailureBus) Write8(addr uint32, value byte) error {
	b.writes++
	if b.failWrite {
		return errors.New("自製目的失敗")
	}
	return b.memory.Write8(addr, value)
}

func TestSUBByteMemoryRejectsWithoutPublishingFlags(t *testing.T) {
	for _, kind := range []string{"未知段", "越界", "唯讀", "bus讀失敗", "bus寫失敗"} {
		t.Run(kind, func(t *testing.T) {
			c, mem := subByteMemoryFixture([]byte{0x80, 0x2d, 32, 0, 0, 0, 8})
			mem[288] = 1
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
			state := snapshotInImmediate(c)
			before := append([]byte(nil), mem...)
			writes := 0
			if kind == "bus寫失敗" {
				writes = 1
			}
			if err := c.Step(); err == nil || snapshotInImmediate(c) != state || !bytes.Equal(mem, before) || bus.writes != writes {
				t.Fatalf("拒絕需保持全部狀態／六旗標：%v writes=%d", err, bus.writes)
			}
		})
	}
	code := []byte{0x80, 0x2d, 32, 0, 0, 0, 8}
	cases := [][]byte{{0x80, 0x2c}, {0x80, 0x6c, 0x24}, {0x80, 0xac, 0x24}}
	for n := 0; n < len(code); n++ {
		cases = append(cases, append([]byte(nil), code[:n]...))
	}
	for _, instruction := range cases {
		// 截短 code 的 bus 必須真的在該處不可讀，不能用補零代替缺位元組。
		c, mem := inImmediateFixture(instruction)
		state := snapshotInImmediate(c)
		before := append([]byte(nil), mem...)
		if err := c.Step(); err == nil || snapshotInImmediate(c) != state || !bytes.Equal(mem, before) {
			t.Fatalf("截短 %X：%v", instruction, err)
		}
	}
	for _, prefix := range []byte{0x66, 0x67, 0x26, 0x2e, 0x36, 0x3e, 0x64, 0x65, 0xf2, 0xf3, 0xf0} {
		instruction := append([]byte{prefix}, code...)
		c, mem := subByteMemoryFixture(instruction)
		// 所有段都可寫，確保拒絕來自前綴，不會被未知段掩蓋。
		for i := range c.Seg {
			c.Seg[i] = 0x188
		}
		mem[288] = 1
		state := snapshotInImmediate(c)
		before := append([]byte(nil), mem...)
		if err := c.Step(); err == nil || snapshotInImmediate(c) != state || !bytes.Equal(mem, before) {
			t.Fatalf("前綴 %X：%v", instruction, err)
		}
	}
}

func TestSUBByteMemoryOriginalRawStateAndExistingRegisterShape(t *testing.T) {
	// 原始 DS offset 重定位為 32，完整 R／段／初始旗標沿原版有限樣本。
	c, mem := subByteMemoryFixture([]byte{0x80, 0x2d, 32, 0, 0, 0, 8})
	c.R = [8]uint32{0x80000000, 0x6bbc7c, 0x272610, 0x31488, 0x2bda8c, 0x31488, 0x6bcc64, 0x6bbc80}
	c.Seg = [6]uint16{8, 0x188, 0x188, 0, 0x20, 0x188}
	c.EFlags = 0x212
	mem[288] = 0x16
	before := snapshotInImmediate(c)
	if err := c.Step(); err != nil || c.EIP != 7 || snapshotInImmediate(c) != before || mem[288] != 0x0e {
		t.Fatalf("原始 byte／完整狀態：%v", err)
	}
	for _, reg := range []byte{0, 1, 2, 3, 4, 5, 6, 7} {
		c, _ := inImmediateFixture([]byte{0x80, 0xe8 | reg, 8})
		before := c.reg8(int(reg))
		flags := subByteMemoryFlags(before, 8, c.EFlags)
		if err := c.Step(); err != nil || c.reg8(int(reg)) != before-8 || c.EFlags != flags {
			t.Fatalf("既有 register SUB %d：%v", reg, err)
		}
	}
}
