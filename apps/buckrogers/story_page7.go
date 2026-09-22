package buckrogers

import (
	"crypto/sha256"
	"fmt"
)

type StoryPage7Identity struct {
	Sequence, OriginalLength uint8
	EventKey                 string
	OriginalSHA256           [32]byte
	Caller, Guard            Address
}
type StoryPage7Catalog struct{ entries [6]StoryPage7Identity }

func NewStoryPage7Catalog(es []StoryPage7Identity) (*StoryPage7Catalog, error) {
	if len(es) != 6 {
		return nil, fmt.Errorf("buckrogers: 第 7 頁必須六行")
	}
	var c StoryPage7Catalog
	seen := map[string]bool{}
	for i, e := range es {
		if e.Sequence != uint8(i+1) || e.EventKey == "" || seen[e.EventKey] || e.OriginalLength == 0 || e.OriginalSHA256 == ([32]byte{}) || e.Caller != (Address{0x0763, 0x04ff}) || e.Guard != storyGlyphPrimitive {
			return nil, fmt.Errorf("buckrogers: 第 7 頁 identity %d 無效", i+1)
		}
		seen[e.EventKey] = true
		c.entries[i] = e
	}
	return &c, nil
}

type StoryPage7Event struct {
	Generation, EntryStep, PostCallStep uint64
	EventKey                            string
	Row, Column                         uint8
}
type page7Frame struct {
	caller, guard Address
	ss, sp        uint16
	args          [7]uint16
	step          uint64
}
type page7Line struct {
	id    StoryPage7Identity
	entry uint64
	bytes []byte
}
type StoryPage7Watcher struct {
	c                *StoryPage7Catalog
	f                *page7Frame
	p                *page7Line
	done, events     []StoryPage7Event
	generation, last uint64
	active           bool
}

func NewStoryPage7Watcher(c *StoryPage7Catalog) (*StoryPage7Watcher, error) {
	if c == nil {
		return nil, fmt.Errorf("buckrogers: 第 7 頁缺 catalog")
	}
	return &StoryPage7Watcher{c: c, generation: 1}, nil
}
func (w *StoryPage7Watcher) drop() { w.f = nil; w.p = nil; w.done = nil; w.last = 0 }
func (w *StoryPage7Watcher) ObserveGlyphEntry(guard, caller Address, ss, sp uint16, a [7]uint16, step uint64) {
	if w == nil || w.active {
		return
	}
	if w.f != nil {
		w.drop()
	}
	for _, v := range a {
		if v > 255 {
			w.drop()
			return
		}
	}
	line, col := len(w.done), 1
	if w.p != nil {
		col += len(w.p.bytes)
	}
	if line >= 6 || w.last >= step || guard != storyGlyphPrimitive || caller != (Address{0x0763, 0x04ff}) || a[0] != 1 || a[2] != 1 || a[3] != 0 || a[4] != 10 || a[5] != uint16(17+line) || a[6] != uint16(col) {
		w.drop()
		return
	}
	w.f = &page7Frame{caller, guard, ss, sp, a, step}
}
func (w *StoryPage7Watcher) ObserveVerifiedGlyphReturn(previous Address, opcode byte, caller Address, ss, sp uint16, post uint64) {
	if w == nil || w.f == nil {
		return
	}
	f := w.f
	w.f = nil
	if previous != (Address{0x0763, 0x03d6}) || opcode != 0xca || caller != f.caller || ss != f.ss || sp != f.sp+storyGlyphStackDelta || post <= f.step {
		w.drop()
		return
	}
	if w.p == nil {
		w.p = &page7Line{id: w.c.entries[len(w.done)], entry: f.step}
	}
	w.p.bytes = append(w.p.bytes, byte(f.args[1]))
	w.last = post
	if len(w.p.bytes) != int(w.p.id.OriginalLength) {
		return
	}
	if sha256.Sum256(w.p.bytes) != w.p.id.OriginalSHA256 {
		w.drop()
		return
	}
	p := w.p
	w.done = append(w.done, StoryPage7Event{w.generation, p.entry, post, p.id.EventKey, uint8(17 + len(w.done)), 1})
	w.p = nil
	if len(w.done) == 6 {
		w.events = append(w.events, w.done...)
		w.done = nil
		w.active = true
	}
}
func StoryPage7VideoSpanIntersects(di, count uint16) bool {
	if count == 0 {
		return false
	}
	s, e := uint32(di), uint32(di)+uint32(count)
	for r := s / 320; r <= (e-1)/320; r++ {
		if r < 136 || r >= 184 {
			continue
		}
		a, b := s, e
		if a < r*320 {
			a = r * 320
		}
		if b > (r+1)*320 {
			b = (r + 1) * 320
		}
		if a-r*320 < 320 && b-r*320 > 8 {
			return true
		}
	}
	return false
}
func (w *StoryPage7Watcher) ObserveVideoWrite(at Address, es, di, count uint16) bool {
	if w == nil || at != storyFillInstruction || es != storyVideoSegment || !StoryPage7VideoSpanIntersects(di, count) {
		return false
	}
	was := w.active || w.f != nil || w.p != nil || len(w.done) > 0
	w.drop()
	w.active = false
	if was {
		w.generation++
	}
	return was
}
func (w *StoryPage7Watcher) ObserveExecutionDiscontinuity() {
	if w != nil {
		w.drop()
		if w.active {
			w.active = false
			w.generation++
		}
	}
}
func (w *StoryPage7Watcher) Events() []StoryPage7Event {
	return append([]StoryPage7Event(nil), w.events...)
}
func (w *StoryPage7Watcher) Active() bool { return w != nil && w.active }
