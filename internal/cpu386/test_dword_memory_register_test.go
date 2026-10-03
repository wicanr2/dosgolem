package cpu386

import (
	"bytes"
	"encoding/binary"
	"testing"
)

// 342沿287的獨立逐bit交集與五旗標oracle，AF只驗工具模型。
func checkTESTDwordMemory(t *testing.T, c *CPU, mem testBus, target uint32, size int) {
	t.Helper()
	result := testDwordRegisterIntersection(binary.LittleEndian.Uint32(mem[target:target+4]), c.R[(mem[1]>>3)&7])
	want := snapshotInImmediate(c)
	defined := testDwordRegisterDefinedFlags(result, c.EFlags)
	want.flags = defined &^ AF
	before := append([]byte(nil), mem...)
	bus := &subByteFailureBus{memory: mem, failWrite: true}
	c.Bus = bus
	if err := c.Step(); err != nil || c.EIP != uint32(size) || snapshotInImmediate(c) != want || !bytes.Equal(mem, before) || bus.writes != 0 {
		t.Fatalf("TEST target=%X result=%X：%v EIP=%X writes=%d", target, result, err, c.EIP, bus.writes)
	}
	if c.EFlags&^AF != defined&^AF {
		t.Fatal("定義五旗標不符")
	}
	if c.EFlags&AF != 0 {
		t.Fatal("AF清除只驗工具模型")
	}
}
func TestTESTDwordMemoryAllSourcesAndIndependentValues(t *testing.T) {
	values := []uint32{0, 1, 0x7f, 0x80, 0xff, 0x100, 0x7fffffff, 0x80000000, 0xffffffff, 0x55555555, 0xaaaaaaaa, 0x12345678}
	for bit := uint(0); bit < 32; bit++ {
		values = append(values, 1<<bit, ^uint32(1<<bit))
	}
	for src := byte(0); src < 8; src++ {
		for _, a := range values {
			for _, b := range values {
				for _, initial := range []uint32{2 | IF | DF | 0x200000, 2 | IF | DF | 0x200000 | CF | PF | AF | ZF | SF | OF} {
					code := []byte{0x85, src<<3 | 5, 32, 0, 0, 0}
					c, mem := subByteMemoryFixture(code)
					c.R[src], c.EFlags = b, initial
					binary.LittleEndian.PutUint32(mem[288:292], a)
					checkTESTDwordMemory(t, c, mem, 288, len(code))
				}
			}
		}
	}
	// 全部低byte配對，同時保留相異高24位，驗PF只消費結果低byte。
	for a := uint32(0); a < 256; a++ {
		for b := uint32(0); b < 256; b++ {
			c, mem := subByteMemoryFixture([]byte{0x85, 5, 32, 0, 0, 0})
			c.R[EAX] = 0x12000000 + b
			binary.LittleEndian.PutUint32(mem[288:292], 0x81000000+a)
			checkTESTDwordMemory(t, c, mem, 288, 6)
		}
	}
	for bits := uint32(0); bits < 64; bits++ {
		initial := uint32(2 | IF | DF | 0x200000)
		for i, flag := range []uint32{CF, PF, AF, ZF, SF, OF} {
			if bits/(1<<i)%2 != 0 {
				initial |= flag
			}
		}
		for _, a := range []uint32{0, 0x80000000, 0xffffffff, 0x40} {
			c, mem := subByteMemoryFixture([]byte{0x85, 5, 32, 0, 0, 0})
			c.R[EAX], c.EFlags = 0, initial
			binary.LittleEndian.PutUint32(mem[288:292], a)
			checkTESTDwordMemory(t, c, mem, 288, 6)
		}
	}
}

