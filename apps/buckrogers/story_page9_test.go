package buckrogers

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"strings"
	"testing"

	"github.com/wicanr2/dosgolem/internal/machine"
	"github.com/wicanr2/dosgolem/xlate"
)

func page9Fake(t *testing.T) (*StoryPage9Watcher, [20]byte) {
	t.Helper()
	var raw [20]byte
	for i := range raw {
		raw[i] = byte(i + 1)
	}
	catalog := &StoryPage9Catalog{entry: StoryPage9Identity{EventKey: "story.page9.line.001", OriginalLength: 20,
		OriginalSHA256: sha256.Sum256(raw[:]), Caller: Address{0x0763, 0x04ff}, Guard: storyGlyphPrimitive,
		Mode: 1, Repeat: 1, Foreground: 10, Row: 17, Column: 1}}
	w, err := NewStoryPage9Watcher(catalog)
	if err != nil {
		t.Fatal(err)
	}
	return w, raw
}

func page9Glyph(w *StoryPage9Watcher, column int, glyph byte, mutate func(*[7]uint16), writes int, writeAt func(int, *machine.VideoWrite), previous Address, opcode byte, caller Address, ss, sp uint16) {
	step := uint64(100 + column*100)
	args := [7]uint16{1, uint16(glyph), 1, 0, 10, 17, uint16(column)}
	if mutate != nil {
		mutate(&args)
	}
	w.ObserveGlyphEntry(storyGlyphPrimitive, Address{0x0763, 0x04ff}, 7, 9, args, step)
	for i := 0; i < writes; i++ {
		v := machine.VideoWrite{Step: step + uint64(i) + 1, CS: 0x0763, IP: 0x184d,
			Offset: uint32((136+i/8)*320 + column*8 + i%8)}
		if writeAt != nil {
			writeAt(i, &v)
		}
		w.ObserveVideoWrite(v)
	}
	w.ObserveVerifiedGlyphReturn(previous, opcode, caller, ss, sp, step+90)
}

func page9GoodGlyph(w *StoryPage9Watcher, column int, glyph byte) {
	page9Glyph(w, column, glyph, nil, 64, nil, Address{0x0763, 0x03d6}, 0xca, Address{0x0763, 0x04ff}, 7, 9+storyGlyphStackDelta)
}

func page9Complete(w *StoryPage9Watcher, raw [20]byte) {
	for i, b := range raw {
		page9GoodGlyph(w, i+1, b)
	}
}

func TestStoryPage9AtomicTwentyReturns(t *testing.T) {
	w, raw := page9Fake(t)
	for i, b := range raw {
		page9GoodGlyph(w, i+1, b)
		if i < 19 && (w.Active() || len(w.Events()) != 0) {
			t.Fatalf("%d/20 過早提交", i+1)
		}
	}
	if !w.Active() || len(w.Events()) != 1 || w.Events()[0].Generation != 1 || w.Events()[0].EntryStep >= w.Events()[0].PostCallStep {
		t.Fatalf("20/20 未原子提交: %#v", w.Events())
	}
}

