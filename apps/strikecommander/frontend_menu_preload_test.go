package strikecommander

import (
	"bytes"
	"os"
	"reflect"
	"testing"
)

func TestFrontendMenuPreloadMissingAndDefaultIsolation(t *testing.T) {
	for _, root := range []string{"", t.TempDir()} {
		p, e := NewPresenter(PresenterConfig{Fonts: flightTextFonts(t), Scales: []int{2, 3}, MenuRoot: root})
		if e != nil {
			t.Fatal(e)
		}
		info := p.MenuPreload()
		if info.Enabled || len(p.presetMenuButtons) != 0 {
			t.Fatal("invalid preload published")
		}
		if root == "" {
			if info.Reason != "" || p.counts.PlatePreloadMissing != 0 {
				t.Fatal("default path gained preload work")
			}
		} else if info.Reason == "" || p.counts.PlatePreloadMissing != 1 {
			t.Fatal("missing source diagnostic absent")
		}
	}
}
func TestFrontendMenuPreloadActualBytesAndImmediateFiveRows(t *testing.T) {
	root := os.Getenv("SC_MODEL_ROOT")
	if root == "" {
		t.Skip("explicit original read-only root required")
	}
	orig := loadMenuFiles(t) // independent existing test-only PAK/LZW reader+literal SHA
	probe := &Presenter{}
	probe.loadPresetMenu(root)
	if !probe.MenuPreload().Enabled || !bytes.Equal(probe.presetMenuButtons, orig.buttons) {
		t.Fatal("preload does not equal independent original child", probe.MenuPreload())
	}
	for _, scale := range []int{2, 3} {
		early := newArtOverlay(t, scale)
		late := newArtOverlay(t, scale)
		early.presetButtons = probe.presetMenuButtons
		plate := plateEvent(t, orig, 17)
		early.AddDialogArt(plate)
		late.AddDialogArt(plate)
		if early.platePending != nil || len(early.arts) != 5 || late.platePending == nil || len(late.arts) != 0 {
			t.Fatal("not immediate or fallback changed", len(early.arts), len(late.arts))
		}
		late.AddDialogArt(mmEvent(t, orig, 2, 18))
		for i := 0; i < 5; i++ {
			a, b := early.arts[i], late.arts[i]
			if a.ev.Order != 17 || a.ev.Line != i || !reflect.DeepEqual(a.rect, b.rect) || !reflect.DeepEqual(a.idx, b.idx) || !reflect.DeepEqual(a.ref, b.ref) || !reflect.DeepEqual(a.stamp(tPal(), scale).Pix, b.stamp(tPal(), scale).Pix) {
				t.Fatal("preloaded plate differs", scale, i)
			}
		}
		frame := loadIdx18(t, "a4/menu")
		out := make([]byte, 320*200*scale*scale*4)
		scaleFrontendInto(out, frame, tPal(), scale)
		early.drawOverlayInto(out, frame, tPal())
		if bytes.Equal(out, ScaleIndexedRGBA(frame, tPal(), scale)) {
			t.Fatal("five rows produced no actual output")
		}
		early.ClearArt()
		if early.presetPlate != nil {
			t.Fatal("Clear retained old plate event")
		}
	}
}
