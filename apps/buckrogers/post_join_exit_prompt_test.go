package buckrogers

import (
	"bytes"
	"crypto/sha256"
	"strings"
	"testing"

	"github.com/wicanr2/dosgolem/internal/machine"
	"github.com/wicanr2/dosgolem/xlate"
)

func exitPromptFixture() (*PostJoinExitPromptCatalog, TextEvent, TextEvent) {
	q1 := TextEvent{EntryStep: 124811496, Caller: Address{0x37F1, 0x101E}, OriginalLength: 12, OriginalSHA256: sha256.Sum256([]byte("q1")), Background: 0, Foreground: 14, Row: 24, Column: 0}
	q2 := q1
	q2.EntryStep = 124906582
	q2.OriginalLength = 30
	q2.OriginalSHA256 = sha256.Sum256([]byte("q2"))
	c := &PostJoinExitPromptCatalog{byIdentity: map[menuIdentity]postJoinExitEntry{
		skillExitIdentity(q1): {postJoinExitQ1, "甲乙丙丁", skillExitIdentity(q1), 124811496, 124820881, 96},
		skillExitIdentity(q2): {postJoinExitQ2, "甲乙丙丁", skillExitIdentity(q2), 124906582, 124929724, 240},
	}}
	return c, q1, q2
}
func exitPromptFont() *xlate.Font {
	return &xlate.Font{W: 16, H: 16, Glyphs: map[rune][]byte{'甲': bytes.Repeat([]byte{0xff}, 32), '乙': bytes.Repeat([]byte{0xff}, 32), '丙': bytes.Repeat([]byte{0xff}, 32), '丁': bytes.Repeat([]byte{0xff}, 32)}}
}
func exitPromptReturn(e TextEvent, step uint64) TextEvent { e.PostCallStep = step; return e }
func exitPromptWrite(offset uint32) machine.VideoWrite {
	return machine.VideoWrite{Step: 124906844, Offset: offset, CS: 0x0763, IP: 0x184D, Value: 0}
}

func TestPostJoinExitPromptCatalogExactRows(t *testing.T) {
	events := strings.Join([]string{
		strings.Join(skillExitEventHeader, "\t"),
		postJoinExitQ1 + "\t1\texit_confirmation_prompt\tconfirmed\t124811496\t124820881\t12\tc38a515358859a10e7a2104cab69fe10ee2d492d94024b7d8f17d69b1a409032\t37F1:101E\t0\t14\t24\t0",
		postJoinExitQ2 + "\t2\texit_confirmation_prompt\tconfirmed\t124906582\t124929724\t30\t35023ac3208312fb1c932ec15a737aae88817925d6cabb26d83281bf755bdcb8\t37F1:101E\t0\t14\t24\t0",
	}, "\n") + "\n"
	texts := "key\ttranslation\tsource\n" + postJoinExitQ1 + "\t離開至 DOS\truntime-interface\n" + postJoinExitQ2 + "\t遊戲尚未儲存。仍要離開？\truntime-interface\n"
	if _, err := LoadPostJoinExitPromptCatalog([]byte(events), []byte(texts)); err != nil {
		t.Fatal(err)
	}
	for _, bad := range []string{
		strings.Replace(events, "\t14\t24\t0", "\t13\t24\t0", 1),
		strings.Replace(events, "124906582", "124906583", 1),
		strings.Replace(events, "\tconfirmed\t", "\thypothesis\t", 1),
	} {
		if _, err := LoadPostJoinExitPromptCatalog([]byte(bad), []byte(texts)); err == nil {
			t.Fatal("drifted event accepted")
		}
	}
	if _, err := LoadPostJoinExitPromptCatalog([]byte(events), []byte(strings.Replace(texts, "runtime-interface", "semantic", 1))); err == nil {
		t.Fatal("semantic translation accepted")
	}
}

