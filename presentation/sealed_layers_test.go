package presentation

import (
	"bytes"
	"fmt"
	"testing"

	"github.com/wicanr2/dosgolem/host"
	"github.com/wicanr2/dosgolem/xlate"
)

type sealedCountingSource struct {
	frame  host.IndexedFrame
	reads  int
	onRead func()
}

func (s *sealedCountingSource) ReadPresentationFrame() (host.IndexedFrame, error) {
	s.reads++
	if s.onRead != nil {
		s.onRead()
	}
	return s.frame, nil
}

func sealedTestSlots() ([]ActiveLayerSlot, map[string]*xlate.Font, *xlate.Layer, *xlate.Font) {
	font := &xlate.Font{Name: "sealed.test.1", W: 1, H: 1, Glyphs: map[rune][]byte{'中': {0x80}}}
	background := &xlate.Layer{W: 2, H: 1, Stamps: []*xlate.Stamp{{Key: "background", X: 0, Y: 0, Cells: 2, CellW: 1, CellH: 1, State: xlate.Shown, BG: [3]uint8{10, 20, 30}}}}
	text := &xlate.Layer{W: 2, H: 1, Stamps: []*xlate.Stamp{{Key: "text", X: 0, Y: 0, Cells: 1, CellW: 1, CellH: 1, State: xlate.Shown, Font: font, Text: []rune{'中'}, BG: [3]uint8{10, 20, 30}, FG: [3]uint8{200, 210, 220}}}}
	return []ActiveLayerSlot{{Name: "background", Z: 0, Layer: background}, {Name: "text", Z: 1, Layer: text}}, map[string]*xlate.Font{font.Name: font}, text, font
}

func TestSealedLayers2x3xOneFrameNoAliasesAndExpiredLease(t *testing.T) {
	for _, scale := range []int{2, 3} {
		t.Run(fmt.Sprintf("%dx", scale), func(t *testing.T) {
			slots, fonts, sourceLayer, sourceFont := sealedTestSlots()
			source := &sealedCountingSource{frame: host.IndexedFrame{Canvas: host.Canvas{Width: 2, Height: 1}, Indexed: []byte{0, 0}}}
			var escaped *SealedLayerGroup
			err := WithSealedOrderedLayers("sealed.test", 7, 1, slots, fonts, func(group *SealedLayerGroup) error {
				escaped = group
				before, err := ProjectSealedLayers(source, group, scale, func() bool { return true })
				if err != nil || source.reads != 1 || !before.Drew {
					return fmt.Errorf("project err=%v reads=%d drew=%v", err, source.reads, before.Drew)
				}
				if !bytes.Equal(before.RGBA[:4], []byte{200, 210, 220, 255}) {
					return fmt.Errorf("foreground=%v", before.RGBA[:4])
				}
				sourceLayer.Stamps[0].FG = [3]uint8{255, 0, 0}
				sourceFont.Glyphs['中'][0] = 0
				after, err := ProjectSealedLayers(source, group, scale, func() bool { return true })
				if err != nil || !bytes.Equal(before.RGBA, after.RGBA) || source.reads != 2 {
					return fmt.Errorf("source alias changed sealed output: err=%v reads=%d", err, source.reads)
				}
				return nil
			})
			if err != nil {
				t.Fatal(err)
			}
			if shot, err := ProjectSealedLayers(source, escaped, scale, func() bool { return true }); err == nil || len(shot.RGBA) != 0 || source.reads != 2 {
				t.Fatalf("escaped group remained active: err=%v rgba=%d reads=%d", err, len(shot.RGBA), source.reads)
			}
		})
	}
}

