package machine

import "testing"

// Buck repo spec 052 §5.1: every video mode set is visible, the observer is
// optional, and ModeChanges keeps growing.
func TestModeObserverOptionalAndSeesEverySet(t *testing.T) {
	m := New()
	m.SetVideoMode(0x13) // no observer: nothing to call
	before := len(m.ModeChanges)
	var got []ModeChange
	var modeAtCall []uint8
	m.ObserveModeChanges(func(c ModeChange) {
		got = append(got, c)
		modeAtCall = append(modeAtCall, m.VideoMode())
	})
	m.Steps = 7
	m.SetVideoMode(0x12) // planar: the planes are cleared outside Write8
	m.SetVideoMode(0x13) // linear: nothing is cleared
	if len(got) != 2 || got[0].Mode != 0x12 || got[1].Mode != 0x13 || got[0].Step != 7 || got[1].Step != 7 {
		t.Fatalf("got=%+v", got)
	}
	if modeAtCall[0] != 0x12 || modeAtCall[1] != 0x13 {
		t.Fatalf("callback 時 VideoMode=%v，應已是新模式", modeAtCall)
	}
	if len(m.ModeChanges) != before+2 {
		t.Fatalf("ModeChanges=%d，應保留追加（%d）", len(m.ModeChanges), before+2)
	}
	m.ObserveModeChanges(nil)
	m.SetVideoMode(0x03)
	if len(got) != 2 || len(m.ModeChanges) != before+3 {
		t.Fatalf("nil 未關閉 observer：%d 事件，ModeChanges=%d", len(got), len(m.ModeChanges))
	}
}

// The observer is not part of the machine state: a snapshot taken with one
// restores without it and the bytes do not depend on it.
func TestModeObserverIsNotSnapshotState(t *testing.T) {
	a, b := New(), New()
	b.ObserveModeChanges(func(ModeChange) {})
	for _, m := range []*Machine{a, b} {
		m.SetVideoMode(0x13)
		m.SetVideoMode(0x12)
	}
	if a.VideoMode() != b.VideoMode() || a.planarOn != b.planarOn || len(a.ModeChanges) != len(b.ModeChanges) {
		t.Fatal("observer 不應影響機器狀態")
	}
}
