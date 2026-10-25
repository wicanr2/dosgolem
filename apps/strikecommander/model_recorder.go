package strikecommander

// 規格 013-strike-3d-model-layer-rerender 第 1 期與
// 012-strike-3d-hd-rerasterize §3.1、§3.2 的唯讀群組骨架。
// 模型錄製與選配 1 倍重畫共用群組邊界，不畫 HD。

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"reflect"

	"github.com/wicanr2/dosgolem/apps/strikecommander/hd3d"
	"github.com/wicanr2/dosgolem/apps/strikecommander/realspace"
	"github.com/wicanr2/dosgolem/apps/strikecommander/shadesource"
	"github.com/wicanr2/dosgolem/session"
)

type modelView interface {
	FlightView
	BX() uint16
	DX() uint16
	SI() uint16
	ReadBlock(uint32, []byte) bool
	Deliveries() uint64
	Flags() uint16
}

// ModelGroup 是同一 CU 所屬的模型觀察複本。封存後不再修改。
// ReplayReady 只由 NewSceneRecorder 的 1 倍重畫決定；純模型錄製不會設為真。
type ModelGroup struct {
	hd                   *hdSealedCapture // private owned capture; never serialized
	hdState              *boundedGroupState
	hdSourceError        shadesource.Error
	ShadeSourceError     *shadesource.Error `json:",omitempty"`
	ShadeSource          *shadesource.Graph `json:",omitempty"`
	Steps, EndSteps, Key uint64
	Cameras              []realspace.Camera
	CameraReadFailed     []bool
	Objects              []realspace.ObjObs
	Desc                 [6]uint16
	List                 []uint16
	Buf3D                []byte
	ReadFailed           bool
	UnmatchedFrames      uint64
	ReplayReady          bool
	Calls                []hd3d.Call
	Raster               *hd3d.Result
	RasterProblems       []string
	Elements             []RasterElement
	Reads                map[uint16]ModelReadCount
}

// ModelReadCount 是有效資料命中的 ReadBlock 請求量，依原始 IP 分類。
// 包含環繞後的各段請求與失敗請求；不含驗簽、CU 或被送達事件作廢的命中。
type ModelReadCount struct{ Calls, Bytes uint64 }

type modelReadCountingView struct {
	modelView
	count ModelReadCount
}

func (v *modelReadCountingView) ReadBlock(a uint32, dst []byte) bool {
	v.count.Calls++
	v.count.Bytes += uint64(len(dst))
	return v.modelView.ReadBlock(a, dst)
}

// RasterElement 保留 R10 入口的原始元素表頭，供獨立 VT 分類。
// Header 前兩個 bytes 是原始 vtable offset，不以 E1 命中推定分類。
type RasterElement struct {
	Steps           uint64
	Pointer, Method hd3d.FarPtr
	Header          [16]byte
}

type ModelRecorderCounts struct {
	GroupsOpened, GroupsDiscarded, GroupsSealed, Promotions uint64
	GroupsReplaced, No3D, OutsideGroup, MultiCamera         uint64
	HitsVoidedByDelivery, SignatureFailures, ReadFailures   uint64
	FaceRetMismatch, F1RUnpaired, UnmatchedFrame            uint64
}

type modelFrameKey struct{ ss, bp uint16 }
type modelFrameState struct {
	obj, face int
	ss, sp    uint16
	proj      [3]realspace.FarPtr
}

type modelHit struct {
	shade            *shadeHit
	sourceGeneration uint64
	sourceRole       *realspace.FaceSourceRole
	kind             uint16
	steps, del       uint64
	base             uint16
	valid            bool
	ss, sp, bp       uint16
	cam              realspace.Camera
	obj              realspace.ObjObs
	face             realspace.FaceObs
	proj             [3]realspace.FarPtr
	elem             realspace.FarPtr
	buf              []byte
	desc             [6]uint16
	list             []uint16
	readFailed       bool
	raster           *rasterHit
	reads            ModelReadCount
}

// ModelRecorder 只保留原版資料複本，從不保留 modelView。
// OnGroup 在 Frame 升格時呼叫；接收者不得修改這份已封存的複本。
type ModelRecorder struct {
	bounded             *boundedCapture
	boundedClosed       bool
	onHDSealed          func(*hdSealedCapture)
	onHDRejected        func(HDCaptureRejected)
	shade               *shadeCapture
	configurationLocked bool // lifecycle diagnostic only, never affects recorded output
	materialSources     bool
	sourceGeneration    uint64
	sourceProof         *realspace.FaceSourceProof
	sourceCounts        MaterialSourceCounts
	Library             *realspace.Library
	OnGroup             func(*ModelGroup)
	base                uint16
	baseOK              bool
	lastSig             uint64
	retryAt             uint64
	pend                *modelHit
	open                *ModelGroup
	await               *ModelGroup
	shown               *ModelGroup
	vga                 bool
	frames              map[modelFrameKey]modelFrameState
	tracker             cuTracker
	hooks               cuHooks
	counts              ModelRecorderCounts
	logs                []string
	raster              *rasterRecorder
}

func NewModelRecorder(lib *realspace.Library) *ModelRecorder {
	r := &ModelRecorder{Library: lib, frames: make(map[modelFrameKey]modelFrameState)}
	r.hooks.OnEntryAccepted = func(_ FlightView, steps uint64, _ uint16) {
		if r.bounded != nil {
			r.sealBoundedCU(steps)
			return
		}
		if r.await != nil {
			r.counts.GroupsReplaced++
		}
		r.shadeDrop(r.await)
		r.await = nil
		if r.open != nil && r.open.Buf3D != nil {
			g := r.open
			g.Key = steps
			for i := range g.Objects {
				if g.UnmatchedFrames != 0 {
					g.Objects[i].UnmatchedFrame += g.UnmatchedFrames
				}
				g.Objects[i].Verdict = realspace.CheckObject(r.Library, g.Cameras, &g.Objects[i])
			}
			if r.raster != nil {
				r.raster.check(g)
			}
			r.shadeSeal(g, steps)
			r.await = g
			r.counts.GroupsSealed++
		} else {
			r.counts.No3D++
			r.shadeDrop(r.open)
		}
		r.open = nil
		r.clearFrameStates()
		if r.raster != nil {
			r.raster.reset()
		}
		r.vga = false
	}
	r.hooks.OnMU = func(_ FlightView, cx uint16) {
		r.vga = cx == 0
		if r.vga {
			r.shadeReset()
			r.open = nil
			r.clearFrameStates()
			if r.raster != nil {
				r.raster.reset()
			}
		}
	}
	return r
}

func (r *ModelRecorder) Base() (uint16, bool)        { return r.base, r.baseOK }
func (r *ModelRecorder) Counts() ModelRecorderCounts { return r.counts }
func (r *ModelRecorder) Logs() []string              { return append([]string(nil), r.logs...) }
func (r *ModelRecorder) Visible() *ModelGroup {
	if r.bounded != nil || r.boundedClosed {
		return nil
	}
	return r.shown
}

