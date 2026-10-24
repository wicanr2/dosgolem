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

	"github.com/wicanr2/dosgolem/apps/strikecommander/hd3d"
	"github.com/wicanr2/dosgolem/apps/strikecommander/realspace"
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
	materialSources  bool
	sourceGeneration uint64
	sourceProof      *realspace.FaceSourceProof
	sourceCounts     MaterialSourceCounts
	Library          *realspace.Library
	OnGroup          func(*ModelGroup)
	base             uint16
	baseOK           bool
	lastSig          uint64
	retryAt          uint64
	pend             *modelHit
	open             *ModelGroup
	await            *ModelGroup
	shown            *ModelGroup
	vga              bool
	frames           map[modelFrameKey]modelFrameState
	tracker          cuTracker
	hooks            cuHooks
	counts           ModelRecorderCounts
	logs             []string
	raster           *rasterRecorder
}

func NewModelRecorder(lib *realspace.Library) *ModelRecorder {
	r := &ModelRecorder{Library: lib, frames: make(map[modelFrameKey]modelFrameState)}
	r.hooks.OnEntryAccepted = func(_ FlightView, steps uint64, _ uint16) {
		if r.await != nil {
			r.counts.GroupsReplaced++
		}
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
			r.await = g
			r.counts.GroupsSealed++
		} else {
			r.counts.No3D++
		}
		r.open = nil
		clear(r.frames)
		if r.raster != nil {
			r.raster.reset()
		}
		r.vga = false
	}
	r.hooks.OnMU = func(_ FlightView, cx uint16) {
		r.vga = cx == 0
		if r.vga {
			r.open = nil
			clear(r.frames)
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
func (r *ModelRecorder) Visible() *ModelGroup        { return r.shown }

func (r *ModelRecorder) note(s string) {
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
	if r.materialSources {
		r.sourceGeneration++
		r.sourceProof = nil
	}
	r.baseOK = false
	r.retryAt = steps + flightReverifySteps
	r.open, r.await, r.shown, r.pend = nil, nil, nil, nil
	clear(r.frames)
	if r.raster != nil {
		r.raster.reset()
	}
	r.tracker.Reset()
	r.tracker.SetBase(r.base, false)
	r.counts.SignatureFailures++
	r.note(fmt.Sprintf("第 %d 步：模型錄製簽章不符，停用", steps))
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
		return r.lastSig + SigCheckSteps
	}
	return session.NoWake
}

func (r *ModelRecorder) Observe(v modelView) {
	// 先確認暫定命中。只存值與資料複本，不存上一個 StepView。
	if p := r.pend; p != nil {
		r.pend = nil
		if v.Deliveries() == p.del && (!r.materialSources || (v.Steps() > p.steps && p.sourceGeneration == r.sourceGeneration && (p.kind == 0x0102 || r.baseOK && p.base == r.base))) {
			r.apply(p)
		} else {
			r.counts.HitsVoidedByDelivery++
			if r.materialSources && p.sourceRole != nil {
				r.sourceCounts.Voided++
			}
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
		r.pend = &modelHit{kind: ip, steps: v.Steps(), del: v.Deliveries(), base: base, valid: valid, sourceGeneration: r.sourceGeneration}
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
	counted := modelReadCountingView{modelView: v}
	h := r.capture(&counted, ip)
	h.reads = counted.count
	return h
}

func (r *ModelRecorder) capture(v modelView, ip uint16) *modelHit {
	h := &modelHit{kind: ip, steps: v.Steps(), del: v.Deliveries(), ss: v.SS(), sp: v.SP(), bp: v.BP(), base: r.base, sourceGeneration: r.sourceGeneration}
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
		desc := modelWord(v, v.SS(), v.SP()+4, &ok)
		for i := range h.desc {
			h.desc[i] = modelWord(v, v.DS(), desc+uint16(2*i), &ok)
		}
		if h.desc[2] > h.desc[4] || h.desc[3] > h.desc[5] || h.desc[4] > 319 || h.desc[5] > 199 {
			ok = false
			break
		}
		h.list = make([]uint16, int(h.desc[5])+2)
		for i := range h.list {
			h.list[i] = modelWord(v, v.DS(), h.desc[1]+uint16(2*i), &ok)
		}
	case 0x0a51:
		o := &h.obj
		o.Steps = h.steps
		o.CamObj = modelWord(v, v.SS(), v.BP()+10, &ok)
		o.CamIdx = -1
		if r.open != nil {
			o.CamIdx = len(r.open.Cameras) - 1
		}
		o.Rec = modelFar(v, v.SS(), v.BP()+6, &ok)
		ok = modelRead(v, o.Rec.Seg, o.Rec.Off, o.RecRaw[:]) && ok
		o.RecVT = binary.LittleEndian.Uint16(o.RecRaw[:])
		o.Model = realspace.FarPtr{Off: binary.LittleEndian.Uint16(o.RecRaw[2:]), Seg: binary.LittleEndian.Uint16(o.RecRaw[4:])}
		o.LOD = o.RecRaw[9]
		ok = modelRead(v, o.Model.Seg, o.Model.Off, o.ModelHead[:]) && ok
		o.NFaces = int16(modelWord(v, v.SS(), v.BP()-0x30, &ok))
		list := modelFar(v, v.SS(), v.BP()-0x2e, &ok)
		o.FaceBase = modelFar(v, v.SS(), v.BP()-0x2a, &ok)
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
		buf := make([]byte, n*24)
		ok = modelRead(v, vert.Seg, vert.Off, buf) && ok
		o.VertXYZ = make([]byte, n*12)
		o.VertRT = make([][3]int32, n)
		for i := 0; i < n; i++ {
			copy(o.VertXYZ[i*12:], buf[i*24:i*24+12])
			for k := 0; k < 3; k++ {
				o.VertRT[i][k] = int32(binary.LittleEndian.Uint32(buf[i*24+k*4:]))
			}
		}
		o.VertSHA = sha256.Sum256(o.VertXYZ)
		o.FaceList = make([]uint16, int(o.NFaces))
		for i := range o.FaceList {
			o.FaceList[i] = modelWord(v, list.Seg, list.Off+uint16(i*2), &ok)
		}
		var flag [1]byte
		ok = modelRead(v, v.DS(), 0x17bc, flag[:]) && ok
		o.Flag17BC = flag[0]
		o.Near = modelDword(v, v.DS(), 0x1800, &ok)
		o.Far = modelDword(v, v.DS(), 0x51f0, &ok)
	case 0x0a7c:
		h.face.SI = v.SI()
		h.face.VIdx = [3]int16{-1, -1, -1}
		h.face.Face = modelFar(v, v.SS(), v.BP()-0x64, &ok)
		var raw [16]byte
		ok = modelRead(v, h.face.Face.Seg, h.face.Face.Off, raw[:]) && ok
		h.face.FaceVT = binary.LittleEndian.Uint16(raw[:])
		h.face.Method = modelFar(v, v.DS(), h.face.FaceVT+0x18, &ok)
		geom := realspace.FarPtr{Off: binary.LittleEndian.Uint16(raw[2:]), Seg: binary.LittleEndian.Uint16(raw[4:])}
		ok = modelRead(v, geom.Seg, geom.Off, h.face.Geom[:]) && ok
		projSeg := binary.LittleEndian.Uint16(raw[8:])
		h.proj = [3]realspace.FarPtr{{Off: binary.LittleEndian.Uint16(raw[6:]), Seg: projSeg}, {Off: binary.LittleEndian.Uint16(raw[10:]), Seg: projSeg}, {Off: binary.LittleEndian.Uint16(raw[14:]), Seg: projSeg}}
		if st, found := r.frames[modelFrameKey{h.ss, h.bp}]; found && r.open != nil {
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
		if st, found := r.frames[modelFrameKey{h.ss, h.bp}]; found && st.face >= 0 {
			for i, p := range st.proj {
				ok = modelRead(v, p.Seg, p.Off, h.face.Proj[i][:]) && ok
			}
		}
	case 0x0aab:
		h.elem = realspace.FarPtr{Off: v.AX(), Seg: v.DX()}
	case 0x0197:
		if r.open != nil && len(r.open.List) > 0 {
			h.buf = make([]byte, 64000)
			ok = modelRead(v, r.open.Desc[0], r.open.List[0], h.buf)
		} else {
			ok = false
		}
	}
	h.readFailed = !ok
	h.obj.ReadFailed = !ok
	if r.materialSources && (ip == 0x0a7c || ip == 0x0a7f || ip == 0x0aa5 || ip == 0x0aab) {
		role := r.captureMaterialRole(v)
		h.sourceRole = &role
		if ip == 0x0a7c {
			o1 := uint64(0)
			if st, found := r.frames[modelFrameKey{h.ss, h.bp}]; found && r.open != nil {
				o1 = r.open.Objects[st.obj].Steps
			}
			h.face.Source = &realspace.FaceSourceObs{Generation: r.sourceGeneration, O1Steps: o1, F1Call: role, Proof: r.sourceProof}
		}
	}
	return h
}

func (r *ModelRecorder) apply(h *modelHit) {
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
			r.note(fmt.Sprintf("第 %d 步：R7 取代第 %d 步尚未封存的模型群組", h.steps, r.open.Steps))
		}
		r.open = nil
		clear(r.frames)
		if r.raster != nil {
			r.raster.reset()
		}
		if !r.vga {
			r.open = &ModelGroup{Steps: h.steps}
			r.counts.GroupsOpened++
		}
		return
	}
	if r.open == nil {
		r.counts.OutsideGroup++
		return
	}
	g := r.open
	if h.reads.Calls != 0 {
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
		g.Cameras = append(g.Cameras, h.cam)
		g.CameraReadFailed = append(g.CameraReadFailed, h.readFailed)
	case 0x01ed:
		if g.List == nil {
			g.Desc = h.desc
			g.List = h.list
		}
	case 0x0a51:
		if h.obj.CamIdx >= 0 && h.obj.CamIdx < len(g.CameraReadFailed) {
			h.obj.ReadFailed = h.obj.ReadFailed || g.CameraReadFailed[h.obj.CamIdx]
		}
		g.Objects = append(g.Objects, h.obj)
		r.frames[key] = modelFrameState{obj: len(g.Objects) - 1, face: -1}
	case 0x0a7c:
		st, found := r.frames[key]
		if !found {
			r.counts.UnmatchedFrame++
			g.UnmatchedFrames++
			return
		}
		o := &g.Objects[st.obj]
		o.ReadFailed = o.ReadFailed || h.readFailed
		o.Faces = append(o.Faces, h.face)
		if h.face.Source != nil {
			r.sourceCounts.F1Call++
		}
		st.face = len(o.Faces) - 1
		st.ss, st.sp, st.proj = h.ss, h.sp, h.proj
		r.frames[key] = st
	case 0x0a7f:
		st, found := r.frames[key]
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
		if st, found := r.frames[key]; found && st.face >= 0 {
			f := &g.Objects[st.obj].Faces[st.face]
			if f.Source != nil && h.sourceRole != nil {
				f.Source.BuildCall = *h.sourceRole
				r.sourceCounts.BuildCall++
			}
		}
	case 0x0aab:
		st, found := r.frames[key]
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
		r.shown = r.await
		r.await = nil
		if r.shown != nil {
			r.shown.Key = key
			r.counts.Promotions++
			if r.OnGroup != nil {
				r.OnGroup(r.shown)
			}
		}
	}
}

// VideoModeChange 清除三個群組槽與暫定命中，保留仍需重新辨識簽章的基底資料。
// 不寫原版狀態；條件來自 012 §3.7.2。
func (r *ModelRecorder) VideoModeChange() {
	if r.materialSources {
		r.sourceGeneration++
		r.sourceProof = nil
	}
	r.open, r.await, r.shown, r.pend = nil, nil, nil, nil
	clear(r.frames)
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
	q := realspace.FaceSourceRole{Steps: v.Steps(), Deliveries: v.Deliveries(), Generation: r.sourceGeneration, SS: v.SS(), SP: v.SP(), BP: v.BP(), SI: v.SI()}
	ok := true
	q.Face = modelFar(v, v.SS(), v.BP()-0x64, &ok)
	if v.IP() == 0x0a7c || v.IP() == 0x0aa5 {
		q.HasFaceArgument = true
		q.FaceArgument = modelFar(v, v.SS(), v.SP(), &ok)
	}
	if st, found := r.frames[modelFrameKey{q.SS, q.BP}]; found && r.open != nil && st.obj >= 0 && st.obj < len(r.open.Objects) {
		q.Rec = r.open.Objects[st.obj].Rec
	} else {
		ok = false
	}
	ok = modelRead(v, q.Face.Seg, q.Face.Off, q.Raw[:]) && ok
	vt := binary.LittleEndian.Uint16(q.Raw[:])
	q.MethodF1 = modelFar(v, v.DS(), vt+0x18, &ok)
	q.MethodBuild = modelFar(v, v.DS(), vt+0x1c, &ok)
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
