package strikecommander

import (
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"reflect"
	"runtime"
	"runtime/debug"
	"strings"
	"testing"
	"unsafe"

	"github.com/wicanr2/dosgolem/internal/machine"
	"github.com/wicanr2/dosgolem/session"
)

func revAdvance(r *presRig, sparse bool, target uint64) {
	r.v.cs, r.v.ip = 0x9999, 0x1234
	f := machine.StepFilter{IPs: [1024]uint64(r.so.TriggerIPs()), Wake: r.p.nextWake()}
	for r.v.steps < target {
		r.v.steps++
		if !sparse || f.ShouldObserve(r.v.ip, r.v.steps) {
			r.p.Observe(r.v)
			f.Wake = r.p.nextWake()
		}
	}
}

func TestFReviewCrossStates(t *testing.T) {
	type sample struct {
		Info    PresentInfo
		Counts  PresentCounts
		Records []PresentFallbackRec
		Verdict FrameVerdict
	}
	for _, scenario := range []string{"pause0-timeout", "pause15-timeout", "offscreen-timeout", "erase-draw", "erase-flush", "window-timeout-CU-return"} {
		var got []sample
		for _, sparse := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/sparse%v", scenario, sparse), func(t *testing.T) {
				r := newPresRig(t, 0)
				fBoot(r)
				if strings.HasPrefix(scenario, "pause") {
					dEnter(r, "O.shape", 0xfc00, nil)
					start := r.v.steps
					r.write(5, 9, dCS(dRow(t, r.p, "O.shape")), 0x20, machine.VWV86)
					r.p.VideoModeChange(machine.ModeChange{Mode: 0x13, Step: r.v.steps})
					r.frame(nil)
					revAdvance(r, sparse, start+199000)
					k := 0
					if scenario == "pause15-timeout" {
						k = 15
					}
					e := dEvent(k)
					dEnter(r, "O.shape", 0xfb00, &e)
					dReturn(r, "O.shape", 0xfb00)
					revAdvance(r, sparse, start+200000)
					if len(r.p.stack) != 0 || r.p.f.waitFrame {
						t.Fatal("absorbed timeout still blocks ending")
					}
					keys := []int{2}
					if k == 15 {
						keys = []int{17, 19}
					}
					for _, key := range keys {
						e = dEvent(key)
						if key == 2 {
							e.X, e.Y = 134, 127
						}
						dEnter(r, "O.shape", 0xfc00, &e)
						dReturn(r, "O.shape", 0xfc00)
					}
					if r.p.Current().Kind != PresShape || r.p.Current().Seq != 1 || r.p.counts.Fallback != 0 {
						t.Fatal("ending capture missing", r.p.Current())
					}
				} else {
					if scenario == "offscreen-timeout" {
						dEnter(r, "O.shape", 0xfc00, nil)
					}
					windowStart := r.v.steps
					r.inject(r.text("GEAR", 10, 0xFFFF, qList))
					r.cuIn(ipCUDiff, 1, tCUSP)
					r.copyFB()
					if strings.HasPrefix(scenario, "erase") {
						dEnter(r, "O.erase", 0xfc00, nil)
						r.write(5, r.v.vga[5]^255, dCS(dRow(t, r.p, "O.erase")), 0x30, machine.VWV86)
					} else if scenario == "window-timeout-CU-return" {
						dEnter(r, "O.shape", 0xfc00, nil)
						windowStart = r.v.steps
						r.write(5, r.v.vga[5]^255, dCS(dRow(t, r.p, "O.shape")), 0x20, machine.VWV86)
					}
					r.p.VideoModeChange(machine.ModeChange{Mode: 0x13, Step: r.v.steps})
					r.frame(nil)
					if strings.HasPrefix(scenario, "erase") {
						dReturn(r, "O.erase", 0xfc00)
					} else {
						revAdvance(r, sparse, windowStart+200000)
					}
					if scenario == "offscreen-timeout" && r.p.f.waitFrame {
						t.Fatal("unrelated timeout changed CU wait")
					}
					r.cuRet(ipCUDiffRet, tCUSP)
					r.at(0x9999, 0x1234)
					switch scenario {
					case "offscreen-timeout":
						if r.p.counts.Fallback != 1 {
							t.Fatal("CU next Observe blocked by unrelated timeout")
						}
					case "window-timeout-CU-return":
						if r.p.counts.Fallback != 0 || !r.p.f.returnObserved {
							t.Fatal("did not preserve wait until Frame")
						}
						r.frame(nil)
						if r.p.counts.Fallback != 1 || r.p.LastVerdict().Class != VerdictEligible {
							t.Fatal("Frame dragged beyond already observed return", r.p.LastVerdict())
						}
					default:
						if r.p.counts.CUResidue != 1 || r.p.counts.Fallback != 0 || !r.p.d.eraseGap {
							t.Fatal("CUResidue bypassed gap", r.p.Current())
						}
						if r.p.nextWake() == 0 {
							t.Fatal("gap spins on returnPending")
						}
						a := newSparseAudit(r.so)
						a.oracle(r.so, r.v.steps, 0x1234)
						if strings.Contains(strings.Join(a.fails, "\n"), "延後退路待取像") {
							t.Fatal("gap oracle spins", a.fails)
						}
						if scenario == "erase-flush" {
							r.frame(nil)
						} else {
							dEnter(r, "O.shape", 0xfc00, nil)
							r.write(5, 9, dCS(dRow(t, r.p, "O.shape")), 0x20, machine.VWV86)
							dReturn(r, "O.shape", 0xfc00)
						}
						if r.p.counts.Fallback != 1 || r.p.d.eraseGap {
							t.Fatal("gap never completed")
						}
					}
				}
				got = append(got, sample{r.p.Current(), r.p.Counts(), r.p.Fallbacks(), r.p.LastVerdict()})
			})
		}
		if len(got) == 2 && !reflect.DeepEqual(got[0], got[1]) {
			t.Fatal("dense/sparse cross state differs", scenario, got)
		}
	}
}