func (r *ModelRecorder) note(s string) {
	if r.bounded != nil {
		return
	}
	if len(r.logs) < maxLog {
		r.logs = append(r.logs, s)
	}
}

type modelSig struct {
	seg, off uint16
	b        []byte
	reloc    []int
}

func modelHex(s string) []byte {
	b, err := hex.DecodeString(s)
	if err != nil {
		panic(err)
	}
	return b
}

// 全部數值是規格 012／013 的 +段偏移與原始 bytes；不是導覽別名。
var modelSigs = []modelSig{
	{0x1d5e, 0x000c, modelHex("c83800001e5657fcf7c40300742eb908"), nil},
	{0x1d5e, 0x0416, modelHex("c83800001e5657fcf7c40300742eb90c"), nil},
	{0x1d5e, 0x083d, modelHex("c85400001e5657fcf7c40300742eb908"), nil},
	{0x1d5e, 0x0ea8, modelHex("c86800001e5657fcf7c40300742eb912"), nil},
	{0x2198, 0x0c79, modelHex("c83101005657fc8b76061e07bfe613b9"), nil},
	{0x54b0, 0x01ed, modelHex("c8320000565716078b7606bfb038b903"), nil},
	{0x28b7, 0x0102, modelHex("558bec83ec06568b76069a2d02ae2252"), []int{13}},
	{0x28b7, 0x0192, modelHex("9a2400f527599a2d02ae225250665866a32a58803e4d3200740856"), []int{3, 9}},
	{0x1d3a, 0x0090, nil, nil}, // 沿用既有 sigJemm 的 98 bytes。
	{0x27f5, 0x0048, modelHex("ff1f83c404478346fc04397c027fe45f"), nil},
	{0x28b7, 0x0120, modelHex("bd005668b84f0ee838ff83c4048b445c"), nil},
	{0x1ad5, 0x0a51, modelHex("33f6e98c00c45ed28b46d88b56d62603"), nil},
	{0x1ad5, 0x0a7c, modelHex("ff5f1883c4080ac074578d8634ff5066"), nil},
	{0x1ad5, 0x0aa8, modelHex("83c40489569a89469866837e98007425"), nil},
}

func modelSigMatches(v modelView, base uint16, s modelSig) bool {
	if s.b == nil {
		return match(v, base+s.seg, s.off, sigJemm, 0)
	}
	var raw [98]byte
	b := raw[:len(s.b)]
	if !modelRead(v, base+s.seg, s.off, b) {
		return false
	}
	for i, want := range s.b {
		for _, at := range s.reloc {
			word := binary.LittleEndian.Uint16(s.b[at:]) + base
			if i == at {
				want = byte(word)
			}
			if i == at+1 {
				want = byte(word >> 8)
			}
		}
		if b[i] != want {
			return false
		}
	}
	return true
}

func (r *ModelRecorder) verify(v modelView, base uint16, all bool) bool {
	if r.bounded != nil {
		return r.verifyBounded(v, base, all)
	}
	if !all {
		return modelSigMatches(v, base, modelSigs[6]) && modelSigMatches(v, base, modelSigs[7])
	}
	for _, s := range modelSigs {
		if !modelSigMatches(v, base, s) {
			return false
		}
	}
	buf := make([]byte, 0xc8)
	if !modelRead(v, base+0x54b0, 0x0008, buf) {
		return false
	}
	want := modelHex("8cdc17f401184e97c84a978dfee33c057ed8e01dce1e09a66c06267045896792")
	got := sha256.Sum256(buf)
	return string(got[:]) == string(want)
}

func (r *ModelRecorder) disable(steps uint64) {
	r.shadeReset()
	if r.materialSources {
		r.sourceGeneration++
		r.sourceProof = nil
	}
	r.baseOK = false
	r.retryAt = steps + flightReverifySteps
	r.open, r.await, r.shown, r.pend = nil, nil, nil, nil
	r.clearFrameStates()
	if r.raster != nil {
		r.raster.reset()
	}
	r.tracker.Reset()
	r.tracker.SetBase(r.base, false)
	r.counts.SignatureFailures++
	if r.bounded == nil {
		r.note(fmt.Sprintf("第 %d 步：模型錄製簽章不符，停用", steps))
	}
}

func modelRead(v modelView, seg, off uint16, dst []byte) bool {
	for len(dst) > 0 {
		n := min(len(dst), 0x10000-int(off))
		if !v.ReadBlock(uint32(seg)*16+uint32(off), dst[:n]) {
			return false
		}
		dst = dst[n:]
		off = 0
	}
	return true
}

func modelWord(v modelView, seg, off uint16, ok *bool) uint16 {
	var b [2]byte
	if !modelRead(v, seg, off, b[:]) {
		*ok = false
	}
	return binary.LittleEndian.Uint16(b[:])
}
func modelDword(v modelView, seg, off uint16, ok *bool) int32 {
	var b [4]byte
	if !modelRead(v, seg, off, b[:]) {
		*ok = false
	}
	return int32(binary.LittleEndian.Uint32(b[:]))
}
func modelFar(v modelView, seg, off uint16, ok *bool) realspace.FarPtr {
	return realspace.FarPtr{Off: modelWord(v, seg, off, ok), Seg: modelWord(v, seg, off+2, ok)}
}

var modelTriggerIPs = [...]uint16{0x0102, 0x0197, 0x012a, 0x01ed, 0x0a51, 0x0a7c, 0x0a7f, 0x0aab, ipCUPlain, ipCUDiff, ipCUPlainRet, ipCUDiffRet, ipMU}

func (r *ModelRecorder) nextWake() uint64 {
	if r.pend != nil || r.materialSources && r.baseOK && r.sourceProof == nil {
		return 0
	}
	if r.baseOK {
		return min(r.lastSig+SigCheckSteps, r.shadeWake())
	}
	return session.NoWake
}

