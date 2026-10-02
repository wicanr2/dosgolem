package buckrogers

import (
	"reflect"
	"sort"
	"strings"
	"testing"
	"unsafe"

	"github.com/wicanr2/dosgolem/internal/machine"
)

// Buck repo spec 052: a video mode set resets every overlay family without
// failing, without counting and without touching an empty family.

func TestModeResetBodyIconKeepsGeneration(t *testing.T) {
	c := bodyTestCatalog()
	font := bodyTestFont(c)
	w := NewLiveBodyIconWatcher(c)
	o, err := NewRuntimeBodyIconOverlay(c, font, 2, BodyIconLive)
	if err != nil {
		t.Fatal(err)
	}
	var palette [256][3]uint8
	feed := func(step uint64) {
		for i, k := range bodyInitialKeys {
			for _, tr := range w.Observe(bodyTestEvent(c, k, step+uint64(i)*10)) {
				if err := o.Apply(tr, palette); err != nil {
					t.Fatalf("Apply：%v", err)
				}
			}
		}
	}
	// Empty: nothing changes.
	o.ClearAll()
	w.Reset()
	if o.Generation() != 0 || o.InvalidationCount() != 0 || len(o.ActiveKeys()) != 0 {
		t.Fatal("空狀態的重置不得改變任何值")
	}
	feed(10)
	if len(o.ActiveKeys()) == 0 || o.Generation() != 1 {
		t.Fatalf("keys=%v gen=%d", o.ActiveKeys(), o.Generation())
	}
	// A transition is half collected when the mode is set.
	w.Observe(bodyTestEvent(c, bodyInitialKeys[0], 100))
	o.ClearAll()
	w.Reset()
	if len(o.ActiveKeys()) != 0 || o.InvalidationCount() != 0 {
		t.Fatalf("重置後 keys=%v invalidations=%d", o.ActiveKeys(), o.InvalidationCount())
	}
	var indexed [320 * 200]byte
	rgba, missing, drew := o.Draw(indexed[:], palette)
	if len(missing) != 0 || drew || string(rgba) != string(ScaleIndexedRGBA(indexed[:], palette, 2)) {
		t.Fatal("重置後應等於原版放大畫面")
	}
	// The generation kept counting: the next proven screen is accepted.
	feed(200)
	if len(o.ActiveKeys()) == 0 || o.Generation() != 2 {
		t.Fatalf("重置後的下一個畫面：keys=%v gen=%d", o.ActiveKeys(), o.Generation())
	}
}

func TestModeResetActionBarForgetsTheScreen(t *testing.T) {
	c, rects := formalActionOverlay(t)
	style := HotkeyPreservingActionBarNormalStyle()
	for _, scale := range []int{2, 3} {
		o, err := NewRuntimeActionBarOverlay(c, rects, actionOverlayFont(), scale, &style)
		if err != nil {
			t.Fatal(err)
		}
		w := NewActionBarRequestWatcher(c)
		// Empty: nothing changes.
		o.ResetDisplay()
		w.ResetDisplay()
		drops := w.Drops()
		o.ObserveAnchorEvent("career.screen.remaining_points.heading")
		w.ObserveAnchorEvent("career.screen.remaining_points.heading")
		e, r := actionEventFor(c, "career", "action.add", "normal")
		if err := o.Apply(e, r, [256][3]uint8{}); err != nil || len(o.ActiveKeys()) == 0 {
			t.Fatalf("err=%v keys=%v", err, o.ActiveKeys())
		}
		o.ResetDisplay()
		w.ResetDisplay()
		if len(o.ActiveKeys()) != 0 || o.screen != "" || w.collector.screen != "" || w.pending != nil || w.Drops() != drops {
			t.Fatalf("%d×：screen=%q watcher=%q pending=%v drops=%d", scale, o.screen, w.collector.screen, w.pending, w.Drops())
		}
		// Without a screen anchor an event is refused, as in the default branch.
		if err := o.Apply(e, r, [256][3]uint8{}); err == nil {
			t.Fatal("重置後沒有錨點時 Apply 應被拒")
		}
	}
}

func modeResetManualLane(t *testing.T) (*liveLane, *Watcher, catalogEntry) {
	t.Helper()
	texts := manualE1LiveTexts()
	catalog := manualE1LiveCatalog(texts...)
	base, _ := manualE1SyntheticFonts(strings.Join(texts, ""))
	l := manualE1LiveLane(t, catalog, base)
	var entry catalogEntry
	for _, e := range catalog.byIdentity {
		entry = e
	}
	return l, l.manSync[1].watcher, entry
}

