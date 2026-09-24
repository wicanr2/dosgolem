package xlate

import (
	"bytes"
	"math"
	"reflect"
	"testing"
)

func physicalTestFont() *Font {
	g := make([]byte, 16*2)
	g[0] = 0x80
	return &Font{W: 16, H: 16, Name: "physical.test", Glyphs: map[rune][]byte{'A': g}}
}

func TestPixelGlyphCheckedSnapshotAtomicRestore(t *testing.T) {
	f := physicalTestFont()
	s := &Stamp{X: 0, Y: 0, Cells: 2, CellW: 8, CellH: 8, PixelScale: 3, State: Shown, FG: [3]uint8{1, 2, 3}, BG: [3]uint8{4, 5, 6}, PixelGlyphs: []PixelGlyph{{Rune: 'A', Font: f, SrcW: 14, SrcH: 16, X: 0, Y: 0}}}
	l := &Layer{W: 16, H: 16, Stamps: []*Stamp{s}, FontRegistry: map[string]*Font{f.Name: f}}
	dst := make([]byte, 48*48*4)
	if drew, err := l.DrawChecked(dst, 3, nil); err != nil || !drew {
		t.Fatalf("checked draw=%v err=%v", drew, err)
	}
	if got := l.Draw(make([]byte, len(dst)), 3, nil); got {
		t.Fatal("legacy Draw must reject physical plan")
	}
	snap, err := l.Snapshot()
	if err != nil {
		t.Fatal(err)
	}
	var restored Layer
	if err := restored.Restore(snap, map[string]*Font{f.Name: f}); err != nil {
		t.Fatal(err)
	}
	got := make([]byte, len(dst))
	if _, err := restored.DrawChecked(got, 3, nil); err != nil || !bytes.Equal(dst, got) {
		t.Fatalf("round trip err=%v equal=%v", err, bytes.Equal(dst, got))
	}
	before := restored.Stamps
	if err := restored.Restore(bytes.Replace(snap, []byte("physical.test"), []byte("wrong.font___"), 1), map[string]*Font{f.Name: f}); err == nil || restored.Stamps[0] != before[0] {
		t.Fatal("failed restore must be atomic")
	}
}

func TestPixelGlyphPreflightWritesNothing(t *testing.T) {
	f := physicalTestFont()
	l := &Layer{W: 16, H: 16, FontRegistry: map[string]*Font{f.Name: f}, Stamps: []*Stamp{{X: 0, Y: 0, Cells: 1, CellW: 8, CellH: 8, PixelScale: 3, State: Shown, PixelGlyphs: []PixelGlyph{{Rune: 'A', Font: f, SrcW: 17, SrcH: 16}}}}}
	dst := bytes.Repeat([]byte{7}, 48*48*4)
	before := append([]byte(nil), dst...)
	if _, err := l.DrawChecked(dst, 3, nil); err == nil || !bytes.Equal(dst, before) {
		t.Fatal("bad plan must fail before writes")
	}
}

func TestPixelGlyphPreflightRejectsFontOverflowAndParentOutsideCanvas(t *testing.T) {
	for _, tc := range []struct {
		name  string
		layer *Layer
	}{
		{"font_overflow", func() *Layer {
			f := &Font{W: math.MaxInt, H: 2, Name: "huge", Glyphs: map[rune][]byte{'A': nil}}
			return &Layer{W: 16, H: 16, FontRegistry: map[string]*Font{f.Name: f}, Stamps: []*Stamp{{X: 0, Y: 0, Cells: 1, CellW: 8, CellH: 8, PixelScale: 3, State: Shown, PixelGlyphs: []PixelGlyph{{Rune: 'A', Font: f, SrcW: 1, SrcH: 1}}}}}
		}()},
		{"parent_outside", func() *Layer {
			f := physicalTestFont()
			return &Layer{W: 16, H: 16, FontRegistry: map[string]*Font{f.Name: f}, Stamps: []*Stamp{{X: 15, Y: 0, Cells: 1, CellW: 8, CellH: 8, PixelScale: 3, State: Shown, PixelGlyphs: []PixelGlyph{{Rune: 'A', Font: f, SrcW: 1, SrcH: 1}}}}}
		}()},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dst := bytes.Repeat([]byte{9}, 48*48*4)
			before := append([]byte(nil), dst...)
			if _, err := tc.layer.DrawChecked(dst, 3, nil); err == nil || !bytes.Equal(dst, before) {
				t.Fatalf("must fail closed before writes: %v", err)
			}
		})
	}
}

