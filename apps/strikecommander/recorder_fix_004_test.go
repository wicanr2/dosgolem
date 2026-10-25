package strikecommander

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"reflect"
	"testing"

	"github.com/wicanr2/dosgolem/apps/strikecommander/hd3d"
	"github.com/wicanr2/dosgolem/apps/strikecommander/realspace"
	"github.com/wicanr2/dosgolem/apps/strikecommander/shadesource"
)

func fix004Fixture(t *testing.T, tables hd3d.Tables) *ModelRecorder {
	t.Helper()
	r := NewSceneRecorder(nil, tables)
	r.EnableMaterialSources()
	o, e := shadesource.NewOwner(256 << 20)
	if e != nil {
		t.Fatal(e)
	}
	r.shade = &shadeCapture{owner: o, reset: 1, epoch: 1}
	if e = r.EnableBoundedHDCapture(0, 0); e != nil {
		t.Fatal(e)
	}
	r.base = 0x241
	r.baseOK = true
	r.lastSig = 1
	r.sourceProof = r.bounded.proof
	r.shade.modelGeneration = r.sourceGeneration
	r.shade.next = ^uint64(0)
	t.Cleanup(r.CloseShadeSources)
	return r
}
func fix004Ptr(b []byte) uintptr { return reflect.ValueOf(b).Pointer() }
func fix004Owned(a *legacyArena, p uintptr) bool {
	for _, v := range a.records {
		if v.key == p {
			return true
		}
	}
	return false
}

func TestRecorderFix004ResetDropsRolesAndHonorsExternalLease(t *testing.T) {
	for _, slot := range []string{"await", "shown"} {
		for _, external := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/external=%v", slot, external), func(t *testing.T) {
				r := fix004Fixture(t, hd3d.Tables{})
				baseOwned := r.bounded.pool.Usage().LegacyOwned
				r.startBoundedGroup(1)
				r.open.Buf3D = boundedMake[byte](r.captureArena(), 64000)
				r.open.Buf3D[0] = 71
				r.sealBoundedCU(10)
				a := r.await.hd.Arena
				var held *hdSealedCapture
				if external {
					held = r.await.hd
					if e := a.acquire(); e != nil {
						t.Fatal(e)
					}
				}
				r.tracker.cuAwait = true
				r.tracker.cuSteps = 10
				if slot == "shown" {
					r.Frame(11)
					if r.shown == nil {
						t.Fatal("no actual promotion")
					}
				}
				before := r.bounded.pool.Usage().LegacyOwned
				r.hooks.OnMU(newModelTestView(), 0)
				if r.open != nil || r.await != nil || r.shown != nil || r.pend != nil || r.raster.stack != nil || r.raster.boundedBlobs != nil {
					t.Fatal("reset retains producer raw role")
				}
				if e := r.tracker.Entry(); e.InProgress || e.Awaiting {
					t.Fatal("retired CU can promote again")
				}
				r.Frame(12)
				if r.await != nil || r.shown != nil {
					t.Fatal("Frame resurrected retired group")
				}
				if external {
					if held.Group.Buf3D[0] != 71 {
						t.Fatal("external reader lost original raw bytes")
					}
					if r.bounded.pool.Usage().LegacyOwned != before {
						t.Fatal("last external reader backing was refunded early")
					}
					if e := a.acquire(); e == nil {
						t.Fatal("retired scene accepted new lease")
					}
					held = nil
					a.release()
				}
				if r.bounded.pool.Usage().LegacyOwned != baseOwned {
					t.Fatal("group not refunded after its actual final release")
				}
				// A fresh R7/CU resumes without reusing either retired role.
				r.hooks.OnMU(newModelTestView(), 1)
				r.startBoundedGroup(20)
				r.open.Buf3D = boundedMake[byte](r.captureArena(), 64000)
				r.hooks.OnEntryAccepted(newModelTestView(), 30, ipCUPlain)
				if r.await == nil || r.await.Key != 30 || r.await.hd.CUKey != 30 {
					t.Fatal("new CU failed recovery")
				}
			})
		}
	}
}