func (r *ModelRecorder) Observe(v modelView) {
	if r.boundedClosed {
		return
	}
	if r.bounded != nil {
		defer r.catchBoundedLimit(v.Steps())
	}
	r.configurationLocked = true
	// 先確認暫定命中。只存值與資料複本，不存上一個 StepView。
	if p := r.pend; p != nil {
		r.pend = nil
		if v.Deliveries() == p.del && (!r.materialSources || (v.Steps() > p.steps && p.sourceGeneration == r.sourceGeneration && (p.kind == 0x0102 || r.baseOK && p.base == r.base))) {
			r.apply(p)
		} else {
			r.shadeVoided(p)
			r.counts.HitsVoidedByDelivery++
			if r.materialSources && p.sourceRole != nil {
				r.sourceCounts.Voided++
			}
		}
		if r.bounded != nil {
			*p = modelHit{}
		}
	}
	ip := v.IP()
	if r.materialSources && r.baseOK && r.sourceProof == nil {
		if !r.verifyMaterialSource(v) {
			r.disable(v.Steps())
			return
		}
	}
	if r.baseOK && v.Steps()-r.lastSig >= SigCheckSteps {
		if !r.verify(v, r.base, true) {
			r.disable(v.Steps())
			return
		}
		r.lastSig = v.Steps()
		if r.materialSources && !r.verifyMaterialSource(v) {
			r.disable(v.Steps())
			return
		}
	}
	r.shadeBefore(v)
	interesting := r.materialSources && ip == 0x0aa5
	for _, x := range modelTriggerIPs {
		if ip == x {
			interesting = true
			break
		}
	}
	if !interesting && r.raster != nil {
		for _, x := range rasterTriggerIPs {
			if ip == x {
				interesting = true
				break
			}
		}
	}
	if !interesting {
		return
	}
	if ip == 0x0102 {
		base := v.CS() - 0x28b7
		if !r.baseOK && v.Steps() < r.retryAt {
			return
		}
		// 首次辨識只遮重定位 word；全簽章確認後才放進暫定槽。
		if !r.baseOK {
			b := modelSigs[6].b
			for i, w := range b {
				if i != 13 && i != 14 && v.Read8(lin(v.CS(), ip+uint16(i))) != w {
					return
				}
			}
		} else if v.CS() != r.base+0x28b7 {
			return
		}
		valid := r.verify(v, base, !r.baseOK)
		if r.bounded == nil {
			r.pend = &modelHit{kind: ip, steps: v.Steps(), del: v.Deliveries(), base: base, valid: valid, sourceGeneration: r.sourceGeneration}
		} else {
			h := &r.bounded.pending
			*h = modelHit{kind: ip, steps: v.Steps(), del: v.Deliveries(), base: base, valid: valid, sourceGeneration: r.sourceGeneration}
			r.pend = h
		}
		return
	}
	if !r.baseOK {
		return
	}
	// CU/MU 吃原始命中，不經單槽去重（012 §3.2.2）。
	switch ip {
	case ipCUPlain, ipCUDiff, ipCUPlainRet, ipCUDiffRet, ipMU:
		r.tracker.Observe(v, &r.hooks)
		return
	}
	if r.bounded != nil && (r.bounded.failed || r.open == nil) {
		return
	}
	if r.raster != nil && r.raster.matches(v, r.base) {
		r.pend = r.captureMeasured(v, ip)
		return
	}
	seg := uint16(0x1ad5)
	if ip == 0x012a || ip == 0x0197 {
		seg = 0x28b7
	}
	if ip == 0x01ed {
		seg = 0x54b0
	}
	if v.CS() != r.base+seg {
		return
	}
	r.pend = r.captureMeasured(v, ip)
}

func (r *ModelRecorder) captureMeasured(v modelView, ip uint16) *modelHit {
	if r.bounded != nil {
		counted := &r.bounded.counted
		*counted = modelReadCountingView{modelView: v}
		defer func() { counted.modelView = nil }()
		h := r.capture(counted, ip)
		h.reads = counted.count
		h.shade = r.captureShade(v, h)
		return h
	}
	counted := modelReadCountingView{modelView: v}
	h := r.capture(&counted, ip)
	h.reads = counted.count
	h.shade = r.captureShade(v, h)
	return h
}

