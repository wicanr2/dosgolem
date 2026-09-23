package buckrogers

import (
	"bytes"
	"crypto/sha256"
	"testing"

	"github.com/wicanr2/dosgolem/internal/machine"
	"github.com/wicanr2/dosgolem/xlate"
)

func skillExitFixture(t *testing.T) (*SkillExitCatalog, TextEvent, TextEvent) {
	t.Helper()
	career := TextEvent{EntryStep: 10, Caller: Address{0x37F1, 0x101E}, OriginalLength: 33, OriginalSHA256: sha256.Sum256([]byte("career")), Background: 0, Foreground: 13, Row: 24, Column: 0}
	technical := career
	technical.EntryStep = 20
	technical.OriginalLength = 34
	technical.OriginalSHA256 = sha256.Sum256([]byte("technical"))
	c := &SkillExitCatalog{byIdentity: map[menuIdentity]skillExitEntry{skillExitIdentity(career): {"career.key", "career.key", "甲乙丙丁", SkillExitCareer, skillExitIdentity(career)}, skillExitIdentity(technical): {"technical.key", "technical.key", "甲乙丙丁", SkillExitTechnical, skillExitIdentity(technical)}}}
	return c, career, technical
}
func skillExitFont() *xlate.Font {
	return &xlate.Font{W: 16, H: 16, Glyphs: map[rune][]byte{'甲': bytes.Repeat([]byte{0xff}, 32), '乙': bytes.Repeat([]byte{0xff}, 32), '丙': bytes.Repeat([]byte{0xff}, 32), '丁': bytes.Repeat([]byte{0xff}, 32)}}
}

func TestSkillExitRequiresExactEntryAndGuardedReturn(t *testing.T) {
	c, career, _ := skillExitFixture(t)
	w, _ := NewSkillExitWatcher(c)
	if err := w.ObserveEntry(career); err != nil {
		t.Fatal(err)
	}
	bad := career
	bad.PostCallStep = 11
	bad.Caller.Offset++
	if _, err := w.ObserveReturn(bad); err == nil || !w.Failed() {
		t.Fatal("mismatched return must poison")
	}
	w, _ = NewSkillExitWatcher(c)
	if err := w.ObserveEntry(career); err != nil {
		t.Fatal(err)
	}
	done := career
	done.PostCallStep = 11
	if g, err := w.ObserveReturn(done); err != nil || g.Generation != 1 || g.Page != SkillExitCareer {
		t.Fatalf("return=%+v err=%v", g, err)
	}
}

func TestSkillExitIdentityFieldsReject(t *testing.T) {
	c, base, _ := skillExitFixture(t)
	for _, mutate := range []func(*TextEvent){func(e *TextEvent) { e.OriginalLength++ }, func(e *TextEvent) { e.OriginalSHA256[0]++ }, func(e *TextEvent) { e.Caller.Segment++ }, func(e *TextEvent) { e.Caller.Offset++ }, func(e *TextEvent) { e.Background++ }, func(e *TextEvent) { e.Foreground++ }, func(e *TextEvent) { e.Row++ }, func(e *TextEvent) { e.Column++ }} {
		w, _ := NewSkillExitWatcher(c)
		e := base
		mutate(&e)
		if err := w.ObserveEntry(e); err == nil || !w.Failed() {
			t.Fatal("identity mutation must poison")
		}
	}
}

func TestSkillExitPrewriteNormalizesAndClearsTechnicalSameValue(t *testing.T) {
	c, _, technical := skillExitFixture(t)
	w, _ := NewSkillExitWatcher(c)
	if err := w.ObserveEntry(technical); err != nil {
		t.Fatal(err)
	}
	technical.PostCallStep = 21
	if _, err := w.ObserveReturn(technical); err != nil {
		t.Fatal(err)
	}
	cleared, err := w.Prewrite(machine.VideoWrite{Offset: 0xF0A8, CS: 0x0CF4, IP: 0x1B3A, Value: 0})
	if err != nil || !cleared || w.Active() {
		t.Fatalf("technical same-value prewrite cleared=%v active=%v err=%v", cleared, w.Active(), err)
	}
	w, _ = NewSkillExitWatcher(c)
	technical.PostCallStep = 0
	if err := w.ObserveEntry(technical); err != nil {
		t.Fatal(err)
	}
	technical.PostCallStep = 22
	if _, err := w.ObserveReturn(technical); err != nil {
		t.Fatal(err)
	}
	if _, err := w.Prewrite(machine.VideoWrite{Offset: 0x10000}); err == nil || !w.Failed() {
		t.Fatal("out of range must fail closed")
	}
}

