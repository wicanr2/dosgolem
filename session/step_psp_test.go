package session

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/wicanr2/dosgolem/internal/cpu"
	"github.com/wicanr2/dosgolem/internal/machine"
)

type step465Observer struct {
	fn func(StepView) error
}

func (s *step465Observer) BeforeStep(v StepView) error { return s.fn(v) }
func (*step465Observer) VideoWrite(machine.VideoWrite) {}
func (*step465Observer) Frame([]byte, [256][3]uint8)   {}

// Obtain a real, currently open callback view. The private runner stops on the
// callback's sentinel, before executing an instruction; no fake live/token is set.
func step465Callback(t *testing.T, o *Owner, fn func(StepView)) {
	t.Helper()
	stop := errors.New("465 test callback complete")
	old := o.observer
	o.observer = &step465Observer{fn: func(v StepView) error { fn(v); return stop }}
	defer func() { o.observer = old }()
	steps := o.machine.Steps
	_, raw, observed := o.runObserved(1)
	if raw != nil || !errors.Is(observed, stop) || o.machine.Steps != steps {
		t.Fatalf("callback boundary: raw=%v observed=%v steps=%d want=%d", raw, observed, o.machine.Steps, steps)
	}
}

func TestStep465PhysicalOwnershipAndNoMutation(t *testing.T) {
	o := bootPSPOwner(t)
	bootPSPSet(t, o, 0x4321)
	const address = 0x43210 + 0x120
	copy(o.machine.Mem[address:], []byte{11, 22, 33, 44, 55, 66, 77, 88})
	// A distinct linear mapping has different bytes and untouched A/D bits.
	binary.LittleEndian.PutUint32(o.machine.Mem[0x1000:], 0x2007)
	binary.LittleEndian.PutUint32(o.machine.Mem[0x2000+(address>>12)*4:], 0x90007)
	o.machine.Mem[0x90000+(address&4095)] = 99
	if e := o.machine.SetCR3(0x1000); e != nil {
		t.Fatal(e)
	}
	if e := o.machine.SetCR0(0x80000001); e != nil {
		t.Fatal(e)
	}
	step465Callback(t, o, func(v StepView) {
		before := bootPSPSnap(t, o)
		dst := make([]byte, 5)
		for i := 0; i < 2; i++ {
			r, e := v.ReadPSP([]BootPSPRange{{0x120, dst[:4]}, {0x125, dst[2:]}})
			if e != nil || r.PSP != 0x4321 || !bytes.Equal(dst, []byte{11, 22, 66, 77, 88}) {
				t.Fatalf("physical/overlap receipt=%+v dst=%v err=%v", r, dst, e)
			}
			if v.CurrentPSP() != 0x4321 || v.LoadCount() != 0 {
				t.Fatal("wrong callback scalar")
			}
		}
		if !reflect.DeepEqual(before, bootPSPSnap(t, o)) {
			t.Fatal("read mutated VM, memory, page tables, owner or 464 window")
		}
		dst[0] = 0xee
		if o.machine.Mem[address] != 11 {
			t.Fatal("caller buffer aliases RAM")
		}
	})
	// Follow the actual DOS field, including zero, without a production setter.
	for _, psp := range []uint16{0x1234, 0xffff, 0} {
		bootPSPSet(t, o, psp)
		step465Callback(t, o, func(v StepView) {
			if v.CurrentPSP() != psp {
				t.Fatalf("PSP=%x want=%x", v.CurrentPSP(), psp)
			}
			if psp == 0 {
				before := bootPSPSnap(t, o)
				dst := []byte{0xa5}
				if r, e := v.ReadPSP([]BootPSPRange{{0, dst}}); e == nil || r != (BootPSPReceipt{}) || dst[0] != 0xa5 {
					t.Fatal("zero PSP was not atomic")
				}
				if !reflect.DeepEqual(before, bootPSPSnap(t, o)) {
					t.Fatal("zero PSP mutated state")
				}
			}
		})
	}
}

