package buckrogers

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/wicanr2/dosgolem/internal/machine"
	"github.com/wicanr2/dosgolem/xlate"
)

// storyFamily adapts one story page watcher and its two scale presenters to
// the receipt runner's glyph protocol.  All pages see the same 0763:026B glyph
// calls, so the live runtime keeps one shared in-flight frame for them.
type storyFamily interface {
	name() string
	glyphEntry(caller Address, ss, sp uint16, args [7]uint16, step uint64)
	verifiedReturn(entryStep uint64, caller Address, prev Address, opcode byte, ss, sp uint16, step uint64)
	discontinuity()
	// clearWrite is the 0CF4:1B3A REP STOSB; it reports an invalidation.
	clearWrite(at Address, es, di, cx uint16) bool
	prewrite(w machine.VideoWrite)
	apply(palette [256][3]uint8) error
	frame(indexed []byte, palette [256][3]uint8)
	draw(i int, indexed []byte, palette [256][3]uint8) ([]byte, []rune, bool)
	clear()
}

var (
	storyGlyphReturnSite = Address{Segment: 0x0763, Offset: 0x03D6}
	storyClearWrite      = Address{Segment: 0x0CF4, Offset: 0x1B3A}
)

const storyReturnStackDelta = 0x12

// storyPages loads every story family from the text directory.
func storyPages(textDir string, font *xlate.Font) ([]storyFamily, error) {
	read := func(name string) ([]byte, error) {
		b, err := os.ReadFile(filepath.Join(textDir, name))
		if err != nil {
			return nil, fmt.Errorf("buckrogers: 讀取 %s：%w", name, err)
		}
		return b, nil
	}
	pair := func(prefix string) (string, []byte, string, []byte, error) {
		en, tn := prefix+"-events.tsv", prefix+".zh-TW.tsv"
		ed, err := read(en)
		if err != nil {
			return "", nil, "", nil, err
		}
		td, err := read(tn)
		return en, ed, tn, td, err
	}
	var out []storyFamily
	{
		en, ed, tn, td, err := pair("story-opening")
		if err != nil {
			return nil, err
		}
		c, text, err := LoadStoryOpeningCatalog(en, ed, tn, td)
		if err != nil {
			return nil, err
		}
		f := &storyOpeningLive{catalog: c}
		for i, s := range liveScales {
			if f.pres[i], err = NewRuntimeStoryOpeningOverlay(text, font, s); err != nil {
				return nil, err
			}
		}
		if f.w, err = NewStoryOpeningWatcher(c); err != nil {
			return nil, err
		}
		out = append(out, f)
	}
	{
		en, ed, tn, td, err := pair("story-page2")
		if err != nil {
			return nil, err
		}
		c, text, err := LoadStoryPage2Catalog(en, ed, tn, td)
		if err != nil {
			return nil, err
		}
		f := &storyPage2Live{}
		for i, s := range liveScales {
			if f.pres[i], err = NewRuntimeStoryPage2Overlay(text, font, s); err != nil {
				return nil, err
			}
		}
		if f.w, err = NewStoryPage2Watcher(c); err != nil {
			return nil, err
		}
		out = append(out, f)
	}
	{
		en, ed, tn, td, err := pair("story-page3")
		if err != nil {
			return nil, err
		}
		c, text, err := LoadStoryPage3Catalog(en, ed, tn, td)
		if err != nil {
			return nil, err
		}
		f := &storyPage3Live{}
		for i, s := range liveScales {
			if f.pres[i], err = NewRuntimeStoryPage3Overlay(text, font, s); err != nil {
				return nil, err
			}
		}
		if f.w, err = NewStoryPage3Watcher(c); err != nil {
			return nil, err
		}
		out = append(out, f)
	}
	{
		en, ed, tn, td, err := pair("story-page4")
		if err != nil {
			return nil, err
		}
		c, text, err := LoadStoryPage4Catalog(en, ed, tn, td)
		if err != nil {
			return nil, err
		}
		f := &storyPage4Live{}
		for i, s := range liveScales {
			if f.pres[i], err = NewRuntimeStoryPage4Overlay(text, font, s); err != nil {
				return nil, err
			}
		}
		if f.w, err = NewStoryPage4Watcher(c); err != nil {
			return nil, err
		}
		out = append(out, f)
	}
	{
		en, ed, tn, td, err := pair("story-page5")
		if err != nil {
			return nil, err
		}
		c, text, err := LoadStoryPage5Catalog(en, ed, tn, td)
		if err != nil {
			return nil, err
		}
		f := &storyPage5Live{}
		for i, s := range liveScales {
			if f.pres[i], err = NewRuntimeStoryPage5Overlay(text, font, s); err != nil {
				return nil, err
			}
		}
		if f.w, err = NewStoryPage5Watcher(c); err != nil {
			return nil, err
		}
		out = append(out, f)
	}
	{
		en, ed, tn, td, err := pair("story-page6")
		if err != nil {
			return nil, err
		}
		c, text, err := LoadStoryPage6Catalog(en, ed, tn, td)
		if err != nil {
			return nil, err
		}
		f := &storyPage6Live{}
		for i, s := range liveScales {
			if f.pres[i], err = NewRuntimeStoryPage6Overlay(text, font, s); err != nil {
				return nil, err
			}
		}
		if f.w, err = NewStoryPage6Watcher(c); err != nil {
			return nil, err
		}
		out = append(out, f)
	}
	{
		en, ed, tn, td, err := pair("story-page7")
		if err != nil {
			return nil, err
		}
		c, text, err := LoadStoryPage7Catalog(en, ed, tn, td)
		if err != nil {
			return nil, err
		}
		f := &storyPage7Live{}
		for i, s := range liveScales {
			if f.pres[i], err = NewRuntimeStoryPage7Overlay(text, font, s); err != nil {
				return nil, err
			}
		}
		if f.w, err = NewStoryPage7Watcher(c); err != nil {
			return nil, err
		}
		out = append(out, f)
	}
	{
		en, ed, tn, td, err := pair("story-page8")
		if err != nil {
			return nil, err
		}
		c, text, err := LoadStoryPage8Catalog(en, ed, tn, td)
		if err != nil {
			return nil, err
		}
		f := &storyPage8Live{}
		for i, s := range liveScales {
			if f.pres[i], err = NewRuntimeStoryPage8Overlay(text, font, s); err != nil {
				return nil, err
			}
		}
		if f.w, err = NewStoryPage8Watcher(c); err != nil {
			return nil, err
		}
		out = append(out, f)
	}
	{
		en, ed, tn, td, err := pair("story-page9")
		if err != nil {
			return nil, err
		}
		c, text, err := LoadStoryPage9Catalog(en, ed, tn, td)
		if err != nil {
			return nil, err
		}
		f := &storyPage9Live{}
		for i, s := range liveScales {
			w, err := NewStoryPage9Watcher(c)
			if err != nil {
				return nil, err
			}
			p, err := NewRuntimeStoryPage9Overlay(text, font, s)
			if err != nil {
				return nil, err
			}
			if f.owner[i], err = NewStoryPage9Owner(w, p); err != nil {
				return nil, err
			}
		}
		out = append(out, f)
	}
	return out, nil
}

