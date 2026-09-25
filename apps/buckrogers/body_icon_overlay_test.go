package buckrogers

import (
	"crypto/sha256"
	"reflect"
	"testing"

	"github.com/wicanr2/dosgolem/internal/machine"
	"github.com/wicanr2/dosgolem/xlate"
)

var bodyTestSavePrefix, bodyTestSaveSuffix = sha256.Sum256([]byte("pre--")), sha256.Sum256([]byte("?!"))

func bodyTestCatalog() *BodyIconCatalog {
	c := &BodyIconCatalog{byKey: map[string]BodyIconIdentity{}}
	for i, key := range bodyIconKeys {
		if key == bodySaveKey {
			continue
		}
		caller := Address{Segment: 0x1c41, Offset: uint16(0x2708 + (i-2)*0x21)}
		if i == 0 || i == 6 {
			caller = Address{Segment: 0x37f1, Offset: 0x101e}
		}
		row, col, length := uint8(6+i), uint8(0), 3
		x, y, width := 8, int(row)*8, 24
		if i == 0 {
			row, length, x, y, width = 24, 17, 0, 192, 136
		}
		if i == 2 {
			row, col, length, x, y, width = 6, 8, 3, 64, 48, 24
		}
		if i == 3 {
			row, col, length, x, y, width = 10, 3, 14, 24, 80, 112
		}
		if i == 4 {
			row, col, length, x, y, width = 12, 8, 3, 64, 96, 24
		}
		if i == 5 {
			row, col, length, x, y, width = 16, 3, 14, 24, 128, 112
		}
		if i == 6 {
			row, length, x, y, width = 24, 20, 0, 192, 160
		}
		translation := []rune("界")
		if i == 0 {
			translation = []rune("確認")
		}
		if i == 3 || i == 5 {
			translation = []rune("準備動作")
		}
		if i == 6 {
			translation = []rune("選擇圖示")
		}
		c.byKey[key] = BodyIconIdentity{EventKey: key, Caller: caller, Length: uint8(length), SHA256: sha256.Sum256([]byte(key)), BG: 0, FG: 15, Row: row, Column: col, TextKey: key, Translation: string(translation), Rect: PixelRect{x, y, width, 8}, DrawX: x, DrawY: y, Capacity: length}
	}
	c.save = BodyIconAffix{EventKey: bodySaveKey, Caller: Address{Segment: 0x37f1, Offset: 0x101e}, BG: 0, FG: 13, Row: 24, Column: 0,
		PrefixLength: 5, SuffixLength: 2, PrefixSHA256: bodyTestSavePrefix, SuffixSHA256: bodyTestSaveSuffix, SlotMin: 1, SlotMax: 15,
		PrefixText: "儲存", SuffixText: "？", PrefixRect: PixelRect{0, 192, 40, 8}}
	return c
}

// bodyTestSaveEvent builds the save prompt event for a name of n bytes through
// the real recorder, so the affix hashing path is exercised end to end.
func bodyTestSaveEvent(t *testing.T, name string, step uint64) TextEvent {
	t.Helper()
	var r TextRecorder
	if err := r.RegisterAffix(AffixShape{Caller: Address{0x37f1, 0x101e}, PrefixLength: 5, SuffixLength: 2}); err != nil {
		t.Fatal(err)
	}
	original := []byte("pre--" + name + "?!")
	r.ObserveDispatchEntry(Address{0x37f1, 0x101e}, 1, 100, [6]uint16{0, 0, 0, 13, 24, 0}, original, step)
	r.ObserveInstruction(Address{0x37f1, 0x101e}, 1, 100+dispatcherStackDelta, step+1)
	ev := r.Events()
	if len(ev) != 1 {
		t.Fatal("recorder did not complete save prompt frame")
	}
	return ev[0]
}

func bodyTestEvent(c *BodyIconCatalog, key string, step uint64) TextEvent {
	id := c.byKey[key]
	return TextEvent{EntryStep: step, PostCallStep: step + 1, Caller: id.Caller, OriginalLength: id.Length, OriginalSHA256: id.SHA256, Background: id.BG, Foreground: id.FG, Row: id.Row, Column: id.Column}
}
func observeBodyKeys(t *testing.T, w *BodyIconWatcher, c *BodyIconCatalog, keys []string) {
	t.Helper()
	for i, key := range keys {
		if err := w.Observe(bodyTestEvent(c, key, uint64(i+1)*10)); err != nil {
			t.Fatal(err)
		}
	}
}