func TestFSourceFirstNewKeyNoAlloc(t *testing.T) {
	gc := debug.SetGCPercent(-1)
	defer debug.SetGCPercent(gc)
	procs := runtime.GOMAXPROCS(1)
	defer runtime.GOMAXPROCS(procs)
	r := newPresRig(t, 0)
	fBoot(r)
	w := machine.VideoWrite{Step: 7000000000, CS: 0x1234, IP: 0x1111, Offset: 5, Ctx: machine.VWV86}
	var before, after runtime.MemStats
	runtime.ReadMemStats(&before)
	r.p.VideoWrite(w)
	runtime.ReadMemStats(&after)
	if after.Mallocs != before.Mallocs {
		t.Fatal("very first VideoWrite allocated", after.Mallocs-before.Mallocs)
	}
	n := 0
	if a := testing.AllocsPerRun(100, func() { n++; w.Step++; w.IP++; r.p.VideoWrite(w) }); a != 0 {
		t.Fatal("new source allocated", a)
	}
	w.Step++
	w.CS, w.IP = 0x2222, 0x3333
	runtime.ReadMemStats(&before)
	for repeat := 0; repeat < 2; repeat++ {
		for ctx := 0; ctx < 8; ctx++ {
			w.Ctx = machine.VideoWriteCtx(ctx)
			r.p.VideoWrite(w)
		}
	}
	runtime.ReadMemStats(&after)
	if after.Mallocs != before.Mallocs {
		t.Fatal("new context allocated", after.Mallocs-before.Mallocs)
	}
	r.p.VideoModeChange(machine.ModeChange{Mode: 3, Step: w.Step}) // drains outside VideoWrite
	total := uint64(0)
	contexts := 0
	for _, s := range r.p.f.sources {
		if s.Kind == "write" {
			total += s.Count
		}
		if s.CS == 0x2222 && s.IP == 0x3333 {
			contexts++
			if s.Count != 2 {
				t.Fatal(s)
			}
		}
	}
	if total != uint64(1+n+16) || contexts != 8 || r.p.counts.Unattributed != 1+n+16 {
		t.Fatal("mixed sources lost", total, n, contexts)
	}
	cellBytes := unsafe.Sizeof(pendingSourceWrite{})
	if cellBytes != 24 || uint64(cap(r.p.sourceWrites.rows))*uint64(cellBytes) != 786624 {
		t.Fatal("pool shape", cellBytes, cap(r.p.sourceWrites.rows))
	}
	t.Logf("pool cells=%d bytes=%d; extra periodic observations <=1/4096 (0.024414%%)", cap(r.p.sourceWrites.rows), uint64(cap(r.p.sourceWrites.rows))*uint64(cellBytes))
	runtime.ReadMemStats(&before)
	p, err := NewPresenter(PresenterConfig{Fonts: &TextFonts{}})
	if err != nil {
		t.Fatal(err)
	}
	runtime.ReadMemStats(&after)
	runtime.KeepAlive(p)
	t.Logf("minimal constructor bytes=%d allocations=%d", after.TotalAlloc-before.TotalAlloc, after.Mallocs-before.Mallocs)
	owned := newPresRig(t, 0)
	fBoot(owned)
	dEnter(owned, "O.shape", 0xfc00, nil)
	w = machine.VideoWrite{Step: owned.v.steps, CS: dCS(dRow(t, owned.p, "O.shape")), IP: 0x20, Offset: 5, Ctx: machine.VWV86}
	runtime.ReadMemStats(&before)
	owned.p.VideoWrite(w)
	runtime.ReadMemStats(&after)
	if after.Mallocs != before.Mallocs {
		t.Fatal("first owned write allocated", after.Mallocs-before.Mallocs)
	}
}

