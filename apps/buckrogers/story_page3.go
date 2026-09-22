package buckrogers

import (
	"crypto/sha256"
	"fmt"
)

// StoryPage3Identity is an exact, content-safe page-three glyph-run identity.
// Original bytes live only in the watcher until their digest is checked.
type StoryPage3Identity struct {
	Sequence                                          uint8
	EventKey                                          string
	OriginalLength                                    uint8
	OriginalSHA256                                    [32]byte
	Caller, Guard                                     Address
	Mode, Repeat, Background, Foreground, Row, Column uint8
}

type StoryPage3Catalog struct{ entries [5]StoryPage3Identity }

func NewStoryPage3Catalog(entries []StoryPage3Identity) (*StoryPage3Catalog, error) {
	if len(entries) != 5 {
		return nil, fmt.Errorf("buckrogers: 第 3 頁劇情 catalog 必須恰有五行")
	}
	var c StoryPage3Catalog
	seen := map[string]bool{}
	for i, e := range entries {
		if e.Sequence != uint8(i+1) || e.EventKey == "" || seen[e.EventKey] || e.OriginalLength == 0 ||
			e.OriginalSHA256 == ([32]byte{}) || e.Caller != (Address{0x0763, 0x04ff}) || e.Guard != storyGlyphPrimitive ||
			e.Mode != 1 || e.Repeat != 1 || e.Background != 0 || e.Foreground != 10 || e.Row != uint8(17+i) || e.Column != 1 {
			return nil, fmt.Errorf("buckrogers: 第 3 頁劇情 identity %d 不符合 READY 形狀", i+1)
		}
		seen[e.EventKey] = true
		c.entries[i] = e
	}
	return &c, nil
}

type StoryPage3VerifiedReturn struct {
	EntryStep, PostCallStep uint64
	Caller                  Address
	SS, SP                  uint16
}
type StoryPage3Event struct {
	Generation, EntryStep, PostCallStep uint64
	EventKey                            string
	OriginalLength                      uint8
	OriginalSHA256                      [32]byte
	Background, Foreground, Row, Column uint8
}
type storyPage3Frame struct {
	caller, guard                         Address
	ss, sp                                uint16
	entry                                 uint64
	mode, glyph, repeat, bg, fg, row, col uint8
}
type storyPage3Line struct {
	identity StoryPage3Identity
	entry    uint64
	bytes    []byte
}

// StoryPage3Watcher has no machine, renderer, input, or file dependency.
// It is fail-closed: anything not proving the measured frame sequence drops it.
type StoryPage3Watcher struct {
	catalog           *StoryPage3Catalog
	pending           *storyPage3Frame
	candidate         *storyPage3Line
	completed, events []StoryPage3Event
	generation        uint64
	active            bool
	drops, misses     int
}

func NewStoryPage3Watcher(c *StoryPage3Catalog) (*StoryPage3Watcher, error) {
	if c == nil {
		return nil, fmt.Errorf("buckrogers: 第 3 頁劇情 watcher 缺少 catalog")
	}
	return &StoryPage3Watcher{catalog: c, generation: 1}, nil
}
func (w *StoryPage3Watcher) ObserveGlyphEntry(guard, caller Address, ss, sp uint16, args [7]uint16, step uint64) {
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
	w.pending = &storyPage3Frame{guard: guard, caller: caller, ss: ss, sp: sp, entry: step, mode: uint8(args[0]), glyph: uint8(args[1]), repeat: uint8(args[2]), bg: uint8(args[3]), fg: uint8(args[4]), row: uint8(args[5]), col: uint8(args[6])}
}
func (w *StoryPage3Watcher) ObserveVerifiedGlyphReturn(r StoryPage3VerifiedReturn) {
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
func (w *StoryPage3Watcher) observe(f *storyPage3Frame, post uint64) {
	if w.candidate == nil {
		if w.active {
			return
		}
		if len(w.completed) >= 5 || !page3Matches(w.catalog.entries[len(w.completed)], 0, f) {
			if len(w.completed) != 0 {
				w.drop()
				w.drops++
			}
			return
		}
		w.candidate = &storyPage3Line{identity: w.catalog.entries[len(w.completed)], entry: f.entry, bytes: []byte{f.glyph}}
	} else {
		i := len(w.candidate.bytes)
		if !page3Matches(w.candidate.identity, i, f) {
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
func page3Matches(e StoryPage3Identity, i int, f *storyPage3Frame) bool {
	return f.guard == e.Guard && f.caller == e.Caller && f.mode == e.Mode && f.repeat == e.Repeat && f.bg == e.Background && f.fg == e.Foreground && f.row == e.Row && int(f.col) == int(e.Column)+i
}
func (w *StoryPage3Watcher) finish(post uint64) {
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
	w.completed = append(w.completed, StoryPage3Event{Generation: w.generation, EntryStep: c.entry, PostCallStep: post, EventKey: c.identity.EventKey, OriginalLength: c.identity.OriginalLength, OriginalSHA256: c.identity.OriginalSHA256, Background: c.identity.Background, Foreground: c.identity.Foreground, Row: c.identity.Row, Column: c.identity.Column})
	if len(w.completed) == 5 {
		w.events = append(w.events, w.completed...)
		w.completed = nil
		w.active = true
	}
}
func (w *StoryPage3Watcher) drop() {
	if w != nil {
		w.candidate = nil
		w.completed = nil
	}
}

// ObserveExecutionDiscontinuity clears both in-flight and active state. A
// restored/unknown execution epoch must exact-hit all five lines before draw.
func (w *StoryPage3Watcher) ObserveExecutionDiscontinuity() {
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

// ObserveVideoWrite recognizes only the measured pre-execution fill instruction.
func (w *StoryPage3Watcher) ObserveVideoWrite(at Address, es, di, count uint16) bool {
	if w == nil || at != storyFillInstruction || es != storyVideoSegment || !StoryPage3VideoSpanIntersects(di, count) {
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
func StoryPage3VideoSpanIntersects(di, count uint16) bool {
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
func (w *StoryPage3Watcher) Events() []StoryPage3Event {
	if w == nil {
		return nil
	}
	return append([]StoryPage3Event(nil), w.events...)
}
func (w *StoryPage3Watcher) Active() bool { return w != nil && w.active }
func (w *StoryPage3Watcher) Generation() uint64 {
	if w == nil {
		return 0
	}
	return w.generation
}
func (w *StoryPage3Watcher) Pending() bool {
	return w != nil && (w.pending != nil || w.candidate != nil || len(w.completed) != 0)
}
