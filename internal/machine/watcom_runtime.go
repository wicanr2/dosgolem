package machine

import (
	"encoding/binary"
	"fmt"
	"sort"

	"github.com/wicanr2/dosgolem/internal/cpu386"
)

// WatcomNearHeap 模擬已由證據登錄的 Watcom 32-bit _nmalloc 入口。
// 它只處理指定入口；其他 EIP 必須回到 CPU 的一般解碼路徑。
type WatcomHeapRange struct{ Base, Size uint32 }

type WatcomNearHeap struct {
	base            uint32
	freeEntry       uint32
	freeFlagAddress uint32
	live            map[uint32]uint32
	freeRanges      []WatcomHeapRange
	machine         *LEMachine
	entry           uint32
	next            uint32
	limit           uint32
	backedEnd       uint32
}

type WatcomMemset struct {
	machine *LEMachine
	entry   uint32
}

type WatcomInt386DPMI struct {
	machine *LEMachine
	entry   uint32
	host    *DPMIHost
	heap    *WatcomNearHeap
}

type WatcomInitArgv struct {
	machine        *LEMachine
	heap           *WatcomNearHeap
	entry          uint32
	commandPointer uint32
	programPointer uint32
	internalArgc   uint32
	internalArgv   uint32
	publicArgc     uint32
	publicArgv     uint32
}

func NewWatcomNearHeap(m *LEMachine, entry, capacity uint32) (*WatcomNearHeap, error) {
	if m == nil || m.CPU == nil || capacity == 0 {
		return nil, fmt.Errorf("machine: Watcom near heap 參數無效")
	}
	base64 := (uint64(len(m.Mem)) + 3) &^ uint64(3)
	limit64 := base64 + uint64(capacity)
	if limit64 > uint64(^uint32(0)) {
		return nil, fmt.Errorf("machine: Watcom near heap 位址溢位")
	}
	return &WatcomNearHeap{base: uint32(base64), live: make(map[uint32]uint32), machine: m, entry: entry, next: uint32(base64), limit: uint32(limit64), backedEnd: uint32(len(m.Mem))}, nil
}

func (h *WatcomNearHeap) Handle(c *cpu386.CPU) (bool, error) {
	if h.freeEntry != 0 && c.EIP == h.freeEntry {
		return h.handleFree(c)
	}

	if c.EIP != h.entry {
		return false, nil
	}
	stack, ok := c.Descriptors[c.Seg[cpu386.SegSS]]
	if !ok || stack.Base != 0 || c.R[cpu386.ESP] > stack.Limit || stack.Limit-c.R[cpu386.ESP] < 7 {
		return true, fmt.Errorf("machine: Watcom _nmalloc cdecl 堆疊不可讀")
	}
	ret, err := h.machine.Read32(c.R[cpu386.ESP])
	if err != nil {
		return true, fmt.Errorf("machine: Watcom _nmalloc 返回位址：%w", err)
	}
	size, err := h.machine.Read32(c.R[cpu386.ESP] + 4)
	if err != nil {
		return true, fmt.Errorf("machine: Watcom _nmalloc 參數：%w", err)
	}
	result, _ := h.allocate(size)
	c.R[cpu386.EAX] = result
	c.R[cpu386.ESP] += 4
	c.EIP = ret
	return true, nil
}

func (h *WatcomNearHeap) allocate(size uint32) (uint32, bool) {
	if size == 0 {
		return 0, true
	}
	aligned64 := (uint64(size) + 3) &^ uint64(3)
	if aligned64 > uint64(^uint32(0)) {
		return 0, false
	}
	for i, r := range h.freeRanges {
		if uint64(r.Size) < aligned64 {
			continue
		}
		addr := r.Base
		n := uint32(aligned64)
		if uint64(addr)+uint64(n) > uint64(len(h.machine.Mem)) {
			return 0, false
		}
		if r.Size == n {
			h.freeRanges = append(h.freeRanges[:i], h.freeRanges[i+1:]...)
		} else {
			h.freeRanges[i].Base += n
			h.freeRanges[i].Size -= n
		}
		clear(h.machine.Mem[addr : addr+n])
		h.live[addr] = n
		return addr, true
	}
	// 外部 DPMI 配置不可被後續 heap 物件覆蓋。
	if uint64(len(h.machine.Mem)) > uint64(h.backedEnd) {
		next := uint32((uint64(len(h.machine.Mem)) + 3) &^ uint64(3))
		if next > h.next {
			h.next = next
		}
		h.backedEnd = uint32(len(h.machine.Mem))
	}
	end64 := uint64(h.next) + aligned64
	if aligned64 > uint64(^uint32(0)) || end64 > uint64(h.limit) {
		return 0, false
	}
	end := uint32(end64)
	// 物件邊界與 DPMI 背書頁分離；保留 next/limit 的配置容量。
	backedEnd := (end64 + 4095) &^ uint64(4095)
	if backedEnd > uint64(len(h.machine.Mem)) {
		h.machine.Mem = append(h.machine.Mem, make([]byte, int(backedEnd-uint64(len(h.machine.Mem))))...)
	}
	h.backedEnd = uint32(len(h.machine.Mem))
	result := h.next
	h.next = end
	h.live[result] = uint32(aligned64)
	return result, true
}

