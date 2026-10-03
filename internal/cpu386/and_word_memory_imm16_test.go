package cpu386

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"testing"
)

// 規格353：逐bit與逐位同位數核算，AF只驗工具模型。
func andWordIntersection353(a, b uint16) uint16 {
	var r uint16
	for bit := uint(0); bit < 16; bit++ {
		flag := uint16(1) << bit
		if a/flag%2 == 1 && b/flag%2 == 1 {
			r += flag
		}
	}
	return r
}
func andWordFlags353(r uint16, initial uint32) uint32 {
	flags := initial &^ (CF | PF | ZF | SF | OF)
	if r == 0 {
		flags |= ZF
	}
	if r >= 0x8000 {
		flags |= SF
	}
	ones := 0
	for n := int(byte(r)); n > 0; n /= 2 {
		ones += n % 2
	}
	if ones%2 == 0 {
		flags |= PF
	}
	return flags
}
func andWordFixture353(code []byte) (*CPU, testBus) {
	mem := make(testBus, 1600)
	copy(mem, code)
	c := New(mem)
	c.Seg = [6]uint16{0, 0x188, 0x198, 0x1a0, 0x1a8, 0x190}
	c.SetDescriptor(0x188, Descriptor{Base: 512, Limit: 511, Writable: true})
	c.SetDescriptor(0x190, Descriptor{Base: 1024, Limit: 511, Writable: true})
	c.R = [8]uint32{16, 17, 18, 19, 20, 21, 22, 23}
	c.EFlags = 2 | IF | DF | 0x200000 | CF | PF | AF | ZF | SF | OF
	c.FPUControl = 0x127f
	c.FPUStatus = 0x20
	c.FPUDepth = 2
	c.FPUStack = [8]float64{1.25, -2.5, 3, 4, 5, 6, 7, 8}
	return c, mem
}
func checkANDWordMemory353(t *testing.T, c *CPU, mem testBus, target uint32, mask uint16, size int) {
	t.Helper()
	value := binary.LittleEndian.Uint16(mem[target : target+2])
	result := andWordIntersection353(value, mask)
	want := snapshotInImmediate(c)
	defined := andWordFlags353(result, c.EFlags)
	want.flags = defined &^ AF
	expected := append([]byte(nil), mem...)
	binary.LittleEndian.PutUint16(expected[target:target+2], result)
	if err := c.Step(); err != nil || c.EIP != uint32(size) || snapshotInImmediate(c) != want || !bytes.Equal(mem, expected) {
		t.Fatalf("AND word value=%X mask=%X target=%X flags=%X want=%X EIP=%X err=%v", value, mask, target, c.EFlags, want.flags, c.EIP, err)
	}
	if c.EFlags&^AF != defined&^AF || c.EFlags&AF != 0 {
		t.Fatal("五定義旗標／AF工具模型")
	}
}
func TestANDWordMemoryImmediateIndependentValues(t *testing.T) {
	code := []byte{0x66, 0x81, 0x25, 32, 0, 0, 0, 0x7f, 0xfe}
	// 全16bit來源配原mask，完整iw不可按低byte符號延伸。
	for value := uint32(0); value < 65536; value++ {
		c, mem := andWordFixture353(code)
		binary.LittleEndian.PutUint16(mem[544:546], uint16(value))
		checkANDWordMemory353(t, c, mem, 544, 0xfe7f, len(code))
	}
	for a := uint16(0); a < 256; a++ {
		for b := uint16(0); b < 256; b++ {
			code := binary.LittleEndian.AppendUint16([]byte{0x66, 0x81, 0x25, 32, 0, 0, 0}, 0x8000+b)
			c, mem := andWordFixture353(code)
			binary.LittleEndian.PutUint16(mem[544:546], 0x8100+a)
			checkANDWordMemory353(t, c, mem, 544, 0x8000+b, len(code))
		}
	}
	values := []uint16{0, 1, 0xffff, 0x7fff, 0x8000, 0xaaaa, 0x5555, 0xfe7f, 0x1234}
	for bit := uint(0); bit < 16; bit++ {
		values = append(values, 1<<bit, ^(uint16(1) << bit))
	}
	for _, a := range values {
		for _, b := range values {
			for _, initial := range []uint32{2 | IF | DF | 0x200000, 2 | IF | DF | 0x200000 | CF | PF | AF | ZF | SF | OF} {
				code := binary.LittleEndian.AppendUint16([]byte{0x66, 0x81, 0x25, 32, 0, 0, 0}, b)
				c, mem := andWordFixture353(code)
				c.EFlags = initial
				binary.LittleEndian.PutUint16(mem[544:546], a)
				checkANDWordMemory353(t, c, mem, 544, b, len(code))
			}
		}
	}
	for combo := uint32(0); combo < 64; combo++ {
		for _, a := range []uint16{0, 0x8000, 0xffff, 0x40} {
			c, mem := andWordFixture353(code)
			c.EFlags = 2 | IF | DF | 0x200000
			for i, flag := range []uint32{CF, PF, AF, ZF, SF, OF} {
				if combo/(1<<i)%2 != 0 {
					c.EFlags |= flag
				}
			}
			binary.LittleEndian.PutUint16(mem[544:546], a)
			checkANDWordMemory353(t, c, mem, 544, 0xfe7f, len(code))
		}
	}
}
func TestANDWordMemoryImmediateAllModRMAndSIB(t *testing.T) {
	for mod := byte(0); mod < 3; mod++ {
		for rm := byte(0); rm < 8; rm++ {
			scales, indexes, bases := []byte{0}, []byte{4}, []byte{rm}
			if rm == 4 {
				scales = []byte{0, 1, 2, 3}
				indexes = []byte{0, 1, 2, 3, 4, 5, 6, 7}
				bases = indexes
			}
			for _, scale := range scales {
				for _, index := range indexes {
					for _, base := range bases {
						code := []byte{0x66, 0x81, mod<<6 | 4<<3 | rm}
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
						code = binary.LittleEndian.AppendUint16(code, 0xfe7f)
						c, mem := andWordFixture353(code)
						binary.LittleEndian.PutUint16(mem[512+offset:514+offset], 0xf123)
						binary.LittleEndian.PutUint16(mem[1024+offset:1026+offset], 0x8abc)
						target := 512 + offset
						if stack {
							target = 1024 + offset
						}
						checkANDWordMemory353(t, c, mem, target, 0xfe7f, len(code))
					}
				}
			}
		}
	}
}
func TestANDWordMemoryImmediateWrapUnalignedAndLastWord(t *testing.T) {
	for _, tc := range []struct {
		code         []byte
		base, target uint32
	}{
		{[]byte{0x66, 0x81, 0xa0, 32, 0, 0, 0, 0x7f, 0xfe}, 0xfffffff0, 528},
		{[]byte{0x66, 0x81, 0x25, 33, 0, 0, 0, 0x7f, 0xfe}, 0, 545},
		{[]byte{0x66, 0x81, 0x25, 254, 1, 0, 0, 0x7f, 0xfe}, 0, 1022},
	} {
		c, mem := andWordFixture353(tc.code)
		c.R[EAX] = tc.base
		binary.LittleEndian.PutUint16(mem[tc.target:tc.target+2], 0xffff)
		checkANDWordMemory353(t, c, mem, tc.target, 0xfe7f, len(tc.code))
	}
}

