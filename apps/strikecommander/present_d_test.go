package strikecommander

import (
	"bytes"
	"reflect"
	"testing"

	"github.com/wicanr2/dosgolem/internal/machine"
	"github.com/wicanr2/dosgolem/session"
)

func dRow(t *testing.T, p *Presenter, name string) *presRoutine {
	t.Helper()
	for i := range p.routines {
		if p.routines[i].name == name {
			return &p.routines[i]
		}
	}
	t.Fatal(name)
	return nil
}
func dCS(r *presRoutine) uint16 {
	if r.prog == progAbs {
		return r.seg
	}
	return tpBase + r.seg
}
func dEnter(r *presRig, name string, sp uint16, ev *ArtEvent) {
	cs, ip, _, sig := dFrozenEntry(r.t, name)
	r.v.steps += 100
	r.v.cs, r.v.ip, r.v.ss, r.v.sp = cs, ip, tSS, sp
	for i, b := range sig {
		r.v.mem[lin(cs, ip+uint16(i))] = b
	}
	if ev != nil {
		e := *ev
		e.Steps = r.v.steps
		r.so.Labels.OnArt(e)
	}
	r.p.Observe(r.v)
}
func dReturn(r *presRig, name string, sp uint16) {
	cs, _, ret, _ := dFrozenEntry(r.t, name)
	r.v.steps += 100
	r.v.cs, r.v.ip, r.v.ss, r.v.sp = cs, ret, tSS, sp
	r.p.Observe(r.v)
}
func dEvent(k int) ArtEvent {
	el, _, _ := elementFor(k)
	return ArtEvent{File: ArtSaveLoad, K: k, X: 80, Y: 30, W: el.w, H: el.h, Entry: bytes.Repeat([]byte{9}, el.w*el.h), Base0: bytes.Repeat([]byte{9}, el.w*el.h)}
}
func TestDWindowTableAndSignatures(t *testing.T) {
	r := newPresRig(t, 0)
	if len(presRoutines) != 24 {
		t.Fatal(len(presRoutines))
	}
	pairs := map[[18]byte]bool{}
	for _, row := range presRoutines {
		var k [18]byte
		k[0], k[1] = byte(row.entryIP), byte(row.entryIP>>8)
		copy(k[2:], row.sig[:])
		if pairs[k] {
			t.Fatal("ambiguous (IP,sig)", row.name)
		}
		pairs[k] = true
		if row.name == "M.zoom" && row.timeout != 1000000 {
			t.Fatal(row)
		}
	}
	dEnter(r, "O.shape", 0xfc00, nil)
	if len(r.p.stack) != 1 {
		t.Fatal("shape did not open")
	}
	dReturn(r, "O.shape", 0xfc00)
	if r.p.Current().Seq != 0 {
		t.Fatal("offscreen shape captured")
	}
	row := dRow(t, r.p, "O.shape")
	r.v.steps++
	r.v.cs, r.v.ip = dCS(row), row.entryIP
	r.v.mem[lin(r.v.cs, row.entryIP)] ^= 1
	r.p.Observe(r.v)
	if len(r.p.stack) != 0 {
		t.Fatal("bad signature opened window")
	}
	dEnter(r, "O.puts", 0xfc00, nil)
	r.write(12, 3, 0x9999, 0x7777, machine.VWV86)
	if r.p.counts.Unattributed != 0 {
		t.Fatal("puts cross-CS not owned")
	}
	dReturn(r, "O.puts", 0xfc00)
	if r.p.Current().Seq != 1 {
		t.Fatal("puts did not capture")
	}
	dEnter(r, "O.local", 0xfc00, nil)
	r.write(13, 4, dCS(dRow(t, r.p, "O.local")), 0x30, machine.VWV86)
	dReturn(r, "O.local", 0xfc00)
	c := r.p.Counts().Routines["O.local"]
	if c.Opened != 1 || c.Returned != 1 || c.WroteVGA != 1 {
		t.Fatal(c)
	}
}
func TestDMenuPeriodAndCandidates(t *testing.T) {
	r := newPresRig(t, 0)
	dEnter(r, "M.memcpy", 0xfc00, nil)
	if len(r.p.stack) != 0 || r.p.d.menu {
		t.Fatal("memcpy started MENU")
	}
	dEnter(r, "M.fill", 0xfc00, nil)
	if len(r.p.stack) != 1 || !r.p.d.menu {
		t.Fatal("MENU fill did not start period")
	}
	dReturn(r, "M.fill", 0xfc00)
	r.p.VideoModeChange(machine.ModeChange{Mode: 0x13})
	dEnter(r, "M.memcpy", 0xfc00, nil)
	if len(r.p.stack) != 0 {
		t.Fatal("mode did not reset MENU")
	}
	for _, program := range []string{"O.fill", "S.fill"} {
		r := newPresRig(t, 0)
		r.so.Labels.baseKnown, r.so.Labels.baseOK = false, false
		r.so.Flight.baseOK = false
		if program == "O.fill" {
			r.v.w8(tpBase+segText, ipPutsEntry, sigPutsEntry...)
			for _, row := range baseRows {
				b := append([]int(nil), row.bytes...)
				wild := 0
				for j := range b {
					if b[j] < 0 {
						b[j] = int((tpBase+segText)>>(8*wild)) & 255
						wild++
					}
				}
				r.v.w8(tpBase+row.seg, row.off, b...)
			}
		}
		dEnter(r, program, 0xfc00, nil)
		if len(r.p.stack) != 1 {
			t.Fatal("candidate failed", program, r.p.windows)
		}
		if _, known := r.so.Labels.Base(); known {
			t.Fatal("Presenter changed Label base")
		}
		if _, known := r.so.Flight.Base(); known {
			t.Fatal("Presenter changed Flight base")
		}
	}
}
func TestDDeferralsAndEndingCapture(t *testing.T) {
	r := newPresRig(t, 0)
	e := dEvent(15)
	dEnter(r, "O.shape", 0xfc00, &e)
	r.write(2, 3, dCS(dRow(t, r.p, "O.shape")), 0x20, machine.VWV86)
	dReturn(r, "O.shape", 0xfc00)
	if r.p.Current().Seq != 0 || r.p.d.mode != deferK15 {
		t.Fatal("K15 captured early")
	}
	for _, k := range []int{17, 19} {
		e = dEvent(k)
		dEnter(r, "O.shape", 0xfc00, &e)
		r.write(k, 4, dCS(dRow(t, r.p, "O.shape")), 0x20, machine.VWV86)
		dReturn(r, "O.shape", 0xfc00)
		if k == 17 && r.p.Current().Seq != 0 {
			t.Fatal("one confirm button ended deferral")
		}
	}
	if r.p.Current().Seq != 1 || r.p.d.mode != deferIdle {
		t.Fatal("K15 ending capture missing")
	}
	for _, load := range []bool{false, true} {
		r := newPresRig(t, 0)
		e := dEvent(0)
		dEnter(r, "O.shape", 0xfc00, &e)
		r.write(1, 3, dCS(dRow(t, r.p, "O.shape")), 0x20, machine.VWV86)
		dReturn(r, "O.shape", 0xfc00)
		k := 2
		if load {
			k = 4
		}
		e = dEvent(k)
		e.X, e.Y = 134, 127
		dEnter(r, "O.shape", 0xfc00, &e)
		dReturn(r, "O.shape", 0xfc00)
		if load {
			if r.p.Current().Seq != 0 {
				t.Fatal("load captured before title")
			}
			e = dEvent(1)
			dEnter(r, "O.shape", 0xfc00, &e)
			dReturn(r, "O.shape", 0xfc00)
		}
		if r.p.Current().Seq != 1 || r.p.d.mode != 0 {
			t.Fatal("K0 ending capture", load, r.p.d)
		}
	}
}
func TestDK0DerivedRowsAreLocalAndExact(t *testing.T) {
	e := dEvent(0)
	for i := range e.Entry {
		e.Entry[i] = byte(i % 251)
		e.Base0[i] = byte((i * 3) % 251)
	}
	rows := presentK0Rows(e)
	if len(rows) != 3 {
		t.Fatal(rows)
	}
	for i, offset := range [][3]int{{54, 97, 75}, {10, 112, 80}, {94, 112, 80}} {
		row := rows[i]
		if row.Order != e.Order || row.G != e.G || row.X != e.X+offset[0] || row.Y != e.Y+offset[1] {
			t.Fatal(row)
		}
		for y := 0; y < 12; y++ {
			want := e.Entry[(offset[1]+y)*182+offset[0] : (offset[1]+y)*182+offset[0]+offset[2]]
			if !bytes.Equal(row.Entry[y*row.W:(y+1)*row.W], want) {
				t.Fatal("independent crop mismatch")
			}
		}
	}
	before := append([]byte(nil), e.Entry...)
	rows[0].Entry[0] ^= 255
	if !bytes.Equal(before, e.Entry) {
		t.Fatal("derived rows alias original")
	}
}
func TestDTimeoutWakeAndSparseOracle(t *testing.T) {
	for _, sparse := range []bool{false, true} {
		r := newPresRig(t, 0)
		r.v.steps = 1000
		e := dEvent(0)
		e.Steps = 1000
		r.so.Labels.OnArt(e)
		r.p.d.pending = true
		if got := r.p.nextWake(); got != 201000 {
			t.Fatal("deadline", got)
		}
		r.v.steps = 1500
		e.Steps = 1500
		r.so.Labels.OnArt(e)
		if got := r.p.nextWake(); got != 201500 {
			t.Fatal("restart", got)
		}
		r.v.cs, r.v.ip = 0x9999, 0x1234
		if sparse {
			f := machine.StepFilter{Wake: r.p.nextWake()}
			if f.ShouldObserve(r.v.ip, 201499) {
				t.Fatal("premature wake")
			}
			if !f.ShouldObserve(r.v.ip, 201500) {
				t.Fatal("missed wake")
			}
		}
		r.v.steps = 201500
		r.p.Observe(r.v)
		if r.p.counts.ModeTimeout != 1 || r.p.Current().CaptureSteps != 201500 {
			t.Fatal(sparse, r.p.Current(), r.p.counts)
		}
		// The independent audit sees an overdue field state without calling nextWake.
		r.p.d.mode = deferK0
		r.p.d.start = 1500
		a := newSparseAudit(r.so)
		a.oracle(r.so, 201500, 0x1234)
		if a.failN == 0 {
			t.Fatal("independent audit missed deadline")
		}
	}
}

