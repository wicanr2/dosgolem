package buckrogers

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"strings"
	"testing"

	"github.com/wicanr2/dosgolem/xlate"
)

func openingMatrixCatalog(t *testing.T) (*StoryOpeningCatalog, [][]byte) {
	t.Helper()
	raw := [][]byte{[]byte("aa"), []byte("bb"), []byte("cc"), []byte("dd"), []byte("ee")}
	entries := make([]StoryOpeningIdentity, len(raw))
	for i, line := range raw {
		entries[i] = StoryOpeningIdentity{
			Sequence: uint8(i + 1), EventKey: fmt.Sprintf("story.opening.line.%03d", i+1),
			OriginalLength: uint8(len(line)), OriginalSHA256: sha256.Sum256(line),
			Caller: Address{Segment: 0x0763, Offset: 0x04FF}, Guard: storyGlyphPrimitive,
			Mode: 1, Repeat: 1, Background: 0, Foreground: 10, Row: uint8(17 + i), Column: 1,
		}
	}
	catalog, err := NewStoryOpeningCatalog(entries)
	if err != nil {
		t.Fatal(err)
	}
	return catalog, raw
}

func openingMatrixPresenter(t *testing.T, scale int) *RuntimeStoryOpeningOverlay {
	t.Helper()
	font := &xlate.Font{W: 16, H: 16, Glyphs: map[rune][]byte{}}
	text := make(map[string]string, 5)
	for i, r := range []rune("甲乙丙丁戊") {
		font.Glyphs[r] = make([]byte, 32)
		text[fmt.Sprintf("story.opening.line.%03d", i+1)] = string(r)
	}
	o, err := NewRuntimeStoryOpeningOverlay(text, font, scale)
	if err != nil {
		t.Fatal(err)
	}
	return o
}

func assertOpeningMatrixBaseline(t *testing.T, scale int, w *StoryOpeningWatcher) {
	t.Helper()
	if w.Active() || len(w.Events()) != 0 {
		t.Fatalf("watcher 不得 active/events: active=%v events=%d", w.Active(), len(w.Events()))
	}
	o := openingMatrixPresenter(t, scale)
	indexed, palette := make([]byte, 320*200), [256][3]uint8{}
	baseline := ScaleIndexedRGBA(indexed, palette, scale)
	got, missing, drew := o.Draw(indexed, palette)
	if len(o.ActiveKeys()) != 0 || drew || len(missing) != 0 || !bytes.Equal(got, baseline) {
		t.Fatalf("%dx 失敗案例必須零 active、零 draw 且 RGBA=baseline: keys=%v drew=%v missing=%q equal=%v", scale, o.ActiveKeys(), drew, string(missing), bytes.Equal(got, baseline))
	}
}

func emitOpeningMatrixGroup(w *StoryOpeningWatcher, c *StoryOpeningCatalog, raw [][]byte) {
	step := uint64(10)
	for i, line := range raw {
		for j, glyph := range line {
			emitStoryGlyph(w, c.entries[i], glyph, uint8(j+1), 7, 9, step)
			step += 2
		}
	}
}

func TestStoryOpeningWatcherFailClosedMatrixBothScales(t *testing.T) {
	catalog, raw := openingMatrixCatalog(t)
	type scenario struct {
		name string
		run  func(*StoryOpeningWatcher)
	}
	bad := func(index int, value uint16) func(*StoryOpeningWatcher) {
		return func(w *StoryOpeningWatcher) {
			args := [7]uint16{1, uint16(raw[0][0]), 1, 0, 10, 17, 1}
			args[index] = value
			w.ObserveGlyphEntry(storyGlyphPrimitive, catalog.entries[0].Caller, 7, 9, args, 10)
			w.ObserveVerifiedGlyphReturn(StoryOpeningVerifiedReturn{EntryStep: 10, PostCallStep: 11, Caller: catalog.entries[0].Caller, SS: 7, SP: 9 + storyGlyphStackDelta})
		}
	}
	scenarios := []scenario{
		{"mode", bad(0, 2)}, {"glyph hash", bad(1, uint16(raw[0][0]^1))}, {"repeat", bad(2, 2)},
		{"background", bad(3, 1)}, {"foreground", bad(4, 9)}, {"row", bad(5, 18)}, {"column", bad(6, 2)},
		{"caller", func(w *StoryOpeningWatcher) {
			e := catalog.entries[0]
			e.Caller = Address{1, 2}
			emitStoryGlyph(w, e, raw[0][0], 1, 7, 9, 10)
		}},
		{"guard", func(w *StoryOpeningWatcher) {
			e := catalog.entries[0]
			e.Guard = Address{1, 2}
			emitStoryGlyph(w, e, raw[0][0], 1, 7, 9, 10)
		}},
		{"return step equal", func(w *StoryOpeningWatcher) {
			e := catalog.entries[0]
			args := [7]uint16{1, uint16(raw[0][0]), 1, 0, 10, 17, 1}
			w.ObserveGlyphEntry(e.Guard, e.Caller, 7, 9, args, 10)
			w.ObserveVerifiedGlyphReturn(StoryOpeningVerifiedReturn{EntryStep: 10, PostCallStep: 10, Caller: e.Caller, SS: 7, SP: 9 + storyGlyphStackDelta})
		}},
		{"partial", func(w *StoryOpeningWatcher) { emitStoryGlyph(w, catalog.entries[0], raw[0][0], 1, 7, 9, 10) }},
		{"mixed", func(w *StoryOpeningWatcher) {
			emitStoryGlyph(w, catalog.entries[0], raw[0][0], 1, 7, 9, 10)
			emitStoryGlyph(w, catalog.entries[1], raw[1][0], 1, 7, 9, 12)
		}},
		{"duplicate", func(w *StoryOpeningWatcher) {
			emitStoryGlyph(w, catalog.entries[0], raw[0][0], 1, 7, 9, 10)
			emitStoryGlyph(w, catalog.entries[0], raw[0][1], 2, 7, 9, 12)
			emitStoryGlyph(w, catalog.entries[0], raw[0][0], 1, 7, 9, 14)
		}},
		{"restore discontinuity", func(w *StoryOpeningWatcher) {
			emitStoryGlyph(w, catalog.entries[0], raw[0][0], 1, 7, 9, 10)
			w.ObserveExecutionDiscontinuity()
			emitStoryGlyph(w, catalog.entries[0], raw[0][1], 2, 7, 9, 12)
		}},
	}
	for _, scale := range []int{2, 3} {
		for _, tc := range scenarios {
			t.Run(fmt.Sprintf("%dx/%s", scale, tc.name), func(t *testing.T) {
				watcher, err := NewStoryOpeningWatcher(catalog)
				if err != nil {
					t.Fatal(err)
				}
				tc.run(watcher)
				assertOpeningMatrixBaseline(t, scale, watcher)
			})
		}
	}
}