func TestPixelGlyphWholeLayerAndTransparentCellFailClosed(t *testing.T) {
	f := physicalTestFont()
	valid := &Stamp{X: 0, Y: 0, Cells: 2, CellW: 8, CellH: 8, PixelScale: 3, State: Shown, PixelGlyphs: []PixelGlyph{{Rune: 'A', Font: f, SrcW: 1, SrcH: 1}}}
	bad := &Stamp{X: 0, Y: 8, Cells: 1, CellW: 8, CellH: 8, PixelScale: 3, State: Shown, PixelGlyphs: []PixelGlyph{{Rune: 'A', Font: f, SrcW: 99, SrcH: 1}}}
	l := &Layer{W: 16, H: 16, FontRegistry: map[string]*Font{f.Name: f}, Stamps: []*Stamp{valid, bad}}
	dst := bytes.Repeat([]byte{5}, 48*48*4)
	before := append([]byte(nil), dst...)
	if _, err := l.DrawChecked(dst, 3, nil); err == nil || !bytes.Equal(dst, before) {
		t.Fatal("later invalid stamp must prevent all writes")
	}
	valid.Transparent = []bool{true, false}
	l.Stamps = l.Stamps[:1]
	if _, err := l.DrawChecked(dst, 3, nil); err != nil {
		t.Fatal(err)
	}
	if dst[0] != 5 {
		t.Fatal("transparent logical cell received physical ink")
	}
}

func TestPixelGlyphRestoreMultiStampIsAtomicAndRejectsSameNameDifferentBytes(t *testing.T) {
	f := physicalTestFont()
	makeStamp := func(y int) *Stamp {
		return &Stamp{X: 0, Y: y, Cells: 1, CellW: 8, CellH: 8, PixelScale: 3, State: Shown, PixelGlyphs: []PixelGlyph{{Rune: 'A', Font: f, SrcW: 1, SrcH: 1, X: 0, Y: y * 3}}}
	}
	source := &Layer{W: 16, H: 16, FontRegistry: map[string]*Font{f.Name: f}, Stamps: []*Stamp{makeStamp(0), makeStamp(8)}}
	snap, err := source.Snapshot()
	if err != nil {
		t.Fatal(err)
	}
	oldFont := physicalTestFont()
	oldFont.Name = "old"
	oldStamp := &Stamp{Key: "old"}
	hook := func(*Stamp, string) {}
	target := &Layer{W: 7, H: 9, Stamps: []*Stamp{oldStamp}, FontRegistry: map[string]*Font{oldFont.Name: oldFont}, OnDrop: hook}
	assertUnchanged := func() {
		if target.W != 7 || target.H != 9 || len(target.Stamps) != 1 || target.Stamps[0] != oldStamp || target.FontRegistry[oldFont.Name] != oldFont || reflect.ValueOf(target.OnDrop).Pointer() != reflect.ValueOf(hook).Pointer() {
			t.Fatal("failed restore changed layer")
		}
	}
	needle := []byte("\"SrcW\":1")
	off := bytes.LastIndex(snap, needle)
	if off < 0 {
		t.Fatal("snapshot lacks second crop")
	}
	badCrop := append(append([]byte(nil), snap[:off]...), append([]byte("\"SrcW\":99"), snap[off+len(needle):]...)...)
	if err := target.Restore(badCrop, map[string]*Font{f.Name: f}); err == nil {
		t.Fatal("bad second crop accepted")
	}
	assertUnchanged()
	other := physicalTestFont()
	other.Glyphs['A'][0] = 0x40
	if err := target.Restore(snap, map[string]*Font{f.Name: other}); err == nil {
		t.Fatal("same-name different bitmap accepted")
	}
	assertUnchanged()
}

