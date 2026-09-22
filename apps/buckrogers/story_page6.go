package buckrogers

import (
	"crypto/sha256"
	"fmt"
)

// StoryPage6Identity is a reviewed, output-only identity.  It intentionally
// contains a digest, never retained original text.
type StoryPage6Identity struct {
	Sequence, OriginalLength, Mode, Repeat, Background, Foreground, Row, Column uint8
	EventKey                                                                    string
	OriginalSHA256                                                              [32]byte
	Caller, Guard                                                               Address
}
type StoryPage6Catalog struct{ entries [6]StoryPage6Identity }

func NewStoryPage6Catalog(entries []StoryPage6Identity) (*StoryPage6Catalog, error) {
	if len(entries) != 6 {
		return nil, fmt.Errorf("buckrogers: 第 6 頁劇情 catalog 必須恰有六行")
	}
	var catalog StoryPage6Catalog
	seen := map[string]bool{}
	for i, entry := range entries {
		if entry.Sequence != uint8(i+1) || entry.EventKey == "" || seen[entry.EventKey] || entry.OriginalLength == 0 || entry.OriginalSHA256 == ([32]byte{}) || entry.Caller != (Address{0x0763, 0x04ff}) || entry.Guard != storyGlyphPrimitive || entry.Mode != 1 || entry.Repeat != 1 || entry.Background != 0 || entry.Foreground != 10 || entry.Row != uint8(17+i) || entry.Column != 1 {
			return nil, fmt.Errorf("buckrogers: 第 6 頁劇情 identity %d 不符合 READY 形狀", i+1)
		}
		seen[entry.EventKey] = true
		catalog.entries[i] = entry
	}
	return &catalog, nil
}

type StoryPage6Event struct {
	Generation, EntryStep, PostCallStep uint64
	EventKey                            string
	Row, Column                         uint8
}
type storyPage6Frame struct {
	guard, caller Address
	ss, sp        uint16
	args          [7]uint16
	step          uint64
}
type storyPage6Pending struct {
	identity StoryPage6Identity
	entry    uint64
	bytes    []byte
}

// StoryPage6Watcher needs both glyph entry and the verified RETF callback.
// Neither a single entry observation nor a partial line may activate it.
type StoryPage6Watcher struct {
	catalog           *StoryPage6Catalog
	frame             *storyPage6Frame
	pending           *storyPage6Pending
	completed, events []StoryPage6Event
	generation        uint64
	lastPostStep      uint64
	active            bool
}

func NewStoryPage6Watcher(catalog *StoryPage6Catalog) (*StoryPage6Watcher, error) {
	if catalog == nil {
		return nil, fmt.Errorf("buckrogers: 第 6 頁劇情 watcher 缺少 catalog")
	}
	return &StoryPage6Watcher{catalog: catalog, generation: 1}, nil
}

// ObserveGlyphEntry preserves all seven ABI words before rejecting a high byte.
func (watcher *StoryPage6Watcher) ObserveGlyphEntry(guard, caller Address, ss, sp uint16, args [7]uint16, step uint64) {
	if watcher == nil || watcher.active {
		return
	}
	if watcher.frame != nil {
		watcher.drop()
	}
	for _, word := range args {
		if word > 0xff {
			watcher.drop()
			return
		}
	}
	line := len(watcher.completed)
	column := 1
	if watcher.pending != nil {
		column += len(watcher.pending.bytes)
	}
	if line >= 6 || (watcher.lastPostStep != 0 && step <= watcher.lastPostStep) || guard != storyGlyphPrimitive || caller != (Address{0x0763, 0x04ff}) || args[0] != 1 || args[2] != 1 || args[3] != 0 || args[4] != 10 || args[5] != uint16(17+line) || args[6] != uint16(column) {
		watcher.drop()
		return
	}
	watcher.frame = &storyPage6Frame{guard: guard, caller: caller, ss: ss, sp: sp, args: args, step: step}
}