func TestModeResetManualVisibleThenRealClear(t *testing.T) {
	l, w, entry := modeResetManualLane(t)
	manualE1LiveEvents(w, 1, entry)
	w.collector.generation, w.collector.hasVisible = 1, true
	l.syncManual(ManualTextStyle{}, false, len(w.presentation))
	if len(l.manPres[0].ActiveKeys()) == 0 || len(l.manPres[1].ActiveKeys()) == 0 {
		t.Fatal("兩個倍率都應顯示")
	}
	w.ObserveDisplayReset(9)
	last := w.presentation[len(w.presentation)-1]
	if last.Kind != ManualPresentationClear || last.Generation != 1 || w.collector.active() {
		t.Fatalf("last=%+v active=%v", last, w.collector.active())
	}
	l.syncManual(ManualTextStyle{}, false, len(w.presentation))
	if len(l.manPres[0].ActiveKeys()) != 0 || len(l.manPres[1].ActiveKeys()) != 0 || l.resets["manual"] != 0 {
		t.Fatalf("事件後 keys 應為空、無 reset：%v", l.resets)
	}
	// The real clear that follows is not forwarded again (the collector is idle).
	n := len(w.presentation)
	w.ObserveClear(10)
	if len(w.presentation) != n {
		t.Fatal("真正的清除不得再送一次 Clear")
	}
	l.syncManual(ManualTextStyle{}, false, len(w.presentation))
	if l.resets["manual"] != 0 {
		t.Fatalf("resets=%v", l.resets)
	}
	// A repeated event is idempotent.
	w.ObserveDisplayReset(11)
	if len(w.presentation) != n {
		t.Fatal("重複的模式事件應冪等")
	}
}

func TestModeResetManualPendingIsPoisoned(t *testing.T) {
	l, w, _ := modeResetManualLane(t)
	w.presentation = append(w.presentation, ManualPresentationEvent{Kind: ManualPresentationBegin, Generation: 1})
	w.collector.generation, w.collector.pending = 1, &pendingQuestion{generation: 1}
	l.syncManual(ManualTextStyle{}, false, len(w.presentation))
	n := len(w.presentation)
	w.ObserveDisplayReset(9)
	if len(w.presentation) != n || !w.collector.pending.poisoned {
		t.Fatalf("Pending 只應被標記 poisoned：events=%d poisoned=%v", len(w.presentation)-n, w.collector.pending.poisoned)
	}
	// The real clear is a no-op for a pending presenter, and nothing is stuck.
	w.ObserveClear(10)
	l.syncManual(ManualTextStyle{}, false, len(w.presentation))
	if l.resets["manual"] != 0 {
		t.Fatalf("resets=%v", l.resets)
	}
	// An idle watcher is left alone.
	w2 := NewWatcher(w.catalog)
	w2.ObserveDisplayReset(1)
	if len(w2.presentation) != 0 {
		t.Fatal("閒置的 watcher 不得有事件")
	}
}

func TestModeResetPostJoinWatcherBranches(t *testing.T) {
	r := manualE1FormalRuntime(t)
	idle := r.postJoin
	r.VideoModeChange(machine.ModeChange{Mode: 0x13, Step: 1})
	if r.postJoin != idle || r.resets["post-join"] != 0 {
		t.Fatal("閒置的 post-join watcher 不應被重建")
	}
	r.postJoin.active, r.postJoin.stage = true, 3
	active := r.postJoin
	r.VideoModeChange(machine.ModeChange{Mode: 0x13, Step: 2})
	if r.postJoin == active || r.postJoin.active || r.postJoin.failed || r.resets["post-join"] != 0 {
		t.Fatal("進行中的 post-join watcher 應重建，且不計 reset")
	}
	r.postJoin.failed = true
	failed := r.postJoin
	r.VideoModeChange(machine.ModeChange{Mode: 0x13, Step: 3})
	if r.postJoin != failed || !failed.failed || r.resets["post-join"] != 0 {
		t.Fatal("已 failed 的 watcher 不得被吞掉（計數留給下一次 entry）")
	}
}