func TestStep465PreflightAtomicAndRetry(t *testing.T) {
	cases := []string{"empty", "range33", "nilDst", "size4097", "total16385", "badSecond", "tail", "maxOffset"}
	for _, name := range cases {
		t.Run(name, func(t *testing.T) {
			o := bootPSPOwner(t)
			step465Callback(t, o, func(v StepView) {
				last := uint32(len(o.machine.Mem)) - uint32(o.dos.CurrentPSP())*16 - 1
				dst := []byte{0xa5, 0xa5}
				var ranges []BootPSPRange
				switch name {
				case "range33":
					for i := 0; i < 33; i++ {
						ranges = append(ranges, BootPSPRange{0, []byte{0xa5}})
					}
				case "nilDst":
					ranges = []BootPSPRange{{0, nil}}
				case "size4097":
					ranges = []BootPSPRange{{0, bytes.Repeat([]byte{0xa5}, 4097)}}
				case "total16385":
					for i := 0; i < 4; i++ {
						ranges = append(ranges, BootPSPRange{0, bytes.Repeat([]byte{0xa5}, 4096)})
					}
					ranges = append(ranges, BootPSPRange{0, dst[:1]})
				case "badSecond":
					ranges = []BootPSPRange{{0x100, dst}, {0xffffffff, []byte{0xa5}}}
				case "tail":
					ranges = []BootPSPRange{{last, dst}}
				case "maxOffset":
					ranges = []BootPSPRange{{0xffffffff, dst}}
				}
				original := make([][]byte, len(ranges))
				for i, r := range ranges {
					original[i] = append([]byte(nil), r.Dst...)
				}
				before := bootPSPSnap(t, o)
				if r, e := v.ReadPSP(ranges); e == nil || r != (BootPSPReceipt{}) {
					t.Fatalf("rejection=%+v %v", r, e)
				}
				for i, r := range ranges {
					if !bytes.Equal(r.Dst, original[i]) {
						t.Fatalf("partial Dst write at range%d", i)
					}
				}
				if !reflect.DeepEqual(before, bootPSPSnap(t, o)) || o.viewMisuse {
					t.Fatal("parameter error changed state or fault")
				}
				if _, e := v.ReadPSP([]BootPSPRange{{0, dst}}); e != nil {
					t.Fatal("valid retry rejected", e)
				}
			})
		})
	}
}

func TestStep465PositiveLimitsAnd464WindowIsolation(t *testing.T) {
	for _, n := range []int{4, 32} {
		t.Run(fmt.Sprintf("ranges%d", n), func(t *testing.T) {
			o := bootPSPOwner(t)
			base := uint32(o.dos.CurrentPSP()) * 16
			for i := 0; i < 16384; i++ {
				o.machine.Mem[int(base)+i] = byte(i*17 + 3)
			}
			step465Callback(t, o, func(v StepView) {
				ranges := make([]BootPSPRange, n)
				for i := range ranges {
					ranges[i] = BootPSPRange{uint32(i * (16384 / n)), make([]byte, 16384/n)}
				}
				before := bootPSPSnap(t, o)
				if r, e := v.ReadPSP(ranges); e != nil || r.PSP != o.dos.CurrentPSP() {
					t.Fatalf("boundary %+v %v", r, e)
				}
				for i, r := range ranges {
					for j, b := range r.Dst {
						if b != byte((i*(16384/n)+j)*17+3) {
							t.Fatalf("boundary bytes %d/%d", i, j)
						}
					}
				}
				last := uint32(len(o.machine.Mem)) - base - 1
				d := []byte{0}
				want := o.machine.Mem[len(o.machine.Mem)-1]
				if _, e := v.ReadPSP([]BootPSPRange{{last, d}}); e != nil || d[0] != want {
					t.Fatal("last physical byte", e)
				}
				if !reflect.DeepEqual(before, bootPSPSnap(t, o)) {
					t.Fatal("boundary changed state")
				}
			})
			// Callback reads have not used the independent boot-only batch.
			if _, e := o.ReadBootPSP([]BootPSPRange{{0, make([]byte, 1)}}); e != nil {
				t.Fatal("step read consumed 464", e)
			}
		})
	}
	for _, used := range []bool{false, true} {
		for _, closed := range []bool{false, true} {
			o := bootPSPOwner(t)
			o.bootPSPUsed, o.bootPSPClosed = used, closed
			step465Callback(t, o, func(v StepView) {
				before := bootPSPSnap(t, o)
				if _, e := v.ReadPSP([]BootPSPRange{{0, make([]byte, 1)}}); e != nil {
					t.Fatal("465 inherited 464 guard", e)
				}
				if !reflect.DeepEqual(before, bootPSPSnap(t, o)) {
					t.Fatal("465 changed prior 464 flags")
				}
			})
		}
	}
}

// Exercise the installed DOS INT handler, including the real loader and
// process-stack/queue paths. No test appends ExecLog or assigns a load count.
func step465INT(t *testing.T, o *Owner, ax uint16) {
	t.Helper()
	o.machine.CPU.R[cpu.AX] = ax
	if !o.machine.CPU.IntHook(o.machine.CPU, 0x21) {
		t.Fatalf("installed INT21 rejected AX=%04x", ax)
	}
}

