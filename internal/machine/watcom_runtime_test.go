package machine

import (
	"encoding/binary"
	"testing"

	"github.com/wicanr2/dosgolem/internal/cpu386"
)

func watcomHeapFixture(t *testing.T, capacity uint32) (*LEMachine, *WatcomNearHeap) {
	t.Helper()
	m := &LEMachine{Mem: make([]byte, 0x100)}
	m.CPU = cpu386.New(m)
	m.CPU.Seg[cpu386.SegSS] = 0x160
	m.CPU.SetDescriptor(0x160, cpu386.Descriptor{Base: 0, Limit: 0xffffffff, Writable: true})
	m.CPU.R[cpu386.ESP] = 0x40
	m.CPU.EIP = 0x1234
	heap, err := NewWatcomNearHeap(m, 0x1234, capacity)
	if err != nil {
		t.Fatal(err)
	}
	m.CPU.StepHook = heap.Handle
	return m, heap
}

func callWatcomHeap(t *testing.T, m *LEMachine, size, ret uint32) uint32 {
	t.Helper()
	m.CPU.EIP = 0x1234
	m.CPU.R[cpu386.ESP] = 0x40
	binary.LittleEndian.PutUint32(m.Mem[0x40:], ret)
	binary.LittleEndian.PutUint32(m.Mem[0x44:], size)
	if err := m.CPU.Step(); err != nil {
		t.Fatal(err)
	}
	if m.CPU.EIP != ret || m.CPU.R[cpu386.ESP] != 0x44 {
		t.Fatalf("cdecl return EIP=%X ESP=%X", m.CPU.EIP, m.CPU.R[cpu386.ESP])
	}
	return m.CPU.R[cpu386.EAX]
}

func TestWatcomNearHeapDeterministicAlignedWritableAllocations(t *testing.T) {
	m, _ := watcomHeapFixture(t, 16)
	first := callWatcomHeap(t, m, 1, 0x2000)
	second := callWatcomHeap(t, m, 5, 0x3000)
	if first != 0x100 || second != 0x104 || len(m.Mem) != 0x10c {
		t.Fatalf("allocations first=%X second=%X len=%X", first, second, len(m.Mem))
	}
	if err := m.Write8(second+4, 0x7f); err != nil || m.Mem[second+4] != 0x7f {
		t.Fatalf("allocated memory not writable: %v", err)
	}
	if got := callWatcomHeap(t, m, 8, 0x4000); got != 0 || len(m.Mem) != 0x10c {
		t.Fatalf("exhausted allocation result=%X len=%X", got, len(m.Mem))
	}
}

func TestWatcomNearHeapZeroAndUnregisteredEntry(t *testing.T) {
	m, _ := watcomHeapFixture(t, 16)
	if got := callWatcomHeap(t, m, 0, 0x2000); got != 0 || len(m.Mem) != 0x100 {
		t.Fatalf("zero allocation result=%X len=%X", got, len(m.Mem))
	}
	m.CPU.EIP = 0
	m.Mem[0] = 0xfb
	if err := m.CPU.Step(); err != nil || m.CPU.EIP != 1 {
		t.Fatalf("unregistered entry did not use CPU decoder: EIP=%X err=%v", m.CPU.EIP, err)
	}
}

func TestWatcomNearHeapRejectsUnreadableStack(t *testing.T) {
	m, _ := watcomHeapFixture(t, 16)
	m.CPU.Descriptors = map[uint16]cpu386.Descriptor{}
	if err := m.CPU.Step(); err == nil || m.CPU.EIP != 0x1234 {
		t.Fatalf("unreadable stack err=%v EIP=%X", err, m.CPU.EIP)
	}
}

func TestWatcomMemset(t *testing.T) {
	m, _ := watcomHeapFixture(t, 16)
	service := &WatcomMemset{machine: m, entry: 0x2000}
	m.CPU.StepHook = service.Handle
	m.CPU.EIP = 0x2000
	m.CPU.R[cpu386.ESP] = 0x40
	binary.LittleEndian.PutUint32(m.Mem[0x40:], 0x3000)
	binary.LittleEndian.PutUint32(m.Mem[0x44:], 0x80)
	binary.LittleEndian.PutUint32(m.Mem[0x48:], 0x123456ab)
	binary.LittleEndian.PutUint32(m.Mem[0x4c:], 3)
	if err := m.CPU.Step(); err != nil {
		t.Fatal(err)
	}
	if m.CPU.EIP != 0x3000 || m.CPU.R[cpu386.ESP] != 0x44 || m.CPU.R[cpu386.EAX] != 0x80 || m.Mem[0x80] != 0xab || m.Mem[0x82] != 0xab {
		t.Fatalf("memset EIP=%X ESP=%X EAX=%X bytes=% X", m.CPU.EIP, m.CPU.R[cpu386.ESP], m.CPU.R[cpu386.EAX], m.Mem[0x80:0x83])
	}
}

func TestWatcomMemsetZeroLengthAndBounds(t *testing.T) {
	m, _ := watcomHeapFixture(t, 16)
	service := &WatcomMemset{machine: m, entry: 0x2000}
	m.CPU.StepHook = service.Handle
	for _, test := range []struct {
		destination uint32
		length      uint32
		wantError   bool
	}{{0xffffffff, 0, false}, {0xff, 2, true}} {
		m.CPU.EIP, m.CPU.R[cpu386.ESP] = 0x2000, 0x40
		binary.LittleEndian.PutUint32(m.Mem[0x40:], 0x3000)
		binary.LittleEndian.PutUint32(m.Mem[0x44:], test.destination)
		binary.LittleEndian.PutUint32(m.Mem[0x48:], 0)
		binary.LittleEndian.PutUint32(m.Mem[0x4c:], test.length)
		err := m.CPU.Step()
		if (err != nil) != test.wantError {
			t.Fatalf("destination=%X length=%X err=%v", test.destination, test.length, err)
		}
	}
}
