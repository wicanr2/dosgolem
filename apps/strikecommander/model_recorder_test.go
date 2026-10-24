package strikecommander

import (
	"encoding/binary"
	"testing"

	"github.com/wicanr2/dosgolem/apps/strikecommander/realspace"
	"github.com/wicanr2/dosgolem/internal/machine"
	"github.com/wicanr2/dosgolem/session"
)

type modelTestView struct {
	mem                                        map[uint32]byte
	steps, deliveries                          uint64
	cs, ip, ss, sp, bp, ds, ax, cx, dx, si, bx uint16
	reads                                      int
	fail                                       uint32
}

func newModelTestView() *modelTestView {
	return &modelTestView{mem: map[uint32]byte{}, ss: 0x5000, sp: 0x900, bp: 0x100, ds: 0x6000}
}
func (v *modelTestView) Steps() uint64        { return v.steps }
func (v *modelTestView) Deliveries() uint64   { return v.deliveries }
func (v *modelTestView) CS() uint16           { return v.cs }
func (v *modelTestView) IP() uint16           { return v.ip }
func (v *modelTestView) SS() uint16           { return v.ss }
func (v *modelTestView) SP() uint16           { return v.sp }
func (v *modelTestView) BP() uint16           { return v.bp }
func (v *modelTestView) DS() uint16           { return v.ds }
func (v *modelTestView) AX() uint16           { return v.ax }
func (v *modelTestView) CX() uint16           { return v.cx }
func (v *modelTestView) DX() uint16           { return v.dx }
func (v *modelTestView) SI() uint16           { return v.si }
func (v *modelTestView) BX() uint16           { return v.bx }
func (v *modelTestView) Flags() uint16        { return 0 }
func (v *modelTestView) Read8(a uint32) uint8 { v.reads++; return v.mem[a] }
func (v *modelTestView) ReadBlock(a uint32, b []byte) bool {
	v.reads++
	for i := range b {
		b[i] = v.mem[a+uint32(i)]
		if v.fail != 0 && a+uint32(i) == v.fail {
			clear(b)
			return false
		}
	}
	return true
}
func (v *modelTestView) put(seg, off uint16, b []byte) {
	for i, x := range b {
		v.mem[lin(seg, off+uint16(i))] = x
	}
}
func (v *modelTestView) word(seg, off, w uint16) {
	var b [2]byte
	binary.LittleEndian.PutUint16(b[:], w)
	v.put(seg, off, b[:])
}
func (v *modelTestView) far(seg, off uint16, p realspace.FarPtr) {
	v.word(seg, off, p.Off)
	v.word(seg, off+2, p.Seg)
}

func TestModelReadWrap(t *testing.T) {
	v := newModelTestView()
	want := make([]byte, 32)
	for i := range want {
		want[i] = byte(i + 1)
	}
	v.put(0x2000, 0xfff0, want)
	got := make([]byte, 32)
	if !modelRead(v, 0x2000, 0xfff0, got) {
		t.Fatal("read failed")
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("byte %d: %d want %d", i, got[i], want[i])
		}
	}
	if v.reads != 2 {
		t.Fatalf("ReadBlock calls=%d want 2", v.reads)
	}
}