func step465Load(t *testing.T, o *Owner, name string, overlay bool) {
	t.Helper()
	m, c := o.machine, o.machine.CPU
	c.Seg[cpu.DS], c.R[cpu.DX] = 0x5000, 0
	c.Seg[cpu.ES], c.R[cpu.BX] = 0x5000, 0x100
	m.WriteBytes(0x50000, append([]byte(name), 0))
	m.WriteBytes(0x50100, make([]byte, 16))
	ax := uint16(0x4b00)
	if overlay {
		m.Write16(0x50100, 0x6000)
		m.Write16(0x50102, 0x6000)
		ax = 0x4b03
	}
	step465INT(t, o, ax)
}

func TestStep465RealDOSLoadsAndSamePSPReuse(t *testing.T) {
	o := startSyntheticOwner(t, []byte{0x90, 0xeb, 0xfd})
	o.dos.Root = t.TempDir()
	child := []byte{0xb8, 0x2a, 0x4c, 0xcd, 0x21}
	if e := os.WriteFile(filepath.Join(o.dos.Root, "CHILD.COM"), child, 0600); e != nil {
		t.Fatal(e)
	}
	mz := minimalBootMZ(t)
	if e := os.WriteFile(filepath.Join(o.dos.Root, "OVL.EXE"), mz, 0600); e != nil {
		t.Fatal(e)
	}
	if e := os.WriteFile(filepath.Join(o.dos.Root, "BAD.EXE"), []byte("MZ"), 0600); e != nil {
		t.Fatal(e)
	}
	step465Callback(t, o, func(v StepView) {
		if v.LoadCount() != 0 {
			t.Fatal("new owner inherited history")
		}
		parent := v.CurrentPSP()
		var first uint16
		for attempt := uint64(1); attempt <= 3; attempt++ {
			step465Load(t, o, "CHILD.COM", false)
			if o.machine.CPU.Flags&cpu.CF != 0 {
				t.Fatalf("spawn failed AX=%x Missing=%v", o.machine.CPU.R[cpu.AX], o.dos.Missing)
			}
			psp := v.CurrentPSP()
			if psp == parent || psp != o.dos.CurrentPSP() || v.LoadCount() != attempt {
				t.Fatalf("spawn%d PSP=%x count=%d", attempt, psp, v.LoadCount())
			}
			if attempt == 1 {
				first = psp
			} else if psp != first {
				t.Fatalf("fixture did not reuse PSP: %x/%x", psp, first)
			}
			got := make([]byte, len(child))
			if r, e := v.ReadPSP([]BootPSPRange{{0x100, got}}); e != nil || r.PSP != psp || !bytes.Equal(got, child) {
				t.Fatal("actual child bytes", r, e, got)
			}
			step465INT(t, o, 0x4c2a)
			if v.CurrentPSP() != parent || v.LoadCount() != attempt || o.dos.Exited {
				t.Fatal("child terminate changed load count or failed to return")
			}
		}
		for _, name := range []string{"MISSING.COM", "BAD.EXE"} {
			step465Load(t, o, name, false)
			if o.machine.CPU.Flags&cpu.CF == 0 || v.LoadCount() != 3 || v.CurrentPSP() != parent {
				t.Fatalf("failed spawn %s changed count/PSP", name)
			}
		}
		step465Load(t, o, "OVL.EXE", true)
		if o.machine.CPU.Flags&cpu.CF != 0 || v.LoadCount() != 4 || v.CurrentPSP() != parent {
			t.Fatal("successful overlay count/PSP")
		}
		if len(o.dos.Overlays) != 1 || o.dos.Overlays[0].Seg != 0x6000 {
			t.Fatal("overlay loader did not run")
		}
		for _, name := range []string{"MISSING.EXE", "BAD.EXE"} {
			step465Load(t, o, name, true)
			if o.machine.CPU.Flags&cpu.CF == 0 || v.LoadCount() != 4 || v.CurrentPSP() != parent {
				t.Fatal("failed overlay changed count/PSP")
			}
		}
		step465INT(t, o, 0x4c00)
		if !o.dos.Exited || v.LoadCount() != 4 {
			t.Fatal("empty queue termination added load")
		}
	})
}

