package buckrogers

import (
	"crypto/sha256"
	"fmt"
)

// TextEvent is answer-free, content-free metadata for one completed original
// dispatcher call. Original bytes are represented only by length and SHA-256.
type TextEvent struct {
	EntryStep      uint64   `json:"entry_step"`
	PostCallStep   uint64   `json:"post_call_step"`
	Caller         Address  `json:"caller"`
	OriginalLength uint8    `json:"original_length"`
	OriginalSHA256 [32]byte `json:"original_sha256"`
	Background     uint8    `json:"background"`
	Foreground     uint8    `json:"foreground"`
	Row            uint8    `json:"row"`
	Column         uint8    `json:"column"`
	// Affix is set only for callers registered with RegisterAffix. It keeps
	// the hashes of a fixed prefix and suffix plus the variable slot length;
	// the slot bytes (a player-entered name) are never retained.
	Affix *TextAffix `json:"affix,omitempty"`
}

// TextAffix is the content-safe view of a prefix + variable slot + suffix call.
type TextAffix struct {
	PrefixSHA256 [32]byte `json:"prefix_sha256"`
	SuffixSHA256 [32]byte `json:"suffix_sha256"`
	SlotLength   uint8    `json:"slot_length"`
}

// AffixShape registers one caller whose string is a fixed-length prefix, a
// variable slot of at least one byte, and a fixed-length suffix.
type AffixShape struct {
	Caller       Address
	PrefixLength int
	SuffixLength int
}

type textFrame struct {
	event   TextEvent
	ss      uint16
	entrySP uint16
}

// TextRecorder records dispatcher metadata without translating, drawing, or
// retaining original text. It is an observation helper, not an overlay.
type TextRecorder struct {
	pending *textFrame
	events  []TextEvent
	drops   int
	affixes map[Address]AffixShape
}

// RegisterAffix enables affix hashing for one caller. Registering the same
// caller twice or an empty prefix/suffix is rejected.
func (r *TextRecorder) RegisterAffix(s AffixShape) error {
	if s.PrefixLength <= 0 || s.SuffixLength <= 0 || s.PrefixLength+s.SuffixLength >= 255 {
		return fmt.Errorf("affix shape invalid")
	}
	if r.affixes == nil {
		r.affixes = map[Address]AffixShape{}
	}
	if _, ok := r.affixes[s.Caller]; ok {
		return fmt.Errorf("affix caller registered twice")
	}
	r.affixes[s.Caller] = s
	return nil
}

// ObserveDispatchEntry starts one frame. args are the six raw 16-bit words
// accepted by 0763:0424; visual fields intentionally use their proven low byte.
func (r *TextRecorder) ObserveDispatchEntry(caller Address, ss, sp uint16, args [6]uint16, original []byte, step uint64) {
	if r.pending != nil || len(original) > 255 {
		r.pending = nil
		r.drops++
		return
	}
	r.pending = &textFrame{
		event: TextEvent{
			EntryStep: step, Caller: caller, OriginalLength: uint8(len(original)),
			OriginalSHA256: sha256.Sum256(original), Background: uint8(args[2]),
			Foreground: uint8(args[3]), Row: uint8(args[4]), Column: uint8(args[5]),
		},
		ss: ss, entrySP: sp,
	}
	if shape, ok := r.affixes[caller]; ok && len(original) > shape.PrefixLength+shape.SuffixLength {
		r.pending.event.Affix = &TextAffix{
			PrefixSHA256: sha256.Sum256(original[:shape.PrefixLength]),
			SuffixSHA256: sha256.Sum256(original[len(original)-shape.SuffixLength:]),
			SlotLength:   uint8(len(original) - shape.PrefixLength - shape.SuffixLength),
		}
	}
}

// ObserveInstruction completes only the exact far-return/SS/SP guard.
func (r *TextRecorder) ObserveInstruction(at Address, ss, sp uint16, step uint64) {
	f := r.pending
	if f == nil || at != f.event.Caller {
		return
	}
	if ss != f.ss || sp != f.entrySP+dispatcherStackDelta {
		r.pending = nil
		r.drops++
		return
	}
	f.event.PostCallStep = step
	r.events = append(r.events, f.event)
	r.pending = nil
}

func (r *TextRecorder) Events() []TextEvent { return append([]TextEvent(nil), r.events...) }
func (r *TextRecorder) Drops() int          { return r.drops }
func (r *TextRecorder) Pending() bool       { return r.pending != nil }