func modelFaceFixture() (*ModelRecorder, *modelTestView) {
	r := NewModelRecorder(nil)
	r.base, r.baseOK, r.lastSig = 0x241, true, 1
	r.open = &ModelGroup{Cameras: []realspace.Camera{{CamObj: 10}}, CameraReadFailed: []bool{false}, Objects: []realspace.ObjObs{{CamObj: 10, CamIdx: 0, VertRT: make([][3]int32, 3)}}}
	o := &r.open.Objects[0]
	binary.LittleEndian.PutUint16(o.ModelHead[4:], 0x1000)
	binary.LittleEndian.PutUint16(o.ModelHead[6:], 0x4000)
	v := newModelTestView()
	v.steps, v.cs, v.ip = 10, r.base+0x1ad5, 0x0a7c
	r.frames[modelFrameKey{v.ss, v.bp}] = modelFrameState{obj: 0, face: -1}
	face := realspace.FarPtr{Off: 0x100, Seg: 0x6100}
	geom := realspace.FarPtr{Off: 0x200, Seg: 0x6200}
	v.far(v.ss, v.bp-0x64, face)
	v.word(face.Seg, face.Off, 0x3441)
	v.far(face.Seg, face.Off+2, geom)
	v.far(face.Seg, face.Off+6, realspace.FarPtr{Off: 0x100, Seg: 0x7000})
	v.word(face.Seg, face.Off+10, 0x112)
	v.word(face.Seg, face.Off+14, 0x124)
	v.far(geom.Seg, geom.Off+0x1a, realspace.FarPtr{Off: 0x1000, Seg: 0x4000})
	v.word(geom.Seg, geom.Off+0x1e, 0x1018)
	v.word(geom.Seg, geom.Off+0x22, 0x1030)
	return r, v
}

func TestModelHitDeliveryAndReturn(t *testing.T) {
	r, v := modelFaceFixture()
	r.Observe(v)
	if len(r.open.Objects[0].Faces) != 0 || r.pend == nil {
		t.Fatal("tentative F1 changed the group")
	}
	v.steps, v.ip, v.deliveries = 11, 0, 1
	r.Observe(v)
	if len(r.open.Objects[0].Faces) != 0 || r.Counts().HitsVoidedByDelivery != 1 {
		t.Fatal("IRQ did not void tentative F1")
	}
	if len(r.open.Reads) != 0 {
		t.Fatal("voided hit changed accepted read metrics")
	}
	v.steps, v.ip = 12, 0x0a7c
	r.Observe(v)
	// 暫定槽跨呼叫保留；下一次可屬於下一回合，不重設錄製器。
	v.steps, v.ip, v.ax = 13, 0x0a7f, 1
	r.Observe(v)
	v.steps, v.ip = 14, 0
	r.Observe(v)
	fs := r.open.Objects[0].Faces
	if len(fs) != 1 || !fs[0].Returned || fs[0].VIdx != ([3]int16{0, 1, 2}) {
		t.Fatalf("faces=%+v", fs)
	}
	if r.Counts().FaceRetMismatch != 0 || r.Counts().F1RUnpaired != 0 {
		t.Fatal(r.Counts())
	}
}

func TestModelReturnMismatchAndUnpaired(t *testing.T) {
	r, v := modelFaceFixture()
	r.Observe(v)
	v.steps, v.ip = 11, 0
	r.Observe(v)
	v.steps, v.ip, v.sp = 12, 0x0a7f, v.sp+2
	r.Observe(v)
	v.steps, v.ip = 13, 0
	r.Observe(v)
	if r.open.Objects[0].Faces[0].Returned || r.Counts().FaceRetMismatch != 1 {
		t.Fatal(r.Counts())
	}
	v.steps, v.ip, v.sp = 14, 0x0a7f, v.sp-2
	r.Observe(v)
	v.steps, v.ip = 15, 0
	r.Observe(v)
	v.steps, v.ip = 16, 0x0a7f
	r.Observe(v)
	v.steps, v.ip = 17, 0
	r.Observe(v)
	if r.Counts().F1RUnpaired != 1 {
		t.Fatal(r.Counts())
	}
}

func TestModelCUGroupUsesFirstEntryKey(t *testing.T) {
	r := NewModelRecorder(nil)
	r.base, r.baseOK, r.lastSig = 0x241, true, 1
	r.tracker.SetBase(r.base, true)
	r.open = &ModelGroup{Buf3D: make([]byte, 64000)}
	v := newModelTestView()
	v.cs, v.ip, v.steps = r.base+segFJemm, ipCUDiff, 100
	for i, b := range sigJemm {
		v.mem[lin(v.cs, 0x90+uint16(i))] = byte(b)
	}
	seen := 0
	r.OnGroup = func(g *ModelGroup) {
		seen++
		if g.Key != 100 {
			t.Fatalf("key=%d", g.Key)
		}
	}
	r.Observe(v)
	v.steps = 110
	r.Observe(v)
	v.steps, v.ip = 120, ipCUDiffRet
	r.Observe(v)
	r.Frame(121)
	r.Frame(122)
	if seen != 1 || r.Counts().GroupsSealed != 1 || r.Counts().GroupsReplaced != 0 {
		t.Fatal(seen, r.Counts())
	}
}

