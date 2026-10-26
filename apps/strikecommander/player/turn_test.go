package player

import (
	"github.com/wicanr2/dosgolem/session"
	"testing"
	"time"
)

type fakeSession struct {
	calls []string
	u     session.CapturedUpdate
}

func (f *fakeSession) View() (session.View, error) {
	f.calls = append(f.calls, "View")
	return session.View{SourceGeneration: 7}, nil
}
func (f *fakeSession) Deliver(u session.CapturedUpdate) (session.InputReceipt, error) {
	f.calls = append(f.calls, "Deliver")
	f.u = u
	return session.InputReceipt{Epoch: 3}, nil
}
func (f *fakeSession) Advance(n session.InstructionBudget) (session.TickReceipt, error) {
	f.calls = append(f.calls, "Advance")
	return session.TickReceipt{Budget: n, Steps: uint64(n)}, nil
}
func TestTurnOrderAndZero(t *testing.T) {
	var f fakeSession
	if _, e := Turn(&f, 0, session.CapturedUpdate{}); e != nil || len(f.calls) != 0 {
		t.Fatal(f.calls, e)
	}
	r, e := Turn(&f, 7, session.CapturedUpdate{})
	if e != nil || len(f.calls) != 3 || f.calls[0] != "View" || f.calls[1] != "Deliver" || f.calls[2] != "Advance" || f.u.SourceGeneration != 7 || r.Tick.Steps != 7 {
		t.Fatal(f.calls, r, e)
	}
}
func TestPaceSlowHysteresisLiteral(t *testing.T) {
	var p Pace
	for i := 0; i < 60; i++ {
		p.Step(time.Second/60, i == 0)
	}
	// Independent arithmetic: large0.2s hits cap385000 and clamps acc385000;
	// short1/60 spends cap385000, leaves just under192500. 29 saturated rows.
	for i := 0; i < 29; i++ {
		a := p.Step(time.Second/5, false)
		b := p.Step(time.Second/60, false)
		if !a.Saturated || b.Saturated || a.Slow || b.Slow || a.Cap != 385000 || b.Cap != 385000 {
			t.Fatal(i, a, b)
		}
	}
	// 30 long/30 short median=(0.2 + 0.016666666)/2; roundedF1251250.
	r := p.Step(time.Second, false)
	if !r.Saturated || !r.Slow || r.Cap != 2502500 || r.Acc != 2502500 {
		t.Fatal(r)
	}
	// First zero has cap1251250, leaves equalcap (saturated); second median
	// returns to short dt, cap192500 and clamps; third zero spends it, clean1.
	a := p.Step(0, false)
	b := p.Step(0, false)
	r = p.Step(0, false)
	if a.Cap != 1251250 || !a.Saturated || b.Cap != 192500 || !b.Saturated || r.Saturated || !r.Slow || r.Acc != 0 {
		t.Fatal(a, b, r)
	}
	for i := 2; i < 120; i++ {
		r = p.Step(0, false)
		if !r.Slow || r.Saturated {
			t.Fatal(i, r)
		}
	}
	r = p.Step(0, false)
	if r.Slow || r.Saturated {
		t.Fatal("not120 clean", r)
	}
}