func (r *ModelRecorder) capture(v modelView, ip uint16) *modelHit {
	var h *modelHit
	if r.bounded == nil {
		h = &modelHit{}
	} else {
		h = &r.bounded.pending
		*h = modelHit{}
	}
	h.kind, h.steps, h.del, h.ss, h.sp, h.bp, h.base, h.sourceGeneration = ip, v.Steps(), v.Deliveries(), v.SS(), v.SP(), v.BP(), r.base, r.sourceGeneration
	if r.raster != nil && r.raster.matches(v, r.base) {
		h.raster = r.raster.capture(v, r.base, r.open)
		if ip != 0x01ed {
			return h
		}
	}
	ok := true
	switch ip {
	case 0x012a:
		h.cam = realspace.Camera{Steps: h.steps, DS: v.DS(), CamObj: v.SI()}
		ok = modelRead(v, v.DS(), 0x4fb8, h.cam.Raw[:])
	case 0x01ed:
		desc := r.modelWord(v, v.SS(), v.SP()+4, &ok)
		for i := range h.desc {
			h.desc[i] = r.modelWord(v, v.DS(), desc+uint16(2*i), &ok)
		}
		if h.desc[2] > h.desc[4] || h.desc[3] > h.desc[5] || h.desc[4] > 319 || h.desc[5] > 199 {
			ok = false
			break
		}
		h.list = boundedMake[uint16](r.captureArena(), int(h.desc[5])+2)
		for i := range h.list {
			h.list[i] = r.modelWord(v, v.DS(), h.desc[1]+uint16(2*i), &ok)
		}
	case 0x0a51:
		o := &h.obj
		o.Steps = h.steps
		o.CamObj = r.modelWord(v, v.SS(), v.BP()+10, &ok)
		o.CamIdx = -1
		if r.open != nil {
			o.CamIdx = len(r.open.Cameras) - 1
		}
		o.Rec = r.modelFar(v, v.SS(), v.BP()+6, &ok)
		ok = modelRead(v, o.Rec.Seg, o.Rec.Off, o.RecRaw[:]) && ok
		o.RecVT = binary.LittleEndian.Uint16(o.RecRaw[:])
		o.Model = realspace.FarPtr{Off: binary.LittleEndian.Uint16(o.RecRaw[2:]), Seg: binary.LittleEndian.Uint16(o.RecRaw[4:])}
		o.LOD = o.RecRaw[9]
		ok = modelRead(v, o.Model.Seg, o.Model.Off, o.ModelHead[:]) && ok
		o.NFaces = int16(r.modelWord(v, v.SS(), v.BP()-0x30, &ok))
		list := r.modelFar(v, v.SS(), v.BP()-0x2e, &ok)
		o.FaceBase = r.modelFar(v, v.SS(), v.BP()-0x2a, &ok)
		n := int(binary.LittleEndian.Uint16(o.ModelHead[2:]))
		if n < 1 || n > 255 || o.NFaces < 1 || o.NFaces > 1024 {
			o.OutOfBounds = true
			break
		}
		if 0x22+4*int(o.LOD)+4 > len(o.ModelHead) || 0x42+2*int(o.LOD)+2 > len(o.ModelHead) {
			o.OutOfBounds = true
			break
		}
		want := realspace.FarPtr{Off: binary.LittleEndian.Uint16(o.ModelHead[0x22+4*int(o.LOD):]), Seg: binary.LittleEndian.Uint16(o.ModelHead[0x24+4*int(o.LOD):])}
		o.FaceTableMismatch = list != want || o.NFaces != int16(binary.LittleEndian.Uint16(o.ModelHead[0x42+2*int(o.LOD):]))
		vert := realspace.FarPtr{Off: binary.LittleEndian.Uint16(o.ModelHead[4:]), Seg: binary.LittleEndian.Uint16(o.ModelHead[6:])}
		buf := boundedMake[byte](r.captureArena(), n*24)
		ok = modelRead(v, vert.Seg, vert.Off, buf) && ok
		o.VertXYZ = boundedMake[byte](r.captureArena(), n*12)
		o.VertRT = boundedMake[[3]int32](r.captureArena(), n)
		for i := 0; i < n; i++ {
			copy(o.VertXYZ[i*12:], buf[i*24:i*24+12])
			for k := 0; k < 3; k++ {
				o.VertRT[i][k] = int32(binary.LittleEndian.Uint32(buf[i*24+k*4:]))
			}
		}
		o.VertSHA = sha256.Sum256(o.VertXYZ)
		if r.bounded != nil {
			legacyFree(r.captureArena(), buf)
		}
		o.FaceList = boundedMake[uint16](r.captureArena(), int(o.NFaces))
		for i := range o.FaceList {
			o.FaceList[i] = r.modelWord(v, list.Seg, list.Off+uint16(i*2), &ok)
		}
		flagBytes := r.scalarBuffer(1)
		ok = modelRead(v, v.DS(), 0x17bc, flagBytes) && ok
		o.Flag17BC = flagBytes[0]
		o.Near = r.modelDword(v, v.DS(), 0x1800, &ok)
		o.Far = r.modelDword(v, v.DS(), 0x51f0, &ok)
	case 0x0a7c:
		h.face.SI = v.SI()
		h.face.VIdx = [3]int16{-1, -1, -1}
		h.face.Face = r.modelFar(v, v.SS(), v.BP()-0x64, &ok)
		raw := r.faceRawBuffer()
		ok = modelRead(v, h.face.Face.Seg, h.face.Face.Off, raw) && ok
		h.face.FaceVT = binary.LittleEndian.Uint16(raw[:])
		h.face.Method = r.modelFar(v, v.DS(), h.face.FaceVT+0x18, &ok)
		geom := realspace.FarPtr{Off: binary.LittleEndian.Uint16(raw[2:]), Seg: binary.LittleEndian.Uint16(raw[4:])}
		ok = modelRead(v, geom.Seg, geom.Off, h.face.Geom[:]) && ok
		projSeg := binary.LittleEndian.Uint16(raw[8:])
		h.proj = [3]realspace.FarPtr{{Off: binary.LittleEndian.Uint16(raw[6:]), Seg: projSeg}, {Off: binary.LittleEndian.Uint16(raw[10:]), Seg: projSeg}, {Off: binary.LittleEndian.Uint16(raw[14:]), Seg: projSeg}}
		if st, found := r.frameState(modelFrameKey{h.ss, h.bp}); found && r.open != nil {
			o := &r.open.Objects[st.obj]
			vert := realspace.FarPtr{Off: binary.LittleEndian.Uint16(o.ModelHead[4:]), Seg: binary.LittleEndian.Uint16(o.ModelHead[6:])}
			gs := binary.LittleEndian.Uint16(h.face.Geom[0x1c:])
			for i, at := range []int{0x1a, 0x1e, 0x22} {
				p := realspace.FarPtr{Off: binary.LittleEndian.Uint16(h.face.Geom[at:]), Seg: gs}
				d := int64(p.Linear()) - int64(vert.Linear())
				if d >= 0 && d%24 == 0 && d/24 < int64(len(o.VertRT)) {
					h.face.VIdx[i] = int16(d / 24)
				}
			}
		}
	case 0x0a7f:
		h.face.AL = uint8(v.AX())
		if st, found := r.frameState(modelFrameKey{h.ss, h.bp}); found && st.face >= 0 {
			for i, p := range st.proj {
				ok = modelRead(v, p.Seg, p.Off, h.face.Proj[i][:]) && ok
			}
		}
	case 0x0aab:
		h.elem = realspace.FarPtr{Off: v.AX(), Seg: v.DX()}
	case 0x0197:
		if r.open != nil && len(r.open.List) > 0 {
			h.buf = boundedMake[byte](r.captureArena(), 64000)
			ok = modelRead(v, r.open.Desc[0], r.open.List[0], h.buf)
		} else {
			ok = false
		}
	}
	h.readFailed = !ok
	h.obj.ReadFailed = !ok
	if r.materialSources && (ip == 0x0a7c || ip == 0x0a7f || ip == 0x0aa5 || ip == 0x0aab) {
		if r.bounded == nil {
			role := r.captureMaterialRole(v)
			h.sourceRole = &role
		} else {
			r.bounded.role = r.captureBoundedRole(v)
			h.sourceRole = &r.bounded.role
		}
		role := *h.sourceRole
		if ip == 0x0a7c {
			o1 := uint64(0)
			if st, found := r.frameState(modelFrameKey{h.ss, h.bp}); found && r.open != nil {
				o1 = r.open.Objects[st.obj].Steps
			}
			h.face.Source = boundedNew[realspace.FaceSourceObs](r.captureArena())
			*h.face.Source = realspace.FaceSourceObs{Generation: r.sourceGeneration, O1Steps: o1, F1Call: role, Proof: r.sourceProof}
			if r.bounded != nil {
				h.face.Source.Proof = r.groupProof()
			}
		}
	}
	return h
}

