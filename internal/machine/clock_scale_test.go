package machine

import (
	"bytes"
	"testing"
)

func TestScaleSteps(t *testing.T) {
	cases := []struct {
		v    uint64
		p    int
		want uint64
	}{{0, 50, 0}, {1, 10, 1}, {165000, 50, 82500}, {165000, 33, 54450}, {3, 50, 2}, {165000, 100, 165000}}
	for _, c := range cases {
		if got := ScaleSteps(c.v, c.p); got != c.want {
			t.Fatalf("ScaleSteps(%d,%d)=%d want %d", c.v, c.p, got, c.want)
		}
	}
}

func TestSetClockPercent(t *testing.T) {
	m := New()
	before := clockFields(m)
	if err := m.SetClockPercent(100); err != nil {
		t.Fatal(err)
	}
	if !eqU(clockFields(m), before) {
		t.Fatal("p=100 改了欄位")
	}
	m = New()
	if err := m.SetClockPercent(50); err != nil {
		t.Fatal(err)
	}
	if m.IRQ0Base != 82500 || m.VGAFrameEvery != 82500 || m.KeyEvery != DefaultKeyIRQEvery/2 ||
		m.nextIRQ0 != 82500 || m.nextFrame != 82500 || m.nextKey != 0 || m.ClockPercent() != 50 {
		t.Fatalf("p=50 欄位 %+v", []uint64{m.IRQ0Base, m.VGAFrameEvery, m.KeyEvery, m.nextIRQ0, m.nextFrame, m.nextKey})
	}
	if want := stepsPerTick(82500, PITDefaultDivisor); m.IRQ0Every != want {
		t.Fatalf("IRQ0Every %d want %d", m.IRQ0Every, want)
	}
	m.Out8(0x43, 0x36)
	m.Out8(0x40, uint8(17000&0xFF))
	m.Out8(0x40, uint8(17000>>8))
	if want := stepsPerTick(82500, 17000); m.IRQ0Every != want {
		t.Fatalf("PIT 17000 後 %d want %d", m.IRQ0Every, want)
	}
	for _, p := range []int{9, 101} {
		if New().SetClockPercent(p) == nil {
			t.Fatalf("p=%d 應失敗", p)
		}
	}
	m = New()
	m.Steps = 1
	if m.SetClockPercent(50) == nil {
		t.Fatal("Steps!=0 應失敗")
	}
	m = New()
	m.CycleClock = true
	if m.SetClockPercent(50) == nil {
		t.Fatal("CycleClock 應失敗")
	}
}

func clockFields(m *Machine) []uint64 {
	return []uint64{m.IRQ0Base, m.IRQ0Every, m.VGAFrameEvery, m.KeyEvery, m.nextIRQ0, m.nextFrame, m.nextKey, uint64(m.ClockPercent())}
}

func eqU(a, b []uint64) bool {
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return len(a) == len(b)
}

func TestClockPercentSnapshotAndState(t *testing.T) {
	m := New()
	m.SetClockPercent(50)
	m.nextKey = 1234
	want := clockFields(m)
	snap := m.Snapshot()
	var buf bytes.Buffer
	if err := m.SaveState(&buf); err != nil {
		t.Fatal(err)
	}
	n := New()
	n.Restore(snap)
	if got := clockFields(n); !eqU(got, want) {
		t.Fatalf("快照往返 %v want %v", got, want)
	}
	n = New()
	if err := n.LoadState(bytes.NewReader(buf.Bytes())); err != nil {
		t.Fatal(err)
	}
	if got := clockFields(n); !eqU(got, want) {
		t.Fatalf("存檔往返 %v want %v", got, want)
	}
}

func TestLoadOldStateRestoresFullSpeed(t *testing.T) {
	// 舊格式：以 100% 存檔後清掉 242 的欄位。
	old := New()
	var buf bytes.Buffer
	if err := old.SaveState(&buf); err != nil {
		t.Fatal(err)
	}
	s := decodeStateForTest(t, buf.Bytes())
	s.VGAFrameEvery, s.NextFrame, s.KeyEvery, s.NextKey, s.ClockPercent = 0, 0, 0, 0, 0
	legacy := encodeStateForTest(t, s)

	full := clockFields(New())
	m := New()
	m.SetClockPercent(50)
	if err := m.LoadState(bytes.NewReader(legacy)); err != nil {
		t.Fatal(err)
	}
	got := clockFields(m)
	// nextFrame／nextKey 維持當下值（既有行為），其餘回 100%。
	full[5], full[6] = m.nextFrame, m.nextKey
	if !eqU(got, full) {
		t.Fatalf("縮放後載入舊存檔 %v want %v", got, full)
	}
	// 未縮放的機器載入舊存檔：自訂的 KeyEvery 保留（probe 的用法）。
	p := New()
	p.KeyEvery = 777
	if err := p.LoadState(bytes.NewReader(legacy)); err != nil {
		t.Fatal(err)
	}
	if p.KeyEvery != 777 || p.ClockPercent() != 100 {
		t.Fatalf("自訂 KeyEvery 被改 %d", p.KeyEvery)
	}
}