func TestStep465RealDOSQueuedHandoff(t *testing.T) {
	for _, good := range []bool{true, false} {
		t.Run(fmt.Sprintf("success=%v", good), func(t *testing.T) {
			o := startSyntheticOwner(t, []byte{0x90, 0xeb, 0xfd})
			o.dos.Root = t.TempDir()
			if good {
				if e := os.WriteFile(filepath.Join(o.dos.Root, "NEXT.COM"), []byte{0xb8, 7, 0x4c, 0xcd, 0x21}, 0600); e != nil {
					t.Fatal(e)
				}
			}
			o.dos.Enqueue("NEXT.COM", "")
			step465Callback(t, o, func(v StepView) {
				if v.LoadCount() != 0 {
					t.Fatal("queue was counted before load")
				}
				step465INT(t, o, 0x4c00)
				if good {
					if o.dos.Exited || v.LoadCount() != 1 || v.CurrentPSP() != o.dos.CurrentPSP() {
						t.Fatal("queued success missing")
					}
					got := make([]byte, 5)
					if _, e := v.ReadPSP([]BootPSPRange{{0x100, got}}); e != nil || !bytes.Equal(got, []byte{0xb8, 7, 0x4c, 0xcd, 0x21}) {
						t.Fatal("queued bytes", e, got)
					}
					step465INT(t, o, 0x4c07)
					if !o.dos.Exited || v.LoadCount() != 1 {
						t.Fatal("queued final terminate added load")
					}
				} else if !o.dos.Exited || v.LoadCount() != 0 || len(o.dos.Missing) == 0 {
					t.Fatal("queued failure counted a load")
				}
			})
		})
	}
}

func step465Invalid(t *testing.T, v StepView, method string) {
	t.Helper()
	switch method {
	case "PSP":
		if v.CurrentPSP() != 0 {
			t.Fatal("invalid PSP not zero")
		}
	case "Load":
		if v.LoadCount() != 0 {
			t.Fatal("invalid load count not zero")
		}
	case "Read":
		dst := []byte{0xa5}
		if r, e := v.ReadPSP([]BootPSPRange{{0, dst}}); e == nil || r != (BootPSPReceipt{}) || dst[0] != 0xa5 {
			t.Fatal("invalid read not atomic", r, e, dst)
		}
	}
}

func TestStep465ZeroView(t *testing.T) {
	for _, method := range []string{"PSP", "Load", "Read"} {
		t.Run(method, func(t *testing.T) { step465Invalid(t, StepView{}, method) })
	}
}

type step465StaleObserver struct {
	t            *testing.T
	o            *Owner
	method       string
	frame        bool
	kept         StepView
	seen         int
	used, forced bool
}

func (s *step465StaleObserver) misuse() {
	step465Invalid(s.t, s.kept, s.method)
	s.used = true
	s.forced = s.o.filter.Force
}
func (s *step465StaleObserver) BeforeStep(v StepView) error {
	s.seen++
	if s.seen == 1 {
		s.kept = v
	} else if !s.frame && !s.used {
		s.misuse()
	}
	return nil
}
func (*step465StaleObserver) VideoWrite(machine.VideoWrite) {}
func (s *step465StaleObserver) Frame([]byte, [256][3]uint8) {
	if s.frame && !s.used {
		s.misuse()
	}
}
func (*step465StaleObserver) TriggerIPs() IPSet { return IPSet{} }
func (s *step465StaleObserver) NextWake() uint64 {
	if !s.frame && s.seen == 1 {
		return 1
	}
	return NoWake
}
func (*step465StaleObserver) FrameAt(uint64) {}

func TestStep465RetainedAndFrameViewsFault(t *testing.T) {
	for _, method := range []string{"PSP", "Load", "Read"} {
		for _, frame := range []bool{false, true} {
			for _, sparse := range []bool{false, true} {
				t.Run(fmt.Sprintf("%s/frame=%v/sparse=%v", method, frame, sparse), func(t *testing.T) {
					s := &step465StaleObserver{t: t, method: method, frame: frame}
					o := startSyntheticOwner(t, []byte{0x90, 0xeb, 0xfd})
					s.o = o
					o.observer = s
					o.sparse = sparse
					r, e := runTurn(t, o, InstructionBudget(machine.DefaultVGAFrameEvery+10))
					if e == nil || r.Reason != StopReasonObserverFault || !s.used || !s.forced || o.Status().Phase != PhaseFailed {
						t.Fatalf("missing fault/force r=%+v err=%v used=%v forced=%v", r, e, s.used, s.forced)
					}
					if !frame && r.Steps != 2 {
						t.Fatalf("stale view stopped after %d steps want2", r.Steps)
					}
					if frame && (o.machine.Frames != 1 || r.Steps > machine.DefaultVGAFrameEvery+1) {
						t.Fatal("frame misuse did not stop at next attempt")
					}
				})
			}
		}
	}
}