func TestPostJoinExitPromptQ2PendingOverlapsQ1AndSameValueClears(t *testing.T) {
	c, q1, q2 := exitPromptFixture()
	o, e := NewPostJoinExitPromptOwner(c, exitPromptFont(), 2)
	if e != nil {
		t.Fatal(e)
	}
	p := [256][3]uint8{}
	p[14] = [3]uint8{255, 255, 0}
	if e = o.ObserveEntry(q1); e != nil {
		t.Fatal(e)
	}
	if o.Watcher.Active() {
		t.Fatal("entry must remain pending")
	}
	if e = o.ObserveReturn(exitPromptReturn(q1, 124820881), p); e != nil {
		t.Fatal(e)
	}
	if e = o.ObserveEntry(q2); e != nil {
		t.Fatal(e)
	}
	if !o.Watcher.Pending() || !o.Watcher.Active() {
		t.Fatal("q2 pending must overlap q1 active")
	}
	if e = o.Prewrite(exitPromptWrite(0xF000)); e != nil {
		t.Fatal(e)
	}
	if o.Watcher.Active() || !o.Watcher.Pending() || len(o.Presenter.ActiveKeys()) != 0 {
		t.Fatal("q1 clear must preserve q2 pending")
	}
	if e = o.ObserveReturn(exitPromptReturn(q2, 124929724), p); e != nil {
		t.Fatal(e)
	}
	if !o.Watcher.Active() || o.Presenter.ActiveKeys()[0] != postJoinExitQ2 {
		t.Fatal("q2 did not activate")
	}
	o.Stop()
	if !o.Watcher.Closed() || o.Watcher.Active() || o.Watcher.Pending() || len(o.Presenter.ActiveKeys()) != 0 {
		t.Fatal("Stop did not clear")
	}
	if e = o.ObserveEntry(q1); e == nil {
		t.Fatal("closed owner rearmed")
	}
}

func TestPostJoinExitPromptQ2Pending1854ClearsQ1Only(t *testing.T) {
	c, q1, q2 := exitPromptFixture()
	o, e := NewPostJoinExitPromptOwner(c, exitPromptFont(), 2)
	if e != nil {
		t.Fatal(e)
	}
	p := [256][3]uint8{}
	p[14] = [3]uint8{255, 255, 0}
	if e = o.ObserveEntry(q1); e != nil {
		t.Fatal(e)
	}
	if e = o.ObserveReturn(exitPromptReturn(q1, 124820881), p); e != nil {
		t.Fatal(e)
	}
	if e = o.ObserveEntry(q2); e != nil {
		t.Fatal(e)
	}
	// q2's reviewed lower-row glyph write intersects the still-active q1 body.
	if e = o.Prewrite(machine.VideoWrite{Step: 124906941, Offset: 0xF142, CS: 0x0763, IP: 0x1854}); e != nil {
		t.Fatal(e)
	}
	if o.Watcher.Active() || !o.Watcher.Pending() || len(o.Presenter.ActiveKeys()) != 0 {
		t.Fatal("q2 pending 1854 must clear q1 and retain q2 pending")
	}
	if e = o.ObserveReturn(exitPromptReturn(q2, 124929724), p); e != nil {
		t.Fatal(e)
	}
	if !o.Watcher.Active() {
		t.Fatal("q2 did not activate")
	}
}
func TestPostJoinExitPromptOrderAndWriterFailClosed(t *testing.T) {
	c, q1, q2 := exitPromptFixture()
	for _, bad := range []TextEvent{q2, func() TextEvent { v := q1; v.Foreground = 13; return v }()} {
		w, _ := NewPostJoinExitPromptWatcher(c)
		if w.ObserveEntry(bad) == nil || !w.Failed() {
			t.Fatal("invalid entry accepted")
		}
	}
	w, _ := NewPostJoinExitPromptWatcher(c)
	w.ObserveEntry(q1)
	if _, e := w.ObserveReturn(exitPromptReturn(q2, 124929724)); e == nil || !w.Failed() {
		t.Fatal("mismatched return accepted")
	}
	w, _ = NewPostJoinExitPromptWatcher(c)
	w.ObserveEntry(q1)
	if _, e := w.Prewrite(machine.VideoWrite{Offset: 0x10000}); e == nil || !w.Failed() {
		t.Fatal("invalid offset accepted")
	}
	w, _ = NewPostJoinExitPromptWatcher(c)
	w.ObserveEntry(q1)
	if _, e := w.Prewrite(machine.VideoWrite{Offset: 0xF000, CS: 0x1111, IP: 0x2222}); e == nil || !w.Failed() {
		t.Fatal("unknown writer accepted")
	}
	w, _ = NewPostJoinExitPromptWatcher(c)
	w.ObserveEntry(q1)
	w.ObserveReturn(exitPromptReturn(q1, 124820881))
	w.ObserveEntry(q2)
	if _, e := w.ObserveReturn(exitPromptReturn(q2, 124929724)); e == nil || !w.Failed() {
		t.Fatal("q2 return before q1 clear accepted")
	}
}

