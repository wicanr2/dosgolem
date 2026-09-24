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