func TestStoryOpeningActiveClearAndRestoreBothScales(t *testing.T) {
	catalog, raw := openingMatrixCatalog(t)
	for _, scale := range []int{2, 3} {
		t.Run(fmt.Sprintf("%dx prewrite", scale), func(t *testing.T) {
			watcher, _ := NewStoryOpeningWatcher(catalog)
			emitOpeningMatrixGroup(watcher, catalog, raw)
			o := openingMatrixPresenter(t, scale)
			if !watcher.Active() || o.Apply(watcher.Events(), [256][3]uint8{}) != nil {
				t.Fatal("前置完整 group 未 active/draw")
			}
			if !watcher.ObserveVideoWrite(storyFillInstruction, storyVideoSegment, 136*320+319, 2) {
				t.Fatal("相交 prewrite 未清除 active group")
			}
			o.Clear()
			assertOpeningMatrixBaseline(t, scale, watcher)
		})
		t.Run(fmt.Sprintf("%dx restore", scale), func(t *testing.T) {
			watcher, _ := NewStoryOpeningWatcher(catalog)
			emitOpeningMatrixGroup(watcher, catalog, raw)
			o := openingMatrixPresenter(t, scale)
			if !watcher.Active() || o.Apply(watcher.Events(), [256][3]uint8{}) != nil {
				t.Fatal("前置完整 group 未 active/draw")
			}
			watcher.ObserveExecutionDiscontinuity()
			o.Clear()
			assertOpeningMatrixBaseline(t, scale, watcher)
		})
	}
}

func TestStoryOpeningCatalogAndFontFailClosedMatrix(t *testing.T) {
	_, raw := openingMatrixCatalog(t)
	events := "event_key\tsequence\toriginal_length\toriginal_sha256\tcaller\tglyph_guard\tbackground\tforeground\trow\tcolumn\tentry_step\tpost_call_step\tevidence_level\tcatalog_status\n"
	text := "key\ttranslation\tsource\n"
	for i, line := range raw {
		key := fmt.Sprintf("story.opening.line.%03d", i+1)
		events += fmt.Sprintf("%s\t%d\t%d\t%x\t0763:04FF\t0763:026B\t0\t10\t%d\t1\t%d\t%d\tconfirmed\tREADY\n", key, i+1, len(line), sha256.Sum256(line), 17+i, 100+i, 200+i)
		text += key + "\t甲\truntime-editorial\n"
	}
	for _, tc := range []struct{ name, events, text string }{
		{"DRAFT catalog", strings.Replace(events, "READY", "DRAFT", 1), text},
		{"missing translation", events, strings.Replace(text, "story.opening.line.005\t甲\truntime-editorial\n", "", 1)},
		{"wrong key", strings.Replace(events, "story.opening.line.003", "story.opening.line.999", 1), text},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, _, err := LoadStoryOpeningCatalog("events", []byte(tc.events), "translations", []byte(tc.text)); err == nil {
				t.Fatal("非 READY 或不完整首屏 catalog 必須失敗即關閉")
			}
		})
	}
	for _, scale := range []int{2, 3} {
		t.Run(fmt.Sprintf("%dx missing glyph", scale), func(t *testing.T) {
			text := make(map[string]string, 5)
			for i := 1; i <= 5; i++ {
				text[fmt.Sprintf("story.opening.line.%03d", i)] = "甲"
			}
			if _, err := NewRuntimeStoryOpeningOverlay(text, &xlate.Font{W: 16, H: 16, Glyphs: map[rune][]byte{}}, scale); err == nil {
				t.Fatal("缺字必須失敗即關閉")
			}
		})
	}
}