func TestFSourceDrainDeadlineAndLossless(t *testing.T) {
	type sample struct {
		Total   int
		Sources map[fallbackTriggerKey]PresentFallbackTrigger
		Rows    map[[2]uint16]PresentUnattr
	}
	var samples []sample
	for _, sparse := range []bool{false, true} {
		r := newPresRig(t, 0)
		v := &fView{pview: r.v, l: machine.IndexedLayout{Kind: machine.IndexedLinear, Mode: 3, W: 320, H: 200}}
		r.v.steps = 7000000000
		r.v.cs, r.v.ip = 0x9999, 0x1234
		r.p.Observe(v)
		base := r.v.steps
		r.p.d.mode, r.p.d.start = deferK15, base
		if r.p.nextWake() != base+4096 {
			t.Fatal("empty pool not prearmed", r.p.nextWake())
		}
		filter := machine.StepFilter{IPs: [1024]uint64(r.so.TriggerIPs()), Wake: r.p.nextWake()}
		observed := 0
		for i := 1; i <= 8242; i++ {
			if !sparse || filter.ShouldObserve(r.v.ip, r.v.steps) {
				r.p.Observe(v)
				observed++
				filter.Wake = r.p.nextWake()
				filter.Force = false
			}
			r.v.steps++
			r.p.VideoWrite(machine.VideoWrite{Step: r.v.steps, CS: 0x1234, IP: uint16(i), Offset: 5, Ctx: machine.VideoWriteCtx(i % 8)})
			if i == 4103 {
				r.p.VideoModeChange(machine.ModeChange{Mode: 3, Step: r.v.steps})
				filter.Force = true
			}
		}
		if r.p.sourceWrites.drainStep != base+8192 {
			t.Fatal("periodic deadline drift", sparse, r.p.sourceWrites.drainStep-base)
		}
		if sparse && observed > 4 {
			t.Fatal("unexpected extra wake rate", observed)
		}
		c := r.p.Counts()
		rows := map[[2]uint16]PresentUnattr{}
		for k, row := range r.p.unattr {
			rows[k] = *row
		}
		if c.Unattributed != 8242 || len(rows) != 8242 {
			t.Fatal("sources capped/lost", c.Unattributed, len(rows))
		}
		samples = append(samples, sample{c.Unattributed, r.p.f.sources, rows})
	}
	if !reflect.DeepEqual(samples[0], samples[1]) {
		t.Fatal("source aggregation differs dense/sparse")
	}
}

func TestFSourcePoolBoundFailsExplicitly(t *testing.T) {
	r := newPresRig(t, 0)
	for step := uint64(0); step <= 4096; step++ {
		for ctx := 0; ctx < 8; ctx++ {
			r.p.VideoWrite(machine.VideoWrite{Step: step, CS: 0x1234, IP: uint16(step), Offset: 5, Ctx: machine.VideoWriteCtx(ctx)})
		}
	}
	if len(r.p.sourceWrites.rows) != 32776 {
		t.Fatal("bound not fully retained")
	}
	defer func() {
		if msg := recover(); msg != "strikecommander: source drain deadline/pool invariant violated" {
			t.Fatal("overflow not explicit", msg)
		}
	}()
	r.p.VideoWrite(machine.VideoWrite{Step: 4097, CS: 0x1234, IP: 4097, Offset: 5})
}

