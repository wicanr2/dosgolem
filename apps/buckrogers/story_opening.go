package buckrogers

import (
	"crypto/sha256"
	"fmt"
)

const (
	storyGlyphStackDelta = uint16(0x12)
	storyVideoSegment    = uint16(0xA000)
)

var storyFillInstruction = Address{Segment: 0x0CF4, Offset: 0x1B3A}
var storyGlyphPrimitive = Address{Segment: 0x0763, Offset: 0x026B}

// StoryOpeningIdentity is one content-safe, exact glyph-run identity. It is
// intentionally data-only: source bytes exist only inside the watcher until
// their SHA-256 has been verified.
type StoryOpeningIdentity struct {
	Sequence       uint8
	EventKey       string
	OriginalLength uint8
	OriginalSHA256 [32]byte
	Caller         Address
	Guard          Address
	Mode, Repeat   uint8
	Background     uint8
	Foreground     uint8
	Row, Column    uint8
}

// StoryOpeningCatalog is an immutable ordered five-line identity set.
type StoryOpeningCatalog struct{ entries [5]StoryOpeningIdentity }

// NewStoryOpeningCatalog checks the narrow, proven first-screen shape. TSV
// loading deliberately belongs to the later project-level READY gate.
func NewStoryOpeningCatalog(entries []StoryOpeningIdentity) (*StoryOpeningCatalog, error) {
	if len(entries) != 5 {
		return nil, fmt.Errorf("buckrogers: 首屏劇情 catalog 必須恰有五行")
	}
	var catalog StoryOpeningCatalog
	seen := make(map[string]bool, len(entries))
	for i, entry := range entries {
		if entry.Sequence != uint8(i+1) || entry.EventKey == "" || seen[entry.EventKey] || entry.OriginalLength == 0 ||
			entry.OriginalSHA256 == ([32]byte{}) || entry.Caller == (Address{}) || entry.Guard == (Address{}) ||
			entry.Caller != (Address{Segment: 0x0763, Offset: 0x04FF}) || entry.Guard != storyGlyphPrimitive ||
			entry.Mode != 1 || entry.Repeat != 1 || entry.Background != 0 || entry.Foreground != 10 ||
			entry.Row != uint8(17+i) || entry.Column != 1 {
			return nil, fmt.Errorf("buckrogers: 首屏劇情 identity %d 不符合已確認形狀", i+1)
		}
		seen[entry.EventKey] = true
		catalog.entries[i] = entry
	}
	return &catalog, nil
}

// StoryOpeningGlyphCall is one completed-guard candidate glyph invocation.
// Glyph is not exported through StoryOpeningEvent.
type StoryOpeningGlyphCall struct {
	EntryStep, PostCallStep             uint64
	Guard, Caller                       Address
	Mode, Glyph, Repeat                 uint8
	Background, Foreground, Row, Column uint8
}

// StoryOpeningVerifiedReturn is emitted only by an adapter that observed the
// actual far-return edge for the matching entry instruction. A later visit to
// Caller is not a return edge and must never be supplied here.
type StoryOpeningVerifiedReturn struct {
	EntryStep, PostCallStep uint64
	Caller                  Address
	SS, SP                  uint16
}

// StoryOpeningEvent is content-safe metadata for one fully matched line.
type StoryOpeningEvent struct {
	Generation     uint64   `json:"generation"`
	EntryStep      uint64   `json:"entry_step"`
	PostCallStep   uint64   `json:"post_call_step"`
	EventKey       string   `json:"event_key"`
	OriginalLength uint8    `json:"original_length"`
	OriginalSHA256 [32]byte `json:"original_sha256"`
	Background     uint8    `json:"background"`
	Foreground     uint8    `json:"foreground"`
	Row            uint8    `json:"row"`
	Column         uint8    `json:"column"`
}

type storyGlyphFrame struct {
	call   StoryOpeningGlyphCall
	ss, sp uint16
}

type storyLineCandidate struct {
	identity StoryOpeningIdentity
	entry    uint64
	bytes    []byte
}