func TestBodyIconWatcherFixedRoutesAndGeneration(t *testing.T) {
	c := bodyTestCatalog()
	cases := []struct {
		route       BodyIconRoute
		tail        []string
		generations []uint64
		groups      []string
	}{{BodyIconMove, []string{"body.icon.selection.instruction"}, []uint64{1, 2}, []string{"body-selection", "selection-redraw"}}, {BodyIconRefuse, []string{"body.icon.confirmation", "body.icon.old.label", "body.icon.old.action", "body.icon.new.label", "body.icon.new.action", "body.icon.selection.instruction"}, []uint64{1, 2, 3}, []string{"body-selection", "confirmation", "body-selection"}}}
	for _, tc := range cases {
		w, err := NewBodyIconWatcher(tc.route, c)
		if err != nil {
			t.Fatal(err)
		}
		keys := append(append([]string{}, bodyInitialKeys...), tc.tail...)
		observeBodyKeys(t, w, c, keys)
		if !w.Complete() {
			t.Fatalf("%s incomplete", tc.route)
		}
		tr := w.Transitions()
		if len(tr) != len(tc.generations) {
			t.Fatalf("%s transitions=%d", tc.route, len(tr))
		}
		for i, v := range tr {
			if v.Generation != tc.generations[i] || v.Group != tc.groups[i] {
				t.Fatalf("%s transition %d=%+v", tc.route, i, v)
			}
		}
	}
}

func TestBodyIconWatcherFailClosedNegativeMatrix(t *testing.T) {
	c := bodyTestCatalog()
	t.Run("reorder", func(t *testing.T) {
		w, _ := NewBodyIconWatcher(BodyIconMove, c)
		if err := w.Observe(bodyTestEvent(c, bodyInitialKeys[1], 1)); err == nil || !w.Failed() {
			t.Fatal("reorder accepted")
		}
	})
	t.Run("wrong identity", func(t *testing.T) {
		w, _ := NewBodyIconWatcher(BodyIconMove, c)
		e := bodyTestEvent(c, bodyInitialKeys[0], 1)
		e.Column++
		if err := w.Observe(e); err == nil || !w.Failed() {
			t.Fatal("wrong identity accepted")
		}
	})
	t.Run("extra group event", func(t *testing.T) {
		w, _ := NewBodyIconWatcher(BodyIconMove, c)
		observeBodyKeys(t, w, c, bodyInitialKeys)
		if err := w.Observe(bodyTestEvent(c, "body.icon.confirmation", 100)); err == nil || !w.Failed() {
			t.Fatal("out-of-route confirmation accepted")
		}
	})
	t.Run("incomplete", func(t *testing.T) {
		w, _ := NewBodyIconWatcher(BodyIconMove, c)
		observeBodyKeys(t, w, c, bodyInitialKeys[:4])
		if w.Complete() {
			t.Fatal("partial group completed")
		}
	})
	t.Run("unknown route", func(t *testing.T) {
		if _, err := NewBodyIconWatcher("restore", c); err == nil {
			t.Fatal("unknown route accepted")
		}
	})
}

func TestBodyIconRuntimePrewriteAtomicStampInvalidation(t *testing.T) {
	c := bodyTestCatalog()
	font := bodyTestFont(c)
	for _, scale := range []int{2, 3} {
		o, err := NewRuntimeBodyIconOverlay(c, font, scale, BodyIconMove)
		if err != nil {
			t.Fatal(err)
		}
		w, _ := NewBodyIconWatcher(BodyIconMove, c)
		observeBodyKeys(t, w, c, bodyInitialKeys)
		if err = o.Apply(w.Transitions()[0], [256][3]uint8{}); err != nil {
			t.Fatal(err)
		}
		before := len(o.ActiveKeys())
		if before != 5 {
			t.Fatalf("scale %d active=%d", scale, before)
		}
		// The same-value property is irrelevant to the observer: callback metadata
		// alone must invalidate the complete intersected stamp before the write.
		o.Prewrite(machine.VideoWrite{Step: 77, CS: 0xdead, IP: 0xbeef, Offset: uint32(48*320 + 64), WriteMode: 1, Value: 0})
		after := len(o.ActiveKeys())
		if after != before-1 || len(o.Invalidations()) != 1 || len(o.Invalidations()[0].Invalidated) != 1 {
			t.Fatalf("scale %d partial/missed invalidation: before=%d after=%d trace=%+v", scale, before, after, o.Invalidations())
		}
		// A second intersecting writer removes its entire stamp, not one clipped cell.
		o.Prewrite(machine.VideoWrite{Step: 78, CS: 0xaaaa, IP: 0xbbbb, Offset: uint32(80*320 + 24), WriteMode: 0, Value: 1})
		if len(o.ActiveKeys()) != before-2 {
			t.Fatalf("scale %d mixed writer left partial stamp", scale)
		}
		all, err := NewRuntimeBodyIconOverlay(c, font, scale, BodyIconMove)
		if err != nil {
			t.Fatal(err)
		}
		if err = all.Apply(w.Transitions()[0], [256][3]uint8{}); err != nil {
			t.Fatal(err)
		}
		all.Prewrite(machine.VideoWrite{Step: 79, CS: 0xffff, IP: 0xffff, Offset: 0x10000, WriteMode: 3})
		if len(all.ActiveKeys()) != 0 || len(all.Invalidations()) != 1 || all.Invalidations()[0].Reason != "a000-offset-out-of-range" || len(all.Invalidations()[0].Invalidated) != 5 {
			t.Fatalf("scale %d out-of-range did not fail closed: %+v", scale, all.Invalidations())
		}
	}
}

