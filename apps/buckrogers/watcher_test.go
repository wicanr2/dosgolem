package buckrogers

import "testing"

func primedWatcher(t *testing.T, catalog *Catalog) (*Watcher, dispatchFrame) {
	t.Helper()
	w := NewWatcher(catalog)
	g, ok := w.collector.BeginEntry(manualBegin, manualBeginText)
	if !ok {
		t.Fatal("無法建立測試 generation")
	}
	f := dispatchFrame{
		caller: validManualEvents[0].caller, returnTo: Address{0x2A33, 0x0220},
		ss: 0x1841, entrySP: 0x3900, text: validManualEvents[0].text, generation: g,
	}
	w.pending = &f
	return w, f
}

func TestWatcherGuardedReturn(t *testing.T) {
	w, f := primedWatcher(t, nil)
	w.ObserveInstruction(f.returnTo, f.ss, f.entrySP+dispatcherStackDelta, 10)
	if w.pending != nil || w.collector.pending.stage != 1 {
		t.Fatalf("有效返回未提交：pending=%#v stage=%d", w.pending, w.collector.pending.stage)
	}
}

func TestWatcherNaturalFallthroughAndStaleHook(t *testing.T) {
	w, f := primedWatcher(t, nil)
	w.ObserveInstruction(Address{0x37F1, 0x15BD}, f.ss, f.entrySP, 10)
	if w.pending == nil || w.collector.pending.stage != 0 {
		t.Fatal("自然 fall-through 不得提交或丟棄另一個 frame")
	}
	w.ObserveInstruction(Address{0x1111, 0x2222}, f.ss, f.entrySP+dispatcherStackDelta, 11)
	if w.pending == nil {
		t.Fatal("stale return hook 不得丟棄目前 frame")
	}
}

func TestWatcherWrongSSAndSPFailClosed(t *testing.T) {
	for _, tc := range []struct {
		name   string
		ss, sp uint16
	}{
		{"錯誤 SS", 0x1842, 0x3910},
		{"錯誤 SP", 0x1841, 0x390F},
	} {
		t.Run(tc.name, func(t *testing.T) {
			w, f := primedWatcher(t, nil)
			w.ObserveInstruction(f.returnTo, tc.ss, tc.sp, 10)
			if w.pending != nil || w.collector.pending.stage != 0 || len(w.requests) != 0 {
				t.Fatal("guard 失敗必須丟棄 frame 且不得提交")
			}
		})
	}
}

func TestWatcherWrongCallerPoisons(t *testing.T) {
	w, f := primedWatcher(t, nil)
	f.caller = Address{0xFFFF, 0xFFFF}
	w.pending = &f
	w.ObserveInstruction(f.returnTo, f.ss, f.entrySP+dispatcherStackDelta, 10)
	if !w.collector.pending.poisoned || len(w.requests) != 0 {
		t.Fatal("未知 caller 應 poison 且不得產生請求")
	}
}

func TestWatcherCatalogMiss(t *testing.T) {
	w := NewWatcher(loadFixture(t, eventFixture, ordinalFixture(), textFixture))
	g, _ := w.collector.BeginEntry(manualBegin, manualBeginText)
	for i, event := range validManualEvents {
		f := dispatchFrame{caller: event.caller, returnTo: Address{0x2A33, uint16(0x1000 + i)}, ss: 1, entrySP: 2, text: event.text, generation: g}
		w.pending = &f
		w.ObserveInstruction(f.returnTo, 1, 0x12, uint64(i))
	}
	if len(w.requests) != 0 || w.events[len(w.events)-1].Kind != "catalog-miss" {
		t.Fatal("未收錄題目必須 catalog miss 且不產生請求")
	}
}

func TestWatcherNestedFrameFailsClosed(t *testing.T) {
	w, _ := primedWatcher(t, nil)
	w.dropNested(10, Address{0x1234, 0x5678})
	if w.pending != nil || len(w.requests) != 0 || w.events[0].Kind != "nested-drop" {
		t.Fatal("巢狀 frame 必須失敗即關閉")
	}
}