func TestModeResetEmptyRuntimeChangesNothing(t *testing.T) {
	r := manualE1FormalRuntime(t)
	var palette [256][3]uint8
	palette[10] = [3]uint8{85, 255, 85}
	indexed := make([]byte, 320*200)
	r.Frame(indexed, palette)
	before, resets := r.DebugSummary(), strings.Join(sortedResets(r.Resets()), ",")
	for _, scale := range []int{2, 3} {
		out, _, err := r.ComposeWith(indexed, palette, scale)
		if err != nil || string(out) != string(ScaleIndexedRGBA(indexed, palette, scale)) {
			t.Fatalf("%d×：空 runtime 的合成應等於原版", scale)
		}
	}
	r.VideoModeChange(machine.ModeChange{Mode: 0x13, Step: 7})
	if after := r.DebugSummary(); after != before {
		t.Fatalf("空 runtime 的 DebugSummary 改變：\n前 %s\n後 %s", before, after)
	}
	if after := strings.Join(sortedResets(r.Resets()), ","); after != resets {
		t.Fatalf("Resets 改變：%s → %s", resets, after)
	}
	if fams := r.ActiveFamilies(LangZhTW); len(fams) != 0 {
		t.Fatalf("空 runtime 不應有 active 家族：%v", fams)
	}
}

func sortedResets(m map[string]int) []string {
	var out []string
	for k, v := range m {
		out = append(out, k+"="+string(rune('0'+v%10)))
	}
	sort.Strings(out)
	return out
}

// An empty story family changes nothing (spec 052 R2): the shallow copy of the
// watcher is identical after modeReset.
func TestModeResetEmptyStoryFamiliesChangeNothing(t *testing.T) {
	r := manualE1FormalRuntime(t)
	for _, l := range r.lanes {
		for _, f := range l.stories {
			v := reflect.ValueOf(f).Elem()
			w := v.FieldByName("w")
			if !w.IsValid() || w.Kind() != reflect.Ptr {
				continue // page 9 keeps its watchers in the owners
			}
			// The field is unexported: reach it through its address.
			elem := reflect.NewAt(w.Type(), unsafe.Pointer(w.UnsafeAddr())).Elem().Elem()
			before := reflect.New(elem.Type()).Elem()
			before.Set(elem)
			f.modeReset()
			if !reflect.DeepEqual(before.Interface(), elem.Interface()) {
				t.Errorf("%s：空狀態的 modeReset 改變了 watcher", f.name())
			}
		}
	}
}

// Every field of the live structs is classified; a new field without a row
// fails the test (spec 052 §3.3).
func TestModeResetClassifiesEveryField(t *testing.T) {
	reset := "reset"
	keep := "keep"
	lane := map[string]string{
		"lang": keep, "font": keep, "textDir": keep, "langDir": keep, "menuTexts": keep, "menuHdr": keep,
		"menuPres": reset, "skillCatalog": keep, "skill": reset, "exitCatalog": keep, "exit": reset,
		"postCatalog": keep, "postPres": reset, "actCatalog": keep, "actPres": reset, "bodyCatalog": keep, "bodyPres": reset,
		"manCatalog": keep, "manPres": reset, "manSync": keep, "manSeen": keep, "manStyle": keep, "manHas": keep,
		"manE1": keep, "manE1Rows": keep, "stories": reset, "names": keep, "players": keep, "playersOff": keep,
		"ecl": reset, "eclPres": reset, "eclGen": keep, "hmenu": reset, "hmenuPres": reset, "hmenuGen": keep,
		"engDisp": reset, "engDispPres": reset, "engDispGen": keep, "logbook": reset, "logbookPres": reset,
		"resets": keep, "rebuilds": keep, "skips": keep,
	}
	runtime := map[string]string{
		"help": keep, "font": keep, "menu": keep, "menuShared": keep, "postJoin": reset, "postShared": keep,
		"action": reset, "actRef": keep, "actRects": keep, "body": reset, "manual": reset, "manLayout": keep,
		"manCatalog": keep, "manEng": keep, "manEngPres": keep, "manEngOff": keep, "textDir": keep, "started": keep,
		"trace": keep, "files": keep, "storyPending": reset, "prevAt": reset, "prevOp": reset, "storyDirty": keep,
		"party": keep, "inlineNames": keep, "ovl": keep, "norm": keep, "normFrame": keep, "asciiFound": keep, "asciiTry": keep,
		"asciiScans": keep, "asciiStep": keep, "asciiStoryErrs": keep, "lanes": reset, "cur": keep, "off": keep,
		"indexed": keep, "palette": keep, "hasFrame": keep, "frameSeen": keep, "resets": keep,
	}
	check := func(name string, typ reflect.Type, table map[string]string) {
		for i := 0; i < typ.NumField(); i++ {
			f := typ.Field(i).Name
			if _, ok := table[f]; !ok {
				t.Errorf("%s.%s 沒有分類（規格 052 §3.3：重置或明示不動）", name, f)
			}
		}
		for f := range table {
			if _, ok := typ.FieldByName(f); !ok {
				t.Errorf("%s.%s 已不存在，分類表要更新", name, f)
			}
		}
	}
	check("liveLane", reflect.TypeOf(liveLane{}), lane)
	check("LiveRuntime", reflect.TypeOf(LiveRuntime{}), runtime)
}

