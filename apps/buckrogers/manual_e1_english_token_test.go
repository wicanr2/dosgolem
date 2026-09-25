package buckrogers

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/wicanr2/dosgolem/xlate"
)

// TestManualE1PlanDeimosPrisonEnglishTokens pins the production E1 word
// boundary contract for the first-question paragraph: every occurrence of
// RAM, Deimos and Stockade stays inside a single sealed token (never split
// across a line break) and every consecutive ASCII letter pair advances
// exactly 14 physical pixels.
func TestManualE1PlanDeimosPrisonEnglishTokens(t *testing.T) {
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
	var translation, eventKey string
	for _, entry := range catalog.byIdentity {
		if entry.textKey == "manual.log.49.deimos_prison" {
			translation, eventKey = entry.translation, entry.eventKey
		}
	}
	if translation == "" {
		t.Fatal("manual.log.49.deimos_prison not found in private catalog")
	}
	plan, err := BuildManualE1Plan(loadManualOverlayLayout(t), catalog,
		DisplayRequest{Generation: 1, EventKey: eventKey, TextKey: "manual.log.49.deimos_prison", Translation: translation},
		base, derived)
	if err != nil {
		t.Fatal(err)
	}
	runes := []rune(translation)
	type tokenSpan struct {
		start, end, line int
	}
	var spans []tokenSpan
	for row := range plan.lines {
		for _, token := range plan.lines[row].tokens {
			spans = append(spans, tokenSpan{start: token.start, end: token.end, line: row})
		}
	}
	containing := func(s, e int) int {
		found := -1
		for i, span := range spans {
			if span.start <= s && e <= span.end {
				if found != -1 {
					t.Fatalf("occurrence [%d,%d) %q spans overlapping tokens", s, e, string(runes[s:e]))
				}
				found = i
			}
		}
		return found
	}
	for word, want := range map[string]int{"RAM": 3, "Deimos": 1, "Stockade": 1} {
		count := strings.Count(translation, word)
		if count != want {
			t.Fatalf("translation holds %d %q, want %d", count, word, want)
		}
		offset := 0
		for n := 0; n < want; n++ {
			at := strings.Index(translation[offset:], word)
			if at < 0 {
				t.Fatalf("occurrence %d of %q not found", n, word)
			}
			s := len([]rune(translation[:offset+at]))
			e := s + len([]rune(word))
			if containing(s, e) == -1 {
				t.Fatalf("%q occurrence %d [%d,%d) is split across sealed tokens", word, n, s, e)
			}
			offset += at + len(word)
		}
	}
	for row := range plan.lines {
		for _, token := range plan.lines[row].tokens {
			var prevX int
			var prevLetter bool
			for _, glyph := range token.glyphs {
				letter := glyph.r < 128 && manualE1ASCIIAlpha(glyph.r)
				if letter && prevLetter && glyph.x-prevX != 14 {
					t.Fatalf("row %d token %q: %q advances %dpx, want 14px", row, string(token.runes), string(glyph.r), glyph.x-prevX)
				}
				prevX, prevLetter = glyph.x, letter
			}
		}
	}
}
