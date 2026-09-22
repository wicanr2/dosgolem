package buckrogers

// Spec 013: page four is a six-line, output-only READY slice.  This file
// deliberately owns neither a machine nor a renderer integration hook.

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"github.com/wicanr2/dosgolem/xlate"
	"strconv"
)

type StoryPage4Identity struct {
	Sequence, OriginalLength, Mode, Repeat, Background, Foreground, Row, Column uint8
	EventKey                                                                    string
	OriginalSHA256                                                              [32]byte
	Caller, Guard                                                               Address
}
type StoryPage4Catalog struct{ entries [6]StoryPage4Identity }

var storyPage4Approved = [6]struct {
	n uint8
	h string
}{{34, "70dcbd264497c53d685f4261bf3bff0b8e4a3110b43b918105be9753048d95bf"}, {37, "b2005b94a3d6fb0fe926ff3c77bbb2f6d2c6a43169affb4cd10ee6b2641b6247"}, {33, "41248df16995d7ec129c735e250e4cbae84411a7b9da14bc67efe0594293cda7"}, {31, "35b59e99604cbd7264ad1218ae3ad20760dfc7a0d21ca04afaadeebe87a00eda"}, {33, "c83d64d5bcfeae8f41fc4fc899637fa45af2826b62d33e9ee9042331e91b561b"}, {24, "ab0a680e1bd4ba6ed62ac7438633c6559b22412508953561e0034d7440b65066"}}

func LoadStoryPage4Catalog(en string, ed []byte, tn string, td []byte) (*StoryPage4Catalog, map[string]string, error) {
	e, err := readTSV(en, ed, storyPage3EventHeader)
	if err != nil {
		return nil, nil, err
	}
	t, err := readTSV(tn, td, storyOpeningTextHeader)
	if err != nil {
		return nil, nil, err
	}
	if len(e) != 6 || len(t) != 6 {
		return nil, nil, fmt.Errorf("buckrogers: 第 4 頁 catalog 必須各有六行")
	}
	tr := map[string]string{}
	for _, r := range t {
		if r[0] == "" || r[1] == "" || tr[r[0]] != "" {
			return nil, nil, fmt.Errorf("buckrogers: 第 4 頁譯文 key 無效")
		}
		tr[r[0]] = r[1]
	}
	c := &StoryPage4Catalog{}
	for i, r := range e {
		key := fmt.Sprintf("story.page4.line.%03d", i+1)
		n, x := strconv.Atoi(r[2])
		if r[0] != key || r[1] != strconv.Itoa(i+1) || x != nil || n != int(storyPage4Approved[i].n) || r[3] != storyPage4Approved[i].h || r[4] != "0763:04FF" || r[5] != "0763:026B" || r[6] != "0" || r[7] != "10" || r[8] != strconv.Itoa(17+i) || r[9] != "1" || r[12] != "confirmed" || r[13] != "READY" || tr[key] == "" {
			return nil, nil, fmt.Errorf("buckrogers: 第 4 頁非 READY identity")
		}
		b, x := hex.DecodeString(r[3])
		if x != nil || len(b) != 32 {
			return nil, nil, fmt.Errorf("buckrogers: 第 4 頁 hash 無效")
		}
		var h [32]byte
		copy(h[:], b)
		c.entries[i] = StoryPage4Identity{uint8(i + 1), uint8(n), 1, 1, 0, 10, uint8(17 + i), 1, key, h, Address{0x0763, 0x04ff}, storyGlyphPrimitive}
	}
	return c, tr, nil
}

type StoryPage4Event struct {
	Generation, EntryStep, PostCallStep uint64
	EventKey                            string
	Row, Column                         uint8
}
type page4Pending struct {
	e      StoryPage4Identity
	entry  uint64
	bytes  []byte
	ss, sp uint16
}
type StoryPage4Watcher struct {
	c            *StoryPage4Catalog
	p            *page4Pending
	done, events []StoryPage4Event
	active       bool
	generation   uint64
	frame        *page4Frame
}
type page4Frame struct {
	guard, caller Address
	ss, sp        uint16
	args          [7]uint16
	step          uint64
}

