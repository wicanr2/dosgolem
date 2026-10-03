package cpu386

import (
	"bytes"
	"encoding/binary"
	"testing"
)

// 以整數範圍、除法與位元計數建立獨立預期，不呼叫CPU算術helper。
func negDwordValue(value uint32) uint32 { return uint32((uint64(1) << 32) - uint64(value)) }

func negDwordFlags(value, initial uint32) uint32 {
	result := negDwordValue(value)
	flags := initial &^ (CF | PF | AF | ZF | SF | OF)
	if value != 0 {
		flags |= CF
	}
	if value%16 != 0 {
		flags |= AF
	}
	if value == 0x80000000 {
		flags |= OF
	}
	if result == 0 {
		flags |= ZF
	}
	if result >= 1<<31 {
		flags |= SF
	}
	ones := 0
	for n := result % 256; n > 0; n /= 2 {
		ones += int(n % 2)
	}
	if ones%2 == 0 {
		flags |= PF
	}
	return flags
}

func checkNegDwordMemory(t *testing.T, c *CPU, mem testBus, target uint32, size int) {
	t.Helper()
	value := binary.LittleEndian.Uint32(mem[target : target+4])
	want := snapshotInImmediate(c)
	want.flags = negDwordFlags(value, c.EFlags)
	before := append([]byte(nil), mem...)
	binary.LittleEndian.PutUint32(before[target:target+4], negDwordValue(value))
	if err := c.Step(); err != nil || c.EIP != uint32(size) || snapshotInImmediate(c) != want || !bytes.Equal(mem, before) {
		t.Fatalf("NEG target=%X source=%X：%v EIP=%X", target, value, err, c.EIP)
	}
}

func TestNegDwordMemoryIndependentValuesAndFlags(t *testing.T) {
	values := []uint32{0, 1, 0x7f, 0x80, 0xff, 0x100, 0x7fffffff, 0x80000000, 0xffffffff, 0x55555555, 0xaaaaaaaa, 0x12345678}
	for bit := uint(0); bit < 32; bit++ {
		values = append(values, 1<<bit, ^uint32(1<<bit))
	}
	for value := uint32(0); value < 65536; value++ {
		values = append(values, value)
	}
	for _, value := range values {
		for _, initial := range []uint32{2 | IF | DF | 0x200000, 2 | IF | DF | 0x200000 | CF | PF | AF | ZF | SF | OF} {
			code := []byte{0xf7, 0x1d, 33, 0, 0, 0}
			c, mem := subByteMemoryFixture(code)
			c.EFlags = initial
			binary.LittleEndian.PutUint32(mem[289:293], value)
			checkNegDwordMemory(t, c, mem, 289, len(code))
		}
	}
	for bits := uint32(0); bits < 64; bits++ {
		initial := uint32(2 | IF | DF | 0x200000)
		for i, flag := range []uint32{CF, PF, AF, ZF, SF, OF} {
			if bits/(1<<i)%2 != 0 {
				initial |= flag
			}
		}
		for _, value := range []uint32{0, 1, 0x80000000, 0xffffffff, 0x40} {
			c, mem := subByteMemoryFixture([]byte{0xf7, 0x1d, 32, 0, 0, 0})
			c.EFlags = initial
			binary.LittleEndian.PutUint32(mem[288:292], value)
			checkNegDwordMemory(t, c, mem, 288, 6)
		}
	}
}

func TestNegDwordMemoryFailuresAndPartialWriteModel(t *testing.T) {
	for _, kind := range []string{"未知段", "越界", "唯讀", "位址溢位"} {
		c, mem := subByteMemoryFixture([]byte{0xf7, 0x1d, 32, 0, 0, 0})
		binary.LittleEndian.PutUint32(mem[288:292], 0x01020304)
		bus := &subByteFailureBus{memory: mem, target: 288}
		c.Bus = bus
		switch kind {
		case "未知段":
			c.Seg[SegDS] = 0x200
		case "越界":
			c.SetDescriptor(0x188, Descriptor{Base: 256, Limit: 34, Writable: true})
		case "唯讀":
			c.SetDescriptor(0x188, Descriptor{Base: 256, Limit: 255})
		case "位址溢位":
			binary.LittleEndian.PutUint32(mem[2:6], 0xfffffffd)
			c.SetDescriptor(0x188, Descriptor{Limit: 0xffffffff, Writable: true})
		}
		want, before := snapshotInImmediate(c), append([]byte(nil), mem...)
		if err := c.Step(); err == nil || snapshotInImmediate(c) != want || !bytes.Equal(mem, before) || bus.writes != 0 {
			t.Fatalf("%s：%v", kind, err)
		}
	}
	for byteIndex := uint32(0); byteIndex < 4; byteIndex++ {
		for _, write := range []bool{false, true} {
			c, mem := subByteMemoryFixture([]byte{0xf7, 0x1d, 32, 0, 0, 0})
			binary.LittleEndian.PutUint32(mem[288:292], 0x01020304)
			bus := &orDwordFailureBus{subByteFailureBus{memory: mem, target: 288 + byteIndex, failRead: !write, failWrite: write}}
			c.Bus = bus
			want, before := snapshotInImmediate(c), append([]byte(nil), mem...)
			writes := 0
			var result [4]byte
			binary.LittleEndian.PutUint32(result[:], negDwordValue(0x01020304))
			if write {
				writes = int(byteIndex) + 1
				copy(before[288:288+byteIndex], result[:byteIndex])
			}
			if err := c.Step(); err == nil || snapshotInImmediate(c) != want || !bytes.Equal(mem, before) || bus.writes != writes {
				t.Fatalf("Bus write=%t byte=%d：%v writes=%d", write, byteIndex, err, bus.writes)
			}
		}
	}
}