func TestPostJoinExitPromptDormantIgnoresEarlierUnrelatedRow24(t *testing.T) {
	c, q1, _ := exitPromptFixture()
	w, _ := NewPostJoinExitPromptWatcher(c)
	other := TextEvent{EntryStep: 123151198, Caller: Address{0x37F1, 0x101E}, OriginalLength: 18, Background: 0, Foreground: 14, Row: 24, Column: 0}
	other.OriginalSHA256, _ = menuHash("925f9d44689040dbbecd13614cbe9c786da2e278dfeec900b17c19de3b08b572")
	if w.ShouldObserveEntry(other) {
		t.Fatal("earlier non-Exit prompt routed into dormant owner")
	}
	if !w.ShouldObserveEntry(q1) {
		t.Fatal("exact q1 not routed")
	}
	if e := w.ObserveEntry(q1); e != nil {
		t.Fatal(e)
	}
	if !w.ShouldObserveEntry(other) {
		t.Fatal("unknown event while pending must reach poison path")
	}
	if e := w.ObserveEntry(other); e == nil || !w.Failed() {
		t.Fatal("unknown event did not poison pending owner")
	}
}

func TestPostJoinExitPromptNReturnIgnoresUnrelatedMagentaEntry(t *testing.T) {
	c, q1, _ := exitPromptFixture()
	w, _ := NewPostJoinExitPromptWatcher(c)
	if e := w.ObserveEntry(q1); e != nil {
		t.Fatal(e)
	}
	if _, e := w.ObserveReturn(exitPromptReturn(q1, 124820881)); e != nil {
		t.Fatal(e)
	}
	other := TextEvent{EntryStep: 125250877, Caller: Address{0x37F1, 0x101E}, OriginalLength: 18, Background: 0, Foreground: 13, Row: 24, Column: 0}
	other.OriginalSHA256, _ = menuHash("925f9d44689040dbbecd13614cbe9c786da2e278dfeec900b17c19de3b08b572")
	if w.ShouldObserveEntry(other) {
		t.Fatal("normal N-return magenta message routed to Exit owner")
	}
	if !w.Active() {
		t.Fatal("q1 cleared before first intersecting A000 prewrite")
	}
	partial := q1
	partial.Foreground = 13
	if !w.ShouldObserveEntry(partial) {
		t.Fatal("q1 same bytes with partial style must be checked")
	}
	if e := w.ObserveEntry(partial); e == nil || !w.Failed() {
		t.Fatal("partial q1 did not poison")
	}
}

