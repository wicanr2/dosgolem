package strikecommander

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"github.com/wicanr2/dosgolem/apps/strikecommander/hd3d"
	"github.com/wicanr2/dosgolem/apps/strikecommander/hd3d/model"
	"github.com/wicanr2/dosgolem/apps/strikecommander/realspace"
	"github.com/wicanr2/dosgolem/apps/strikecommander/shadesource"
	"io"
	"os"
	"testing"
)

func shadeOriginalEXE(t *testing.T) []byte {
	t.Helper()
	s := os.Getenv("SC_SHADE_EXE")
	if s == "" {
		t.Skip("SC_SHADE_EXE original profile required")
	}
	b, e := os.ReadFile(s)
	if e != nil {
		t.Fatal(e)
	}
	return b
}
func shadeBuildFixture(t *testing.T, on bool) (*ModelRecorder, *modelTestView) {
	t.Helper()
	r, v := modelFaceFixture()
	r.raster = &rasterRecorder{}
	r.EnableMaterialSources()
	for _, s := range shadesource.Signatures() {
		v.put(r.base+s.Seg, s.Off, shadesource.SignatureBytes(s, r.base))
	}
	if !r.verifyMaterialSource(v) {
		t.Fatal("old proof")
	}
	if on {
		if e := r.EnableShadeSourceCapture(shadeOriginalEXE(t), 0); e != nil {
			t.Fatal(e)
		}
		g, e := r.shade.owner.NewGraph(r.shadeID(0))
		if e != nil {
			t.Fatal(e)
		}
		r.open.ShadeSource = g
	}
	v.ds = r.base + 0x6c4c
	face := realspace.FarPtr{Seg: 0x6100, Off: 0xffeb}
	raw := make([]byte, 18)
	binary.LittleEndian.PutUint16(raw, 0x33b9)
	v.put(face.Seg, face.Off, raw)
	v.put(face.Seg, face.Off+0x12, []byte{1, 2, 3, 4, 127})
	v.far(v.ss, v.bp-0x64, face)
	v.far(v.ss, v.sp, face)
	r.open.Objects[0].Faces = []realspace.FaceObs{{Face: face, FaceVT: 0x33b9, Source: &realspace.FaceSourceObs{Generation: r.sourceGeneration}}}
	r.frames[modelFrameKey{v.ss, v.bp}] = modelFrameState{obj: 0, face: 0}
	v.steps = 30
	v.cs = r.base + 0x1ad5
	v.ip = 0xaa5
	return r, v
}
func TestShadeBuildPendingIRQWrapAndOldIsolation(t *testing.T) {
	r, v := shadeBuildFixture(t, true)
	defer r.CloseShadeSources()
	r.Observe(v)
	if r.open.ShadeSource.BuildValue(shadesource.Index(0)) != nil || r.nextWake() != 0 {
		t.Fatal("unconfirmed Build applied")
	}
	v.steps = 31
	v.ip = 0
	v.deliveries++
	r.Observe(v)
	if r.open.ShadeSource.BuildValue(shadesource.Index(0)) != nil {
		t.Fatal("IRQ raw5 survived")
	}
	v.steps = 32
	v.ip = 0xaa5
	r.Observe(v)
	v.steps = 33
	v.ip = 0
	v.bp = 0x999
	v.si = 77
	r.Observe(v)
	b := r.open.ShadeSource.BuildValue(shadesource.Index(0))
	if b == nil || b.Raw5 == nil || b.Raw5.State != shadesource.Complete || len(b.Raw5.Spans) != 2 || !bytes.Equal(shadesource.ReadBytes(b.Raw5), []byte{1, 2, 3, 4, 127}) {
		t.Fatal("raw5 same-segment wrap not preserved", b)
	}
	if b.Context.SS != 0x5000 || b.Context.SP != 0x900 || r.open.ReadFailed || r.open.Objects[0].ReadFailed {
		t.Fatal("callee state/optional contaminated legacy")
	}
}
func TestShadeOptionalFailureLeavesLegacyJSONAndReads(t *testing.T) {
	a, av := shadeBuildFixture(t, false)
	b, bv := shadeBuildFixture(t, true)
	defer b.CloseShadeSources()
	bv.fail = lin(0x6100, 0xfffd)
	for _, x := range []struct {
		r *ModelRecorder
		v *modelTestView
	}{{a, av}, {b, bv}} {
		x.r.Observe(x.v)
		x.v.steps++
		x.v.ip = 0
		x.r.Observe(x.v)
	}
	ac, bc := *a.open, *b.open
	bc.ShadeSource = nil
	ja, _ := json.Marshal(ac)
	jb, _ := json.Marshal(bc)
	if !bytes.Equal(ja, jb) {
		t.Fatalf("legacy JSON/Reads changed\nA%s\nB%s", ja, jb)
	}
	if a.MaterialSourceCounts() != b.MaterialSourceCounts() || a.Counts() != b.Counts() {
		t.Fatal("optional failure changed old counters")
	}
	raw := b.open.ShadeSource.BuildValue(shadesource.Index(0))
	if raw.Raw5.State != shadesource.Failed {
		t.Fatal("optional failure hidden")
	}
	b.VideoModeChange()
	if b.shade.valid || b.shade.proof != nil || b.pend != nil {
		t.Fatal("mode did not clear optional source")
	}
}
func TestShadeEnableProfileAndLateLifecycle(t *testing.T) {
	r := NewSceneRecorder(nil, hd3d.Tables{})
	r.EnableMaterialSources()
	if e := r.EnableShadeSourceCapture([]byte("wrong EXE"), 0); e == nil || r.shade != nil {
		t.Fatal("unverified EXE accepted")
	}
	r.Observe(newModelTestView())
	if e := r.EnableShadeSourceCapture(shadeOriginalEXE(t), 0); e == nil {
		t.Fatal("late source opt-in")
	}
}