func TestTESTDwordMemoryAllModRMAndSIBWithDistinctSegments(t *testing.T) {
	for src := byte(0); src < 8; src++ {
		for mod := byte(0); mod < 3; mod++ {
			for rm := byte(0); rm < 8; rm++ {
				scales, indexes, bases := []byte{0}, []byte{4}, []byte{rm}
				if rm == 4 {
					scales, indexes, bases = []byte{0, 1, 2, 3}, []byte{0, 1, 2, 3, 4, 5, 6, 7}, []byte{0, 1, 2, 3, 4, 5, 6, 7}
				}
				for _, scale := range scales {
					for _, index := range indexes {
						for _, base := range bases {
							code := []byte{0x85, mod<<6 | src<<3 | rm}
							if rm == 4 {
								code = append(code, scale<<6|index<<3|base)
							}
							noBase := mod == 0 && base == 5
							offset := uint32(16 + base)
							stack := base == 4 || base == 5
							if noBase {
								offset, stack = 32, false
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
							binary.LittleEndian.PutUint32(mem[ds:ds+4], 0x11223344)
							binary.LittleEndian.PutUint32(mem[ss:ss+4], 0x55667788)
							target := ds
							if stack {
								target = ss
							}
							binary.LittleEndian.PutUint32(mem[target:target+4], 0x80000240)
							checkTESTDwordMemory(t, c, mem, target, len(code))
						}
					}
				}
			}
		}
	}
}
func TestTESTDwordMemoryWrapAndLastDword(t *testing.T) {
	for _, tc := range []struct {
		code   []byte
		base   uint32
		target uint32
	}{
		{[]byte{0x85, 0x88, 32, 0, 0, 0}, 0xfffffff0, 272},
		{[]byte{0x85, 0x0d, 252, 0, 0, 0}, 0, 508},
		{[]byte{0x85, 0x0d, 33, 0, 0, 0}, 0, 289},
	} {
		c, mem := subByteMemoryFixture(tc.code)
		c.R[EAX], c.R[ECX] = tc.base, 0x80000004
		binary.LittleEndian.PutUint32(mem[tc.target:tc.target+4], 1)
		checkTESTDwordMemory(t, c, mem, tc.target, len(tc.code))
	}
}

func TestTESTDwordMemoryFailuresAndReadonly(t *testing.T) {
	for _, kind := range []string{"未知段", "越界", "位址溢位", "唯讀"} {
		c, mem := subByteMemoryFixture([]byte{0x85, 0x0d, 32, 0, 0, 0})
		c.R[ECX] = 0x80000001
		binary.LittleEndian.PutUint32(mem[288:292], 0x80000001)
		switch kind {
		case "未知段":
			c.Seg[SegDS] = 0x200
		case "越界":
			c.SetDescriptor(0x188, Descriptor{Base: 256, Limit: 34})
		case "位址溢位":
			binary.LittleEndian.PutUint32(mem[2:6], 0xfffffffd)
			c.SetDescriptor(0x188, Descriptor{Limit: 0xffffffff})
		case "唯讀":
			c.SetDescriptor(0x188, Descriptor{Base: 256, Limit: 255})
			checkTESTDwordMemory(t, c, mem, 288, 6)
			continue
		}
		bus := &subByteFailureBus{memory: mem, failWrite: true}
		c.Bus = bus
		want, before := snapshotInImmediate(c), append([]byte(nil), mem...)
		if err := c.Step(); err == nil || snapshotInImmediate(c) != want || !bytes.Equal(mem, before) || bus.writes != 0 {
			t.Fatalf("%s：%v", kind, err)
		}
	}
	for i := uint32(0); i < 4; i++ {
		c, mem := subByteMemoryFixture([]byte{0x85, 0x0d, 32, 0, 0, 0})
		c.R[ECX] = 0xffffffff
		binary.LittleEndian.PutUint32(mem[288:292], 0x80000001)
		bus := &subByteFailureBus{memory: mem, target: 288 + i, failRead: true, failWrite: true}
		c.Bus = bus
		want, before := snapshotInImmediate(c), append([]byte(nil), mem...)
		if err := c.Step(); err == nil || snapshotInImmediate(c) != want || !bytes.Equal(mem, before) || bus.writes != 0 {
			t.Fatalf("byte%d：%v", i, err)
		}
	}
	for _, full := range [][]byte{{0x85, 0x0d, 32, 0, 0, 0}, {0x85, 0x0c, 0x25, 32, 0, 0, 0}, {0x85, 0x4c, 0x24, 32}, {0x85, 0x8c, 0x24, 32, 0, 0, 0}} {
		for n := 0; n < len(full); n++ {
			c, mem := inImmediateFixture(full[:n])
			want := snapshotInImmediate(c)
			if err := c.Step(); err == nil || snapshotInImmediate(c) != want || !bytes.Equal(mem, full[:n]) {
				t.Fatalf("截短%X：%v", full[:n], err)
			}
		}
	}
	for _, prefix := range []byte{0x66, 0x67, 0x26, 0x2e, 0x36, 0x3e, 0x64, 0x65, 0xf2, 0xf3, 0xf0} {
		c, mem := subByteMemoryFixture(append([]byte{prefix}, []byte{0x85, 0x0d, 32, 0, 0, 0}...))
		binary.LittleEndian.PutUint32(mem[288:292], 0x80000001)
		want, before := snapshotInImmediate(c), append([]byte(nil), mem...)
		bus := &subByteFailureBus{memory: mem, failWrite: true}
		c.Bus = bus
		if err := c.Step(); err == nil || snapshotInImmediate(c) != want || !bytes.Equal(mem, before) || bus.writes != 0 {
			t.Fatalf("prefix%X：%v", prefix, err)
		}
	}
}

func TestTESTDwordMemoryKeepsRegisterWidthsAndF2Rejection(t *testing.T) {
	for dst := byte(0); dst < 8; dst++ {
		for src := byte(0); src < 8; src++ {
			for _, word := range []bool{false, true} {
				code := []byte{0x85, 0xc0 | src<<3 | dst}
				if word {
					code = append([]byte{0x66}, code...)
				}
				c, mem := inImmediateFixture(code)
				want := snapshotInImmediate(c)
				a, b := c.R[dst], c.R[src]
				if word {
					a, b = uint32(uint16(a)), uint32(uint16(b))
				}
				result := testDwordRegisterIntersection(a, b)
				want.flags = testDwordRegisterDefinedFlags(result, c.EFlags) &^ AF
				if word && result >= 0x8000 {
					want.flags |= SF
				}
				if err := c.Step(); err != nil || c.EIP != uint32(len(code)) || snapshotInImmediate(c) != want || !bytes.Equal(mem, code) {
					t.Fatalf("dst%d src%d word%t：%v", dst, src, word, err)
				}
			}
		}
	}
	for _, code := range [][]byte{{0xf2, 0x85, 0xc0}, {0xf2, 0x66, 0x85, 0xc0}, {0x84, 0x05, 32, 0, 0, 0}} {
		c, mem := subByteMemoryFixture(code)
		want, before := snapshotInImmediate(c), append([]byte(nil), mem...)
		if err := c.Step(); err == nil || snapshotInImmediate(c) != want || !bytes.Equal(mem, before) {
			t.Fatalf("原拒絕邊界%X：%v", code, err)
		}
	}
}

func TestTESTDwordMemorySETNEAndRETConsumers(t *testing.T) {
	for _, mask := range []uint32{0, 1} {
		code := []byte{0x85, 0x0d, 32, 0, 0, 0, 0x0f, 0x95, 0xc0, 0xc3}
		c, mem := subByteMemoryFixture(code)
		c.R[ECX] = mask
		c.R[EAX] = 0xabcdeeff
		c.R[ESP] = 32
		binary.LittleEndian.PutUint32(mem[288:292], 1)
		binary.LittleEndian.PutUint32(mem[800:804], 0x60)
		checkTESTDwordMemory(t, c, mem, 288, 6)
		want := snapshotInImmediate(c)
		before := append([]byte(nil), mem...)
		want.r[EAX] = 0xabcdee00 | mask
		if err := c.Step(); err != nil || c.EIP != 9 || snapshotInImmediate(c) != want || !bytes.Equal(mem, before) {
			t.Fatalf("SETNE mask%d：%v", mask, err)
		}
		want.r[ESP] += 4
		if err := c.Step(); err != nil || c.EIP != 0x60 || snapshotInImmediate(c) != want || !bytes.Equal(mem, before) {
			t.Fatalf("RET mask%d：%v", mask, err)
		}
	}
}