func TestDLongWindowAndReturnHistoryBound(t *testing.T) {
	r := newPresRig(t, 0)
	dEnter(r, "M.zoom", 0xfc00, nil)
	start := r.v.steps
	r.v.steps, r.v.cs, r.v.ip = start+421203, 0x9999, 0x1234
	r.p.Observe(r.v)
	if len(r.p.stack) != 1 || r.p.counts.WindowTimeout != 0 {
		t.Fatal("MENU zoom timed out before 1000000")
	}
	r.v.steps = start + 1000000
	r.p.Observe(r.v)
	if len(r.p.stack) != 0 || r.p.counts.WindowTimeout != 1 || r.p.Current().Seq != 0 {
		t.Fatal("timeout captured or stayed open", r.p.counts)
	}
	r = newPresRig(t, 0)
	for i := 0; i < 65; i++ {
		sp := uint16(0xfc00 - i*4)
		dEnter(r, "O.shape", sp, nil)
		dReturn(r, "O.shape", sp)
	}
	if r.p.counts.ReturnHistoryEvicted != 1 {
		t.Fatal("return history eviction", r.p.counts.ReturnHistoryEvicted)
	}
	r.v.deliv++
	dReturn(r, "O.shape", 0xfc00)
	if r.p.counts.ReturnWithoutEntry != 1 {
		t.Fatal("oldest key retained")
	}
	dReturn(r, "O.shape", 0xfc00-64*4)
	if r.p.counts.ReturnDuplicateDelivered != 1 {
		t.Fatal("newest key lost")
	}
	r.v.steps += 200001
	r.v.deliv++
	dReturn(r, "O.shape", 0xfc00-64*4)
	if r.p.counts.ReturnWithoutEntry != 2 {
		t.Fatal("expired key accepted")
	}
}
func TestDNestedReturnsAndPendingVerdict(t *testing.T) {
	r := newPresRig(t, 0)
	r.win2d(0xfd00, func() { r.write(20, 1, t2dCS, t2dEntry, machine.VWV86) })
	dEnter(r, "O.puts", 0xfc00, nil)
	dEnter(r, "O.shape", 0xfb00, nil)
	r.write(20, 7, dCS(dRow(t, r.p, "O.shape")), 0x20, machine.VWV86)
	dReturn(r, "O.shape", 0xfb00)
	r.frame(nil)
	v := r.p.LastVerdict()
	if v.Class != VerdictExempt || v.Reason != ExemptPendingCapture || !v.FrameComplete {
		t.Fatal(v)
	}
	dReturn(r, "O.puts", 0xfc00)
	r.frame(nil)
	if r.p.LastVerdict().Class != VerdictEligible {
		t.Fatal(r.p.LastVerdict())
	}
	// A callback return must not erase the main loop's recent-return identity.
	dEnter(r, "O.shape", 0xfc00, nil)
	dReturn(r, "O.shape", 0xfc00)
	dEnter(r, "O.shape", 0xfb00, nil)
	dReturn(r, "O.shape", 0xfb00)
	r.v.deliv++
	dReturn(r, "O.shape", 0xfc00)
	if r.p.counts.ReturnDuplicateDelivered != 1 || r.p.counts.ReturnWithoutEntry != 0 {
		t.Fatal(r.p.counts)
	}
	dEnter(r, "O.shape", 0xfc00, nil)
	dEnter(r, "O.shape", 0xfb00, nil)
	if len(r.p.stack) != 2 {
		t.Fatal("same-routine callback collapsed", len(r.p.stack))
	}
	dReturn(r, "O.shape", 0xfb00)
	if len(r.p.stack) != 1 || r.p.stack[0].sp != 0xfc00 {
		t.Fatal("callback closed outer window")
	}
	dReturn(r, "O.shape", 0xfc00)
}

