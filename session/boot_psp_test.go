package session

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"encoding/gob"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/wicanr2/dosgolem/internal/state"
)

func bootPSPOwner(t *testing.T) *Owner {
	t.Helper()
	cfg := stateConfig()
	cfg.ExtendedMemoryKB = 8192
	cfg.ProtectedModeProfile = true
	o, err := New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { o.Close() })
	exe := minimalBootMZ(t)
	if _, e := o.BootOriginal(BootInput{EXE: exe, ExpectedEXESHA256: sha256.Sum256(exe), SaveRoot: t.TempDir()}); e != nil {
		t.Fatal(e)
	}
	return o
}

// This tiny DOS-state fixture sets an actual nondefault CurPSP without guest
// execution, a new production setter, unsafe, or knowledge of private layout.
func bootPSPSet(t *testing.T, o *Owner, psp uint16) {
	t.Helper()
	var b bytes.Buffer
	x := struct {
		Magic   string
		Version int
		CurPSP  uint16
	}{"DOSGOLEM-D", 3, psp}
	if e := gob.NewEncoder(&b).Encode(x); e != nil {
		t.Fatal(e)
	}
	if e := o.dos.LoadState(&b); e != nil {
		t.Fatal(e)
	}
}

type bootPSPSnapshot struct {
	Status                                Status
	Gen, Epoch, Steps, Frames, Deliveries uint64
	Used, Closed                          bool
	Digest                                StateDigest
	Full                                  FullDigest
	Mem                                   [32]byte
}

func bootPSPSnap(t *testing.T, o *Owner) bootPSPSnapshot {
	t.Helper()
	full, e := fullDigestOf(o.machine, o.dos)
	if e != nil {
		t.Fatal(e)
	}
	return bootPSPSnapshot{o.Status(), o.generation, o.epoch, o.machine.Steps, o.machine.Frames, o.machine.Deliveries(), o.bootPSPUsed, o.bootPSPClosed, digestOf(o.machine, o.dos), full, sha256.Sum256(o.machine.Mem[:])}
}

func TestBootPSPPhysicalBatchAndOwnership(t *testing.T) {
	o := bootPSPOwner(t)
	bootPSPSet(t, o, 0x4321)
	base := uint64(0x4321) * 16
	copy(o.machine.Mem[base+0x120:], []byte{11, 22, 33, 44, 55})
	// Map the linear source page elsewhere. Physical data must still be read,
	// with no A/D page-table writes or translation side effects.
	binary.LittleEndian.PutUint32(o.machine.Mem[0x1000:], 0x2000|7)
	binary.LittleEndian.PutUint32(o.machine.Mem[0x2000+((base+0x120)>>12)*4:], 0x90000|7)
	if e := o.machine.SetCR3(0x1000); e != nil {
		t.Fatal(e)
	}
	if e := o.machine.SetCR0(0x80000001); e != nil {
		t.Fatal(e)
	}
	o.machine.Mem[0x90000+((base+0x120)&4095)] = 99
	if _, e := o.View(); e != nil {
		t.Fatal(e)
	}
	o.Status()
	if _, e := o.Digest(); e != nil {
		t.Fatal(e)
	}
	if _, e := o.MachineDigest(); e != nil {
		t.Fatal(e)
	}
	before := bootPSPSnap(t, o)
	dst := make([]byte, 6)
	r, e := o.ReadBootPSP([]BootPSPRange{{0x120, dst[:4]}, {0x122, dst[2:5]}})
	if e != nil || r.PSP != 0x4321 || !bytes.Equal(dst, []byte{11, 22, 33, 44, 55, 0}) {
		t.Fatalf("physical receipt=%+v dst=%v err=%v", r, dst, e)
	}
	after := bootPSPSnap(t, o)
	after.Used = before.Used
	if !reflect.DeepEqual(before, after) {
		t.Fatal("read changed owner or VM outside private Used")
	}
	dst[0] = 77
	if o.machine.Mem[base+0x120] != 11 {
		t.Fatal("caller aliases VM")
	}
}