func NewStoryPage4Watcher(c *StoryPage4Catalog) (*StoryPage4Watcher, error) {
	if c == nil {
		return nil, fmt.Errorf("buckrogers: 第 4 頁缺 catalog")
	}
	return &StoryPage4Watcher{c: c, generation: 1}, nil
}
func (w *StoryPage4Watcher) ObserveGlyphEntry(guard, caller Address, ss, sp uint16, args [7]uint16, step uint64) {
	if w == nil || w.active {
		return
	}
	if w.frame != nil {
		w.drop()
	}
	for _, v := range args {
		if v > 255 {
			w.drop()
			return
		}
	}
	i := len(w.done)
	if w.p != nil {
		i = len(w.done)
	}
	if i >= 6 || guard != storyGlyphPrimitive || caller != (Address{0x0763, 0x04ff}) || args[0] != 1 || args[2] != 1 || args[3] != 0 || args[4] != 10 || args[5] != uint16(17+i) || args[6] != uint16(1+func() int {
		if w.p == nil {
			return 0
		}
		return len(w.p.bytes)
	}()) {
		w.drop()
		return
	}
	w.frame = &page4Frame{guard, caller, ss, sp, args, step}
}
func (w *StoryPage4Watcher) ObserveVerifiedGlyphReturn(previous Address, opcode byte, caller Address, ss, sp uint16, post uint64) {
	if w == nil || w.frame == nil {
		return
	}
	f := w.frame
	w.frame = nil
	if previous != (Address{0x0763, 0x03d6}) || opcode != 0xca || caller != f.caller || ss != f.ss || sp != f.sp+storyGlyphStackDelta || post <= f.step {
		w.drop()
		return
	}
	i := len(w.done)
	if w.p == nil {
		w.p = &page4Pending{e: w.c.entries[i], entry: f.step, ss: f.ss, sp: f.sp}
	}
	w.p.bytes = append(w.p.bytes, byte(f.args[1]))
	if len(w.p.bytes) == int(w.p.e.OriginalLength) {
		if sha256.Sum256(w.p.bytes) != w.p.e.OriginalSHA256 {
			w.drop()
			return
		}
		w.done = append(w.done, StoryPage4Event{w.generation, w.p.entry, post, w.p.e.EventKey, w.p.e.Row, 1})
		w.p = nil
		if len(w.done) == 6 {
			w.events = append(w.events, w.done...)
			w.done = nil
			w.active = true
		}
	}
}
func (w *StoryPage4Watcher) drop() { w.p = nil; w.frame = nil; w.done = nil }
func (w *StoryPage4Watcher) ObserveExecutionDiscontinuity() {
	if w != nil {
		w.drop()
		if w.active {
			w.active = false
			w.generation++
		}
	}
}
func StoryPage4VideoSpanIntersects(di, count uint16) bool {
	const top, bottom, left, right uint32 = 136, 184, 8, 320
	if count == 0 {
		return false
	}
	s, e := uint32(di), uint32(di)+uint32(count)
	if e <= top*320 || s >= bottom*320 {
		return false
	}
	if s < top*320 {
		s = top * 320
	}
	if e > bottom*320 {
		e = bottom * 320
	}
	for r := s / 320; r <= (e-1)/320; r++ {
		a, b := s, e
		if a < r*320 {
			a = r * 320
		}
		if b > (r+1)*320 {
			b = (r + 1) * 320
		}
		if a-r*320 < right && b-r*320 > left {
			return true
		}
	}
	return false
}
func (w *StoryPage4Watcher) ObserveVideoWrite(at Address, es, di, count uint16) bool {
	if w == nil || at != storyFillInstruction || es != storyVideoSegment || !StoryPage4VideoSpanIntersects(di, count) {
		return false
	}
	was := w.active || w.p != nil || len(w.done) > 0
	w.drop()
	w.active = false
	if was {
		w.generation++
	}
	return was
}
func (w *StoryPage4Watcher) Events() []StoryPage4Event {
	return append([]StoryPage4Event(nil), w.events...)
}
func (w *StoryPage4Watcher) Active() bool { return w != nil && w.active }

type RuntimeStoryPage4Overlay struct {
	layer  *xlate.Layer
	font   *xlate.Font
	text   map[string]string
	scale  int
	active bool
}

func NewRuntimeStoryPage4Overlay(t map[string]string, f *xlate.Font, scale int) (*RuntimeStoryPage4Overlay, error) {
	if f == nil || f.W != 16 || f.H != 16 || (scale != 2 && scale != 3) || len(t) != 6 {
		return nil, fmt.Errorf("buckrogers: 第 4 頁 presenter 輸入無效")
	}
	for _, s := range t {
		for _, r := range s {
			if g, ok := f.Glyphs[r]; !ok || len(g) != 32 {
				return nil, fmt.Errorf("buckrogers: 第 4 頁缺字")
			}
		}
	}
	return &RuntimeStoryPage4Overlay{&xlate.Layer{W: 320, H: 200}, f, t, scale, false}, nil
}
func (o *RuntimeStoryPage4Overlay) Apply(es []StoryPage4Event, p [256][3]uint8) error {
	if o == nil || o.active || len(es) != 6 {
		return fmt.Errorf("buckrogers: 第 4 頁 apply 無效")
	}
	g := es[0].Generation
	if g == 0 {
		return fmt.Errorf("buckrogers: 第 4 頁 generation 無效")
	}
	for i, e := range es {
		if e.Generation != g || e.EventKey != fmt.Sprintf("story.page4.line.%03d", i+1) || e.Row != uint8(17+i) || e.Column != 1 || o.text[e.EventKey] == "" {
			return fmt.Errorf("buckrogers: 第 4 頁 event 無效")
		}
		o.layer.Add(&xlate.Stamp{Key: e.EventKey, X: 8, Y: int(e.Row) * 8, Cells: 39, CellW: 8, CellH: 8, Font: o.font, Text: []rune(o.text[e.EventKey]), State: xlate.Shown, BG: p[0], FG: p[10]})
	}
	o.active = true
	return nil
}
func (o *RuntimeStoryPage4Overlay) Clear() {
	if o != nil {
		o.layer.Clear(8, 136, 320, 184)
		o.active = false
	}
}