func (r *ModelRecorder) apply(h *modelHit) {
	defer r.applyShade(h)
	if h.kind == 0x0102 {
		if !h.valid {
			r.disable(h.steps)
			return
		}
		if !r.baseOK {
			r.base = h.base
			r.baseOK = true
			if r.materialSources {
				r.sourceGeneration++
				r.sourceProof = nil
			}
			r.lastSig = h.steps
			r.tracker.SetBase(r.base, true)
		}
		if r.open != nil {
			r.counts.GroupsDiscarded++
			if r.bounded == nil {
				r.note(fmt.Sprintf("第 %d 步：R7 取代第 %d 步尚未封存的模型群組", h.steps, r.open.Steps))
			}
		}
		r.shadeDrop(r.open)
		r.open = nil
		r.clearFrameStates()
		if r.raster != nil {
			r.raster.reset()
		}
		if !r.vga {
			if r.bounded == nil {
				r.open = &ModelGroup{Steps: h.steps}
			} else {
				r.startBoundedGroup(h.steps)
			}
			r.counts.GroupsOpened++
		}
		return
	}
	if r.open == nil {
		r.counts.OutsideGroup++
		return
	}
	g := r.open
	if r.bounded != nil {
		r.addBoundedRead(h.kind, h.reads)
	} else if h.reads.Calls != 0 {
		if g.Reads == nil {
			g.Reads = make(map[uint16]ModelReadCount)
		}
		n := g.Reads[h.kind]
		n.Calls += h.reads.Calls
		n.Bytes += h.reads.Bytes
		g.Reads[h.kind] = n
	}
	if h.readFailed {
		g.ReadFailed = true
		r.counts.ReadFailures++
	}
	key := modelFrameKey{h.ss, h.bp}
	switch h.kind {
	case 0x012a:
		if len(g.Cameras) > 0 {
			r.counts.MultiCamera++
		}
		boundedAppend(r.captureArena(), &g.Cameras, h.cam)
		boundedAppend(r.captureArena(), &g.CameraReadFailed, h.readFailed)
	case 0x01ed:
		if g.List == nil {
			g.Desc = h.desc
			g.List = h.list
		}
	case 0x0a51:
		if h.obj.CamIdx >= 0 && h.obj.CamIdx < len(g.CameraReadFailed) {
			h.obj.ReadFailed = h.obj.ReadFailed || g.CameraReadFailed[h.obj.CamIdx]
		}
		boundedAppend(r.captureArena(), &g.Objects, h.obj)
		r.setFrameState(key, modelFrameState{obj: len(g.Objects) - 1, face: -1})
	case 0x0a7c:
		st, found := r.frameState(key)
		if !found {
			r.counts.UnmatchedFrame++
			g.UnmatchedFrames++
			return
		}
		o := &g.Objects[st.obj]
		o.ReadFailed = o.ReadFailed || h.readFailed
		boundedAppend(r.captureArena(), &o.Faces, h.face)
		if h.face.Source != nil {
			r.sourceCounts.F1Call++
		}
		st.face = len(o.Faces) - 1
		st.ss, st.sp, st.proj = h.ss, h.sp, h.proj
		r.setFrameState(key, st)
	case 0x0a7f:
		st, found := r.frameState(key)
		if !found {
			r.counts.UnmatchedFrame++
			g.UnmatchedFrames++
			return
		}
		o := &g.Objects[st.obj]
		if st.face < 0 || o.Faces[st.face].Returned {
			o.F1RUnpaired++
			r.counts.F1RUnpaired++
			return
		}
		if st.ss != h.ss || st.sp != h.sp {
			o.FaceRetMismatch++
			r.counts.FaceRetMismatch++
			return
		}
		o.ReadFailed = o.ReadFailed || h.readFailed
		f := &o.Faces[st.face]
		f.Returned, f.AL, f.Proj = true, h.face.AL, h.face.Proj
		if f.Source != nil && h.sourceRole != nil {
			f.Source.F1Return = *h.sourceRole
			r.sourceCounts.F1Return++
		}
	case 0x0aa5:
		if st, found := r.frameState(key); found && st.face >= 0 {
			f := &g.Objects[st.obj].Faces[st.face]
			if f.Source != nil && h.sourceRole != nil {
				f.Source.BuildCall = *h.sourceRole
				r.sourceCounts.BuildCall++
			}
		}
	case 0x0aab:
		st, found := r.frameState(key)
		if !found || st.face < 0 {
			r.counts.UnmatchedFrame++
			g.UnmatchedFrames++
			return
		}
		f := &g.Objects[st.obj].Faces[st.face]
		if !f.Returned || f.AL == 0 {
			g.Objects[st.obj].UnmatchedFrame++
			r.counts.UnmatchedFrame++
			return
		}
		f.Elem = h.elem
		if f.Source != nil && h.sourceRole != nil {
			f.Source.ElementReturn = *h.sourceRole
			f.Source.Element = h.elem
			r.sourceCounts.ElementReturn++
		}
	case 0x0197:
		g.EndSteps = h.steps
		g.Buf3D = h.buf
	}
	if h.raster != nil {
		r.raster.apply(g, h.raster)
	}
}

func (r *ModelRecorder) Frame(steps uint64) {
	if !r.baseOK {
		return
	}
	if promote, key := r.tracker.Frame(steps, &r.hooks); promote {
		r.shadeDrop(r.shown)
		r.shown = r.await
		r.await = nil
		if r.shown != nil {
			if r.bounded == nil {
				r.shown.Key = key
			}
			r.counts.Promotions++
			if r.bounded == nil && r.OnGroup != nil {
				r.OnGroup(r.shown)
			}
		}
	}
}

// VideoModeChange 清除三個群組槽與暫定命中，保留仍需重新辨識簽章的基底資料。
// 不寫原版狀態；條件來自 012 §3.7.2。
func (r *ModelRecorder) VideoModeChange() {
	r.shadeReset()
	if r.materialSources {
		r.sourceGeneration++
		r.sourceProof = nil
	}
	r.open, r.await, r.shown, r.pend = nil, nil, nil, nil
	r.clearFrameStates()
	if r.raster != nil {
		r.raster.reset()
	}
	r.tracker.Reset()
	r.vga = false
}

// MaterialSourceCounts is separate from the original default recorder counters.
// Opt-in adds no default JSON fields or process-global epoch counters.
type MaterialSourceCounts struct{ F1Call, F1Return, BuildCall, ElementReturn, ReadFailed, Voided, SignatureChecks uint64 }

func (r *ModelRecorder) EnableMaterialSources() {
	if !r.materialSources {
		r.materialSources = true
		r.sourceGeneration++
		r.sourceProof = nil
	}
}
func (r *ModelRecorder) MaterialSourcesEnabled() bool               { return r.materialSources }
func (r *ModelRecorder) MaterialSourceCounts() MaterialSourceCounts { return r.sourceCounts }
func (r *ModelRecorder) MaterialSourceStatus() (generation uint64, valid bool) {
	return r.sourceGeneration, r.materialSources && r.baseOK && r.sourceProof != nil
}
func (r *ModelRecorder) verifyMaterialSource(v modelView) bool {
	if r.bounded != nil {
		return r.verifyBoundedMaterial(v)
	}
	p := &realspace.FaceSourceProof{Rule: realspace.FaceSourceRule, Base: r.base, Generation: r.sourceGeneration}
	for _, sig := range realspace.FaceSourceSignatures() {
		want := realspace.SourceSignatureBytes(sig, r.base)
		raw := make([]byte, len(want))
		if !modelRead(v, r.base+sig.Seg, sig.Off, raw) || !bytes.Equal(raw, want) {
			return false
		}
		p.Bytes = append(p.Bytes, realspace.SourceBytes{Seg: r.base + sig.Seg, Off: sig.Off, Data: raw})
	}
	r.sourceProof = p
	r.sourceCounts.SignatureChecks++
	return true
}
func (r *ModelRecorder) captureMaterialRole(v modelView) realspace.FaceSourceRole {
	if r.bounded != nil {
		return r.captureBoundedRole(v)
	}
	q := realspace.FaceSourceRole{Steps: v.Steps(), Deliveries: v.Deliveries(), Generation: r.sourceGeneration, SS: v.SS(), SP: v.SP(), BP: v.BP(), SI: v.SI()}
	ok := true
	q.Face = r.modelFar(v, v.SS(), v.BP()-0x64, &ok)
	if v.IP() == 0x0a7c || v.IP() == 0x0aa5 {
		q.HasFaceArgument = true
		q.FaceArgument = r.modelFar(v, v.SS(), v.SP(), &ok)
	}
	if st, found := r.frameState(modelFrameKey{q.SS, q.BP}); found && r.open != nil && st.obj >= 0 && st.obj < len(r.open.Objects) {
		q.Rec = r.open.Objects[st.obj].Rec
	} else {
		ok = false
	}
	ok = modelRead(v, q.Face.Seg, q.Face.Off, q.Raw[:]) && ok
	vt := binary.LittleEndian.Uint16(q.Raw[:])
	q.MethodF1 = r.modelFar(v, v.DS(), vt+0x18, &ok)
	q.MethodBuild = r.modelFar(v, v.DS(), vt+0x1c, &ok)
	first := binary.LittleEndian.Uint16(q.Raw[8:])
	for i, at := range []int{6, 10, 14} {
		q.FullFar[i] = realspace.FarPtr{Off: binary.LittleEndian.Uint16(q.Raw[at:]), Seg: binary.LittleEndian.Uint16(q.Raw[at+2:])}
		ok = modelRead(v, q.FullFar[i].Seg, q.FullFar[i].Off, q.FullProj[i][:]) && ok
		ok = modelRead(v, first, q.FullFar[i].Off, q.ConventionProj[i][:]) && ok
	}
	q.ReadFailed = !ok
	if !ok {
		r.sourceCounts.ReadFailed++
	}
	return q
}