func TestDCUShotPromotionPredicate(t *testing.T) {
	for _, change := range []string{"none", "write", "state", "residue"} {
		t.Run(change, func(t *testing.T) {
			r := newPresRig(t, 0)
			// Original bytes and pending text are produced independently of the
			// presenter's frozen image. Keep a real matching HUD ink/band so a
			// wrong flight set changes the 3x composite rather than hiding silently.
			r.w.fillBand(0, 200, 0)
			r.w.hud("GEAR", 190, 10)
			r.cuIn(ipCUDiff, 1, tCUSP)
			r.copyFB()
			r.cuRet(ipCUDiffRet, tCUSP)
			r.at(0x1234, 0x10)
			r.frame(nil)
			if !r.p.d.cuCapture.promoted || r.p.stateSinceCap != r.p.d.cuCapture.returnState {
				t.Fatal("own promotion changed shot state", r.p.d.cuCapture, r.p.stateSinceCap)
			}
			switch change {
			case "write":
				r.write(0, r.v.vga[0], 0x9999, 0x1111, machine.VWSegmented)
			case "state":
				r.p.stateSinceCap++
			case "residue":
				r.p.d.cuCapture.residue = true
			}
			cur := map[int][]byte{}
			for _, sc := range r.p.cfg.Scales {
				ov, err := NewOverlayText(r.p.cfg.Fonts, testCatalog(), map[string]string{}, sc)
				if err != nil {
					t.Fatal(err)
				}
				fov, err := NewFlightOverlayText(r.p.cfg.Fonts, flightTestCats(), sc)
				if err != nil {
					t.Fatal(err)
				}
				// Observation overlays are new instances. Their source is current
				// VGA/palette and the observer's promoted visible set, never p.Compose.
				out := composeScale(scaleIn{Overlay: ov, Flight: fov, Scale: sc,
					Frame: r.v.vga, Palette: r.v.pal,
					LabelsV: r.so.Labels.Visible(), LabelsG: r.so.Labels.Generation(),
					FlightV: r.so.Flight.Visible(), FlightG: r.so.Flight.Generation()})
				cur[sc] = out.Comp
				if sc == 3 && bytes.Equal(out.Comp, out.Plain) {
					t.Fatal("fixture did not render HUD translation")
				}
			}
			shot := r.p.shotCheck(cur)
			if shot.Cond != (change == "none") {
				t.Fatal(change, shot.Cond, shot.Why)
			}
			if change == "none" {
				for _, eq := range shot.Equal {
					if !eq {
						t.Fatal("shot composition differs")
					}
				}
			}
		})
	}
}
func TestDCursorGapAndFlush(t *testing.T) {
	r := newPresRig(t, 0)
	dEnter(r, "O.erase", 0xfc00, nil)
	r.write(40, 2, dCS(dRow(t, r.p, "O.erase")), 0x30, machine.VWV86)
	dReturn(r, "O.erase", 0xfc00)
	if r.p.Current().Seq != 0 || !r.p.d.eraseGap {
		t.Fatal("erase captured")
	}
	r.frame(nil)
	if r.p.counts.CursorEraseFlush != 1 || r.p.LastVerdict().Class != VerdictEligible {
		t.Fatal(r.p.counts, r.p.LastVerdict())
	}
	dEnter(r, "O.erase", 0xfc00, nil)
	r.write(40, 3, dCS(dRow(t, r.p, "O.erase")), 0x30, machine.VWV86)
	dReturn(r, "O.erase", 0xfc00)
	dEnter(r, "O.shape", 0xfb00, nil)
	r.write(41, 3, dCS(dRow(t, r.p, "O.shape")), 0x30, machine.VWV86)
	r.frame(nil)
	if r.p.LastVerdict().Reason != ExemptWindowWriting {
		t.Fatal(r.p.LastVerdict())
	}
	dReturn(r, "O.shape", 0xfb00)
	if r.p.counts.CursorEraseFlush != 2 {
		t.Fatal("deferred flush missing", r.p.counts)
	}
	// A deferred flush never turns another erase return into a completion point.
	dEnter(r, "O.erase", 0xfc00, nil)
	r.write(40, 4, dCS(dRow(t, r.p, "O.erase")), 0x30, machine.VWV86)
	dReturn(r, "O.erase", 0xfc00)
	seq := r.p.Current().Seq
	dEnter(r, "O.erase", 0xfc00, nil)
	r.write(40, 5, dCS(dRow(t, r.p, "O.erase")), 0x30, machine.VWV86)
	r.frame(nil)
	dReturn(r, "O.erase", 0xfc00)
	if r.p.Current().Seq != seq {
		t.Fatal("deferred flush landed on erase return")
	}
	r.frame(nil)
	if r.p.counts.CursorEraseFlush != 3 || r.p.LastVerdict().Class != VerdictEligible {
		t.Fatal("next frame did not flush", r.p.counts, r.p.LastVerdict())
	}
}
func TestDFrameVerdictPriorityAndNoAlloc(t *testing.T) {
	r := newPresRig(t, 0)
	if r.p.LastVerdict().Class != VerdictNoImage {
		t.Fatal("initial verdict")
	}
	r.frame(nil)
	if r.p.LastVerdict().Class != VerdictNoImage {
		t.Fatal(r.p.LastVerdict())
	}
	r.p.d.mode = deferK0
	r.frame(nil)
	if v := r.p.LastVerdict(); v.Reason != ExemptDeferral || v.Class != VerdictExempt || !v.FrameComplete {
		t.Fatal(v)
	}
	r.p.d.mode = deferIdle
	r.p.capture(r.v, PresRoutine, 0, 0, nil, nil, nil)
	r.v.vga[99] ^= 1
	r.frame(nil)
	if v := r.p.LastVerdict(); v.Class != VerdictStale || v.Diff != 1 {
		t.Fatal(v)
	}
	r.v.vga[99] ^= 1
	count := 0
	r.p.OnVerdict(func(v FrameVerdict) { count++ })
	if n := testing.AllocsPerRun(100, func() { r.p.FrameAt(r.v.steps, r.v.vga, r.v.pal); _ = r.p.LastVerdict() }); n != 0 {
		t.Fatal("verdict alloc", n)
	}
	if count == 0 {
		t.Fatal("no verdict callback")
	}
	for _, reason := range []ExemptReason{ExemptInCU, ExemptAfterCUVoid, ExemptEraseGapDeferred} {
		r.p.cuOpen = false
		r.p.d.flushDeferred = false
		r.p.staleVoid = false
		r.p.stack = nil
		switch reason {
		case ExemptInCU:
			r.p.flight = nil
			r.p.cuOpen = true
		case ExemptAfterCUVoid:
			r.p.staleVoid = true
		case ExemptEraseGapDeferred:
			r.p.d.flushDeferred = true
		}
		r.p.FrameAt(555, r.v.vga, r.v.pal)
		if v := r.p.LastVerdict(); v.Reason != reason || v.FrameComplete != (reason != ExemptInCU) {
			t.Fatal(reason, v)
		}
	}
}
func TestDDenseMachineFrameClockAndAppForward(t *testing.T) {
	m := machine.New()
	if err := m.LoadCOM([]byte{0x90, 0xeb, 0xfd}); err != nil {
		t.Fatal(err)
	}
	var before uint64
	var frames []uint64
	m.SetOnFrame(func() {
		if m.Steps != before {
			t.Fatalf("frame %d before %d", m.Steps, before)
		}
		frames = append(frames, m.Steps)
	})
	for i := 0; i < 2*machine.DefaultVGAFrameEvery+2; i++ {
		before = m.Steps
		if err := m.Step(); err != nil {
			t.Fatal(err)
		}
		if m.Steps != before+1 {
			t.Fatal("adjacent step", m.Steps, before)
		}
	}
	if len(frames) < 2 {
		t.Fatal("no real frames")
	}
	r := newPresRig(t, 0)
	r.so.steps = 1000
	r.so.Frame(r.v.vga, r.v.pal)
	if r.p.LastVerdict().Steps != 1000 {
		t.Fatal("dense forwarding")
	}
	r.so.steps = 1
	r.so.FrameAt(2000)
	r.so.Frame(r.v.vga, r.v.pal)
	if r.p.LastVerdict().Steps != 2000 {
		t.Fatal("sparse exact injection")
	}
	if reflect.DeepEqual(frames, []uint64{0}) {
		t.Fatal("invalid zero source")
	}
}

var _ = session.NoWake
