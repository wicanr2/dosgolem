package buckrogers

import (
	"crypto/sha256"
	"testing"
)

func TestTextRecorderGuardAndMetadata(t *testing.T) {
	r := &TextRecorder{}
	args := [6]uint16{0x1234, 0x5678, 0x5700, 0x570d, 0x5702, 0x5701}
	text := []byte("Pick Race")
	caller := Address{0x37F1, 0x158C}
	r.ObserveDispatchEntry(caller, 0x1841, 0x3900, args, text, 100)
	r.ObserveInstruction(Address{0x1111, 0x2222}, 0x1841, 0x3910, 101)
	if !r.Pending() || len(r.Events()) != 0 {
		t.Fatal("不相關或自然落入位址不得提交／丟棄 frame")
	}
	r.ObserveInstruction(caller, 0x1841, 0x3910, 200)
	events := r.Events()
	if len(events) != 1 || r.Pending() || r.Drops() != 0 {
		t.Fatalf("events=%d pending=%v drops=%d", len(events), r.Pending(), r.Drops())
	}
	e := events[0]
	if e.OriginalLength != 9 || e.OriginalSHA256 != sha256.Sum256(text) ||
		e.Background != 0 || e.Foreground != 13 || e.Row != 2 || e.Column != 1 ||
		e.EntryStep != 100 || e.PostCallStep != 200 {
		t.Fatalf("event metadata = %#v", e)
	}
}

func TestTextRecorderFailsClosed(t *testing.T) {
	caller := Address{0x37F1, 0x15BD}
	for _, tc := range []struct {
		name   string
		ss, sp uint16
	}{
		{"錯誤 SS", 2, 0x1010},
		{"錯誤 SP", 1, 0x100f},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := &TextRecorder{}
			r.ObserveDispatchEntry(caller, 1, 0x1000, [6]uint16{}, []byte("x"), 1)
			r.ObserveInstruction(caller, tc.ss, tc.sp, 2)
			if r.Pending() || len(r.Events()) != 0 || r.Drops() != 1 {
				t.Fatal("錯誤 guard 必須失敗即關閉")
			}
		})
	}

	r := &TextRecorder{}
	r.ObserveDispatchEntry(caller, 1, 1, [6]uint16{}, []byte("a"), 1)
	r.ObserveDispatchEntry(caller, 1, 1, [6]uint16{}, []byte("b"), 2)
	if r.Pending() || r.Drops() != 1 || len(r.Events()) != 0 {
		t.Fatal("巢狀 frame 必須失敗即關閉")
	}
}