// ObserveVerifiedGlyphReturn accepts only the measured RETF edge.
func (watcher *StoryPage6Watcher) ObserveVerifiedGlyphReturn(previous Address, opcode byte, caller Address, ss, sp uint16, postStep uint64) {
	if watcher == nil || watcher.frame == nil {
		return
	}
	frame := watcher.frame
	watcher.frame = nil
	if previous != (Address{0x0763, 0x03d6}) || opcode != 0xca || caller != frame.caller || ss != frame.ss || sp != frame.sp+storyGlyphStackDelta || postStep <= frame.step {
		watcher.drop()
		return
	}
	if watcher.pending == nil {
		watcher.pending = &storyPage6Pending{identity: watcher.catalog.entries[len(watcher.completed)], entry: frame.step}
	}
	watcher.pending.bytes = append(watcher.pending.bytes, byte(frame.args[1]))
	watcher.lastPostStep = postStep
	if len(watcher.pending.bytes) != int(watcher.pending.identity.OriginalLength) {
		return
	}
	if sha256.Sum256(watcher.pending.bytes) != watcher.pending.identity.OriginalSHA256 {
		watcher.drop()
		return
	}
	if len(watcher.completed) > 0 && watcher.pending.entry <= watcher.completed[len(watcher.completed)-1].PostCallStep {
		watcher.drop()
		return
	}
	line := watcher.pending
	watcher.completed = append(watcher.completed, StoryPage6Event{Generation: watcher.generation, EntryStep: line.entry, PostCallStep: postStep, EventKey: line.identity.EventKey, Row: line.identity.Row, Column: 1})
	watcher.pending = nil
	if len(watcher.completed) == 6 {
		watcher.events = append(watcher.events, watcher.completed...)
		watcher.completed = nil
		watcher.active = true
	}
}

func (watcher *StoryPage6Watcher) drop() {
	watcher.frame = nil
	watcher.pending = nil
	watcher.completed = nil
	watcher.lastPostStep = 0
}
func (watcher *StoryPage6Watcher) ObserveExecutionDiscontinuity() {
	if watcher == nil {
		return
	}
	watcher.drop()
	if watcher.active {
		watcher.active = false
		watcher.generation++
	}
}

func StoryPage6VideoSpanIntersects(di, count uint16) bool {
	const top, bottom, left, right uint32 = 136, 184, 8, 320
	if count == 0 {
		return false
	}
	start, end := uint32(di), uint32(di)+uint32(count)
	if end <= top*320 || start >= bottom*320 {
		return false
	}
	if start < top*320 {
		start = top * 320
	}
	if end > bottom*320 {
		end = bottom * 320
	}
	for row := start / 320; row <= (end-1)/320; row++ {
		rowStart, rowEnd := row*320, (row+1)*320
		a, b := start, end
		if a < rowStart {
			a = rowStart
		}
		if b > rowEnd {
			b = rowEnd
		}
		if a-rowStart < right && b-rowStart > left {
			return true
		}
	}
	return false
}

// ObserveVideoWrite has clear authority only for the measured Mode 13h fill.
func (watcher *StoryPage6Watcher) ObserveVideoWrite(at Address, es, di, count uint16) bool {
	if watcher == nil || at != storyFillInstruction || es != storyVideoSegment || !StoryPage6VideoSpanIntersects(di, count) {
		return false
	}
	was := watcher.active || watcher.frame != nil || watcher.pending != nil || len(watcher.completed) != 0
	watcher.drop()
	watcher.active = false
	if was {
		watcher.generation++
	}
	return was
}
func (watcher *StoryPage6Watcher) Events() []StoryPage6Event {
	if watcher == nil {
		return nil
	}
	return append([]StoryPage6Event(nil), watcher.events...)
}
func (watcher *StoryPage6Watcher) Active() bool { return watcher != nil && watcher.active }
func (watcher *StoryPage6Watcher) Generation() uint64 {
	if watcher == nil {
		return 0
	}
	return watcher.generation
}
func (watcher *StoryPage6Watcher) Pending() bool {
	return watcher != nil && (watcher.frame != nil || watcher.pending != nil || len(watcher.completed) != 0)
}