type andWordFailureBus353 struct {
	testBus
	readFail, writeFail uint32
	writes              []uint32
}

func (b *andWordFailureBus353) Read8(addr uint32) (uint8, error) {
	if addr == b.readFail {
		return 0, fmt.Errorf("受控讀取失敗")
	}
	return b.testBus.Read8(addr)
}
func (b *andWordFailureBus353) Write8(addr uint32, v uint8) error {
	b.writes = append(b.writes, addr)
	if addr == b.writeFail {
		return fmt.Errorf("受控寫入失敗")
	}
	return b.testBus.Write8(addr, v)
}
func TestANDWordMemoryImmediateFailures(t *testing.T) {
	for _, kind := range []string{"未知段", "唯讀", "段末", "線性溢位", "首讀", "末讀", "首寫", "末寫"} {
		mem := make(testBus, 80)
		copy(mem, []byte{0x66, 0x81, 0x25, 64, 0, 0, 0, 0, 0x12})
		binary.LittleEndian.PutUint16(mem[64:66], 0xffff)
		bus := &andWordFailureBus353{testBus: mem, readFail: ^uint32(0), writeFail: ^uint32(0)}
		c := New(bus)
		c.Seg[SegDS] = 0x188
		c.SetDescriptor(0x188, Descriptor{Limit: 79, Writable: true})
		c.EFlags = 2 | IF | CF | PF | AF | ZF | SF | OF
		switch kind {
		case "未知段":
			c.Seg[SegDS] = 0x200
		case "唯讀":
			c.SetDescriptor(0x188, Descriptor{Limit: 79})
		case "段末":
			c.SetDescriptor(0x188, Descriptor{Limit: 64, Writable: true})
		case "線性溢位":
			c.SetDescriptor(0x188, Descriptor{Base: 0xffffffe0, Limit: 79, Writable: true})
		case "首讀":
			bus.readFail = 64
		case "末讀":
			bus.readFail = 65
		case "首寫":
			bus.writeFail = 64
		case "末寫":
			bus.writeFail = 65
		}
		want := snapshotInImmediate(c)
		expected := append([]byte(nil), mem...)
		if kind == "末寫" {
			expected[64] = 0
		}
		if err := c.Step(); err == nil || snapshotInImmediate(c) != want || !bytes.Equal(mem, expected) {
			t.Fatalf("拒絕%s flags=%X err=%v", kind, c.EFlags, err)
		}
		count := 0
		if kind == "首寫" {
			count = 1
		}
		if kind == "末寫" {
			count = 2
		}
		if len(bus.writes) != count {
			t.Fatalf("%s writes=%v", kind, bus.writes)
		}
	}
}
func TestANDWordMemoryImmediateRejectedForms(t *testing.T) {
	code := []byte{0x66, 0x81, 0x25, 64, 0, 0, 0, 0x7f, 0xfe}
	cases := [][]byte{}
	for n := 0; n < len(code); n++ {
		cases = append(cases, code[:n])
	}
	for _, prefix := range []byte{0x66, 0x26, 0x2e, 0x36, 0x3e, 0x64, 0x65, 0xf2, 0xf3, 0x67, 0xf0} {
		cases = append(cases, append([]byte{prefix}, code...))
	}
	for _, group := range []byte{0, 1, 2, 3, 5, 6} {
		cases = append(cases, []byte{0x66, 0x81, group<<3 | 5, 64, 0, 0, 0, 0, 0x12})
	}
	for _, code := range cases {
		mem := testBus(append([]byte(nil), code...))
		c := New(mem)
		c.EFlags = 2 | IF | CF | PF | AF | ZF | SF | OF
		want := snapshotInImmediate(c)
		expected := append([]byte(nil), mem...)
		if err := c.Step(); err == nil || snapshotInImmediate(c) != want || !bytes.Equal(mem, expected) {
			t.Fatalf("未審查形式=%X err=%v", code, err)
		}
	}
}
func TestANDWordMemoryImmediateExistingWordRegisterAndCMP(t *testing.T) {
	for reg := byte(0); reg < 8; reg++ {
		for _, group := range []byte{0, 1, 4, 5, 7} {
			code := []byte{0x66, 0x81, 0xc0 | group<<3 | reg, 1, 0}
			mem := testBus(code)
			c := New(mem)
			c.R[reg] = 0xabcd0003
			c.EFlags = 2 | IF | CF | AF | ZF | SF | OF
			want := snapshotInImmediate(c)
			result := uint32(0xabcd0002)
			want.flags = 2 | IF
			switch group {
			case 0:
				result = 0xabcd0004
			case 1:
				result = 0xabcd0003
				want.flags |= PF
			case 4:
				result = 0xabcd0001
			case 7:
				result = 0xabcd0003
			}
			want.r[reg] = result
			if err := c.Step(); err != nil || c.EIP != 5 || snapshotInImmediate(c) != want {
				t.Fatalf("原register群組=%d reg=%d R=%X flags=%X err=%v", group, reg, c.R, c.EFlags, err)
			}
		}
	}
	code := []byte{0x66, 0x81, 0x3d, 32, 0, 0, 0, 1, 0}
	c, mem := andWordFixture353(code)
	binary.LittleEndian.PutUint16(mem[544:546], 3)
	c.EFlags = 2 | IF | CF | AF | ZF | SF | OF
	before := append([]byte(nil), mem...)
	want := snapshotInImmediate(c)
	want.flags = 2 | IF
	if err := c.Step(); err != nil || c.EIP != 9 || snapshotInImmediate(c) != want || !bytes.Equal(mem, before) {
		t.Fatalf("原memory CMP保持 err=%v", err)
	}
}