// StoryOpeningWatcher has no machine, renderer, input, or file dependency.
// It only turns guarded glyph metadata into exact content-safe events.
type StoryOpeningWatcher struct {
	catalog    *StoryOpeningCatalog
	pending    *storyGlyphFrame
	candidate  *storyLineCandidate
	completed  []StoryOpeningEvent
	events     []StoryOpeningEvent
	generation uint64
	active     bool
	drops      int
	misses     int
}

func NewStoryOpeningWatcher(catalog *StoryOpeningCatalog) (*StoryOpeningWatcher, error) {
	if catalog == nil {
		return nil, fmt.Errorf("buckrogers: 首屏劇情 watcher 缺少 catalog")
	}
	return &StoryOpeningWatcher{catalog: catalog, generation: 1}, nil
}

// ObserveGlyphEntry snapshots the seven ABI words. The original glyph ABI
// consumes their low bytes; high bytes are deliberately excluded from this
// display identity.
func (w *StoryOpeningWatcher) ObserveGlyphEntry(guard, caller Address, ss, sp uint16, args [7]uint16, step uint64) {
	if w == nil {
		return
	}
	if w.pending != nil {
		w.pending = nil
		w.dropCandidate()
		w.drops++
	}
	w.pending = &storyGlyphFrame{call: StoryOpeningGlyphCall{
		EntryStep: step, Guard: guard, Caller: caller, Mode: uint8(args[0]), Glyph: uint8(args[1]), Repeat: uint8(args[2]),
		Background: uint8(args[3]), Foreground: uint8(args[4]), Row: uint8(args[5]), Column: uint8(args[6]),
	}, ss: ss, sp: sp}
}

// ObserveVerifiedGlyphReturn submits exactly one glyph only after the adapter
// has observed its actual far-return control-flow edge. It deliberately does
// not accept a generic instruction address: a later execution at Caller with
// the same SS/SP is insufficient proof that this pending call returned.
func (w *StoryOpeningWatcher) ObserveVerifiedGlyphReturn(ret StoryOpeningVerifiedReturn) {
	if w == nil || w.pending == nil || ret.Caller != w.pending.call.Caller || ret.EntryStep != w.pending.call.EntryStep {
		return
	}
	f := w.pending
	w.pending = nil
	if ret.PostCallStep <= f.call.EntryStep || ret.SS != f.ss || ret.SP != f.sp+storyGlyphStackDelta {
		w.dropCandidate()
		w.drops++
		return
	}
	f.call.PostCallStep = ret.PostCallStep
	w.observeGlyph(f.call)
}

// ObserveExecutionDiscontinuity must be called when the adapter can no longer
// prove that the pending glyph frame is still executing (for example a stop,
// state restore, or an unobserved control-flow handoff). It is deliberately
// not a step-count timeout: no instruction budget has been measured.
func (w *StoryOpeningWatcher) ObserveExecutionDiscontinuity() {
	if w == nil || w.pending == nil {
		return
	}
	w.pending = nil
	w.dropCandidate()
	w.drops++
}

func (w *StoryOpeningWatcher) observeGlyph(call StoryOpeningGlyphCall) {
	if w.candidate == nil {
		if w.active {
			// An active group is immutable until a proven video write removes it.
			return
		}
		entry := w.catalog.entries[len(w.completed)]
		if !storyGlyphMatches(entry, 0, call) {
			if len(w.completed) != 0 {
				w.dropCandidate()
				w.drops++
			}
			return
		}
		w.candidate = &storyLineCandidate{identity: entry, entry: call.EntryStep, bytes: []byte{call.Glyph}}
		if entry.OriginalLength == 1 {
			w.finishLine(call.PostCallStep)
		}
		return
	}

	candidate := w.candidate
	index := len(candidate.bytes)
	if !storyGlyphMatches(candidate.identity, index, call) {
		w.dropCandidate()
		w.drops++
		return
	}
	candidate.bytes = append(candidate.bytes, call.Glyph)
	if len(candidate.bytes) == int(candidate.identity.OriginalLength) {
		w.finishLine(call.PostCallStep)
	}
}