func TestBodyIconRuntimeRejectsPartialMixedWrongKeyAndGeneration(t *testing.T) {
	c := bodyTestCatalog()
	font := bodyTestFont(c)
	w, _ := NewBodyIconWatcher(BodyIconMove, c)
	observeBodyKeys(t, w, c, bodyInitialKeys)
	valid := w.Transitions()[0]
	cases := []struct {
		name   string
		mutate func(*BodyIconTransition)
	}{{"partial", func(t *BodyIconTransition) { t.Events = t.Events[:len(t.Events)-1] }}, {"wrong generation", func(t *BodyIconTransition) { t.Generation++; t.Events[0].Generation++ }}, {"wrong key", func(t *BodyIconTransition) { t.Events[0].EventKey = "body.icon.confirmation" }}, {"mixed group", func(t *BodyIconTransition) { t.Events[0].Group = "confirmation" }}}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			o, err := NewRuntimeBodyIconOverlay(c, font, 2, BodyIconMove)
			if err != nil {
				t.Fatal(err)
			}
			bad := valid
			bad.Events = append([]BodyIconEvent(nil), valid.Events...)
			tc.mutate(&bad)
			if err := o.Apply(bad, [256][3]uint8{}); err == nil || len(o.ActiveKeys()) != 0 {
				t.Fatal("invalid transition was not rejected atomically")
			}
		})
	}
}

func bodyTestFont(c *BodyIconCatalog) *xlate.Font {
	font := &xlate.Font{W: 16, H: 16, Glyphs: map[rune][]byte{}}
	texts := []string{c.save.PrefixText, c.save.SuffixText}
	for _, id := range c.byKey {
		texts = append(texts, id.Translation)
	}
	for _, text := range texts {
		for _, r := range text {
			font.Glyphs[r] = make([]byte, 32)
			for i := range font.Glyphs[r] {
				font.Glyphs[r][i] = 0xff
			}
		}
	}
	return font
}

func confirmToSavePrompt(t *testing.T, c *BodyIconCatalog, name string) *BodyIconWatcher {
	t.Helper()
	w, _ := NewBodyIconWatcher(BodyIconConfirm, c)
	observeBodyKeys(t, w, c, append(append([]string{}, bodyInitialKeys...), "body.icon.confirmation"))
	if err := w.Observe(bodyTestSaveEvent(t, name, 1000)); err != nil {
		t.Fatal(err)
	}
	return w
}

func TestBodyIconSavePromptAffixAcceptsAnyNameLength(t *testing.T) {
	c := bodyTestCatalog()
	for _, name := range []string{"Z", "BUCK", "A 1.?", "ABCDEFGHIJKLMNO", "?!", "pre--"} {
		w := confirmToSavePrompt(t, c, name)
		tr := w.Transitions()
		if !w.Complete() || len(tr) != 3 || tr[2].Group != "save_prompt" || tr[2].Events[0].SlotLength != uint8(len(name)) {
			t.Fatalf("%q: transitions=%+v", name, tr)
		}
	}
}

func TestBodyIconSavePromptAffixFailsClosed(t *testing.T) {
	c := bodyTestCatalog()
	base := func(t *testing.T) *BodyIconWatcher {
		w, _ := NewBodyIconWatcher(BodyIconConfirm, c)
		observeBodyKeys(t, w, c, append(append([]string{}, bodyInitialKeys...), "body.icon.confirmation"))
		return w
	}
	cases := map[string]func(*TextEvent){
		"prefix mismatch":  func(e *TextEvent) { e.Affix.PrefixSHA256[0] ^= 1 },
		"suffix mismatch":  func(e *TextEvent) { e.Affix.SuffixSHA256[0] ^= 1 },
		"length mismatch":  func(e *TextEvent) { e.OriginalLength++ },
		"slot too long":    func(e *TextEvent) { e.Affix.SlotLength = 16; e.OriginalLength = 23 },
		"no affix":         func(e *TextEvent) { e.Affix = nil },
		"wrong foreground": func(e *TextEvent) { e.Foreground = 15 },
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			w := base(t)
			e := bodyTestSaveEvent(t, "BUCK", 1000)
			a := *e.Affix
			e.Affix = &a
			mutate(&e)
			if err := w.Observe(e); err == nil || !w.Failed() {
				t.Fatal("mutated save prompt accepted")
			}
		})
	}
	t.Run("suffix-only collision", func(t *testing.T) {
		// A same-caller string ending with the save suffix but a different
		// prefix (the confirmation question shape) must not match the affix.
		w := base(t)
		var r TextRecorder
		_ = r.RegisterAffix(AffixShape{Caller: Address{0x37f1, 0x101e}, PrefixLength: 5, SuffixLength: 2})
		r.ObserveDispatchEntry(Address{0x37f1, 0x101e}, 1, 100, [6]uint16{0, 0, 0, 13, 24, 0}, []byte("OTHERBUCK?!"), 1000)
		r.ObserveInstruction(Address{0x37f1, 0x101e}, 1, 100+dispatcherStackDelta, 1001)
		if err := w.Observe(r.Events()[0]); err == nil || !w.Failed() {
			t.Fatal("suffix-only match accepted")
		}
	})
}