// BEGIN bounded capture shared contract 002. Coordinate changes with the registry owner.
type hdPatchJob struct {
	Call, Start, End int
	Before, After    []byte
}
type hdReadCount struct {
	IP    uint16
	Count ModelReadCount
}
type hdSealedCapture struct {
	Group                                  *ModelGroup
	Arena                                  *legacyArena
	PatchJobs                              []hdPatchJob
	Reads                                  []hdReadCount
	ProfileVersion                         string
	LegacyLimit, ExtraLimit, Serial, CUKey uint64
	ShadeEpoch, SignatureGeneration        uint64
	SignatureValid                         bool
}
type HDCaptureRejectReason uint8

const (
	HDCaptureQuota HDCaptureRejectReason = iota + 1
	HDCaptureClosed
)

type HDCaptureRejected struct {
	Serial, Steps, CUKey uint64
	Reason               HDCaptureRejectReason
}

// END bounded capture shared contract 002.

// Bounded producer control is private and distinct from the legacy public API.
type hdFrameEntry struct {
	Key   modelFrameKey
	State modelFrameState
}
type boundedGroupState struct {
	frames []hdFrameEntry
	proof  *realspace.FaceSourceProof
}
type boundedLimit struct{}
type boundedCapture struct {
	pool             *shadesource.ProtectedProducer
	control          *legacyArena
	fixedBytes       uint64
	pending          modelHit
	rasterPending    rasterHit
	role             realspace.FaceSourceRole
	counted          modelReadCountingView
	scalar           [4]byte
	raw16            [16]byte
	sig98            [98]byte
	table200         [200]byte
	signatures       []realspace.SourceSignature
	proof            *realspace.FaceSourceProof
	wants            [][]byte
	serial           uint64
	failed           bool
	rejectedNotified bool // fixed control state, not serialized or a new lease
	rejected         HDCaptureRejected
}

const boundedSignatureMetadata uint64 = 32768 // Fixed reservation before the legacy signature API allocates its immutable table copy.
func boundedMake[T any](a *legacyArena, n int) []T {
	v, e := legacyMake[T](a, n)
	if e != nil {
		panic(boundedLimit{})
	}
	return v
}
func boundedNew[T any](a *legacyArena) *T {
	v, e := legacyNew[T](a)
	if e != nil {
		panic(boundedLimit{})
	}
	return v
}
func boundedAppend[T any](a *legacyArena, p *[]T, v T) {
	if e := legacyAppend(a, p, v); e != nil {
		panic(boundedLimit{})
	}
}
func (r *ModelRecorder) captureArena() *legacyArena {
	if r.bounded == nil {
		return nil
	}
	if r.open == nil || r.open.hd == nil {
		panic(boundedLimit{})
	}
	return r.open.hd.Arena
}

// EnableBoundedHDCapture must follow explicit source opt-in. It never reads VM state.
func (r *ModelRecorder) EnableBoundedHDCapture(legacy, extra uint64) (err error) {
	if r == nil || r.configurationLocked || r.bounded != nil || r.boundedClosed || r.shade == nil || r.shade.closed || r.raster == nil {
		return &shadesource.Error{Status: shadesource.H3, Reason: shadesource.InvalidSchema, Detail: "bounded capture requires unused explicit shade recorder"}
	}
	pool, err := r.shade.owner.EnableProtectedProducer(legacy, extra)
	if err != nil {
		return err
	}
	var arena *legacyArena
	var fixed uint64
	defer func() {
		if x := recover(); x != nil {
			if _, ok := x.(boundedLimit); !ok {
				panic(x)
			}
			err = &shadesource.Error{Status: shadesource.H3, Reason: shadesource.ResourceLimit, Detail: "bounded capture control allocation rejected"}
		}
		if err != nil {
			if arena != nil {
				arena.retire()
			}
			if fixed != 0 {
				pool.ReleaseLegacy(fixed)
			}
			_ = pool.RollbackUnused()
		}
	}()
	arena, err = newLegacyArena(pool)
	if err != nil {
		return err
	}
	fixed = boundedSignatureMetadata + uint64(cap(r.raster.tables.R6Table))
	if err = pool.ReserveLegacy(fixed); err != nil {
		fixed = 0
		return err
	}
	b := boundedNew[boundedCapture](arena)
	b.pool, b.control, b.fixedBytes = pool, arena, fixed
	// FaceSourceSignatures copies only this fixed, frozen compile-time table. Its
	// complete returned capacities are covered before the call, not after adoption.
	b.signatures = realspace.FaceSourceSignatures()
	used := uint64(cap(b.signatures)) * uint64(reflect.TypeFor[realspace.SourceSignature]().Size())
	for _, sig := range b.signatures {
		used += uint64(cap(sig.Reloc)) * uint64(reflect.TypeFor[int]().Size())
	}
	if used > boundedSignatureMetadata {
		panic(boundedLimit{})
	}
	b.proof = boundedNew[realspace.FaceSourceProof](arena)
	b.proof.Bytes = boundedMake[realspace.SourceBytes](arena, len(b.signatures))
	b.wants = boundedMake[[]byte](arena, len(b.signatures))
	for i, sig := range b.signatures {
		n := len(sig.Hex) / 2
		b.proof.Bytes[i].Data = boundedMake[byte](arena, n)
		b.wants[i] = boundedMake[byte](arena, n)
	}
	r.frames = nil
	r.sourceProof = nil
	r.bounded = b
	r.raster.pending = &b.rasterPending
	r.raster.colorBuf = b.scalar[:1]
	return nil
}
func (r *ModelRecorder) catchBoundedLimit(steps uint64) {
	if x := recover(); x != nil {
		if _, ok := x.(boundedLimit); !ok {
			panic(x)
		}
		r.rejectBounded(HDCaptureQuota, steps)
	}
}
func (r *ModelRecorder) rejectBounded(reason HDCaptureRejectReason, steps uint64) {
	b := r.bounded
	if b == nil {
		return
	}
	if !b.failed {
		b.rejected = HDCaptureRejected{Serial: b.serial, Steps: steps, CUKey: b.rejected.CUKey, Reason: reason}
		b.rejectedNotified = false
	}
	b.failed = true
	if r.pend != nil {
		r.shadeVoided(r.pend)
	}
	r.pend = nil
	r.shadeDrop(r.open)
	r.open = nil
	b.pending = modelHit{}
	b.rasterPending = rasterHit{}
	b.role = realspace.FaceSourceRole{}
	b.counted.modelView = nil
	if r.raster != nil {
		r.raster.reset()
	}
}
func (r *ModelRecorder) startBoundedGroup(steps uint64) {
	b := r.bounded
	b.serial++
	b.failed = false
	b.rejectedNotified = false
	b.rejected = HDCaptureRejected{}
	a, e := newLegacyArena(b.pool)
	if e != nil {
		r.rejectBounded(HDCaptureQuota, steps)
		return
	}
	// Give rejectBounded ownership before any following allocation can fail.
	var g *ModelGroup
	defer func() {
		if x := recover(); x != nil {
			if g == nil || g.hd == nil {
				r.open = nil
				a.retire()
			}
			panic(x)
		}
	}()
	g = boundedNew[ModelGroup](a)
	r.open = g
	g.Steps = steps
	sc := boundedNew[hdSealedCapture](a)
	sc.Group, sc.Arena = g, a
	g.hd = sc
	st := boundedNew[boundedGroupState](a)
	g.hdState = st
	u := b.pool.Usage()
	*sc = hdSealedCapture{Group: g, Arena: a, ProfileVersion: shadesource.ProtectedProducerVersion, LegacyLimit: u.LegacyLimit, ExtraLimit: u.ExtraLimit, Serial: b.serial}
	if r.raster != nil {
		r.raster.arena = a
	}
}

