package buckrogers

import (
	"crypto/sha256"
	"testing"

	"github.com/wicanr2/dosgolem/internal/machine"
)

func postJoinTestCatalog() *PostJoinMenuCatalog {
	m := map[menuIdentity]catalogEntry{}
	for i, row := range []uint8{13, 14, 15, 16, 18, 19, 20} {
		for _, caller := range []Address{{0x37f1, 0x15bd}, {0x37f1, 0x1856}, {0x37f1, 0x175d}} {
			bg, fg := uint8(0), uint8(10)
			if caller.Offset == 0x175d {
				bg, fg = 15, 0
			}
			m[menuIdentity{uint8(10 + i), sha256.Sum256([]byte{byte(i)}), caller, bg, fg, row, 9}] = catalogEntry{eventKey: postJoinKeys[i], textKey: "k", translation: "甲"}
		}
	}
	return &PostJoinMenuCatalog{base: &MenuCatalog{byIdentity: m}}
}
func postJoinEvent(i int, caller Address, step uint64) TextEvent {
	bg, fg := uint8(0), uint8(10)
	if caller.Offset == 0x175d {
		bg, fg = 15, 0
	}
	return TextEvent{EntryStep: step, Caller: caller, OriginalLength: uint8(10 + i), OriginalSHA256: sha256.Sum256([]byte{byte(i)}), Background: bg, Foreground: fg, Row: []uint8{13, 14, 15, 16, 18, 19, 20}[i], Column: 9}
}
func completePostJoin(t *testing.T, w *PostJoinMenuWatcher, e TextEvent) {
	t.Helper()
	if err := w.ObserveEntry(e); err != nil {
		t.Fatal(err)
	}
	e.PostCallStep = e.EntryStep + 1
	if err := w.ObserveReturn(e); err != nil {
		t.Fatal(err)
	}
}

func TestPostJoinWatcherInvalidatesSameValueBeforeRedraw(t *testing.T) {
	w, _ := NewPostJoinMenuWatcher(postJoinTestCatalog())
	for i := 0; i < 7; i++ {
		completePostJoin(t, w, postJoinEvent(i, Address{0x37f1, 0x15bd}, uint64(i+1)))
	}
	completePostJoin(t, w, postJoinEvent(0, Address{0x37f1, 0x175d}, 8))
	if !w.Active() {
		t.Fatal("initial generation not active")
	}
	e := postJoinEvent(0, Address{0x37f1, 0x1856}, 9)
	if err := w.ObserveEntry(e); err != nil {
		t.Fatal(err)
	}
	w.Prewrite(machine.VideoWrite{Offset: 13*8*320 + 72, Value: 0})
	if w.Active() {
		t.Fatal("same-value prewrite retained active overlay")
	}
	e.PostCallStep = 10
	if err := w.ObserveReturn(e); err != nil {
		t.Fatal(err)
	}
	completePostJoin(t, w, postJoinEvent(1, Address{0x37f1, 0x175d}, 11))
	if got := len(w.Generations()); got != 2 {
		t.Fatalf("generations=%d", got)
	}
}
func TestPostJoinWatcherUnknownRowAndDiscontinuityFailClosed(t *testing.T) {
	w, _ := NewPostJoinMenuWatcher(postJoinTestCatalog())
	w.Prewrite(machine.VideoWrite{Offset: 13*8*320 + 72})
	if !w.Failed() {
		t.Fatal("unknown writer did not fail")
	}
	w, _ = NewPostJoinMenuWatcher(postJoinTestCatalog())
	w.ObserveDiscontinuity()
	if !w.Failed() || w.Active() {
		t.Fatal("discontinuity did not clear/fail")
	}
}
func TestPostJoinFixtureHashFailsClosed(t *testing.T) {
	if _, err := LoadPostJoinMenuCatalog([]byte("bad"), []byte("bad"), []byte("bad")); err == nil {
		t.Fatal("mutated READY fixture accepted")
	}
}
