//go:build draft_session_oracle_loop

package session

// DRAFT receipt only: this test aliases an Owner's private machine/DOS into
// Oracle through test-only reflection.  It neither exports an Owner resource
// nor changes the production lifecycle.  Its sole purpose is to measure the
// existing Oracle.RunUntil ordering before a sealed observer-aware Advance
// contract can be proposed.

import (
	"errors"
	"reflect"
	"testing"
	"unsafe"

	"github.com/wicanr2/dosgolem/apps/buckrogers"
	"github.com/wicanr2/dosgolem/internal/cpu"
	"github.com/wicanr2/dosgolem/oracle"
)

// draftOwnerOracle aliases exactly the Owner-owned pair.  Oracle has no
// constructor for an already-owned machine by design; using unsafe here keeps
// that production boundary intact while making the DRAFT experiment falsifiable.
func draftOwnerOracle(t *testing.T, owner *Owner) *oracle.Oracle {
	t.Helper()
	if owner == nil || owner.machine == nil || owner.dos == nil {
		t.Fatal("DRAFT setup needs one live Owner-owned machine and DOS")
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
	onCall := fields.FieldByName("onCall")
	if !onCall.IsValid() || !onCall.CanAddr() {
		t.Fatal("Oracle onCall field is unavailable; DRAFT bridge is stale")
	}
	reflect.NewAt(onCall.Type(), unsafe.Pointer(onCall.UnsafeAddr())).Elem().Set(reflect.MakeMap(onCall.Type()))
	return o
}

// TestDraftOwnerOracleLoopAliasesPrivateMachineAndPreservesOracleOrdering is
// not a READY production contract.  It documents the current Oracle.RunUntil
// ordering when a test-only alias points at an Owner's private machine/DOS.
// A future sealed runner and its TickReceipt mapping need separate proof.
func TestDraftOwnerOracleLoopAliasesPrivateMachineAndPreservesOracleOrdering(t *testing.T) {
	t.Run("callback sees the Owner machine and advances it", func(t *testing.T) {
		owner := startSyntheticOwner(t, []byte{0x90, 0x90})
		defer owner.Close()
		o := draftOwnerOracle(t, owner)
		start := o.IP()
		var calls []uint64
		o.OnCall(start, func(callback *oracle.Oracle) {
			if callback != o || callback.IP() != start {
				t.Fatalf("callback Oracle/IP = %p/%v, want %p/%v", callback, callback.IP(), o, start)
			}
			calls = append(calls, callback.Steps())
		})
		// Oracle evaluates Steps(2) on the next loop head, so its own current
		// runner needs a third budget slot to stop without a third Step.
		if err := o.RunUntil(oracle.Steps(2), oracle.Budget(3)); err != nil {
			t.Fatal(err)
		}
		if got := o.Steps(); got != 2 || owner.machine.Steps != got || len(calls) != 1 || calls[0] != 0 {
			t.Fatalf("alias receipt: oracle/owner steps=%d/%d calls=%v, want 2/2 and [0]", got, owner.machine.Steps, calls)
		}
	})

	t.Run("Steps n needs budget n plus one for Oracle success", func(t *testing.T) {
		for _, tc := range []struct {
			name      string
			budget    uint64
			wantError bool
		}{
			{"budget equals n returns BudgetError after n steps", 2, true},
			{"budget equals n plus one returns nil after the same n steps", 3, false},
		} {
			t.Run(tc.name, func(t *testing.T) {
				owner := startSyntheticOwner(t, []byte{0x90, 0x90})
				defer owner.Close()
				o := draftOwnerOracle(t, owner)
				err := o.RunUntil(oracle.Steps(2), oracle.Budget(tc.budget))
				var budgetErr *oracle.BudgetError
				if tc.wantError {
					if !errors.As(err, &budgetErr) {
						t.Fatalf("budget=%d err=%v, want BudgetError", tc.budget, err)
					}
				} else if err != nil {
					t.Fatalf("budget=%d err=%v, want nil", tc.budget, err)
				}
				if o.Steps() != 2 || owner.machine.Steps != 2 {
					t.Fatalf("budget=%d oracle/owner steps=%d/%d, want 2/2", tc.budget, o.Steps(), owner.machine.Steps)
				}
			})
		}
	})

	t.Run("initial condition wins before OnCall", func(t *testing.T) {
		owner := startSyntheticOwner(t, []byte{0x90})
		defer owner.Close()
		o := draftOwnerOracle(t, owner)
		calls := 0
		o.OnCall(o.IP(), func(*oracle.Oracle) { calls++ })
		if err := o.RunUntil(oracle.NewCond("DRAFT initial true", func(*oracle.Oracle) bool { return true }), oracle.Budget(1)); err != nil || calls != 0 || o.Steps() != 0 {
			t.Fatalf("initial-condition receipt: err=%v calls=%d steps=%d", err, calls, o.Steps())
		}
	})

	t.Run("OnCall runs before a far stub", func(t *testing.T) {
		owner := startSyntheticOwner(t, []byte{0x90, 0x90, 0x90, 0x90, 0x90, 0x90})
		defer owner.Close()
		o := draftOwnerOracle(t, owner)
		entry := o.IP()
		target := oracle.Far(entry.Seg, entry.Off+0x10)
		owner.machine.WriteBytes(entry.Linear(), []byte{0x9A, uint8(target.Off), uint8(target.Off >> 8), uint8(target.Seg), uint8(target.Seg >> 8), 0x90})
		var order []string
		o.OnCall(target, func(*oracle.Oracle) { order = append(order, "hook") })
		o.Stub(target, func(*oracle.Oracle) uint32 { order = append(order, "stub"); return 0x12345678 })
		if err := o.RunUntil(oracle.Steps(2), oracle.Budget(3)); err != nil {
			t.Fatal(err)
		}
		if len(order) != 2 || order[0] != "hook" || order[1] != "stub" || o.AX() != 0x5678 || o.DX() != 0x1234 || owner.machine.Steps != 2 {
			t.Fatalf("hook/stub receipt: order=%v AX:DX=%04X:%04X steps=%d", order, o.AX(), o.DX(), owner.machine.Steps)
		}
	})

	t.Run("Buck Watcher installs and receives its dynamic return hook", func(t *testing.T) {
		owner := startSyntheticOwner(t, []byte{0x90})
		defer owner.Close()
		o := draftOwnerOracle(t, owner)
		const (
			dispatcherSeg = 0x0763
			dispatcherOff = 0x0424
			returnSeg     = 0x2000
			returnOff     = 0x0100
			textSeg       = 0x2000
			textOff       = 0x0300
		)
		// Synthetic RETF 0Ch supplies exactly the stack delta in
		// 008-buck-rogers-manual-runtime-watcher.  It is a hook-order probe,
		// not original-program or player-path evidence.
		owner.machine.CPU.Seg[cpu.CS], owner.machine.CPU.IP = dispatcherSeg, dispatcherOff
		owner.machine.CPU.R[cpu.SP] = 0xFF00
		stack := cpu.Addr(owner.machine.CPU.Seg[cpu.SS], owner.machine.CPU.R[cpu.SP])
		owner.machine.Write16(stack, returnOff)
		owner.machine.Write16(stack+2, returnSeg)
		owner.machine.Write16(stack+4, textOff)
		owner.machine.Write16(stack+6, textSeg)
		for i := uint32(8); i <= 14; i += 2 {
			owner.machine.Write16(stack+i, 0)
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
			t.Fatalf("Watcher dynamic-return receipt: events=%+v owner steps=%d", events, owner.machine.Steps)
		}
	})

	t.Run("A0000 guard wins before OnCall or Step", func(t *testing.T) {
		owner := startSyntheticOwner(t, []byte{0x90})
		defer owner.Close()
		owner.machine.CPU.Seg[cpu.CS], owner.machine.CPU.IP = 0xA000, 0
		o := draftOwnerOracle(t, owner)
		calls := 0
		o.OnCall(o.IP(), func(*oracle.Oracle) { calls++ })
		if err := o.RunUntil(oracle.Steps(1), oracle.Budget(1)); err == nil || calls != 0 || o.Steps() != 0 || owner.machine.Steps != 0 {
			t.Fatalf("A0000 guard receipt: err=%v calls=%d oracle/owner steps=%d/%d", err, calls, o.Steps(), owner.machine.Steps)
		}
	})

	t.Run("zero budget returns Oracle budget error before condition or callback", func(t *testing.T) {
		owner := startSyntheticOwner(t, []byte{0x90})
		defer owner.Close()
		o := draftOwnerOracle(t, owner)
		calls := 0
		o.OnCall(o.IP(), func(*oracle.Oracle) { calls++ })
		err := o.RunUntil(oracle.Steps(0), oracle.Budget(0))
		var budgetErr *oracle.BudgetError
		if !errors.As(err, &budgetErr) || calls != 0 || o.Steps() != 0 || owner.machine.Steps != 0 {
			t.Fatalf("zero-budget receipt: err=%v calls=%d oracle/owner steps=%d/%d", err, calls, o.Steps(), owner.machine.Steps)
		}
	})

	for _, terminal := range []struct {
		name string
		set  func(*Owner)
		want any
	}{
		{"DOS exit", func(owner *Owner) { owner.dos.Exited, owner.dos.ExitCode = true, 7 }, &oracle.ExitError{}},
		{"HLT", func(owner *Owner) { owner.machine.CPU.Halted = true }, nil},
	} {
		t.Run(terminal.name+" suppresses callback before Step", func(t *testing.T) {
			owner := startSyntheticOwner(t, []byte{0x90})
			defer owner.Close()
			terminal.set(owner)
			o := draftOwnerOracle(t, owner)
			calls := 0
			o.OnCall(o.IP(), func(*oracle.Oracle) { calls++ })
			err := o.RunUntil(oracle.Steps(1), oracle.Budget(1))
			if err == nil || calls != 0 || o.Steps() != 0 || owner.machine.Steps != 0 {
				t.Fatalf("terminal receipt: err=%v calls=%d oracle/owner steps=%d/%d", err, calls, o.Steps(), owner.machine.Steps)
			}
			if terminal.want != nil {
				var exitErr *oracle.ExitError
				if !errors.As(err, &exitErr) || exitErr.Code != 7 {
					t.Fatalf("DOS exit error = %v, want ExitError code 7", err)
				}
			}
		})
	}
}
