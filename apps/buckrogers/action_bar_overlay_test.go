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
	return &xlate.Font{W: 16, H: 16, Glyphs: map[rune][]byte{'加': glyph, '點': glyph, '減': glyph, '上': glyph, '頁': glyph, '下': glyph, '完': glyph, '成': glyph}}
}

func actionOverlayEvent(c *ActionBarRequestCatalog, variant string) (ActionBarEvent, DisplayRequest) {
	entry := c.events.byScreen["career"][0]
	e := ActionBarEvent{EntryStep: 1, PostCallStep: 2, Screen: "career", EventKey: "career.action.add." + variant, Variant: variant,
		OriginalLength: entry.length, OriginalSHA256: entry.hash, Row: entry.row, Column: entry.column, X0: entry.x0, Y0: entry.y0, X1: entry.x1, Y1: entry.y1}
	r, _ := c.Resolve(e)
	return e, r
}

func TestActionBarOverlayBuildsBothExplicitNormalCandidatesAtBothScales(t *testing.T) {
	c, rects := formalActionOverlay(t)
	e, r := actionOverlayEvent(c, "normal")
	for _, scale := range []int{2, 3} {
		for _, colors := range [][]uint8{{15, 10}, {10, 10}} {
			o, err := BuildActionBarOverlay(c, rects, e, r, actionOverlayFont(), [256][3]uint8{}, scale, &ActionBarNormalStyle{RuneForegrounds: colors})
			if err != nil || len(o.Layer.Stamps) != 2 || o.ClearRect != (PixelRect{0, 192 * scale, 24 * scale, 8 * scale}) {
				t.Fatalf("scale=%d colors=%v overlay=%#v err=%v", scale, colors, o, err)
			}
		}
	}
}

func TestActionBarOverlayFocusUsesExactStyleWithoutNormalPolicy(t *testing.T) {
	c, rects := formalActionOverlay(t)
	e, r := actionOverlayEvent(c, "focus")
	o, err := BuildActionBarOverlay(c, rects, e, r, actionOverlayFont(), [256][3]uint8{}, 2, nil)
	if err != nil || len(o.RuneForegrounds) != 2 || o.RuneForegrounds[0] != 0 || o.RuneForegrounds[1] != 0 {
		t.Fatalf("overlay=%#v err=%v", o, err)
	}
	if _, err := BuildActionBarOverlay(c, rects, e, r, actionOverlayFont(), [256][3]uint8{}, 2, &ActionBarNormalStyle{RuneForegrounds: []uint8{10, 10}}); err == nil {
		t.Fatal("focus must reject normal policy")
	}
}

func TestActionBarOverlayRejectsMissingAndUnprovenNormalPolicy(t *testing.T) {
	c, rects := formalActionOverlay(t)
	e, r := actionOverlayEvent(c, "normal")
	for _, style := range []*ActionBarNormalStyle{nil, {RuneForegrounds: []uint8{10}}, {RuneForegrounds: []uint8{10, 9}}} {
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
	rects.byEvent["career.action.add.normal"] = overlayRect{0, 192, 24, 8, 0, 192, 3, 1, "single-line-reject"}
	rects.byEvent["orphan"] = overlayRect{0, 192, 24, 8, 0, 192, 3, 1, "single-line-reject"}
	if ValidateActionBarOverlayCoverage(c, rects) == nil {
		t.Fatal("orphan rect must fail")
	}
}