func fix004CallView(r *ModelRecorder, kind hd3d.CallKind) *modelTestView {
	v := newModelTestView()
	v.steps = 10
	v.cs = r.base + 0x1d5e
	v.ip = map[hd3d.CallKind]uint16{hd3d.R1: 0x000c, hd3d.R2: 0x0416, hd3d.R5: 0x0c79}[kind]
	if kind == hd3d.R5 {
		v.cs = r.base + 0x2198
	}
	v.word(v.ss, v.sp+2, 0x1111)
	params := [13]uint16{0x100, 3, 0x1000, 0x7000, 0x3000, 0x7000}
	if kind == hd3d.R5 {
		params = [13]uint16{0x100, 0x1000, 0x7000, 0x1016, 0x7000, 0x102c, 0x7000, 0x5000, 0x7000, 32}
	}
	for i, x := range params {
		v.word(v.ss, v.sp+4+uint16(i*2), x)
	}
	desc := [6]uint16{0x7000, 0x200, 0, 0, 319, 199}
	for i, x := range desc {
		v.word(v.ds, 0x100+uint16(i*2), x)
	}
	r.open.Desc = desc
	r.open.List = boundedMake[uint16](r.captureArena(), 201)
	for i := range r.open.List {
		r.open.List[i] = uint16(16 + i*320)
	}
	verts := make([]byte, 66)
	for i := 0; i < 3; i++ {
		binary.LittleEndian.PutUint16(verts[i*22:], uint16(10+i))
		binary.LittleEndian.PutUint16(verts[i*22+2:], 20)
		binary.LittleEndian.PutUint16(verts[i*22+4:], 7)
	}
	v.put(0x7000, 0x1000, verts)
	v.put(0x7000, 0x3000, bytes.Repeat([]byte{0x44}, 256))
	v.far(v.ds, 0x13da, realspace.FarPtr{Seg: 0x7000, Off: 0x6000})
	v.put(0x7000, 0x6000, bytes.Repeat([]byte{0x55}, 3072))
	return v
}
func fix004Capture(t *testing.T, r *ModelRecorder, v *modelTestView) {
	t.Helper()
	r.Observe(v)
	if r.pend == nil || r.pend.raster != &r.bounded.rasterPending || r.pend.raster.call.ReadFailed {
		t.Fatal("did not capture real fixed pending call")
	}
}
func fix004Confirm(r *ModelRecorder, v *modelTestView) {
	ip, cs := v.ip, v.cs
	v.steps++
	v.ip = 0
	v.cs = 0
	r.Observe(v)
	v.ip, v.cs = ip, cs
	v.steps++
}

func TestRecorderFix004InternTransfersPendingBackingBeforeRefund(t *testing.T) {
	for _, kind := range []hd3d.CallKind{hd3d.R1, hd3d.R2, hd3d.R5} {
		t.Run(fmt.Sprint(kind), func(t *testing.T) {
			r := fix004Fixture(t, hd3d.Tables{})
			r.startBoundedGroup(1)
			v := fix004CallView(r, kind)
			fix004Capture(t, r, v)
			fix004Confirm(r, v)
			if len(r.open.Calls) != 1 {
				t.Fatal("first call missing")
			}
			first := &r.open.Calls[0]
			want := [3]uintptr{fix004Ptr(first.Verts), fix004Ptr(first.Table), fix004Ptr(first.Aux.Table)}
			fix004Capture(t, r, v)
			h := &r.bounded.rasterPending
			dup := [3]uintptr{fix004Ptr(h.call.Verts), fix004Ptr(h.call.Table), fix004Ptr(h.call.Aux.Table)}
			for i, p := range dup {
				if p != 0 && (p == want[i] || !fix004Owned(r.captureArena(), p)) {
					t.Fatal("capture did not retain separately charged duplicate")
				}
			}
			fix004Confirm(r, v)
			if len(r.open.Calls) != 2 || !reflect.DeepEqual(r.bounded.rasterPending.call, hd3d.Call{}) {
				t.Fatal("fixed pending retains old call after transfer")
			}
			got := &r.open.Calls[1]
			pointers := [3]uintptr{fix004Ptr(got.Verts), fix004Ptr(got.Table), fix004Ptr(got.Aux.Table)}
			if pointers != want || got.Seq != 1 {
				t.Fatal("canonical sharing/sequence changed")
			}
			for _, p := range dup {
				if p != 0 && fix004Owned(r.captureArena(), p) {
					t.Fatal("unreachable duplicate still charged")
				}
			}
		})
	}
}

