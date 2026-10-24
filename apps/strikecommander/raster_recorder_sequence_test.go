package strikecommander

import (
	"bytes"
	"encoding/binary"
	"testing"

	"github.com/wicanr2/dosgolem/apps/strikecommander/hd3d"
	"github.com/wicanr2/dosgolem/apps/strikecommander/realspace"
)

// 期望影像由平色背景、單列 R1 與單點 R4 直接建立，沒有以 Replay 製作 oracle。
func rasterSequenceFixture() (*ModelRecorder, *modelTestView, []byte) {
	tab := bytes.Repeat([]byte{9}, 200)
	for i, off := range []uint16{8, 56, 104, 152} {
		binary.LittleEndian.PutUint16(tab[0xc0+2*i:], off)
	}
	r := NewSceneRecorder(nil, hd3d.Tables{R6Table: tab})
	r.base, r.baseOK, r.lastSig = 0x241, true, 1
	r.tracker.SetBase(r.base, true)
	v := newModelTestView()
	v.steps, v.bx = 10, 0x200
	g := &ModelGroup{Desc: [6]uint16{0xffff, 0xe654, 0, 0, 319, 199}, List: make([]uint16, 201)}
	for i := range g.List {
		g.List[i] = uint16(16 + 320*i)
	}
	r.open = g
	image := bytes.Repeat([]byte{9}, hd3d.PixelCount)
	v.put(g.Desc[0], g.List[0], image)
	for i, word := range g.Desc {
		v.word(v.ds, 0x400+uint16(2*i), word)
	}
	r.raster.append(g, hd3d.Call{
		Kind: hd3d.R6, Desc: g.Desc, DS: v.ds, SS: v.ds,
		Params: [13]uint16{0, 43, 32, 160, 32},
		Aux:    hd3d.AuxRef{Color0: 9, Color1: 9, HasFlags: true},
	})
	return r, v, image
}

func rasterSequenceHit(r *ModelRecorder, v *modelTestView, cs, ip uint16) uint64 {
	v.steps++
	v.cs, v.ip = cs, ip
	step := v.steps
	r.Observe(v)
	v.steps++
	v.ip = 0
	r.Observe(v)
	return step
}

func rasterSequenceEntry(r *ModelRecorder, v *modelTestView, sp uint16, element uint16) {
	v.sp = sp
	v.far(v.ds, v.bx, realspace.FarPtr{Off: 0x1d0, Seg: r.base + 0x1b8d})
	v.far(v.ss, v.sp, realspace.FarPtr{Off: element, Seg: 0x7000})
	rasterSequenceHit(r, v, r.base+0x27f5, 0x0048)
}

func rasterSequenceRow(r *ModelRecorder, v *modelTestView, textured bool) {
	v.sp = 0x800
	v.word(v.ss, v.sp+2, 0x1000) // 不屬於 R1–R4 同段重入。
	params := []uint16{0x400, 3, 0x1000, 0x6200}
	xs, color, ip := [3]uint16{10, 14, 12}, uint16(7), uint16(0x000c)
	if textured {
		xs, color, ip = [3]uint16{30, 30, 30}, 0, 0x0ea8
		params = append(params, 0x300, 0x6300, 1, 0, 0)
		v.put(0x6300, 0x300, []byte{11})
	}
	verts := make([]byte, 66)
	for i, x := range xs {
		binary.LittleEndian.PutUint16(verts[22*i:], x)
		binary.LittleEndian.PutUint16(verts[22*i+2:], 20)
		binary.LittleEndian.PutUint16(verts[22*i+4:], color)
	}
	v.put(0x6200, 0x1000, verts)
	for i, word := range params {
		v.word(v.ss, v.sp+4+uint16(2*i), word)
	}
	rasterSequenceHit(r, v, r.base+0x1d5e, ip)
}

func rasterSequenceBuffer(r *ModelRecorder, v *modelTestView, image []byte) {
	v.put(r.open.Desc[0], r.open.List[0], image)
}

