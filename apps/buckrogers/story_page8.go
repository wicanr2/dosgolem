package buckrogers

import (
	"crypto/sha256"
	"fmt"
)

type StoryPage8Identity struct {
	Sequence, OriginalLength, Mode, Repeat, Background, Foreground, Row, Column uint8
	EntryStep, PostCallStep                                                     uint64
	EventKey                                                                    string
	OriginalSHA256                                                              [32]byte
	Caller, Guard                                                               Address
}

type StoryPage8Catalog struct{ entries [4]StoryPage8Identity }

func NewStoryPage8Catalog(entries []StoryPage8Identity) (*StoryPage8Catalog, error) {
	if len(entries) != 4 {
		return nil, fmt.Errorf("buckrogers: 第 8 頁必須四行")
	}
	var catalog StoryPage8Catalog
	seen := make(map[string]bool, 4)
	for i, entry := range entries {
		approved := storyPage8Approved[i]
		if entry.Sequence != uint8(i+1) || entry.EventKey != fmt.Sprintf("story.page8.line.%03d", i+1) || seen[entry.EventKey] ||
			entry.OriginalLength != approved.length || fmt.Sprintf("%x", entry.OriginalSHA256) != approved.hash ||
			entry.EntryStep != approved.entryStep || entry.PostCallStep != approved.postCallStep ||
			entry.Caller != (Address{0x0763, 0x04ff}) || entry.Guard != storyGlyphPrimitive ||
			entry.Mode != 1 || entry.Repeat != 1 || entry.Background != 0 || entry.Foreground != 10 ||
			entry.Row != uint8(17+i) || entry.Column != 1 {
			return nil, fmt.Errorf("buckrogers: 第 8 頁 identity %d 無效", i+1)
		}
		seen[entry.EventKey] = true
		catalog.entries[i] = entry
	}
	return &catalog, nil
}

type StoryPage8Event struct {
	Generation, EntryStep, PostCallStep uint64
	EventKey                            string
	Row, Column                         uint8
}

type page8Frame struct {
	caller, guard Address
	ss, sp        uint16
	args          [7]uint16
	step          uint64
}

type page8Line struct {
	identity StoryPage8Identity
	entry    uint64
	bytes    []byte
}

type StoryPage8Watcher struct {
	catalog          *StoryPage8Catalog
	frame            *page8Frame
	pending          *page8Line
	done, events     []StoryPage8Event
	generation, last uint64
	active           bool
}

func NewStoryPage8Watcher(catalog *StoryPage8Catalog) (*StoryPage8Watcher, error) {
	if catalog == nil {
		return nil, fmt.Errorf("buckrogers: 第 8 頁缺 catalog")
	}
	return &StoryPage8Watcher{catalog: catalog, generation: 1}, nil
}

func (w *StoryPage8Watcher) dropPending() {
	w.frame = nil
	w.pending = nil
	w.done = nil
	w.last = 0
}

func (w *StoryPage8Watcher) ObserveGlyphEntry(guard, caller Address, ss, sp uint16, args [7]uint16, step uint64) {
	if w == nil || w.active {
		return
	}
	if w.frame != nil {
		w.dropPending()
	}
	for _, value := range args {
		if value > 0xff {
			w.dropPending()
			return
		}
	}
	line, column := len(w.done), 1
	if w.pending != nil {
		column += len(w.pending.bytes)
	}
	if line >= 4 || w.last >= step || guard != storyGlyphPrimitive || caller != (Address{0x0763, 0x04ff}) ||
		args[0] != 1 || args[2] != 1 || args[3] != 0 || args[4] != 10 ||
		args[5] != uint16(17+line) || args[6] != uint16(column) {
		w.dropPending()
		return
	}
	w.frame = &page8Frame{caller: caller, guard: guard, ss: ss, sp: sp, args: args, step: step}
}

func (w *StoryPage8Watcher) ObserveVerifiedGlyphReturn(previous Address, opcode byte, caller Address, ss, sp uint16, postCallStep uint64) {
	if w == nil || w.frame == nil {
		return
	}
	frame := w.frame
	w.frame = nil
	if previous != (Address{0x0763, 0x03d6}) || opcode != 0xca || caller != frame.caller ||
		ss != frame.ss || sp != frame.sp+storyGlyphStackDelta || postCallStep <= frame.step {
		w.dropPending()
		return
	}
	if w.pending == nil {
		w.pending = &page8Line{identity: w.catalog.entries[len(w.done)], entry: frame.step}
	}
	w.pending.bytes = append(w.pending.bytes, byte(frame.args[1]))
	w.last = postCallStep
	if len(w.pending.bytes) != int(w.pending.identity.OriginalLength) {
		return
	}
	if sha256.Sum256(w.pending.bytes) != w.pending.identity.OriginalSHA256 {
		w.dropPending()
		return
	}
	pending := w.pending
	w.done = append(w.done, StoryPage8Event{
		Generation: w.generation, EntryStep: pending.entry, PostCallStep: postCallStep,
		EventKey: pending.identity.EventKey, Row: uint8(17 + len(w.done)), Column: 1,
	})
	w.pending = nil
	if len(w.done) == 4 {
		w.events = append(w.events, w.done...)
		w.done = nil
		w.active = true
	}
}

// StoryPage8VideoSpanIntersects checks byte writes against the READY half-open
// rectangle [8,312)x[136,168), row by row so a wrapped span cannot borrow the
// next row's left margin.
func StoryPage8VideoSpanIntersects(di, count uint16) bool {
	if count == 0 {
		return false
	}
	start, end := uint32(di), uint32(di)+uint32(count)
	for row := start / 320; row <= (end-1)/320; row++ {
		if row < 136 || row >= 168 {
			continue
		}
		rowStart, rowEnd := row*320, (row+1)*320
		a, b := start, end
		if a < rowStart {
			a = rowStart
		}
		if b > rowEnd {
			b = rowEnd
		}
		if a-rowStart < 312 && b-rowStart > 8 {
			return true
		}
	}
	return false
}

func (w *StoryPage8Watcher) ObserveVideoWrite(at Address, es, di, count uint16) bool {
	if w == nil || at != storyFillInstruction || es != storyVideoSegment || !StoryPage8VideoSpanIntersects(di, count) {
		return false
	}
	wasDerived := w.active || w.frame != nil || w.pending != nil || len(w.done) != 0
	w.dropPending()
	w.active = false
	w.events = nil
	if wasDerived {
		w.generation++
	}
	return wasDerived
}

func (w *StoryPage8Watcher) ObserveExecutionDiscontinuity() {
	if w == nil {
		return
	}
	w.dropPending()
	w.events = nil
	if w.active {
		w.active = false
		w.generation++
	}
}

func (w *StoryPage8Watcher) Events() []StoryPage8Event {
	if w == nil {
		return nil
	}
	return append([]StoryPage8Event(nil), w.events...)
}

func (w *StoryPage8Watcher) Active() bool { return w != nil && w.active }

func (w *StoryPage8Watcher) Generation() uint64 {
	if w == nil {
		return 0
	}
	return w.generation
}
