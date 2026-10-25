package strikecommander

import (
	"encoding/binary"
	"fmt"
	"testing"

	"github.com/wicanr2/dosgolem/apps/strikecommander/hd3d"
)

// Existing Presenter fixture plus the scalar registers required by Models.
// There is no Machine, StepView, guest instruction or saved-state mutation.
type fix005View struct{ *pview }

func (v *fix005View) BX() uint16    { return 0x200 }
func (v *fix005View) DX() uint16    { return 0 }
func (v *fix005View) SI() uint16    { return 0 }
func (v *fix005View) Flags() uint16 { return 0 }

func TestRecorderFix005ObserveCUQuotaPublishesOnceBeforePresenter(t *testing.T) {
	for _, ip := range []uint16{ipCUPlain, ipCUDiff} {
		t.Run(fmt.Sprintf("CU%04x", ip), func(t *testing.T) {
			rig := newPresRig(t, 0)
			v := &fix005View{rig.v}
			v.ss, v.sp = tSS, 0x8000
			r := fix004Fixture(t, hd3d.Tables{})
			rig.so.Models = r
			r.tracker.SetBase(r.base, true)
			// Keep the already-tested optional source verifier dormant. This fixture
			// isolates the legacy CU allocation boundary, not EXE-proof acquisition.
			sourceReady := func() { r.shade.modelGeneration = r.sourceGeneration; r.shade.next = ^uint64(0) }
			for _, sig := range modelSigs[6:8] {
				data := append([]byte(nil), sig.b...)
				for _, at := range sig.reloc {
					binary.LittleEndian.PutUint16(data[at:], binary.LittleEndian.Uint16(data[at:])+r.base)
				}
				for i, b := range data {
					v.mem[lin(r.base+sig.seg, sig.off+uint16(i))] = b
				}
			}
			registry, e := newHDSceneRegistry(r.shade.owner, r.bounded.pool, nil, hd3d.Tables{})
			if e != nil {
				t.Fatal(e)
			}
			defer registry.close()
			output, e := NewFrontendPresentation(rig.p, [32]byte{1})
			if e != nil {
				t.Fatal(e)
			}
			defer output.Close()
			rt := &FrontendRuntime{so: rig.so, present: rig.p, output: output, hd: &frontendHD{registry: registry}}
			rig.p.bindHDImage = rt.bindHDImage
			// The existing same-base BufCU descriptor is a Presenter precondition.
			rig.p.fbDesc = presDesc{valid: true, ds: qDS, seg: 0xffff, list: qList, base: r.base}
			var rejected []HDCaptureRejected
			sealed := 0
			r.onHDRejected = func(x HDCaptureRejected) { rejected = append(rejected, x); registry.onRejected(x) }
			r.onHDSealed = func(c *hdSealedCapture) { sealed++; registry.onSealed(c) }
			step := func(cs, pc, cx uint16) {
				v.steps += 10
				v.cs, v.ip, v.cx = cs, pc, cx
				sourceReady()
				rig.so.Labels.Observe(v)
				rig.so.Flight.Observe(v)
				r.Observe(v)
				rig.p.Observe(v)
			}
			newGroup := func() {
				step(r.base+0x28b7, 0x0102, 1)
				step(0x5555, 0, 1)
				if r.open == nil {
					t.Fatal("real R7 did not open group")
				}
				// A previously captured R6 line list is owned before the actual R8 read.
				r.open.Desc = [6]uint16{0xffff, qList, 0, 0, 319, 199}
				r.open.List = boundedMake[uint16](r.captureArena(), 201)
				for y := range r.open.List {
					r.open.List[y] = uint16(16 + y*320)
				}
				step(r.base+0x28b7, 0x0197, 1)
				step(0x5555, 0, 1)
				if r.open == nil || len(r.open.Buf3D) != 64000 || r.open.ReadFailed {
					t.Fatal("real R8 not confirmed")
				}
			}
			step(r.base+segFJemm, ipMU, 0)
			step(r.base+segFJemm, ipMU, 1)
			newGroup()
			// Real R10 entry/confirmation leaves exactly one unclosed known context.
			v.w16(v.ds, 0x200, 0x00af)
			v.w16(v.ds, 0x202, r.base+0x314a)
			v.w16(v.ss, v.sp, 0x1234)
			v.w16(v.ss, v.sp+2, 0x7000)
			v.w16(0x7000, 0x1234, 0x338d)
			step(r.base+0x27f5, 0x0048, 1)
			step(0x5555, 0, 1)
			if len(r.raster.stack) != 1 || cap(r.open.RasterProblems) != 0 {
				t.Fatal("not at actual diagnostic growth boundary")
			}
			serial := r.bounded.serial
			before := r.Counts()
			u := r.bounded.pool.Usage()
			fill := u.LegacyLimit - u.LegacyOwned
			if e = r.bounded.pool.ReserveLegacy(fill); e != nil {
				t.Fatal(e)
			}
			step(r.base+segFJemm, ip, 0)
			key := v.steps
			if len(rejected) != 1 || rejected[0].CUKey != key || rejected[0].Serial != serial || rejected[0].Reason != HDCaptureQuota {
				t.Fatalf("same-CU keyed rejection missing: %+v key=%d serial=%d", rejected, key, serial)
			}
			if registry.rejected != rejected[0] {
				t.Fatal("registry did not receive same fixed rejection")
			}
			if entry := r.tracker.Entry(); !entry.InProgress || entry.Steps != key {
				t.Fatal("quota panic prevented Model CU acceptance", entry)
			}
			if f := rig.so.Flight.cu.Entry(); f.Steps != key || !f.InProgress {
				t.Fatal("Flight/Model CU diverged")
			}
			if sealed != 0 || r.open != nil || r.await != nil || r.shown != nil || r.Counts().No3D != before.No3D || r.Counts().ReadFailures != before.ReadFailures {
				t.Fatal("quota failure published partial source or changed original failure counters")
			}
			old := rig.p.cur
			if old == nil || old.info.Kind != PresCU || old.hdBinding.cu != key || old.hdBinding.slot >= 0 {
				t.Fatal("Presenter did not bind this CU to fixed NoHD")
			}
			r.bounded.pool.ReleaseLegacy(fill)
			// Neither newly available bytes nor a repeated same-key hook may refill it.
			r.sealBoundedCU(key)
			if len(rejected) != 1 || sealed != 0 || old.hdBinding.slot >= 0 {
				t.Fatal("same CU was retried or notified twice")
			}
			ret := uint16(ipCUPlainRet)
			if ip == ipCUDiff {
				ret = ipCUDiffRet
			}
			step(r.base+segFJemm, ret, 0)
			newGroup()
			nextSerial := r.bounded.serial
			step(r.base+segFJemm, ip, 0)
			nextKey := v.steps
			if nextSerial <= serial || nextKey == key || sealed != 1 || r.await == nil || r.await.Key != nextKey || rig.p.cur.hdBinding.cu != nextKey || rig.p.cur.hdBinding.slot < 0 {
				t.Fatal("new R7/CU did not recover")
			}
			if _, e = registry.lookup(old.hdBinding); e == nil {
				t.Fatal("old NoHD presentation acquired later source")
			}
		})
	}
}

