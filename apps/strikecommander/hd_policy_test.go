package strikecommander

import (
	"image"
	"testing"
)

func TestHDPolicyExactThresholdAndPersistentChange(t *testing.T) {
	state := hdPolicyState{viewport: image.Rect(1, 1, 3, 3)}
	frame := make([]byte, 64000)
	var pal [256][3]uint8
	if e := state.frame(100, frame, pal); e != nil {
		t.Fatal(e)
	}
	// Static pause never expires, even far beyondT.
	state.frame(100+HDStaleSteps*10, frame, pal)
	if state.revoked {
		t.Fatal("static pause expired")
	}
	state = hdPolicyState{viewport: image.Rect(1, 1, 3, 3)}
	state.frame(100, frame, pal)
	frame[321] = 1
	state.frame(100+HDStaleSteps, frame, pal)
	if state.revoked || !state.everChanged {
		t.Fatal("threshold equality")
	}
	frame[321] = 0
	state.frame(101+HDStaleSteps, frame, pal)
	if !state.revoked {
		t.Fatal("changed-then-restored viewport erased history")
	}
	state = hdPolicyState{viewport: image.Rect(1, 1, 3, 3)}
	state.frame(500, frame, pal)
	if state.revoked {
		t.Fatal("new group failed recovery")
	}
	for i := 0; i < 63; i++ {
		pal[i][0] = 1
	}
	if !state.checkPalette(pal) {
		t.Fatal("63 rejected")
	}
	pal[63][0] = 1
	if state.checkPalette(pal) {
		t.Fatal("64 accepted")
	}
	pal = [256][3]uint8{}
	if state.checkPalette(pal) {
		t.Fatal("palette restore revived old ref")
	}
}
func TestHDSafetyTransitionsAndIncompleteFrameExemption(t *testing.T) {
	var s hdSafetyTracker
	frame := make([]byte, 64000)
	var pal [256][3]uint8
	s.capture(1, true)
	s.verdict(FrameVerdict{Seq: 1, Class: VerdictEligible, FrameComplete: true}, frame, pal)
	base := s.state
	s.verdict(FrameVerdict{Seq: 1, Steps: 999, Class: VerdictEligible, FrameComplete: true}, frame, pal)
	if s.state != base {
		t.Fatal("Frame Steps invented new safety version")
	}
	frame[0] = 9
	s.verdict(FrameVerdict{Seq: 1, Class: VerdictExempt, Reason: ExemptInCU, FrameComplete: false}, frame, pal)
	if s.state != base || s.frame[0] != 0 {
		t.Fatal("saved intermediate CU Frame")
	}
	s.verdict(FrameVerdict{Seq: 1, Class: VerdictStale, FrameComplete: true}, frame, pal)
	stale := s.state
	if stale.Eligibility != HDStale || stale.Version <= base.Version {
		t.Fatal(stale)
	}
	s.verdict(FrameVerdict{Seq: 1, Class: VerdictEligible, FrameComplete: true}, frame, pal)
	if s.state.Eligibility != HDEligible || s.state.Version <= stale.Version || s.state.FrameCompleteKey != stale.FrameCompleteKey {
		t.Fatal("sameSeq stale->eligible lacks distinct safety key", s.state)
	}
}
