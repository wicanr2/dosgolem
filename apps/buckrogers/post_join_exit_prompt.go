package buckrogers

// Output-only Exit question identity and lifecycle. Project
// docs/spec/021-post-join-exit-prompt-body-only-draft.md and
// docs/re/phase-215-exit-prompt-body-ready-review.md authorize only the two
// row-24 bodies; the original suffix stays.

import (
	"fmt"
	"strconv"
	"unicode"

	"github.com/wicanr2/dosgolem/internal/machine"
)

const (
	postJoinExitQ1 = "post_join_exit.prompt.quit_to_dos.001"
	postJoinExitQ2 = "post_join_exit.prompt.unsaved_quit_anyway.001"
)

type postJoinExitEntry struct {
	key, text             string
	id                    menuIdentity
	entryStep, returnStep uint64
	width                 uint32
}

type PostJoinExitPromptCatalog struct {
	byIdentity map[menuIdentity]postJoinExitEntry
}

// LoadPostJoinExitPromptCatalog accepts only the two reviewed exact identities.
// The TSV text is presentation data and never flows into the DOS machine.
func LoadPostJoinExitPromptCatalog(events, translations []byte) (*PostJoinExitPromptCatalog, error) {
	tr, err := readTSV("post-join-exit-prompt.zh-TW.tsv", translations, textHeader)
	if err != nil {
		return nil, err
	}
	texts := map[string]string{}
	for _, r := range tr {
		if r[2] != "runtime-interface" || (r[0] != postJoinExitQ1 && r[0] != postJoinExitQ2) || texts[r[0]] != "" || r[1] == "" {
			return nil, fmt.Errorf("Exit prompt translation key or role invalid")
		}
		for _, ch := range r[1] {
			if unicode.IsControl(ch) {
				return nil, fmt.Errorf("Exit prompt control character")
			}
		}
		texts[r[0]] = r[1]
	}
	if len(texts) != 2 {
		return nil, fmt.Errorf("Exit prompt translations incomplete")
	}
	rows, err := readTSV("post-join-exit-prompt-events.tsv", events, skillExitEventHeader)
	if err != nil {
		return nil, err
	}
	if len(rows) != 2 {
		return nil, fmt.Errorf("Exit prompt events must contain exactly two rows")
	}
	wants := []struct {
		key, hash  string
		length     uint8
		entry, ret uint64
		width      uint32
	}{
		{postJoinExitQ1, "c38a515358859a10e7a2104cab69fe10ee2d492d94024b7d8f17d69b1a409032", 12, 124811496, 124820881, 96},
		{postJoinExitQ2, "35023ac3208312fb1c932ec15a737aae88817925d6cabb26d83281bf755bdcb8", 30, 124906582, 124929724, 240},
	}
	out := &PostJoinExitPromptCatalog{byIdentity: map[menuIdentity]postJoinExitEntry{}}
	for i, r := range rows {
		want := wants[i]
		if r[0] != want.key || r[1] != strconv.Itoa(i+1) || r[2] != skillExitRole || r[3] != "confirmed" || r[7] != want.hash {
			return nil, fmt.Errorf("Exit prompt event order or evidence invalid")
		}
		entry, e1 := strconv.ParseUint(r[4], 10, 64)
		ret, e2 := strconv.ParseUint(r[5], 10, 64)
		length, e3 := menuByte(r[6])
		hash, e4 := menuHash(r[7])
		caller, e5 := menuAddress(r[8])
		var v [4]uint8
		var e6 error
		for j := range v {
			v[j], e6 = menuByte(r[9+j])
			if e6 != nil {
				break
			}
		}
		if e1 != nil || e2 != nil || e3 != nil || e4 != nil || e5 != nil || e6 != nil || entry != want.entry || ret != want.ret || length != want.length || caller != (Address{0x37F1, 0x101E}) || v != [4]uint8{0, 14, 24, 0} {
			return nil, fmt.Errorf("Exit prompt canonical identity drift")
		}
		id := menuIdentity{length, hash, caller, v[0], v[1], v[2], v[3]}
		if _, exists := out.byIdentity[id]; exists {
			return nil, fmt.Errorf("Exit prompt duplicate identity")
		}
		out.byIdentity[id] = postJoinExitEntry{want.key, texts[want.key], id, entry, ret, want.width}
	}
	return out, nil
}

