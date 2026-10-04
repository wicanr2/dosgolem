package cpu386

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"testing"
)

// 規格378：記錄真正code fetch；資料write必須為零。
type subALBus378 struct {
	testBus
	sparse map[uint32]byte
	refuse *uint32
	reads  []uint32
	writes int
}

func (b *subALBus378) Read8(addr uint32) (uint8, error) {
	b.reads = append(b.reads, addr)
	if b.refuse != nil && addr == *b.refuse {
		return 0, fmt.Errorf("SUB AL測試拒絕fetch")
	}
	if b.sparse != nil {
		if v, ok := b.sparse[addr]; ok {
			return v, nil
		}
		return 0, fmt.Errorf("SUB AL稀疏code越界")
	}
	return b.testBus.Read8(addr)
}
func (b *subALBus378) Write8(addr uint32, value uint8) error {
	b.writes++
	return b.testBus.Write8(addr, value)
}

// 用位元組編碼保留EAX高24bit；旗標oracle用寬差值／nibble借位及有號範圍。
func subALWant378(s inImmediateState, a, b byte) inImmediateState {
	var lane [4]byte
	binary.LittleEndian.PutUint32(lane[:], s.r[EAX])
	lane[0] = byte((int(a) - int(b) + 256) % 256)
	s.r[EAX] = binary.LittleEndian.Uint32(lane[:])
	s.flags = subByteMemoryFlags(a, b, s.flags)
	return s
}
func subALCheck378(t *testing.T, c *CPU, bus *subALBus378, mem testBus, initial inImmediateState, a, b byte, upper, flags uint32) {
	t.Helper()
	c.EIP = 0
	c.R = initial.r
	c.EFlags = flags
	var lane [4]byte
	binary.LittleEndian.PutUint32(lane[:], upper)
	lane[0] = a
	c.R[EAX] = binary.LittleEndian.Uint32(lane[:])
	mem[1] = b
	bus.reads = bus.reads[:0]
	bus.writes = 0
	want := subALWant378(snapshotInImmediate(c), a, b)
	if err := c.Step(); err != nil || c.EIP != 2 || snapshotInImmediate(c) != want || bus.writes != 0 || !bytes.Equal(mem, []byte{0x2c, b}) || len(bus.reads) != 2 || bus.reads[0] != 0 || bus.reads[1] != 1 {
		t.Fatalf("SUB AL a=%X imm=%X upper=%X initialflags=%X EIP=%X flags=%X reads=%v writes=%d err=%v", a, b, upper, flags, c.EIP, c.EFlags, bus.reads, bus.writes, err)
	}
}
func TestSUBAL378AllValuesAndHighEAX(t *testing.T) {
	c, mem := inImmediateFixture([]byte{0x2c, 0})
	bus := &subALBus378{testBus: mem}
	c.Bus = bus
	initial := snapshotInImmediate(c)
	for _, upper := range []uint32{0, 0xa5c33d00, 0xffffff00} {
		for _, flags := range []uint32{2 | IF | DF | 0x200000, 0xa5a52fd7} {
			for a := 0; a < 256; a++ {
				for b := 0; b < 256; b++ {
					subALCheck378(t, c, bus, mem, initial, byte(a), byte(b), upper, flags)
				}
			}
		}
	}
}
func TestSUBAL378AllIncomingArithmeticFlags(t *testing.T) {
	c, mem := inImmediateFixture([]byte{0x2c, 0})
	bus := &subALBus378{testBus: mem}
	c.Bus = bus
	initial := snapshotInImmediate(c)
	bits := []uint32{CF, PF, AF, ZF, SF, OF}
	values := []byte{0, 1, 7, 15, 16, 127, 128, 129, 255}
	for setting := 0; setting < 64; setting++ {
		flags := uint32(2 | IF | DF | 0x200000)
		for j, flag := range bits {
			if setting/(1<<j)%2 != 0 {
				flags |= flag
			}
		}
		for _, a := range values {
			for _, b := range values {
				subALCheck378(t, c, bus, mem, initial, a, b, 0xa5c33d00, flags)
			}
		}
	}
}
func TestSUBAL378OriginalStop(t *testing.T) {
	c, mem := inImmediateFixture([]byte{0x2c, 0x17})
	c.R = [8]uint32{0x1a, 0, 0x2bd800, 0xd, 0x2bafc8, 0x2bd8a0, 0x29e1d8, 0x2bd348}
	c.EFlags = 0x206
	c.FPUControl = 0x127f
	c.FPUStatus = 0
	c.FPUDepth = 0
	c.FPUStack = [8]float64{}
	want := snapshotInImmediate(c)
	want.r[EAX] = 3
	if err := c.Step(); err != nil || c.EIP != 2 || snapshotInImmediate(c) != want || !bytes.Equal(mem, []byte{0x2c, 0x17}) {
		t.Fatalf("原2C17未符合03h／206h：%v R=%X flags=%X", err, c.R, c.EFlags)
	}
}
func TestSUBAL378OriginalFourInstructionConsumers(t *testing.T) {
	code := []byte{0x2c, 0x17, 0x3c, 0x08, 0x0f, 0x87, 0xed, 0, 0, 0, 0x0f, 0xb6, 0xc0}
	c, mem := inImmediateFixture(code)
	c.EFlags = 0x206
	c.R[EAX] = 0x1a
	want := snapshotInImmediate(c)
	want = subALWant378(want, 0x1a, 0x17)
	for j, eip := range []uint32{2, 4, 10, 13} {
		if j == 1 {
			want.flags = subByteMemoryFlags(3, 8, want.flags)
		}
		if err := c.Step(); err != nil || c.EIP != eip || snapshotInImmediate(c) != want || !bytes.Equal(mem, code) {
			t.Fatalf("原四步j=%d EIP=%X flags=%X err=%v", j, c.EIP, c.EFlags, err)
		}
	}
}
func TestSUBAL378FetchFailureAndModelPrefixBoundary(t *testing.T) {
	for _, code := range [][]byte{{}, {0x2c}, {0x66, 0x2c, 0x17}, {0x67, 0x2c, 0x17}, {0x26, 0x2c, 0x17}, {0x2e, 0x2c, 0x17}, {0x36, 0x2c, 0x17}, {0x3e, 0x2c, 0x17}, {0x64, 0x2c, 0x17}, {0x65, 0x2c, 0x17}, {0xf2, 0x2c, 0x17}, {0xf3, 0x2c, 0x17}, {0xf0, 0x2c, 0x17}} {
		c, mem := inImmediateFixture(code)
		want := snapshotInImmediate(c)
		bus := &subALBus378{testBus: mem}
		c.Bus = bus
		err := c.Step()
		if err == nil || snapshotInImmediate(c) != want || !bytes.Equal(mem, code) || bus.writes != 0 {
			t.Fatalf("拒絕前發布結果code=%X err=%v", code, err)
		}
		if len(code) == 1 {
			var detail *Error
			if !errors.As(err, &detail) || detail.EIP != 0 || detail.Opcode != 0x2c || c.EIP != 1 {
				t.Fatalf("立即數fetch失敗定位：%v", err)
			}
		}
	}
	c, mem := inImmediateFixture([]byte{0x2c, 0x17})
	refuse := uint32(1)
	bus := &subALBus378{testBus: mem, refuse: &refuse}
	c.Bus = bus
	want := snapshotInImmediate(c)
	err := c.Step()
	if err == nil || c.EIP != 1 || snapshotInImmediate(c) != want || bus.writes != 0 || !bytes.Equal(mem, []byte{0x2c, 0x17}) {
		t.Fatalf("Bus拒絕立即數仍發布：%v", err)
	}
}
func TestSUBAL378FetchAtLastByteAndEIPWrap(t *testing.T) {
	for _, start := range []uint32{0xfffffffe, 0xffffffff} {
		c, _ := inImmediateFixture(nil)
		c.EIP = start
		c.EFlags = 0x206
		c.R[EAX] = 0xa5c33d1a
		bus := &subALBus378{sparse: map[uint32]byte{start: 0x2c, start + 1: 0x17}}
		c.Bus = bus
		want := subALWant378(snapshotInImmediate(c), 0x1a, 0x17)
		if err := c.Step(); err != nil || c.EIP != start+2 || snapshotInImmediate(c) != want || bus.writes != 0 || len(bus.reads) != 2 || bus.reads[0] != start || bus.reads[1] != start+1 {
			t.Fatalf("末byte／EIP wrap：start=%X err=%v", start, err)
		}
	}
}