func storyGlyphMatches(identity StoryOpeningIdentity, index int, call StoryOpeningGlyphCall) bool {
	return call.Guard == identity.Guard && call.Caller == identity.Caller && call.Mode == identity.Mode && call.Repeat == identity.Repeat &&
		call.Background == identity.Background && call.Foreground == identity.Foreground &&
		call.Row == identity.Row && int(call.Column) == int(identity.Column)+index
}

func (w *StoryOpeningWatcher) finishLine(post uint64) {
	candidate := w.candidate
	w.candidate = nil
	if len(candidate.bytes) != int(candidate.identity.OriginalLength) || sha256.Sum256(candidate.bytes) != candidate.identity.OriginalSHA256 {
		w.completed = nil
		w.misses++
		return
	}
	w.completed = append(w.completed, StoryOpeningEvent{
		Generation: w.generation, EntryStep: candidate.entry, PostCallStep: post, EventKey: candidate.identity.EventKey,
		OriginalLength: candidate.identity.OriginalLength, OriginalSHA256: candidate.identity.OriginalSHA256,
		Background: candidate.identity.Background, Foreground: candidate.identity.Foreground,
		Row: candidate.identity.Row, Column: candidate.identity.Column,
	})
	if len(w.completed) != len(w.catalog.entries) {
		return
	}
	w.events = append(w.events, w.completed...)
	w.completed = nil
	w.active = true
}

func (w *StoryOpeningWatcher) dropCandidate() {
	if w == nil {
		return
	}
	w.candidate, w.completed = nil, nil
}

// ObserveVideoWrite invalidates only the proven fill instruction when its
// pre-execution video span intersects the story rectangle. Unknown writes are
// intentionally ignored; they can never revive an old overlay group.
func (w *StoryOpeningWatcher) ObserveVideoWrite(at Address, es, di, count uint16) bool {
	if w == nil || at != storyFillInstruction || es != storyVideoSegment || count == 0 ||
		!StoryOpeningVideoSpanIntersects(di, count) {
		return false
	}
	wasActive := w.active || w.candidate != nil || len(w.completed) != 0
	w.pending = nil
	w.dropCandidate()
	w.active = false
	if wasActive {
		w.generation++
	}
	return wasActive
}

// StoryOpeningVideoSpanIntersects tests a contiguous Mode 13h destination
// span against [8,320)x[136,176). Work is bounded to the 40 story rows even
// if a malformed caller supplies a large count.
func StoryOpeningVideoSpanIntersects(di, count uint16) bool {
	const screenWidth = uint32(320)
	const top, bottom = uint32(136), uint32(176)
	const left, right = uint32(8), uint32(320)
	if count == 0 {
		return false
	}
	start, end := uint32(di), uint32(di)+uint32(count)
	if end <= top*screenWidth || start >= bottom*screenWidth {
		return false
	}
	if start < top*screenWidth {
		start = top * screenWidth
	}
	if end > bottom*screenWidth {
		end = bottom * screenWidth
	}
	for row := start / screenWidth; row <= (end-1)/screenWidth; row++ {
		rowStart, rowEnd := row*screenWidth, (row+1)*screenWidth
		x0, x1 := start, end
		if x0 < rowStart {
			x0 = rowStart
		}
		if x1 > rowEnd {
			x1 = rowEnd
		}
		if x0-rowStart < right && x1-rowStart > left {
			return true
		}
	}
	return false
}

func (w *StoryOpeningWatcher) Events() []StoryOpeningEvent {
	if w == nil {
		return nil
	}
	return append([]StoryOpeningEvent(nil), w.events...)
}
func (w *StoryOpeningWatcher) Active() bool { return w != nil && w.active }
func (w *StoryOpeningWatcher) Pending() bool {
	return w != nil && (w.pending != nil || w.candidate != nil || len(w.completed) != 0)
}
func (w *StoryOpeningWatcher) Generation() uint64 {
	if w == nil {
		return 0
	}
	return w.generation
}
func (w *StoryOpeningWatcher) Drops() int {
	if w == nil {
		return 0
	}
	return w.drops
}
func (w *StoryOpeningWatcher) Misses() int {
	if w == nil {
		return 0
	}
	return w.misses
}
