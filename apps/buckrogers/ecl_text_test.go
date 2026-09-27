package buckrogers

import (
	"crypto/sha256"
	"fmt"
	"strings"
	"testing"
)

func eclFixture(t *testing.T, pairs ...string) *EclTextCatalog {
	t.Helper()
	ev := "event_key\toriginal_length\toriginal_sha256\tsources\n"
	tr := "key\ttranslation\tsource\n"
	for i := 0; i+1 < len(pairs); i += 2 {
		k := fmt.Sprintf("ecl.t.%02d", i/2)
		ev += fmt.Sprintf("%s\t%d\t%x\tECL1:0:%d\n", k, len(pairs[i]), sha256.Sum256([]byte(pairs[i])), i)
		if pairs[i+1] != "" {
			tr += fmt.Sprintf("%s\t%s\tecl-batch-editorial\n", k, pairs[i+1])
		}
	}
	c, err := LoadEclTextCatalog([]byte(ev), []byte(tr))
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func eclEntry(s string, clear bool, col, row uint8) EclTextEntry {
	return EclTextEntry{Step: 1, SS: 0x1841, SP: 0x3D00, Return: Address{0x2E13, 0x0B79}, Original: []byte(s),
		Clear: clear, Background: 0, Foreground: 10, Left: 1, Top: 17, Right: 38, Bottom: 22, CursorCol: col, CursorRow: row}
}

func TestEclTextHitReturnAndInvalidation(t *testing.T) {
	w := NewEclTextWatcher(eclFixture(t, "HELLO THERE", "你們好。", "UNKNOWN", ""))
	w.ObserveEntry(eclEntry("HELLO THERE", true, 1, 17))
	p := w.Page()
	if p == nil || len(p.Lines) != 1 || string(p.Lines[0].Text) != "你們好。" || p.Top != 17 || !w.InCall() {
		t.Fatalf("hit: %+v", p)
	}
	// In-call writes (typing, page-turn clear) keep the page.
	w.ObserveVideoWrite(18*8*320 + 20)
	if w.Page() == nil {
		t.Fatal("in-call write dropped page")
	}
	// Wrong SP is not the return.
	w.ObserveInstruction(Address{0x2E13, 0x0B79}, 0x1841, 0x3D00)
	if !w.InCall() {
		t.Fatal("unverified return closed call")
	}
	w.ObserveInstruction(Address{0x2E13, 0x0B79}, 0x1841, 0x3D00+eclTextReturnDelta)
	if w.InCall() {
		t.Fatal("verified return not seen")
	}
	// Outside the window: kept.  Inside after the call: dropped.
	w.ObserveVideoWrite(5 * 320)
	if w.Page() == nil {
		t.Fatal("write outside window dropped page")
	}
	w.ObserveVideoWrite(22*8*320 + 38*8 + 7)
	if p := w.Page(); p == nil || p.Shows(22) || !p.Shows(17) {
		t.Fatal("post-call write must remove only its row")
	}
	w.ObserveVideoWrite(17*8*320 + 38*8 + 7)
	if w.Page() != nil {
		t.Fatal("post-call write on the translated row kept page")
	}
	// An untranslated continuation joins the page verbatim (§3.7).
	w.ObserveEntry(eclEntry("HELLO THERE", true, 1, 17))
	w.ObserveInstruction(Address{0x2E13, 0x0B79}, 0x1841, 0x3D00+eclTextReturnDelta)
	w.ObserveEntry(eclEntry("UNKNOWN", false, 12, 17))
	p = w.Page()
	if p == nil || w.Stats.Misses != 1 || w.Stats.Passthrough != 1 || len(p.Lines) != 2 ||
		string(p.Lines[1].Text) != "UNKNOWN" || p.Lines[1].Row != 17 || p.Lines[1].Col != 5 {
		t.Fatalf("passthrough: %+v", p)
	}
	// An untranslated fresh window is left alone.
	w.ObserveInstruction(Address{0x2E13, 0x0B79}, 0x1841, 0x3D00+eclTextReturnDelta)
	w.ObserveEntry(eclEntry("UNKNOWN", true, 1, 17))
	if w.Page() != nil {
		t.Fatal("untranslated fresh window drew")
	}
}

func TestEclTextSeparateWindowsKeepTheirPages(t *testing.T) {
	w := NewEclTextWatcher(eclFixture(t, "ATTACKS", "攻擊", "SWORD", "劍", "ROUND", "回合"))
	ret := func() { w.ObserveInstruction(Address{0x2E13, 0x0B79}, 0x1841, 0x3D00+eclTextReturnDelta) }
	a := eclEntry("ATTACKS", true, 1, 7)
	a.Top, a.Bottom, a.Right = 7, 9, 38
	w.ObserveEntry(a)
	ret()
	b := eclEntry("SWORD", true, 1, 11)
	b.Top, b.Bottom = 11, 21
	w.ObserveEntry(b)
	ret()
	if len(w.Pages()) != 2 {
		t.Fatalf("two windows, got %d pages", len(w.Pages()))
	}
	// A post-call write removes only its own row.
	w.ObserveVideoWrite(15*8*320 + 10*8)
	if len(w.Pages()) != 2 || w.Pages()[1].Shows(15) || !w.Pages()[1].Shows(11) {
		t.Fatalf("row write: %+v", w.Pages())
	}
	// A write on the only translated row drops the page.
	w.ObserveVideoWrite(11*8*320 + 10*8)
	if len(w.Pages()) != 1 || w.Pages()[0].Top != 7 {
		t.Fatalf("write dropped wrong page: %+v", w.Pages())
	}
	// A fresh window overlapping the upper one's rows replaces them.
	c := eclEntry("ROUND", true, 1, 5)
	c.Top, c.Bottom = 5, 8
	w.ObserveEntry(c)
	if len(w.Pages()) != 1 || string(w.Pages()[0].Lines[0].Text) != "回合" {
		t.Fatalf("overlap not replaced: %+v", w.Pages())
	}
}

func TestEclTextFreshWindowKeepsRowsAbove(t *testing.T) {
	w := NewEclTextWatcher(eclFixture(t, "Attacks", "攻擊", "Hitting", "命中"))
	ret := func() { w.ObserveInstruction(Address{0x2E13, 0x0B79}, 0x1841, 0x3D00+eclTextReturnDelta) }
	a := eclEntry("Attacks", true, 23, 11)
	a.Left, a.Top, a.Bottom = 23, 11, 21
	w.ObserveEntry(a)
	ret()
	b := eclEntry("Hitting", true, 30, 11)
	b.Left, b.Top, b.Bottom = 23, 13, 16
	w.ObserveEntry(b)
	ret()
	if len(w.Pages()) != 2 || !w.Pages()[0].Shows(11) || w.Pages()[0].Shows(13) {
		t.Fatalf("fresh window removed rows above it: %+v", w.Pages())
	}
}

func TestEclTextContinuationAndReentry(t *testing.T) {
	w := NewEclTextWatcher(eclFixture(t, "A B", "甲乙", "C D", "丙丁", "E F", "戊"))
	ret := func() { w.ObserveInstruction(Address{0x2E13, 0x0B79}, 0x1841, 0x3D00+eclTextReturnDelta) }
	w.ObserveEntry(eclEntry("A B", true, 1, 17))
	ret()
	// Original cursor at the left column: Chinese continues on the next row.
	w.ObserveEntry(eclEntry("C D", false, 1, 21))
	ret()
	p := w.Page()
	if len(p.Lines) != 2 || p.Lines[1].Row != 18 || p.Lines[1].Col != 1 || p.Top != 17 {
		t.Fatalf("left-column continuation: %+v", p.Lines)
	}
	// Mid-line cursor: Chinese continues on the same row.
	w.ObserveEntry(eclEntry("E F", false, 9, 21))
	p = w.Page()
	if len(p.Lines) != 3 || p.Lines[2].Row != 18 || p.Lines[2].Col != 3 {
		t.Fatalf("mid-line continuation: %+v", p.Lines)
	}
	w.ObserveEntry(eclEntry("A B", true, 1, 17))
	if w.Page() != nil || w.Stats.Reentries != 1 {
		t.Fatal("re-entry kept page")
	}
}

func TestEclTextOverflowAndBadWindow(t *testing.T) {
	long := strings.Repeat("字", 38*6+1)
	w := NewEclTextWatcher(eclFixture(t, "LONG ONE", long, "OK", "好"))
	w.ObserveEntry(eclEntry("LONG ONE", true, 1, 17))
	if w.Page() != nil || w.Stats.Overflows != 1 {
		t.Fatal("overflow drew")
	}
	e := eclEntry("OK", true, 1, 17)
	e.Right = 0
	w.ObserveEntry(e)
	if w.Page() != nil {
		t.Fatal("bad window drew")
	}
}

func TestLayoutEclTextKeepsLatinAndPunctuation(t *testing.T) {
	lines, _, _, ok := layoutEclText([]rune("一二三 NEO。四"), 17, 1, 1, 5, 22)
	if !ok || len(lines) != 2 || string(lines[1].Text) != "NEO。四" || string(lines[0].Text) != "一二三" {
		t.Fatalf("%q", lines)
	}
	lines, _, _, ok = layoutEclText([]rune("一二三四五。"), 17, 1, 1, 5, 22)
	if !ok || string(lines[1].Text) != "五。" {
		t.Fatalf("closing punctuation led a line: %q", lines)
	}
}

func TestEclTextContinuationAfterUntranslatedKeepsEnglishAbove(t *testing.T) {
	w := NewEclTextWatcher(eclFixture(t, "A B", "", "CC DD", "丙丁"))
	w.ObserveEntry(eclEntry("A B", true, 1, 17))
	w.ObserveEntry(eclEntry("CC DD", false, 1, 21))
	p := w.Page()
	if p == nil || p.Top != 21 || p.Lines[0].Row != 21 || p.Lines[0].Col != 1 {
		t.Fatalf("continuation after miss must start at the original cursor: %+v", p)
	}
}

// Spec 027 §3.1 (2026-09-27): tier-2 short pieces.
func TestEclTextShortPieces(t *testing.T) {
	ret := func(w *EclTextWatcher) { w.ObserveInstruction(Address{0x2E13, 0x0B79}, 0x1841, 0x3D00+eclTextReturnDelta) }
	for _, v := range []struct {
		s     string
		short bool
	}{{" ATTACKS.", true}, {"BELOW", true}, {"A B", true}, {"HIM", true}, {"GO TO X", false}, {" THE END. ", false}} {
		if eclShortPiece([]byte(v.s)) != v.short {
			t.Errorf("%q short=%v", v.s, !v.short)
		}
	}
	// After a Chinese page the piece joins it on the same row.
	w := NewEclTextWatcher(eclFixture(t, "FROM THE ", "來自", "BELOW", "下方"))
	w.ObserveEntry(eclEntry("FROM THE ", true, 1, 17))
	ret(w)
	w.ObserveEntry(eclEntry("BELOW", false, 10, 17))
	ret(w)
	p := w.Page()
	if p == nil || len(p.Lines) != 2 || p.Lines[1].Row != 17 || p.Lines[1].Col != 3 || p.endCol != 5 {
		t.Fatalf("piece after page: %+v end=%d", p, p.endCol)
	}
	// With no page of ours (the name before it was not translated) the
	// piece shows the original and masks nothing.
	w = NewEclTextWatcher(eclFixture(t, " ATTACKS.", "發動攻擊。", "GO TO THE BRIDGE", "前往艦橋"))
	w.ObserveEntry(eclEntry("CELESTE", true, 1, 17))
	w.ObserveEntry(eclEntry(" ATTACKS.", false, 8, 17))
	if w.Page() != nil || w.Stats.Misses != 2 {
		t.Fatalf("short piece without page drew: %+v misses=%d", w.Page(), w.Stats.Misses)
	}
	// A long string in the same situation keeps the old behaviour.
	w.ObserveEntry(eclEntry("GO TO THE BRIDGE", false, 17, 17))
	if p := w.Page(); p == nil || p.Lines[0].Col != 17 {
		t.Fatalf("long string without page: %+v", p)
	}
	// A fresh page (clear flag) takes a short piece normally.
	w = NewEclTextWatcher(eclFixture(t, "LAUNCH...", "發射……"))
	w.ObserveEntry(eclEntry("LAUNCH...", true, 5, 17))
	if p := w.Page(); p == nil || string(p.Lines[0].Text) != "發射……" {
		t.Fatalf("fresh short piece: %+v", p)
	}
}
