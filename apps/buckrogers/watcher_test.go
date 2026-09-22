package buckrogers

import (
	"reflect"
	"testing"
)

func observeManualDispatch(w *Watcher, caller Address, text string, step uint64) {
	w.ObserveDispatchEntry(caller, 1, 2, text, step)
	w.ObserveInstruction(caller, 1, 2+dispatcherStackDelta, step+1)
}

func deimosManualEvents() []struct {
	caller Address
	text   string
} {
	return []struct {
		caller Address
		text   string
	}{
		{Address{0x2A33, 0x021B}, "34"},
		{Address{0x2A33, 0x0231}, "following the heading"},
		{Address{0x2A33, 0x027A}, "Deimos Prison"},
		{Address{0x2A33, 0x02A5}, "what is the"},
		{Address{0x2A33, 0x02E2}, "tenth"},
		{Address{0x2A33, 0x0309}, "word?"},
	}
}

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

func TestManualPresentationEventShape(t *testing.T) {
	typ := reflect.TypeOf(ManualPresentationEvent{})
	if typ.NumField() != 4 {
		t.Fatalf("ManualPresentationEvent 欄位數=%d，要 4", typ.NumField())
	}
	for i, want := range []string{"Step", "Kind", "Generation", "Request"} {
		if typ.Field(i).Name != want {
			t.Errorf("欄位 %d=%s，要 %s", i, typ.Field(i).Name, want)
		}
	}
	if typ.Field(3).Type != reflect.TypeOf(DisplayRequest{}) {
		t.Fatalf("Request 型別=%v，要 DisplayRequest", typ.Field(3).Type)
	}
}

func TestWatcherRetainsOnlyExactBeginStyle(t *testing.T) {
	w := NewWatcher(nil)
	style := ManualTextStyle{Background: 0, Foreground: 10, Row: 2, Column: 3}
	w.ObserveDispatchEntryWithStyle(manualBegin, 1, 2, manualBeginText, style, 9)
	got, ok := w.ManualStyle()
	if !ok || got != style {
		t.Fatalf("style=%+v ok=%v", got, ok)
	}
	got.Foreground = 15
	again, ok := w.ManualStyle()
	if !ok || again.Foreground != 10 {
		t.Fatalf("style 必須是 value copy：%+v", again)
	}
	w2 := NewWatcher(nil)
	w2.ObserveDispatchEntryWithStyle(Address{1, 2}, 1, 2, "other", style, 1)
	if _, ok := w2.ManualStyle(); ok {
		t.Fatal("非精確 begin 不得產生樣式")
	}
	w3 := NewWatcher(nil)
	w3.ObserveDispatchEntry(manualBegin, 1, 2, manualBeginText, 1)
	if _, ok := w3.ManualStyle(); ok {
		t.Fatal("未提供樣式的相容入口不得偽稱已觀測色彩")
	}
}

func TestWatcherPresentationLifecyclePendingClearAndVisibleClear(t *testing.T) {
	w := NewWatcher(loadFixture(t, eventFixture, ordinalFixture(), textFixture))
	observeManualDispatch(w, manualBegin, manualBeginText, 10)
	for i, event := range deimosManualEvents()[:2] {
		observeManualDispatch(w, event.caller, event.text, uint64(20+i*10))
	}
	w.ObserveClear(45)
	for i, event := range deimosManualEvents()[2:] {
		observeManualDispatch(w, event.caller, event.text, uint64(50+i*10))
	}

	got := w.PresentationEvents()
	if len(got) != 3 {
		t.Fatalf("lifecycle events=%#v", got)
	}
	if got[0].Kind != ManualPresentationBegin || got[0].Generation != 1 || got[0].Step != 10 {
		t.Fatalf("begin=%#v", got[0])
	}
	if got[1].Kind != ManualPresentationClear || got[1].Generation != 1 || got[1].Step != 45 || got[1].Request != (DisplayRequest{}) {
		t.Fatalf("pending clear=%#v", got[1])
	}
	if got[2].Kind != ManualPresentationRequest || got[2].Generation != 1 || got[2].Request.Generation != 1 ||
		got[2].Request.EventKey != "manual.page34.deimos_prison.word10" || got[2].Request.TextKey != "manual.log.49.deimos_prison" {
		t.Fatalf("request=%#v", got[2])
	}
	requestBefore := got[2].Request
	got[2].Request.TextKey = "mutated"
	if w.PresentationEvents()[2].Request != requestBefore {
		t.Fatal("presentation request value 不得被呼叫端回寫")
	}

	w.ObserveClear(120)
	got = w.PresentationEvents()
	if len(got) != 4 || got[3].Kind != ManualPresentationClear || got[3].Generation != 1 || got[3].Step != 120 {
		t.Fatalf("visible clear=%#v", got)
	}
}

func TestWatcherPresentationLifecycleFailsClosedAndReturnsCopies(t *testing.T) {
	w := NewWatcher(loadFixture(t, eventFixture, ordinalFixture(), textFixture))
	w.ObserveClear(1)
	if len(w.PresentationEvents()) != 0 {
		t.Fatal("沒有手冊 context 的 clear 不得產生 presentation event")
	}
	observeManualDispatch(w, manualBegin, manualBeginText, 10)
	for i, event := range validManualEvents {
		observeManualDispatch(w, event.caller, event.text, uint64(20+i*10))
	}
	got := w.PresentationEvents()
	if len(got) != 1 || got[0].Kind != ManualPresentationBegin {
		t.Fatalf("catalog miss lifecycle=%#v", got)
	}
	got[0].Generation = 999
	if w.PresentationEvents()[0].Generation != 1 {
		t.Fatal("presentation queue 回傳切片不得污染 watcher")
	}
	w = NewWatcher(loadFixture(t, eventFixture, ordinalFixture(), textFixture))
	observeManualDispatch(w, manualBegin, manualBeginText, 10)
	first := deimosManualEvents()[0]
	w.ObserveDispatchEntry(first.caller, 1, 2, first.text, 20)
	w.ObserveInstruction(first.caller, 1, 2+dispatcherStackDelta-1, 21)
	if got := w.PresentationEvents(); len(got) != 1 || got[0].Kind != ManualPresentationBegin {
		t.Fatalf("guard failure lifecycle=%#v", got)
	}

	w, f := primedWatcher(t, nil)
	w.dropNested(10, Address{0x1234, 0x5678})
	w.ObserveInstruction(f.returnTo, f.ss, f.entrySP+dispatcherStackDelta, 11)
	if len(w.PresentationEvents()) != 0 {
		t.Fatal("nested 或 stale frame 不得產生 presentation event")
	}
}