// A proven CU entry must always return normally after a quota failure so the
// shared tracker can commit the same CU before Presenter binds its image.
func (r *ModelRecorder) sealBoundedCU(key uint64) {
	b := r.bounded
	if b.rejected.CUKey != key {
		b.rejectedNotified = false
	}
	b.rejected.CUKey, b.rejected.Serial = key, b.serial
	defer r.finishBoundedCU()
	defer func() {
		if x := recover(); x != nil {
			if _, ok := x.(boundedLimit); !ok {
				panic(x)
			}
			r.rejectBounded(HDCaptureQuota, key)
			// A private necessary setup may fail after the producer reached await.
			// Drop those roles too; any real external refs still retain their charge.
			await, shown := r.await, r.shown
			r.await, r.shown = nil, nil
			r.shadeDrop(await)
			r.shadeDrop(shown)
			r.notifyBoundedCURejected(key)
		}
	}()
	r.shadeDrop(r.await)
	r.shadeDrop(r.shown)
	r.await, r.shown = nil, nil
	if b.failed {
		r.notifyBoundedCURejected(key)
	} else if r.open != nil && r.open.Buf3D != nil {
		g := r.open
		g.Key = key
		g.hd.CUKey = key
		g.hd.ShadeEpoch = r.shade.epoch
		g.hd.SignatureGeneration, g.hd.SignatureValid = r.MaterialSourceStatus()
		if g.UnmatchedFrames != 0 {
			for i := range g.Objects {
				g.Objects[i].UnmatchedFrame += g.UnmatchedFrames
			}
		}
		if r.raster != nil {
			r.raster.checkOpenFrames(g)
		}
		r.shadeSeal(g, key)
		g.hd.Arena.seal()
		r.await = g
		if r.onHDSealed != nil {
			r.onHDSealed(g.hd)
		}
		r.counts.GroupsSealed++
	} else {
		r.counts.No3D++
		r.shadeDrop(r.open)
	}
}

func (r *ModelRecorder) notifyBoundedCURejected(key uint64) {
	b := r.bounded
	if b.rejectedNotified && b.rejected.CUKey == key {
		return
	}
	t := b.rejected
	t.CUKey, t.Serial = key, b.serial
	b.rejected, b.rejectedNotified = t, true
	if r.onHDRejected != nil {
		r.onHDRejected(t)
	}
}

