package cpu386

import (
	"bytes"
	"encoding/binary"
	"errors"
	"testing"
)

// 預期逐bit聯集以整除計算，不呼叫CPU的邏輯運算／旗標helper。
func orDwordMemoryValue(a, b uint32) uint32 {
	var result uint64
	for weight := uint64(1); weight <= 1<<31; weight *= 2 {
		if uint64(a)/weight%2 != 0 || uint64(b)/weight%2 != 0 {
			result += weight
		}
	}
	return uint32(result)
}

func orDwordMemoryFlags(result, initial uint32) uint32 {
	flags := initial &^ (CF | PF | AF | ZF | SF | OF)
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

func checkORDwordMemory(t *testing.T, c *CPU, mem testBus, target uint32, size int) {
	t.Helper()
	result := orDwordMemoryValue(binary.LittleEndian.Uint32(mem[target:target+4]), c.R[(mem[1]>>3)&7])
	want := snapshotInImmediate(c)
	want.flags = orDwordMemoryFlags(result, c.EFlags)
	before := append([]byte(nil), mem...)
	binary.LittleEndian.PutUint32(before[target:target+4], result)
	if err := c.Step(); err != nil || c.EIP != uint32(size) || snapshotInImmediate(c) != want || !bytes.Equal(mem, before) {
		t.Fatalf("OR dword target=%X result=%X：%v EIP=%X", target, result, err, c.EIP)
	}
	if c.EFlags&AF != 0 {
		t.Fatal("AF清除只驗工具模型")
	}
}

func TestORDwordMemoryAllSourcesAndIndependentValues(t *testing.T) {
	values := []uint32{0, 1, 0x7f, 0x80, 0xff, 0x100, 0x7fffffff, 0x80000000, 0xffffffff, 0x55555555, 0xaaaaaaaa, 0x12345678}
	for bit := uint(0); bit < 32; bit++ {
		values = append(values, 1<<bit, ^uint32(1<<bit))
	}
	for src := byte(0); src < 8; src++ {
		for _, a := range values {
			for _, b := range values {
				for _, initial := range []uint32{2 | IF | DF | 0x200000, 2 | IF | DF | 0x200000 | CF | PF | AF | ZF | SF | OF} {
					code := []byte{0x09, src<<3 | 5, 32, 0, 0, 0}
					c, mem := subByteMemoryFixture(code)
					c.R[src], c.EFlags = b, initial
					binary.LittleEndian.PutUint32(mem[288:292], a)
					checkORDwordMemory(t, c, mem, 288, len(code))
				}
			}
		}
	}
	// 全部低byte配對，同時保留相異高24位，驗PF只消費結果低byte。
	for a := uint32(0); a < 256; a++ {
		for b := uint32(0); b < 256; b++ {
			c, mem := subByteMemoryFixture([]byte{0x09, 5, 32, 0, 0, 0})
			c.R[EAX] = 0x12000000 + b
			binary.LittleEndian.PutUint32(mem[288:292], 0x81000000+a)
			checkORDwordMemory(t, c, mem, 288, 6)
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
			c, mem := subByteMemoryFixture([]byte{0x09, 5, 32, 0, 0, 0})
			c.R[EAX], c.EFlags = 0, initial
			binary.LittleEndian.PutUint32(mem[288:292], a)
			checkORDwordMemory(t, c, mem, 288, 6)
		}
	}
}

func TestORDwordMemoryAllModRMAndSIBWithDistinctSegments(t *testing.T) {
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
							code := []byte{0x09, mod<<6 | src<<3 | rm}
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
							checkORDwordMemory(t, c, mem, target, len(code))
						}
					}
				}
			}
		}
	}
}

type orDwordFailureBus struct{ subByteFailureBus }

func (b *orDwordFailureBus) Write8(addr uint32, value byte) error {
	b.writes++
	if addr == b.target && b.failWrite {
		return errors.New("自製指定byte寫失敗")
	}
	return b.memory.Write8(addr, value)
}