// A mode set between a post-join call and its return: the rebuilt watcher
// knows nothing of the call, so the return fails closed (spec 052 §6: this is
// the expected fail-closed result and it is counted by resetPostJoin).
func TestModeResetPostJoinStraddleFailsClosed(t *testing.T) {
	c := postJoinTestCatalog()
	w, _ := NewPostJoinMenuWatcher(c)
	e := postJoinEvent(0, Address{0x37f1, 0x15bd}, 1)
	if err := w.ObserveEntry(e); err != nil {
		t.Fatal(err)
	}
	if w.idle() {
		t.Fatal("entry 之後不是閒置")
	}
	rebuilt, _ := NewPostJoinMenuWatcher(c) // what VideoModeChange does for a non-idle watcher
	e.PostCallStep = e.EntryStep + 1
	if err := rebuilt.ObserveReturn(e); err == nil {
		t.Fatal("跨事件的 return 應 fail closed")
	}
	// A watcher that already failed is not rebuilt: its failure stays pending.
	f, _ := NewPostJoinMenuWatcher(c)
	f.ObserveDiscontinuity()
	if f.idle() || !f.failed {
		t.Fatal("failed 的 watcher 不是閒置")
	}
	if err := f.ObserveEntry(postJoinEvent(0, Address{0x37f1, 0x15bd}, 2)); err == nil {
		t.Fatal("failed 的 watcher 在下一次 entry 應回錯（由 resetPostJoin 計數）")
	}
}

// Q1 shown, the mode is set, then the original prints Q2: the restored
// watcher has not accepted Q1, so Q2 fails and the lane's recover counts it
// once per scale.
func TestModeResetExitPromptQ1ThenQ2Entry(t *testing.T) {
	c, q1, q2 := exitPromptFixture()
	l := &liveLane{lang: LangZhTW, font: exitPromptFont(), exitCatalog: c,
		resets: map[string]int{}, rebuilds: map[string]int{}, skips: map[string]int{}}
	var palette [256][3]uint8
	for i, scale := range liveScales {
		o, err := NewPostJoinExitPromptOwner(c, l.font, scale)
		if err != nil {
			t.Fatal(err)
		}
		l.exit[i] = o
		if err := o.ObserveEntry(q1); err != nil {
			t.Fatal(err)
		}
		if err := o.ObserveReturn(exitPromptReturn(q1, q1.EntryStep+9000), palette); err != nil || len(o.Presenter.ActiveKeys()) == 0 {
			t.Fatalf("Q1 應顯示：err=%v", err)
		}
		o.Restore() // VideoModeChange
		if len(o.Presenter.ActiveKeys()) != 0 {
			t.Fatal("Restore 後 presenter 應為空")
		}
		if o.Watcher.Failed() {
			t.Fatal("Restore 不得使 watcher failed")
		}
	}
	// The original now prints Q2.
	for i := range liveScales {
		if l.exit[i].Watcher.ShouldObserveEntry(q2) {
			if err := l.exit[i].ObserveEntry(q2); err == nil {
				t.Fatal("Q2 在 Q1 未被接受時應 fail")
			}
			if err := l.recover("exit-prompt", func() error { return l.resetExit(i) }); err != nil {
				t.Fatal(err)
			}
		}
	}
	if l.resets["exit-prompt"] != 2 {
		t.Fatalf("resets[exit-prompt] = %d，應為每個倍率各一次（2）", l.resets["exit-prompt"])
	}
}