func TestRecorderAffixDoesNotRetainSlotBytes(t *testing.T) {
	var r TextRecorder
	if err := r.RegisterAffix(AffixShape{Caller: Address{1, 2}, PrefixLength: 5, SuffixLength: 2}); err != nil {
		t.Fatal(err)
	}
	if err := r.RegisterAffix(AffixShape{Caller: Address{1, 2}, PrefixLength: 5, SuffixLength: 2}); err == nil {
		t.Fatal("duplicate caller accepted")
	}
	r.ObserveDispatchEntry(Address{1, 2}, 1, 100, [6]uint16{}, []byte("pre--"+"?!"), 1)
	r.ObserveInstruction(Address{1, 2}, 1, 100+dispatcherStackDelta, 2)
	r.ObserveDispatchEntry(Address{3, 4}, 1, 100, [6]uint16{}, []byte("pre--X?!"), 3)
	r.ObserveInstruction(Address{3, 4}, 1, 100+dispatcherStackDelta, 4)
	ev := r.Events()
	if ev[0].Affix != nil || ev[1].Affix != nil {
		t.Fatal("empty slot or unregistered caller produced an affix")
	}
	typ := reflect.TypeOf(TextAffix{})
	for i := 0; i < typ.NumField(); i++ {
		if k := typ.Field(i).Type.Kind(); k == reflect.Slice || k == reflect.String {
			t.Fatalf("TextAffix field %s can retain slot bytes", typ.Field(i).Name)
		}
	}
}

func TestBodyIconSavePromptStampsSkipNameCells(t *testing.T) {
	c := bodyTestCatalog()
	font := bodyTestFont(c)
	for _, scale := range []int{2, 3} {
		for _, name := range []string{"Z", "BUCK", "ABCDEFGHIJKLMNO"} {
			o, err := NewRuntimeBodyIconOverlay(c, font, scale, BodyIconConfirm)
			if err != nil {
				t.Fatal(err)
			}
			w := confirmToSavePrompt(t, c, name)
			for _, tr := range w.Transitions() {
				if err := o.Apply(tr, [256][3]uint8{}); err != nil {
					t.Fatal(err)
				}
			}
			n := len(name)
			rects := o.SafeRects()
			want := []PixelRect{{0, 192 * scale, 40 * scale, 8 * scale}, {(5 + n) * 8 * scale, 192 * scale, 16 * scale, 8 * scale}}
			if len(rects) != 2 || rects[0] != want[0] || rects[1] != want[1] {
				t.Fatalf("scale %d %q rects=%v want %v", scale, name, rects, want)
			}
			indexed := make([]byte, 320*200)
			rgba, missing, drew := o.Draw(indexed, [256][3]uint8{})
			if !drew || len(missing) != 0 {
				t.Fatalf("scale %d %q draw=%v missing=%v", scale, name, drew, missing)
			}
			base := ScaleIndexedRGBA(indexed, [256][3]uint8{}, scale)
			for y := 192 * scale; y < 200*scale; y++ {
				for x := 40 * scale; x < (5+n)*8*scale; x++ {
					i := (y*320*scale + x) * 4
					if string(rgba[i:i+4]) != string(base[i:i+4]) {
						t.Fatalf("scale %d %q name cell (%d,%d) changed", scale, name, x, y)
					}
				}
			}
			// A write into the suffix alone invalidates the whole prompt.
			o.Prewrite(machine.VideoWrite{Step: 5000, Offset: uint32(192*320 + (5+n)*8)})
			if len(o.ActiveKeys()) != 0 || len(o.Invalidations()) != 1 || len(o.Invalidations()[0].Invalidated) != 2 {
				t.Fatalf("scale %d %q suffix write left %v", scale, name, o.ActiveKeys())
			}
		}
	}
}
