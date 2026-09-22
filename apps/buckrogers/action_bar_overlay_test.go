package buckrogers

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/wicanr2/dosgolem/xlate"
)

func formalActionOverlay(t *testing.T) (*ActionBarRequestCatalog, *MenuOverlayRects) {
	t.Helper()
	root := os.Getenv("BUCKROGERS_CHT_ROOT")
	if root == "" {
		t.Skip("BUCKROGERS_CHT_ROOT not set")
	}
	events, err := os.ReadFile(filepath.Join(root, "text", "skill-action-bar-events.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	texts, err := os.ReadFile(filepath.Join(root, "text", "skill-action-bar.zh-TW.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	rectData, err := os.ReadFile(filepath.Join(root, "text", "skill-action-bar-text-safe-rects.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	catalog, err := LoadActionBarRequestCatalog(events, texts)
	if err != nil {
		t.Fatal(err)
	}
	rects, err := LoadActionBarOverlayRects("skill-action-bar-text-safe-rects.tsv", rectData)
	if err != nil {
		t.Fatal(err)
	}
	if err := ValidateActionBarOverlayCoverage(catalog, rects); err != nil {
		t.Fatal(err)
	}
	return catalog, rects
}

func actionOverlayFont() *xlate.Font {
	glyph := make([]byte, 32)
	for y := 0; y < 16; y++ {
		glyph[y*2] = 0xff
		glyph[y*2+1] = 0xff
	}
	return &xlate.Font{W: 16, H: 16, Glyphs: map[rune][]byte{'(': glyph, ')': glyph, 'A': glyph, 'S': glyph, 'P': glyph, 'N': glyph, 'D': glyph, '加': glyph, '點': glyph, '減': glyph, '上': glyph, '頁': glyph, '下': glyph, '完': glyph, '成': glyph}}
}

func actionOverlayEvent(c *ActionBarRequestCatalog, variant string) (ActionBarEvent, DisplayRequest) {
	entry := c.events.byScreen["career"][0]
	e := ActionBarEvent{EntryStep: 1, PostCallStep: 2, Screen: "career", EventKey: "career.action.add." + variant, Variant: variant,
		OriginalLength: entry.length, OriginalSHA256: entry.hash, Row: entry.row, Column: entry.column, X0: entry.x0, Y0: entry.y0, X1: entry.x1, Y1: entry.y1}
	r, _ := c.Resolve(e)
	return e, r
}

func TestActionBarOverlayBuildsHotkeyPreservingNormalAtBothScales(t *testing.T) {
	c, rects := formalActionOverlay(t)
	e, r := actionOverlayEvent(c, "normal")
	for _, scale := range []int{2, 3} {
		style := HotkeyPreservingActionBarNormalStyle()
		o, err := BuildActionBarOverlay(c, rects, e, r, actionOverlayFont(), [256][3]uint8{}, scale, &style)
		if err != nil || len(o.Layer.Stamps) != 5 || o.ClearRect != (PixelRect{0, 192 * scale, 32 * scale, 8 * scale}) {
			t.Fatalf("scale=%d overlay=%#v err=%v", scale, o, err)
		}
	}
}

func TestActionBarOverlayUses22PixelCJKOnlyAtThreeTimes(t *testing.T) {
	c, rects := formalActionOverlay(t)
	e, r := actionOverlayEvent(c, "normal")
	style := HotkeyPreservingActionBarNormalStyle()
	for _, scale := range []int{2, 3} {
		o, err := BuildActionBarOverlay(c, rects, e, r, actionOverlayFont(), [256][3]uint8{}, scale, &style)
		if err != nil {
			t.Fatal(err)
		}
		for index, stamp := range o.Layer.Stamps {
			isCJK := index >= 3
			if scale == 2 || !isCJK {
				wantOffset := (8*scale - 16) / 2
				if stamp.Font.W != 16 || stamp.Font.H != 16 || stamp.GlyphX != wantOffset || stamp.GlyphY != wantOffset || stamp.GlyphScale != 0 {
					t.Fatalf("scale=%d index=%d ASCII/2x drift: %#v", scale, index, stamp)
				}
				continue
			}
			if stamp.Font.W != 22 || stamp.Font.H != 22 || stamp.GlyphX != 1 || stamp.GlyphY != 1 || stamp.GlyphScale != 1 {
				t.Fatalf("3x CJK index=%d layout=%#v", index, stamp)
			}
			ink, err := menuInkRect(stamp, scale)
			if err != nil || ink.Width > 22 || ink.Height > 22 || ink.X < stamp.X*scale || ink.Y < stamp.Y*scale || ink.X+ink.Width > (stamp.X+stamp.CellW)*scale || ink.Y+ink.Height > (stamp.Y+stamp.CellH)*scale {
				t.Fatalf("3x CJK index=%d ink=%#v err=%v", index, ink, err)
			}
		}
	}
}

func TestActionBarOverlayFocusUsesExactStyleWithoutNormalPolicy(t *testing.T) {
	c, rects := formalActionOverlay(t)
	e, r := actionOverlayEvent(c, "focus")
	o, err := BuildActionBarOverlay(c, rects, e, r, actionOverlayFont(), [256][3]uint8{}, 2, nil)
	if err != nil || len(o.RuneForegrounds) != 5 || o.RuneForegrounds[0] != 0 || o.RuneForegrounds[4] != 0 {
		t.Fatalf("overlay=%#v err=%v", o, err)
	}
	style := HotkeyPreservingActionBarNormalStyle()
	if _, err := BuildActionBarOverlay(c, rects, e, r, actionOverlayFont(), [256][3]uint8{}, 2, &style); err == nil {
		t.Fatal("focus must reject normal policy")
	}
}

func TestActionBarOverlayRejectsMissingAndUnprovenNormalPolicy(t *testing.T) {
	c, rects := formalActionOverlay(t)
	e, r := actionOverlayEvent(c, "normal")
	for _, style := range []*ActionBarNormalStyle{nil, {RuneForegrounds: []uint8{10}}, {RuneForegrounds: []uint8{10, 10, 10, 10, 10}}} {
		if _, err := BuildActionBarOverlay(c, rects, e, r, actionOverlayFont(), [256][3]uint8{}, 2, style); err == nil {
			t.Fatalf("style=%#v must fail", style)
		}
	}
}

func TestActionBarOverlayCoverageRejectsMissingAndOrphan(t *testing.T) {
	c, rects := formalActionOverlay(t)
	delete(rects.byEvent, "career.action.add.normal")
	if ValidateActionBarOverlayCoverage(c, rects) == nil {
		t.Fatal("missing rect must fail")
	}
	rects.byEvent["career.action.add.normal"] = overlayRect{0, 192, 32, 8, 0, 192, 4, 1, "single-line-reject"}
	rects.byEvent["orphan"] = overlayRect{0, 192, 24, 8, 0, 192, 3, 1, "single-line-reject"}
	if ValidateActionBarOverlayCoverage(c, rects) == nil {
		t.Fatal("orphan rect must fail")
	}
}
