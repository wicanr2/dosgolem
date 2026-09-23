package buckrogers

import (
	"crypto/sha256"
	"os"
	"path/filepath"
	"testing"

	"github.com/wicanr2/dosgolem/internal/machine"
)

func postJoinTestCatalog() *PostJoinMenuCatalog {
	m := map[menuIdentity]catalogEntry{}
	v := map[menuIdentity]postJoinVariant{}
	for i, row := range []uint8{13, 14, 15, 16, 18, 19, 20} {
		for j, caller := range []Address{{0x37f1, 0x15bd}, {0x37f1, 0x1856}, {0x37f1, 0x175d}} {
			bg, fg := uint8(0), uint8(10)
			if caller.Offset == 0x175d {
				bg, fg = 15, 0
			}
			id := menuIdentity{uint8(10 + i), sha256.Sum256([]byte{byte(i)}), caller, bg, fg, row, 9}
			m[id] = catalogEntry{eventKey: postJoinKeys[i], textKey: "k", translation: "甲"}
			v[id] = postJoinVariant{postJoinKeys[i], []string{"initial_normal", "normal_redraw", "selected"}[j], id}
		}
	}
	return &PostJoinMenuCatalog{base: &MenuCatalog{byIdentity: m}, variants: v}
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
	w.Prewrite(machine.VideoWrite{CS: 0x0763, IP: 0x184d, Offset: 13*8*320 + 72, Value: 0})
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
func TestPostJoinInitialAndSixDownPairsThenRow21FailClosed(t *testing.T) {
	w, _ := NewPostJoinMenuWatcher(postJoinTestCatalog())
	for i := 0; i < 7; i++ {
		completePostJoin(t, w, postJoinEvent(i, Address{0x37f1, 0x15bd}, uint64(i+1)))
	}
	completePostJoin(t, w, postJoinEvent(0, Address{0x37f1, 0x175d}, 8))
	for i := 0; i < 6; i++ {
		e := postJoinEvent(i, Address{0x37f1, 0x1856}, uint64(9+i*2))
		if err := w.ObserveEntry(e); err != nil {
			t.Fatal(err)
		}
		w.Prewrite(machine.VideoWrite{CS: 0x0763, IP: 0x184d, Offset: uint32([]int{13, 14, 15, 16, 18, 19}[i]*8*320 + 72), Value: 0})
		e.PostCallStep = e.EntryStep + 1
		if err := w.ObserveReturn(e); err != nil {
			t.Fatal(err)
		}
		completePostJoin(t, w, postJoinEvent(i+1, Address{0x37f1, 0x175d}, uint64(10+i*2)))
	}
	if got := len(w.Generations()); got != 7 {
		t.Fatalf("generation count=%d", got)
	}
	e := postJoinEvent(6, Address{0x37f1, 0x1856}, 30)
	if err := w.ObserveEntry(e); err != nil {
		t.Fatal(err)
	}
	w.Prewrite(machine.VideoWrite{CS: 0x0763, IP: 0x184d, Offset: 20*8*320 + 72})
	e.PostCallStep = 31
	if err := w.ObserveReturn(e); err != nil {
		t.Fatal(err)
	}
	unknown := postJoinEvent(6, Address{0x37f1, 0x175d}, 32)
	unknown.Row = 21
	if err := w.ObserveEntry(unknown); err == nil || !w.Failed() {
		t.Fatal("unknown row21 accepted")
	}
}
func TestPostJoinWatcherUnknownRowAndDiscontinuityFailClosed(t *testing.T) {
	w, _ := NewPostJoinMenuWatcher(postJoinTestCatalog())
	w.Prewrite(machine.VideoWrite{CS: 0x0763, IP: 0x184d, Offset: 13*8*320 + 72})
	if !w.Failed() {
		t.Fatal("unknown writer did not fail")
	}
	w, _ = NewPostJoinMenuWatcher(postJoinTestCatalog())
	w.ObserveDiscontinuity()
	if !w.Failed() || w.Active() {
		t.Fatal("discontinuity did not clear/fail")
	}
}
func TestPostJoinPendingUnknownWriterFailsClosed(t *testing.T) {
	w, _ := NewPostJoinMenuWatcher(postJoinTestCatalog())
	e := postJoinEvent(0, Address{0x37f1, 0x15bd}, 1)
	if err := w.ObserveEntry(e); err != nil {
		t.Fatal(err)
	}
	w.Prewrite(machine.VideoWrite{CS: 0x1234, IP: 0xbeef, Offset: 13*8*320 + 72})
	if !w.Failed() {
		t.Fatal("pending unknown writer accepted")
	}
}
func TestPostJoinFixtureHashFailsClosed(t *testing.T) {
	if _, err := LoadPostJoinMenuCatalog([]byte("bad"), []byte("bad"), []byte("bad")); err == nil {
		t.Fatal("mutated READY fixture accepted")
	}
}

// The project mounts its approved non-original TSV fixtures here in the Docker
// receipt/test command.  A developer without that mount gets no false fixture
// substitute; CI/receipt must set POST_JOIN_READY_FIXTURES.
func TestPostJoinRealReadyFixtures(t *testing.T) {
	d := os.Getenv("POST_JOIN_READY_FIXTURES")
	if d == "" {
		t.Skip("requires Docker-mounted READY fixtures")
	}
	read := func(n string) []byte {
		b, e := os.ReadFile(filepath.Join(d, n))
		if e != nil {
			t.Fatal(e)
		}
		return b
	}
	if _, err := LoadPostJoinMenuCatalog(read("post-join-menu-events.tsv"), read("post-join-menu-variants.tsv"), read("post-join-menu.zh-TW.tsv")); err != nil {
		t.Fatal(err)
	}
}
