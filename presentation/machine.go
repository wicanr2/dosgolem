// Package presentation contains generic bridges between a dosgolem Machine
// and a host presenter. It deliberately has no window-backend dependency.
package presentation

import (
	"fmt"

	"github.com/wicanr2/dosgolem/host"
	"github.com/wicanr2/dosgolem/internal/machine"
)

// MachineFrameSource supplies the current Machine display to a host
// PresentationSnapshotProvider.
//
// ReadPresentationFrame must be called by the same goroutine that steps
// Machine. In particular, callers must not call it concurrently with Step:
// Indexed and Palette are individually safe copies, but a concurrent Step
// could pair two different DOS presentation instants. This type never steps
// the machine and never writes to it.
type MachineFrameSource struct {
	machine *machine.Machine
}

// NewMachineFrameSource constructs a read-only frame source for m.
func NewMachineFrameSource(m *machine.Machine) (*MachineFrameSource, error) {
	if m == nil {
		return nil, fmt.Errorf("presentation: MachineFrameSource 的 machine 不得為 nil")
	}
	return &MachineFrameSource{machine: m}, nil
}

// ReadPresentationFrame returns one indexed/palette pair from the current
// machine state. Machine.Indexed returns its own bytes copy and Palette is a
// value, so neither result grants the host a write path into DOS state.
func (s *MachineFrameSource) ReadPresentationFrame() (host.IndexedFrame, error) {
	if s == nil || s.machine == nil {
		return host.IndexedFrame{}, fmt.Errorf("presentation: MachineFrameSource 不得為 nil")
	}
	w, h := s.machine.VideoSize()
	if w <= 0 || h <= 0 {
		return host.IndexedFrame{}, fmt.Errorf("presentation: machine 回報無效畫布 %dx%d", w, h)
	}
	indexed := s.machine.Indexed()
	if len(indexed) != w*h {
		return host.IndexedFrame{}, fmt.Errorf("presentation: machine indexed 長度=%d，預期 %d (%dx%d)", len(indexed), w*h, w, h)
	}
	return host.IndexedFrame{
		Canvas:  host.Canvas{Width: w, Height: h},
		Indexed: indexed,
		Palette: s.machine.Palette(),
	}, nil
}
