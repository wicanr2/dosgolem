//go:build draft_session_oracle_receipt_matrix

package session

// DRAFT-only characterization of the boundary a future sealed Owner would
// cross if it tried to reuse oracle.RunUntil for an observer-aware Advance.
//
// This file is deliberately self-contained: it creates one synthetic Owner,
// aliases its private machine/DOS into an Oracle only through test-only
// reflection, and never changes the production owner or machine runner.  It
// is evidence for a later typed TickReceipt design, not that design.

import (
	"errors"
	"reflect"
	"testing"
	"unsafe"

	"github.com/wicanr2/dosgolem/apps/buckrogers"
	"github.com/wicanr2/dosgolem/host"
	"github.com/wicanr2/dosgolem/internal/cpu"
	"github.com/wicanr2/dosgolem/oracle"
)

func draftReceiptLayout() host.MouseLayout {
	return host.MouseLayout{
		Epoch:        1,
		Scale:        host.OutputScale2,
		ChromeHeight: 36,
		Canvas:       host.Canvas{Width: 320, Height: 200},
		FrameWidth:   640,
		FrameHeight:  436,
		PanelOpen:    false,
	}
}

func draftReceiptOwner(t *testing.T, code []byte) *Owner {
	t.Helper()
	o, err := New(Config{InitialScale: host.OutputScale2, InitialLayout: draftReceiptLayout()})
	if err != nil {
		t.Fatal(err)
	}
	if err := o.machine.LoadCOM(code); err != nil {
		_ = o.Close()
		t.Fatal(err)
	}
	if err := o.startLoadedMachine(); err != nil {
		_ = o.Close()
		t.Fatal(err)
	}
	return o
}

// draftReceiptOracle is intentionally not a production constructor.  Oracle
// owns no exported constructor for an already-owned machine, which is the
// ownership boundary the eventual observer-aware API must preserve.
func draftReceiptOracle(t *testing.T, owner *Owner) *oracle.Oracle {
	t.Helper()
	if owner == nil || owner.machine == nil || owner.dos == nil {
		t.Fatal("DRAFT setup requires a live sealed Owner")
	}
	o := new(oracle.Oracle)
	fields := reflect.ValueOf(o).Elem()
	set := func(name string, value any) {
		t.Helper()
		field := fields.FieldByName(name)
		if !field.IsValid() || !field.CanAddr() {
			t.Fatalf("Oracle private field %q is unavailable; DRAFT bridge is stale", name)
		}
		reflect.NewAt(field.Type(), unsafe.Pointer(field.UnsafeAddr())).Elem().Set(reflect.ValueOf(value))
	}
	set("m", owner.machine)
	set("d", owner.dos)
	set("onCall", map[uint32][]func(*oracle.Oracle){})
	return o
}

func draftReceiptIsBudgetError(err error) bool {
	var budgetErr *oracle.BudgetError
	return errors.As(err, &budgetErr)
}