func TestANDWordMemoryImmediateAlwaysWritesTwoBytes(t *testing.T) {
	for _, value := range []uint16{0, 0xffff} {
		mem := make(testBus, 80)
		copy(mem, []byte{0x66, 0x81, 0x25, 64, 0, 0, 0, 0x7f, 0xfe})
		binary.LittleEndian.PutUint16(mem[64:66], value)
		bus := &andWordFailureBus353{testBus: mem, readFail: ^uint32(0), writeFail: ^uint32(0)}
		c := New(bus)
		c.Seg[SegDS] = 0x188
		c.SetDescriptor(0x188, Descriptor{Limit: 65, Writable: true})
		c.EFlags = 2 | IF | CF | AF | OF
		want := snapshotInImmediate(c)
		result := andWordIntersection353(value, 0xfe7f)
		want.flags = andWordFlags353(result, c.EFlags) &^ AF
		expected := append([]byte(nil), mem...)
		binary.LittleEndian.PutUint16(expected[64:66], result)
		if err := c.Step(); err != nil || c.EIP != 9 || snapshotInImmediate(c) != want || !bytes.Equal(mem, expected) || len(bus.writes) != 2 || bus.writes[0] != 64 || bus.writes[1] != 65 {
			t.Fatalf("完整word寫回value=%X writes=%v err=%v", value, bus.writes, err)
		}
	}
}