func TestPostJoinExitPromptRouterKeepsPartialCandidates(t *testing.T) {
	c, q1, q2 := exitPromptFixture()
	w, _ := NewPostJoinExitPromptWatcher(c)
	partial := q1
	partial.Foreground = 13
	if !w.ShouldObserveEntry(partial) {
		t.Fatal("dormant q1 bytes with wrong style bypassed exact guard")
	}
	if e := w.ObserveEntry(partial); e == nil || !w.Failed() {
		t.Fatal("partial q1 accepted")
	}
	w, _ = NewPostJoinExitPromptWatcher(c)
	w.ObserveEntry(q1)
	w.ObserveReturn(exitPromptReturn(q1, 124820881))
	partial = q2
	partial.Foreground = 13
	if !w.ShouldObserveEntry(partial) {
		t.Fatal("q2 bytes with wrong style bypassed exact guard")
	}
	if e := w.ObserveEntry(partial); e == nil || !w.Failed() {
		t.Fatal("partial q2 accepted")
	}
}

func TestPostJoinExitPromptOwnerUnknownActiveWriterClearsPresenter(t *testing.T) {
	c, q1, _ := exitPromptFixture()
	o, e := NewPostJoinExitPromptOwner(c, exitPromptFont(), 2)
	if e != nil {
		t.Fatal(e)
	}
	p := [256][3]uint8{}
	p[14] = [3]uint8{255, 255, 0}
	if e = o.ObserveEntry(q1); e != nil {
		t.Fatal(e)
	}
	if e = o.ObserveReturn(exitPromptReturn(q1, 124820881), p); e != nil {
		t.Fatal(e)
	}
	if e = o.Prewrite(machine.VideoWrite{Step: 125251139, Offset: 0xF000, CS: 0x1111, IP: 0x2222}); e == nil {
		t.Fatal("unknown active writer accepted")
	}
	if !o.Watcher.Failed() || o.Watcher.Active() || len(o.Presenter.ActiveKeys()) != 0 {
		t.Fatal("owner did not clear both layers")
	}
}

func TestPostJoinExitPromptEveryLowerScanlinePrewrite(t *testing.T) {
	c, q1, q2 := exitPromptFixture()
	p := [256][3]uint8{}
	p[14] = [3]uint8{255, 255, 0}
	for _, tc := range []struct {
		key   string
		width uint32
	}{{postJoinExitQ1, 96}, {postJoinExitQ2, 240}} {
		for row := uint32(193); row < 200; row++ {
			o, e := NewPostJoinExitPromptOwner(c, exitPromptFont(), 2)
			if e != nil {
				t.Fatal(e)
			}
			if e = o.ObserveEntry(q1); e != nil {
				t.Fatal(e)
			}
			if e = o.ObserveReturn(exitPromptReturn(q1, 124820881), p); e != nil {
				t.Fatal(e)
			}
			if tc.key == postJoinExitQ2 {
				if e = o.ObserveEntry(q2); e != nil {
					t.Fatal(e)
				}
				if e = o.Prewrite(exitPromptWrite(0xF000)); e != nil {
					t.Fatal(e)
				}
				if e = o.ObserveReturn(exitPromptReturn(q2, 124929724), p); e != nil {
					t.Fatal(e)
				}
			}
			// The first suffix pixel and previous scanline are outside the body.
			if e = o.Prewrite(exitPromptWrite(row*320 + tc.width)); e != nil {
				t.Fatal(e)
			}
			if !o.Watcher.Active() {
				t.Fatalf("%s row %d suffix cleared active", tc.key, row)
			}
			if e = o.Prewrite(exitPromptWrite(row*320 + tc.width - 1)); e != nil {
				t.Fatal(e)
			}
			if o.Watcher.Active() || len(o.Presenter.ActiveKeys()) != 0 {
				t.Fatalf("%s row %d body same-value write did not clear", tc.key, row)
			}
		}
	}
}