type sourceDeliveryObserver struct {
	*SessionObserver
	mode   session.ModeObserver
	writes int
}

func (s *sourceDeliveryObserver) VideoWrite(w machine.VideoWrite) {
	s.writes++
	s.SessionObserver.VideoWrite(w)
}
func (s *sourceDeliveryObserver) VideoModeChange(c machine.ModeChange) { s.mode.VideoModeChange(c) }

func TestFSourcePublicDeliverBoundary(t *testing.T) {
	// Synthetic MZ: set 13h, then repeatedly write ES:[DI] and increment DI.
	code := []byte{0xb8, 0x13, 0, 0xcd, 0x10, 0xb8, 0, 0xa0, 0x8e, 0xc0, 0x31, 0xff, 0x26, 0xc6, 0x05, 1, 0x47, 0xeb, 0xf9}
	exe := make([]byte, 512)
	copy(exe, "MZ")
	binary.LittleEndian.PutUint16(exe[4:], 1)
	binary.LittleEndian.PutUint16(exe[8:], 2)
	binary.LittleEndian.PutUint16(exe[10:], 0x1000)
	binary.LittleEndian.PutUint16(exe[12:], 0xffff)
	binary.LittleEndian.PutUint16(exe[16:], 0xfffe)
	copy(exe[32:], code)
	root := t.TempDir()
	type sample struct {
		Digest               session.StateDigest
		Writes, Unattributed int
	}
	var results []sample
	for _, sparse := range []bool{false, true} {
		so := &SessionObserver{Labels: NewLabelObserver()}
		p, err := NewPresenter(PresenterConfig{Fonts: &TextFonts{}})
		if err != nil {
			t.Fatal(err)
		}
		p.Attach(so)
		cfg := SessionConfig(so)
		cfg.ProgramPath = `C:\POOL.EXE`
		cfg.SparseObserve = sparse
		obs := &sourceDeliveryObserver{SessionObserver: so, mode: cfg.Observer.(session.ModeObserver)}
		cfg.Observer = obs
		o, err := session.New(cfg)
		if err != nil {
			t.Fatal(err)
		}
		if _, err = o.BootOriginal(session.BootInput{EXE: exe, ExpectedEXESHA256: sha256.Sum256(exe), SaveRoot: root}); err != nil {
			o.Close()
			t.Fatal(err)
		}
		deliver := func() {
			v, err := o.View()
			if err != nil {
				t.Fatal(err)
			}
			if _, err = o.Deliver(session.CapturedUpdate{StartedPanel: v.Panel, Layout: v.Layout, SourceGeneration: v.SourceGeneration}); err != nil {
				t.Fatal(err)
			}
		}
		deliver()
		first, err := o.Advance(session.InstructionBudget(2000))
		if err != nil {
			o.Close()
			t.Fatal(err)
		}
		pending := append([]pendingSourceWrite(nil), p.sourceWrites.rows...)
		if len(pending) == 0 {
			t.Fatal("no pending source at Deliver boundary")
		}
		deliver()
		if !reflect.DeepEqual(pending, p.sourceWrites.rows) {
			t.Fatal("Deliver changed pending origins")
		}
		second, err := o.Advance(session.InstructionBudget(5000))
		if err != nil {
			o.Close()
			t.Fatal(err)
		}
		if second.MachineStepsBefore != first.MachineStepsAfter || second.MachineStepsAfter != 7000 || so.Frames != 0 {
			t.Fatal("Deliver advanced CPU or a Frame masked draining", first, second, so.Frames)
		}
		c := p.Counts()
		sum := 0
		for _, u := range p.unattr {
			sum += u.Count
		}
		if sum != obs.writes || c.Unattributed != obs.writes || p.sourceWrites.drainStep != 4096 {
			t.Fatal("public session lost buffered writes", sum, obs.writes, c.Unattributed, p.sourceWrites.drainStep)
		}
		d, err := o.Digest()
		if err != nil {
			t.Fatal(err)
		}
		results = append(results, sample{d, obs.writes, c.Unattributed})
		o.Close()
	}
	if !reflect.DeepEqual(results[0], results[1]) {
		t.Fatal("public dense/sparse delivery differs", results)
	}
}
