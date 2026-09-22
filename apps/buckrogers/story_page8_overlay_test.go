package buckrogers

import (
	"bytes"
	"fmt"
	"testing"

	"github.com/wicanr2/dosgolem/xlate"
)

func page8OverlayFixture(t *testing.T, scale int) (*RuntimeStoryPage8Overlay, []StoryPage8Event) {
	t.Helper()
	font := &xlate.Font{W: 16, H: 16, Glyphs: make(map[rune][]byte)}
	text := make(map[string]string, 4)
	events := make([]StoryPage8Event, 4)
	for i, r := range []rune("甲乙丙丁") {
		font.Glyphs[r] = make([]byte, 32)
		key := fmt.Sprintf("story.page8.line.%03d", i+1)
		text[key] = string(r)
		events[i] = StoryPage8Event{
			Generation: 1, EntryStep: uint64(100 + i*20), PostCallStep: uint64(110 + i*20),
			EventKey: key, Row: uint8(17 + i), Column: 1,
		}
	}
	overlay, err := NewRuntimeStoryPage8Overlay(text, font, scale)
	if err != nil {
		t.Fatal(err)
	}
	return overlay, events
}

func assertPage8OverlayBaseline(t *testing.T, overlay *RuntimeStoryPage8Overlay, scale int) {
	t.Helper()
	indexed := make([]byte, 320*200)
	palette := [256][3]uint8{}
	baseline := ScaleIndexedRGBA(indexed, palette, scale)
	got, missing, drew := overlay.Draw(indexed, palette)
	if len(overlay.ActiveKeys()) != 0 || drew || len(missing) != 0 || !bytes.Equal(got, baseline) {
		t.Fatalf("%dx 必須零 active/零 draw/RGBA=baseline: keys=%v drew=%v missing=%q equal=%v", scale, overlay.ActiveKeys(), drew, string(missing), bytes.Equal(got, baseline))
	}
}

func rgbaPixel(data []byte, width, x, y int) [4]byte {
	offset := (y*width + x) * 4
	return [4]byte{data[offset], data[offset+1], data[offset+2], data[offset+3]}
}

func TestStoryPage8OverlayClearsFullEnglishWidthWithoutRightBoundary(t *testing.T) {
	for _, scale := range []int{2, 3} {
		t.Run(fmt.Sprintf("%dx", scale), func(t *testing.T) {
			overlay, events := page8OverlayFixture(t, scale)
			palette := [256][3]uint8{}
			palette[0] = [3]uint8{1, 2, 3}
			palette[10] = [3]uint8{200, 100, 50}
			indexed := make([]byte, 320*200)
			for i := range indexed {
				indexed[i] = 10
			}
			baseline := ScaleIndexedRGBA(indexed, palette, scale)
			if err := overlay.Apply(events, palette); err != nil {
				t.Fatal(err)
			}
			got, missing, drew := overlay.Draw(indexed, palette)
			if !drew || len(missing) != 0 || len(overlay.ActiveKeys()) != 4 {
				t.Fatalf("drew=%v missing=%q keys=%v", drew, string(missing), overlay.ActiveKeys())
			}
			width := 320 * scale
			background := [4]byte{1, 2, 3, 255}
			for y := 136 * scale; y < 168*scale; y++ {
				for x := 8 * scale; x < 312*scale; x++ {
					if pixel := rgbaPixel(got, width, x, y); pixel != background {
						t.Fatalf("安全矩形輸出像素 (%d,%d) 未清成背景: %v", x, y, pixel)
					}
				}
			}
			for y := 136 * scale; y < 168*scale; y++ {
				for x := 312 * scale; x < 313*scale; x++ {
					if gotPixel, wantPixel := rgbaPixel(got, width, x, y), rgbaPixel(baseline, width, x, y); gotPixel != wantPixel {
						t.Fatalf("右界外輸出像素 (%d,%d) 被侵入: got=%v want=%v", x, y, gotPixel, wantPixel)
					}
				}
			}
			for y := 168 * scale; y < 169*scale; y++ {
				for x := 8 * scale; x < 312*scale; x++ {
					if gotPixel, wantPixel := rgbaPixel(got, width, x, y), rgbaPixel(baseline, width, x, y); gotPixel != wantPixel {
						t.Fatalf("下界外輸出像素 (%d,%d) 被侵入: got=%v want=%v", x, y, gotPixel, wantPixel)
					}
				}
			}
		})
	}
}

