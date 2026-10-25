package strikecommander

import (
	"bytes"
	"fmt"
	"image"
)

const HDPolicyVersion = "hd-policy-2145000-63-v1"
const HDStaleSteps uint64 = 2145000
const HDPaletteShiftMax = 63

type HDPolicy struct {
	StaleSteps      uint64
	PaletteShiftMax int
}

func FixedHDPolicy() HDPolicy { return HDPolicy{HDStaleSteps, HDPaletteShiftMax} }
func hdStale(now, lastPromote uint64, changed bool, p HDPolicy) bool {
	return now >= lastPromote && now-lastPromote > p.StaleSteps && changed
}
func paletteShift(a, b [256][3]uint8) int {
	n := 0
	for i, v := range a {
		if v != b[i] {
			n++
		}
	}
	return n
}

// hdPolicyState belongs to exactly one immutable CU ref. The first promotion
// Frame establishes the baseline; later Frames can only revoke that ref.
type hdPolicyState struct {
	promoted, everChanged, revoked bool
	promotionSteps                 uint64
	viewport                       image.Rectangle
	baseline                       [64000]byte
	palette                        [256][3]uint8
	reason                         string
}

func (s *hdPolicyState) frame(steps uint64, indexed []byte, palette [256][3]uint8) error {
	if len(indexed) != 64000 {
		return fmt.Errorf("HD policy Frame length")
	}
	if !s.promoted {
		s.promoted = true
		s.promotionSteps = steps
		copy(s.baseline[:], indexed)
		s.palette = palette
		return nil
	}
	box := s.viewport.Intersect(image.Rect(0, 0, 320, 200))
	for y := box.Min.Y; y < box.Max.Y && !s.everChanged; y++ {
		at := y*320 + box.Min.X
		if !bytes.Equal(s.baseline[at:at+box.Dx()], indexed[at:at+box.Dx()]) {
			s.everChanged = true
		}
	}
	if hdStale(steps, s.promotionSteps, s.everChanged, FixedHDPolicy()) {
		s.revoked = true
		s.reason = "stale-source"
	}
	s.checkPalette(palette)
	return nil
}
func (s *hdPolicyState) checkPalette(p [256][3]uint8) bool {
	if s.promoted && paletteShift(p, s.palette) > HDPaletteShiftMax {
		s.revoked = true
		s.reason = "palette-shift"
	}
	return !s.revoked
}

type HDEligibility uint8

const (
	HDNoImage HDEligibility = iota
	HDStale
	HDEligible
)

type HDSafetyState struct {
	Eligibility               HDEligibility
	Version, FrameCompleteKey uint64
}
type hdSafetyTracker struct {
	state    HDSafetyState
	seq      uint64
	hasFrame bool
	frame    [64000]byte
	palette  [256][3]uint8
}

func (s *hdSafetyTracker) set(class HDEligibility) {
	if s.state.Version == 0 || s.state.Eligibility != class {
		s.state.Version++
		s.state.Eligibility = class
	}
}

// A proven completion capture is independent of a later VGA exemption. A
// repeated Frame reason/Steps never advances display qualification by itself.
func (s *hdSafetyTracker) capture(seq uint64, complete bool) {
	if s.seq != seq {
		s.seq = seq
		s.state.Version++
	}
	if complete {
		s.set(HDEligible)
	} else if seq == 0 {
		s.set(HDNoImage)
	}
}
func (s *hdSafetyTracker) verdict(v FrameVerdict, indexed []byte, pal [256][3]uint8) {
	if v.FrameComplete && len(indexed) == 64000 {
		if !s.hasFrame || !bytes.Equal(s.frame[:], indexed) || s.palette != pal {
			s.hasFrame = true
			copy(s.frame[:], indexed)
			s.palette = pal
			s.state.FrameCompleteKey++
			s.state.Version++
		}
	}
	switch v.Class {
	case VerdictNoImage:
		s.set(HDNoImage)
	case VerdictStale:
		s.set(HDStale)
	case VerdictEligible:
		s.set(HDEligible)
	}
	// Exempt retains the existing qualification. Ref/CU revocation and every
	// capture's completion proof remain separate mandatory HD conditions.
}
