package buckrogers

import (
	"crypto/sha256"
	"testing"

	"github.com/wicanr2/dosgolem/internal/machine"
	"github.com/wicanr2/dosgolem/xlate"
)

func bodyTestCatalog() *BodyIconCatalog {
	c := &BodyIconCatalog{byKey: map[string]BodyIconIdentity{}}
	for i, key := range bodyIconKeys {
		caller := Address{Segment: 0x1c41, Offset: uint16(0x2708 + (i-2)*0x21)}
		if i == 0 || i == 1 || i == 6 {
			caller = Address{Segment: 0x37f1, Offset: 0x101e}
		}
		row, col, length := uint8(6+i), uint8(0), 3
		x, y, width := 8, int(row)*8, 24
		if i == 0 {
			row, length, x, y, width = 24, 17, 0, 192, 136
		}
		if i == 1 {
			row, length, x, y, width = 24, 8, 0, 192, 64
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
		if i == 1 {
			translation = []rune("儲存")
		}
		if i == 3 || i == 5 {
			translation = []rune("準備動作")
		}
		if i == 6 {
			translation = []rune("選擇圖示")
		}
		c.byKey[key] = BodyIconIdentity{EventKey: key, Caller: caller, Length: uint8(length), SHA256: sha256.Sum256([]byte(key)), BG: 0, FG: 15, Row: row, Column: col, TextKey: key, Translation: string(translation), Rect: PixelRect{x, y, width, 8}, DrawX: x, DrawY: y, Capacity: length}
	}
	return c
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
	}{{BodyIconMove, []string{"body.icon.selection.instruction"}, []uint64{1, 2}, []string{"body-selection", "selection-redraw"}}, {BodyIconRefuse, []string{"body.icon.confirmation", "body.icon.old.label", "body.icon.old.action", "body.icon.new.label", "body.icon.new.action", "body.icon.selection.instruction"}, []uint64{1, 2, 3}, []string{"body-selection", "confirmation", "body-selection"}}, {BodyIconConfirm, []string{"body.icon.confirmation", "body.icon.save_prompt"}, []uint64{1, 2, 3}, []string{"body-selection", "confirmation", "save_prompt"}}}
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
	font := &xlate.Font{W: 16, H: 16, Glyphs: map[rune][]byte{}}
	for _, id := range c.byKey {
		for _, r := range id.Translation {
			font.Glyphs[r] = make([]byte, 32)
			for i := range font.Glyphs[r] {
				font.Glyphs[r][i] = 0xff
			}
		}
	}
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
	font := &xlate.Font{W: 16, H: 16, Glyphs: map[rune][]byte{}}
	for _, id := range c.byKey {
		for _, r := range id.Translation {
			font.Glyphs[r] = make([]byte, 32)
			for i := range font.Glyphs[r] {
				font.Glyphs[r][i] = 0xff
			}
		}
	}
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