func (h *WatcomNearHeap) release(address uint32) error {
	if address == 0 {
		return nil
	}
	size, ok := h.live[address]
	if !ok {
		return fmt.Errorf("machine: 未知或重複近堆釋放 %08X", address)
	}
	delete(h.live, address)
	h.freeRanges = append(h.freeRanges, WatcomHeapRange{address, size})
	sort.Slice(h.freeRanges, func(i, j int) bool { return h.freeRanges[i].Base < h.freeRanges[j].Base })
	merged := h.freeRanges[:0]
	for _, r := range h.freeRanges {
		if len(merged) > 0 && uint64(merged[len(merged)-1].Base)+uint64(merged[len(merged)-1].Size) == uint64(r.Base) {
			merged[len(merged)-1].Size += r.Size
		} else {
			merged = append(merged, r)
		}
	}
	h.freeRanges = merged
	return nil
}
func (h *WatcomNearHeap) handleFree(c *cpu386.CPU) (bool, error) {

	stack, ok := c.Descriptors[c.Seg[cpu386.SegSS]]
	esp := c.R[cpu386.ESP]
	if !ok || stack.Base != 0 || esp > stack.Limit || stack.Limit-esp < 7 || uint64(esp)+8 > uint64(len(h.machine.Mem)) {
		return true, fmt.Errorf("machine: _nfree堆疊無效")
	}
	ret, err := h.machine.Read32(esp)
	if err != nil {
		return true, err
	}
	address, err := h.machine.Read32(esp + 4)
	if err != nil {
		return true, err
	}
	if h.freeFlagAddress >= uint32(len(h.machine.Mem)) {
		return true, fmt.Errorf("machine: _nfree旗標位址無效")
	}
	if err := h.release(address); err != nil {
		return true, err
	}
	h.machine.Mem[h.freeFlagAddress] = 0
	c.EIP = ret
	c.R[cpu386.ESP] += 4
	return true, nil
}

func (s *WatcomMemset) Handle(c *cpu386.CPU) (bool, error) {
	if c.EIP != s.entry {
		return false, nil
	}
	stack, ok := c.Descriptors[c.Seg[cpu386.SegSS]]
	if !ok || stack.Base != 0 || c.R[cpu386.ESP] > stack.Limit || stack.Limit-c.R[cpu386.ESP] < 15 {
		return true, fmt.Errorf("machine: Watcom memset cdecl 堆疊不可讀")
	}
	ret, err := s.machine.Read32(c.R[cpu386.ESP])
	if err != nil {
		return true, fmt.Errorf("machine: Watcom memset 返回位址：%w", err)
	}
	destination, err := s.machine.Read32(c.R[cpu386.ESP] + 4)
	if err != nil {
		return true, fmt.Errorf("machine: Watcom memset 目的地：%w", err)
	}
	value, err := s.machine.Read32(c.R[cpu386.ESP] + 8)
	if err != nil {
		return true, fmt.Errorf("machine: Watcom memset 填充值：%w", err)
	}
	length, err := s.machine.Read32(c.R[cpu386.ESP] + 12)
	if err != nil {
		return true, fmt.Errorf("machine: Watcom memset 長度：%w", err)
	}
	end := uint64(destination) + uint64(length)
	if length != 0 && end > uint64(len(s.machine.Mem)) {
		return true, fmt.Errorf("machine: Watcom memset 範圍 0x%X+0x%X 超界", destination, length)
	}
	for address := uint64(destination); address < end; address++ {
		s.machine.Mem[address] = byte(value)
	}
	c.R[cpu386.EAX] = destination
	c.R[cpu386.ESP] += 4
	c.EIP = ret
	return true, nil
}

