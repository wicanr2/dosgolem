package session

import "fmt"

// BootPSPRange names physical RAM relative to the actual DOS PSP (spec 464).
// Dst belongs to the caller and is never retained.
type BootPSPRange struct {
	Offset uint32
	Dst    []byte
}

// BootPSPReceipt contains no live machine view.
type BootPSPReceipt struct{ PSP uint16 }

// ReadBootPSP copies one bounded batch before the first Deliver/Advance attempt.
// Like other Owner methods, it requires the single frontend goroutine.
// Rejections neither consume the batch nor change the owner fault or phase.
func (o *Owner) ReadBootPSP(ranges []BootPSPRange) (BootPSPReceipt, error) {
	if o == nil {
		return BootPSPReceipt{}, fmt.Errorf("session: nil Owner")
	}
	if o.phase != PhaseRunning || o.closed || o.bootPSPClosed || o.bootPSPUsed {
		return BootPSPReceipt{}, fmt.Errorf("session: bootstrap PSP window unavailable")
	}
	psp := o.dos.CurrentPSP()
	if psp == 0 {
		return BootPSPReceipt{}, fmt.Errorf("session: bootstrap PSP is zero")
	}
	if len(ranges) == 0 || len(ranges) > 32 {
		return BootPSPReceipt{}, fmt.Errorf("session: bootstrap range count out of bounds")
	}
	base := uint64(psp) * 16
	total := uint64(0)
	for _, r := range ranges {
		n := uint64(len(r.Dst))
		if n == 0 || n > 4096 {
			return BootPSPReceipt{}, fmt.Errorf("session: bootstrap range size out of bounds")
		}
		total += n
		if total > 16*1024 {
			return BootPSPReceipt{}, fmt.Errorf("session: bootstrap batch size out of bounds")
		}
		start := base + uint64(r.Offset)
		if start+n > uint64(len(o.machine.Mem)) {
			return BootPSPReceipt{}, fmt.Errorf("session: bootstrap physical range out of bounds")
		}
	}
	o.bootPSPUsed = true
	for _, r := range ranges {
		start := base + uint64(r.Offset)
		copy(r.Dst, o.machine.Mem[start:start+uint64(len(r.Dst))])
	}
	return BootPSPReceipt{PSP: psp}, nil
}
