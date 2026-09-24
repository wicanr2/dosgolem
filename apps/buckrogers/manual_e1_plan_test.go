package buckrogers

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/wicanr2/dosgolem/xlate"
)

func manualE1SyntheticFonts(text string) (*xlate.Font, *xlate.Font) {
	glyphs := make(map[rune][]byte)
	for _, r := range text {
		glyph := make([]byte, 32)
		if manualE1ASCIIAlnum(r) || manualE1ASCIIConnector(r) || r == '%' || r == '（' || r == '）' || r == '(' || r == ')' {
			glyph[0] = 0x80
		} else {
			for i := range glyph {
				glyph[i] = 0xff
			}
		}
		glyphs[r] = glyph
	}
	base := &xlate.Font{Name: "e1.synthetic.base16", W: 16, H: 16, Glyphs: glyphs}
	derived := manualThreeXFont(base)
	derived.Name = "e1.synthetic.derived22"
	return base, derived
}

func TestManualE1PlanAll39PrivateCatalog(t *testing.T) {
	project := os.Getenv("BUCK_OWNER_PROJECT")
	if project == "" {
		t.Skip("private local catalog and font are not available")
	}
	read := func(path string) []byte {
		t.Helper()
		data, err := os.ReadFile(filepath.Join(project, path))
		if err != nil {
			t.Fatal(err)
		}
		return data
	}
	base, err := xlate.ParseFont(read("workplace/current-font/buckrogers-eten-top-pad.golemfnt"))
	if err != nil {
		t.Fatal(err)
	}
	base.Name = "e1.private.base16"
	derived := manualThreeXFont(base)
	derived.Name = "e1.private.derived22"
	catalog, err := LoadCatalog(read("text/manual-events.tsv"), read("text/manual-ordinals.tsv"), read("text/manual.zh-TW.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	if len(catalog.byIdentity) != 39 {
		t.Fatalf("catalog entries=%d, want 39", len(catalog.byIdentity))
	}
	leading, trailing := 0, 0
	for _, entry := range catalog.byIdentity {
		request := DisplayRequest{Generation: 1, EventKey: entry.eventKey, TextKey: entry.textKey, Translation: entry.translation}
		plan, err := BuildManualE1Plan(loadManualOverlayLayout(t), catalog, request, base, derived)
		if err != nil {
			t.Fatalf("text_key=%s: %v", entry.textKey, err)
		}
		layer, err := plan.TextLayer(base, derived)
		if err != nil || layer == nil || len(layer.Stamps) != 14 {
			t.Fatalf("text_key=%s: layer=%v err=%v", entry.textKey, layer, err)
		}
		for _, line := range plan.lines {
			for index, token := range line.tokens {
				if token.kind != "space" || token.advance != 0 || len(token.glyphs) != 0 {
					continue
				}
				if index == 0 {
					leading++
				}
				if index == len(line.tokens)-1 {
					trailing++
				}
			}
		}
		for _, stamp := range layer.Stamps {
			stamp.State = xlate.Shown
			stamp.BG, stamp.FG = [3]uint8{1, 2, 3}, [3]uint8{4, 5, 6}
		}
		if err := layer.ValidatePixelGlyphPlan(3); err != nil {
			t.Fatalf("text_key=%s: %v", entry.textKey, err)
		}
		dst := make([]byte, 960*600*4)
		if drew, err := layer.DrawChecked(dst, 3, nil); err != nil || !drew {
			t.Fatalf("text_key=%s checked draw: drew=%v err=%v", entry.textKey, drew, err)
		}
		snapshot, err := layer.Snapshot()
		if err != nil {
			t.Fatalf("text_key=%s snapshot: %v", entry.textKey, err)
		}
		var restored xlate.Layer
		if err := restored.Restore(snapshot, layer.FontRegistry); err != nil {
			t.Fatalf("text_key=%s restore: %v", entry.textKey, err)
		}
		got := make([]byte, len(dst))
		if drew, err := restored.DrawChecked(got, 3, nil); err != nil || !drew || !bytes.Equal(dst, got) {
			t.Fatalf("text_key=%s restore draw: drew=%v err=%v equal=%v", entry.textKey, drew, err, bytes.Equal(dst, got))
		}
	}
	if leading != 3 || trailing != 11 {
		t.Fatalf("soft separators leading/trailing=%d/%d, want 3/11", leading, trailing)
	}
}

func TestManualE1PlanSyntheticBuildCheckedRoundTrip(t *testing.T) {
	text := strings.Repeat("甲", 35) + " RAM…乙（甲）"
	catalog := manualOverlayCatalog(text)
	base, derived := manualE1SyntheticFonts(text)
	plan, err := BuildManualE1Plan(loadManualOverlayLayout(t), catalog, manualOverlayRequest(catalog, 7), base, derived)
	if err != nil {
		t.Fatal(err)
	}
	if plan.generation != 7 || plan.eventKey != "manual.fixture.word1" || plan.textKey != "manual.fixture" {
		t.Fatalf("request identity lost: %d/%q/%q", plan.generation, plan.eventKey, plan.textKey)
	}
	if plan.hash() != plan.CanonicalHash() {
		t.Fatal("canonical hash does not recompute")
	}
	var rebuilt strings.Builder
	next, soft := 0, 0
	for _, line := range plan.lines {
		for index, token := range line.tokens {
			if token.start != next || token.end-token.start != len(token.runes) {
				t.Fatalf("source span does not continue at %d: %#v", next, token)
			}
			next = token.end
			rebuilt.WriteString(string(token.runes))
			if token.kind == "space" && (index == 0 || index == len(line.tokens)-1) {
				if token.advance != 0 || len(token.glyphs) != 0 {
					t.Fatalf("visible boundary separator: %#v", token)
				}
				soft++
			}
		}
	}
	if rebuilt.String() != text || next != plan.runeCount || soft == 0 {
		t.Fatalf("source identity or soft separator lost: runes=%d soft=%d", next, soft)
	}
	layer, err := plan.TextLayer(base, derived)
	if err != nil {
		t.Fatal(err)
	}
	if len(layer.Stamps) != 14 {
		t.Fatalf("text stamps=%d, want 14", len(layer.Stamps))
	}
	var ram []manualE1Glyph
	for _, line := range plan.lines {
		for _, token := range line.tokens {
			if strings.HasPrefix(string(token.runes), "RAM") {
				ram = token.glyphs
			}
		}
	}
	if len(ram) < 3 || ram[1].x-ram[0].x != 14 || ram[2].x-ram[1].x != 14 {
		t.Fatalf("14px ASCII origins lost: %#v", ram)
	}
	for row, stamp := range layer.Stamps {
		if len(plan.lines[row].tokens) == 0 {
			if stamp.PixelScale != 0 || len(stamp.PixelGlyphs) != 0 || len(stamp.Text) != 0 {
				t.Fatalf("tail row %d is not a legacy empty stamp", row)
			}
		} else if stamp.PixelScale != 3 || len(stamp.PixelGlyphs) == 0 {
			t.Fatalf("nonempty row %d is not a physical stamp", row)
		}
		stamp.State = xlate.Shown
		stamp.BG = [3]uint8{1, 2, 3}
		stamp.FG = [3]uint8{4, 5, 6}
	}
	if err := layer.ValidatePixelGlyphPlan(3); err != nil {
		t.Fatal(err)
	}
	dst := make([]byte, 960*600*4)
	if drew, err := layer.DrawChecked(dst, 3, nil); err != nil || !drew {
		t.Fatalf("checked draw: drew=%v err=%v", drew, err)
	}
	snapshot, err := layer.Snapshot()
	if err != nil {
		t.Fatal(err)
	}
	var restored xlate.Layer
	if err := restored.Restore(snapshot, layer.FontRegistry); err != nil {
		t.Fatal(err)
	}
	got := make([]byte, len(dst))
	if drew, err := restored.DrawChecked(got, 3, nil); err != nil || !drew || !bytes.Equal(dst, got) {
		t.Fatalf("restore changed RGBA: drew=%v err=%v", drew, err)
	}
}

func TestManualE1PlanRejectsWideParenAndChangedInputs(t *testing.T) {
	text := "（甲）"
	catalog := manualOverlayCatalog(text)
	request := manualOverlayRequest(catalog, 1)
	base, derived := manualE1SyntheticFonts(text)
	wide := manualThreeXFont(base)
	wide.Name = derived.Name
	wide.Glyphs['（'] = bytes.Repeat([]byte{0xff}, 22*3)
	if _, err := manualE1PlanMeasure([]rune("（甲）"), wide, 14); err == nil {
		t.Fatal("22px parenthesis ink passed E1 token measurement")
	}
	if plan, err := BuildManualE1Plan(loadManualOverlayLayout(t), catalog, request, base, wide); err == nil || plan != nil {
		t.Fatal("22px parenthesis ink must fail closed")
	}
	plan, err := BuildManualE1Plan(loadManualOverlayLayout(t), catalog, request, base, derived)
	if err != nil {
		t.Fatal(err)
	}
	request.Translation = "（乙）"
	if changed, err := BuildManualE1Plan(loadManualOverlayLayout(t), catalog, request, base, derived); err == nil || changed != nil {
		t.Fatal("request outside exact catalog accepted")
	}
	plan.lines[0].tokens[0].runes[0] = '乙'
	if layer, err := plan.TextLayer(base, derived); err == nil || layer != nil {
		t.Fatal("changed plan produced a text layer")
	}
}

func TestManualE1PlanRejectsOrphanASCIIConnectors(t *testing.T) {
	for _, text := range []string{".RAM", "RAM/", "RAM++1", "A - B", "%RAM", "RAM%%"} {
		t.Run(text, func(t *testing.T) {
			catalog := manualOverlayCatalog(text)
			base, derived := manualE1SyntheticFonts(text)
			if plan, err := BuildManualE1Plan(loadManualOverlayLayout(t), catalog, manualOverlayRequest(catalog, 1), base, derived); err == nil || plan != nil {
				t.Fatalf("orphan connector accepted: plan=%v err=%v", plan, err)
			}
		})
	}
}
