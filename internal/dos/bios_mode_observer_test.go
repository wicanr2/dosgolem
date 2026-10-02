package dos

import (
	"testing"

	"github.com/wicanr2/dosgolem/internal/machine"
)

// Buck repo spec 052 §5.1: int 10h AH=00 reaches the mode observer exactly
// once, with the mode in AL (bit 7, the "do not clear" flag, is dropped).
func TestInt10SetModeNotifiesModeObserver(t *testing.T) {
	m, d := newTest(t)
	var modes []uint8
	m.ObserveModeChanges(func(c machine.ModeChange) { modes = append(modes, c.Mode) })
	call(m, d, 0x10, 0x0013)
	call(m, d, 0x10, 0x0093) // AL bit 7 set: same mode 13h
	call(m, d, 0x10, 0x0E41) // teletype output is not a mode set
	if len(modes) != 2 || modes[0] != 0x13 || modes[1] != 0x13 {
		t.Fatalf("modes=%v", modes)
	}
}
