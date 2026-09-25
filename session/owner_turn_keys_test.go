package session

import (
	"testing"

	"github.com/wicanr2/dosgolem/host"
	"github.com/wicanr2/dosgolem/internal/dos"
)

// pollOnceCOM checks the BIOS keyboard (AH=01), spins until a key is
// present, reads exactly one (AH=00), then spins forever.
var pollOnceCOM = []byte{0xB4, 0x01, 0xCD, 0x16, 0x74, 0xFA, 0xB4, 0x00, 0xCD, 0x16, 0xEB, 0xFE}

func TestAdvanceReportsKeyboardConsumption(t *testing.T) {
	key, ok := dos.KeyForRune(' ')
	if !ok {
		t.Fatal("space has no BIOS key")
	}
	o := startSyntheticOwnerWithLayout(t, pollOnceCOM, sessionLayout(1, host.OutputScale2, false))
	deliverOwner(t, o, nil, []dos.Key{key})
	receipt, err := o.Advance(100000)
	if err != nil {
		t.Fatal(err)
	}
	if receipt.Reason != StopReasonBudgetExhausted {
		t.Fatalf("reason = %v", receipt.Reason)
	}
	if receipt.KeysPendingBefore != 1 || receipt.KeysPendingAfter != 0 {
		t.Fatalf("keys before/after = %d/%d, want 1/0",
			receipt.KeysPendingBefore, receipt.KeysPendingAfter)
	}
}

func TestAdvanceReportsNoKeysWhenNoneDelivered(t *testing.T) {
	o := startSyntheticOwnerWithLayout(t, pollOnceCOM, sessionLayout(1, host.OutputScale2, false))
	deliverOwner(t, o, nil, nil)
	receipt, err := o.Advance(1000)
	if err != nil {
		t.Fatal(err)
	}
	if receipt.KeysPendingBefore != 0 || receipt.KeysPendingAfter != 0 {
		t.Fatalf("keys before/after = %d/%d, want 0/0",
			receipt.KeysPendingBefore, receipt.KeysPendingAfter)
	}
}