func (s *WatcomInitArgv) Handle(c *cpu386.CPU) (bool, error) {
	if c.EIP != s.entry {
		return false, nil
	}
	stack, ok := c.Descriptors[c.Seg[cpu386.SegSS]]
	if !ok || stack.Base != 0 || c.R[cpu386.ESP] > stack.Limit || stack.Limit-c.R[cpu386.ESP] < 3 {
		return true, fmt.Errorf("machine: Watcom __Init_Argv 返回堆疊不可讀")
	}
	ret, err := s.machine.Read32(c.R[cpu386.ESP])
	if err != nil {
		return true, fmt.Errorf("machine: Watcom __Init_Argv 返回位址：%w", err)
	}
	command, err := s.machine.Read32(s.commandPointer)
	if err != nil || uint64(command) >= uint64(len(s.machine.Mem)) || s.machine.Mem[command] != 0 {
		return true, fmt.Errorf("machine: Watcom __Init_Argv 只支援已驗證的空 command line")
	}
	program, err := s.machine.Read32(s.programPointer)
	if err != nil || uint64(program) >= uint64(len(s.machine.Mem)) {
		return true, fmt.Errorf("machine: Watcom __Init_Argv program pointer 無效")
	}
	foundNUL := false
	for address := uint64(program); address < uint64(len(s.machine.Mem)); address++ {
		if s.machine.Mem[address] == 0 {
			foundNUL = true
			break
		}
	}
	if !foundNUL {
		return true, fmt.Errorf("machine: Watcom __Init_Argv program name 未終止")
	}
	base, ok := s.heap.allocate(9)
	if !ok {
		return true, fmt.Errorf("machine: Watcom __Init_Argv 配置失敗")
	}
	argv := base + 1
	s.machine.Mem[base] = 0
	binary.LittleEndian.PutUint32(s.machine.Mem[argv:], program)
	binary.LittleEndian.PutUint32(s.machine.Mem[argv+4:], 0)
	for _, output := range []struct{ address, value uint32 }{
		{s.internalArgc, 1}, {s.internalArgv, argv}, {s.publicArgc, 1}, {s.publicArgv, argv},
	} {
		if uint64(output.address)+4 > uint64(len(s.machine.Mem)) {
			return true, fmt.Errorf("machine: Watcom __Init_Argv output 0x%X 超界", output.address)
		}
		binary.LittleEndian.PutUint32(s.machine.Mem[output.address:], output.value)
	}
	c.R[cpu386.EAX] = argv
	c.R[cpu386.ESP] += 4
	c.EIP = ret
	return true, nil
}