func TestPostJoinExitPromptPendingLowerScanlineWriterWindow(t *testing.T) {
	c, q1, _ := exitPromptFixture()
	o, e := NewPostJoinExitPromptOwner(c, exitPromptFont(), 2)
	if e != nil {
		t.Fatal(e)
	}
	if e = o.ObserveEntry(q1); e != nil {
		t.Fatal(e)
	}
	// Phase 217 proves this exact q1 pending lower-scanline writer.
	if e = o.Prewrite(machine.VideoWrite{Step: 124811855, Offset: 0xF142, CS: 0x0763, IP: 0x1854, Value: 0}); e != nil {
		t.Fatal(e)
	}
	if !o.Watcher.Pending() || o.Watcher.Active() || o.Watcher.Failed() {
		t.Fatal("reviewed pending writer changed lifecycle")
	}
	for _, bad := range []machine.VideoWrite{
		{Step: 124811855, Offset: 0xF142, CS: 0x0763, IP: 0x1855},
		{Step: 124820881, Offset: 0xF142, CS: 0x0763, IP: 0x1854},
	} {
		other, _ := NewPostJoinExitPromptOwner(c, exitPromptFont(), 2)
		other.ObserveEntry(q1)
		if e = other.Prewrite(bad); e == nil || !other.Watcher.Failed() || other.Watcher.Pending() || len(other.Presenter.ActiveKeys()) != 0 {
			t.Fatal("unreviewed writer/window did not fail closed")
		}
	}
}
func TestPostJoinExitPromptLifecycleAndGeneration(t *testing.T) {
	c, q1, _ := exitPromptFixture()
	for _, f := range []func(*PostJoinExitPromptWatcher){(*PostJoinExitPromptWatcher).Stop, (*PostJoinExitPromptWatcher).Restore, (*PostJoinExitPromptWatcher).Discontinuity, (*PostJoinExitPromptWatcher).Fault} {
		w, _ := NewPostJoinExitPromptWatcher(c)
		w.ObserveEntry(q1)
		f(w)
		if w.Pending() || w.Active() {
			t.Fatal("pending survived lifecycle")
		}
		if w.state == postJoinExitPromptOpen {
			if _, e := w.ObserveReturn(exitPromptReturn(q1, 124820881)); e == nil {
				t.Fatal("old return survived Restore")
			}
			continue
		}
		if e := w.ObserveEntry(q1); e == nil {
			t.Fatal("terminal owner rearmed")
		}
	}
	w, _ := NewPostJoinExitPromptWatcher(c)
	w.ObserveEntry(q1)
	g, _ := w.ObserveReturn(exitPromptReturn(q1, 124820881))
	w.Restore()
	w.ObserveEntry(q1)
	next, _ := w.ObserveReturn(exitPromptReturn(q1, 124820881))
	if next.Generation <= g.Generation {
		t.Fatal("generation did not increase")
	}
}
func TestPostJoinExitPromptTailSentinelTwoAndThreeX(t *testing.T) {
	for _, scale := range []int{2, 3} {
		o, _ := NewRuntimePostJoinExitPromptOverlay(exitPromptFont(), scale)
		p := [256][3]uint8{}
		p[14] = [3]uint8{255, 255, 0}
		indexed := make([]byte, 320*200)
		base := ScaleIndexedRGBA(indexed, p, scale)
		for _, tc := range []struct {
			key   string
			width uint32
		}{{postJoinExitQ1, 96}, {postJoinExitQ2, 240}} {
			if e := o.Apply(PostJoinExitPromptGeneration{1, tc.key, "甲乙丙丁", tc.width}, p); e != nil {
				t.Fatal(e)
			}
			got, missing, drew, e := o.Draw(indexed, p)
			if e != nil || len(missing) != 0 || !drew {
				t.Fatalf("scale %d draw %v", scale, e)
			}
			for y := 192 * scale; y < 200*scale; y++ {
				a := (y*320*scale + int(tc.width)*scale) * 4
				b := (y*320*scale + int(tc.width+48)*scale) * 4
				if !bytes.Equal(got[a:b], base[a:b]) {
					t.Fatal("protected suffix changed")
				}
			}
			o.Clear()
			got, _, _, e = o.Draw(indexed, p)
			if e != nil || !bytes.Equal(got, base) {
				t.Fatal("clear left pixels")
			}
		}
	}
}
