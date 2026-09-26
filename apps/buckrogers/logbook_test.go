package buckrogers

import (
	"strings"
	"testing"
)

func TestLogbookLayoutAndPanel(t *testing.T) {
	long := strings.Repeat("字", 36*19) + `\n` + "第二頁。"
	c, err := LoadLogbookCatalog([]byte("key\ttranslation\tsource\nlogbook.5\t" + long + "\tecl-batch-editorial\nlogbook.5.title\t警報\tecl-batch-editorial\n"))
	if err != nil {
		t.Fatal(err)
	}
	if got := len(c.entries[5].Pages); got != 2 {
		t.Fatalf("pages %d", got)
	}
	if _, err := LoadLogbookCatalog([]byte("key\ttranslation\tsource\nlogbook.6\t" + strings.Repeat("字", 36*19*3+1) + "\tx\n")); err == nil {
		t.Fatal("4-page entry accepted")
	}
	w := NewLogbookWatcher(c, nil)
	w.ObserveEntry([]byte(" and you record THE LOG as logbook entry  5."), 1, 17, 38, 22)
	if n, _ := w.Open(); n != 5 {
		t.Fatalf("open %d", n)
	}
	if !w.Turn(1) || !w.Turn(1) {
		t.Fatal("turn not taken")
	}
	if _, p := w.Open(); p != 1 {
		t.Fatalf("page %d", p)
	}
	// A write only to the top-left cell does not close it.
	w.ObserveVideoWrite(0x0CF4, 0x1B3A, 17*8*320+8)
	w.ObserveVideoWrite(0x0763, 0x184D, 22*8*320+38*8)
	if n, _ := w.Open(); n != 5 {
		t.Fatal("unrelated writes closed panel")
	}
	w.ObserveVideoWrite(0x0CF4, 0x1B3A, 22*8*320+38*8)
	if n, _ := w.Open(); n != 0 {
		t.Fatal("whole-window clear kept panel")
	}
	// Next printer entry closes; unknown or out-of-range numbers never open.
	w.ObserveEntry([]byte(" as logbook entry 5."), 1, 17, 38, 22)
	w.ObserveEntry([]byte("NEXT TEXT"), 1, 17, 38, 22)
	if n, _ := w.Open(); n != 0 || w.Turn(1) {
		t.Fatal("next entry kept panel")
	}
	w.ObserveEntry([]byte(" as logbook entry 99."), 1, 17, 38, 22)
	w.ObserveEntry([]byte(" as logbook entry 7."), 1, 17, 38, 22)
	if n, _ := w.Open(); n != 0 {
		t.Fatal("missing entry opened")
	}
}
