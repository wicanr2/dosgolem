package strikecommander

import (
	"bytes"
	"encoding/binary"
	"testing"

	"github.com/wicanr2/dosgolem/apps/strikecommander/hd3d"
	"github.com/wicanr2/dosgolem/apps/strikecommander/realspace"
)

func rasterTestVertex(u, v uint16) []byte {
	b := make([]byte, 22)
	binary.LittleEndian.PutUint16(b[6:], u<<8)
	binary.LittleEndian.PutUint16(b[8:], v<<8)
	return b
}

func TestRasterTextureRectangleWrap(t *testing.T) {
	v := newModelTestView()
	verts := append(rasterTestVertex(1, 1), rasterTestVertex(3, 2)...)
	v.put(0x2000, 0xffff, []byte{11, 12, 13})
	v.put(0x2000, 2, []byte{21, 22, 23})
	c, ok := rasterCaptureTexture(v, 0x2000, 0xfffb, 3, verts, 0, true)
	if !ok || len(c.Spans) != 2 || c.Spans[0].Off != 0xffff || c.Spans[1].Off != 2 || !bytes.Equal(c.Spans[0].Data, []byte{11, 12, 13}) || !bytes.Equal(c.Spans[1].Data, []byte{21, 22, 23}) {
		t.Fatalf("rectangle with segment wrap: %+v, %t", c, ok)
	}
	// ±2 是 R5 的擷取邊界；負數仍經 16-bit offset 環繞。
	c, ok = rasterCaptureTexture(v, 0x2000, 0xfffb, 3, verts, 2, true)
	if !ok || len(c.Spans) != 6 || c.Spans[0].Off != 0xfff7 || len(c.Spans[0].Data) != 7 {
		t.Fatalf("padded rectangle: %+v, %t", c, ok)
	}
	v.fail = lin(0x2000, 0xffff)
	if _, ok := rasterCaptureTexture(v, 0x2000, 0xfffb, 3, verts, 0, true); ok {
		t.Fatal("texture read failure was accepted")
	}
}

func TestRasterSnapshotInterningIsPerGroup(t *testing.T) {
	r := &rasterRecorder{}
	g := &ModelGroup{}
	for _, b := range [][]byte{{1, 2}, {1, 2}, {1, 3}} {
		r.append(g, hd3d.Call{Table: b, Tex: hd3d.TexRef{Pixels: hd3d.Capture{Spans: []hd3d.Span{{Off: 0xfffe, Data: append([]byte(nil), b...)}}}}})
	}
	if &g.Calls[0].Table[0] != &g.Calls[1].Table[0] || &g.Calls[1].Table[0] == &g.Calls[2].Table[0] || !bytes.Equal(g.Calls[2].Table, []byte{1, 3}) {
		t.Fatal("snapshots do not share only equal immutable bytes")
	}
	if &g.Calls[0].Table[0] != &g.Calls[0].Tex.Pixels.Spans[0].Data[0] {
		t.Fatal("cross-field equal bytes were not deduplicated")
	}
	r.reset()
	if len(r.blobs) != 0 || !bytes.Equal(g.Calls[0].Table, []byte{1, 2}) {
		t.Fatal("group reset leaked cache or changed sealed data")
	}
}

func TestRasterRawElementAndDeliveryVoid(t *testing.T) {
	r := NewSceneRecorder(nil, hd3d.Tables{})
	r.base, r.baseOK, r.lastSig = 0x241, true, 1
	r.open = &ModelGroup{}
	v := newModelTestView()
	v.cs, v.ip, v.steps = r.base+0x27f5, 0x0048, 10
	v.far(v.ds, v.bx, realspace.FarPtr{Off: 0x00af, Seg: r.base + 0x314a})
	v.far(v.ss, v.sp, realspace.FarPtr{Off: 0x1234, Seg: 0x7000})
	v.put(0x7000, 0x1234, []byte{0xaa, 0xbb})
	r.Observe(v)
	if len(r.open.Elements) != 0 || len(r.raster.stack) != 0 || len(r.raster.blobs) != 0 {
		t.Fatal("tentative R10 changed recorder state")
	}
	v.steps, v.ip, v.deliveries = 11, 0, 1
	r.Observe(v)
	if len(r.open.Elements) != 0 || len(r.raster.stack) != 0 || r.counts.HitsVoidedByDelivery != 1 {
		t.Fatal("IRQ did not void tentative R10")
	}
	v.steps, v.ip = 12, 0x0048
	r.Observe(v)
	v.steps, v.ip = 13, 0
	r.Observe(v)
	if len(r.open.Elements) != 1 || r.open.Elements[0].Header[0] != 0xaa || r.open.Elements[0].Header[1] != 0xbb || r.open.Elements[0].Pointer.Off != 0x1234 || len(r.raster.stack) != 1 {
		t.Fatalf("raw R10 snapshot missing: %+v", r.open.Elements)
	}
}

func TestRasterRequiredGroupReadFailure(t *testing.T) {
	r := &rasterRecorder{}
	g := &ModelGroup{ReadFailed: true, Buf3D: make([]byte, 64000)}
	r.check(g)
	found := false
	for _, f := range g.Raster.Diagnostics.Faults {
		found = found || f == "群組必要輸入讀取失敗"
	}
	if !found || g.ReplayReady || g.Raster.Valid || g.Raster.Repaintable {
		t.Fatal("failed Buf3D read did not fail closed")
	}
}
