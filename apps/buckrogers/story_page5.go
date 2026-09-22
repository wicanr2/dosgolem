package buckrogers

import (
	"crypto/sha256"
	"fmt"
)

// StoryPage5Identity is an exact content-safe identity for one fifth-page
// glyph run. Original bytes live only until their digest is verified.
type StoryPage5Identity struct {
	Sequence                                          uint8
	EventKey                                          string
	OriginalLength                                    uint8
	OriginalSHA256                                    [32]byte
	Caller, Guard                                     Address
	Mode, Repeat, Background, Foreground, Row, Column uint8
}
type StoryPage5Catalog struct{ entries [5]StoryPage5Identity }

func NewStoryPage5Catalog(entries []StoryPage5Identity) (*StoryPage5Catalog, error) {
	if len(entries) != 5 {
		return nil, fmt.Errorf("buckrogers: 第 5 頁劇情 catalog 必須恰有五行")
	}
	var c StoryPage5Catalog
	seen := map[string]bool{}
	for i, e := range entries {
		if e.Sequence != uint8(i+1) || e.EventKey == "" || seen[e.EventKey] || e.OriginalLength == 0 || e.OriginalSHA256 == ([32]byte{}) || e.Caller != (Address{0x0763, 0x04ff}) || e.Guard != storyGlyphPrimitive || e.Mode != 1 || e.Repeat != 1 || e.Background != 0 || e.Foreground != 10 || e.Row != uint8(17+i) || e.Column != 1 {
			return nil, fmt.Errorf("buckrogers: 第 5 頁劇情 identity %d 不符合 READY 形狀", i+1)
		}
		seen[e.EventKey] = true
		c.entries[i] = e
	}
	return &c, nil
}

type StoryPage5VerifiedReturn struct {
	EntryStep, PostCallStep uint64
	Caller                  Address
	SS, SP                  uint16
}
type StoryPage5Event struct {
	Generation, EntryStep, PostCallStep uint64
	EventKey                            string
	OriginalLength                      uint8
	OriginalSHA256                      [32]byte
	Background, Foreground, Row, Column uint8
}
type storyPage5Frame struct {
	caller, guard                         Address
	ss, sp                                uint16
	entry                                 uint64
	mode, glyph, repeat, bg, fg, row, col uint8
}
type storyPage5Line struct {
	identity StoryPage5Identity
	entry    uint64
	bytes    []byte
}

// StoryPage5Watcher is output-only and fail-closed. It is deliberately not
// wired to an Oracle here; a future adapter must supply both observed stages.
type StoryPage5Watcher struct {
	catalog           *StoryPage5Catalog
	pending           *storyPage5Frame
	candidate         *storyPage5Line
	completed, events []StoryPage5Event
	generation        uint64
	active            bool
	drops, misses     int
}

func NewStoryPage5Watcher(c *StoryPage5Catalog) (*StoryPage5Watcher, error) {
	if c == nil {
		return nil, fmt.Errorf("buckrogers: 第 5 頁劇情 watcher 缺少 catalog")
	}
	return &StoryPage5Watcher{catalog: c, generation: 1}, nil
}

