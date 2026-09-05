package machine

import (
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
	result := uint32(0)
	if size != 0 {
		aligned64 := (uint64(size) + 3) &^ uint64(3)
		end64 := uint64(h.next) + aligned64
		if aligned64 <= uint64(^uint32(0)) && end64 <= uint64(h.limit) {
			end := uint32(end64)
			if uint64(end) > uint64(len(h.machine.Mem)) {
				h.machine.Mem = append(h.machine.Mem, make([]byte, int(uint64(end)-uint64(len(h.machine.Mem))))...)
			}
			result = h.next
			h.next = end
		}
	}
	c.R[cpu386.EAX] = result
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
	m.CPU.StepHook = heap.Handle
	return heap, nil
}
