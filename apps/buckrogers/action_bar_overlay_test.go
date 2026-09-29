package buckrogers

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
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

// actionOverlayFont is a synthetic 16×16 font: CJK glyphs fill the cell,
// half-width glyphs fill columns 4–11 (spec 039 §3.2 ink rule).
func actionOverlayFont() *xlate.Font {
	glyph := make([]byte, 32)
	half := make([]byte, 32)
	for y := 0; y < 16; y++ {
		glyph[y*2] = 0xff
		glyph[y*2+1] = 0xff
		half[y*2] = 0x0f
		half[y*2+1] = 0xf0
	}
	return &xlate.Font{W: 16, H: 16, Glyphs: map[rune][]byte{'(': half, ')': half, 'A': half, 'S': half, 'P': half, 'N': half, 'D': half, '加': glyph, '點': glyph, '減': glyph, '上': glyph, '頁': glyph, '下': glyph, '完': glyph, '成': glyph}}
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
			isCJK := !isHalfwidth(stamp.Text[0])
			if !isCJK {
				// Spec 039 §3.3: the half font (8×16 at 2×, 12×24 at 3×) at
				// offset 0 in a 4-pixel cell; the key stays <event>#<rune>.
				if stamp.Font.W != 4*scale || stamp.Font.H != 8*scale || !strings.HasSuffix(stamp.Font.Name, fmt.Sprintf(".half%dx%d", 4*scale, 8*scale)) ||
					stamp.CellW != 4 || stamp.GlyphX != 0 || stamp.GlyphY != 0 || stamp.GlyphScale != 1 {
					t.Fatalf("scale=%d index=%d half layout drift: %#v", scale, index, stamp)
				}
				continue
			}
			if scale == 2 {
				if stamp.Font.W != 16 || stamp.Font.H != 16 || stamp.GlyphX != 0 || stamp.GlyphY != 0 || stamp.GlyphScale != 0 {
					t.Fatalf("scale=%d index=%d CJK/2x drift: %#v", scale, index, stamp)
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

// Spec 039 §5.2: the mnemonic letters are drawn whole (spec 215 clipped the
// right half of the 16×16 glyph); keys stay <event>#<rune index>.
func TestActionBarOverlayMnemonicWholeAndKeysKept(t *testing.T) {
	c, rects := formalActionOverlay(t)
	e, r := actionOverlayEvent(c, "normal")
	style := HotkeyPreservingActionBarNormalStyle()
	for _, scale := range []int{2, 3} {
		o, err := BuildActionBarOverlay(c, rects, e, r, actionOverlayFont(), [256][3]uint8{}, scale, &style)
		if err != nil {
			t.Fatal(err)
		}
		x := 0
		for index, stamp := range o.Layer.Stamps {
			if want := fmt.Sprintf("%s#%d", e.EventKey, index); stamp.Key != want || stamp.X != x {
				t.Fatalf("scale=%d stamp %d key %q x %d", scale, index, stamp.Key, stamp.X)
			}
			x += stamp.CellW
			if !isHalfwidth(stamp.Text[0]) {
				continue
			}
			ink, err := menuInkRect(stamp, scale)
			if err != nil || ink.X != stamp.X*scale || ink.Width != 4*scale || ink.Height != 8*scale {
				t.Fatalf("scale=%d rune %q ink %+v err %v", scale, stamp.Text[0], ink, err)
			}
			// Drawn pixels: the whole half cell carries the glyph colour.
			rgba := make([]byte, 320*scale*200*scale*4)
			stamp.State = xlate.Shown
			stamp.FG = [3]uint8{255, 255, 255}
			(&xlate.Layer{W: 320, H: 200, Stamps: []*xlate.Stamp{stamp}}).Draw(rgba, scale, nil)
			for dx := 0; dx < 4*scale; dx++ {
				p := 4 * ((stamp.Y*scale+3)*320*scale + stamp.X*scale + dx)
				if rgba[p] != 255 {
					t.Fatalf("scale=%d rune %q column %d not drawn", scale, stamp.Text[0], dx)
				}
			}
		}
	}
}
