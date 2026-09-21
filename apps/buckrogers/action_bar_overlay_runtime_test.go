package buckrogers

import (
	"bytes"
	"testing"
)

func actionEventFor(c *ActionBarRequestCatalog, screen, key, variant string) (ActionBarEvent, DisplayRequest) {
	for _, entry := range c.events.byScreen[screen] {
		if entry.key != key {
			continue
		}
		e := ActionBarEvent{EntryStep: 1, PostCallStep: 2, Screen: screen,
			EventKey: screen + "." + key + "." + variant, Variant: variant, OriginalLength: entry.length,
			OriginalSHA256: entry.hash, Row: entry.row, Column: entry.column,
			X0: entry.x0, Y0: entry.y0, X1: entry.x1, Y1: entry.y1}
		r, _ := c.Resolve(e)
		return e, r
	}
	return ActionBarEvent{}, DisplayRequest{}
}

func TestRuntimeActionBarOverlayRequiresHotkeyPreservingStyleAtBothScales(t *testing.T) {
	c, rects := formalActionOverlay(t)
	if _, err := NewRuntimeActionBarOverlay(c, rects, actionOverlayFont(), 2, nil); err == nil {
		t.Fatal("nil style must fail")
	}
	if _, err := NewRuntimeActionBarOverlay(c, rects, actionOverlayFont(), 2, &ActionBarNormalStyle{RuneForegrounds: []uint8{10, 10, 10, 10, 10}}); err == nil {
		t.Fatal("discarding the white mnemonic must fail")
	}
	style := HotkeyPreservingActionBarNormalStyle()
	for _, scale := range []int{2, 3} {
		o, err := NewRuntimeActionBarOverlay(c, rects, actionOverlayFont(), scale, &style)
		if err != nil {
			t.Fatal(err)
		}
		o.ObserveAnchorEvent("career.screen.remaining_points.heading")
		e, r := actionEventFor(c, "career", "action.add", "normal")
		if err := o.Apply(e, r, [256][3]uint8{}); err != nil || len(o.ActiveKeys()) != 1 {
			t.Fatalf("err=%v keys=%v", err, o.ActiveKeys())
		}
	}
}

func TestRuntimeActionBarOverlayAtomicallyReplacesVariantsAndPreservesOtherActions(t *testing.T) {
	c, rects := formalActionOverlay(t)
	style := HotkeyPreservingActionBarNormalStyle()
	o, _ := NewRuntimeActionBarOverlay(c, rects, actionOverlayFont(), 2, &style)
	o.ObserveAnchorEvent("career.screen.remaining_points.heading")
	for _, item := range [][2]string{{"action.add", "normal"}, {"action.subtract", "normal"}, {"action.add", "focus"}, {"action.add", "normal"}} {
		e, r := actionEventFor(c, "career", item[0], item[1])
		if err := o.Apply(e, r, [256][3]uint8{}); err != nil {
			t.Fatal(err)
		}
	}
	keys := o.ActiveKeys()
	if len(keys) != 2 || keys[0] != "career.action.add.normal" || keys[1] != "career.action.subtract.normal" {
		t.Fatalf("keys=%v", keys)
	}
	if len(o.layer.Stamps) != 10 {
		t.Fatalf("stamps=%d", len(o.layer.Stamps))
	}
}

func TestRuntimeActionBarOverlayClearAndAnchorInvalidationAreGroupAtomic(t *testing.T) {
	c, rects := formalActionOverlay(t)
	style := HotkeyPreservingActionBarNormalStyle()
	o, _ := NewRuntimeActionBarOverlay(c, rects, actionOverlayFont(), 2, &style)
	o.ObserveAnchorEvent("career.screen.remaining_points.heading")
	for _, key := range []string{"action.add", "action.subtract"} {
		e, r := actionEventFor(c, "career", key, "normal")
		if err := o.Apply(e, r, [256][3]uint8{}); err != nil {
			t.Fatal(err)
		}
	}
	if err := o.ClearTextCells(24, 1, 24, 1); err != nil {
		t.Fatal(err)
	}
	if keys := o.ActiveKeys(); len(keys) != 1 || keys[0] != "career.action.subtract.normal" {
		t.Fatalf("keys=%v", keys)
	}
	o.ObserveAnchorEvent("technical.screen.general_points.heading")
	if len(o.ActiveKeys()) != 0 {
		t.Fatal("screen transition must clear all groups")
	}
	e, r := actionEventFor(c, "technical", "action.add", "normal")
	if err := o.Apply(e, r, [256][3]uint8{}); err != nil {
		t.Fatal(err)
	}
	o.ObserveAnchorEvent("career.screen.maximum_per_skill.heading")
	if len(o.ActiveKeys()) != 1 {
		t.Fatal("proven shared key must preserve technical group")
	}
	o.ObserveAnchorEvent("class.option.warrior")
	if len(o.ActiveKeys()) != 0 {
		t.Fatal("unrelated exact event must clear groups")
	}
	if err := o.ClearTextCells(2, 1, 3, 1); err == nil {
		t.Fatal("invalid clear must fail")
	}
}

func TestRuntimeActionBarOverlayFrameDrawReappliesInjectedColors(t *testing.T) {
	c, rects := formalActionOverlay(t)
	style := HotkeyPreservingActionBarNormalStyle()
	o, _ := NewRuntimeActionBarOverlay(c, rects, actionOverlayFont(), 2, &style)
	o.ObserveAnchorEvent("career.screen.remaining_points.heading")
	e, r := actionEventFor(c, "career", "action.add", "normal")
	var pal [256][3]uint8
	pal[10] = [3]uint8{1, 2, 3}
	pal[15] = [3]uint8{4, 5, 6}
	if err := o.Apply(e, r, pal); err != nil {
		t.Fatal(err)
	}
	indexed := bytes.Repeat([]byte{0}, 320*200)
	indexed[192*320] = 15
	indexed[192*320+1] = 10
	o.Frame(indexed, pal)
	for index, stamp := range o.layer.Stamps {
		want := pal[10]
		if index == 1 {
			want = pal[15]
		}
		if stamp.FG != want {
			t.Fatalf("快捷字母配色被覆寫 index=%d got=%v want=%v", index, stamp.FG, want)
		}
	}
	rgba, missing, drew := o.Draw(indexed, pal)
	if !drew || len(missing) != 0 || len(rgba) != 640*400*4 {
		t.Fatalf("drew=%v missing=%v rgba=%d", drew, missing, len(rgba))
	}
	actions := o.Actions()
	actions[0].RuneForegrounds[0] = 99
	if o.Actions()[0].RuneForegrounds[0] == 99 {
		t.Fatal("actions must return deep copies")
	}
}

func TestRuntimeActionBarOverlayRejectsUnanchoredAndIdentityDrift(t *testing.T) {
	c, rects := formalActionOverlay(t)
	style := HotkeyPreservingActionBarNormalStyle()
	o, _ := NewRuntimeActionBarOverlay(c, rects, actionOverlayFont(), 2, &style)
	e, r := actionEventFor(c, "career", "action.add", "normal")
	if err := o.Apply(e, r, [256][3]uint8{}); err == nil {
		t.Fatal("unanchored apply must fail")
	}
	o.ObserveAnchorEvent("career.screen.remaining_points.heading")
	e.X1++
	if err := o.Apply(e, r, [256][3]uint8{}); err == nil {
		t.Fatal("identity drift must fail")
	}
}