func rasterSequencePatch(t *testing.T, call hd3d.Call, want ...hd3d.PixelSet) {
	t.Helper()
	if call.Kind != hd3d.BlackBox || len(call.Patch) != len(want) {
		t.Fatalf("補丁種類／點數：%s %+v，期望 %+v", call.Kind, call.Patch, want)
	}
	for i := range want {
		if call.Patch[i] != want[i] {
			t.Fatalf("補丁 %d：%+v，期望 %+v", i, call.Patch[i], want[i])
		}
	}
}

func TestRasterSequenceUnknownSubtractsR1AndR4(t *testing.T) {
	r, v, image := rasterSequenceFixture()
	rasterSequenceEntry(r, v, 0x900, 0x1234)
	rasterSequenceRow(r, v, false)
	for x := 10; x <= 14; x++ {
		image[20*320+x] = 7 // R1 的含右端單列。
	}
	rasterSequenceRow(r, v, true)
	image[20*320+30] = 11 // R4 的單點特例。
	image[20*320+10], image[20*320+50] = 123, 88
	rasterSequenceBuffer(r, v, image)
	v.sp = 0x900
	step := rasterSequenceHit(r, v, r.base+0x27f5, 0x004a)
	g := r.open
	if len(g.Calls) != 4 || len(r.raster.stack) != 0 || len(g.RasterProblems) != 0 {
		t.Fatalf("呼叫／框：%d %d %v", len(g.Calls), len(r.raster.stack), g.RasterProblems)
	}
	call := g.Calls[3]
	rasterSequencePatch(t, call, hd3d.PixelSet{Index: 20*320 + 10, Value: 123}, hd3d.PixelSet{Index: 20*320 + 50, Value: 88})
	if call.Steps != step || call.DS != v.ds || call.SS != v.ss || call.SP != 0x900 || call.Element != (hd3d.FarPtr{Off: 0x1234, Seg: 0x7000}) {
		t.Fatalf("黑箱返回收據欄位：%+v", call)
	}
	g.Buf3D = append([]byte(nil), image...)
	r.raster.check(g)
	if !g.ReplayReady || !bytes.Equal(g.Raster.Pixels[:], image) {
		t.Fatalf("獨立影像比對失敗：%+v", g.Raster.Diagnostics)
	}
}

func TestRasterSequenceTwoUnknownFrames(t *testing.T) {
	r, v, image := rasterSequenceFixture()
	rasterSequenceEntry(r, v, 0x900, 0x1111)
	rasterSequenceEntry(r, v, 0x880, 0x2222)
	rasterSequenceRow(r, v, false)
	for x := 10; x <= 14; x++ {
		image[20*320+x] = 7
	}
	image[20*320+10] = 123
	rasterSequenceBuffer(r, v, image)
	v.sp = 0x880
	rasterSequenceHit(r, v, r.base+0x27f5, 0x004a)
	if len(r.raster.stack) != 1 || len(r.open.Calls) != 3 {
		t.Fatalf("內層返回破壞外層：%d 框、%d 呼叫", len(r.raster.stack), len(r.open.Calls))
	}
	rasterSequencePatch(t, r.open.Calls[2], hd3d.PixelSet{Index: 20*320 + 10, Value: 123})
	image[20*320+50] = 88
	rasterSequenceBuffer(r, v, image)
	v.sp = 0x900
	rasterSequenceHit(r, v, r.base+0x27f5, 0x004a)
	if len(r.raster.stack) != 0 || len(r.open.Calls) != 4 || len(r.open.RasterProblems) != 0 {
		t.Fatalf("外層返回：%d 框、%d 呼叫、%v", len(r.raster.stack), len(r.open.Calls), r.open.RasterProblems)
	}
	rasterSequencePatch(t, r.open.Calls[3], hd3d.PixelSet{Index: 20*320 + 50, Value: 88})
	if r.open.Calls[2].Element.Off != 0x2222 || r.open.Calls[3].Element.Off != 0x1111 {
		t.Fatal("內外補丁的元素歸屬互換")
	}
}