func TestStoryPage9PendingFailClosedMatrix(t *testing.T) {
	cases := []struct {
		name string
		run  func(*StoryPage9Watcher, [20]byte)
	}{
		{"19/20", func(w *StoryPage9Watcher, raw [20]byte) {
			for i := 0; i < 19; i++ {
				page9GoodGlyph(w, i+1, raw[i])
			}
		}},
		{"63 writes", func(w *StoryPage9Watcher, raw [20]byte) {
			page9Glyph(w, 1, raw[0], nil, 63, nil, Address{0x0763, 0x03d6}, 0xca, Address{0x0763, 0x04ff}, 7, 9+storyGlyphStackDelta)
		}},
		{"65 writes", func(w *StoryPage9Watcher, raw [20]byte) {
			page9Glyph(w, 1, raw[0], nil, 65, func(i int, v *machine.VideoWrite) {
				if i == 64 {
					v.Offset = 136*320 + 8
				}
			}, Address{0x0763, 0x03d6}, 0xca, Address{0x0763, 0x04ff}, 7, 9+storyGlyphStackDelta)
		}},
		{"wrong writer", func(w *StoryPage9Watcher, raw [20]byte) {
			page9Glyph(w, 1, raw[0], nil, 64, func(i int, v *machine.VideoWrite) {
				if i == 10 {
					v.IP = 0x1855
				}
			}, Address{0x0763, 0x03d6}, 0xca, Address{0x0763, 0x04ff}, 7, 9+storyGlyphStackDelta)
		}},
		{"wrong cell", func(w *StoryPage9Watcher, raw [20]byte) {
			page9Glyph(w, 1, raw[0], nil, 64, func(i int, v *machine.VideoWrite) {
				if i == 10 {
					v.Offset += 8
				}
			}, Address{0x0763, 0x03d6}, 0xca, Address{0x0763, 0x04ff}, 7, 9+storyGlyphStackDelta)
		}},
		{"frame exterior", func(w *StoryPage9Watcher, raw [20]byte) {
			page9GoodGlyph(w, 1, raw[0])
			w.ObserveVideoWrite(machine.VideoWrite{Step: 250, Offset: 136*320 + 8})
		}},
		{"bad hash", func(w *StoryPage9Watcher, raw [20]byte) { raw[19]++; page9Complete(w, raw) }},
		{"bad return address", func(w *StoryPage9Watcher, raw [20]byte) {
			page9Glyph(w, 1, raw[0], nil, 64, nil, Address{0x0763, 0x03d7}, 0xca, Address{0x0763, 0x04ff}, 7, 9+storyGlyphStackDelta)
		}},
		{"bad opcode", func(w *StoryPage9Watcher, raw [20]byte) {
			page9Glyph(w, 1, raw[0], nil, 64, nil, Address{0x0763, 0x03d6}, 0xcb, Address{0x0763, 0x04ff}, 7, 9+storyGlyphStackDelta)
		}},
		{"bad caller", func(w *StoryPage9Watcher, raw [20]byte) {
			page9Glyph(w, 1, raw[0], nil, 64, nil, Address{0x0763, 0x03d6}, 0xca, Address{0x0763, 0x0500}, 7, 9+storyGlyphStackDelta)
		}},
		{"bad SS", func(w *StoryPage9Watcher, raw [20]byte) {
			page9Glyph(w, 1, raw[0], nil, 64, nil, Address{0x0763, 0x03d6}, 0xca, Address{0x0763, 0x04ff}, 8, 9+storyGlyphStackDelta)
		}},
		{"bad SP", func(w *StoryPage9Watcher, raw [20]byte) {
			page9Glyph(w, 1, raw[0], nil, 64, nil, Address{0x0763, 0x03d6}, 0xca, Address{0x0763, 0x04ff}, 7, 9)
		}},
		{"wrong column", func(w *StoryPage9Watcher, raw [20]byte) {
			page9Glyph(w, 1, raw[0], func(a *[7]uint16) { a[6] = 2 }, 64, nil, Address{0x0763, 0x03d6}, 0xca, Address{0x0763, 0x04ff}, 7, 9+storyGlyphStackDelta)
		}},
		{"wrong style", func(w *StoryPage9Watcher, raw [20]byte) {
			page9Glyph(w, 1, raw[0], func(a *[7]uint16) { a[4] = 9 }, 64, nil, Address{0x0763, 0x03d6}, 0xca, Address{0x0763, 0x04ff}, 7, 9+storyGlyphStackDelta)
		}},
		{"wrong entry caller", func(w *StoryPage9Watcher, raw [20]byte) {
			w.ObserveGlyphEntry(storyGlyphPrimitive, Address{0x0763, 0x0500}, 7, 9, [7]uint16{1, uint16(raw[0]), 1, 0, 10, 17, 1}, 200)
		}},
		{"wrong guard", func(w *StoryPage9Watcher, raw [20]byte) {
			w.ObserveGlyphEntry(Address{0x0763, 0x026c}, Address{0x0763, 0x04ff}, 7, 9, [7]uint16{1, uint16(raw[0]), 1, 0, 10, 17, 1}, 200)
		}},
		{"wrong row", func(w *StoryPage9Watcher, raw [20]byte) {
			page9Glyph(w, 1, raw[0], func(a *[7]uint16) { a[5] = 18 }, 64, nil, Address{0x0763, 0x03d6}, 0xca, Address{0x0763, 0x04ff}, 7, 9+storyGlyphStackDelta)
		}},
		{"wrong mode", func(w *StoryPage9Watcher, raw [20]byte) {
			page9Glyph(w, 1, raw[0], func(a *[7]uint16) { a[0] = 2 }, 64, nil, Address{0x0763, 0x03d6}, 0xca, Address{0x0763, 0x04ff}, 7, 9+storyGlyphStackDelta)
		}},
		{"wrong repeat", func(w *StoryPage9Watcher, raw [20]byte) {
			page9Glyph(w, 1, raw[0], func(a *[7]uint16) { a[2] = 2 }, 64, nil, Address{0x0763, 0x03d6}, 0xca, Address{0x0763, 0x04ff}, 7, 9+storyGlyphStackDelta)
		}},
		{"wrong background", func(w *StoryPage9Watcher, raw [20]byte) {
			page9Glyph(w, 1, raw[0], func(a *[7]uint16) { a[3] = 1 }, 64, nil, Address{0x0763, 0x03d6}, 0xca, Address{0x0763, 0x04ff}, 7, 9+storyGlyphStackDelta)
		}},
		{"duplicate column", func(w *StoryPage9Watcher, raw [20]byte) {
			page9GoodGlyph(w, 1, raw[0])
			page9Glyph(w, 2, raw[1], func(a *[7]uint16) { a[6] = 1 }, 64, nil, Address{0x0763, 0x03d6}, 0xca, Address{0x0763, 0x04ff}, 7, 9+storyGlyphStackDelta)
		}},
		{"offset >=64000", func(w *StoryPage9Watcher, raw [20]byte) {
			w.ObserveGlyphEntry(storyGlyphPrimitive, Address{0x0763, 0x04ff}, 7, 9, [7]uint16{1, uint16(raw[0]), 1, 0, 10, 17, 1}, 200)
			w.ObserveVideoWrite(machine.VideoWrite{Step: 201, Offset: 64000})
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			w, raw := page9Fake(t)
			tc.run(w, raw)
			if w.Active() || len(w.Events()) != 0 {
				t.Fatal("失敗案例顯示覆繪")
			}
			if tc.name != "19/20" && w.Pending() {
				t.Fatal("異常後 pending 未清")
			}
		})
	}
	for i := 0; i < 7; i++ {
		t.Run(fmt.Sprintf("ABI high %d", i), func(t *testing.T) {
			w, raw := page9Fake(t)
			page9Glyph(w, 1, raw[0], func(a *[7]uint16) { a[i] |= 0x100 }, 64, nil, Address{0x0763, 0x03d6}, 0xca, Address{0x0763, 0x04ff}, 7, 9+storyGlyphStackDelta)
			if w.Active() || w.Pending() {
				t.Fatal("ABI high word 被接受")
			}
		})
	}
}