// R4 payload expectations are a manually built 16-row pattern. No Replay or
// capture output constructs expected bytes, and the table is deliberately not a
// claim about the frozen original table profile.
func TestShadeR4SameCallWrapIRQAndShadeOff(t *testing.T) {
	for _, scalar := range []byte{127, 255} {
		r, v := shadeBuildFixture(t, true)
		v.put(0x6100, 0x0001, []byte{scalar})
		r.Observe(v)
		v.steps = 31
		v.ip = 0
		r.Observe(v)
		f := &r.open.Objects[0].Faces[0]
		elem := realspace.FarPtr{Seg: 0x7000, Off: 0x300}
		f.Elem = elem
		f.Returned = true
		f.AL = 1
		f.Source.Element = elem
		f.Source.ElementReturn = realspace.FaceSourceRole{Steps: 40, Face: f.Face}
		r.open.Elements = []RasterElement{{Pointer: hd3d.FarPtr{Seg: elem.Seg, Off: elem.Off}}}
		r.raster.stack = []rasterContext{{element: hd3d.FarPtr{Seg: elem.Seg, Off: elem.Off}}}
		table := make([]byte, 4096)
		for i := range table {
			table[i] = byte(i / 256)
		}
		v.put(0x8000, 0xff80, table)
		row := uint16(8)
		if scalar == 255 {
			row = 0
		}
		shadeOff := uint16(0xff80 + row*256)
		v.steps = 50
		v.cs = r.base + 0x1d5e
		v.ip = 0xea8
		makeHit := func() *modelHit {
			h := &modelHit{kind: 0xea8, steps: v.steps, del: v.deliveries, base: r.base, ss: v.ss, sp: v.sp, bp: v.bp, sourceGeneration: r.sourceGeneration}
			c := hd3d.Call{Kind: hd3d.R4, Steps: v.steps, DS: v.ds, SS: v.ss, SP: v.sp, Verts: make([]byte, 66)}
			c.Params[1] = 3
			c.Params[7] = shadeOff
			c.Params[8] = 0x8000
			h.raster = &rasterHit{kind: "call", call: c}
			h.shade = r.captureShade(v, h)
			return h
		}
		h := makeHit()
		if h.shade == nil || h.shade.call.RowOrigin != shadesource.ExtraRead {
			t.Fatal("R4 source not captured")
		}
		w := h.shade.call
		if len(w.Table.Spans) != 2 || w.Table.Spans[0].Requested != 128 || w.Table.Spans[1].Off != 0 || !bytes.Equal(shadesource.ReadBytes(w.Table), table) {
			t.Fatal("R4 full table wrap differs")
		}
		if !bytes.Equal(shadesource.ReadBytes(w.Row), bytes.Repeat([]byte{byte(row)}, 256)) {
			t.Fatal("wrong row or class7 scalar")
		}
		if scalar == 255 && len(w.Row.Spans) != 2 {
			t.Fatal("row wrap not actual split")
		}
		r.pend = h
		v.steps = 51
		v.ip = 0
		v.deliveries++
		r.Observe(v)
		if len(r.open.Calls) != 0 {
			t.Fatal("IRQ R4 leaked into original calls")
		}
		v.steps = 60
		v.ip = 0xea8
		h = makeHit()
		r.apply(h)
		if len(r.open.Calls) != 1 || r.open.Calls[0].Seq != 0 || h.shade.call.CallIndex != 0 {
			t.Fatal("confirmed R4 not bound to actual Seq/index")
		}
		first := shadesource.ReadBytes(h.shade.call.Table)
		table[0] = 99
		v.put(0x8000, 0xff80, table)
		v.steps = 70
		h = makeHit()
		r.apply(h)
		if shadesource.ReadBytes(h.shade.call.Table)[0] != 99 || first[0] != 0 {
			t.Fatal("same far reused stale table or mutated first witness")
		}
		v.steps = 80
		h = makeHit()
		h.shade.reservation.Release()
		h.raster.call.Params[7] = 0
		h.raster.call.Params[8] = 0
		before := v.reads
		h.shade = r.captureShade(v, h)
		if v.reads != before || h.shade.call.Row != nil || h.shade.call.Table != nil {
			t.Fatal("shade-off read optional memory")
		}
		h.shade.reservation.Release()
		r.CloseShadeSources()
		if r.ShadeSourceOwner().OwnedBytes() != 0 {
			t.Fatal("R4 resource leak")
		}
	}
}
func TestShadeH4RevokesHeldRawAndRecoversNewEpoch(t *testing.T) {
	r, v := shadeBuildFixture(t, true)
	defer r.CloseShadeSources()
	r.Observe(v)
	v.steps = 31
	v.ip = 0
	r.Observe(v)
	g := r.open
	g.Key = 1
	g.ShadeSource.BindCU(1)
	g.ShadeSource.Seal()
	raw, e := g.ShadeSource.Acquire()
	if e != nil {
		t.Fatal(e)
	}
	defer raw.Release()
	r.shown = g
	r.open = nil
	oldEpoch := g.ShadeSource.ID().ShadeEpoch
	oldGen := r.sourceGeneration
	addr := lin(r.base+0x2579, 0xc02)
	saved := v.mem[addr]
	v.mem[addr] ^= 1
	v.steps = 32
	r.shade.next = 32
	r.Observe(v)
	if r.shade.valid || r.sourceGeneration != oldGen || !r.baseOK || r.sourceProof == nil {
		t.Fatal("new-profile H4 contaminated old metadata")
	}
	legacy := model.Input{Key: 1, Raster: hd3d.Group{Key: 1}, Objects: g.Objects, Cameras: g.Cameras}
	_, e = raw.Export(legacy, io.Discard)
	if e == nil {
		t.Fatal("held pre-H4 raw acquired new eligibility")
	}
	v.mem[addr] = saved
	v.steps = 33
	r.shade.next = 33
	r.Observe(v)
	if !r.shade.valid || r.shade.epoch <= oldEpoch {
		t.Fatal("new epoch did not recover")
	}
	_, e = raw.Export(legacy, io.Discard)
	if e == nil {
		t.Fatal("old raw ref borrowed restored epoch")
	}
	so := &SessionObserver{Models: r, Labels: &LabelObserver{}}
	a := newSparseAudit(so)
	a.oracle(so, r.shade.next, 0)
	found := false
	for _, s := range a.fails {
		found = found || bytes.Contains([]byte(s), []byte("013b"))
	}
	if !found {
		t.Fatal("independent audit missed source deadline")
	}
}

func TestShadeKnownAndUnknownQuietPathNoReads(t *testing.T) {
	r, v := shadeBuildFixture(t, true)
	defer r.CloseShadeSources()
	r.Observe(v)
	v.steps = 31
	v.ip = 0
	r.Observe(v)
	v.ip = 0x8888
	v.steps = 32
	before := v.reads
	r.Observe(v)
	if v.reads != before {
		t.Fatal("known-base quiet source read memory")
	}
	r.baseOK = false
	v.steps = 33
	before = v.reads
	r.Observe(v)
	if v.reads != before {
		t.Fatal("unknown-base quiet source read memory")
	}
}