// TestDraftOwnerOracleReceiptMatrix records facts that a future sealed
// observer-aware Advance must map explicitly instead of treating Oracle's
// nil/error result as a machine Stop or an Owner TickReceipt reason.
func TestDraftOwnerOracleReceiptMatrix(t *testing.T) {
	t.Run("Steps n has an Oracle-only n versus n plus one boundary", func(t *testing.T) {
		for _, tc := range []struct {
			name       string
			budget     uint64
			wantBudget bool
		}{
			{"budget n returns BudgetError after n attempts", 2, true},
			{"budget n plus one reaches the next-head predicate", 3, false},
		} {
			t.Run(tc.name, func(t *testing.T) {
				owner := draftReceiptOwner(t, []byte{0x90, 0x90})
				defer owner.Close()
				o := draftReceiptOracle(t, owner)
				err := o.RunUntil(oracle.Steps(2), oracle.Budget(tc.budget))
				if draftReceiptIsBudgetError(err) != tc.wantBudget || (!tc.wantBudget && err != nil) {
					t.Fatalf("budget %d: err=%v, want BudgetError=%t", tc.budget, err, tc.wantBudget)
				}
				if o.Steps() != 2 || owner.machine.Steps != 2 {
					t.Fatalf("budget %d: Oracle/Owner steps=%d/%d, want 2/2", tc.budget, o.Steps(), owner.machine.Steps)
				}
			})
		}
	})

	// Oracle tests exit/HALT/the video-ROM guard only at a loop head.  Thus an
	// event caused by the final permitted Step is hidden by BudgetError; with
	// one additional budget slot it becomes the next loop-head error.
	for _, tc := range []struct {
		name       string
		code       []byte
		finalSteps uint64
		midBudget  uint64
		isExit     bool
	}{
		{"DOS exit", []byte{0xB8, 0x03, 0x4C, 0xCD, 0x21}, 2, 3, true},
		{"HLT", []byte{0xF4}, 1, 2, false},
		{"A0000 guard after far jump", []byte{0xEA, 0x00, 0x00, 0x00, 0xA0}, 1, 2, false},
	} {
		t.Run(tc.name+" at final step is reported as budget", func(t *testing.T) {
			owner := draftReceiptOwner(t, tc.code)
			defer owner.Close()
			o := draftReceiptOracle(t, owner)
			hooks := 0
			// The guard case specifically proves guard-before-OnCall at its next
			// head; the other cases retain the same observable callback count.
			o.OnCall(oracle.Far(0xA000, 0), func(*oracle.Oracle) { hooks++ })
			err := o.RunUntil(oracle.Steps(10), oracle.Budget(tc.finalSteps))
			if !draftReceiptIsBudgetError(err) || o.Steps() != tc.finalSteps || owner.machine.Steps != tc.finalSteps || hooks != 0 {
				t.Fatalf("final-step result: err=%v steps=%d/%d hooks=%d", err, o.Steps(), owner.machine.Steps, hooks)
			}
			if tc.isExit && !owner.dos.Exited {
				t.Fatal("final DOS-exit instruction did not set DOS.Exited")
			}
		})

		t.Run(tc.name+" before a remaining head is not a budget receipt", func(t *testing.T) {
			owner := draftReceiptOwner(t, tc.code)
			defer owner.Close()
			o := draftReceiptOracle(t, owner)
			hooks := 0
			o.OnCall(oracle.Far(0xA000, 0), func(*oracle.Oracle) { hooks++ })
			err := o.RunUntil(oracle.Steps(10), oracle.Budget(tc.midBudget))
			if err == nil || draftReceiptIsBudgetError(err) || o.Steps() != tc.finalSteps || owner.machine.Steps != tc.finalSteps || hooks != 0 {
				t.Fatalf("mid-run result: err=%v steps=%d/%d hooks=%d", err, o.Steps(), owner.machine.Steps, hooks)
			}
			if tc.isExit {
				var exitErr *oracle.ExitError
				if !errors.As(err, &exitErr) || exitErr.Code != 3 {
					t.Fatalf("DOS exit error=%v, want ExitError code 3", err)
				}
			}
		})
	}

	t.Run("Watcher OnCall installs and receives a dynamic far return hook", func(t *testing.T) {
		owner := draftReceiptOwner(t, []byte{0x90})
		defer owner.Close()
		o := draftReceiptOracle(t, owner)
		const (
			dispatcherSeg = 0x0763
			dispatcherOff = 0x0424
			returnSeg     = 0x2000
			returnOff     = 0x0100
			textSeg       = 0x2000
			textOff       = 0x0300
		)
		owner.machine.CPU.Seg[cpu.CS], owner.machine.CPU.IP = dispatcherSeg, dispatcherOff
		owner.machine.CPU.R[cpu.SP] = 0xFF00
		stack := cpu.Addr(owner.machine.CPU.Seg[cpu.SS], owner.machine.CPU.R[cpu.SP])
		owner.machine.Write16(stack, returnOff)
		owner.machine.Write16(stack+2, returnSeg)
		owner.machine.Write16(stack+4, textOff)
		owner.machine.Write16(stack+6, textSeg)
		for offset := uint32(8); offset <= 14; offset += 2 {
			owner.machine.Write16(stack+offset, 0)
		}
		owner.machine.WriteBytes(cpu.Addr(dispatcherSeg, dispatcherOff), []byte{0xCA, 0x0C, 0x00})
		owner.machine.Write8(cpu.Addr(returnSeg, returnOff), 0x90)
		owner.machine.WriteBytes(cpu.Addr(textSeg, textOff), []byte{1, 'x'})

		watcher := buckrogers.NewWatcher(nil)
		watcher.Install(o)
		if err := o.RunUntil(oracle.Steps(2), oracle.Budget(3)); err != nil {
			t.Fatal(err)
		}
		events := watcher.Observations()
		if len(events) != 1 || events[0].Kind != "post-call" || events[0].Step != 1 ||
			events[0].Caller.Segment != returnSeg || events[0].Caller.Offset != returnOff || owner.machine.Steps != 2 {
			t.Fatalf("dynamic-return receipt: events=%+v owner steps=%d", events, owner.machine.Steps)
		}
	})

	t.Run("stub at the same address reenters but consumes one Step per callback", func(t *testing.T) {
		owner := draftReceiptOwner(t, []byte{0x90})
		defer owner.Close()
		o := draftReceiptOracle(t, owner)
		at := o.IP()
		// Make the synthetic far return point back to the stub itself.  The
		// bounded Steps condition proves this cannot spin forever merely because
		// fireStub skips Machine.Step: fireStub increments Machine.Steps itself.
		owner.machine.CPU.R[cpu.SP] = 0xFF00
		stack := cpu.Addr(owner.machine.CPU.Seg[cpu.SS], owner.machine.CPU.R[cpu.SP])
		// fireStub consumes its far return frame.  Seed three adjacent frames so
		// the same address reenters three times without relying on an unbounded
		// malformed stack loop.
		for frame := uint32(0); frame < 3; frame++ {
			owner.machine.Write16(stack+frame*4, at.Off)
			owner.machine.Write16(stack+frame*4+2, at.Seg)
		}
		calls := 0
		o.Stub(at, func(*oracle.Oracle) uint32 {
			calls++
			if calls > 3 {
				t.Fatal("bounded DRAFT stub probe exceeded its expected callbacks")
			}
			return 0
		})
		if err := o.RunUntil(oracle.Steps(3), oracle.Budget(4)); err != nil {
			t.Fatal(err)
		}
		if calls != 3 || o.Steps() != 3 || owner.machine.Steps != 3 || o.IP() != at {
			t.Fatalf("same-address stub: calls=%d steps=%d/%d IP=%v, want 3/3/3 and %v", calls, o.Steps(), owner.machine.Steps, o.IP(), at)
		}
	})

	t.Run("raw Step error wins over the final-budget shape but Oracle exposes no raw Stop", func(t *testing.T) {
		owner := draftReceiptOwner(t, []byte{0x90, 0x63}) // 63h is unsupported by the 8086 model.
		defer owner.Close()
		o := draftReceiptOracle(t, owner)
		err := o.RunUntil(oracle.Steps(10), oracle.Budget(2))
		if err == nil || draftReceiptIsBudgetError(err) || o.Steps() != 2 || owner.machine.Steps != 2 {
			t.Fatalf("raw-error receipt: err=%v steps=%d/%d", err, o.Steps(), owner.machine.Steps)
		}
		// Oracle.RunUntil returns only error/nil.  It does not return the raw
		// machine.Stop value that a TickReceipt must preserve, so this test must
		// not be mistaken for authority to infer StopBudget from the budget.
	})

	t.Run("deadline addition overflow returns BudgetError without one attempt", func(t *testing.T) {
		owner := draftReceiptOwner(t, []byte{0x90})
		defer owner.Close()
		o := draftReceiptOracle(t, owner)
		owner.machine.Steps = 1
		err := o.RunUntil(oracle.NewCond("DRAFT always false", func(*oracle.Oracle) bool { return false }), oracle.Budget(^uint64(0)))
		if !draftReceiptIsBudgetError(err) || o.Steps() != 1 || owner.machine.Steps != 1 {
			t.Fatalf("overflow receipt: err=%v steps=%d/%d, want BudgetError and no attempt", err, o.Steps(), owner.machine.Steps)
		}
		// A sealed Owner must reject this budget (or specify a safe maximum)
		// before delegating.  Oracle's error alone is indistinguishable from a
		// genuine exhausted budget to a caller that omits the step delta.
	})
}
