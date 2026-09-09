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
	if first != 0x100 || second != 0x104 || len(m.Mem) != 0x1000 {
		t.Fatalf("allocations first=%X second=%X len=%X", first, second, len(m.Mem))
	}
	if err := m.Write8(second+4, 0x7f); err != nil || m.Mem[second+4] != 0x7f {
		t.Fatalf("allocated memory not writable: %v", err)
	}
	if got := callWatcomHeap(t, m, 8, 0x4000); got != 0 || len(m.Mem) != 0x1000 {
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

func TestWatcomInitArgvEmptyCommandLine(t *testing.T) {
	m, heap := watcomHeapFixture(t, 32)
	copy(m.Mem[0x80:], []byte{0, 'F', 'D', '2', '.', 'E', 'X', 'E', 0})
	binary.LittleEndian.PutUint32(m.Mem[0x60:], 0x80)
	binary.LittleEndian.PutUint32(m.Mem[0x64:], 0x81)
	binary.LittleEndian.PutUint32(m.Mem[0x40:], 0x3000)
	service := &WatcomInitArgv{machine: m, heap: heap, entry: 0x2000, commandPointer: 0x60,
		programPointer: 0x64, internalArgc: 0x68, internalArgv: 0x6c, publicArgc: 0x70, publicArgv: 0x74}
	m.CPU.StepHook = service.Handle
	m.CPU.EIP = 0x2000
	if err := m.CPU.Step(); err != nil {
		t.Fatal(err)
	}
	argv := uint32(0x101)
	for _, address := range []uint32{0x68, 0x70} {
		if got, _ := m.Read32(address); got != 1 {
			t.Fatalf("argc at %X=%X", address, got)
		}
	}
	for _, address := range []uint32{0x6c, 0x74} {
		if got, _ := m.Read32(address); got != argv {
			t.Fatalf("argv at %X=%X", address, got)
		}
	}
	if first, _ := m.Read32(argv); first != 0x81 || m.CPU.R[cpu386.EAX] != argv || m.CPU.EIP != 0x3000 || m.CPU.R[cpu386.ESP] != 0x44 {
		t.Fatalf("argv[0]=%X EAX=%X EIP=%X ESP=%X", first, m.CPU.R[cpu386.EAX], m.CPU.EIP, m.CPU.R[cpu386.ESP])
	}
}

func TestWatcomInitArgvRejectsArguments(t *testing.T) {
	m, heap := watcomHeapFixture(t, 32)
	m.Mem[0x80] = 'x'
	binary.LittleEndian.PutUint32(m.Mem[0x60:], 0x80)
	binary.LittleEndian.PutUint32(m.Mem[0x64:], 0x81)
	service := &WatcomInitArgv{machine: m, heap: heap, entry: 0x2000, commandPointer: 0x60, programPointer: 0x64}
	m.CPU.StepHook = service.Handle
	m.CPU.EIP = 0x2000
	if err := m.CPU.Step(); err == nil {
		t.Fatal("non-empty command line was accepted")
	}
}

func TestWatcomInt386DPMILockLinearRegion(t *testing.T) {
	m, _ := watcomHeapFixture(t, 16)
	service := &WatcomInt386DPMI{machine: m, entry: 0x2000, host: NewDPMIHost(m)}
	m.CPU.StepHook = service.Handle
	m.CPU.EIP = 0x2000
	m.CPU.R[cpu386.ESP] = 0x20
	for offset, value := range []uint32{0x3000, 0x31, 0x60, 0x80} {
		binary.LittleEndian.PutUint32(m.Mem[0x20+offset*4:], value)
	}
	for i, value := range []uint32{0x0600, 0, 0x40, 0, 0, 0x20, 1} {
		binary.LittleEndian.PutUint32(m.Mem[0x60+i*4:], value)
	}
	if err := m.CPU.Step(); err != nil {
		t.Fatal(err)
	}
	if m.CPU.EIP != 0x3000 || m.CPU.R[cpu386.ESP] != 0x24 || m.CPU.R[cpu386.EAX] != 0x0600 {
		t.Fatalf("int386 EIP=%X ESP=%X EAX=%X", m.CPU.EIP, m.CPU.R[cpu386.ESP], m.CPU.R[cpu386.EAX])
	}
	if cflag := binary.LittleEndian.Uint32(m.Mem[0x80+24:]); cflag != 0 {
		t.Fatalf("DPMI CFLAG=%X", cflag)
	}
}

func TestWatcomInt386DPMIRejectsUnknownInterrupt(t *testing.T) {
	m, _ := watcomHeapFixture(t, 16)
	service := &WatcomInt386DPMI{machine: m, entry: 0x2000, host: NewDPMIHost(m)}
	m.CPU.StepHook = service.Handle
	m.CPU.EIP = 0x2000
	m.CPU.R[cpu386.ESP] = 0x20
	for offset, value := range []uint32{0x3000, 0x10, 0x60, 0x80} {
		binary.LittleEndian.PutUint32(m.Mem[0x20+offset*4:], value)
	}
	if err := m.CPU.Step(); err == nil || m.CPU.EIP != 0x2000 || m.CPU.R[cpu386.ESP] != 0x20 {
		t.Fatalf("unknown int err=%v EIP=%X ESP=%X", err, m.CPU.EIP, m.CPU.R[cpu386.ESP])
	}
}

func TestWatcomInt386DPMIRejectsUnknownFunction(t *testing.T) {
	for _, test := range []struct {
		eax    uint32
		start  uint32
		length uint32
	}{{0x0601, 0x40, 0x20}} {
		m, _ := watcomHeapFixture(t, 16)
		service := &WatcomInt386DPMI{machine: m, entry: 0x2000, host: NewDPMIHost(m)}
		m.CPU.StepHook = service.Handle
		m.CPU.EIP = 0x2000
		m.CPU.R[cpu386.ESP] = 0x20
		for offset, value := range []uint32{0x3000, 0x31, 0x60, 0x80} {
			binary.LittleEndian.PutUint32(m.Mem[0x20+offset*4:], value)
		}
		for i, value := range []uint32{test.eax, test.start >> 16, test.start & 0xffff, 0, test.length >> 16, test.length & 0xffff, 1} {
			binary.LittleEndian.PutUint32(m.Mem[0x60+i*4:], value)
		}
		if err := m.CPU.Step(); err == nil || m.CPU.EIP != 0x2000 || m.CPU.R[cpu386.ESP] != 0x20 {
			t.Fatalf("eax=%X start=%X length=%X err=%v EIP=%X ESP=%X", test.eax, test.start, test.length, err, m.CPU.EIP, m.CPU.R[cpu386.ESP])
		}
	}
}

func TestWatcomHeapPageBackingAndNonPagingLock(t *testing.T) {
	for _, tc := range []struct {
		length uint32
		reject bool
	}{{41, false}, {0xf00, false}, {0xf01, true}} {
		m, h := watcomHeapFixture(t, 40)
		addr := callWatcomHeap(t, m, 40, 0x3000)
		if addr != 0x100 || h.next != 0x128 || len(m.Mem) != 0x1000 {
			t.Fatalf("配置或頁容量錯誤: %x %x %x", addr, h.next, len(m.Mem))
		}
		if got := callWatcomHeap(t, m, 1, 0x3000); got != 0 {
			t.Fatal("不得將頁尾空間當成可配置容量")
		}
		for _, b := range m.Mem[0x128:] {
			if b != 0 {
				t.Fatal("新增頁非零")
			}
		}
		svc := &WatcomInt386DPMI{machine: m, entry: 0x2000, host: NewDPMIHost(m)}
		m.CPU.StepHook = svc.Handle
		m.CPU.EIP = 0x2000
		m.CPU.R[cpu386.ESP] = 0x20
		for i, v := range []uint32{0x3000, 0x31, 0x60, 0x80} {
			binary.LittleEndian.PutUint32(m.Mem[0x20+i*4:], v)
		}
		for i, v := range []uint32{0x0600, 0, addr, 0, 0, tc.length, 1} {
			binary.LittleEndian.PutUint32(m.Mem[0x60+i*4:], v)
		}
		sizeBefore := len(m.Mem)
		err := m.CPU.Step()
		if err != nil {
			t.Fatal(err)
		}
		if len(m.Mem) != sizeBefore || binary.LittleEndian.Uint32(m.Mem[0x80+24:]) != 0 {
			t.Fatal("非分頁鎖定改變容量或CFLAG")
		}
		if len(svc.host.Locks) != 1 || svc.host.Locks[0].Base != addr || svc.host.Locks[0].Size != tc.length {
			t.Fatal("未共用host紀錄")
		}
	}
}

func TestWatcomDOSAllocationBridgeDoesNotOverlapHeap(t *testing.T) {
	m, h := watcomHeapFixture(t, 0x10000)
	a := callWatcomHeap(t, m, 40, 0x3000)
	m.Mem[a] = 0x5a
	host := NewDPMIHost(m)
	m.CPU.SetDescriptor(0x100, cpu386.Descriptor{Base: 0x55, Limit: 0xff})
	svc := &WatcomInt386DPMI{machine: m, entry: 0x2000, host: host, heap: h}
	m.CPU.StepHook = svc.Handle
	m.CPU.EIP = 0x2000
	m.CPU.R[cpu386.ESP] = 0x20
	for i, v := range []uint32{0x3000, 0x31, 0x60, 0x80} {
		binary.LittleEndian.PutUint32(m.Mem[0x20+i*4:], v)
	}
	for i, v := range []uint32{0x0100, 8, 0xaabb, 0xccdd, 0xeeff, 0x1122, 1} {
		binary.LittleEndian.PutUint32(m.Mem[0x60+i*4:], v)
	}
	before := m.CPU.R
	flags := m.CPU.EFlags
	if err := m.CPU.Step(); err != nil {
		t.Fatal(err)
	}
	seg := binary.LittleEndian.Uint32(m.Mem[0x80:]) & 0xffff
	sel := uint16(binary.LittleEndian.Uint32(m.Mem[0x8c:]))
	d := m.CPU.Descriptors[sel]
	if seg<<4 != d.Base || d.Base < 0x1000 || d.Limit != 127 || sel == 0x100 || !d.Writable {
		t.Fatalf("DOS配置不正確: seg=%x sel=%x descriptor=%+v", seg, sel, d)
	}
	if m.CPU.EFlags != flags {
		t.Fatal("轉接改寫呼叫者旗標")
	}
	for i, v := range before {
		if i != cpu386.EAX && i != cpu386.ESP && m.CPU.R[i] != v {
			t.Fatal("轉接改寫呼叫者暫存器")
		}
	}
	m.Mem[d.Base] = 0xa5
	m.CPU.StepHook = h.Handle
	b := callWatcomHeap(t, m, 16, 0x3000)
	if b < d.Base+128 || m.Mem[a] != 0x5a || m.Mem[d.Base] != 0xa5 {
		t.Fatalf("配置互相覆蓋 a=%x dos=%x b=%x", a, d.Base, b)
	}
	if m.CPU.Descriptors[0x100].Base != 0x55 {
		t.Fatal("覆蓋既有selector")
	}
}

func TestWatcomHeapFreeReuseAndCoalesce(t *testing.T) {
	m, h := watcomHeapFixture(t, 24)
	a := callWatcomHeap(t, m, 8, 0x2000)
	b := callWatcomHeap(t, m, 8, 0x2000)
	c := callWatcomHeap(t, m, 8, 0x2000)
	m.Mem[a] = 0x55
	m.Mem[c] = 0xaa
	if err := h.release(b); err != nil {
		t.Fatal(err)
	}
	if err := h.release(a); err != nil {
		t.Fatal(err)
	}
	if len(h.freeRanges) != 1 || h.freeRanges[0].Size != 16 || m.Mem[a] != 0x55 {
		t.Fatal("釋放合併或改動bytes")
	}
	d := callWatcomHeap(t, m, 4, 0x2000)
	if d != a || m.Mem[d] != 0 || m.Mem[c] != 0xaa {
		t.Fatal("重用／清零影響活物件")
	}
	e := callWatcomHeap(t, m, 12, 0x2000)
	if e != a+4 || len(h.freeRanges) != 0 {
		t.Fatal("first-fit分割")
	}
	if got := callWatcomHeap(t, m, 1, 0x2000); got != 0 {
		t.Fatal("容量越界")
	}
	if err := h.release(e + 4); err == nil {
		t.Fatal("内部指標")
	}
	if err := h.release(c); err != nil {
		t.Fatal(err)
	}
	if err := h.release(c); err == nil {
		t.Fatal("重複釋放")
	}
	if err := h.release(0); err != nil {
		t.Fatal("NULL")
	}
}
func TestWatcomNFreeCdeclAndFailure(t *testing.T) {
	m, h := watcomHeapFixture(t, 32)
	h.freeEntry = 0x2222
	h.freeFlagAddress = 0xf0
	a := callWatcomHeap(t, m, 8, 0x2000)
	invoke := func(address uint32) error {
		m.CPU.EIP = 0x2222
		m.CPU.R[cpu386.ESP] = 0x40
		binary.LittleEndian.PutUint32(m.Mem[0x40:], 0x3333)
		binary.LittleEndian.PutUint32(m.Mem[0x44:], address)
		return m.CPU.Step()
	}
	m.CPU.R[cpu386.EAX] = 0x1234
	m.CPU.EFlags = 0xed7
	m.Mem[0xf0] = 0x55
	before := m.CPU.R
	if err := invoke(a); err != nil {
		t.Fatal(err)
	}
	if m.CPU.EIP != 0x3333 || m.CPU.R[cpu386.ESP] != 0x44 || m.CPU.EFlags != 0xed7 || m.Mem[0xf0] != 0 {
		t.Fatal("cdecl／旗標")
	}
	for i, v := range before {
		if i != cpu386.ESP && m.CPU.R[i] != v {
			t.Fatal("暫存器")
		}
	}
	m.Mem[0xf0] = 0x55
	if err := invoke(a); err == nil || m.CPU.R[cpu386.ESP] != 0x40 || m.CPU.EIP != 0x2222 || m.Mem[0xf0] != 0x55 {
		t.Fatal("錯誤發布")
	}
	if err := invoke(0); err != nil || m.Mem[0xf0] != 0 {
		t.Fatal("NULL仍有原版外層旗標副作用")
	}
}

func TestWatcomDPMIFreeDOSBridge(t *testing.T) {
	m, h := watcomHeapFixture(t, 0x10000)
	host := NewDPMIHost(m)
	svc := &WatcomInt386DPMI{machine: m, entry: 0x2000, host: host, heap: h}
	m.CPU.StepHook = svc.Handle
	invoke := func(fn, bx, dx uint32) {
		m.CPU.EIP = 0x2000
		m.CPU.R[cpu386.ESP] = 0x20
		for i, v := range []uint32{0x3000, 0x31, 0x60, 0x80} {
			binary.LittleEndian.PutUint32(m.Mem[0x20+i*4:], v)
		}
		for i, v := range []uint32{fn, bx, 0, dx, 0, 0, 1} {
			binary.LittleEndian.PutUint32(m.Mem[0x60+i*4:], v)
		}
		if err := m.CPU.Step(); err != nil {
			t.Fatal(err)
		}
	}
	invoke(0x0100, 8, 0)
	sel := uint16(binary.LittleEndian.Uint32(m.Mem[0x8c:]))
	if _, ok := m.CPU.Descriptors[sel]; !ok {
		t.Fatal("未配置")
	}
	invoke(0x0101, 0, uint32(sel))
	if _, ok := m.CPU.Descriptors[sel]; ok {
		t.Fatal("未刪descriptor")
	}
	if len(host.DOSMemory()) != 0 || binary.LittleEndian.Uint32(m.Mem[0x98:]) != 0 {
		t.Fatal("未釋放")
	}
	invoke(0x0101, 0, uint32(sel))
	if binary.LittleEndian.Uint32(m.Mem[0x80:])&0xffff != 0x8022 || binary.LittleEndian.Uint32(m.Mem[0x98:]) != 1 {
		t.Fatal("未回報無效selector")
	}
}

func TestFD2HeapDOSAndLinearIsolation(t *testing.T) {
	m, _ := watcomHeapFixture(t, 16)
	host := NewDPMIHost(m)
	h, err := InstallFD2WatcomRuntime(m, host)
	if err != nil {
		t.Fatal(err)
	}
	a, ok := h.allocate(32)
	if !ok || a < 0x100000 {
		t.Fatal("FD2 heap未避開VGA")
	}
	m.Mem[a] = 0x55
	dos, ok := host.allocDOS(16)
	if !ok || dos+256 > 0xa0000 {
		t.Fatal("DOS配置侵入VGA")
	}
	m.Mem[dos] = 0x66
	linear, ok := host.alloc(32)
	if !ok || linear < a+32 {
		t.Fatal("DPMI覆寫heap")
	}
	m.Mem[linear] = 0x77
	b, ok := h.allocate(32)
	if !ok || b < linear+32 {
		t.Fatal("heap覆寫DPMI")
	}
	if m.Mem[a] != 0x55 || m.Mem[dos] != 0x66 || m.Mem[linear] != 0x77 {
		t.Fatal("交錯配置污染")
	}
	if err := h.release(a); err != nil {
		t.Fatal(err)
	}
	reused, ok := h.allocate(16)
	if !ok || reused != a || m.Mem[linear] != 0x77 {
		t.Fatal("高位空閒區重用")
	}
	host.dosBrk = 0x9fff0
	if _, ok := host.allocDOS(1); !ok {
		t.Fatal("640KiB末段應可配置")
	}
	before := len(m.Mem)
	if _, ok := host.allocDOS(1); ok || len(m.Mem) != before {
		t.Fatal("640KiB之外必須拒絕")
	}
}
