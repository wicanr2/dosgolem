package main

import (
	"bytes"
	"reflect"
	"testing"
)

func TestScreenEventsUseExactSelectionVariants(t *testing.T) {
	wantSteady := []string{
		"race.screen.prompt", "race.heading.terran", "race.option.martian",
		"race.option.venusian", "race.option.mercurian", "race.option.tinker",
		"race.option.desert_runner",
	}
	wantDown := []string{
		"race.screen.prompt", "race.selection.normal.terran", "race.selection.selected.martian",
		"race.option.venusian", "race.option.mercurian", "race.option.tinker",
		"race.option.desert_runner",
	}
	if got := screenEvents("steady"); !reflect.DeepEqual(got, wantSteady) {
		t.Fatalf("steady events = %v", got)
	}
	if got := screenEvents("down"); !reflect.DeepEqual(got, wantDown) {
		t.Fatalf("down events = %v", got)
	}
	wants := map[string][]string{
		"gender-steady": {"gender.screen.prompt", "gender.selection.selected.male", "gender.option.female"},
		"gender-down":   {"gender.screen.prompt", "gender.selection.normal.male", "gender.selection.selected.female"},
		"class-steady":  {"class.screen.prompt", "class.selection.selected.rocket_jock", "class.option.warrior", "class.option.medic", "class.option.engineer", "class.option.rogue"},
		"class-down":    {"class.screen.prompt", "class.selection.normal.rocket_jock", "class.selection.selected.warrior", "class.option.medic", "class.option.engineer", "class.option.rogue"},
	}
	for screen, want := range wants {
		if got := screenEvents(screen); !reflect.DeepEqual(got, want) {
			t.Fatalf("%s events = %v，要 %v", screen, got, want)
		}
	}
	if validScreen("unknown") || screenEvents("unknown") != nil {
		t.Fatal("未知 screen 必須拒絕")
	}
}

func TestDecodePaletteKeepsProbeEightBitRGB(t *testing.T) {
	raw := make([]byte, 256*3)
	raw[10*3], raw[10*3+1], raw[10*3+2] = 85, 255, 85
	got := decodePalette(raw)
	if got[10] != [3]uint8{85, 255, 85} {
		t.Fatalf("palette[10] = %v；.pal 不可再次做 6→8 位元轉換", got[10])
	}
}

func TestDiffOutsideApprovedRect(t *testing.T) {
	base := bytes.Repeat([]byte{0, 0, 0, 255}, 4*4)
	rendered := append([]byte(nil), base...)
	events := []eventJSON{{ClearRect: rectJSON{X: 1, Y: 1, Width: 2, Height: 2}}}
	inside := (1*4 + 1) * 4
	rendered[inside] = 1
	if got := diffOutside(base, rendered, events, 4, 4); got != 0 {
		t.Fatalf("矩形內差異被算成外部差異：%d", got)
	}
	outside := (0*4 + 0) * 4
	rendered[outside] = 1
	if got := diffOutside(base, rendered, events, 4, 4); got != 1 {
		t.Fatalf("矩形外差異 = %d，要 1", got)
	}
}

func TestOverlappingRects(t *testing.T) {
	events := []eventJSON{
		{ClearRect: rectJSON{X: 0, Y: 0, Width: 8, Height: 8}},
		{ClearRect: rectJSON{X: 8, Y: 0, Width: 8, Height: 8}},
	}
	if got := overlapping(events); got != 0 {
		t.Fatalf("相鄰矩形誤判重疊：%d", got)
	}
	events[1].ClearRect.X = 7
	if got := overlapping(events); got != 1 {
		t.Fatalf("重疊矩形計數 = %d，要 1", got)
	}
}