// ObserveGlyphEntry preserves 16-bit ABI words long enough to reject any
// nonzero high word before narrowing the approved byte fields.
func (w *StoryPage5Watcher) ObserveGlyphEntry(guard, caller Address, ss, sp uint16, args [7]uint16, step uint64) {
	if w == nil {
		return
	}
	for _, word := range args {
		if word > 0xff {
			w.pending = nil
			w.drop()
			w.drops++
			return
		}
	}
	if w.pending != nil {
		w.pending = nil
		w.drop()
		w.drops++
	}
	w.pending = &storyPage5Frame{guard: guard, caller: caller, ss: ss, sp: sp, entry: step, mode: uint8(args[0]), glyph: uint8(args[1]), repeat: uint8(args[2]), bg: uint8(args[3]), fg: uint8(args[4]), row: uint8(args[5]), col: uint8(args[6])}
}
func (w *StoryPage5Watcher) ObserveVerifiedGlyphReturn(r StoryPage5VerifiedReturn) {
	if w == nil || w.pending == nil || r.Caller != w.pending.caller || r.EntryStep != w.pending.entry {
		return
	}
	f := w.pending
	w.pending = nil
	if r.PostCallStep <= f.entry || r.SS != f.ss || r.SP != f.sp+storyGlyphStackDelta {
		w.drop()
		w.drops++
		return
	}
	w.observe(f, r.PostCallStep)
}
func page5Matches(e StoryPage5Identity, i int, f *storyPage5Frame) bool {
	return f.guard == e.Guard && f.caller == e.Caller && f.mode == e.Mode && f.repeat == e.Repeat && f.bg == e.Background && f.fg == e.Foreground && f.row == e.Row && int(f.col) == int(e.Column)+i
}
func (w *StoryPage5Watcher) observe(f *storyPage5Frame, post uint64) {
	if w.candidate == nil {
		if w.active {
			return
		}
		if len(w.completed) >= 5 || !page5Matches(w.catalog.entries[len(w.completed)], 0, f) {
			if len(w.completed) != 0 {
				w.drop()
				w.drops++
			}
			return
		}
		w.candidate = &storyPage5Line{identity: w.catalog.entries[len(w.completed)], entry: f.entry, bytes: []byte{f.glyph}}
	} else {
		i := len(w.candidate.bytes)
		if !page5Matches(w.candidate.identity, i, f) {
			w.drop()
			w.drops++
			return
		}
		w.candidate.bytes = append(w.candidate.bytes, f.glyph)
	}
	if len(w.candidate.bytes) == int(w.candidate.identity.OriginalLength) {
		w.finish(post)
	}
}
func (w *StoryPage5Watcher) finish(post uint64) {
	c := w.candidate
	w.candidate = nil
	if len(c.bytes) != int(c.identity.OriginalLength) || sha256.Sum256(c.bytes) != c.identity.OriginalSHA256 {
		w.completed = nil
		w.misses++
		return
	}
	if len(w.completed) > 0 && c.entry <= w.completed[len(w.completed)-1].PostCallStep {
		w.drop()
		w.drops++
		return
	}
	w.completed = append(w.completed, StoryPage5Event{Generation: w.generation, EntryStep: c.entry, PostCallStep: post, EventKey: c.identity.EventKey, OriginalLength: c.identity.OriginalLength, OriginalSHA256: c.identity.OriginalSHA256, Background: c.identity.Background, Foreground: c.identity.Foreground, Row: c.identity.Row, Column: c.identity.Column})
	if len(w.completed) == 5 {
		w.events = append(w.events, w.completed...)
		w.completed = nil
		w.active = true
	}
}
func (w *StoryPage5Watcher) drop() {
	if w != nil {
		w.candidate = nil
		w.completed = nil
	}
}
func (w *StoryPage5Watcher) ObserveExecutionDiscontinuity() {
	if w == nil {
		return
	}
	w.pending = nil
	w.drop()
	if w.active {
		w.active = false
		w.generation++
	}
	w.drops++
}

// ObserveVideoWrite accepts only the measured page5→page6 pre-execution fill.
func (w *StoryPage5Watcher) ObserveVideoWrite(at Address, es, di, count uint16) bool {
	if w == nil || at != storyFillInstruction || es != storyVideoSegment || !StoryPage5VideoSpanIntersects(di, count) {
		return false
	}
	was := w.active || w.pending != nil || w.candidate != nil || len(w.completed) != 0
	w.pending = nil
	w.drop()
	w.active = false
	if was {
		w.generation++
	}
	return was
}
func StoryPage5VideoSpanIntersects(di, count uint16) bool {
	const width uint32 = 320
	const top, bottom, left, right uint32 = 136, 176, 8, 320
	if count == 0 {
		return false
	}
	start, end := uint32(di), uint32(di)+uint32(count)
	if end <= top*width || start >= bottom*width {
		return false
	}
	if start < top*width {
		start = top * width
	}
	if end > bottom*width {
		end = bottom * width
	}
	for row := start / width; row <= (end-1)/width; row++ {
		rs, re := row*width, (row+1)*width
		a, b := start, end
		if a < rs {
			a = rs
		}
		if b > re {
			b = re
		}
		if a-rs < right && b-rs > left {
			return true
		}
	}
	return false
}
func (w *StoryPage5Watcher) Events() []StoryPage5Event {
	if w == nil {
		return nil
	}
	return append([]StoryPage5Event(nil), w.events...)
}
func (w *StoryPage5Watcher) Active() bool { return w != nil && w.active }
func (w *StoryPage5Watcher) Generation() uint64 {
	if w == nil {
		return 0
	}
	return w.generation
}
func (w *StoryPage5Watcher) Pending() bool {
	return w != nil && (w.pending != nil || w.candidate != nil || len(w.completed) != 0)
}