func TestStoryPage9ActivePrewriteBoundariesAndLifecycle(t *testing.T) {
	for _, tc := range []struct {
		name   string
		offset uint32
		clear  bool
	}{
		{"x7", 136*320 + 7, false}, {"x8", 136*320 + 8, true}, {"x167", 143*320 + 167, true}, {"x168", 136*320 + 168, false},
		{"row135", 135*320 + 8, false}, {"row136", 136*320 + 8, true}, {"row143", 143*320 + 8, true}, {"row144", 144*320 + 8, false},
		{"row15", 15*8*320 + 8, false}, {"row24", 24*8*320 + 8, false}, {"invalid", 64000, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			w, raw := page9Fake(t)
			page9Complete(w, raw)
			before := w.Generation()
			changed := w.ObserveVideoWrite(machine.VideoWrite{Step: 4000, Offset: tc.offset})
			if changed != tc.clear || w.Active() == tc.clear || (tc.clear && w.Generation() != before+1) {
				t.Fatalf("changed=%v active=%v generation=%d", changed, w.Active(), w.Generation())
			}
		})
	}
	for _, name := range []string{"Stop", "Restore", "epoch"} {
		t.Run(name, func(t *testing.T) {
			w, raw := page9Fake(t)
			page9Complete(w, raw)
			switch name {
			case "Stop":
				w.Stop()
			case "Restore":
				w.Restore()
			default:
				w.ObserveExecutionDiscontinuity()
			}
			if w.Active() || len(w.Events()) != 0 {
				t.Fatal("舊覆繪殘留")
			}
			page9GoodGlyph(w, 1, raw[0])
			if w.Active() {
				t.Fatal("只重播一字不得顯示")
			}
		})
	}
}