// --- opening ---------------------------------------------------------------

type storyOpeningLive struct {
	catalog *StoryOpeningCatalog
	w       *StoryOpeningWatcher
	pres    [2]*RuntimeStoryOpeningOverlay
	gen     uint64
}

func (f *storyOpeningLive) name() string { return "story-opening" }
func (f *storyOpeningLive) glyphEntry(caller Address, ss, sp uint16, args [7]uint16, step uint64) {
	f.w.ObserveGlyphEntry(glyphEntry, caller, ss, sp, args, step)
}
func (f *storyOpeningLive) verifiedReturn(entry uint64, caller, _ Address, _ byte, ss, sp uint16, step uint64) {
	f.w.ObserveVerifiedGlyphReturn(StoryOpeningVerifiedReturn{EntryStep: entry, PostCallStep: step, Caller: caller, SS: ss, SP: sp})
}
func (f *storyOpeningLive) discontinuity() { f.w.ObserveExecutionDiscontinuity() }
func (f *storyOpeningLive) clearWrite(at Address, es, di, cx uint16) bool {
	if f.w.ObserveVideoWrite(at, es, di, cx) {
		f.clear()
		return true
	}
	return false
}
func (f *storyOpeningLive) prewrite(machine.VideoWrite) {}
func (f *storyOpeningLive) apply(p [256][3]uint8) error {
	if !f.w.Active() || f.w.Generation() == f.gen {
		return nil
	}
	var events []StoryOpeningEvent
	for _, e := range f.w.Events() {
		if e.Generation == f.w.Generation() {
			events = append(events, e)
		}
	}
	for i := range liveScales {
		if err := f.pres[i].Apply(events, p); err != nil {
			return err
		}
	}
	f.gen = f.w.Generation()
	return nil
}
func (f *storyOpeningLive) frame(ix []byte, p [256][3]uint8) {
	for i := range liveScales {
		f.pres[i].Frame(ix, p)
	}
}
func (f *storyOpeningLive) draw(i int, ix []byte, p [256][3]uint8) ([]byte, []rune, bool) {
	if len(f.pres[i].ActiveKeys()) == 0 {
		return nil, nil, false
	}
	rgba, missing, _ := f.pres[i].Draw(ix, p)
	return rgba, missing, true
}
func (f *storyOpeningLive) clear() {
	for i := range liveScales {
		f.pres[i].Clear()
	}
	f.gen = 0
}