func TestPixelGlyphScaleMismatchFailsBeforeWrites(t *testing.T) {
	f := physicalTestFont()
	l := &Layer{W: 16, H: 16, FontRegistry: map[string]*Font{f.Name: f}, Stamps: []*Stamp{{X: 0, Y: 0, Cells: 1, CellW: 8, CellH: 8, PixelScale: 3, State: Shown, PixelGlyphs: []PixelGlyph{{Rune: 'A', Font: f, SrcW: 1, SrcH: 1}}}}}
	for _, scale := range []int{0, -1, 2} {
		dst := bytes.Repeat([]byte{3}, 48*48*4)
		before := append([]byte(nil), dst...)
		if _, err := l.DrawChecked(dst, scale, nil); err == nil || !bytes.Equal(dst, before) {
			t.Fatalf("scale %d wrote or passed", scale)
		}
	}
}

// 此 JSON 是 spec 202 的舊格式；新 optional 欄位不得改動既有位元組。
func TestPixelGlyphLegacySnapshotAndTwoTimesBytes(t *testing.T) {
	s := &Stamp{Key: "legacy", X: 0, Y: 0, Cells: 1, CellW: 8, CellH: 8, State: Shown, BG: [3]uint8{4, 5, 6}}
	l := &Layer{W: 8, H: 8, Stamps: []*Stamp{s}}
	snap, err := l.Snapshot()
	if err != nil {
		t.Fatal(err)
	}
	const oldJSON = `{"w":8,"h":8,"stamps":[{"key":"legacy","x":0,"y":0,"cells":1,"cell_w":8,"cell_h":8,"glyph_x":0,"glyph_y":0,"glyph_scale":0,"text":"","state":2,"fg":[0,0,0],"bg":[4,5,6],"hashes":null,"misses":null}]}`
	if string(snap) != oldJSON {
		t.Fatalf("legacy Snapshot bytes changed: %s", snap)
	}
	want := bytes.Repeat([]byte{4, 5, 6, 0xff}, 16*16)
	legacy := make([]byte, len(want))
	checked := make([]byte, len(want))
	if !l.Draw(legacy, 2, nil) || !bytes.Equal(legacy, want) {
		t.Fatal("legacy 2× RGBA bytes changed")
	}
	if drew, err := l.DrawChecked(checked, 2, nil); err != nil || !drew || !bytes.Equal(checked, want) {
		t.Fatalf("checked legacy 2× bytes changed: drew=%v err=%v", drew, err)
	}
}

func TestPixelGlyphLegacyDrawRejectsWithoutWrites(t *testing.T) {
	f := physicalTestFont()
	l := &Layer{W: 16, H: 16, FontRegistry: map[string]*Font{f.Name: f}, Stamps: []*Stamp{{X: 0, Y: 0, Cells: 1, CellW: 8, CellH: 8, PixelScale: 3, State: Shown, PixelGlyphs: []PixelGlyph{{Rune: 'A', Font: f, SrcW: 1, SrcH: 1}}}}}
	dst := bytes.Repeat([]byte{0x5a}, 48*48*4)
	before := append([]byte(nil), dst...)
	if l.Draw(dst, 3, nil) || !bytes.Equal(dst, before) {
		t.Fatal("legacy Draw must reject physical plan without writing")
	}
}