type PostJoinExitPromptGeneration struct {
	Generation            uint64
	EventKey, Translation string
	Width                 uint32
}
type postJoinExitPromptState uint8

const (
	postJoinExitPromptOpen postJoinExitPromptState = iota
	postJoinExitPromptClosed
	postJoinExitPromptFailed
)

type PostJoinExitPromptWatcher struct {
	c            *PostJoinExitPromptCatalog
	pending      *postJoinExitEntry
	pendingEvent TextEvent
	active       *PostJoinExitPromptGeneration
	generation   uint64
	q1Accepted   bool
	state        postJoinExitPromptState
}

// PostJoinExitPromptSnapshot exposes only lifecycle state, never original text.
type PostJoinExitPromptSnapshot struct {
	Q1Active  bool `json:"q1_active"`
	Q2Pending bool `json:"q2_pending"`
	Q2Active  bool `json:"q2_active"`
	Closed    bool `json:"closed"`
	Failed    bool `json:"failed"`
}

func (w *PostJoinExitPromptWatcher) Snapshot() PostJoinExitPromptSnapshot {
	if w == nil {
		return PostJoinExitPromptSnapshot{Failed: true}
	}
	return PostJoinExitPromptSnapshot{
		Q1Active:  w.Active() && w.active.EventKey == postJoinExitQ1,
		Q2Pending: w.Pending() && w.pending.key == postJoinExitQ2,
		Q2Active:  w.Active() && w.active.EventKey == postJoinExitQ2,
		Closed:    w.Closed(), Failed: w.Failed(),
	}
}

func postJoinExitBodyIntersects(local, width uint32) bool {
	if local >= 320*200 || width == 0 {
		return false
	}
	x, y := local%320, local/320
	return y >= 192 && y < 200 && x < width
}

func NewPostJoinExitPromptWatcher(c *PostJoinExitPromptCatalog) (*PostJoinExitPromptWatcher, error) {
	if c == nil || len(c.byIdentity) != 2 {
		return nil, fmt.Errorf("Exit prompt catalog invalid")
	}
	return &PostJoinExitPromptWatcher{c: c}, nil
}
func (w *PostJoinExitPromptWatcher) fail(why string) error {
	if w != nil {
		w.pending = nil
		w.active = nil
		w.state = postJoinExitPromptFailed
	}
	return fmt.Errorf("Exit prompt fail-closed: %s", why)
}

