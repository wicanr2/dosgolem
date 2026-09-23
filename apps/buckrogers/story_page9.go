package buckrogers

import (
	"crypto/sha256"
	"fmt"

	"github.com/wicanr2/dosgolem/internal/machine"
)

// StoryPage9Identity is the one fixed line proven at legal page8→page9 entry.
type StoryPage9Identity struct {
	EventKey                                          string
	OriginalLength                                    uint8
	OriginalSHA256                                    [32]byte
	Caller, Guard                                     Address
	Mode, Repeat, Background, Foreground, Row, Column uint8
	EntryStep, PostCallStep                           uint64 // provenance only; never a runtime clock gate
}

type StoryPage9Catalog struct{ entry StoryPage9Identity }

func NewStoryPage9Catalog(entry StoryPage9Identity) (*StoryPage9Catalog, error) {
	if entry.EventKey != "story.page9.line.001" || entry.OriginalLength != 20 ||
		fmt.Sprintf("%x", entry.OriginalSHA256) != storyPage9ApprovedHash ||
		entry.Caller != (Address{0x0763, 0x04ff}) || entry.Guard != storyGlyphPrimitive ||
		entry.Mode != 1 || entry.Repeat != 1 || entry.Background != 0 || entry.Foreground != 10 ||
		entry.Row != 17 || entry.Column != 1 ||
		entry.EntryStep != 351155910 || entry.PostCallStep != 351988536 {
		return nil, fmt.Errorf("buckrogers: 第 9 頁 identity 無效")
	}
	return &StoryPage9Catalog{entry: entry}, nil
}

type StoryPage9Event struct {
	Generation, EntryStep, PostCallStep uint64
	EventKey                            string
	Row, Column                         uint8
}

type storyPage9Frame struct {
	caller, guard Address
	ss, sp        uint16
	args          [7]uint16
	step          uint64
	writes        uint8
	lastWrite     uint64
}

type StoryPage9Watcher struct {
	catalog     *StoryPage9Catalog
	frame       *storyPage9Frame
	bytes       [20]byte
	count       uint8
	entry, last uint64
	event       *StoryPage9Event
	generation  uint64
}

func NewStoryPage9Watcher(catalog *StoryPage9Catalog) (*StoryPage9Watcher, error) {
	if catalog == nil {
		return nil, fmt.Errorf("buckrogers: 第 9 頁缺 READY catalog")
	}
	return &StoryPage9Watcher{catalog: catalog, generation: 1}, nil
}

func (w *StoryPage9Watcher) clear() bool {
	if w == nil {
		return false
	}
	had := w.frame != nil || w.count != 0 || w.event != nil
	w.frame, w.count, w.entry, w.last, w.event = nil, 0, 0, 0, nil
	w.bytes = [20]byte{}
	if had {
		w.generation++
	}
	return had
}

func (w *StoryPage9Watcher) Stop()                          { w.clear() }
func (w *StoryPage9Watcher) Restore()                       { w.clear() }
func (w *StoryPage9Watcher) ObserveExecutionDiscontinuity() { w.clear() }
func (w *StoryPage9Watcher) Active() bool                   { return w != nil && w.event != nil }
func (w *StoryPage9Watcher) Pending() bool                  { return w != nil && (w.frame != nil || w.count != 0) }
func (w *StoryPage9Watcher) Generation() uint64 {
	if w == nil {
		return 0
	}
	return w.generation
}
func (w *StoryPage9Watcher) Events() []StoryPage9Event {
	if !w.Active() {
		return nil
	}
	return []StoryPage9Event{*w.event}
}

func (w *StoryPage9Watcher) ObserveGlyphEntry(guard, caller Address, ss, sp uint16, args [7]uint16, step uint64) {
	if w == nil || w.Active() {
		return
	}
	if w.frame != nil {
		w.clear()
		return
	}
	first := w.count == 0
	if first && (guard != w.catalog.entry.Guard || caller != w.catalog.entry.Caller || args[5] != 17 || args[6] != 1) {
		return
	}
	if guard != w.catalog.entry.Guard || caller != w.catalog.entry.Caller ||
		args[0] != 1 || args[2] != 1 || args[3] != 0 || args[4] != 10 ||
		args[5] != 17 || args[6] != uint16(w.count)+1 || (w.count != 0 && step <= w.last) {
		w.clear()
		return
	}
	for _, v := range args {
		if v > 0xff {
			w.clear()
			return
		}
	}
	if first {
		w.entry = step
	}
	w.frame = &storyPage9Frame{caller: caller, guard: guard, ss: ss, sp: sp, args: args, step: step}
}

// ObserveVideoWrite receives every A000 byte before the original VGA write.
// A pending glyph must build its own cell; an active overlay loses ownership on
// the first intersecting write, including a same-value write.
func (w *StoryPage9Watcher) ObserveVideoWrite(v machine.VideoWrite) bool {
	if w == nil {
		return false
	}
	if v.Offset >= 64000 {
		return w.clear()
	}
	x, y := v.Offset%320, v.Offset/320
	if x < 8 || x >= 168 || y < 136 || y >= 144 {
		return false
	}
	if w.Active() {
		return w.clear()
	}
	if w.count == 0 && w.frame == nil {
		return false
	}
	f := w.frame
	if f == nil || v.Step <= f.step || (f.writes != 0 && v.Step < f.lastWrite) ||
		(v.CS != 0x0763 || (v.IP != 0x184d && v.IP != 0x1854)) ||
		x < uint32(f.args[6])*8 || x >= (uint32(f.args[6])+1)*8 || f.writes >= 64 {
		w.clear()
		return false
	}
	f.writes++
	f.lastWrite = v.Step
	return false
}

func (w *StoryPage9Watcher) ObserveVerifiedGlyphReturn(previous Address, opcode byte, caller Address, ss, sp uint16, postCallStep uint64) {
	if w == nil || w.frame == nil || w.Active() {
		return
	}
	f := w.frame
	if previous != (Address{0x0763, 0x03d6}) || opcode != 0xca || caller != f.caller ||
		ss != f.ss || sp != f.sp+storyGlyphStackDelta || postCallStep <= f.step ||
		postCallStep <= f.lastWrite || f.writes != 64 {
		w.clear()
		return
	}
	w.frame = nil
	w.bytes[w.count] = byte(f.args[1])
	w.count++
	w.last = postCallStep
	if w.count != 20 {
		return
	}
	if sha256.Sum256(w.bytes[:]) != w.catalog.entry.OriginalSHA256 {
		w.clear()
		return
	}
	w.event = &StoryPage9Event{Generation: w.generation, EntryStep: w.entry, PostCallStep: postCallStep,
		EventKey: w.catalog.entry.EventKey, Row: 17, Column: 1}
	w.count = 0
	w.bytes = [20]byte{}
}