func TestBootPSPOverlapLaterRangeWins(t *testing.T) {
	o := bootPSPOwner(t)
	p := uint64(o.dos.CurrentPSP()) * 16
	copy(o.machine.Mem[p+0x100:], []byte{1, 2, 3, 4, 5, 6, 7, 8})
	dst := make([]byte, 5)
	_, e := o.ReadBootPSP([]BootPSPRange{{0x100, dst[:4]}, {0x105, dst[2:]}})
	if e != nil || !bytes.Equal(dst, []byte{1, 2, 6, 7, 8}) {
		t.Fatalf("dst=%v err=%v", dst, e)
	}
}

func TestBootPSPPreflightAtomicRetry(t *testing.T) {
	cases := []struct {
		name string
		make func([]byte) []BootPSPRange
	}{
		{"empty", func(d []byte) []BootPSPRange { return nil }},
		{"range33", func(d []byte) []BootPSPRange {
			r := make([]BootPSPRange, 33)
			for i := range r {
				r[i] = BootPSPRange{0, make([]byte, 1)}
			}
			return r
		}},
		{"emptyRange", func(d []byte) []BootPSPRange { return []BootPSPRange{{0, nil}} }},
		{"range4097", func(d []byte) []BootPSPRange { return []BootPSPRange{{0, make([]byte, 4097)}} }},
		{"total16385", func(d []byte) []BootPSPRange {
			return []BootPSPRange{{0, make([]byte, 4096)}, {0, make([]byte, 4096)}, {0, make([]byte, 4096)}, {0, make([]byte, 4096)}, {0, make([]byte, 1)}}
		}},
		{"secondOutOfBounds", func(d []byte) []BootPSPRange { return []BootPSPRange{{0x100, d}, {0xffffffff, make([]byte, 1)}} }},
		{"endOutOfBounds", func(d []byte) []BootPSPRange { return []BootPSPRange{{uint32(len(bootPSPDummyMem)) - 1, d}} }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			o := bootPSPOwner(t)
			dst := []byte{0xaa, 0xaa}
			ranges := tc.make(dst)
			before := bootPSPSnap(t, o)
			r, e := o.ReadBootPSP(ranges)
			if e == nil || r != (BootPSPReceipt{}) {
				t.Fatalf("expected zero rejection: %+v %v", r, e)
			}
			if !reflect.DeepEqual(before, bootPSPSnap(t, o)) || !bytes.Equal(dst, []byte{0xaa, 0xaa}) {
				t.Fatal("non-atomic rejection")
			}
			if _, e = o.ReadBootPSP([]BootPSPRange{{0x100, dst}}); e != nil {
				t.Fatal("rejection consumed read", e)
			}
		})
	}
}

var bootPSPDummyMem [1 << 20]byte

func TestBootPSPBoundaryLimitsAndOnce(t *testing.T) {
	for _, n := range []int{4, 32} {
		t.Run(string(rune('A'+n)), func(t *testing.T) {
			o := bootPSPOwner(t)
			ranges := make([]BootPSPRange, n)
			for i := range ranges {
				ranges[i] = BootPSPRange{0, make([]byte, 16384/n)}
			}
			if _, e := o.ReadBootPSP(ranges); e != nil {
				t.Fatal(e)
			}
			before := bootPSPSnap(t, o)
			dst := []byte{0xaa}
			if r, e := o.ReadBootPSP([]BootPSPRange{{0, dst}}); e == nil || r != (BootPSPReceipt{}) {
				t.Fatal("second read accepted")
			}
			if !reflect.DeepEqual(before, bootPSPSnap(t, o)) || dst[0] != 0xaa {
				t.Fatal("second read mutated state")
			}
		})
	}
	o := bootPSPOwner(t)
	last := uint32(len(o.machine.Mem)) - uint32(o.dos.CurrentPSP())*16 - 1
	o.machine.Mem[len(o.machine.Mem)-1] = 0x75
	d := []byte{0}
	if _, e := o.ReadBootPSP([]BootPSPRange{{last, d}}); e != nil || d[0] != 0x75 {
		t.Fatal("last physical byte", e)
	}
}