func TestRecorderFix005EarlierGroupH3BindsCUAndSuppressesDuplicate(t *testing.T) {
	r := fix004Fixture(t, hd3d.Tables{})
	r.startBoundedGroup(0)
	r.rejectBounded(HDCaptureQuota, 0)
	var got []HDCaptureRejected
	r.onHDRejected = func(x HDCaptureRejected) { got = append(got, x) }
	r.sealBoundedCU(0)
	r.sealBoundedCU(0)
	if len(got) != 1 || got[0].CUKey != 0 || got[0].Steps != 0 || got[0].Serial != 1 {
		t.Fatal("zero-step CU or repeated rejection mishandled", got)
	}
	r.sealBoundedCU(10)
	r.sealBoundedCU(10)
	if len(got) != 2 || got[1].CUKey != 10 || got[1].Steps != 0 {
		t.Fatal("later accepted CU did not get its own fixed rejection", got)
	}
}

func TestRecorderFix005CUAllocationAfterSealDoesNotLeakSuccess(t *testing.T) {
	r := fix004Fixture(t, hd3d.Tables{})
	r.startBoundedGroup(1)
	r.open.Buf3D = boundedMake[byte](r.captureArena(), 64000)
	var got []HDCaptureRejected
	r.onHDRejected = func(x HDCaptureRejected) { got = append(got, x) }
	// A failing necessary private CU consumer setup must use the same boundary.
	// It performs no render or VM read. The producer arena is already sealed.
	r.onHDSealed = func(c *hdSealedCapture) { boundedMake[byte](c.Arena, 1) }
	v := newModelTestView()
	v.steps = 10
	v.cs = r.base + segFJemm
	v.ip = ipCUPlain
	for i, b := range sigJemm {
		v.mem[lin(v.cs, 0x90+uint16(i))] = byte(b)
	}
	r.tracker.SetBase(r.base, true)
	r.shade.next = ^uint64(0)
	r.shade.modelGeneration = r.sourceGeneration
	r.Observe(v)
	if len(got) != 1 || got[0].CUKey != 10 || r.open != nil || r.await != nil || r.shown != nil || r.Counts().GroupsSealed != 0 {
		t.Fatal("CU configuration failure leaked successful producer role", got, r.Counts())
	}
	if e := r.tracker.Entry(); !e.InProgress || e.Steps != 10 {
		t.Fatal("post-seal configuration failure aborted CU", e)
	}
}

var _ modelView = (*fix005View)(nil)

func TestRecorderFix005RejectedCUUsesFixedControl(t *testing.T) {
	r := fix004Fixture(t, hd3d.Tables{})
	r.startBoundedGroup(1)
	r.rejectBounded(HDCaptureQuota, 2)
	seen := 0
	r.onHDRejected = func(HDCaptureRejected) { seen++ }
	key := uint64(10)
	allocs := testing.AllocsPerRun(50, func() { key++; r.sealBoundedCU(key) })
	if allocs != 0 || seen != 51 {
		t.Fatalf("fixed CU failure control allocs=%g notifications=%d", allocs, seen)
	}
}