// --- pages 2, 3, 5: struct-shaped verified returns -------------------------

type storyPage2Live struct {
	w    *StoryPage2Watcher
	pres [2]*RuntimeStoryPage2Overlay
	gen  uint64
}

func (f *storyPage2Live) name() string { return "story-page2" }
func (f *storyPage2Live) glyphEntry(c Address, ss, sp uint16, a [7]uint16, step uint64) {
	f.w.ObserveGlyphEntry(glyphEntry, c, ss, sp, a, step)
}
func (f *storyPage2Live) verifiedReturn(entry uint64, caller, _ Address, _ byte, ss, sp uint16, step uint64) {
	f.w.ObserveVerifiedGlyphReturn(StoryPage2VerifiedReturn{EntryStep: entry, PostCallStep: step, Caller: caller, SS: ss, SP: sp})
}
func (f *storyPage2Live) discontinuity() { f.w.ObserveExecutionDiscontinuity() }
func (f *storyPage2Live) clearWrite(at Address, es, di, cx uint16) bool {
	if f.w.ObserveVideoWrite(at, es, di, cx) {
		f.clear()
		return true
	}
	return false
}
func (f *storyPage2Live) prewrite(machine.VideoWrite) {}
func (f *storyPage2Live) apply(p [256][3]uint8) error {
	if !f.w.Active() || f.w.Generation() == f.gen {
		return nil
	}
	var es []StoryPage2Event
	for _, e := range f.w.Events() {
		if e.Generation == f.w.Generation() {
			es = append(es, e)
		}
	}
	for i := range liveScales {
		if err := f.pres[i].Apply(es, p); err != nil {
			return err
		}
	}
	f.gen = f.w.Generation()
	return nil
}
func (f *storyPage2Live) frame(ix []byte, p [256][3]uint8) {
	for i := range liveScales {
		f.pres[i].Frame(ix, p)
	}
}
func (f *storyPage2Live) draw(i int, ix []byte, p [256][3]uint8) ([]byte, []rune, bool) {
	if len(f.pres[i].ActiveKeys()) == 0 {
		return nil, nil, false
	}
	rgba, missing, _ := f.pres[i].Draw(ix, p)
	return rgba, missing, true
}
func (f *storyPage2Live) clear() {
	for i := range liveScales {
		f.pres[i].Clear()
	}
	f.gen = 0
}