func TestNegDwordMemoryTruncationPrefixesAndGroups(t *testing.T) {
	for _, full := range [][]byte{{0xf7, 0x1d, 32, 0, 0, 0}, {0xf7, 0x1c, 0x25, 32, 0, 0, 0}, {0xf7, 0x5c, 0x24, 32}, {0xf7, 0x9c, 0x24, 32, 0, 0, 0}, {0xf7, 0x5d, 0xd8}} {
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
		code := append([]byte{prefix}, []byte{0xf7, 0x1d, 32, 0, 0, 0}...)
		c, mem := subByteMemoryFixture(code)
		binary.LittleEndian.PutUint32(mem[288:292], 0x12345678)
		// 358：word NEG已接通，此66負例明確限定未知selector。
		if prefix == 0x66 {
			c.Seg[SegDS] = 0x200
		}
		want, before := snapshotInImmediate(c), append([]byte(nil), mem...)
		if err := c.Step(); err == nil || snapshotInImmediate(c) != want || !bytes.Equal(mem, before) {
			t.Fatalf("前綴 %X：%v", prefix, err)
		}
	}
	for _, group := range []byte{1, 2, 4, 5} {
		c, mem := subByteMemoryFixture([]byte{0xf7, group<<3 | 5, 32, 0, 0, 0})
		binary.LittleEndian.PutUint32(mem[288:292], 0x12345678)
		want, before := snapshotInImmediate(c), append([]byte(nil), mem...)
		if err := c.Step(); err == nil || snapshotInImmediate(c) != want || !bytes.Equal(mem, before) {
			t.Fatalf("其他F7群組 %d：%v", group, err)
		}
	}
}

func TestNegDwordMemoryWrapAndLastDword(t *testing.T) {
	for _, tc := range []struct {
		code         []byte
		base, target uint32
	}{
		{[]byte{0xf7, 0x98, 32, 0, 0, 0}, 0xfffffff0, 272},
		{[]byte{0xf7, 0x1d, 252, 0, 0, 0}, 0, 508},
	} {
		c, mem := subByteMemoryFixture(tc.code)
		c.R[EAX] = tc.base
		binary.LittleEndian.PutUint32(mem[tc.target:tc.target+4], 0x80000001)
		checkNegDwordMemory(t, c, mem, tc.target, len(tc.code))
	}
}

func TestNegDwordRegisterAndESPRegression(t *testing.T) {
	for dst := byte(0); dst < 8; dst++ {
		for _, value := range []uint32{0, 1, 0xffffffff, 0x80000000, 0x12345678} {
			code := []byte{0xf7, 0xd8 | dst}
			c, mem := inImmediateFixture(code)
			c.R[dst] = value
			want := snapshotInImmediate(c)
			want.r[dst] = negDwordValue(value)
			want.flags = negDwordFlags(value, c.EFlags)
			if err := c.Step(); err != nil || c.EIP != 2 || snapshotInImmediate(c) != want || !bytes.Equal(mem, code) {
				t.Fatalf("register%d：%v", dst, err)
			}
		}
	}
	for _, delta := range []byte{4, 0xfc} {
		c, mem := subByteMemoryFixture([]byte{0xf7, 0x5c, 0x24, delta})
		c.R[ESP] = 32
		target := uint32(768 + 32 + int32(int8(delta)))
		binary.LittleEndian.PutUint32(mem[target:target+4], 0x12345678)
		checkNegDwordMemory(t, c, mem, target, 4)
	}
}

func TestNegDwordMemoryAllModRMAndSIBWithDistinctSegments(t *testing.T) {
	const group = byte(3)
	{
		for mod := byte(0); mod < 3; mod++ {
			for rm := byte(0); rm < 8; rm++ {
				scales, indexes, bases := []byte{0}, []byte{4}, []byte{rm}
				if rm == 4 {
					scales, indexes, bases = []byte{0, 1, 2, 3}, []byte{0, 1, 2, 3, 4, 5, 6, 7}, []byte{0, 1, 2, 3, 4, 5, 6, 7}
				}
				for _, scale := range scales {
					for _, index := range indexes {
						for _, base := range bases {
							code := []byte{0xf7, mod<<6 | group<<3 | rm}
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
							checkNegDwordMemory(t, c, mem, target, len(code))
						}
					}
				}
			}
		}
	}
}