func TestModelRecorderDormantDoesNotRead(t *testing.T) {
	r := NewModelRecorder(nil)
	v := newModelTestView()
	v.ip, v.steps = 0x7654, 100
	r.Observe(v)
	if v.reads != 0 {
		t.Fatalf("dormant read %d times", v.reads)
	}
	if r.nextWake() != ^uint64(0) {
		t.Fatal("unexpected wake")
	}
	so := &SessionObserver{Models: r}
	set := so.TriggerIPs()
	for _, ip := range modelTriggerIPs {
		if !set.Has(ip) {
			t.Fatalf("missing IP %04x", ip)
		}
	}
}

func modelObjectFixture() (*ModelRecorder, *modelTestView) {
	r := NewModelRecorder(nil)
	r.base, r.baseOK, r.lastSig = 0x241, true, 1
	r.open = &ModelGroup{}
	v := newModelTestView()
	v.steps = 100
	rec := realspace.FarPtr{Off: 0x100, Seg: 0x6100}
	model := realspace.FarPtr{Off: 0x200, Seg: 0x6200}
	list := realspace.FarPtr{Off: 0x400, Seg: 0x6400}
	v.far(v.ss, v.bp+6, rec)
	v.word(v.ss, v.bp+10, 10)
	v.far(v.ss, v.bp-0x2e, list)
	v.word(v.ss, v.bp-0x30, 1)
	v.far(v.ss, v.bp-0x2a, realspace.FarPtr{Off: 0x500, Seg: 0x6500})
	v.word(rec.Seg, rec.Off, 0x10c2)
	v.far(rec.Seg, rec.Off+2, model)
	v.word(model.Seg, model.Off+2, 3)
	v.far(model.Seg, model.Off+4, realspace.FarPtr{Off: 0x300, Seg: 0x6300})
	v.far(model.Seg, model.Off+0x22, list)
	v.word(model.Seg, model.Off+0x42, 1)
	return r, v
}

func TestModelCaptureCamerasAndObjects(t *testing.T) {
	r, v := modelObjectFixture()
	observe := func(ip, cs uint16) { v.ip, v.cs = ip, cs; r.Observe(v); v.steps++ }
	for _, identity := range []uint16{10, 20} {
		v.si = identity
		v.word(v.ds, 0x4fc0, identity) // 相機常數不同，原始相機區也必須各自保存。
		observe(0x012a, r.base+0x28b7)
		v.word(v.ss, v.bp+10, identity)
		observe(0x0a51, r.base+0x1ad5)
		observe(0, 0)
	}
	if len(r.open.Cameras) != 2 || len(r.open.Objects) != 2 {
		t.Fatal("C1/O1 capture count")
	}
	for i, identity := range []uint16{10, 20} {
		o := &r.open.Objects[i]
		if o.CamIdx != i || o.CamObj != identity || r.open.Cameras[i].CamObj != identity || binary.LittleEndian.Uint16(r.open.Cameras[i].Raw[8:]) != identity {
			t.Fatalf("object %d did not bind its C1: %+v", i, o)
		}
		if o.ReadFailed || o.FaceTableMismatch || o.OutOfBounds {
			t.Fatalf("capture faults: %+v", o)
		}
	}
	// K23 甲：身分必須由真正 C1/O1 讀到的值比較，不能只比相機索引。
	v.si = 30
	observe(0x012a, r.base+0x28b7)
	v.word(v.ss, v.bp+10, 31)
	observe(0x0a51, r.base+0x1ad5)
	observe(0, 0)
	o := &r.open.Objects[2]
	if verdict := realspace.CheckObject(nil, r.open.Cameras, o); verdict.Status != realspace.Failed || verdict.Reason != "camera mismatch" {
		t.Fatalf("mismatched captured camera accepted: %+v", verdict)
	}
}

