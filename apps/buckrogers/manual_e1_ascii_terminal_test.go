package buckrogers

import (
	"strings"
	"testing"
)

// Buck repo spec 055 §3.1: ASCII sentence punctuation and the slash between
// two non-identifier characters are terminal punctuation of the E1 plan.

func manualE1TerminalPlan(t *testing.T, text string) (*ManualE1Plan, error) {
	t.Helper()
	catalog := manualOverlayCatalog(text)
	base, derived := manualE1SyntheticFonts(text)
	return BuildManualE1Plan(loadManualOverlayLayout(t), catalog, manualOverlayRequest(catalog, 1), base, derived, halfFontsOf(base).X3)
}

func TestManualE1PlanASCIITerminalAccepted(t *testing.T) {
	for _, text := range []string{
		"甲...", "「甲...」", "(甲...)", "「甲.」", "RAM.", "RAM, 乙", "甲,乙", "甲:乙", "甲!", "甲?",
		"甲/乙", "甲/RAM", "甲(RAM)乙", "甲(RAM.)乙", "乙 RAM. 乙", "(RAM).",
	} {
		if plan, err := manualE1TerminalPlan(t, text); err != nil || plan == nil {
			t.Errorf("%q 被拒：%v", text, err)
		}
	}
}

func TestManualE1PlanASCIITerminalRefused(t *testing.T) {
	for _, text := range []string{
		// Spec 055 §2: the orphan connectors stay refused.
		".RAM", "RAM/", "RAM++1", "A - B", "%RAM", "RAM%%",
		// Start of a paragraph, after a space, first in a bracket.
		",甲", ":甲", "!甲", "?甲", "/甲", "甲 ,乙", "甲 .", "甲 /乙", "(,甲)", "(.甲)", "(/甲)",
		// A bracket name must start with a letter or a digit.
		"(.A)", "(/A)", "(.5)", "（.A）", "A（/1）",
	} {
		if plan, err := manualE1TerminalPlan(t, text); err == nil || plan != nil {
			t.Errorf("%q 被接受", text)
		}
	}
}

// The punctuation folds into the token before it: one cluster, its width the
// token's plus 12 pixels a mark, and a line never starts with it.
func TestManualE1PlanASCIITerminalFoldsAndWraps(t *testing.T) {
	plan, err := manualE1TerminalPlan(t, "甲...乙")
	if err != nil {
		t.Fatal(err)
	}
	tokens := plan.lines[0].tokens
	if len(tokens) != 2 || string(tokens[0].runes) != "甲..." || tokens[0].advance != manualE1PixelCJK+3*manualE1PixelLatin || string(tokens[1].runes) != "乙" {
		t.Fatalf("tokens=%+v", tokens)
	}
	plan, err = manualE1TerminalPlan(t, "RAM.")
	if err != nil {
		t.Fatal(err)
	}
	if tokens = plan.lines[0].tokens; len(tokens) != 1 || tokens[0].advance != 3*manualE1PixelLatin+manualE1PixelLatin {
		t.Fatalf("RAM. tokens=%+v", tokens)
	}
	// 35 full-width characters fill 840 of the 864 pixels; the 36th with its
	// comma is 36 pixels wide, so the pair moves to the next row together.
	plan, err = manualE1TerminalPlan(t, strings.Repeat("字", 35)+"字,字")
	if err != nil {
		t.Fatal(err)
	}
	second := plan.lines[1].tokens
	if len(second) == 0 || string(second[0].runes) != "字," {
		t.Fatalf("第二列首個 token = %q，應為「字,」", func() string {
			if len(second) == 0 {
				return ""
			}
			return string(second[0].runes)
		}())
	}
	if plan.lines[0].used != 35*manualE1PixelCJK {
		t.Errorf("第一列用了 %d 像素", plan.lines[0].used)
	}
}