type storyPage3Live struct {
	w    *StoryPage3Watcher
	pres [2]*RuntimeStoryPage3Overlay
	gen  uint64
}

func (f *storyPage3Live) name() string { return "story-page3" }
func (f *storyPage3Live) glyphEntry(c Address, ss, sp uint16, a [7]uint16, step uint64) {
	f.w.ObserveGlyphEntry(glyphEntry, c, ss, sp, a, step)
}
func (f *storyPage3Live) verifiedReturn(entry uint64, caller, _ Address, _ byte, ss, sp uint16, step uint64) {
	f.w.ObserveVerifiedGlyphReturn(StoryPage3VerifiedReturn{EntryStep: entry, PostCallStep: step, Caller: caller, SS: ss, SP: sp})
}
func (f *storyPage3Live) discontinuity() { f.w.ObserveExecutionDiscontinuity() }
func (f *storyPage3Live) clearWrite(at Address, es, di, cx uint16) bool {
	if f.w.ObserveVideoWrite(at, es, di, cx) {
		f.clear()
		return true
	}
	return false
}
func (f *storyPage3Live) prewrite(machine.VideoWrite) {}
func (f *storyPage3Live) apply(p [256][3]uint8) error {
	if !f.w.Active() || f.w.Generation() == f.gen {
		return nil
	}
	var es []StoryPage3Event
	for _, e := range f.w.Events() {
		if e.Generation == f.w.Generation() {
			es = append(es, e)
		}
	}
	for i := range liveScales {
		if err := f.pres[i].Apply(es, p); err != nil {
			return err
		}
	}
	f.gen = f.w.Generation()
	return nil
}
func (f *storyPage3Live) frame(ix []byte, p [256][3]uint8) {
	for i := range liveScales {
		f.pres[i].Frame(ix, p)
	}
}
func (f *storyPage3Live) draw(i int, ix []byte, p [256][3]uint8) ([]byte, []rune, bool) {
	if len(f.pres[i].ActiveKeys()) == 0 {
		return nil, nil, false
	}
	rgba, missing, _ := f.pres[i].Draw(ix, p)
	return rgba, missing, true
}
func (f *storyPage3Live) clear() {
	for i := range liveScales {
		f.pres[i].Clear()
	}
	f.gen = 0
}

type storyPage5Live struct {
	w    *StoryPage5Watcher
	pres [2]*RuntimeStoryPage5Overlay
	gen  uint64
}

func (f *storyPage5Live) name() string { return "story-page5" }
func (f *storyPage5Live) glyphEntry(c Address, ss, sp uint16, a [7]uint16, step uint64) {
	f.w.ObserveGlyphEntry(glyphEntry, c, ss, sp, a, step)
}
func (f *storyPage5Live) verifiedReturn(entry uint64, caller, _ Address, _ byte, ss, sp uint16, step uint64) {
	f.w.ObserveVerifiedGlyphReturn(StoryPage5VerifiedReturn{EntryStep: entry, PostCallStep: step, Caller: caller, SS: ss, SP: sp})
}
func (f *storyPage5Live) discontinuity() { f.w.ObserveExecutionDiscontinuity() }
func (f *storyPage5Live) clearWrite(at Address, es, di, cx uint16) bool {
	if f.w.ObserveVideoWrite(at, es, di, cx) {
		f.clear()
		return true
	}
	return false
}
func (f *storyPage5Live) prewrite(machine.VideoWrite) {}
func (f *storyPage5Live) apply(p [256][3]uint8) error {
	if !f.w.Active() || f.w.Generation() == f.gen {
		return nil
	}
	var es []StoryPage5Event
	for _, e := range f.w.Events() {
		if e.Generation == f.w.Generation() {
			es = append(es, e)
		}
	}
	for i := range liveScales {
		if err := f.pres[i].Apply(es, p); err != nil {
			return err
		}
	}
	f.gen = f.w.Generation()
	return nil
}
func (f *storyPage5Live) frame(ix []byte, p [256][3]uint8) {
	for i := range liveScales {
		f.pres[i].Frame(ix, p)
	}
}
func (f *storyPage5Live) draw(i int, ix []byte, p [256][3]uint8) ([]byte, []rune, bool) {
	if len(f.pres[i].ActiveKeys()) == 0 {
		return nil, nil, false
	}
	rgba, missing, _ := f.pres[i].Draw(ix, p)
	return rgba, missing, true
}
func (f *storyPage5Live) clear() {
	for i := range liveScales {
		f.pres[i].Clear()
	}
	f.gen = 0
}