func (r *ModelRecorder) finishBoundedCU() {
	r.open = nil
	b := r.bounded
	b.pending = modelHit{}
	b.rasterPending = rasterHit{}
	b.role = realspace.FaceSourceRole{}
	if r.raster != nil {
		r.raster.reset()
	}
	r.vga = false
}
func (r *ModelRecorder) frameState(key modelFrameKey) (modelFrameState, bool) {
	if r.bounded == nil {
		v, ok := r.frames[key]
		return v, ok
	}
	if r.open != nil && r.open.hdState != nil {
		for _, e := range r.open.hdState.frames {
			if e.Key == key {
				return e.State, true
			}
		}
	}
	return modelFrameState{}, false
}
func (r *ModelRecorder) setFrameState(key modelFrameKey, st modelFrameState) {
	if r.bounded == nil {
		r.frames[key] = st
		return
	}
	a := r.captureArena()
	p := &r.open.hdState.frames
	for i := range *p {
		if (*p)[i].Key == key {
			(*p)[i].State = st
			return
		}
	}
	boundedAppend(a, p, hdFrameEntry{key, st})
}
func (r *ModelRecorder) clearFrameStates() {
	if r.bounded == nil {
		clear(r.frames)
	} else if r.open != nil && r.open.hdState != nil {
		r.open.hdState.frames = r.open.hdState.frames[:0]
	}
}
func (r *ModelRecorder) addBoundedRead(ip uint16, n ModelReadCount) {
	if n.Calls == 0 {
		return
	}
	p := &r.open.hd.Reads
	for i := range *p {
		if (*p)[i].IP == ip {
			(*p)[i].Count.Calls += n.Calls
			(*p)[i].Count.Bytes += n.Bytes
			return
		}
	}
	boundedAppend(r.captureArena(), p, hdReadCount{ip, n})
}
func (r *ModelRecorder) groupProof() *realspace.FaceSourceProof {
	if r.sourceProof == nil {
		return nil
	}
	s := r.open.hdState
	if s.proof != nil {
		return s.proof
	}
	a := r.captureArena()
	p := boundedNew[realspace.FaceSourceProof](a)
	*p = *r.sourceProof
	p.Bytes = boundedMake[realspace.SourceBytes](a, len(r.sourceProof.Bytes))
	for i, v := range r.sourceProof.Bytes {
		p.Bytes[i] = v
		p.Bytes[i].Data = boundedMake[byte](a, len(v.Data))
		copy(p.Bytes[i].Data, v.Data)
	}
	s.proof = p
	return p
}
func (r *ModelRecorder) modelWord(v modelView, seg, off uint16, ok *bool) uint16 {
	if r.bounded == nil {
		return modelWord(v, seg, off, ok)
	}
	b := r.bounded.scalar[:2]
	clear(b)
	if !modelRead(v, seg, off, b) {
		*ok = false
	}
	return binary.LittleEndian.Uint16(b)
}
func (r *ModelRecorder) modelDword(v modelView, seg, off uint16, ok *bool) int32 {
	if r.bounded == nil {
		return modelDword(v, seg, off, ok)
	}
	b := r.bounded.scalar[:4]
	clear(b)
	if !modelRead(v, seg, off, b) {
		*ok = false
	}
	return int32(binary.LittleEndian.Uint32(b))
}
func (r *ModelRecorder) modelFar(v modelView, seg, off uint16, ok *bool) realspace.FarPtr {
	return realspace.FarPtr{Off: r.modelWord(v, seg, off, ok), Seg: r.modelWord(v, seg, off+2, ok)}
}
func (r *ModelRecorder) verifyBounded(v modelView, base uint16, all bool) bool {
	lo, hi := 0, len(modelSigs)
	if !all {
		lo, hi = 6, 8
	}
	for _, s := range modelSigs[lo:hi] {
		if s.b == nil {
			if !match(v, base+s.seg, s.off, sigJemm, 0) {
				return false
			}
			continue
		}
		b := r.bounded.sig98[:len(s.b)]
		if !modelRead(v, base+s.seg, s.off, b) {
			return false
		}
		for i, want := range s.b {
			for _, at := range s.reloc {
				w := binary.LittleEndian.Uint16(s.b[at:]) + base
				if i == at {
					want = byte(w)
				}
				if i == at+1 {
					want = byte(w >> 8)
				}
			}
			if b[i] != want {
				return false
			}
		}
	}
	if !all {
		return true
	}
	buf := r.bounded.table200[:]
	if !modelRead(v, base+0x54b0, 0x0008, buf) {
		return false
	}
	want := [32]byte{0x8c, 0xdc, 0x17, 0xf4, 0x01, 0x18, 0x4e, 0x97, 0xc8, 0x4a, 0x97, 0x8d, 0xfe, 0xe3, 0x3c, 0x05, 0x7e, 0xd8, 0xe0, 0x1d, 0xce, 0x1e, 0x09, 0xa6, 0x6c, 0x06, 0x26, 0x70, 0x45, 0x89, 0x67, 0x92}
	return sha256.Sum256(buf) == want
}
func (r *ModelRecorder) verifyBoundedMaterial(v modelView) bool {
	b := r.bounded
	p := b.proof
	p.Rule, p.Base, p.Generation = realspace.FaceSourceRule, r.base, r.sourceGeneration
	for i, sig := range b.signatures {
		want := b.wants[i]
		for j := range want {
			want[j] = boundedHexNibble(sig.Hex[2*j])<<4 | boundedHexNibble(sig.Hex[2*j+1])
		}
		for _, at := range sig.Reloc {
			binary.LittleEndian.PutUint16(want[at:], binary.LittleEndian.Uint16(want[at:])+r.base)
		}
		raw := p.Bytes[i].Data
		if !modelRead(v, r.base+sig.Seg, sig.Off, raw) || !bytes.Equal(raw, want) {
			return false
		}
		p.Bytes[i].Seg, p.Bytes[i].Off = r.base+sig.Seg, sig.Off
	}
	r.sourceProof = p
	r.sourceCounts.SignatureChecks++
	return true
}
func (r *ModelRecorder) captureBoundedRole(v modelView) realspace.FaceSourceRole {
	q := &r.bounded.role
	*q = realspace.FaceSourceRole{Steps: v.Steps(), Deliveries: v.Deliveries(), Generation: r.sourceGeneration, SS: v.SS(), SP: v.SP(), BP: v.BP(), SI: v.SI()}
	ok := true
	q.Face = r.modelFar(v, v.SS(), v.BP()-0x64, &ok)
	if v.IP() == 0x0a7c || v.IP() == 0x0aa5 {
		q.HasFaceArgument = true
		q.FaceArgument = r.modelFar(v, v.SS(), v.SP(), &ok)
	}
	if st, found := r.frameState(modelFrameKey{q.SS, q.BP}); found && r.open != nil && st.obj >= 0 && st.obj < len(r.open.Objects) {
		q.Rec = r.open.Objects[st.obj].Rec
	} else {
		ok = false
	}
	ok = modelRead(v, q.Face.Seg, q.Face.Off, q.Raw[:]) && ok
	vt := binary.LittleEndian.Uint16(q.Raw[:])
	q.MethodF1 = r.modelFar(v, v.DS(), vt+0x18, &ok)
	q.MethodBuild = r.modelFar(v, v.DS(), vt+0x1c, &ok)
	first := binary.LittleEndian.Uint16(q.Raw[8:])
	for i, at := range []int{6, 10, 14} {
		q.FullFar[i] = realspace.FarPtr{Off: binary.LittleEndian.Uint16(q.Raw[at:]), Seg: binary.LittleEndian.Uint16(q.Raw[at+2:])}
		ok = modelRead(v, q.FullFar[i].Seg, q.FullFar[i].Off, q.FullProj[i][:]) && ok
		ok = modelRead(v, first, q.FullFar[i].Off, q.ConventionProj[i][:]) && ok
	}
	q.ReadFailed = !ok
	if !ok {
		r.sourceCounts.ReadFailed++
	}
	return *q
}

func (r *ModelRecorder) scalarBuffer(n int) []byte {
	if r.bounded != nil {
		b := r.bounded.scalar[:n]
		clear(b)
		return b
	}
	return make([]byte, n)
}
func (r *ModelRecorder) faceRawBuffer() []byte {
	if r.bounded != nil {
		b := r.bounded.raw16[:]
		clear(b)
		return b
	}
	return make([]byte, 16)
}
func boundedHexNibble(b byte) byte {
	if b >= '0' && b <= '9' {
		return b - '0'
	}
	if b >= 'a' && b <= 'f' {
		return b - 'a' + 10
	}
	panic("invalid fixed signature hex")
}

func (r *ModelRecorder) closeBoundedCapture() {
	b := r.bounded
	if b == nil {
		return
	}
	if r.pend != nil {
		r.shadeVoided(r.pend)
	}
	r.pend = nil
	r.shadeDrop(r.open)
	r.shadeDrop(r.await)
	r.shadeDrop(r.shown)
	r.open, r.await, r.shown = nil, nil, nil
	r.sourceProof = nil
	r.frames = nil
	b.pending = modelHit{}
	b.rasterPending = rasterHit{}
	b.role = realspace.FaceSourceRole{}
	b.counted.modelView = nil
	b.signatures = nil
	b.proof = nil
	b.wants = nil
	if r.raster != nil {
		r.raster.reset()
		r.raster.pending = nil
		r.raster.colorBuf = nil
		r.raster.tables.R6Table = nil
	}
	pool, fixed, control := b.pool, b.fixedBytes, b.control
	r.bounded = nil
	r.boundedClosed = true
	control.retire()
	pool.ReleaseLegacy(fixed)
}
