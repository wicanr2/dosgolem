package buckrogers

import (
	"bytes"
	"testing"

	"github.com/wicanr2/dosgolem/xlate"
)

func overlayFont() *xlate.Font {
	glyph := bytes.Repeat([]byte{0xff}, 32)
	return &xlate.Font{W: 16, H: 16, Glyphs: map[rune][]byte{'地': glyph, '球': glyph}}
}

func overlayEntry() MenuOverlayEntry {
	return MenuOverlayEntry{
		EventKey: "race.heading.terran", TextKey: "race.terran", Translation: "地球",
		Background: 15, Foreground: 0, X: 8, Y: 24, Width: 64, Height: 8,
		DrawX: 24, DrawY: 24, Capacity: 6, LineCount: 1, Overflow: "single-line-reject",
	}
}

func TestBuildMenuOverlayTwoAndThreeScale(t *testing.T) {
	var palette [256][3]uint8
	palette[10] = [3]uint8{85, 255, 85}
	for _, scale := range []int{2, 3} {
		overlay, err := BuildMenuOverlay([]MenuOverlayEntry{overlayEntry()}, overlayFont(), palette, scale)
		if err != nil {
			t.Fatal(err)
		}
		if len(overlay.Layer.Stamps) != 1 || len(overlay.Events) != 1 {
			t.Fatal("應建立單一完整覆繪")
		}
		stamp, event := overlay.Layer.Stamps[0], overlay.Events[0]
		wantOffset := 0
		if scale == 3 {
			wantOffset = 4
		}
		if stamp.GlyphX != wantOffset || stamp.GlyphY != wantOffset {
			t.Fatalf("scale %d offset=%d,%d", scale, stamp.GlyphX, stamp.GlyphY)
		}
		if stamp.BG != [3]uint8{} || stamp.FG != [3]uint8{} {
			t.Fatal("selected 黑底黑字不得被改色")
		}
		if event.ClearRect != (PixelRect{8 * scale, 24 * scale, 64 * scale, 8 * scale}) ||
			event.DrawAnchorX != 24*scale || !event.Contained {
			t.Fatalf("幾何不符：%#v", event)
		}
		if string(stamp.Text[:2]) != "　　" || string(stamp.Text[2:]) != "地球" {
			t.Fatalf("prefix／譯文不符：%q", string(stamp.Text))
		}
		buf := make([]byte, 320*scale*200*scale*4)
		if !overlay.Layer.Draw(buf, scale, nil) {
			t.Fatal("有效 layer 應可繪製")
		}
	}
}

func TestBuildMenuOverlayRejectsInvalidBatchAtomically(t *testing.T) {
	valid := overlayEntry()
	cases := []struct {
		name   string
		mutate func(*MenuOverlayEntry)
		font   *xlate.Font
		scale  int
	}{
		{"零倍率", func(*MenuOverlayEntry) {}, overlayFont(), 0},
		{"錯誤字型尺寸", func(*MenuOverlayEntry) {}, &xlate.Font{W: 8, H: 8}, 2},
		{"缺字", func(e *MenuOverlayEntry) { e.Translation = "地月" }, overlayFont(), 2},
		{"容量", func(e *MenuOverlayEntry) { e.Capacity++ }, overlayFont(), 2},
		{"越界", func(e *MenuOverlayEntry) { e.X = 300 }, overlayFont(), 2},
		{"錯誤 overflow", func(e *MenuOverlayEntry) { e.Overflow = "truncate" }, overlayFont(), 2},
		{"空譯文", func(e *MenuOverlayEntry) { e.Translation = "" }, overlayFont(), 2},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			e := valid
			tc.mutate(&e)
			if got, err := BuildMenuOverlay([]MenuOverlayEntry{e}, tc.font, [256][3]uint8{}, tc.scale); err == nil || got != nil {
				t.Fatalf("無效批次應回 nil,error；got=%#v err=%v", got, err)
			}
		})
	}
}

func TestBuildMenuOverlayRejectsOverlapAndDuplicate(t *testing.T) {
	a, b := overlayEntry(), overlayEntry()
	b.EventKey, b.TextKey = "race.selection.selected.martian", "race.martian"
	if got, err := BuildMenuOverlay([]MenuOverlayEntry{a, b}, overlayFont(), [256][3]uint8{}, 2); err == nil || got != nil {
		t.Fatal("重疊批次必須整批失敗")
	}
	b = overlayEntry()
	b.X, b.Y, b.DrawX, b.DrawY = 80, 32, 96, 32
	if got, err := BuildMenuOverlay([]MenuOverlayEntry{a, b}, overlayFont(), [256][3]uint8{}, 2); err == nil || got != nil {
		t.Fatal("重複 event key 必須整批失敗")
	}
}
