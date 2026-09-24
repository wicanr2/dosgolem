package oracle

import (
	"errors"
	"strings"
	"testing"

	"github.com/wicanr2/dosgolem/internal/dos"
	"github.com/wicanr2/dosgolem/internal/machine"
)

// The input is a synthetic machine, not an original game. A budget that
// cannot be added to the current step count must fail before any instruction
// attempt; it must not masquerade as an exhausted budget.
func TestRunUntilRejectsDeadlineOverflowBeforeStepping(t *testing.T) {
	m := machine.New()
	d := dos.New(m, "")
	defer d.Close()
	o := &Oracle{m: m, d: d}
	m.Steps = 1
	conditionCalls := 0
	condition := NewCond("never", func(*Oracle) bool {
		conditionCalls++
		return false
	})
	err := o.RunUntil(condition, Budget(^uint64(0)))
	var exhausted *BudgetError
	if err == nil || errors.As(err, &exhausted) || !strings.Contains(err.Error(), "預算溢位") {
		t.Fatalf("overflow error = %v, want non-BudgetError overflow", err)
	}
	if m.Steps != 1 || conditionCalls != 0 {
		t.Fatalf("overflow changed machine/condition: steps=%d calls=%d", m.Steps, conditionCalls)
	}
}