func TestORDwordMemoryFailuresAndPartialWriteModel(t *testing.T) {
	for _, kind := range []string{"未知段", "越界", "唯讀", "位址溢位"} {
		c, mem := subByteMemoryFixture([]byte{0x09, 0x0d, 32, 0, 0, 0})
		c.R[ECX] = 0xffffffff
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
			c, mem := subByteMemoryFixture([]byte{0x09, 0x0d, 32, 0, 0, 0})
			c.R[ECX] = 0xffffffff
			binary.LittleEndian.PutUint32(mem[288:292], 0x01020304)
			bus := &orDwordFailureBus{subByteFailureBus{memory: mem, target: 288 + byteIndex, failRead: !write, failWrite: write}}
			c.Bus = bus
			want, before := snapshotInImmediate(c), append([]byte(nil), mem...)
			writes := 0
			if write {
				writes = int(byteIndex) + 1
				for i := uint32(0); i < byteIndex; i++ {
					before[288+i] = 0xff
				}
			}
			if err := c.Step(); err == nil || snapshotInImmediate(c) != want || !bytes.Equal(mem, before) || bus.writes != writes {
				t.Fatalf("Bus write=%t byte=%d：%v writes=%d", write, byteIndex, err, bus.writes)
			}
		}
	}
	for _, full := range [][]byte{{0x09, 0x0d, 32, 0, 0, 0}, {0x09, 0x0c, 0x25, 32, 0, 0, 0}, {0x09, 0x4c, 0x24, 32}, {0x09, 0x8c, 0x24, 32, 0, 0, 0}} {
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
		code := append([]byte{prefix}, []byte{0x09, 0x0d, 32, 0, 0, 0}...)
		c, mem := subByteMemoryFixture(code)
		binary.LittleEndian.PutUint32(mem[288:292], 0x12345678)
		want, before := snapshotInImmediate(c), append([]byte(nil), mem...)
		if err := c.Step(); err == nil || snapshotInImmediate(c) != want || !bytes.Equal(mem, before) {
			t.Fatalf("前綴 %X：%v", prefix, err)
		}
	}
}

func TestORDwordMemoryWrapAndLastDword(t *testing.T) {
	for _, tc := range []struct {
		code   []byte
		base   uint32
		target uint32
	}{
		{[]byte{0x09, 0x88, 32, 0, 0, 0}, 0xfffffff0, 272},
		{[]byte{0x09, 0x0d, 252, 0, 0, 0}, 0, 508},
	} {
		c, mem := subByteMemoryFixture(tc.code)
		c.R[EAX], c.R[ECX] = tc.base, 0x80000004
		binary.LittleEndian.PutUint32(mem[tc.target:tc.target+4], 1)
		checkORDwordMemory(t, c, mem, tc.target, len(tc.code))
	}
}

func TestORDwordMemoryKeepsRegisterWidths(t *testing.T) {
	for dst := byte(0); dst < 8; dst++ {
		for src := byte(0); src < 8; src++ {
			for _, word := range []bool{false, true} {
				code := []byte{0x09, 0xc0 | src<<3 | dst}
				if word {
					code = append([]byte{0x66}, code...)
				}
				c, mem := inImmediateFixture(code)
				want := snapshotInImmediate(c)
				a, b := c.R[dst], c.R[src]
				if word {
					a, b = uint32(uint16(a)), uint32(uint16(b))
				}
				result := orDwordMemoryValue(a, b)
				want.r[dst] = result
				want.flags = orDwordMemoryFlags(result, c.EFlags)
				if word {
					want.r[dst] = c.R[dst]&0xffff0000 | result
					if result >= 0x8000 {
						want.flags |= SF
					}
				}
				if err := c.Step(); err != nil || c.EIP != uint32(len(code)) || snapshotInImmediate(c) != want || !bytes.Equal(mem, code) {
					t.Fatalf("dst=%d src=%d word=%t：%v", dst, src, word, err)
				}
			}
		}
	}
}