func (s *WatcomInt386DPMI) Handle(c *cpu386.CPU) (bool, error) {
	if c.EIP != s.entry {
		return false, nil
	}
	stack, ok := c.Descriptors[c.Seg[cpu386.SegSS]]
	if !ok || stack.Base != 0 || c.R[cpu386.ESP] > stack.Limit || stack.Limit-c.R[cpu386.ESP] < 15 {
		return true, fmt.Errorf("machine: Watcom int386 cdecl 堆疊不可讀")
	}
	ret, err := s.machine.Read32(c.R[cpu386.ESP])
	if err != nil {
		return true, fmt.Errorf("machine: Watcom int386 回傳位址：%w", err)
	}
	intno, err := s.machine.Read32(c.R[cpu386.ESP] + 4)
	if err != nil || (intno != 0x31 && intno != 0x10 && intno != 0x16) {
		return true, fmt.Errorf("machine: Watcom int386 只支援已驗證的 INT31／INT10／INT16子集")
	}
	in, errIn := s.machine.Read32(c.R[cpu386.ESP] + 8)
	out, errOut := s.machine.Read32(c.R[cpu386.ESP] + 12)
	if errIn != nil || errOut != nil || uint64(in)+28 > uint64(len(s.machine.Mem)) || uint64(out)+28 > uint64(len(s.machine.Mem)) {
		return true, fmt.Errorf("machine: Watcom int386 REGS 範圍無效")
	}
	var regs [7]uint32
	for i := range regs {
		regs[i] = binary.LittleEndian.Uint32(s.machine.Mem[in+uint32(i*4):])
	}
	if intno == 0x16 {
		if s.machine.Keyboard == nil || byte(regs[0]>>8) != 0x10 {
			return true, fmt.Errorf("machine: INT16 AH=%02X尚未支援或裝置未安裝", byte(regs[0]>>8))
		}
		value, ready, err := s.machine.Keyboard.ReadEnhanced()
		if err != nil {
			return true, err
		}
		if !ready {
			return true, nil
		}
		regs[0] = regs[0]&0xffff0000 | uint32(value)
	} else if intno == 0x10 {
		if s.machine.Video == nil {
			return true, fmt.Errorf("machine: LE視訊裝置未安裝")
		}
		shadow := *c
		order := [...]int{cpu386.EAX, cpu386.EBX, cpu386.ECX, cpu386.EDX, cpu386.ESI, cpu386.EDI}
		for i, r := range order {
			shadow.R[r] = regs[i]
		}
		if !s.machine.Video.Handle(&shadow) {
			return true, fmt.Errorf("machine: INT10 AX=%04X尚未支援或VRAM未背書", uint16(regs[0]))
		}
		for i, r := range order {
			regs[i] = shadow.R[r]
		}
		regs[6] = 0
	} else {
		switch uint16(regs[0]) {
		case 0x0100, 0x0101, 0x0600:
			if s.host == nil {
				return true, fmt.Errorf("machine: Watcom DPMI host 未綁定")
			}
			if uint16(regs[0]) == 0x0100 && s.heap != nil && s.heap.base < dosMemTop && s.host.dosBrk < s.heap.backedEnd {
				s.host.dosBrk = (s.heap.backedEnd + 15) &^ 15
			}
			shadow := *c
			order := [...]int{cpu386.EAX, cpu386.EBX, cpu386.ECX, cpu386.EDX, cpu386.ESI, cpu386.EDI}
			for i, r := range order {
				shadow.R[r] = regs[i]
			}
			if !s.host.Handle(&shadow) {
				return true, fmt.Errorf("machine: DPMI %04Xh 未實作", uint16(regs[0]))
			}
			for i, r := range order {
				regs[i] = shadow.R[r]
			}
			regs[6] = 0
			if shadow.EFlags&cpu386.CF != 0 {
				regs[6] = 1
			}
		default:
			return true, fmt.Errorf("machine: Watcom int386 未支援 DPMI AX=%04X", uint16(regs[0]))
		}
	}
	for i, value := range regs {
		binary.LittleEndian.PutUint32(s.machine.Mem[out+uint32(i*4):], value)
	}
	c.R[cpu386.EAX] = regs[0]
	c.R[cpu386.ESP] += 4
	c.EIP = ret
	return true, nil
}

// InstallFD2WatcomRuntime 登錄固定雜湊 FD2.EXE 已證實的 Watcom runtime 入口。
func InstallFD2WatcomRuntime(m *LEMachine, hosts ...*DPMIHost) (*WatcomNearHeap, error) {
	return InstallFD2WatcomRuntimeWithHeapCapacity(m, 1024*1024, hosts...)
}

// InstallFD2WatcomRuntimeWithHeapCapacity 明示驗證環境的近堆預算；不模擬原版配置位址。
func InstallFD2WatcomRuntimeWithHeapCapacity(m *LEMachine, capacity uint32, hosts ...*DPMIHost) (*WatcomNearHeap, error) {
	if capacity == 0 || capacity > 64*1024*1024 {
		return nil, fmt.Errorf("machine: FD2近堆容量需為1至67108864 bytes")
	}
	heap, err := NewWatcomNearHeap(m, 0x36d26, capacity)
	if err != nil {
		return nil, err
	}
	if heap.base < 0x100000 {
		heap.base = 0x100000
		heap.next = heap.base
		heap.limit = heap.base + capacity
	}
	heap.freeEntry = 0x37426
	heap.freeFlagAddress = 0x5419c
	memset := &WatcomMemset{machine: m, entry: 0x375c0}
	argv := &WatcomInitArgv{machine: m, heap: heap, entry: 0x46114, commandPointer: 0x52808,
		programPointer: 0x5280c, internalArgc: 0x527f8, internalArgv: 0x527fc,
		publicArgc: 0x5462c, publicArgv: 0x54628}
	host := NewDPMIHost(m)
	if len(hosts) > 0 && hosts[0] != nil {
		host = hosts[0]
		if host.m != m {
			host.Attach(m)
		}
	}
	int386 := &WatcomInt386DPMI{machine: m, entry: 0x36d98, host: host, heap: heap}
	m.CPU.StepHook = func(c *cpu386.CPU) (bool, error) {
		if handled, err := heap.Handle(c); handled || err != nil {
			return handled, err
		}
		if handled, err := memset.Handle(c); handled || err != nil {
			return handled, err
		}
		if handled, err := argv.Handle(c); handled || err != nil {
			return handled, err
		}
		return int386.Handle(c)
	}
	return heap, nil
}
