package player

import (
	"github.com/wicanr2/dosgolem/session"
)

// Turn is the sole View→Deliver→Advance path for windows and headless replay.
// n=0 returns before View; the caller keeps its target state for a later turn.
type Session interface {
	View() (session.View, error)
	Deliver(session.CapturedUpdate) (session.InputReceipt, error)
	Advance(session.InstructionBudget) (session.TickReceipt, error)
}
type TurnReceipt struct {
	Update session.CapturedUpdate
	Input  session.InputReceipt
	Tick   session.TickReceipt
}

func Turn(owner Session, n uint64, u session.CapturedUpdate) (TurnReceipt, error) {
	if n == 0 {
		return TurnReceipt{}, nil
	}
	v, e := owner.View()
	if e != nil {
		return TurnReceipt{}, e
	}
	u.StartedPanel = v.Panel
	u.Layout = v.Layout
	u.SourceGeneration = v.SourceGeneration
	ir, e := owner.Deliver(u)
	if e != nil {
		return TurnReceipt{}, e
	}
	t, e := owner.Advance(session.InstructionBudget(n))
	return TurnReceipt{Input: ir, Tick: t, Update: u}, e
}