// --- pages 4, 6, 7, 8: instruction-shaped verified returns -----------------

type storyPage4Live struct {
	w    *StoryPage4Watcher
	pres [2]*RuntimeStoryPage4Overlay
	gen  uint64
}

func (f *storyPage4Live) name() string { return "story-page4" }
func (f *storyPage4Live) glyphEntry(c Address, ss, sp uint16, a [7]uint16, step uint64) {
	f.w.ObserveGlyphEntry(glyphEntry, c, ss, sp, a, step)
}
func (f *storyPage4Live) verifiedReturn(_ uint64, caller, prev Address, op byte, ss, sp uint16, step uint64) {
	f.w.ObserveVerifiedGlyphReturn(prev, op, caller, ss, sp, step)
}
func (f *storyPage4Live) discontinuity() { f.w.ObserveExecutionDiscontinuity() }
func (f *storyPage4Live) clearWrite(at Address, es, di, cx uint16) bool {
	if f.w.ObserveVideoWrite(at, es, di, cx) {
		f.clear()
		return true
	}
	return false
}
func (f *storyPage4Live) prewrite(machine.VideoWrite) {}
func (f *storyPage4Live) apply(p [256][3]uint8) error {
	if !f.w.Active() || f.w.Generation() == f.gen {
		return nil
	}
	var es []StoryPage4Event
	for _, e := range f.w.Events() {
		if e.Generation == f.w.Generation() {
			es = append(es, e)
		}
	}
	for i := range liveScales {
		if err := f.pres[i].Apply(es, p); err != nil {
			return err
		}
	}
	f.gen = f.w.Generation()
	return nil
}
func (f *storyPage4Live) frame(ix []byte, p [256][3]uint8) {
	for i := range liveScales {
		f.pres[i].Frame(ix, p)
	}
}
func (f *storyPage4Live) draw(i int, ix []byte, p [256][3]uint8) ([]byte, []rune, bool) {
	if len(f.pres[i].ActiveKeys()) == 0 {
		return nil, nil, false
	}
	rgba, missing, _ := f.pres[i].Draw(ix, p)
	return rgba, missing, true
}
func (f *storyPage4Live) clear() {
	for i := range liveScales {
		f.pres[i].Clear()
	}
	f.gen = 0
}

type storyPage6Live struct {
	w    *StoryPage6Watcher
	pres [2]*RuntimeStoryPage6Overlay
	gen  uint64
}

