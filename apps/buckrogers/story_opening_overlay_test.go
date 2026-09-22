package buckrogers

import (
	"github.com/wicanr2/dosgolem/xlate"
	"testing"
)

func storyOpeningOverlayFixture(t *testing.T, scale int) (*RuntimeStoryOpeningOverlay, []StoryOpeningEvent) {
	t.Helper()
	f := &xlate.Font{W: 16, H: 16, Glyphs: map[rune][]byte{}}
	for _, r := range []rune("甲乙丙丁戊") {
		f.Glyphs[r] = make([]byte, 32)
		f.Glyphs[r][0] = 0x80
	}
	text := map[string]string{}
	events := make([]StoryOpeningEvent, 5)
	for i := range events {
		key := "story.opening.line.00" + string(rune('1'+i))
		text[key] = string([]rune("甲乙丙丁戊")[i])
		entry := uint64(100 + i*10)
		events[i] = StoryOpeningEvent{Generation: 1, EntryStep: entry, PostCallStep: entry + 1, EventKey: key, Row: uint8(17 + i), Column: 1}
	}
	o, err := NewRuntimeStoryOpeningOverlay(text, f, scale)
	if err != nil {
		t.Fatal(err)
	}
	return o, events
}

func TestRuntimeStoryOpeningOverlayAtomicAndClear(t *testing.T) {
	o, ev := storyOpeningOverlayFixture(t, 3)
	layer := o.PresentationLayer()
	if layer == nil {
		t.Fatal("presentation layer nil")
	}
	if o.font.Name != "buckrogers-story-3x22" {
		t.Fatalf("未命名 3×衍生字型 name=%q", o.font.Name)
	}
	if err := o.Apply(ev, [256][3]uint8{}); err != nil {
		t.Fatal(err)
	}
	out, miss, d := o.Draw(make([]byte, 320*200), [256][3]uint8{})
	if !d || len(miss) != 0 || len(out) != 320*200*9*4 {
		t.Fatal("draw")
	}
	o.Clear()
	if o.PresentationLayer() != layer {
		t.Fatal("Clear replaced presentation layer")
	}
	if len(layer.Stamps) != 0 {
		t.Fatal("Clear retained story stamps")
	}
	_, _, d = o.Draw(make([]byte, 320*200), [256][3]uint8{})
	if d {
		t.Fatal("clear")
	}
}

func TestRuntimeStoryOpeningOverlayRejectsMalformedGroupsWithoutDrawing(t *testing.T) {
	for _, scale := range []int{2, 3} {
		for _, tc := range []struct {
			name string
			edit func([]StoryOpeningEvent)
		}{
			{"zero generation", func(es []StoryOpeningEvent) { es[0].Generation = 0 }},
			{"mixed generation", func(es []StoryOpeningEvent) { es[4].Generation = 2 }},
			{"duplicate key", func(es []StoryOpeningEvent) { es[4].EventKey = es[3].EventKey }},
			{"wrong expected key", func(es []StoryOpeningEvent) { es[3].EventKey = "story.opening.line.999" }},
			{"wrong row", func(es []StoryOpeningEvent) { es[4].Row++ }},
			{"wrong column", func(es []StoryOpeningEvent) { es[4].Column++ }},
			{"zero duration", func(es []StoryOpeningEvent) { es[4].PostCallStep = es[4].EntryStep }},
			{"overlapping order", func(es []StoryOpeningEvent) { es[4].EntryStep = es[3].PostCallStep }},
		} {
			t.Run(tc.name, func(t *testing.T) {
				o, events := storyOpeningOverlayFixture(t, scale)
				tc.edit(events)
				if err := o.Apply(events, [256][3]uint8{}); err == nil {
					t.Fatal("malformed group accepted")
				}
				if len(o.ActiveKeys()) != 0 || o.active {
					t.Fatalf("malformed group retained stamps=%v active=%v", o.ActiveKeys(), o.active)
				}
				_, missing, drew := o.Draw(make([]byte, 320*200), [256][3]uint8{})
				if drew || len(missing) != 0 {
					t.Fatalf("malformed group drew=%v missing=%v", drew, missing)
				}
			})
		}
	}
}