// ShouldObserveEntry routes only the Exit candidate family. Other row-24
// messages can coexist with q1 active on the N return path.
func (w *PostJoinExitPromptWatcher) ShouldObserveEntry(e TextEvent) bool {
	if w == nil || w.state != postJoinExitPromptOpen {
		return true
	}
	_, known := w.c.byIdentity[skillExitIdentity(e)]
	if known {
		return true
	}
	for _, v := range w.c.byIdentity {
		if e.OriginalLength == v.id.length && e.OriginalSHA256 == v.id.hash {
			return true
		}
	}
	return e.Background == 0 && e.Foreground == 14 && (w.pending != nil || w.active != nil)
}
func (w *PostJoinExitPromptWatcher) ObserveEntry(e TextEvent) error {
	if w == nil || w.state != postJoinExitPromptOpen || w.pending != nil || e.PostCallStep != 0 {
		return w.fail("entry state invalid")
	}
	v, ok := w.c.byIdentity[skillExitIdentity(e)]
	if !ok {
		return w.fail(fmt.Sprintf("unknown exact entry at step %d length %d sha256 %x", e.EntryStep, e.OriginalLength, e.OriginalSHA256))
	}
	if (v.key == postJoinExitQ1 && w.active != nil) || (v.key == postJoinExitQ2 && (!w.q1Accepted || w.active == nil || w.active.EventKey != postJoinExitQ1)) {
		return w.fail("question order invalid")
	}
	w.pending, w.pendingEvent = &v, e
	return nil
}
func (w *PostJoinExitPromptWatcher) ObserveReturn(e TextEvent) (PostJoinExitPromptGeneration, error) {
	if w == nil || w.state != postJoinExitPromptOpen || w.pending == nil || e.PostCallStep <= e.EntryStep || e.EntryStep != w.pendingEvent.EntryStep || skillExitIdentity(e) != skillExitIdentity(w.pendingEvent) {
		return PostJoinExitPromptGeneration{}, w.fail("guarded return mismatch")
	}
	if w.pending.key == postJoinExitQ2 && w.active != nil {
		return PostJoinExitPromptGeneration{}, w.fail("q1 still active at q2 return")
	}
	w.generation++
	g := PostJoinExitPromptGeneration{w.generation, w.pending.key, w.pending.text, w.pending.width}
	if w.pending.key == postJoinExitQ1 {
		w.q1Accepted = true
	}
	w.pending = nil
	w.active = &g
	return g, nil
}
func (w *PostJoinExitPromptWatcher) Prewrite(v machine.VideoWrite) (bool, error) {
	if w == nil || w.state != postJoinExitPromptOpen {
		return false, nil
	}
	linear, err := NormalizeSkillExitVideoOffset(v.Offset)
	if err != nil {
		return false, w.fail("A000 offset out of range")
	}
	local := linear - skillExitVideoBase
	pendingHit := w.pending != nil && postJoinExitBodyIntersects(local, w.pending.width)
	activeHit := w.active != nil && postJoinExitBodyIntersects(local, w.active.Width)
	if !pendingHit && !activeHit {
		return false, nil
	}
	// Project phase 217 proves both writers only inside each exact pending
	// Entry→Return window. A second pending generation or another player path
	// must go through evidence review before gaining this exception.
	pendingWriter := pendingHit && w.pendingEvent.EntryStep == w.pending.entryStep &&
		v.Step >= w.pendingEvent.EntryStep && v.Step < w.pending.returnStep &&
		v.CS == 0x0763 && (v.IP == 0x184D || v.IP == 0x1854)
	activeWriter := v.CS == 0x0763 && v.IP == 0x184D
	if (pendingHit && !pendingWriter) || (activeHit && !activeWriter && !pendingWriter) {
		return false, w.fail(fmt.Sprintf("unknown intersecting writer step %d %04X:%04X A000:%04X", v.Step, v.CS, v.IP, v.Offset))
	}
	// Row 24 spans eight 320-pixel scanlines. Every store intersects one pixel,
	// including an old==new store; no writer is assumed from its value.
	if activeHit {
		w.active = nil
		return true, nil
	}
	return false, nil
}
func (w *PostJoinExitPromptWatcher) Active() bool {
	return w != nil && w.state == postJoinExitPromptOpen && w.active != nil
}
func (w *PostJoinExitPromptWatcher) Pending() bool {
	return w != nil && w.state == postJoinExitPromptOpen && w.pending != nil
}
func (w *PostJoinExitPromptWatcher) Failed() bool {
	return w == nil || w.state == postJoinExitPromptFailed
}
func (w *PostJoinExitPromptWatcher) Closed() bool {
	return w != nil && w.state == postJoinExitPromptClosed
}
func (w *PostJoinExitPromptWatcher) Stop() {
	if w != nil {
		w.pending = nil
		w.active = nil
		w.state = postJoinExitPromptClosed
	}
}
func (w *PostJoinExitPromptWatcher) Restore() {
	if w != nil && w.state == postJoinExitPromptOpen {
		w.pending = nil
		w.active = nil
		w.q1Accepted = false
	}
}
func (w *PostJoinExitPromptWatcher) Discontinuity() { w.fail("discontinuity") }
func (w *PostJoinExitPromptWatcher) Fault()         { w.fail("fault") }
