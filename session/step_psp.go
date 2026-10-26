package session

import "fmt"

// CurrentPSP returns the actual DOS process segment inside this callback.
// A process can reuse a previous PSP; this is not a program identity (spec 465).
func (v StepView) CurrentPSP() uint16 {
	if !v.live() {
		return 0
	}
	return v.o.dos.CurrentPSP()
}

// LoadCount includes successful program and overlay loads already recorded by
// DOS. It is only comparable within this Owner, not across snapshot restores.
func (v StepView) LoadCount() uint64 {
	if !v.live() {
		return 0
	}
	return uint64(len(v.o.dos.ExecLog))
}

// ReadPSP copies physical RAM relative to the current DOS PSP (spec 465).
// It neither consumes nor reopens the Owner's separate ReadBootPSP window.
func (v StepView) ReadPSP(ranges []BootPSPRange) (BootPSPReceipt, error) {
	if !v.live() {
		return BootPSPReceipt{}, fmt.Errorf("session: PSP read outside current StepView")
	}
	psp := v.o.dos.CurrentPSP()
	if psp == 0 {
		return BootPSPReceipt{}, fmt.Errorf("session: current PSP is zero")
	}
	if len(ranges) == 0 || len(ranges) > 32 {
		return BootPSPReceipt{}, fmt.Errorf("session: PSP range count out of bounds")
	}
	base := uint64(psp) * 16
	total := uint64(0)
	for _, r := range ranges {
		n := uint64(len(r.Dst))
		if n == 0 || n > 4096 {
			return BootPSPReceipt{}, fmt.Errorf("session: PSP range size out of bounds")
		}
		total += n
		if total > 16*1024 {
			return BootPSPReceipt{}, fmt.Errorf("session: PSP batch size out of bounds")
		}
		start := base + uint64(r.Offset)
		if start+n > uint64(len(v.o.machine.Mem)) {
			return BootPSPReceipt{}, fmt.Errorf("session: PSP physical range out of bounds")
		}
	}
	for _, r := range ranges {
		start := base + uint64(r.Offset)
		copy(r.Dst, v.o.machine.Mem[start:start+uint64(len(r.Dst))])
	}
	return BootPSPReceipt{PSP: psp}, nil
}
