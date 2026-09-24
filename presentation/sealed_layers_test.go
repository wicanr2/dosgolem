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