func TestBootPSPPhaseAndZeroPSP(t *testing.T) {
	var nilOwner *Owner
	if r, e := nilOwner.ReadBootPSP(nil); e == nil || r != (BootPSPReceipt{}) {
		t.Fatal("nil accepted")
	}
	for _, phase := range []Phase{PhaseBooting, PhaseStopped, PhaseFailed, PhaseClosed} {
		o := bootPSPOwner(t)
		o.phase = phase
		before := bootPSPSnap(t, o)
		if r, e := o.ReadBootPSP([]BootPSPRange{{0, make([]byte, 1)}}); e == nil || r != (BootPSPReceipt{}) {
			t.Fatal("phase accepted", phase)
		}
		if !reflect.DeepEqual(before, bootPSPSnap(t, o)) {
			t.Fatal("phase rejection mutated owner")
		}
	}
	o := bootPSPOwner(t)
	bootPSPSet(t, o, 0)
	before := bootPSPSnap(t, o)
	if r, e := o.ReadBootPSP([]BootPSPRange{{0, make([]byte, 1)}}); e == nil || r != (BootPSPReceipt{}) {
		t.Fatal("zero PSP accepted")
	}
	if !reflect.DeepEqual(before, bootPSPSnap(t, o)) {
		t.Fatal("zero PSP rejection mutated owner")
	}
	bootPSPSet(t, o, 0x1234)
	if _, e := o.ReadBootPSP([]BootPSPRange{{0, make([]byte, 1)}}); e != nil {
		t.Fatal(e)
	}
}

func TestBootPSPAttemptWindow(t *testing.T) {
	for _, name := range []string{"deliverError", "advanceError", "deliverOK"} {
		t.Run(name, func(t *testing.T) {
			o := bootPSPOwner(t)
			switch name {
			case "deliverError":
				o.Deliver(CapturedUpdate{})
			case "advanceError":
				o.Advance(0)
			case "deliverOK":
				v, e := o.View()
				if e != nil {
					t.Fatal(e)
				}
				if _, e = o.Deliver(CapturedUpdate{StartedPanel: v.Panel, Layout: v.Layout, SourceGeneration: v.SourceGeneration}); e != nil {
					t.Fatal(e)
				}
			}
			if !o.bootPSPClosed {
				t.Fatal("attempt did not close window")
			}
			// Restore only phase/turn state in this private test to distinguish the
			// window guard from ordinary failed-phase rejection after bad inputs.
			o.phase = PhaseRunning
			o.turnPending = false
			before := bootPSPSnap(t, o)
			dst := []byte{0xaa}
			if r, e := o.ReadBootPSP([]BootPSPRange{{0, dst}}); e == nil || r != (BootPSPReceipt{}) {
				t.Fatal("closed window accepted")
			}
			if !reflect.DeepEqual(before, bootPSPSnap(t, o)) || dst[0] != 0xaa {
				t.Fatal("window error changed prior fault/phase")
			}
		})
	}
}

func TestBootPSPStateBootAndSnapshotBytes(t *testing.T) {
	source := bootPSPOwner(t)
	root := t.TempDir()
	p := filepath.Join(t.TempDir(), "zero.state")
	if e := state.Save(p, source.machine, source.dos); e != nil {
		t.Fatal(e)
	}
	raw, e := os.ReadFile(p)
	if e != nil {
		t.Fatal(e)
	}
	cfg := stateConfig()
	cfg.ExtendedMemoryKB = 8192
	cfg.ProtectedModeProfile = true
	o := newStateOwner(t, cfg)
	if _, e = o.BootFromState(StateInput{State: raw, ExpectedStateSHA256: sha256.Sum256(raw), SaveRoot: root}); e != nil {
		t.Fatal(e)
	}
	if _, e = o.View(); e != nil {
		t.Fatal(e)
	}
	if _, e = o.Digest(); e != nil {
		t.Fatal(e)
	}
	if _, e = o.MachineDigest(); e != nil {
		t.Fatal(e)
	}
	o.Status()
	var before, after bytes.Buffer
	if e = o.machine.SaveState(&before); e != nil {
		t.Fatal(e)
	}
	d := make([]byte, 256)
	want := append([]byte(nil), o.machine.Mem[uint64(o.dos.CurrentPSP())*16:][:256]...)
	if _, e = o.ReadBootPSP([]BootPSPRange{{0, d}}); e != nil || !bytes.Equal(d, want) {
		t.Fatal("state bootstrap read", e)
	}
	if e = o.machine.SaveState(&after); e != nil {
		t.Fatal(e)
	}
	if !bytes.Equal(before.Bytes(), after.Bytes()) {
		t.Fatal("machine snapshot bytes changed")
	}
}
