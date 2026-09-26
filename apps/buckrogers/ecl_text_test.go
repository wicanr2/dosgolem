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
	if w.Page() != nil {
		t.Fatal("post-call write inside window kept page")
	}
	// Untranslated keys are misses.
	w.ObserveEntry(eclEntry("HELLO THERE", true, 1, 17))
	w.ObserveInstruction(Address{0x2E13, 0x0B79}, 0x1841, 0x3D00+eclTextReturnDelta)
	w.ObserveEntry(eclEntry("UNKNOWN", false, 5, 17))
	if w.Page() != nil || w.Stats.Misses != 1 {
		t.Fatal("miss kept page")
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
	w := NewEclTextWatcher(eclFixture(t, "A B", "", "C D", "丙丁"))
	w.ObserveEntry(eclEntry("A B", true, 1, 17))
	w.ObserveEntry(eclEntry("C D", false, 1, 21))
	p := w.Page()
	if p == nil || p.Top != 21 || p.Lines[0].Row != 21 || p.Lines[0].Col != 1 {
		t.Fatalf("continuation after miss must start at the original cursor: %+v", p)
	}
}
