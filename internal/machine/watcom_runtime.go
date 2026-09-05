package machine

import (
	"encoding/binary"
	"fmt"

	"github.com/wicanr2/dosgolem/internal/cpu386"
)

// WatcomNearHeap 模擬已由證據登錄的 Watcom 32-bit _nmalloc 入口。
// 它只處理指定入口；其他 EIP 必須回到 CPU 的一般解碼路徑。
type WatcomNearHeap struct {
	machine *LEMachine
	entry   uint32
	next    uint32
	limit   uint32
}

type WatcomMemset struct {
	machine *LEMachine
	entry   uint32
}

type WatcomInt386DPMI struct {
	machine *LEMachine
	entry   uint32
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
	return &WatcomNearHeap{machine: m, entry: entry, next: uint32(base64), limit: uint32(limit64)}, nil
}

func (h *WatcomNearHeap) Handle(c *cpu386.CPU) (bool, error) {
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
	end64 := uint64(h.next) + aligned64
	if aligned64 > uint64(^uint32(0)) || end64 > uint64(h.limit) {
		return 0, false
	}
	end := uint32(end64)
	if uint64(end) > uint64(len(h.machine.Mem)) {
		h.machine.Mem = append(h.machine.Mem, make([]byte, int(uint64(end)-uint64(len(h.machine.Mem))))...)
	}
	result := h.next
	h.next = end
	return result, true
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
	if err != nil || intno != 0x31 {
		return true, fmt.Errorf("machine: Watcom int386 只支援已驗證的 INT 31h")
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
	if uint16(regs[0]) != 0x0600 {
		return true, fmt.Errorf("machine: Watcom int386 未支援 DPMI AX=%04X", uint16(regs[0]))
	}
	start := uint64(uint16(regs[1]))<<16 | uint64(uint16(regs[2]))
	length := uint64(uint16(regs[4]))<<16 | uint64(uint16(regs[5]))
	if length == 0 || start+length < start || start+length > uint64(len(s.machine.Mem)) {
		return true, fmt.Errorf("machine: DPMI 0600h 線性區域 0x%X+0x%X 無效", start, length)
	}
	regs[6] = 0
	for i, value := range regs {
		binary.LittleEndian.PutUint32(s.machine.Mem[out+uint32(i*4):], value)
	}
	c.R[cpu386.EAX] = regs[0]
	c.R[cpu386.ESP] += 4
	c.EIP = ret
	return true, nil
}

// InstallFD2WatcomRuntime 登錄固定雜湊 FD2.EXE 已證實的 Watcom runtime 入口。
func InstallFD2WatcomRuntime(m *LEMachine) (*WatcomNearHeap, error) {
	heap, err := NewWatcomNearHeap(m, 0x36d26, 1024*1024)
	if err != nil {
		return nil, err
	}
	memset := &WatcomMemset{machine: m, entry: 0x375c0}
	argv := &WatcomInitArgv{machine: m, heap: heap, entry: 0x46114, commandPointer: 0x52808,
		programPointer: 0x5280c, internalArgc: 0x527f8, internalArgv: 0x527fc,
		publicArgc: 0x5462c, publicArgv: 0x54628}
	int386 := &WatcomInt386DPMI{machine: m, entry: 0x36d98}
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