func (f *storyPage6Live) name() string { return "story-page6" }
func (f *storyPage6Live) glyphEntry(c Address, ss, sp uint16, a [7]uint16, step uint64) {
	f.w.ObserveGlyphEntry(glyphEntry, c, ss, sp, a, step)
}
func (f *storyPage6Live) verifiedReturn(_ uint64, caller, prev Address, op byte, ss, sp uint16, step uint64) {
	f.w.ObserveVerifiedGlyphReturn(prev, op, caller, ss, sp, step)
}
func (f *storyPage6Live) discontinuity() { f.w.ObserveExecutionDiscontinuity() }
func (f *storyPage6Live) clearWrite(at Address, es, di, cx uint16) bool {
	if f.w.ObserveVideoWrite(at, es, di, cx) {
		f.clear()
		return true
	}
	return false
}
func (f *storyPage6Live) prewrite(machine.VideoWrite) {}
func (f *storyPage6Live) apply(p [256][3]uint8) error {
	if !f.w.Active() || f.w.Generation() == f.gen {
		return nil
	}
	var es []StoryPage6Event
	for _, e := range f.w.Events() {
		if e.Generation == f.w.Generation() {
			es = append(es, e)
		}
	}
	for i := range liveScales {
		if err := f.pres[i].Apply(es, p); err != nil {
			return err
		}
	}
	f.gen = f.w.Generation()
	return nil
}
func (f *storyPage6Live) frame(ix []byte, p [256][3]uint8) {
	for i := range liveScales {
		f.pres[i].Frame(ix, p)
	}
}
func (f *storyPage6Live) draw(i int, ix []byte, p [256][3]uint8) ([]byte, []rune, bool) {
	if len(f.pres[i].ActiveKeys()) == 0 {
		return nil, nil, false
	}
	rgba, missing, _ := f.pres[i].Draw(ix, p)
	return rgba, missing, true
}
func (f *storyPage6Live) clear() {
	for i := range liveScales {
		f.pres[i].Clear()
	}
	f.gen = 0
}

type storyPage7Live struct {
	w    *StoryPage7Watcher
	pres [2]*RuntimeStoryPage7Overlay
	gen  uint64
}

func (f *storyPage7Live) name() string { return "story-page7" }
func (f *storyPage7Live) glyphEntry(c Address, ss, sp uint16, a [7]uint16, step uint64) {
	f.w.ObserveGlyphEntry(glyphEntry, c, ss, sp, a, step)
}
func (f *storyPage7Live) verifiedReturn(_ uint64, caller, prev Address, op byte, ss, sp uint16, step uint64) {
	f.w.ObserveVerifiedGlyphReturn(prev, op, caller, ss, sp, step)
}
func (f *storyPage7Live) discontinuity() { f.w.ObserveExecutionDiscontinuity() }
func (f *storyPage7Live) clearWrite(at Address, es, di, cx uint16) bool {
	if f.w.ObserveVideoWrite(at, es, di, cx) {
		f.clear()
		return true
	}
	return false
}
func (f *storyPage7Live) prewrite(machine.VideoWrite) {}
func (f *storyPage7Live) apply(p [256][3]uint8) error {
	if !f.w.Active() || f.w.Generation() == f.gen {
		return nil
	}
	var es []StoryPage7Event
	for _, e := range f.w.Events() {
		if e.Generation == f.w.Generation() {
			es = append(es, e)
		}
	}
	for i := range liveScales {
		if err := f.pres[i].Apply(es, p); err != nil {
			return err
		}
	}
	f.gen = f.w.Generation()
	return nil
}
func (f *storyPage7Live) frame(ix []byte, p [256][3]uint8) {
	for i := range liveScales {
		f.pres[i].Frame(ix, p)
	}
}
func (f *storyPage7Live) draw(i int, ix []byte, p [256][3]uint8) ([]byte, []rune, bool) {
	if len(f.pres[i].ActiveKeys()) == 0 {
		return nil, nil, false
	}
	rgba, missing, _ := f.pres[i].Draw(ix, p)
	return rgba, missing, true
}
func (f *storyPage7Live) clear() {
	for i := range liveScales {
		f.pres[i].Clear()
	}
	f.gen = 0
}

type storyPage8Live struct {
	w    *StoryPage8Watcher
	pres [2]*RuntimeStoryPage8Overlay
	gen  uint64
}

