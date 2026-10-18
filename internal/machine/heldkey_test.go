package machine

import (
	"bytes"
	"slices"
	"testing"
)

// docs/spec/449 §3.1 第 8 項：按住狀態在 Machine 層的保存、還原、讀檔與範圍。

func TestHeldKeysSnapshotRestore(t *testing.T) {
	const k = 0x39
	m := New()
	s := m.Snapshot()
	if !m.HeldKeyDown(k) {
		t.Fatal("HeldKeyDown 回 false")
	}
	m.Restore(s)
	if got := m.HeldKeys(); len(got) != 0 || m.HeldMask() != ([2]uint64{}) {
		t.Fatalf("還原到按下之前，HeldKeys = %v", got)
	}

	m.HeldKeyDown(k)
	s = m.Snapshot()
	m.HeldKeyUp(k)
	m.Restore(s)
	if got := m.HeldKeys(); !slices.Equal(got, []uint8{k}) {
		t.Fatalf("還原到放開之前，HeldKeys = %v，要 [%02X]", got, k)
	}
	// 還原後仍能照原紀錄放開。
	if !m.HeldKeyUp(k) {
		t.Fatal("還原後 HeldKeyUp 回 false")
	}
}

func TestHeldKeysLoadStateClearsAtCommit(t *testing.T) {
	const k = 0x39
	src := New()
	if err := src.LoadCOM([]byte{0xEB, 0xFE}); err != nil {
		t.Fatal(err)
	}
	src.Steps = 5
	var good bytes.Buffer
	if err := src.SaveState(&good); err != nil {
		t.Fatal(err)
	}
	// 解碼成功、驗證失敗的狀態檔：延伸記憶體大小不同。
	ext := New()
	if err := ext.SetExtendedMemoryKB(4096); err != nil {
		t.Fatal(err)
	}
	var mismatched bytes.Buffer
	if err := ext.SaveState(&mismatched); err != nil {
		t.Fatal(err)
	}

	for name, bad := range map[string][]byte{"Garbage": []byte("not a state"), "ExtMismatch": mismatched.Bytes()} {
		t.Run("Fail_"+name, func(t *testing.T) {
			m := New()
			m.HeldKeyDown(k)
			if err := m.LoadState(bytes.NewReader(bad)); err == nil {
				t.Fatal("壞資料讀檔成功")
			}
			if got := m.HeldKeys(); !slices.Equal(got, []uint8{k}) {
				t.Fatalf("讀檔失敗後 HeldKeys = %v，要 [%02X]", got, k)
			}
		})
	}
	t.Run("Success", func(t *testing.T) {
		m := New()
		m.SetDOSBoxCycles(CyclesXT)
		m.HeldKeyDown(k)
		if err := m.LoadState(bytes.NewReader(good.Bytes())); err != nil {
			t.Fatal(err)
		}
		if got := m.HeldKeys(); len(got) != 0 || m.HeldMask() != ([2]uint64{}) {
			t.Fatalf("讀檔後 HeldKeys = %v", got)
		}
		before := m.TimedKeysPending()
		m.ScheduleHeldRepeats(1 << 40)
		if m.TimedKeysPending() != before {
			t.Fatalf("讀檔後 ScheduleHeldRepeats 排了 %d 個事件", m.TimedKeysPending()-before)
		}
	})
}

func TestHeldKeyDownRange(t *testing.T) {
	m := New()
	if !m.HeldKeyDown(0x7F) {
		t.Fatal("HeldKeyDown(7Fh) 回 false")
	}
	if m.HeldMask() != [2]uint64{0, 1 << 63} {
		t.Fatalf("HeldMask = %x", m.HeldMask())
	}
	before := m.ScheduledKeys()
	for _, s := range []uint8{0, 0x80, 0xE0} {
		if m.HeldKeyDown(s) {
			t.Errorf("HeldKeyDown(%02Xh) 回 true", s)
		}
		if m.HeldKeyUp(s) {
			t.Errorf("HeldKeyUp(%02Xh) 回 true", s)
		}
	}
	if !slices.Equal(m.ScheduledKeys(), before) {
		t.Fatalf("範圍外的呼叫改了 timedKeys：%v", m.ScheduledKeys())
	}
}

func TestCancelTimedKeysFromIncludesStep(t *testing.T) {
	m := New()
	m.ScheduleKey(10, 0x39, false)
	m.ScheduleKey(20, 0x39, false)
	m.ScheduleKey(20, 0x39, true)
	m.ScheduleKey(30, 0x48, false)
	if n := m.CancelTimedKeysFrom(0x39, 20); n != 1 {
		t.Errorf("移除 %d 個，要 1 個", n)
	}
	ks := m.ScheduledKeys()
	if len(ks) != 3 || ks[0].Step != 10 || !ks[1].Break || ks[2].Scan != 0x48 {
		t.Errorf("剩下 %+v", ks)
	}
}