func TestPixelGlyphClearAndChangedFrameDoNotLeaveGhostInk(t *testing.T) {
	f := &Font{W: 1, H: 1, Name: "physical.lifecycle", Glyphs: map[rune][]byte{'A': {0x80}}}
	s := &Stamp{
		X: 0, Y: 0, Cells: 2, CellW: 1, CellH: 1, PixelScale: 3, State: Pending,
		PixelGlyphs: []PixelGlyph{
			{Rune: 'A', Font: f, SrcW: 1, SrcH: 1, X: 1, Y: 0},
			{Rune: 'A', Font: f, SrcW: 1, SrcH: 1, X: 4, Y: 0},
		},
	}
	l := &Layer{W: 2, H: 1, FontRegistry: map[string]*Font{f.Name: f}, Stamps: []*Stamp{s}}
	rgb := []byte{0, 0, 0, 255, 255, 255}
	l.Frame([]byte{0, 1}, rgb)
	if s.State != Shown {
		t.Fatal("physical stamp did not enter shown state")
	}
	dst := bytes.Repeat([]byte{0x5a}, 6*3*4)
	if drew, err := l.DrawChecked(dst, 3, nil); err != nil || !drew || dst[4*(0*6+1)] == 0x5a || dst[4*(0*6+4)] == 0x5a {
		t.Fatalf("initial physical glyphs missing: drew=%v err=%v", drew, err)
	}
	l.Clear(1, 0, 2, 1)
	if len(l.Stamps) != 1 || !s.transparent(1) || s.State != Pending {
		t.Fatal("partial clear did not invalidate the second logical cell")
	}
	l.Frame([]byte{0, 1}, rgb)
	dst = bytes.Repeat([]byte{0x5a}, len(dst))
	if drew, err := l.DrawChecked(dst, 3, nil); err != nil || !drew || dst[4*(0*6+1)] == 0x5a || dst[4*(0*6+4)] != 0x5a {
		t.Fatalf("partially cleared glyph left ink: drew=%v err=%v", drew, err)
	}
	for i := 0; i < 3; i++ {
		l.Frame([]byte{1, 1}, rgb)
	}
	if len(l.Stamps) != 0 {
		t.Fatal("changed source cell left a physical stamp behind")
	}
	dst = bytes.Repeat([]byte{0x5a}, len(dst))
	if drew, err := l.DrawChecked(dst, 3, nil); err != nil || drew || !bytes.Equal(dst, bytes.Repeat([]byte{0x5a}, len(dst))) {
		t.Fatalf("dropped physical stamp left ghost ink: drew=%v err=%v", drew, err)
	}
}

func TestPixelGlyphPartialAddMasksOnlyCoveredCell(t *testing.T) {
	f := &Font{W: 1, H: 1, Name: "physical.add", Glyphs: map[rune][]byte{'A': {0x80}}}
	old := &Stamp{Key: "old", X: 0, Y: 0, Cells: 3, CellW: 1, CellH: 1, PixelScale: 3, State: Shown,
		PixelGlyphs: []PixelGlyph{
			{Rune: 'A', Font: f, SrcW: 1, SrcH: 1, X: 1, Y: 0},
			{Rune: 'A', Font: f, SrcW: 1, SrcH: 1, X: 4, Y: 0},
			{Rune: 'A', Font: f, SrcW: 1, SrcH: 1, X: 7, Y: 0},
		}}
	l := &Layer{W: 3, H: 1, FontRegistry: map[string]*Font{f.Name: f}}
	l.Add(old)
	cover := &Stamp{Key: "cover", X: 1, Y: 0, Cells: 1, CellW: 1, CellH: 1, State: Pending}
	l.Add(cover)
	if len(l.Stamps) != 2 || !old.transparent(1) || old.transparent(0) || old.transparent(2) || old.State != Pending {
		t.Fatalf("partial Add did not mask only the middle physical glyph: stamps=%d transparent=%v state=%v", len(l.Stamps), old.Transparent, old.State)
	}
	l.Frame([]byte{0, 0, 0}, make([]byte, 9))
	if old.State != Shown || cover.State != Shown {
		t.Fatalf("remaining stamps were not recolored: old=%v cover=%v", old.State, cover.State)
	}
	old.BG, old.FG = [3]uint8{10, 20, 30}, [3]uint8{200, 210, 220}
	cover.BG = [3]uint8{40, 50, 60}
	dst := make([]byte, 9*3*4)
	if drew, err := l.DrawChecked(dst, 3, nil); err != nil || !drew {
		t.Fatalf("physical Add draw failed: drew=%v err=%v", drew, err)
	}
	pixel := func(x int) []byte { return dst[4*x : 4*x+4] }
	if !bytes.Equal(pixel(1), []byte{200, 210, 220, 255}) || !bytes.Equal(pixel(7), []byte{200, 210, 220, 255}) || !bytes.Equal(pixel(4), []byte{40, 50, 60, 255}) {
		t.Fatalf("partial Add changed surviving ink or revived covered ink: left=%v middle=%v right=%v", pixel(1), pixel(4), pixel(7))
	}
}