func (f *storyPage8Live) name() string { return "story-page8" }
func (f *storyPage8Live) glyphEntry(c Address, ss, sp uint16, a [7]uint16, step uint64) {
	f.w.ObserveGlyphEntry(glyphEntry, c, ss, sp, a, step)
}
func (f *storyPage8Live) verifiedReturn(_ uint64, caller, prev Address, op byte, ss, sp uint16, step uint64) {
	f.w.ObserveVerifiedGlyphReturn(prev, op, caller, ss, sp, step)
}
func (f *storyPage8Live) discontinuity() { f.w.ObserveExecutionDiscontinuity() }
func (f *storyPage8Live) clearWrite(at Address, es, di, cx uint16) bool {
	if f.w.ObserveVideoWrite(at, es, di, cx) {
		f.clear()
		return true
	}
	return false
}
func (f *storyPage8Live) prewrite(machine.VideoWrite) {}
func (f *storyPage8Live) apply(p [256][3]uint8) error {
	if !f.w.Active() || f.w.Generation() == f.gen {
		return nil
	}
	var es []StoryPage8Event
	for _, e := range f.w.Events() {
		if e.Generation == f.w.Generation() {
			es = append(es, e)
		}
	}
	for i := range liveScales {
		if err := f.pres[i].Apply(es, p); err != nil {
			return err
		}
	}
	f.gen = f.w.Generation()
	return nil
}
func (f *storyPage8Live) frame(ix []byte, p [256][3]uint8) {
	for i := range liveScales {
		f.pres[i].Frame(ix, p)
	}
}
func (f *storyPage8Live) draw(i int, ix []byte, p [256][3]uint8) ([]byte, []rune, bool) {
	if len(f.pres[i].ActiveKeys()) == 0 {
		return nil, nil, false
	}
	rgba, missing, _ := f.pres[i].Draw(ix, p)
	return rgba, missing, true
}
func (f *storyPage8Live) clear() {
	for i := range liveScales {
		f.pres[i].Clear()
	}
	f.gen = 0
}

// --- page 9: owner with A000 pre-write invalidation ------------------------

type storyPage9Live struct {
	owner [2]*StoryPage9Owner
	gen   [2]uint64
}

func (f *storyPage9Live) name() string { return "story-page9" }
func (f *storyPage9Live) glyphEntry(c Address, ss, sp uint16, a [7]uint16, step uint64) {
	for i := range liveScales {
		f.owner[i].Watcher.ObserveGlyphEntry(glyphEntry, c, ss, sp, a, step)
	}
}
func (f *storyPage9Live) verifiedReturn(_ uint64, caller, prev Address, op byte, ss, sp uint16, step uint64) {
	for i := range liveScales {
		f.owner[i].Watcher.ObserveVerifiedGlyphReturn(prev, op, caller, ss, sp, step)
	}
}
func (f *storyPage9Live) discontinuity() {
	for i := range liveScales {
		f.owner[i].ObserveExecutionDiscontinuity()
	}
}
func (f *storyPage9Live) clearWrite(Address, uint16, uint16, uint16) bool { return false }
func (f *storyPage9Live) prewrite(w machine.VideoWrite) {
	for i := range liveScales {
		if f.owner[i].Prewrite(w) {
			f.gen[i] = 0
		}
	}
}
func (f *storyPage9Live) apply(p [256][3]uint8) error {
	for i := range liveScales {
		w := f.owner[i].Watcher
		if !w.Active() || w.Generation() == f.gen[i] {
			continue
		}
		events := w.Events()
		if len(events) != 1 {
			return fmt.Errorf("buckrogers: 第 9 頁事件數量無效")
		}
		if err := f.owner[i].Presenter.Apply(events[0], p); err != nil {
			return err
		}
		f.gen[i] = w.Generation()
	}
	return nil
}
func (f *storyPage9Live) frame(ix []byte, p [256][3]uint8) {
	for i := range liveScales {
		f.owner[i].Presenter.Frame(ix, p)
	}
}
func (f *storyPage9Live) draw(i int, ix []byte, p [256][3]uint8) ([]byte, []rune, bool) {
	if len(f.owner[i].Presenter.ActiveKeys()) == 0 {
		return nil, nil, false
	}
	rgba, missing, _ := f.owner[i].Presenter.Draw(ix, p)
	return rgba, missing, true
}
func (f *storyPage9Live) clear() {
	for i := range liveScales {
		f.owner[i].Presenter.Clear()
		f.gen[i] = 0
	}
}