func TestRasterSequenceReturnIRQRetry(t *testing.T) {
	r, v, image := rasterSequenceFixture()
	rasterSequenceEntry(r, v, 0x900, 0x1234)
	image[42] = 88
	rasterSequenceBuffer(r, v, image)
	v.steps++
	v.cs, v.ip = r.base+0x27f5, 0x004a
	r.Observe(v)
	v.steps++
	v.ip, v.deliveries = 0, 1
	r.Observe(v)
	if len(r.raster.stack) != 1 || len(r.open.Calls) != 1 || r.counts.HitsVoidedByDelivery != 1 {
		t.Fatal("IRQ 作廢的返回已關框或追加補丁")
	}
	image[42] = 98 // 重試要重新擷取，不得沿用被作廢的 After。
	rasterSequenceBuffer(r, v, image)
	rasterSequenceHit(r, v, r.base+0x27f5, 0x004a)
	if len(r.raster.stack) != 0 || len(r.open.Calls) != 2 || len(r.open.RasterProblems) != 0 {
		t.Fatalf("返回重試：%d 框、%d 呼叫、%v", len(r.raster.stack), len(r.open.Calls), r.open.RasterProblems)
	}
	rasterSequencePatch(t, r.open.Calls[1], hd3d.PixelSet{Index: 42, Value: 98})
}

func TestRasterSequenceModeClearsPendingAndFrames(t *testing.T) {
	r, v, image := rasterSequenceFixture()
	rasterSequenceEntry(r, v, 0x900, 0x1234)
	image[42] = 88
	rasterSequenceBuffer(r, v, image)
	v.steps++
	v.cs, v.ip = r.base+0x27f5, 0x004a
	r.Observe(v)
	r.await, r.shown = &ModelGroup{Key: 1}, &ModelGroup{Key: 2}
	r.frames[modelFrameKey{ss: v.ss, bp: v.bp}] = modelFrameState{obj: 0}
	r.tracker.cuIn, r.tracker.cuAwait = true, true
	r.raster.intern([]byte{1, 2})
	if r.pend == nil || len(r.raster.stack) != 1 {
		t.Fatal("沒有建立模式切換的前提")
	}
	r.VideoModeChange()
	if r.open != nil || r.await != nil || r.shown != nil || r.pend != nil || len(r.frames) != 0 || len(r.raster.stack) != 0 || len(r.raster.blobs) != 0 || r.tracker.Entry().InProgress || r.tracker.Entry().Awaiting {
		t.Fatal("模式切換留下群組、暫定命中或框")
	}
	v.steps++
	v.ip = 0
	r.Observe(v)
	r.Frame(v.steps)
	if r.Visible() != nil || len(r.raster.stack) != 0 {
		t.Fatal("舊返回在模式切換後復活")
	}
}

func TestRasterSequenceRequiredCaptureFailures(t *testing.T) {
	for _, where := range []string{"before", "after", "texture"} {
		t.Run(where, func(t *testing.T) {
			r, v, image := rasterSequenceFixture()
			if where == "before" {
				v.fail = lin(r.open.Desc[0], r.open.List[0])
			}
			rasterSequenceEntry(r, v, 0x900, 0x1234)
			v.fail = 0
			if where == "texture" {
				v.fail = lin(0x6300, 0x300)
				rasterSequenceRow(r, v, true)
				if !r.open.Calls[1].ReadFailed {
					t.Fatal("貼圖讀取失敗沒有附在呼叫上")
				}
				v.fail = 0
			}
			if where == "after" {
				v.fail = lin(r.open.Desc[0], r.open.List[0])
			}
			v.sp = 0x900
			rasterSequenceHit(r, v, r.base+0x27f5, 0x004a)
			if len(r.open.RasterProblems) == 0 {
				t.Fatal("必要擷取失敗未記錄")
			}
			r.open.Buf3D = image
			r.raster.check(r.open)
			if r.open.ReplayReady || r.open.Raster.Valid {
				t.Fatal("必要擷取失敗的群組仍可重畫")
			}
		})
	}
}