func TestSealedLayersRejectBeforeReadAndAfterReadInvalidation(t *testing.T) {
	font := &xlate.Font{Name: "sealed.test.1", W: 1, H: 1, Glyphs: map[rune][]byte{'中': {0x80}}}
	badSource := &xlate.Layer{W: 2, H: 1, Stamps: []*xlate.Stamp{nil}}
	called := false
	if err := WithSealedOrderedLayers("sealed.test", 1, 1, []ActiveLayerSlot{{Name: "text", Z: 0, Layer: badSource}}, map[string]*xlate.Font{font.Name: font}, func(*SealedLayerGroup) error { called = true; return nil }); err == nil || called {
		t.Fatalf("nil origin stamp reached callback: err=%v called=%v", err, called)
	}
	frame := host.IndexedFrame{Canvas: host.Canvas{Width: 2, Height: 1}, Indexed: []byte{0, 0}}
	slots, fonts, _, _ := sealedTestSlots()
	if err := WithSealedOrderedLayers("sealed.test", 7, 1, slots, fonts, func(group *SealedLayerGroup) error {
		stale := &sealedCountingSource{frame: frame}
		if shot, err := ProjectSealedLayers(stale, group, 2, func() bool { return false }); err == nil || len(shot.RGBA) != 0 || stale.reads != 0 {
			return fmt.Errorf("stale owner: err=%v rgba=%d reads=%d", err, len(shot.RGBA), stale.reads)
		}
		missing := &sealedCountingSource{frame: frame}
		group.fonts["sealed.test.1"].Glyphs['中'][0] ^= 0xff
		if shot, err := ProjectSealedLayers(missing, group, 2, func() bool { return true }); err == nil || len(shot.RGBA) != 0 || missing.reads != 0 {
			return fmt.Errorf("sealed font tamper: err=%v rgba=%d reads=%d", err, len(shot.RGBA), missing.reads)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	slots, fonts, _, _ = sealedTestSlots()
	if err := WithSealedOrderedLayers("sealed.test", 7, 1, slots, fonts, func(group *SealedLayerGroup) error {
		valid := true
		reentrant := &sealedCountingSource{frame: frame, onRead: func() { valid = false }}
		if shot, err := ProjectSealedLayers(reentrant, group, 2, func() bool { return valid }); err == nil || len(shot.RGBA) != 0 || reentrant.reads != 1 {
			return fmt.Errorf("read-time invalidation: err=%v rgba=%d reads=%d", err, len(shot.RGBA), reentrant.reads)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}

func TestSealedLayersProjectTwoFontPhysicalGlyphsAtFourteenPixelAdvance(t *testing.T) {
	latin := &xlate.Font{Name: "sealed.latin", W: 1, H: 1, Glyphs: map[rune][]byte{'A': {0x80}}}
	cjk := &xlate.Font{Name: "sealed.cjk", W: 1, H: 1, Glyphs: map[rune][]byte{'中': {0x80}}}
	fonts := map[string]*xlate.Font{latin.Name: latin, cjk.Name: cjk}
	bg := [3]uint8{10, 20, 30}
	fg := [3]uint8{200, 210, 220}
	background := &xlate.Layer{W: 16, H: 8, Stamps: []*xlate.Stamp{{Key: "background", X: 0, Y: 0, Cells: 16, CellW: 1, CellH: 8, State: xlate.Shown, BG: bg}}}
	text := &xlate.Layer{W: 16, H: 8, Stamps: []*xlate.Stamp{{Key: "text", X: 0, Y: 0, Cells: 16, CellW: 1, CellH: 8, PixelScale: 3, State: xlate.Shown, BG: bg, FG: fg, PixelGlyphs: []xlate.PixelGlyph{
		{Rune: 'A', Font: latin, SrcW: 1, SrcH: 1, X: 1, Y: 1},
		{Rune: '中', Font: cjk, SrcW: 1, SrcH: 1, X: 15, Y: 1},
	}}}}
	// Source layer deliberately has no FontRegistry: sealing must use the
	// explicit canonical registry without modifying the caller's layer.
	slots := []ActiveLayerSlot{{Name: "background", Z: 0, Layer: background}, {Name: "text", Z: 1, Layer: text}}
	source := &sealedCountingSource{frame: host.IndexedFrame{Canvas: host.Canvas{Width: 16, Height: 8}, Indexed: make([]byte, 16*8)}}
	if err := WithSealedOrderedLayers("sealed.physical", 1, 1, slots, fonts, func(group *SealedLayerGroup) error {
		if text.FontRegistry != nil {
			return fmt.Errorf("seal mutated source layer registry")
		}
		shot, err := ProjectSealedLayers(source, group, 3, func() bool { return true })
		if err != nil || source.reads != 1 || !shot.Drew {
			return fmt.Errorf("physical projection err=%v reads=%d drew=%v", err, source.reads, shot.Drew)
		}
		pixel := func(x, y int) []byte { return shot.RGBA[4*(y*48+x) : 4*(y*48+x)+4] }
		wantFG := []byte{fg[0], fg[1], fg[2], 255}
		wantBG := []byte{bg[0], bg[1], bg[2], 255}
		if !bytes.Equal(pixel(1, 1), wantFG) || !bytes.Equal(pixel(15, 1), wantFG) || !bytes.Equal(pixel(2, 1), wantBG) {
			return fmt.Errorf("14px physical origins or background changed")
		}
		latin.Glyphs['A'][0] = 0
		cjk.Glyphs['中'][0] = 0
		again, err := ProjectSealedLayers(source, group, 3, func() bool { return true })
		if err != nil || !bytes.Equal(shot.RGBA, again.RGBA) {
			return fmt.Errorf("source font alias changed sealed output: %v", err)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if source.reads != 2 {
		t.Fatalf("successful projections read %d frames, want 2", source.reads)
	}
}

func TestSealedLayersPhysicalPreflightRejectsScaleAndUnregisteredGlyphBeforeFrame(t *testing.T) {
	f := &xlate.Font{Name: "sealed.physical", W: 1, H: 1, Glyphs: map[rune][]byte{'A': {0x80}}}
	stamp := &xlate.Stamp{X: 0, Y: 0, Cells: 2, CellW: 1, CellH: 1, PixelScale: 3, State: xlate.Shown, PixelGlyphs: []xlate.PixelGlyph{{Rune: 'A', Font: f, SrcW: 1, SrcH: 1}}}
	layer := &xlate.Layer{W: 2, H: 1, Stamps: []*xlate.Stamp{stamp}}
	slots := []ActiveLayerSlot{{Name: "text", Z: 0, Layer: layer}}
	source := &sealedCountingSource{frame: host.IndexedFrame{Canvas: host.Canvas{Width: 2, Height: 1}, Indexed: []byte{0, 0}}}
	if err := WithSealedOrderedLayers("sealed.physical", 1, 1, slots, map[string]*xlate.Font{f.Name: f}, func(group *SealedLayerGroup) error {
		shot, err := ProjectSealedLayers(source, group, 2, func() bool { return true })
		if err == nil || len(shot.RGBA) != 0 || source.reads != 0 {
			return fmt.Errorf("cross-scale physical group read frame: err=%v reads=%d", err, source.reads)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	other := &xlate.Font{Name: f.Name, W: 1, H: 1, Glyphs: map[rune][]byte{'A': {0x80}}}
	stamp.PixelGlyphs[0].Font = other
	called := false
	if err := WithSealedOrderedLayers("sealed.physical", 1, 1, slots, map[string]*xlate.Font{f.Name: f}, func(*SealedLayerGroup) error { called = true; return nil }); err == nil || called || source.reads != 0 {
		t.Fatalf("unregistered source pixel glyph reached callback: err=%v called=%v reads=%d", err, called, source.reads)
	}
}