func TestStoryPage8OverlayFailClosedBothScales(t *testing.T) {
	for _, scale := range []int{2, 3} {
		overlay, good := page8OverlayFixture(t, scale)
		cases := []struct {
			name string
			edit func([]StoryPage8Event) []StoryPage8Event
		}{
			{"partial", func(events []StoryPage8Event) []StoryPage8Event { return events[:3] }},
			{"mixed generation", func(events []StoryPage8Event) []StoryPage8Event { events[2].Generation = 2; return events }},
			{"zero generation", func(events []StoryPage8Event) []StoryPage8Event { events[0].Generation = 0; return events }},
			{"duplicate key", func(events []StoryPage8Event) []StoryPage8Event {
				events[3].EventKey = events[2].EventKey
				return events
			}},
			{"wrong key", func(events []StoryPage8Event) []StoryPage8Event {
				events[1].EventKey = "story.page8.line.999"
				return events
			}},
			{"row", func(events []StoryPage8Event) []StoryPage8Event { events[1].Row++; return events }},
			{"column", func(events []StoryPage8Event) []StoryPage8Event { events[1].Column++; return events }},
			{"entry equals return", func(events []StoryPage8Event) []StoryPage8Event {
				events[1].EntryStep = events[1].PostCallStep
				return events
			}},
			{"entry after return", func(events []StoryPage8Event) []StoryPage8Event {
				events[1].EntryStep = events[1].PostCallStep + 1
				return events
			}},
			{"event step not increasing", func(events []StoryPage8Event) []StoryPage8Event {
				events[1].EntryStep = events[0].PostCallStep
				return events
			}},
		}
		for _, tc := range cases {
			t.Run(fmt.Sprintf("%dx/%s", scale, tc.name), func(t *testing.T) {
				events := append([]StoryPage8Event(nil), good...)
				if err := overlay.Apply(tc.edit(events), [256][3]uint8{}); err == nil {
					t.Fatal("無效 event group 被接受")
				}
				assertPage8OverlayBaseline(t, overlay, scale)
			})
		}
	}
}

func TestStoryPage8OverlayClearAndDiscontinuityReturnBaseline(t *testing.T) {
	for _, scale := range []int{2, 3} {
		t.Run(fmt.Sprintf("%dx", scale), func(t *testing.T) {
			overlay, events := page8OverlayFixture(t, scale)
			if err := overlay.Apply(events, [256][3]uint8{}); err != nil {
				t.Fatal(err)
			}
			overlay.Clear()
			assertPage8OverlayBaseline(t, overlay, scale)
		})
	}
}

func TestStoryPage8OverlayRejectsMissingTranslationAndGlyph(t *testing.T) {
	for _, scale := range []int{2, 3} {
		font := &xlate.Font{W: 16, H: 16, Glyphs: map[rune][]byte{'甲': make([]byte, 32)}}
		complete := map[string]string{}
		for i := 1; i <= 4; i++ {
			complete[fmt.Sprintf("story.page8.line.%03d", i)] = "甲"
		}
		missingTranslation := make(map[string]string, 4)
		for key, value := range complete {
			missingTranslation[key] = value
		}
		delete(missingTranslation, "story.page8.line.004")
		if _, err := NewRuntimeStoryPage8Overlay(missingTranslation, font, scale); err == nil {
			t.Fatal("缺譯文被接受")
		}
		missingGlyph := make(map[string]string, 4)
		for key, value := range complete {
			missingGlyph[key] = value
		}
		missingGlyph["story.page8.line.004"] = "乙"
		if _, err := NewRuntimeStoryPage8Overlay(missingGlyph, font, scale); err == nil {
			t.Fatal("缺字被接受")
		}
	}
}