func TestRecorderFix004CallsGrowthIncludesDuplicatePeak(t *testing.T) {
	for _, slack := range []int64{-1, 0} {
		t.Run(fmt.Sprint(slack), func(t *testing.T) {
			r := fix004Fixture(t, hd3d.Tables{})
			r.startBoundedGroup(1)
			v := fix004CallView(r, hd3d.R1)
			for i := 0; i < 4; i++ {
				fix004Capture(t, r, v)
				fix004Confirm(r, v)
			}
			if len(r.open.Calls) != 4 || cap(r.open.Calls) != 4 {
				t.Fatal("not at real Calls growth boundary")
			}
			fix004Capture(t, r, v)
			a := r.captureArena()
			duplicate := fix004Ptr(r.bounded.rasterPending.call.Verts)
			if !fix004Owned(a, duplicate) {
				t.Fatal("pending raw duplicate not counted at boundary")
			}
			growth := uint64(8) * uint64(reflect.TypeFor[hd3d.Call]().Size())
			u := r.bounded.pool.Usage()
			remaining := uint64(int64(growth) + slack)
			fill := u.LegacyLimit - u.LegacyOwned - remaining
			if e := r.bounded.pool.ReserveLegacy(fill); e != nil {
				t.Fatal(e)
			}
			fix004Confirm(r, v)
			r.bounded.pool.ReleaseLegacy(fill)
			if slack < 0 {
				if !r.bounded.failed || r.open != nil {
					t.Fatal("Calls growth borrowed pending duplicate refund")
				}
				r.startBoundedGroup(50)
				r.open.Buf3D = boundedMake[byte](r.captureArena(), 64000)
				r.sealBoundedCU(60)
				if r.bounded.failed || r.await == nil || r.await.Key != 60 {
					t.Fatal("H3 did not recover at new group/CU")
				}
			} else if r.bounded.failed || r.open == nil || len(r.open.Calls) != 5 || fix004Owned(a, duplicate) {
				t.Fatal("exact real old/new peak should fit")
			}
		})
	}
}

func fix004ReplayTables() hd3d.Tables {
	b := bytes.Repeat([]byte{9}, 200)
	for i, o := range []uint16{8, 56, 104, 152} {
		binary.LittleEndian.PutUint16(b[0xc0+i*2:], o)
	}
	return hd3d.Tables{R6Table: b}
}
func fix004ReplayGroup(r *ModelRecorder) {
	r.startBoundedGroup(1)
	a := r.captureArena()
	g := r.open
	g.Buf3D = boundedMake[byte](a, 64000)
	for i := range g.Buf3D {
		g.Buf3D[i] = 9
	}
	g.List = boundedMake[uint16](a, 201)
	for i := range g.List {
		g.List[i] = uint16(16 + i*320)
	}
	g.Desc = [6]uint16{0xffff, 0xe654, 0, 0, 319, 199}
	r.raster.append(g, hd3d.Call{Kind: hd3d.R6, Desc: g.Desc, Params: [13]uint16{0, 43, 32, 160, 32}, DS: 0x1000, SS: 0x1000, Aux: hd3d.AuxRef{Color0: 9, Color1: 9, HasFlags: true}})
}
func TestRecorderFix004R10SealDiagnosticSurvivesWorker(t *testing.T) {
	for _, depth := range []int{0, 1, 2} {
		for _, closed := range []bool{false, true} {
			t.Run(fmt.Sprintf("depth%d/closed%v", depth, closed), func(t *testing.T) {
				tables := fix004ReplayTables()
				r := fix004Fixture(t, tables)
				fix004ReplayGroup(r)
				// Known contexts suffice: no residual patch or guessed unknown-body behavior.
				for i := 0; i < depth; i++ {
					r.raster.apply(r.open, &rasterHit{kind: "entry", ctx: rasterContext{key: rasterFrameKey{1, uint16(i + 1)}}})
				}
				if closed {
					for i := depth - 1; i >= 0; i-- {
						r.raster.apply(r.open, &rasterHit{kind: "return", ctx: rasterContext{key: rasterFrameKey{1, uint16(i + 1)}}})
					}
				}
				wantBad := depth > 0 && !closed
				// Evaluate the unchanged old check on a separate shallow value before CU.
				old := *r.open
				old.RasterProblems = append([]string(nil), r.open.RasterProblems...)
				legacy := &rasterRecorder{tables: tables, stack: r.raster.stack}
				legacy.check(&old)
				if old.ReplayReady == wantBad {
					t.Fatal("fixture does not isolate R10 readiness", old.Raster.Diagnostics.Faults)
				}
				registry, e := newHDSceneRegistry(r.shade.owner, r.bounded.pool, nil, tables)
				if e != nil {
					t.Fatal(e)
				}
				defer registry.close()
				r.onHDSealed = registry.onSealed
				r.sealBoundedCU(10)
				if r.await == nil {
					t.Fatal("missing sealed legacy source")
				}
				found := false
				for _, p := range r.await.RasterProblems {
					if p == "封存時R10框未關閉" {
						found = true
					}
				}
				if found != wantBad {
					t.Fatal("CU lost/added R10 unclosed diagnostic")
				}
				if len(r.raster.stack) != 0 {
					t.Fatal("CU did not clear live tracker")
				}
				var slot *hdRegistrySlot
				for i := range registry.slots {
					if registry.slots[i].capture != nil {
						slot = &registry.slots[i]
					}
				}
				if slot == nil {
					t.Fatal("registry did not retain source")
				}
				m, e := registry.materialize(slot)
				if e != nil {
					t.Fatal(e)
				}
				defer m.release()
				if m.group.ReplayReady != old.ReplayReady {
					t.Fatal("outside worker washed R10 failure")
				}
			})
		}
	}
}