func TestModelFlagReadFailure(t *testing.T) {
	r, v := modelObjectFixture()
	r.apply(&modelHit{kind: 0x012a, cam: realspace.Camera{CamObj: 10}})
	v.fail = lin(v.ds, 0x17bc)
	v.cs, v.ip = r.base+0x1ad5, 0x0a51
	r.Observe(v)
	v.steps++
	v.ip = 0
	r.Observe(v)
	if len(r.open.Objects) != 1 || !r.open.Objects[0].ReadFailed {
		t.Fatal("Flag17BC missing page was accepted")
	}
}

func TestModelObjectReadsCurrentMatrix(t *testing.T) {
	r, v := modelObjectFixture()
	r.apply(&modelHit{kind: 0x012a, cam: realspace.Camera{CamObj: 10}})
	rec := realspace.FarPtr{Off: 0x100, Seg: 0x6100}
	v.far(v.ss, v.sp+4, rec) // 01B8 函式入口尚未建立自己的 BP 框。
	v.word(rec.Seg, rec.Off+0x23, 11)
	v.cs, v.ip = r.base+0x1ad5, 0x01b8
	r.Observe(v)
	v.steps++
	v.word(rec.Seg, rec.Off+0x23, 22) // 原版到 O1 之前才更新本畫格矩陣。
	v.ip = 0x0a51
	r.Observe(v)
	v.steps++
	v.ip = 0
	r.Observe(v)
	if len(r.open.Objects) != 1 || binary.LittleEndian.Uint16(r.open.Objects[0].RecRaw[0x23:]) != 22 {
		t.Fatal("O1 沒有讀到更新後的矩陣")
	}
}

func TestModelModeObserverClearsAllSlots(t *testing.T) {
	r, _ := modelObjectFixture()
	r.await, r.shown = &ModelGroup{}, &ModelGroup{}
	r.pend = &modelHit{kind: 0x012a}
	r.frames[modelFrameKey{1, 2}] = modelFrameState{obj: 0}
	r.tracker.cuIn = true
	so := &SessionObserver{Labels: NewLabelObserver(), Models: r}
	mo, ok := SessionConfig(so).Observer.(session.ModeObserver)
	if !ok {
		t.Fatal("Models-only configuration did not install ModeObserver")
	}
	mo.VideoModeChange(machine.ModeChange{})
	if r.open != nil || r.await != nil || r.shown != nil || r.pend != nil || len(r.frames) != 0 || r.tracker.Entry().InProgress {
		t.Fatal("mode switch retained model state")
	}
	plain := &SessionObserver{Labels: NewLabelObserver()}
	if _, ok := SessionConfig(plain).Observer.(session.ModeObserver); ok {
		t.Fatal("nil Models changed the legacy mode hook")
	}
}

func TestModelSparseIndependentOracle(t *testing.T) {
	for _, kind := range []string{"ip", "pending", "signature"} {
		t.Run(kind, func(t *testing.T) {
			r := NewModelRecorder(nil)
			s := &SessionObserver{Labels: NewLabelObserver(), Models: r}
			a := newSparseAudit(s)
			ip, steps := uint16(0x7654), uint64(100)
			switch kind {
			case "ip":
				ip = 0x012a
			case "pending":
				r.pend = &modelHit{}
			case "signature":
				r.baseOK = true
				steps = SigCheckSteps
			}
			a.oracle(s, steps, ip)
			if a.failN == 0 {
				t.Fatal("independent model oracle missed observation")
			}
			if len(sparseAuditRoots(s)) != 2 {
				t.Fatal("model state missing from snapshot roots")
			}
		})
	}
}