func TestPixelGlyphAllAnchorsGoneDropsWholeStamp(t *testing.T) {
	f := &Font{W: 1, H: 1, Name: "physical.anchors", Glyphs: map[rune][]byte{'A': {0x80}}}
	var dropped []string
	s := &Stamp{Key: "anchored", X: 0, Y: 0, Cells: 3, CellW: 2, CellH: 1, PixelScale: 3, State: Pending,
		PixelGlyphs: []PixelGlyph{
			{Rune: 'A', Font: f, SrcW: 1, SrcH: 1, X: 1, Y: 0},
			{Rune: 'A', Font: f, SrcW: 1, SrcH: 1, X: 7, Y: 0},
			{Rune: 'A', Font: f, SrcW: 1, SrcH: 1, X: 13, Y: 0},
		}}
	l := &Layer{W: 6, H: 1, FontRegistry: map[string]*Font{f.Name: f}, Stamps: []*Stamp{s},
		OnDrop: func(stamp *Stamp, why string) { dropped = append(dropped, stamp.Key+":"+why) }}
	original := []byte{1, 1, 1, 15, 1, 15}
	l.Frame(original, make([]byte, 6*3))
	if s.State != Shown || len(s.anchors) != 3 || s.anchors[0] || !s.anchors[1] || !s.anchors[2] {
		t.Fatalf("physical glyph anchor setup failed: state=%v anchors=%v", s.State, s.anchors)
	}
	for i := 0; i < 3; i++ {
		l.Frame([]byte{1, 1, 1, 1, 1, 1}, make([]byte, 6*3))
	}
	if len(l.Stamps) != 0 || len(dropped) != 1 || dropped[0] != "anchored:anchors" {
		t.Fatalf("lost anchors left a physical stamp: stamps=%d dropped=%v", len(l.Stamps), dropped)
	}
	dst := bytes.Repeat([]byte{0x5a}, 18*3*4)
	before := append([]byte(nil), dst...)
	if drew, err := l.DrawChecked(dst, 3, nil); err != nil || drew || !bytes.Equal(dst, before) {
		t.Fatalf("lost anchors revived physical ink: drew=%v err=%v", drew, err)
	}
}

func TestPixelGlyphCanonicalFontHashIgnoresMapOrderAndDetectsBytes(t *testing.T) {
	a := physicalTestFont()
	a.Glyphs['B'] = bytes.Repeat([]byte{0x40}, 32)
	b := &Font{W: a.W, H: a.H, Name: a.Name, Glyphs: map[rune][]byte{'B': append([]byte(nil), a.Glyphs['B']...), 'A': append([]byte(nil), a.Glyphs['A']...)}}
	ha, err := canonicalFontSHA256(a)
	if err != nil {
		t.Fatal(err)
	}
	hb, err := canonicalFontSHA256(b)
	if err != nil || ha != hb {
		t.Fatalf("map insertion order changed hash: err=%v", err)
	}
	b.Glyphs['B'][0] ^= 0x80
	hc, err := canonicalFontSHA256(b)
	if err != nil || ha == hc {
		t.Fatalf("bitmap mutation did not change hash: err=%v", err)
	}
	b.Glyphs['B'] = b.Glyphs['B'][:1]
	if _, err := canonicalFontSHA256(b); err == nil {
		t.Fatal("malformed bitmap accepted by canonical hash")
	}
}