func TestSkillExitLifecycle(t *testing.T) {
	c, e, _ := skillExitFixture(t)
	for _, f := range []func(*SkillExitWatcher){(*SkillExitWatcher).Stop, (*SkillExitWatcher).Restore, (*SkillExitWatcher).Discontinuity, (*SkillExitWatcher).Fault} {
		w, _ := NewSkillExitWatcher(c)
		if err := w.ObserveEntry(e); err != nil {
			t.Fatal(err)
		}
		f(w)
		if w.pending != nil || w.active != nil {
			t.Fatal("lifecycle must clear pending/active")
		}
		if w.state == skillExitClosed || w.state == skillExitFailed {
			if err := w.ObserveEntry(e); err == nil {
				t.Fatal("terminal owner rearmed")
			}
		}
	}
}

func TestSkillExitOverlayNeverTouchesTailAtTwoOrThreeX(t *testing.T) {
	for _, scale := range []int{2, 3} {
		o, _ := NewRuntimeSkillExitOverlay(skillExitFont(), scale)
		p := [256][3]uint8{}
		p[13] = [3]uint8{255, 255, 255}
		indexed := make([]byte, 320*200)
		baseline := ScaleIndexedRGBA(indexed, p, scale)
		if err := o.Apply(SkillExitGeneration{1, SkillExitTechnical, "technical.key", "甲乙丙丁"}, p); err != nil {
			t.Fatal(err)
		}
		got, _, _ := o.Draw(indexed, p)
		// technical tail is [272,320)×[192,200), and must remain byte-identical.
		for y := 192 * scale; y < 200*scale; y++ {
			a := (y*(320*scale) + 272*scale) * 4
			b := (y*(320*scale) + 320*scale) * 4
			if !bytes.Equal(got[a:b], baseline[a:b]) {
				t.Fatalf("scale %d tail changed at y=%d", scale, y)
			}
		}
		o.Clear()
		got, _, _ = o.Draw(indexed, p)
		if !bytes.Equal(got, baseline) {
			t.Fatalf("scale %d clear must leave zero layer", scale)
		}
	}
}

func TestSkillExitOwnerDuplicateOrUnknownEntryClearsActiveLayer(t *testing.T) {
	c, career, _ := skillExitFixture(t)
	for _, mutate := range []func(*TextEvent){func(*TextEvent) {}, func(e *TextEvent) { e.OriginalLength++ }} {
		o, err := NewSkillExitOwner(c, skillExitFont(), 2)
		if err != nil {
			t.Fatal(err)
		}
		p := [256][3]uint8{}
		p[13] = [3]uint8{255, 255, 255}
		if err := o.ObserveEntry(career); err != nil {
			t.Fatal(err)
		}
		done := career
		done.PostCallStep = 11
		if err := o.ObserveReturn(done, p); err != nil {
			t.Fatal(err)
		}
		duplicate := career
		mutate(&duplicate)
		if err := o.ObserveEntry(duplicate); err == nil || !o.Watcher.Failed() {
			t.Fatal("duplicate/unknown entry must poison owner")
		}
		baseline := ScaleIndexedRGBA(make([]byte, 320*200), p, 2)
		got, _, _ := o.Presenter.Draw(make([]byte, 320*200), p)
		if !bytes.Equal(got, baseline) {
			t.Fatal("entry failure must synchronously clear presenter")
		}
	}
}