func TestStoryPage9ReadyCatalogAndOverlay(t *testing.T) {
	events := strings.Join(storyPage8EventHeader, "\t") + "\n" + "story.page9.line.001\t1\t20\t" + storyPage9ApprovedHash + "\t0763:04FF\t0763:026B\t0\t10\t17\t1\t351155910\t351988536\tconfirmed\tREADY\n"
	translation := "key\ttranslation\tsource\nstory.page9.line.001\t你們列隊離開。\truntime-editorial\n"
	_, text, err := LoadStoryPage9Catalog("events", []byte(events), "translations", []byte(translation))
	if err != nil {
		t.Fatal(err)
	}
	for _, bad := range []string{"DRAFT", "unknown"} {
		candidate := strings.Replace(events, "READY", bad, 1)
		if _, _, err := LoadStoryPage9Catalog("events", []byte(candidate), "translations", []byte(translation)); err == nil {
			t.Fatalf("%s 狀態被接受", bad)
		}
	}
	if _, _, err := LoadStoryPage9Catalog("events", []byte(events), "translations", []byte(strings.Replace(translation, "列隊", "押送", 1))); err == nil {
		t.Fatal("未核准譯文被接受")
	}
	for _, scale := range []int{2, 3} {
		font := &xlate.Font{W: 16, H: 16, Glyphs: map[rune][]byte{}}
		for _, r := range text["story.page9.line.001"] {
			font.Glyphs[r] = bytes.Repeat([]byte{0xff}, 32)
		}
		o, err := NewRuntimeStoryPage9Overlay(text, font, scale)
		if err != nil {
			t.Fatal(err)
		}
		e := StoryPage9Event{Generation: 1, EntryStep: 10, PostCallStep: 20, EventKey: "story.page9.line.001", Row: 17, Column: 1}
		if err := o.Apply(e, [256][3]uint8{}); err != nil {
			t.Fatal(err)
		}
		indexed := make([]byte, 320*200)
		palette := [256][3]uint8{}
		palette[10] = [3]uint8{255, 255, 255}
		o.Frame(indexed, palette)
		base := ScaleIndexedRGBA(indexed, palette, scale)
		got, missing, drew := o.Draw(indexed, palette)
		if !drew || len(missing) != 0 {
			t.Fatal("覆繪失敗")
		}
		width := 320 * scale
		outside, inside := 0, 0
		for i := 0; i < len(got); i += 4 {
			if bytes.Equal(got[i:i+4], base[i:i+4]) {
				continue
			}
			x, y := (i/4)%width, (i/4)/width
			if x < 8*scale || x >= 168*scale || y < 136*scale || y >= 144*scale {
				outside++
			} else {
				inside++
			}
		}
		if outside != 0 || inside == 0 {
			t.Fatalf("scale=%d outside=%d inside=%d", scale, outside, inside)
		}
		o.Clear()
		if len(o.ActiveKeys()) != 0 {
			t.Fatal("stamp 未清")
		}
	}
}

func TestStoryPage9OwnerClearsWatcherAndStamp(t *testing.T) {
	for _, action := range []string{"prewrite", "Stop", "Restore", "epoch"} {
		t.Run(action, func(t *testing.T) {
			w, raw := page9Fake(t)
			font := &xlate.Font{W: 16, H: 16, Glyphs: map[rune][]byte{'甲': bytes.Repeat([]byte{0xff}, 32)}}
			p, err := NewRuntimeStoryPage9Overlay(map[string]string{"story.page9.line.001": "甲"}, font, 2)
			if err != nil {
				t.Fatal(err)
			}
			o, err := NewStoryPage9Owner(w, p)
			if err != nil {
				t.Fatal(err)
			}
			page9Complete(w, raw)
			if err := p.Apply(w.Events()[0], [256][3]uint8{}); err != nil {
				t.Fatal(err)
			}
			switch action {
			case "prewrite":
				if !o.Prewrite(machine.VideoWrite{Step: 4000, Offset: 136*320 + 8, Value: 0}) {
					t.Fatal("同值相交寫入未失效")
				}
			case "Stop":
				o.Stop()
			case "Restore":
				o.Restore()
			case "epoch":
				o.ObserveExecutionDiscontinuity()
			}
			if w.Active() || len(w.Events()) != 0 || len(p.ActiveKeys()) != 0 {
				t.Fatal("watcher 或 RGBA stamp 殘留")
			}
		})
	}
}
